package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ToolSearchTool is the tool_search meta-tool. It lets the model discover and
// unlock deferred tools by keyword, reducing prompt size for simple queries.
// It is always visible (IsAlwaysLoad = true) but never itself deferred.
type ToolSearchTool struct {
	registry *Registry
}

func NewToolSearchTool(registry *Registry) *ToolSearchTool {
	return &ToolSearchTool{registry: registry}
}

func (t *ToolSearchTool) Name() string { return "tool_search" }
func (t *ToolSearchTool) Description() string {
	return "Search for additional tools by keyword. Use when you need capabilities not visible in your current tool list (e.g. web browsing, scheduling, messaging, file creation, agent delegation)."
}
func (t *ToolSearchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Keywords to search for (e.g. 'web search', 'file create', 'schedule', 'send message').",
			},
		},
		"required": []string{"query"},
	}
}

// IsAlwaysLoad returns true so tool_search is always included in the visible tool list.
func (t *ToolSearchTool) IsAlwaysLoad() bool { return true }

// IsDeferred returns false — tool_search itself is never hidden.
func (t *ToolSearchTool) IsDeferred() bool { return false }

// DeferredKeywords is unused (IsDeferred = false) but satisfies DeferredTool.
func (t *ToolSearchTool) DeferredKeywords() []string { return nil }

func (t *ToolSearchTool) Execute(ctx context.Context, raw json.RawMessage, tctx ToolContext) (*Result, error) {
	var args struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("tool_search: invalid args: %w", err)
	}
	if strings.TrimSpace(args.Query) == "" {
		return &Result{Output: "Please provide a non-empty search query."}, nil
	}

	matches := t.registry.SearchDeferredTools(args.Query)
	if len(matches) == 0 {
		return &Result{
			Output: fmt.Sprintf("No additional tools found matching %q. Current tools cover your needs.", args.Query),
		}, nil
	}

	names := make([]string, 0, len(matches))
	lines := make([]string, 0, len(matches)+2)
	lines = append(lines, fmt.Sprintf("Found %d tool(s) matching %q:", len(matches), args.Query))
	for _, m := range matches {
		names = append(names, m.Name())
		lines = append(lines, fmt.Sprintf("- %s: %s", m.Name(), m.Description()))
	}
	lines = append(lines, "\nThese tools are now available for use.")

	if tctx.DiscoverTools != nil {
		tctx.DiscoverTools(names)
	}

	return &Result{Output: strings.Join(lines, "\n")}, nil
}
