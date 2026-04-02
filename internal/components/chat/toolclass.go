package chat

import "strings"

// ToolClass determines rendering strategy for a tool call.
type ToolClass int

const (
	// ToolClassCollapsible tools can be grouped into a single summary line.
	ToolClassCollapsible ToolClass = iota

	// ToolClassStandalone tools must render individually with full detail.
	ToolClassStandalone
)

// ToolKind identifies the semantic type of a tool for counting and display.
type ToolKind int

const (
	ToolKindRead   ToolKind = iota // file read operations
	ToolKindSearch                 // grep, web search
	ToolKindList                   // directory listing
	ToolKindBash                   // shell execution
	ToolKindWrite                  // file write
	ToolKindEdit                   // file edit (with diff)
	ToolKindMessage                // channel messaging
	ToolKindFetch                  // web fetch
	ToolKindSpawn                  // agent spawn
	ToolKindOther                  // unknown/unclassified
)

// ClassifyTool determines how a tool should be rendered and counted.
func ClassifyTool(name string) (ToolClass, ToolKind) {
	normalized := strings.ToLower(strings.TrimSpace(name))

	switch normalized {
	case "read_file", "readfile", "file_read":
		return ToolClassCollapsible, ToolKindRead
	case "list_dir", "listdir", "directory_list", "ls":
		return ToolClassCollapsible, ToolKindList
	case "web_search", "search", "grep":
		return ToolClassCollapsible, ToolKindSearch

	case "write_file", "writefile", "file_write":
		return ToolClassStandalone, ToolKindWrite
	case "edit_file", "editfile", "file_edit":
		return ToolClassStandalone, ToolKindEdit
	case "exec", "bash", "shell", "run":
		return ToolClassStandalone, ToolKindBash
	case "web_fetch", "fetch", "curl":
		return ToolClassStandalone, ToolKindFetch
	case "message":
		return ToolClassStandalone, ToolKindMessage
	case "spawn", "spawn_agent":
		return ToolClassStandalone, ToolKindSpawn

	default:
		return ToolClassStandalone, ToolKindOther
	}
}

// IsCollapsible returns true when a tool can be grouped into a summary line.
func IsCollapsible(name string) bool {
	class, _ := ClassifyTool(name)
	return class == ToolClassCollapsible
}
