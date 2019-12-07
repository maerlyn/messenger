package fb

import (
	"strconv"
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

func (n *Message) IsGroup() bool {
	return n.Thread.ThreadFbId != ""
}

type ThreadTyping struct {
	SenderFbId uint64
	State      int
	Thread     string
}

type ReadReceipt struct {
	TimestampMs          string
	ActorFbId            string
	Thread               thread
	WatermarkTimestampMs string
}

func (r *ReadReceipt) fromFBType(orig delta) {
	r.TimestampMs = orig.ActionTimestampMs
	r.ActorFbId = orig.ActorFbId
	r.Thread.fromFBType(orig.ThreadKey)
	r.WatermarkTimestampMs = orig.WatermarkTimestampMs
}

func (r *ReadReceipt) IsGroup() bool {
	return r.Thread.ThreadFbId != ""
}

type MessageReaction struct {
	Thread     thread
	MessageId  string
	Action     int
	ActorFbId  uint64
	Reaction   string
	SenderFbId uint64
}

func (r *MessageReaction) fromFBType(orig deltaMessageReaction) {
	r.Thread.OtherUserFbId = strconv.FormatInt(orig.ThreadKey.OtherUserFbId, 10)
	r.Thread.ThreadFbId = strconv.FormatInt(orig.ThreadKey.ThreadFbId, 10)

	r.MessageId = orig.MessageId
	r.Action = orig.Action
	r.ActorFbId = orig.UserId
	r.Reaction = orig.Reaction
	r.SenderFbId = orig.SenderId
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
