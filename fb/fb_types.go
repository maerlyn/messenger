package fb

type friendlistItem struct {
	Id                          string `json:"id"`
	Name                        string `json:"name"`
	FirstName                   string `json:"firstName"`
	Vanity                      string `json:"vanity"`
	ThumbSrc                    string `json:"thumbSrc"`
	Uri                         string `json:"uri"`
	Type                        string `json:"type"`
	IsFriend                    bool   `json:"is_friend"`
	IsNonfriendMessengerContact bool   `json:"is_nonfriend_messenger_contact"`
}
type friendList map[string]friendlistItem

type lastActiveTimes map[string]int64

type messengerGroup struct {
	Uid           string `json:"uid"`
	MercuryThread struct {
		Participants []string `json:"participants"`
		ImageSrc     string   `json:"image_src"`
		Name         string   `json:"name"`
	} `json:"mercury_thread"`
	ParticipantsToRender []struct {
		Id        uint64 `json:"id"`
		ImageSrc  string `json:"image_src"`
		Name      string `json:"name"`
		ShortName string `json:"short_name"`
	} `json:"participants_to_render"`
}

type lastSeqIdResponse struct {
	O0 struct {
		Data struct {
			Viewer struct {
				MessageThreads struct {
					SyncSequenceId string `json:"sync_sequence_id"`
				} `json:"message_threads"`
			} `json:"viewer"`
		} `json:"data"`
	} `json:"o0"`
}

type orcaPresence struct {
	ListType string `json:"list_type"`
	List     []struct {
		UserID  int64  `json:"u"`
		Present uint8  `json:"p"`
		C       int64  `json:"c"`
	} `json:"list"`
}

type deltaSeqIds struct {
	FirstDeltaSeqId int64  `json:"firstDeltaSeqId"`
	LastIssuedSeqId int64  `json:"lastIssuedSeqId"`
	QueueEntityId   int64  `json:"queueEntityId"`
	SyncToken       string `json:"syncToken"`
}

type threadKey struct {
	OtherUserFbId string `json:"otherUserFbId"`
	ThreadFbId    string `json:"threadFbId"`
}

type threadKeyInt struct {
	OtherUserFbId int64 `json:"otherUserFbId"`
	ThreadFbId    int64 `json:"threadFbId"`
}

type deltaAttachment struct {
	FbId            string `json:"fbid"`
	FileSize        string `json:"fileSize"`
	FileName        string `json:"filename"`
	GenericMetadata struct {
		FbType string `json:"fbtype"`
	} `json:"genericMetadata"`
	Hash           string `json:"hash"`
	HaystackHandle string `json:"haystackHandle"`
	Id             string `json:"id"`
	ImageMetadata  struct {
		Height int `json:"height"`
		Width  int `json:"width"`
	} `json:"imageMetadata"`

	Mercury struct {
		StickerAttachment struct {
			Id   string `json:"id"`
			Pack struct {
				Id string `json:"id"`
			} `json:"pack"`
			Label        string `json:"label"`
			FrameCount   int    `json:"frame_count"`
			FrameRate    int    `json:"frame_rate"`
			FramesPerRow int    `json:"frames_per_row"`
			Url          string `json:"url"`
			Height       int    `json:"height"`
			Width        int    `json:"width"`
		} `json:"sticker_attachment"`

		BlobAttachment struct {
			TypeName string `json:"__typename"`
			FileName string `json:"filename"`
			Preview  struct {
				Uri    string `json:"uri"`
				Height int    `json:"height"`
				Width  int    `json:"width"`
			} `json:"preview"`
			LargePreview struct {
				Uri    string `json:"uri"`
				Height int    `json:"height"`
				Width  int    `json:"width"`
			} `json:"large_preview"`
			Thumbnail struct {
				Uri string `json:"uri"`
			} `json:"thumbnail"`
			LegacyAttachmentId string `json:"legacy_attachment_id"`
			OriginalDimensions struct {
				X int `json:"x"`
				Y int `json:"y"`
			} `json:"original_dimensions"`
			OriginalExtension string `json:"original_extension"`
			RenderAsSticker   bool   `json:"render_as_sticker"`
		} `json:"blob_attachment"`
	} `json:"mercury"`

	MimeType       string   `json:"mimeType"`
	OtherUserFbIds []string `json:"otherUserFbIds"`
	TitanType      int      `json:"titanType"`
	UseRefCounting bool     `json:"useRefCounting"`
}

type delta struct {
	Class string `json:"class"`

	Payload []uint8 `json:"payload"`

	Body      string `json:"body"`
	IrisSeqId string `json:"irisSeqId"`

	MessageMetadata struct {
		ActorFbId string    `json:"actorFbId"`
		MessageId string    `json:"messageId"`
		ThreadKey threadKey `json:"threadKey"`
		Timestamp string    `json:"timestamp"`
	} `json:"messageMetadata"`

	Participants []string `json:"participants"`

	ActionTimestamp      string      `json:"actionTimestamp"`
	ActionTimestampMs    string      `json:"actionTimestampMs"`
	ActorFbId            string      `json:"actorFbId"`
	ThreadKey            threadKey   `json:"threadKey"`
	ThreadKeys           []threadKey `json:"threadKeys"`
	WatermarkTimestamp   string      `json:"watermarkTimestamp"`
	WatermarkTimestampMs string      `json:"watermarkTimestampMs"`

	Attachments []deltaAttachment `json:"attachments"`
}

func (d *delta) isClientPayload() bool {
	return d.Class == "ClientPayload"
}

func (d *delta) isNewMessage() bool {
	return d.Class == "NewMessage"
}

func (d *delta) isReadReceipt() bool {
	return d.Class == "ReadReceipt"
}

func (d *delta) isMarkRead() bool {
	return d.Class == "MarkRead"
}

func (d *delta) decodeClientPayload() []byte {
	if !d.isClientPayload() {
		return []byte{}
	}

	ret := ""
	for _, v := range d.Payload {
		ret = ret + string(v)
	}

	return []byte(ret)
}

type deltaWrapper struct {
	Deltas []delta `json:"deltas"`
}

type threadTyping struct {
	SenderFbId uint64 `json:"sender_fbid"`
	State      int    `json:"state"`
	Type       string `json:"type"`
	Thread     string `json:"thread"`
}

type deltaMessageReaction struct {
	ThreadKey          threadKeyInt `json:"threadKey"`
	MessageId          string       `json:"messageId"`
	Action             int          `json:"action"` // 0 = add, 1 = remove
	UserId             int64        `json:"userId"` // actor
	Reaction           string       `json:"reaction"`
	SenderId           uint64       `json:"senderId"` // original sender
	OfflineThreadingId string       `json:"offlineThreadingId"`
}

type deltaMessageReply struct {
	RepliedToMessage struct {
		MessageMetadata struct {
			ThreadKey threadKeyInt `json:"threadKey"`
			MessageId string       `json:"messageId"`
			ActorFbId int64        `json:"actorFbId"`
			Timestamp uint64       `json:"timestamp"`
		} `json:"messageMetadata"`

		Body string `json:"body"`
	} `json:"repliedToMessage"`

	Message struct {
		MessageMetadata struct {
			ThreadKey threadKeyInt `json:"threadKey"`
			MessageId string       `json:"messageId"`
			ActorFbId int64        `json:"actorFbId"`
			Timestamp uint64       `json:"timestamp"`
		} `json:"messageMetadata"`

		Body      string `json:"body"`
		IrisSeqId int    `json:"irisSeqId"`

		MessageReply struct {
			ReplyToMessageId struct {
				Id string `json:"id"`
			} `json:"replyToMessageId"`
		} `json:"messageReply"`
	} `json:"message"`
}

type clientPayload struct {
	Deltas []struct {
		Reaction deltaMessageReaction `json:"deltaMessageReaction"`
		Reply    deltaMessageReply    `json:"deltaMessageReply"`
	} `json:"deltas"`
}

type friendData struct {
	O0 struct {
		Data struct {
			MessagingActors []struct {
				Id          string `json:"id"`
				Name        string `json:"name"`
				BigImageSrc struct {
					Uri string `json:"uri"`
				} `json:"big_image_src"`
			} `json:"messaging_actors"`
		} `json:"data"`
	} `json:"o0"`
}

type lastMessagesResponse struct {
	O0 struct {
		Data struct {
			MessageThread struct {
				ThreadKey threadKey `json:"thread_key"`
				Messages  struct {
					Nodes []lastMessage `json:"nodes"`
				} `json:"messages"`
			} `json:"message_thread"`
		} `json:"data"`
	} `json:"o0"`
}

type lastMessage struct {
	MessageId     string `json:"message_id"`
	MessageSender struct {
		Id string `json:"id"`
	} `json:"message_sender"`
	Message struct {
		Text string `json:"text"`
	} `json:"message"`
	BlobAttachments []struct {
		LargePreview struct {
			Uri string `json:"uri"`
		} `json:"large_preview"`
	} `json:"blob_attachments"`
	TimestampPrecise string `json:"timestamp_precise"`
	MessageReactions []struct {
		Reaction string `json:"reaction"`
		User     struct {
			Id string `json:"id"`
		} `json:"user"`
	} `json:"message_reactions"`
}

type typingNotification struct {
	Type       string
	State      int
	SenderFbId int `json:"sender_fbid"`
}

type errorCode struct {
	ErrorCode string `json:"errorCode"`
}
