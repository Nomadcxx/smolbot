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
	EmitEvent     func(name string, payload map[string]any)
	// DiscoverTools makes deferred tools available for the rest of the session.
	// Called by the tool_search meta-tool with a list of tool names to unlock.
	DiscoverTools func(names []string)
}

type toolContextKey struct{}

// WithToolContext stores a ToolContext in a Go context.
func WithToolContext(ctx context.Context, tctx ToolContext) context.Context {
	return context.WithValue(ctx, toolContextKey{}, tctx)
}

// ContextToolContext retrieves the ToolContext from a Go context.
// Returns the zero value and false if not present.
func ContextToolContext(ctx context.Context) (ToolContext, bool) {
	tctx, ok := ctx.Value(toolContextKey{}).(ToolContext)
	return tctx, ok
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

// ConcurrencySafer is an optional interface a Tool can implement to declare it
// is safe to execute concurrently with other safe tools (no shared mutable state,
// no file writes, purely read-only or independent network requests).
type ConcurrencySafer interface {
	IsConcurrencySafe() bool
}

// DeferredTool is an optional interface a Tool can implement to opt into
// deferment: the tool is hidden from the model until discovered via tool_search.
type DeferredTool interface {
	IsDeferred() bool
	IsAlwaysLoad() bool  // true = always include regardless of deferment (e.g. tool_search itself)
	DeferredKeywords() []string
}

type Spawner interface {
	Spawn(ctx context.Context, req SpawnRequest) (*SpawnResult, error)
	ProcessDirect(ctx context.Context, req SpawnRequest) (string, error)
	Wait(ctx context.Context, req WaitRequest) (*WaitResult, error)
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
	EmitEvent        func(name string, payload map[string]any)
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

type WaitRequest struct {
	ParentSessionKey string
	AgentIDs         []string
	EmitEvent        func(name string, payload map[string]any)
}

type WaitResult struct {
	Count   int
	Results []WaitResultItem
}

type WaitResultItem struct {
	ID            string
	Name          string
	AgentType     string
	Status        string
	Description   string
	PromptPreview string
	Summary       string
	Error         string
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

// IsConcurrencySafe reports whether the named tool implements ConcurrencySafer and
// returns true. Unknown tools default to false (conservative).
func (r *Registry) IsConcurrencySafe(name string) bool {
	r.mu.RLock()
	t, ok := r.tools[name]
	r.mu.RUnlock()
	if !ok {
		return false
	}
	if cs, ok := t.(ConcurrencySafer); ok {
		return cs.IsConcurrencySafe()
	}
	return false
}

// GetVisibleTools returns tool definitions for the current request, filtering out
// deferred tools that have not yet been discovered, while always including
// AlwaysLoad tools. The disabled list is excluded as with DefinitionsExcluding.
func (r *Registry) GetVisibleTools(discovered map[string]bool, disabled []string) []provider.ToolDef {
	r.mu.RLock()
	defer r.mu.RUnlock()

	blocked := make(map[string]struct{}, len(disabled))
	for _, name := range disabled {
		name = strings.TrimSpace(name)
		if name != "" {
			blocked[name] = struct{}{}
		}
	}

	defs := make([]provider.ToolDef, 0, len(r.tools))
	for _, t := range r.tools {
		if _, skip := blocked[t.Name()]; skip {
			continue
		}
		if dt, ok := t.(DeferredTool); ok {
			if dt.IsAlwaysLoad() {
				// always include (e.g. tool_search)
			} else if dt.IsDeferred() && !discovered[t.Name()] {
				continue
			}
		}
		defs = append(defs, provider.ToolDef{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
	}
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].Name < defs[j].Name
	})
	return defs
}

// SearchDeferredTools returns all deferred tools whose name, description, or keywords
// contain at least one word from the query string.
func (r *Registry) SearchDeferredTools(query string) []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query = strings.ToLower(query)
	words := strings.Fields(query)
	if len(words) == 0 {
		return nil
	}

	var matches []Tool
	for _, t := range r.tools {
		dt, ok := t.(DeferredTool)
		if !ok || !dt.IsDeferred() {
			continue
		}
		searchText := strings.ToLower(t.Name() + " " + t.Description() + " " + strings.Join(dt.DeferredKeywords(), " "))
		for _, word := range words {
			if strings.Contains(searchText, word) {
				matches = append(matches, t)
				break
			}
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Name() < matches[j].Name()
	})
	return matches
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
			case "slice":
				// Models sometimes send "" instead of [] for empty arrays.
				if typed == "" {
					coerced[key] = []any{}
					continue
				}
				// Try parsing as JSON array.
				var arr []any
				if err := json.Unmarshal([]byte(typed), &arr); err == nil {
					coerced[key] = arr
					continue
				}
				// Single value → single-element array.
				coerced[key] = []any{typed}
				continue
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
		case reflect.Slice, reflect.Array:
			m[name] = "slice"
		default:
			m[name] = "string"
		}
	}
	return m
}
