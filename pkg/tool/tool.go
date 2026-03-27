package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Nomadcxx/smolbot/pkg/provider"
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
	Spawn(ctx context.Context, req SpawnRequest) (*SpawnResult, error)
	ProcessDirect(ctx context.Context, req SpawnRequest) (string, error)
}

type SpawnRequest struct {
	ParentSessionKey string
	ChildSessionKey  string
	Description      string
	Prompt           string
	AgentType        string
	Model            string
	ReasoningEffort  string
	MaxIterations    int
	DisabledTools    []string
}

type SpawnResult struct {
	ID              string
	SessionKey      string
	Name            string
	AgentType       string
	Model           string
	ReasoningEffort string
	Description     string
	PromptPreview   string
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
	return r.DefinitionsExcluding(nil)
}

func (r *Registry) DefinitionsExcluding(disabled []string) []provider.ToolDef {
	r.mu.RLock()
	defer r.mu.RUnlock()

	blocked := make(map[string]struct{}, len(disabled))
	for _, name := range disabled {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		blocked[name] = struct{}{}
	}

	defs := make([]provider.ToolDef, 0, len(r.tools))
	for _, tool := range r.tools {
		if _, skip := blocked[tool.Name()]; skip {
			continue
		}
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

	targetTypes := structFieldTypes(zero)

	coerced := make(map[string]any, len(input))
	for key, value := range input {
		switch typed := value.(type) {
		case string:
			expected := targetTypes[key]
			switch expected {
			case "bool":
				if b, ok := parseBoolLoose(typed); ok {
					coerced[key] = b
					continue
				}
			case "int":
				if i, err := strconv.Atoi(typed); err == nil {
					coerced[key] = i
					continue
				}
			default:
				if i, err := strconv.Atoi(typed); err == nil {
					coerced[key] = i
					continue
				}
				if b, err := strconv.ParseBool(typed); err == nil {
					coerced[key] = b
					continue
				}
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

func parseBoolLoose(s string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes", "on":
		return true, true
	case "false", "0", "no", "off":
		return false, true
	}
	return false, false
}

func structFieldTypes(v any) map[string]string {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	m := make(map[string]string, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		jsonTag := f.Tag.Get("json")
		name := strings.Split(jsonTag, ",")[0]
		if name == "" || name == "-" {
			name = f.Name
		}
		switch f.Type.Kind() {
		case reflect.Bool:
			m[name] = "bool"
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			m[name] = "int"
		default:
			m[name] = "string"
		}
	}
	return m
}
