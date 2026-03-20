package agent

type Request struct {
	Content       string
	SessionKey    string
	Channel       string
	ChatID        string
	Media         []MediaAttachment
	IsCronContext bool
}

type MediaAttachment struct {
	Data     []byte
	MimeType string
}

type EventCallback func(Event)

type Event struct {
	Type    EventType
	Content string
	Data    map[string]any
}

type EventType string

const (
	EventThinking  EventType = "thinking"
	EventProgress  EventType = "progress"
	EventToolHint  EventType = "tool.hint"
	EventToolStart EventType = "tool.start"
	EventToolDone  EventType = "tool.done"
	EventDone      EventType = "done"
	EventError     EventType = "error"
)
