package fb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testLogger struct{}

func (l *testLogger) App(msg string)   {}
func (l *testLogger) Error(msg string) { panic(msg) }
func (l *testLogger) Raw(msg string)   {}

func TestPresenceUnmarshal(t *testing.T) {
	rawJson := []byte(`{"list_type":"inc","list":[{"u":1,"p":2,"vc":74,"c":14166218},{"u":2,"p":2},{"u":3,"p":2,"vc":74,"c":14159962},{"u":4,"p":2,"vc":10,"c":9971722}]}`)
	channel := make(chan interface{}, 5)

	client := Client{log: &testLogger{}}
	client.SetEventChannel(channel)

	client.handleOrcaPresence(rawJson)

	assert.Len(t, channel, 1, "expected 1 item in channel, got %d", len(channel))

	obj := <-channel

	if assert.IsType(t, Presence{}, obj, "event on channel is not Presence") {
		p := obj.(Presence)

		assert.Equal(t, "inc", p.ListType, "Presence ListType != inc")
		assert.Len(t, p.List, 4, "expected presence list with 4 items, got %d", len(p.List))
	}
}

func TestFirstDeltaSeqId(t *testing.T) {
	rawJson := []byte(`{"firstDeltaSeqId":454890,"queueEntityId":1,"syncToken":"1"}`)
	client := Client{log: &testLogger{}}

	client.handleDeltaLikeMessage(rawJson)

	assert.Equal(t, "454890", client.lastSeqId, "last seq id expected to be 454890, got %s", client.lastSeqId)
}

func TestNew11Message(t *testing.T) {
	rawJson := []byte(`{"deltas":[{"attachments":[],"body":"bodybodybodybodybody","irisSeqId":"454887","irisTags":["DeltaNewMessage","is_from_iris_fanout"],"messageMetadata":{"actorFbId":"123456789","folderId":{"systemFolderId":"INBOX"},"messageId":"mid.foo","offlineThreadingId":"1","skipBumpThread":false,"skipSnippetUpdate":false,"tags":["source:chat:orca","app_id:256002347743983"],"threadKey":{"otherUserFbId":"1112222"},"threadReadStateEffect":"MARK_UNREAD","timestamp":"987654321"},"requestContext":{"apiArgs":{}},"class":"NewMessage"}],"firstDeltaSeqId":454886,"lastIssuedSeqId":454886,"queueEntityId":2}`)
	channel := make(chan interface{}, 5)

	client := Client{log: &testLogger{}}
	client.SetEventChannel(channel)

	client.handleDeltaLikeMessage(rawJson)

	assert.Equal(t, "454887", client.lastSeqId, "last seq id expected to be 454887, got %s", client.lastSeqId)

	if assert.Len(t, channel, 1) {
		obj := <-channel

		if assert.IsType(t, Message{}, obj, "object on channel expected to be Message, got %T", obj) {
			nm := obj.(Message)
			assert.Equal(t, "123456789", nm.ActorFbId, "new message actor expected 123456789, got %s", nm.ActorFbId)
			assert.Equal(t, "mid.foo", nm.MessageId, "new message messageId expected mid.foo, got %s", nm.MessageId)
			assert.Equal(t, "987654321", nm.Timestamp, "new message timestamp expected 987654321, got %s", nm.Timestamp)
			assert.Equal(t, "1112222", nm.Thread.OtherUserFbId, "new message otherUserFbId expected 112222, got %s", nm.Thread.OtherUserFbId)
			assert.Empty(t, nm.Thread.ThreadFbId, "new message threadFbId expected empty")
			assert.Empty(t, nm.Thread.Participants, "new message participants expected empty")
			assert.Len(t, nm.Body, 20, "new message body length expected 20, got %d", len(nm.Body))
			assert.False(t, nm.IsGroup(), "1v1 message is not group")
		}
	}
}

func TestNewThreadedMessage(t *testing.T) {
	rawJson := []byte(`{"deltas": [{"attachments": [],"body": "bodybodybody","irisSeqId": "454843","messageMetadata": {"actorFbId": "123456789","folderId": {"systemFolderId": "INBOX"},"messageId": "mid.bar","offlineThreadingId": "1","skipBumpThread": false,"tags": ["source:chat:web"],"threadKey": {"threadFbId": "123123"},"threadReadStateEffect": "MARK_UNREAD","timestamp": "1575379793963"},"participants": ["1","2","3"],"requestContext": {"apiArgs": {}},"tqSeqId": "8084","class": "NewMessage"}],"firstDeltaSeqId": 454843,"lastIssuedSeqId": 454843,"queueEntityId": 111}`)
	channel := make(chan interface{}, 5)

	client := Client{log: &testLogger{}}
	client.SetEventChannel(channel)

	client.handleDeltaLikeMessage(rawJson)

	assert.Equal(t, "454843", client.lastSeqId, "last seq id expected 454843, got %s", client.lastSeqId)

	if assert.Len(t, channel, 1) {
		obj := <-channel

		if assert.IsType(t, Message{}, obj, "object on channel expected to be Message, got %T", obj) {
			nm := obj.(Message)
			assert.Len(t, nm.Thread.Participants, 3, "threaded message expected 3 participants, got %d", len(nm.Thread.Participants))
			assert.True(t, nm.IsGroup(), "threaded message is group")
		}
	}
}

func TestThreadTyping(t *testing.T) {
	rawJson := []byte(`{"sender_fbid":1234,"state":1,"type":"typ","thread":"555"}`)
	channel := make(chan interface{}, 5)

	client := Client{log: &testLogger{}}
	client.SetEventChannel(channel)

	client.handleThreadTyping(rawJson)

	if assert.Len(t, channel, 1) {
		obj := <-channel

		if assert.IsType(t, ThreadTyping{}, obj, "object on channel expected to be ThreadTyping, got %T", obj) {
			tt := obj.(ThreadTyping)
			assert.Equal(t, uint64(1234), tt.SenderFbId)
			assert.Equal(t, 1, tt.State)
			assert.Equal(t, "555", tt.Thread)
		}
	}
}

func TestReadReceipt(t *testing.T) {
	rawJson := []byte(`{"deltas":[{"actionTimestampMs":"1575379819325","actorFbId":"111","irisSeqId":"454846","irisTags":["DeltaReadReceipt","is_from_iris_fanout"],"requestContext":{"apiArgs":{}},"threadKey":{"threadFbId":"321321"},"tqSeqId":"8087","watermarkTimestampMs":"1575379817947","class":"ReadReceipt"}],"firstDeltaSeqId":454846,"lastIssuedSeqId":454846,"queueEntityId":1}`)
	channel := make(chan interface{}, 5)

	client := Client{log: &testLogger{}}
	client.SetEventChannel(channel)

	client.handleDeltaLikeMessage(rawJson)
	assert.Equal(t, "454846", client.lastSeqId)

	if assert.Len(t, channel, 1) {
		obj := <-channel

		if assert.IsType(t, ReadReceipt{}, obj) {
			r := obj.(ReadReceipt)

			assert.Equal(t, int64(1575379819), r.Timestamp)
			assert.Equal(t, "111", r.ActorFbId)
			assert.Equal(t, "321321", r.Thread.ThreadFbId)
			assert.Equal(t, "", r.Thread.OtherUserFbId)
			assert.Equal(t, "1575379817947", r.WatermarkTimestampMs)
		}
	}
}

func TestLikeAttachment(t *testing.T) {
	rawJson := []byte(`{"deltas":[{"attachments":[{"mercury":{"sticker_attachment":{"id":"369239263222822","pack":{"id":"227877430692340"},"label":"Like, thumbs up","frame_count":1,"frame_rate":83,"frames_per_row":1,"frames_per_column":1,"sprite_image_2x":null,"sprite_image":null,"padded_sprite_image":null,"padded_sprite_image_2x":null,"url":"url","height":72,"width":72}}}],"irisSeqId":"454853","irisTags":["DeltaNewMessage","is_from_iris_fanout"],"messageMetadata":{"actorFbId":"1","folderId":{"systemFolderId":"INBOX"},"messageId":"mid.foo","offlineThreadingId":"2","skipBumpThread":false,"tags":["source:messenger:web"],"threadKey":{"threadFbId":"333"},"threadReadStateEffect":"MARK_UNREAD","timestamp":"5","unsendType":"deny_for_non_sender"},"participants":["1","2","3"],"requestContext":{"apiArgs":{}},"stickerId":"369239263222822","tqSeqId":"8104","class":"NewMessage"}],"firstDeltaSeqId":454853,"lastIssuedSeqId":454853,"queueEntityId":9}`)
	channel := make(chan interface{}, 5)

	client := Client{log: &testLogger{}}
	client.SetEventChannel(channel)

	client.handleDeltaLikeMessage(rawJson)
	assert.Equal(t, "454853", client.lastSeqId)

	if assert.Len(t, channel, 1) {
		obj := <-channel

		if assert.IsType(t, Message{}, obj) {
			m := obj.(Message)

			if assert.Len(t, m.Attachments, 1) {
				assert.True(t, m.Attachments[0].IsLike())
				assert.Equal(t, "url", m.Attachments[0].StickerAttachment.Url)
			}
		}
	}
}

func TestImageAttachment(t *testing.T) {
	rawJson := []byte(`{"deltas":[{"attachments":[{"fbid":"1","fileSize":"31610","filename":"7_n.jpg","genericMetadata":{"fbtype":"1586"},"hash":"2","haystackHandle":"02SAAAgW4Hy7wBADKOs1AGjqfmXQ::","id":"1","imageMetadata":{"height":960,"width":540},"mercury":{"blob_attachment":{"__typename":"MessageImage","attribution_app":null,"attribution_metadata":null,"filename":"image-4","preview":{"uri":"uri1","height":497,"width":280},"large_preview":{"uri":"uri2","height":853,"width":480},"thumbnail":{"uri":"uri3"},"photo_encodings":[],"legacy_attachment_id":"1","original_dimensions":{"x":540,"y":960},"original_extension":"jpg","render_as_sticker":false,"blurred_image_uri":null}},"mimeType":"image/jpeg","otherUserFbIds":["1","2","3"],"titanType":4,"useRefCounting":true}],"data":{"montage_supported_features":"[\"LIGHTWEIGHT_REPLY\"]"},"irisSeqId":"454871","irisTags":["DeltaNewMessage","is_from_iris_fanout"],"messageMetadata":{"actorFbId":"111","folderId":{"systemFolderId":"INBOX"},"messageId":"mid.foo","offlineThreadingId":"2","skipBumpThread":false,"tags":["source:chat:orca","app_id:256002347743983"],"threadKey":{"threadFbId":"3"},"threadReadStateEffect":"MARK_UNREAD","timestamp":"15757263429","unsendType":"deny_for_non_sender"},"participants":["1","2","3"],"requestContext":{"apiArgs":{}},"tqSeqId":"812","class":"NewMessage"}],"firstDeltaSeqId":454871,"lastIssuedSeqId":454871,"queueEntityId":7}`)
	channel := make(chan interface{}, 5)

	client := Client{log: &testLogger{}}
	client.SetEventChannel(channel)

	client.handleDeltaLikeMessage(rawJson)
	assert.Equal(t, "454871", client.lastSeqId)

	if assert.Len(t, channel, 1) {
		obj := <-channel

		if assert.IsType(t, Message{}, obj) {
			m := obj.(Message)

			if assert.Len(t, m.Attachments, 1) {
				a := m.Attachments[0]

				assert.False(t, a.IsLike())
				assert.Equal(t, "1", a.FbId)
				assert.Equal(t, "MessageImage", a.BlobAttachment.TypeName)
				assert.Equal(t, "uri2", a.BlobAttachment.LargePreview.Uri)
				assert.Equal(t, "jpg", a.BlobAttachment.OriginalExtension)
				assert.Equal(t, "image/jpeg", a.MimeType)
				assert.Len(t, a.OtherUserFbIds, 3)
			}

			assert.Equal(t, "111", m.ActorFbId)
			assert.Equal(t, "mid.foo", m.MessageId)
			assert.True(t, m.IsGroup())
		}
	}
}

func TestMessageRaction(t *testing.T) {
	rawJson := []byte(`{"deltas":[{"payload":[123,34,100,101,108,116,97,115,34,58,91,123,34,100,101,108,116,97,77,101,115,115,97,103,101,82,101,97,99,116,105,111,110,34,58,123,34,116,104,114,101,97,100,75,101,121,34,58,123,34,116,104,114,101,97,100,70,98,73,100,34,58,51,50,49,125,44,34,109,101,115,115,97,103,101,73,100,34,58,34,109,105,100,46,102,111,111,34,44,34,97,99,116,105,111,110,34,58,48,44,34,117,115,101,114,73,100,34,58,49,50,51,44,34,114,101,97,99,116,105,111,110,34,58,34,92,117,100,56,51,100,92,117,100,101,48,100,34,44,34,115,101,110,100,101,114,73,100,34,58,52,53,54,44,34,111,102,102,108,105,110,101,84,104,114,101,97,100,105,110,103,73,100,34,58,34,54,34,125,125,93,125],"class":"ClientPayload"}],"firstDeltaSeqId":454847,"lastIssuedSeqId":454847,"queueEntityId":5}`)
	channel := make(chan interface{}, 5)

	client := Client{log: &testLogger{}}
	client.SetEventChannel(channel)

	client.handleDeltaLikeMessage(rawJson)
	assert.Equal(t, "454847", client.lastSeqId)

	if assert.Len(t, channel, 1) {
		obj := <-channel

		if assert.IsType(t, MessageReaction{}, obj) {
			mr := obj.(MessageReaction)

			assert.Equal(t, "mid.foo", mr.MessageId)
			assert.Equal(t, "321", mr.Thread.ThreadFbId)
			assert.Equal(t, "123", mr.ActorFbId)
			assert.Equal(t, uint64(456), mr.SenderFbId)
			assert.Equal(t, "😍", mr.Reaction)
		}
	}
}

func TestMessageReply(t *testing.T) {
	rawJson := []byte(`{"deltas":[{"payload":[123,34,100,101,108,116,97,115,34,58,91,123,34,100,101,108,116,97,77,101,115,115,97,103,101,82,101,112,108,121,34,58,123,34,114,101,112,108,105,101,100,84,111,77,101,115,115,97,103,101,34,58,123,34,109,101,115,115,97,103,101,77,101,116,97,100,97,116,97,34,58,123,34,116,104,114,101,97,100,75,101,121,34,58,123,34,116,104,114,101,97,100,70,98,73,100,34,58,50,125,44,34,109,101,115,115,97,103,101,73,100,34,58,34,109,105,100,46,102,111,111,34,44,34,111,102,102,108,105,110,101,84,104,114,101,97,100,105,110,103,73,100,34,58,34,54,34,44,34,97,99,116,111,114,70,98,73,100,34,58,49,44,34,116,105,109,101,115,116,97,109,112,34,58,49,53,44,34,116,97,103,115,34,58,91,34,105,110,98,111,120,34,44,34,116,113,34,44,34,115,111,117,114,99,101,58,109,101,115,115,101,110,103,101,114,58,119,101,98,34,93,125,44,34,98,111,100,121,34,58,34,98,111,100,121,98,111,100,121,34,44,34,97,116,116,97,99,104,109,101,110,116,115,34,58,91,93,44,34,114,101,113,117,101,115,116,67,111,110,116,101,120,116,34,58,123,34,97,112,105,65,114,103,115,34,58,34,34,125,44,34,105,114,105,115,84,97,103,115,34,58,91,93,125,44,34,109,101,115,115,97,103,101,34,58,123,34,109,101,115,115,97,103,101,77,101,116,97,100,97,116,97,34,58,123,34,116,104,114,101,97,100,75,101,121,34,58,123,34,116,104,114,101,97,100,70,98,73,100,34,58,50,125,44,34,109,101,115,115,97,103,101,73,100,34,58,34,109,105,100,46,98,97,114,34,44,34,111,102,102,108,105,110,101,84,104,114,101,97,100,105,110,103,73,100,34,58,34,54,54,48,56,54,34,44,34,97,99,116,111,114,70,98,73,100,34,58,53,50,44,34,116,105,109,101,115,116,97,109,112,34,58,49,53,55,44,34,116,97,103,115,34,58,91,34,115,111,117,114,99,101,58,99,104,97,116,58,111,114,99,97,34,44,34,97,112,112,95,105,100,58,50,53,54,48,48,50,51,52,55,55,52,51,57,56,51,34,44,34,109,113,116,116,46,45,51,34,44,34,116,113,34,44,34,99,103,45,101,110,97,98,108,101,100,34,44,34,105,110,98,111,120,34,93,44,34,116,104,114,101,97,100,82,101,97,100,83,116,97,116,101,69,102,102,101,99,116,34,58,50,44,34,115,107,105,112,66,117,109,112,84,104,114,101,97,100,34,58,102,97,108,115,101,44,34,117,110,115,101,110,100,84,121,112,101,34,58,34,100,101,110,121,95,102,111,114,95,110,111,110,95,115,101,110,100,101,114,34,44,34,102,111,108,100,101,114,73,100,34,58,123,34,115,121,115,116,101,109,70,111,108,100,101,114,73,100,34,58,48,125,125,44,34,98,111,100,121,34,58,34,79,100,97,34,44,34,97,116,116,97,99,104,109,101,110,116,115,34,58,91,93,44,34,105,114,105,115,83,101,113,73,100,34,58,52,53,52,57,48,53,44,34,116,113,83,101,113,73,100,34,58,56,50,49,54,44,34,109,101,115,115,97,103,101,82,101,112,108,121,34,58,123,34,114,101,112,108,121,84,111,77,101,115,115,97,103,101,73,100,34,58,123,34,105,100,34,58,34,109,105,100,46,102,111,111,34,125,44,34,115,116,97,116,117,115,34,58,48,125,44,34,114,101,113,117,101,115,116,67,111,110,116,101,120,116,34,58,123,34,97,112,105,65,114,103,115,34,58,34,34,125,44,34,112,97,114,116,105,99,105,112,97,110,116,115,34,58,91,53,44,49,44,49,48,93,44,34,105,114,105,115,84,97,103,115,34,58,91,34,68,101,108,116,97,78,101,119,77,101,115,115,97,103,101,34,44,34,105,115,95,102,114,111,109,95,105,114,105,115,95,102,97,110,111,117,116,34,93,125,44,34,115,116,97,116,117,115,34,58,48,125,125,93,125],"class":"ClientPayload"}],"firstDeltaSeqId":454905,"lastIssuedSeqId":454905,"queueEntityId":561598959}`)
	channel := make(chan interface{}, 5)

	client := Client{log: &testLogger{}}
	client.SetEventChannel(channel)

	client.handleDeltaLikeMessage(rawJson)
	assert.Equal(t, "454905", client.lastSeqId)

	if assert.Len(t, channel, 1) {
		obj := <-channel

		if assert.IsType(t, MessageReply{}, obj) {
			mr := obj.(MessageReply)

			assert.Equal(t, "2", mr.RepliedToMessage.Thread.ThreadFbId)
			assert.Equal(t, "mid.foo", mr.RepliedToMessage.MessageId)
			assert.Equal(t, uint64(1), mr.RepliedToMessage.ActorFbId)
			assert.Equal(t, uint64(15), mr.RepliedToMessage.Timestamp)
			assert.Equal(t, "bodybody", mr.RepliedToMessage.Body)

			assert.Equal(t, "2", mr.Message.Thread.ThreadFbId)
			assert.Equal(t, "mid.bar", mr.Message.MessageId)
			assert.Equal(t, uint64(52), mr.Message.ActorFbId)
			assert.Equal(t, uint64(157), mr.Message.Timestamp)
			assert.Equal(t, "Oda", mr.Message.Body)
		}
	}
}

func TestTyping(t *testing.T) {
	rawJson := []byte(`{"type":"typ","sender_fbid":123456,"state":1}`)
	channel := make(chan interface{}, 5)

	client := Client{log: &testLogger{}}
	client.SetEventChannel(channel)

	client.handleTyping(rawJson)

	if assert.Len(t, channel, 1) {
		obj := <-channel

		if assert.IsType(t, Typing{}, obj) {
			typ := obj.(Typing)

			assert.Equal(t, "typ", typ.Type)
			assert.Equal(t, "123456", typ.SenderFbId)
			assert.Equal(t, 1, typ.State)
		}
	}
}
