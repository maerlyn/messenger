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
		UserID  uint64 `json:"u"`
		Present uint   `json:"p"`
		C       uint64 `json:"c"`
	} `json:"list"`
}

type firstDeltaSeqId struct {
	FirstDeltaSeqId int64  `json:"firstDeltaSeqId"`
	QueueEntityId   int64  `json:"queueEntityId"`
	SyncToken       string `json:"syncToken"`
}
