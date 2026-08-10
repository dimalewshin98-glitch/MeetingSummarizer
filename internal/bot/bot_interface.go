package bot

type BotInterface interface {
	Start()
	SendMessage(message BotResponseMessage) error
	RecieveMessage() (BotRequestMessage, error)
}

type BotRequestMessage struct {
	MessageID string
	UserID    int
	Text      string
	AudioFile []byte
	TextFile  []byte
	Date      int
}

type BotResponseMessage struct {
	MessageID string
	UserID    int
	Text      string
	Date      int
}
