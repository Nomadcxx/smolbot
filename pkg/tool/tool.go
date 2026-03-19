package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/Nomadcxx/nanobot-go/pkg/provider"
)

type ToolContext struct {
	SessionKey string
	Channel    string
	ChatID     string
}

type Result struct {
	Content  string
	Metadata map[string]any
}

type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any
	Execute(ctx context.Context, args json.RawMessage, tctx ToolContext) (*Result, error)
}

type Registry struct {
	mu            sync.RWMutex
	tools         map[string]Tool
	cancelSession func(sessionKey string)
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
}

func (r *Registry) SetCancelSession(fn func(sessionKey string)) {
	r.cancelSession = fn
}

func (r *Registry) Definitions() []provider.ToolDef {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]provider.ToolDef, 0, len(r.tools))
	for _, tool := range r.tools {
		defs = append(defs, provider.ToolDef{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  tool.Parameters(),
		})
	}
	return defs
}

func (r *Registry) Execute(ctx context.Context, name string, args json.RawMessage, tctx ToolContext) (*Result, error) {
	r.mu.RLock()
	tool, ok := r.tools[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("tool %q not found", name)
	}
	return tool.Execute(ctx, args, tctx)
}

func (r *Registry) CancelSession(sessionKey string) {
	if r.cancelSession != nil {
		r.cancelSession(sessionKey)
	}
}
