package chat

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ToolGroup accumulates consecutive collapsible tools for grouped rendering.
type ToolGroup struct {
	ReadCount   int
	SearchCount int
	ListCount   int

	FilePaths     map[string]struct{}
	SearchQueries []string
	ListDirs      []string

	Tools []ToolCall

	IsActive bool
	HasError bool

	MaxReadCount   int
	MaxSearchCount int
	MaxListCount   int

	StartTime     time.Time
	LastToolStart time.Time

	// Anti-flicker for hint line
	lastHintTool string
	lastHintTime time.Time
}

const minHintDisplayDuration = 700 * time.Millisecond

// NewToolGroup creates an empty group ready for accumulation.
func NewToolGroup() *ToolGroup {
	return &ToolGroup{
		FilePaths:     make(map[string]struct{}),
		SearchQueries: make([]string, 0, 4),
		ListDirs:      make([]string, 0, 4),
		Tools:         make([]ToolCall, 0, 8),
		StartTime:     time.Now(),
	}
}

// Add incorporates a tool into the group.
func (g *ToolGroup) Add(tc ToolCall, kind ToolKind) {
	g.Tools = append(g.Tools, tc)

	switch strings.ToLower(tc.Status) {
	case "running":
		g.IsActive = true
		g.LastToolStart = time.Now()
	case "error":
		g.HasError = true
	}

	if path := extractFilePath(tc.Input); path != "" {
		g.FilePaths[path] = struct{}{}
	}

	switch kind {
	case ToolKindRead:
		g.ReadCount++
		g.MaxReadCount = max(g.MaxReadCount, len(g.FilePaths))
	case ToolKindSearch:
		g.SearchCount++
		g.MaxSearchCount = max(g.MaxSearchCount, g.SearchCount)
		if query := extractSearchQuery(tc.Input); query != "" {
			g.SearchQueries = append(g.SearchQueries, query)
		}
	case ToolKindList:
		g.ListCount++
		g.MaxListCount = max(g.MaxListCount, g.ListCount)
		if dir := extractFilePath(tc.Input); dir != "" {
			g.ListDirs = append(g.ListDirs, dir)
		}
	}
}

// Empty returns true if no tools have been added.
func (g *ToolGroup) Empty() bool {
	return len(g.Tools) == 0
}

func (g *ToolGroup) DisplayReadCount() int {
	if g.MaxReadCount > 0 {
		return g.MaxReadCount
	}
	if n := len(g.FilePaths); n > 0 {
		return n
	}
	// Fallback: no paths were extractable from inputs, count by tool invocations.
	return g.ReadCount
}

func (g *ToolGroup) DisplaySearchCount() int {
	return g.MaxSearchCount
}

func (g *ToolGroup) DisplayListCount() int {
	return g.MaxListCount
}

// CurrentOperation returns the most recent running tool.
func (g *ToolGroup) CurrentOperation() *ToolCall {
	for i := len(g.Tools) - 1; i >= 0; i-- {
		if strings.ToLower(g.Tools[i].Status) == "running" {
			return &g.Tools[i]
		}
	}
	return nil
}

// CurrentOperationForHint returns the tool to display in the hint line.
// Implements anti-flicker: holds the same hint for at least minHintDisplayDuration.
func (g *ToolGroup) CurrentOperationForHint() *ToolCall {
	current := g.CurrentOperation()
	if current == nil {
		return nil
	}

	now := time.Now()

	if g.lastHintTool == current.ID {
		return current
	}

	if g.lastHintTool != "" && now.Sub(g.lastHintTime) < minHintDisplayDuration {
		// Hold the old hint
		for i := range g.Tools {
			if g.Tools[i].ID == g.lastHintTool {
				return &g.Tools[i]
			}
		}
	}

	g.lastHintTool = current.ID
	g.lastHintTime = now
	return current
}

// RunningCount returns how many tools are currently running.
func (g *ToolGroup) RunningCount() int {
	count := 0
	for _, tc := range g.Tools {
		if strings.ToLower(tc.Status) == "running" {
			count++
		}
	}
	return count
}

// ErrorCount returns how many tools errored.
func (g *ToolGroup) ErrorCount() int {
	count := 0
	for _, tc := range g.Tools {
		if strings.ToLower(tc.Status) == "error" {
			count++
		}
	}
	return count
}

// --- Phase 4: Natural-Language Summaries ---

// Summary returns a past-tense natural-language description.
// Example: "Read 3 files, searched for 2 patterns"
func (g *ToolGroup) Summary() string {
	return g.buildSummary(false)
}

// ActiveSummary returns a present-tense description with trailing "...".
// Example: "Reading 3 files, searching for 2 patterns..."
func (g *ToolGroup) ActiveSummary() string {
	return g.buildSummary(true) + "..."
}

func (g *ToolGroup) buildSummary(active bool) string {
	var parts []string

	if n := g.DisplayReadCount(); n > 0 {
		if active {
			parts = append(parts, pluralizeVerb("reading", "file", n))
		} else {
			parts = append(parts, pluralizeVerb("read", "file", n))
		}
	}

	if n := g.DisplaySearchCount(); n > 0 {
		if active {
			parts = append(parts, pluralizeFor("searching", "pattern", n))
		} else {
			parts = append(parts, pluralizeFor("searched", "pattern", n))
		}
	}

	if n := g.DisplayListCount(); n > 0 {
		if active {
			parts = append(parts, pluralizeDir("listing", n))
		} else {
			parts = append(parts, pluralizeDir("listed", n))
		}
	}

	if len(parts) == 0 {
		if active {
			return "working"
		}
		return "completed"
	}
	return strings.Join(parts, ", ")
}

func pluralizeVerb(verb, noun string, count int) string {
	if count == 1 {
		return fmt.Sprintf("%s %d %s", verb, count, noun)
	}
	return fmt.Sprintf("%s %d %ss", verb, count, noun)
}

func pluralizeFor(verb, noun string, count int) string {
	if count == 1 {
		return fmt.Sprintf("%s for %d %s", verb, count, noun)
	}
	return fmt.Sprintf("%s for %d %ss", verb, count, noun)
}

func pluralizeDir(verb string, count int) string {
	if count == 1 {
		return fmt.Sprintf("%s %d directory", verb, count)
	}
	return fmt.Sprintf("%s %d directories", verb, count)
}

// --- Phase 3: Collapse Engine ---

// CollapsedBlock represents either a collapsed group or a standalone tool.
type CollapsedBlock struct {
	IsGroup bool
	Group   *ToolGroup
	Tool    *ToolCall
	Kind    ToolKind
}

// CollapseTools transforms a flat list of tools into grouped/standalone blocks.
// Consecutive collapsible tools are merged into a single ToolGroup.
// Standalone tools each become their own block.
func CollapseTools(tools []ToolCall) []CollapsedBlock {
	if len(tools) == 0 {
		return nil
	}

	blocks := make([]CollapsedBlock, 0, len(tools)/2+1)
	var currentGroup *ToolGroup

	flushGroup := func() {
		if currentGroup != nil && !currentGroup.Empty() {
			blocks = append(blocks, CollapsedBlock{
				IsGroup: true,
				Group:   currentGroup,
				Kind:    dominantKind(currentGroup),
			})
			currentGroup = nil
		}
	}

	for i := range tools {
		tc := &tools[i]
		class, kind := ClassifyTool(tc.Name)

		if class == ToolClassCollapsible {
			if currentGroup == nil {
				currentGroup = NewToolGroup()
			}
			currentGroup.Add(*tc, kind)
		} else {
			flushGroup()
			blocks = append(blocks, CollapsedBlock{
				IsGroup: false,
				Tool:    tc,
				Kind:    kind,
			})
		}
	}

	flushGroup()
	return blocks
}

func dominantKind(g *ToolGroup) ToolKind {
	maxCount := 0
	dominant := ToolKindRead

	if n := g.DisplayReadCount(); n > maxCount {
		maxCount = n
		dominant = ToolKindRead
	}
	if n := g.DisplaySearchCount(); n > maxCount {
		maxCount = n
		dominant = ToolKindSearch
	}
	if n := g.DisplayListCount(); n > maxCount {
		dominant = ToolKindList
	}

	return dominant
}

// --- JSON field extraction helpers (also used by message.go) ---

func extractFilePath(input string) string {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		return ""
	}
	if path, ok := data["path"].(string); ok {
		return strings.TrimSpace(path)
	}
	return ""
}

func extractSearchQuery(input string) string {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		return ""
	}
	if query, ok := data["query"].(string); ok {
		return strings.TrimSpace(query)
	}
	if pattern, ok := data["pattern"].(string); ok {
		return strings.TrimSpace(pattern)
	}
	return ""
}
