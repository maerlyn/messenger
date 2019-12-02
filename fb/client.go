package fb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
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

	httpClient http.Client
	dtsg       string
	lastSeqId  string

	groups []messengerGroup

	lastActiveTimes      lastActiveTimes
	lastActiveTimesMutex sync.Mutex

	friendNames      map[string]string
	friendNamesMutex sync.Mutex
	friendList       friendList

	fbStartPage                []byte
	fbStartPageMutex           sync.Mutex
	fbStartPageUpdatedChannels []chan bool

	sessionId uint64
	clientId  string
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

	_ = c.fetchStartPage()
	c.updateDtsg()
	c.updateGroups()
	c.fetchLastSeqId()

	go func() {
		t := time.NewTicker(60 * time.Second)

		for {
			<-t.C

			_ = c.fetchStartPage()

			c.updateDtsg()
			c.updateGroups()
			c.updateLastActiveTimes()
		}
	}()

	c.updateFriendsList()
	c.updateLastActiveTimes()
	//c.GetFriendData(conf.CUser) //meglegyen a sajat nevem a kiirashoz

	c.sessionId = rand.Uint64()
	id, _ := uuid.NewRandom()
	c.clientId = id.String()

	return c
}

func (c *Client) Listen() {
	h := http.Header{}
	h.Add("Cookie", c.config.Cookie)
	h.Add("User-Agent", c.config.UserAgent)
	h.Add("Accept", "*/*")
	h.Add("Referer", "https://www.facebook.com")

	connOpts := mqtt.
		NewClientOptions().
		AddBroker(fmt.Sprintf("wss://edge-chat.facebook.com/chat?region=lla&sid=%d", c.sessionId)).
		SetClientID("mqttwsclient").
		SetUsername(fmt.Sprintf(`{
			"u": "%s",
			"s": %d, 
            "cp": 3,"ecp": 10,"chat_on": true,"fg": true,
			"d": "%s",
			"ct": "websocket","mqtt_sid": "","aid": 219994525426954,"st": [],"pm": [],"dc": "","no_auto_fg": true,"gas": null}`,
			c.config.UserID, c.sessionId, c.clientId)).
		SetProtocolVersion(3).
		SetHTTPHeaders(h).
		SetAutoReconnect(true).
		SetResumeSubs(true).
		SetCleanSession(true)

	connOpts.OnConnect = func(client mqtt.Client) {
		for _, topic := range []string{
			"/inbox",
			"/legacy_web",
			"/webrtc",
			"/br_sr",
			"/sr_res",
			"/t_ms",
			"/thread_typing",
			"/orca_typing_notifications",
			"/notify_disconnect",
			"/orca_presence",
			"/mercury",
			"/messaging_events",
			"/pp",
			//"/t_p",
			"/t_rtc",
			"/webrct_response",
		} {
			if token := client.Subscribe(topic, byte(0), c.mqttMessageHandler); token.Wait() && token.Error() != nil {
				c.log.Error(fmt.Sprintf("cannot subscribe on mqtt topic %s: %s\n", topic, token.Error()))
			}
		}

		token := client.Unsubscribe("/orca_message_notifications")
		token.Wait()

		client.Publish("/messenger_sync_create_queue", byte(0), false, fmt.Sprintf(`{
			"sync_api_version": 10,
			"max_deltas_able_to_process": 1000,
			"delta_batch_size": 500,
			"encoding": "JSON",
			"entity_fbid": "%s",
			"initial_titan_sequence_id": "%s",
			"device_params": null
		}`, c.config.UserID, c.lastSeqId))
	}

	mqtt.ERROR = log.New(os.Stdout, "", 0)

	client := mqtt.NewClient(connOpts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		c.log.Error(fmt.Sprintf("cannot mqtt connect: %s\n", token.Error()))
		return
	} else {
		c.log.App("mqtt connected")
	}
}

func (c *Client) mqttMessageHandler(client mqtt.Client, message mqtt.Message) {
	t := fmt.Sprintf("%s %s", message.Topic(), message.Payload())
	c.log.Raw(t)
	fmt.Printf("%s\n", t)

	switch message.Topic() {
	case "/orca_presence":
		obj := orcaPresence{}
		err := json.Unmarshal(message.Payload(), &obj)
		if err != nil {
			c.log.Error(fmt.Sprintf("error unmarshaling orca_presence: %s\n", err))
			break
		}
		// TODO emit event
		fmt.Printf("%+v\n", obj)

	case "/t_ms":
		if strings.Contains(string(message.Payload()), "firstDeltaSeqId") {
			obj := firstDeltaSeqId{}
			err := json.Unmarshal(message.Payload(), &obj)
			if err != nil {
				c.log.Error(fmt.Sprintf("error unmarshaling firstDeltaSeqId: %s\n", err))
				break
			}

			c.lastSeqId = strconv.FormatInt(obj.FirstDeltaSeqId, 10)
			c.log.App(fmt.Sprintf("got first seq id %s", c.lastSeqId))
			break
		}
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

	//TODO
	//for _, group := range c.Groups {
	//	for _, userId := range group.MercuryThread.Participants {
	//		if _, ok := c.friendNames[userId]; !ok {
	//			c.GetFriendData(userId) //ez a hatterben beallitja a nevet
	//		}
	//	}
	//}
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
	fmt.Println(c.lastSeqId)
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