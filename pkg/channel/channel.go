package channel

import "context"

type InboundMessage struct {
	Channel string
	ChatID  string
	Content string
}

type OutboundMessage struct {
	Channel string
	ChatID  string
	Content string
}

type Handler func(context.Context, InboundMessage)

type Status struct {
	State  string `json:"state"`
	Detail string `json:"detail,omitempty"`
}

type Channel interface {
	Name() string
	Start(ctx context.Context, handler Handler) error
	Stop(ctx context.Context) error
	Send(ctx context.Context, msg OutboundMessage) error
}

type StatusReporter interface {
	Status(ctx context.Context) (Status, error)
}

type LoginHandler interface {
	Login(ctx context.Context) error
}

type InteractiveLoginHandler interface {
	LoginWithUpdates(ctx context.Context, report func(Status) error) error
}
