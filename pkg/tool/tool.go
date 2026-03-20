package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Nomadcxx/nanobot-go/pkg/provider"
)

type ToolContext struct {
	SessionKey    string
	Channel       string
	ChatID        string
	Workspace     string
	Spawner       Spawner
	MessageRouter MessageRouter
	IsCronContext bool
}

type Result struct {
	Output   string
	Content  string
	Error    string
	Metadata map[string]any
}

type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any
	Execute(ctx context.Context, args json.RawMessage, tctx ToolContext) (*Result, error)
}

type Spawner interface {
	ProcessDirect(ctx context.Context, req SpawnRequest) (string, error)
}

type SpawnRequest struct {
	ParentSessionKey string
	ChildSessionKey  string
	Message          string
	MaxIterations    int
	DisabledTools    []string
}

type MessageRouter interface {
	Route(ctx context.Context, channel, chatID, content string) error
}

type CronService interface {
	Handle(ctx context.Context, req CronRequest) (string, error)
}

type CronRequest struct {
	Action   string
	ID       string
	Name     string
	Schedule string
	Timezone string
	Reminder string
	Channel  string
	ChatID   string
	Enabled  bool
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
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].Name < defs[j].Name
	})
	return defs
}

func (r *Registry) Execute(ctx context.Context, name string, args json.RawMessage, tctx ToolContext) (*Result, error) {
	r.mu.RLock()
	tool, ok := r.tools[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("tool %q not found", name)
	}
	result, err := tool.Execute(ctx, args, tctx)
	if err != nil {
		return nil, err
	}
	if result != nil && result.Error != "" && !strings.Contains(result.Error, "try a different approach") {
		result.Error += "\n\n[Analyze the error above and try a different approach.]"
	}
	return result, nil
}

func (r *Registry) CancelSession(sessionKey string) {
	if r.cancelSession != nil {
		r.cancelSession(sessionKey)
	}
}

func CoerceArgs[T any](input map[string]any) (T, error) {
	var zero T

	coerced := make(map[string]any, len(input))
	for key, value := range input {
		switch typed := value.(type) {
		case string:
			if i, err := strconv.Atoi(typed); err == nil {
				coerced[key] = i
				continue
			}
			if b, err := strconv.ParseBool(typed); err == nil {
				coerced[key] = b
				continue
			}
			coerced[key] = typed
		default:
			coerced[key] = value
		}
	}

	data, err := json.Marshal(coerced)
	if err != nil {
		return zero, fmt.Errorf("marshal args: %w", err)
	}

	var out T
	if err := json.Unmarshal(data, &out); err != nil {
		return zero, fmt.Errorf("unmarshal args: %w", err)
	}
	return out, nil
}
