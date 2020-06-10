package fb

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func (c *Client) Listen() {
	h := http.Header{}
	h.Add("Cookie", c.config.Cookie)
	h.Add("User-Agent", c.config.UserAgent)
	h.Add("Accept", "*/*")
	h.Add("Referer", "https://www.facebook.com")
	h.Add("Host", "edge-chat.facebook.com")
	h.Add("Origin", "https:/www.facebook.com")

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
		SetCleanSession(true).
		SetKeepAlive(10 * time.Second)

	connOpts.OnConnect = func(client mqtt.Client) {
		for _, topic := range []string{
			"/t_ms",                      // stuff that happens in chats
			"/thread_typing",             // group typing notifs
			"/orca_typing_notifications", // 1v1 typing notifs
			"/orca_presence",             // who is active and when
			"/legacy_web",                // not chat-related stuff

			"/inbox",
			"/webrtc",
			"/br_sr",
			"/sr_res",
			"/notify_disconnect",
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

		if c.syncToken == "" {
			c.createMessengerQueue(client)
		} else {
			c.log.App("getting messenger diffs after reconnect")
			token := client.Publish("/messenger_sync_get_diffs", byte(0), false, fmt.Sprintf(`{
				"sync_api_version": 10,
				"max_deltas_able_to_process": 1000,
				"delta_batch_size": 50,
				"encoding": "JSON",
				"last_seq_id": "%s",
				"sync_token": "%s"
			}`, c.lastSeqId, c.syncToken))

			token.Wait()
			if token.Error() != nil {
				c.log.Error(fmt.Sprintf("cannot publish messenger_sync_get_diffs: %s", token.Error()))
			}
		}
	}

	mqtt.ERROR = logMQTTToApp{appLogger: c.log}

	client := mqtt.NewClient(connOpts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		c.log.Error(fmt.Sprintf("cannot mqtt connect: %s\n", token.Error()))
		return
	} else {
		c.log.App("mqtt connected")
	}

	c.mqttClient = client
}

func (c *Client) createMessengerQueue(client mqtt.Client) {
	if err := c.fetchLastSeqId(); err != nil {
		c.log.Error(fmt.Sprintf("error fetching last seq id: %s", err))

		go func() {
			time.Sleep(5 * time.Second)
			c.createMessengerQueue(client)
		}()

		return
	}

	c.log.App("creating messenger queue")
	token := client.Publish("/messenger_sync_create_queue", byte(0), false, fmt.Sprintf(`{
				"sync_api_version": 10,
				"max_deltas_able_to_process": 1000,
				"delta_batch_size": 500,
				"encoding": "JSON",
				"entity_fbid": "%s",
				"initial_titan_sequence_id": "%s",
				"device_params": null
			}`, c.config.UserID, c.lastSeqId))

	token.Wait()
	if token.Error() != nil {
		c.log.Error(fmt.Sprintf("cannot publish messenger_sync_create_queue: %s", token.Error()))
	}
}

func (c *Client) mqttMessageHandler(_ mqtt.Client, message mqtt.Message) {
	t := fmt.Sprintf("%s %s", message.Topic(), message.Payload())
	c.log.Raw(t)

	switch message.Topic() {
	case "/orca_presence":
		c.handleOrcaPresence(message.Payload())

	case "/t_ms":
		c.handleDeltaLikeMessage(message.Payload())

	case "/thread_typing":
		c.handleThreadTyping(message.Payload())

	case "/orca_typing_notifications":
		c.handleTyping(message.Payload())
	}
}

func (c *Client) handleOrcaPresence(message []byte) {
	obj := orcaPresence{}
	err := json.Unmarshal(message, &obj)
	if err != nil {
		c.log.Error(fmt.Sprintf("error unmarshaling orca_presence: %s\n", err))
		return
	}

	p := Presence{ListType: obj.ListType}

	c.lastActiveTimesMutex.Lock()
	c.lastActiveTimesMutex.Unlock()
	for _, v := range obj.List {
		uid := strconv.Itoa(int(v.UserID))
		p.List = append(p.List, presenceItem{UserID: uid, Present: v.Present, C: v.C})

		if c.lastActiveTimes[uid] < v.L {
			c.lastActiveTimes[uid] = v.L
		}
	}

	c.emit(p)
}

func (c *Client) handleDeltaLikeMessage(message []byte) {
	if strings.Contains(string(message), "firstDeltaSeqId") || strings.Contains(string(message), "lastIssuedSeqId") {
		obj := deltaSeqIds{}
		err := json.Unmarshal(message, &obj)
		if err != nil {
			c.log.Error(fmt.Sprintf("error unmarshaling deltaSeqIds: %s\n", err))
			return
		}

		if obj.LastIssuedSeqId == 0 {
			c.lastSeqId = strconv.FormatInt(obj.FirstDeltaSeqId, 10)
			c.syncToken = obj.SyncToken
			c.log.App(fmt.Sprintf("got first seq id %s, sync token %s", c.lastSeqId, c.syncToken))
			return
		} else {
			c.lastSeqId = strconv.FormatInt(obj.LastIssuedSeqId, 10)
			c.log.App(fmt.Sprintf("got last issued seq id %s", c.lastSeqId))
			// no return
		}
	}

	if strings.Contains(string(message), "errorCode") {
		ec := errorCode{}
		err := json.Unmarshal(message, &ec)
		if err != nil {
			c.log.Error(fmt.Sprintf("error unmarshaling errorCode: %s", err))
			return
		}
		c.log.Error(fmt.Sprintf("got errorCode %s", ec.ErrorCode))

		if ec.ErrorCode == "ERROR_QUEUE_NOT_FOUND" || ec.ErrorCode == "ERROR_QUEUE_OVERFLOW" {
			c.log.App("re-creating messenger queue")
			c.syncToken = ""
			c.createMessengerQueue(c.mqttClient)
			return
		}
	}

	if strings.Contains(string(message), "deltas") {
		dm := deltaWrapper{}
		err := json.Unmarshal(message, &dm)
		if err != nil {
			c.log.Error(fmt.Sprintf("error unmarshaling delta message: %s\n", err))
			return
		}

		for _, delta := range dm.Deltas {
			if delta.IrisSeqId > c.lastSeqId {
				c.lastSeqId = delta.IrisSeqId
			}

			if delta.isClientPayload() {
				decoded := delta.decodeClientPayload()
				c.log.Raw(fmt.Sprintf("decoded payload: %s", decoded))

				c.handleClientPayload(decoded)
			}

			if delta.isNewMessage() {
				nm := Message{}
				nm.fromFBType(delta)

				c.emit(nm)
			}

			if delta.isReadReceipt() {
				rr := ReadReceipt{}
				rr.fromFBType(delta)

				c.emit(rr)
			}

			if delta.isMarkRead() {
				mr := MarkRead{}
				mr.fromFBType(delta)

				c.emit(mr)
			}
		}
	}
}

func (c *Client) handleThreadTyping(message []byte) {
	obj := threadTyping{}
	err := json.Unmarshal(message, &obj)
	if err != nil {
		c.log.Error(fmt.Sprintf("error unmarshaling thread_typing: %s\n", err))
		return
	}

	t := ThreadTyping{
		SenderFbId: obj.SenderFbId,
		State:      obj.State,
		Thread:     obj.Thread,
	}

	c.emit(t)
}

func (c *Client) handleTyping(message []byte) {
	obj := typingNotification{}
	err := json.Unmarshal(message, &obj)
	if err != nil {
		c.log.Error(fmt.Sprintf("error unmarshaling typing: %s\n", err))
		return
	}

	t := Typing{}
	t.fromFBType(obj)

	c.emit(t)
}

func (c *Client) handleClientPayload(payload []byte) {
	obj := clientPayload{}
	err := json.Unmarshal(payload, &obj)
	if err != nil {
		c.log.Error(fmt.Sprintf("error unmarshaling client payload: %s\n", err))
		return
	}

	for _, v := range obj.Deltas {
		if v.Reaction.MessageId != "" { //simple check to see if has reaction
			mr := MessageReaction{}
			mr.fromFBType(v.Reaction)

			c.emit(mr)
		}

		if v.Reply.Message.MessageMetadata.MessageId != "" {
			s := strconv.Itoa(v.Reply.Message.IrisSeqId)
			if s > c.lastSeqId {
				c.lastSeqId = s
			}

			mr := MessageReply{}
			mr.fromFBType(v.Reply)

			c.emit(mr)
		}
	}
}

func (c *Client) StopListening() {
	token := c.mqttClient.Publish("/browser_close", byte(1), false, "{}")
	token.Wait()

	c.mqttClient.Disconnect(1000)
}
