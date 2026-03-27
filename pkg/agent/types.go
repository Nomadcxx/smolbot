package agent

type Request struct {
	Content       string
	SessionKey    string
	Channel       string
	ChatID        string
	Media         []MediaAttachment
	IsCronContext bool
	Model         string
	ReasoningEffort string
	MaxIterations int
	DisabledTools []string
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
	EventThinking          EventType = "thinking"
	EventProgress          EventType = "progress"
	EventToolHint          EventType = "tool.hint"
	EventToolStart         EventType = "tool.start"
	EventToolDone          EventType = "tool.done"
	EventAgentSpawned      EventType = "agent.spawned"
	EventAgentCompleted    EventType = "agent.completed"
	EventAgentWaitStarted  EventType = "agent.wait.started"
	EventAgentWaitCompleted EventType = "agent.wait.completed"
	EventDone              EventType = "done"
	EventError             EventType = "error"
	EventContextCompacting EventType = "context.compacting"
	EventContextCompressed EventType = "context.compressed"
	EventUsage             EventType = "usage"
)
