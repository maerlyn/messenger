package fb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
)

//https://github.com/Schmavery/facebook-chat-api/pull/765/files
type Config struct {
	UserAgent string
	Cookie    string
	UserID    string
}

type Logger interface {
	App(text string)
	Error(text string)
	Raw(text string)
}

type Client struct {
	log    Logger
	config Config

	eventChannel chan<- interface{}

	httpClient http.Client
	dtsg       string
	lastSeqId  string

	groups []messengerGroup

	lastActiveTimes                lastActiveTimes
	lastActiveTimesMutex           sync.Mutex
	lastActiveTimesUpdatedChannels []chan<- bool

	friendNames      map[string]string
	friendNamesMutex sync.Mutex
	friendList       friendList

	fbStartPage                []byte
	fbStartPageMutex           sync.Mutex
	fbStartPageUpdatedChannels []chan bool

	sessionId uint64
	clientId  string
	syncToken string

	mqttClient mqtt.Client
}

func NewClient(log Logger, conf Config) *Client {
	c := &Client{}

	c.config = conf
	c.log = log

	var t http.RoundTripper = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	c.httpClient = http.Client{
		Transport: t,
	}

	c.friendNames = make(map[string]string)
	c.friendNamesMutex = sync.Mutex{}
	c.fbStartPageMutex = sync.Mutex{}
	c.lastActiveTimes = make(map[string]int64)
	c.lastActiveTimesMutex = sync.Mutex{}

	err := c.fetchStartPage()
	if err != nil {
		panic("initial startpage fetch failed: " + err.Error())
	}

	c.updateDtsg()
	c.updateGroups()

	go func() {
		t := time.NewTicker(60 * time.Second)

		for {
			<-t.C

			err := c.fetchStartPage()
			if err != nil {
				log.Error(fmt.Sprintf("error fetching startpage: %s", err.Error()))
				continue
			}

			c.updateDtsg()
			c.updateGroups()
			c.updateLastActiveTimes()
		}
	}()

	c.updateFriendsList()
	c.updateLastActiveTimes()
	c.LoadFriend(conf.UserID)

	c.sessionId = rand.Uint64()
	id, _ := uuid.NewRandom()
	c.clientId = id.String()

	c.log.App(fmt.Sprintf("generated client id: %s session id: %s", c.clientId, strconv.FormatInt(int64(c.sessionId), 10)))

	return c
}

func (c *Client) SetEventChannel(channel chan<- interface{}) {
	c.eventChannel = channel
}

func (c *Client) emit(event interface{}) {
	if c.eventChannel != nil {
		c.eventChannel <- event
	}
}

func (c *Client) getStartPage() []byte {
	c.fbStartPageMutex.Lock()
	defer c.fbStartPageMutex.Unlock()
	return c.fbStartPage
}

func (c *Client) fetchStartPage() error {
	body, err := c.doHttpRequest("GET", "https://www.facebook.com/", nil, 10*time.Second)
	if err != nil {
		return err
	}

	bs := string(body)
	c.log.App(fmt.Sprintf("startpage body len: %d", len(bs)))

	file, _ := os.Create("fb_response.html")
	_, _ = fmt.Fprint(file, bs)
	_ = file.Close()

	c.fbStartPageMutex.Lock()
	defer c.fbStartPageMutex.Unlock()
	c.fbStartPage = body

	for _, c := range c.fbStartPageUpdatedChannels {
		c <- true
	}

	return nil
}

func (c *Client) updateDtsg() {
	bs := string(c.getStartPage())

	dtsgInitialDataIndex := strings.Index(bs, "DTSGInitialData\"")

	if dtsgInitialDataIndex == -1 {
		return
	}

	tokenStartIndex := dtsgInitialDataIndex + strings.Index(bs[dtsgInitialDataIndex:], "\"token\":\"")
	tokenEndIndex := tokenStartIndex + strings.Index(bs[tokenStartIndex:], "\"}")

	newDtsg := bs[tokenStartIndex+len("\"token:\":\"")-1 : tokenEndIndex]
	if newDtsg != c.dtsg {
		c.dtsg = newDtsg
		c.log.App(fmt.Sprintf("changing dtsg to %s", newDtsg))
	}
}

func (c *Client) updateGroups() {
	bs := string(c.getStartPage())

	groupsIndex := strings.Index(bs, "groups:[{")
	if groupsIndex == -1 {
		return
	}

	groupsEnd := strings.Index(bs[groupsIndex:], "}],list")
	groupsPart := bs[groupsIndex+len("groups:") : groupsIndex+groupsEnd+2]

	for _, s := range []string{
		"uid",
		"mercury_thread",
		"participants",
		"image_src",
		"short_name",
		"name",
		"participants_to_render",
		"id",
		"text",
	} {
		groupsPart = strings.Replace(groupsPart, fmt.Sprintf("%s:", s), fmt.Sprintf("\"%s\":", s), -1)
	}

	err := json.Unmarshal([]byte(groupsPart), &c.groups)
	if err != nil {
		c.log.Error(fmt.Sprintf("error unmarshaling groups: %s\n\nraw: %s\n\n", err, groupsPart))
		return
	}

	go func() {
		for _, group := range c.groups {
			for _, userId := range group.MercuryThread.Participants {
				if _, ok := c.friendNames[userId]; !ok {
					c.LoadFriend(userId)
				}
			}
		}
	}()
}

func (c *Client) updateFriendsList() {
	body := c.getStartPage()
	bs := string(body)

	if len(body) == 0 {
		return
	}

	shotProfilesIndex := strings.Index(bs, "shortProfiles")
	nearbyIndex := strings.Index(bs, "nearby:")

	if shotProfilesIndex == -1 {
		return
	}

	shortProfilesPart := bs[shotProfilesIndex+14 : nearbyIndex-1]

	for _, s := range []string{
		"id",
		"name",
		"firstName",
		"vanity",
		"thumbSrc",
		"uri",
		"gender",
		"i18nGender",
		"type",
		"is_friend",
		"mThumbSrcSmall",
		"mThumbSrcLarge",
		"dir",
		"searchTokens",
		"alternateName",
		"is_nonfriend_messenger_contact",
		"is_birthday",
		"is_blocked",
	} {
		shortProfilesPart = strings.Replace(shortProfilesPart, fmt.Sprintf("%s:", s), fmt.Sprintf("\"%s\":", s), -1)
	}

	friendList := friendList{}

	err := json.Unmarshal([]byte(shortProfilesPart), &friendList)
	if err != nil {
		c.log.Error(fmt.Sprintf("error unmarshaling friend list: %s\nraw: %s\n\n", err, shortProfilesPart))
	}

	c.log.App(fmt.Sprintf("loaded %d friends", len(friendList)))
	c.friendList = friendList

	for _, friend := range friendList {
		c.friendNames[friend.Id] = friend.Name
	}
}

func (c *Client) updateLastActiveTimes() {
	body := c.getStartPage()
	bs := string(body)

	if len(bs) == 0 {
		c.log.Error(fmt.Sprintf("empty LAT response\n"))
		return
	}

	lastActiveTimesIndex := strings.Index(bs, "lastActiveTimes")
	if lastActiveTimesIndex == -1 {
		return
	}

	nextCloseBracket := strings.Index(bs[lastActiveTimesIndex:], "}")
	lastActiveTimesPart := bs[lastActiveTimesIndex+len("lastActiveTimes:") : lastActiveTimesIndex+nextCloseBracket+1]

	lat := lastActiveTimes{}
	err := json.Unmarshal([]byte(lastActiveTimesPart), &lat)
	if err != nil {
		c.log.Error(fmt.Sprintf("error unmarshaling LAT: %s\nraw: %s\n\n", err, lastActiveTimesPart))
		return
	}

	c.lastActiveTimesMutex.Lock()
	defer c.lastActiveTimesMutex.Unlock()
	for k, v := range lat {
		if c.lastActiveTimes[k] < v {
			c.lastActiveTimes[k] = v
		}
	}

	for _, v := range c.lastActiveTimesUpdatedChannels {
		v <- true
	}
}

func (c *Client) fetchLastSeqId() {
	formData := url.Values{}
	formData.Add("fb_dtsg", c.dtsg)
	formData.Add("queries", `{"o0":{"doc_id":"1349387578499440", "query_params":{"limit":1, "tags": ["INBOX"],"before": null, "includeDeliveryReceipts": false,"includeSeqID": true}}}`)
	body := bytes.NewBufferString(formData.Encode())

	resp, err := c.doHttpRequest("POST", "https://www.facebook.com/api/graphqlbatch/", body, time.Minute)
	if err != nil {
		c.log.Error(fmt.Sprintf("error fetching last seq id: %s\n", err))
		return
	}

	firstLine := strings.Split(string(resp), "\n")[0]
	responseObj := lastSeqIdResponse{}
	err = json.Unmarshal([]byte(firstLine), &responseObj)
	if err != nil {
		c.log.Error(fmt.Sprintf("error unmarshaling last seq id response: %s\n", err))
	}

	c.lastSeqId = responseObj.O0.Data.Viewer.MessageThreads.SyncSequenceId
}

func (c *Client) doHttpRequest(method, url string, body io.Reader, timeout time.Duration) ([]byte, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		c.log.Error(fmt.Sprintf("error creating http request for %s %s: %s\n", method, url, err))
		return nil, err
	}

	req.Header.Add("User-Agent", c.config.UserAgent)
	req.Header.Add("Cookie", c.config.Cookie)
	req.Header.Add("Referer", "https://www.facebook.com")

	if method == "POST" && body != nil {
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		c.log.Error(fmt.Sprintf("error doing http request for %s %s: %s\n", method, url, err))
		return nil, err
	}

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.log.Error(fmt.Sprintf("error reading body for %s %s: %s\n", method, url, err))
		return nil, err
	}

	return responseBody, nil
}

func (c *Client) FriendNames() map[string]string {
	c.friendNamesMutex.Lock()
	defer c.friendNamesMutex.Unlock()

	return c.friendNames
}

func (c *Client) IsGroup(id string) bool {
	for _, g := range c.groups {
		if g.Uid == id {
			return true
		}
	}

	return false
}

func (c *Client) GroupName(id string) string {
	for _, g := range c.groups {
		if g.Uid == id {
			if g.MercuryThread.Name != "" {
				return g.MercuryThread.Name
			} else {
				ret := make([]string, 0, len(g.ParticipantsToRender))

				for _, p := range g.ParticipantsToRender {
					ret = append(ret, p.ShortName)
				}

				return strings.Join(ret, ", ")
			}
		}
	}

	return ""
}

func (c *Client) LastActive(userId string) int64 {
	c.lastActiveTimesMutex.Lock()
	defer c.lastActiveTimesMutex.Unlock()

	return c.lastActiveTimes[userId]
}

func (c *Client) RequestNotifyLastActiveTimesUpdate(ch chan<- bool) {
	c.lastActiveTimesUpdatedChannels = append(c.lastActiveTimesUpdatedChannels, ch)
}

func (c *Client) LoadFriend(id string) bool {
	formData := url.Values{}
	formData.Add("fb_dtsg", c.dtsg)
	formData.Add("queries", fmt.Sprintf("{\"o0\":{\"doc_id\":\"1939519269502621\",\"query_params\":{\"ids\":[\"%s\"]}}}", id))
	body := bytes.NewBufferString(formData.Encode())

	resp, err := c.doHttpRequest("POST", "https://www.facebook.com/api/graphqlbatch/", body, time.Minute)
	if err != nil {
		c.log.Error(fmt.Sprintf("error fetching data for friend %s: %s\n", id, err))
		return false
	}

	line := strings.Split(string(resp), "\n")[0]
	friendData := friendData{}
	err = json.Unmarshal([]byte(line), &friendData)
	if err != nil {
		c.log.Error(fmt.Sprintf("error unmarshaling in getFriendData: %s\nraw:%s\n\n", err, line))
		return false
	}

	c.friendNamesMutex.Lock()
	defer c.friendNamesMutex.Unlock()

	c.friendNames[id] = friendData.O0.Data.MessagingActors[0].Name

	return true
}

func (c *Client) LastActiveAll() map[string]int64 {
	c.lastActiveTimesMutex.Lock()
	defer c.lastActiveTimesMutex.Unlock()

	return c.lastActiveTimes
}

func (c *Client) GetLastMessages(userId string, limit int) []Message {
	formData := url.Values{}
	formData.Add("fb_dtsg", c.dtsg)
	formData.Add("queries", fmt.Sprintf("{\"o0\":{\"doc_id\":\"1812394085554099\",\"query_params\":{\"id\":\"%s\",\"message_limit\":%d,\"load_messages\":true,\"load_read_receipts\":false,\"load_delivery_receipts\":false}}}", userId, limit))

	body := bytes.NewBufferString(formData.Encode())

	resp, err := c.doHttpRequest("POST", "https://www.facebook.com/api/graphqlbatch/", body, 10*time.Second)
	if err != nil {
		c.log.Error(fmt.Sprintf("error doing request for GetLastMessages: %s\n", err))
		return []Message{}
	}

	line := strings.Split(string(resp), "\n")[0]

	lmr := lastMessagesResponse{}
	err = json.Unmarshal([]byte(line), &lmr)
	if err != nil {
		c.log.Error(fmt.Sprintf("error unmarshaling GetLastMessages: %s\n\nraw: %s\n\n", err, line))
		return []Message{}
	}

	ret := make([]Message, 0, limit)

	// replies _could_ be handled, intentionally omitting that
	// they get delivered here like normal messages, only with the reply-to message id set
	for _, v := range lmr.O0.Data.MessageThread.Messages.Nodes {
		msg := Message{}
		msg.fromLastMessage(v)
		msg.Thread.fromFBType(lmr.O0.Data.MessageThread.ThreadKey)

		ret = append(ret, msg)
	}

	return ret
}

func (c *Client) FriendName(userId string) string {
	name := userId

	v, ok := c.FriendNames()[userId]
	if ok {
		name = v
	} else {
		if c.LoadFriend(userId) {
			name = c.FriendNames()[userId]
		}
	}

	return name
}

func (c *Client) MarkAsDelivered(msg Message) {
	formData := url.Values{}
	formData.Add("fb_dtsg", c.dtsg)
	formData.Add("message_ids[0]", msg.MessageId)
	formData.Add(fmt.Sprintf("thread_ids[%s][0]", msg.Thread.UniqueId()), msg.MessageId)

	body := bytes.NewBufferString(formData.Encode())

	_, err := c.doHttpRequest("POST", "https://www.facebook.com/ajax/mercury/delivery_receipts.php", body, 10*time.Second)
	if err != nil {
		c.log.Error(fmt.Sprintf("error doing request for MarkAsDelivered: %s\n", err))
		return
	}
}

func (c *Client) MarkAsRead(msg Message) {
	k := strconv.Itoa(int(time.Now().Unix() * 1000))
	l := strconv.Itoa(int(rand.Uint32()))
	threadingId := fmt.Sprintf("<%s:%s-%s@mail.projektitan.com>", k, l, c.clientId)

	formData := url.Values{}
	formData.Add("ids["+msg.Thread.UniqueId()+"]", "1")
	formData.Add("watermarkTimestamp", strconv.Itoa(int(msg.TimestampPrecise)))
	formData.Add("shouldSendReadReceipt", "1")
	formData.Add("commence_last_message_type", "non_ad")
	formData.Add("titanOriginatedThreadId", threadingId)
	formData.Add("fb_dtsg", c.dtsg)

	body := bytes.NewBufferString(formData.Encode())
	c.log.App(fmt.Sprintf("mark as read request body: %s\n", body))

	resp, err := c.doHttpRequest("POST", "https://www.facebook.com/ajax/mercury/change_read_status.php", body, 10*time.Second)
	if err != nil {
		c.log.Error(fmt.Sprintf("error doing request for MarkAsRead: %s\n", err))
	}

	c.log.App(fmt.Sprintf("MarkAsRead response: %s", resp))
}

func (c *Client) SendMessage(userOrGroupId, text string) {
	messageAndOTId := strconv.Itoa(int(time.Now().UnixNano())) //TODO generate

	formData := url.Values{}
	formData.Add("client", "mercury")
	formData.Add("action_type", "ma-type:user-generated-message")
	formData.Add("ephemeral_tll_mode", "0")
	formData.Add("has_attachment", "false")
	formData.Add("message_id", messageAndOTId)
	formData.Add("offline_threading_id", messageAndOTId)
	formData.Add("source", "source:chat:web")
	formData.Add("timestamp", strconv.Itoa(int(time.Now().Unix()*1000)))
	formData.Add("__user", c.config.UserID)
	formData.Add("__a", "1")
	formData.Add("fb_dtsg", c.dtsg)
	formData.Add("body", text)

	if c.IsGroup(userOrGroupId) {
		formData.Add("thread_id", userOrGroupId)
	} else {
		formData.Add("other_user_fbid", userOrGroupId)
	}

	body := bytes.NewBufferString(formData.Encode())
	c.log.App(fmt.Sprintf("send request body: %s\n\n\n", body))

	resp, err := c.doHttpRequest("POST", "https://www.facebook.com/messaging/send/", body, 10*time.Second)
	if err != nil {
		c.log.Error(fmt.Sprintf("error doing request for send: %s", err))
		return
	}

	c.log.App(fmt.Sprintf("send response: %s\n\n", string(resp)))
}
