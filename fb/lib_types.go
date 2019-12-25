package fb

import (
	"fmt"
	"strconv"
	"time"
)

const (
	DateFormatYmdHis = "2006-01-02 15:04:05"
)

type presenceItem struct {
	UserID  uint64
	Present uint8
}
type Presence struct {
	ListType string
	List     []presenceItem
}

type thread struct {
	OtherUserFbId string
	ThreadFbId    string

	Participants []string
}

func (t *thread) fromFBType(orig threadKey) {
	t.OtherUserFbId = orig.OtherUserFbId
	t.ThreadFbId = orig.ThreadFbId
}

func (t *thread) UniqueId() string {
	if t.ThreadFbId != "" {
		return t.ThreadFbId
	}

	return t.OtherUserFbId
}

type MessageAttachment struct {
	FbId string

	ImageMetadata struct {
		Height, Width int
	}

	StickerAttachment struct {
		Id    string
		Label string
		Url   string
	}

	BlobAttachment struct {
		TypeName string
		FileName string

		LargePreview struct {
			Uri           string
			Width, Height int
		}

		LegacyAttachmentId string
		OriginalExtension  string
	}

	MimeType       string
	OtherUserFbIds []string
}

func (a *MessageAttachment) fromFBType(orig deltaAttachment) {
	a.FbId = orig.FbId

	a.ImageMetadata.Height = orig.ImageMetadata.Height
	a.ImageMetadata.Width = orig.ImageMetadata.Width

	a.StickerAttachment.Id = orig.Mercury.StickerAttachment.Id
	a.StickerAttachment.Label = orig.Mercury.StickerAttachment.Label
	a.StickerAttachment.Url = orig.Mercury.StickerAttachment.Url

	a.BlobAttachment.TypeName = orig.Mercury.BlobAttachment.TypeName
	a.BlobAttachment.FileName = orig.Mercury.BlobAttachment.FileName
	a.BlobAttachment.LargePreview.Uri = orig.Mercury.BlobAttachment.LargePreview.Uri
	a.BlobAttachment.LargePreview.Height = orig.Mercury.BlobAttachment.LargePreview.Height
	a.BlobAttachment.LargePreview.Width = orig.Mercury.BlobAttachment.LargePreview.Width
	a.BlobAttachment.LegacyAttachmentId = orig.Mercury.BlobAttachment.LegacyAttachmentId
	a.BlobAttachment.OriginalExtension = orig.Mercury.BlobAttachment.OriginalExtension

	a.MimeType = orig.MimeType
	a.OtherUserFbIds = orig.OtherUserFbIds
}

func (a *MessageAttachment) IsLike() bool {
	return a.StickerAttachment.Label == "Like, thumbs up"
}

type Message struct {
	Attachments []MessageAttachment

	ActorFbId string
	MessageId string
	Timestamp string

	Thread thread

	Body string

	Reactions []MessageReaction
	Replies   []MessageReply
}

func (n *Message) fromFBType(orig delta) {
	n.ActorFbId = orig.MessageMetadata.ActorFbId
	n.MessageId = orig.MessageMetadata.MessageId
	n.Timestamp = orig.MessageMetadata.Timestamp
	n.Body = orig.Body

	n.Thread.fromFBType(orig.MessageMetadata.ThreadKey)
	n.Thread.Participants = orig.Participants

	for _, v := range orig.Attachments {
		a := MessageAttachment{}
		a.fromFBType(v)

		n.Attachments = append(n.Attachments, a)
	}
}

func (n *Message) fromLastMessage(orig lastMessage) {
	n.ActorFbId = orig.MessageSender.Id
	n.MessageId = orig.MessageId

	ti, _ := strconv.Atoi(orig.TimestampPrecise)
	n.Timestamp = strconv.Itoa(ti / 1000)

	n.Body = orig.Message.Text

	for _, v := range orig.MessageReactions {
		r := MessageReaction{
			Reaction:  v.Reaction,
			Action:    0,
			ActorFbId: v.User.Id,
		}

		n.Reactions = append(n.Reactions, r)
	}
}

func (n *Message) String(fbc *Client) string {
	dateInt, _ := strconv.Atoi(n.Timestamp)
	date := time.Unix(int64(dateInt), 0)

	name := fbc.FriendName(n.ActorFbId)

	text := fmt.Sprintf("[%s] %s: %s",
		date.Format(DateFormatYmdHis),
		name,
		n.Body)

	for _, r := range n.Reactions {
		text = fmt.Sprintf("%s\n    %s", text, r.String(fbc))
	}

	//TODO
	//if len(d.Delta.Attachments) > 0 {
	//	for _, v := range d.Delta.Attachments {
	//		if v.Mercury.BlobAttachment.LargePreview.Uri != "" {
	//			text = fmt.Sprintf("%s\n    %s", text, v.Mercury.BlobAttachment.LargePreview.Uri)
	//		}
	//
	//		if v.Mercury.BlobAttachment.AnimatedImage.Uri != "" {
	//			text = fmt.Sprintf("%s\n    %s", text, v.Mercury.BlobAttachment.AnimatedImage.Uri)
	//		}
	//
	//		if v.Mercury.StickerAttachment.Url != "" {
	//			if v.Mercury.StickerAttachment.Label == "Like, thumbs up" {
	//				text = fmt.Sprintf("%s\n    [%s]", text, v.Mercury.StickerAttachment.Label)
	//			} else {
	//				text = fmt.Sprintf("%s\n    [%s] %s", text, v.Mercury.StickerAttachment.Label, v.Mercury.StickerAttachment.Url)
	//			}
	//		}
	//	}
	//}

	return text
}

func (n *Message) IsGroup() bool {
	return n.Thread.ThreadFbId != ""
}

type ThreadTyping struct {
	SenderFbId uint64
	State      int
	Thread     string
}

type ReadReceipt struct {
	Timestamp            int64
	ActorFbId            string
	Thread               thread
	WatermarkTimestampMs string
}

func (r *ReadReceipt) fromFBType(orig delta) {
	ts, _ := strconv.Atoi(orig.ActionTimestampMs)

	r.Timestamp = int64(ts / 1000)
	r.ActorFbId = orig.ActorFbId
	r.Thread.fromFBType(orig.ThreadKey)
	r.WatermarkTimestampMs = orig.WatermarkTimestampMs
}

func (r *ReadReceipt) IsGroup() bool {
	return r.Thread.ThreadFbId != ""
}

func (r *ReadReceipt) String(fbc *Client) string {
	name := fbc.FriendName(r.ActorFbId)
	date := time.Unix(r.Timestamp, 0)

	return fmt.Sprintf("[%s] %s read", date.Format(DateFormatYmdHis), name)
}

type MessageReaction struct {
	Thread     thread
	MessageId  string
	Action     int
	ActorFbId  string
	Reaction   string
	SenderFbId uint64
}

func (r *MessageReaction) fromFBType(orig deltaMessageReaction) {
	r.Thread.OtherUserFbId = strconv.FormatInt(orig.ThreadKey.OtherUserFbId, 10)
	r.Thread.ThreadFbId = strconv.FormatInt(orig.ThreadKey.ThreadFbId, 10)

	r.MessageId = orig.MessageId
	r.Action = orig.Action
	r.ActorFbId = strconv.FormatInt(orig.UserId, 10)
	r.Reaction = orig.Reaction
	r.SenderFbId = orig.SenderId
}

func (r *MessageReaction) String(fbc *Client) string {
	name := fbc.FriendName(r.ActorFbId)

	return fmt.Sprintf("%s: %s", name, r.Reaction)
}

type MessageReply struct {
	RepliedToMessage struct {
		Thread    thread
		MessageId string
		ActorFbId uint64
		Timestamp uint64
		Body      string
	}

	Message struct {
		Thread    thread
		MessageId string
		ActorFbId uint64
		Timestamp uint64
		Body      string
	}
}

func (r *MessageReply) fromFBType(orig deltaMessageReply) {
	r.RepliedToMessage.Thread.OtherUserFbId = strconv.FormatInt(orig.RepliedToMessage.MessageMetadata.ThreadKey.OtherUserFbId, 10)
	r.RepliedToMessage.Thread.ThreadFbId = strconv.FormatInt(orig.RepliedToMessage.MessageMetadata.ThreadKey.ThreadFbId, 10)
	r.RepliedToMessage.MessageId = orig.RepliedToMessage.MessageMetadata.MessageId
	r.RepliedToMessage.ActorFbId = orig.RepliedToMessage.MessageMetadata.ActorFbId
	r.RepliedToMessage.Timestamp = orig.RepliedToMessage.MessageMetadata.Timestamp
	r.RepliedToMessage.Body = orig.RepliedToMessage.Body

	r.Message.Thread.OtherUserFbId = strconv.FormatInt(orig.Message.MessageMetadata.ThreadKey.OtherUserFbId, 10)
	r.Message.Thread.ThreadFbId = strconv.FormatInt(orig.Message.MessageMetadata.ThreadKey.ThreadFbId, 10)
	r.Message.MessageId = orig.Message.MessageMetadata.MessageId
	r.Message.ActorFbId = orig.Message.MessageMetadata.ActorFbId
	r.Message.Timestamp = orig.Message.MessageMetadata.Timestamp
	r.Message.Body = orig.Message.Body
}

type Typing struct {
	Type       string
	SenderFbId string
	State      int
}

func (t *Typing) fromFBType(orig typingNotification) {
	t.Type = orig.Type
	t.SenderFbId = strconv.Itoa(orig.SenderFbId)
	t.State = orig.State
}

func (t *Typing) String(fbc *Client) string {
	name := fbc.FriendName(t.SenderFbId)

	return fmt.Sprintf("%s is typing, state: %d", name, t.State)
}
