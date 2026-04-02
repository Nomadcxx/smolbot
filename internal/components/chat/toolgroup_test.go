package chat

import "testing"

// --- ToolGroup tests ---

func TestToolGroupAdd(t *testing.T) {
	g := NewToolGroup()

	g.Add(ToolCall{Name: "read_file", Input: `{"path": "src/main.go"}`, Status: "done"}, ToolKindRead)
	g.Add(ToolCall{Name: "read_file", Input: `{"path": "src/util.go"}`, Status: "done"}, ToolKindRead)
	g.Add(ToolCall{Name: "read_file", Input: `{"path": "src/main.go"}`, Status: "done"}, ToolKindRead) // duplicate

	if g.ReadCount != 3 {
		t.Errorf("ReadCount = %d, want 3", g.ReadCount)
	}
	if g.DisplayReadCount() != 2 {
		t.Errorf("DisplayReadCount() = %d, want 2 (deduped)", g.DisplayReadCount())
	}
	if len(g.FilePaths) != 2 {
		t.Errorf("len(FilePaths) = %d, want 2", len(g.FilePaths))
	}
}

func TestToolGroupStatus(t *testing.T) {
	g := NewToolGroup()

	g.Add(ToolCall{Name: "read_file", Input: `{"path": "a.go"}`, Status: "done"}, ToolKindRead)
	if g.IsActive {
		t.Error("group should not be active when all tools done")
	}

	g.Add(ToolCall{Name: "read_file", Input: `{"path": "b.go"}`, Status: "running"}, ToolKindRead)
	if !g.IsActive {
		t.Error("group should be active when a tool is running")
	}

	g.Add(ToolCall{Name: "read_file", Input: `{"path": "c.go"}`, Status: "error"}, ToolKindRead)
	if !g.HasError {
		t.Error("group should have error flag set")
	}
}

func TestToolGroupCurrentOperation(t *testing.T) {
	g := NewToolGroup()

	g.Add(ToolCall{ID: "1", Name: "read_file", Input: `{"path": "a.go"}`, Status: "done"}, ToolKindRead)
	g.Add(ToolCall{ID: "2", Name: "read_file", Input: `{"path": "b.go"}`, Status: "running"}, ToolKindRead)

	current := g.CurrentOperation()
	if current == nil {
		t.Fatal("expected current operation, got nil")
	}
	if current.ID != "2" {
		t.Errorf("CurrentOperation().ID = %q, want %q", current.ID, "2")
	}
}

func TestToolGroupAntiJitter(t *testing.T) {
	g := NewToolGroup()

	g.Add(ToolCall{Name: "read_file", Input: `{"path": "a.go"}`, Status: "running"}, ToolKindRead)
	if g.MaxReadCount != 1 {
		t.Errorf("MaxReadCount = %d, want 1", g.MaxReadCount)
	}

	g.Add(ToolCall{Name: "read_file", Input: `{"path": "b.go"}`, Status: "running"}, ToolKindRead)
	if g.MaxReadCount != 2 {
		t.Errorf("MaxReadCount = %d, want 2", g.MaxReadCount)
	}

	if g.DisplayReadCount() < 2 {
		t.Errorf("DisplayReadCount() should never decrease, got %d", g.DisplayReadCount())
	}
}

func TestExtractFilePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`{"path": "src/main.go"}`, "src/main.go"},
		{`{"path": " src/util.go "}`, "src/util.go"},
		{`{"command": "ls"}`, ""},
		{`invalid json`, ""},
		{`{}`, ""},
	}

	for _, tc := range tests {
		got := extractFilePath(tc.input)
		if got != tc.want {
			t.Errorf("extractFilePath(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestExtractSearchQuery(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`{"query": "TODO"}`, "TODO"},
		{`{"pattern": "FIXME"}`, "FIXME"},
		{`{"path": "src/"}`, ""},
	}

	for _, tc := range tests {
		got := extractSearchQuery(tc.input)
		if got != tc.want {
			t.Errorf("extractSearchQuery(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// --- Summary tests (Phase 4) ---

func TestToolGroupSummary(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*ToolGroup)
		wantDone   string
		wantActive string
	}{
		{
			name: "single read",
			setup: func(g *ToolGroup) {
				g.Add(ToolCall{Name: "read_file", Input: `{"path": "a.go"}`, Status: "done"}, ToolKindRead)
			},
			wantDone:   "read 1 file",
			wantActive: "reading 1 file...",
		},
		{
			name: "multiple reads",
			setup: func(g *ToolGroup) {
				g.Add(ToolCall{Name: "read_file", Input: `{"path": "a.go"}`, Status: "done"}, ToolKindRead)
				g.Add(ToolCall{Name: "read_file", Input: `{"path": "b.go"}`, Status: "done"}, ToolKindRead)
				g.Add(ToolCall{Name: "read_file", Input: `{"path": "c.go"}`, Status: "done"}, ToolKindRead)
			},
			wantDone:   "read 3 files",
			wantActive: "reading 3 files...",
		},
		{
			name: "reads and searches",
			setup: func(g *ToolGroup) {
				g.Add(ToolCall{Name: "read_file", Input: `{"path": "a.go"}`, Status: "done"}, ToolKindRead)
				g.Add(ToolCall{Name: "read_file", Input: `{"path": "b.go"}`, Status: "done"}, ToolKindRead)
				g.Add(ToolCall{Name: "web_search", Input: `{"query": "TODO"}`, Status: "done"}, ToolKindSearch)
			},
			wantDone:   "read 2 files, searched for 1 pattern",
			wantActive: "reading 2 files, searching for 1 pattern...",
		},
		{
			name: "all types",
			setup: func(g *ToolGroup) {
				g.Add(ToolCall{Name: "read_file", Input: `{"path": "a.go"}`, Status: "done"}, ToolKindRead)
				g.Add(ToolCall{Name: "web_search", Input: `{"query": "TODO"}`, Status: "done"}, ToolKindSearch)
				g.Add(ToolCall{Name: "web_search", Input: `{"query": "FIXME"}`, Status: "done"}, ToolKindSearch)
				g.Add(ToolCall{Name: "list_dir", Input: `{"path": "src/"}`, Status: "done"}, ToolKindList)
			},
			wantDone:   "read 1 file, searched for 2 patterns, listed 1 directory",
			wantActive: "reading 1 file, searching for 2 patterns, listing 1 directory...",
		},
		{
			name:       "empty group",
			setup:      func(g *ToolGroup) {},
			wantDone:   "completed",
			wantActive: "working...",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewToolGroup()
			tc.setup(g)

			if got := g.Summary(); got != tc.wantDone {
				t.Errorf("Summary() = %q, want %q", got, tc.wantDone)
			}
			if got := g.ActiveSummary(); got != tc.wantActive {
				t.Errorf("ActiveSummary() = %q, want %q", got, tc.wantActive)
			}
		})
	}
}

// --- Collapse engine tests (Phase 3) ---

func TestCollapseToolsEmpty(t *testing.T) {
	if blocks := CollapseTools(nil); len(blocks) != 0 {
		t.Errorf("CollapseTools(nil) = %d blocks, want 0", len(blocks))
	}
	if blocks := CollapseTools([]ToolCall{}); len(blocks) != 0 {
		t.Errorf("CollapseTools([]) = %d blocks, want 0", len(blocks))
	}
}

func TestCollapseToolsAllCollapsible(t *testing.T) {
	tools := []ToolCall{
		{ID: "1", Name: "read_file", Input: `{"path": "a.go"}`, Status: "done"},
		{ID: "2", Name: "read_file", Input: `{"path": "b.go"}`, Status: "done"},
		{ID: "3", Name: "list_dir", Input: `{"path": "src/"}`, Status: "done"},
	}

	blocks := CollapseTools(tools)
	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want 1 (all grouped)", len(blocks))
	}
	if !blocks[0].IsGroup {
		t.Error("expected a group block")
	}
	if len(blocks[0].Group.Tools) != 3 {
		t.Errorf("group has %d tools, want 3", len(blocks[0].Group.Tools))
	}
}

func TestCollapseToolsAllStandalone(t *testing.T) {
	tools := []ToolCall{
		{ID: "1", Name: "write_file", Status: "done"},
		{ID: "2", Name: "exec", Status: "done"},
		{ID: "3", Name: "edit_file", Status: "done"},
	}

	blocks := CollapseTools(tools)
	if len(blocks) != 3 {
		t.Fatalf("got %d blocks, want 3 (each standalone)", len(blocks))
	}
	for i, b := range blocks {
		if b.IsGroup {
			t.Errorf("block %d should be standalone, not group", i)
		}
	}
}

func TestCollapseToolsMixed(t *testing.T) {
	tools := []ToolCall{
		{ID: "1", Name: "read_file", Input: `{"path": "a.go"}`, Status: "done"},
		{ID: "2", Name: "read_file", Input: `{"path": "b.go"}`, Status: "done"},
		{ID: "3", Name: "write_file", Status: "done"},
		{ID: "4", Name: "read_file", Input: `{"path": "c.go"}`, Status: "done"},
		{ID: "5", Name: "exec", Status: "done"},
		{ID: "6", Name: "read_file", Input: `{"path": "d.go"}`, Status: "done"},
	}

	blocks := CollapseTools(tools)
	if len(blocks) != 5 {
		t.Fatalf("got %d blocks, want 5", len(blocks))
	}

	if !blocks[0].IsGroup || len(blocks[0].Group.Tools) != 2 {
		t.Errorf("block 0: want Group(2), got IsGroup=%v len=%d", blocks[0].IsGroup, len(blocks[0].Group.Tools))
	}
	if blocks[1].IsGroup || blocks[1].Tool.Name != "write_file" {
		t.Errorf("block 1: want Standalone(write_file)")
	}
	if !blocks[2].IsGroup || len(blocks[2].Group.Tools) != 1 {
		t.Errorf("block 2: want Group(1)")
	}
	if blocks[3].IsGroup || blocks[3].Tool.Name != "exec" {
		t.Errorf("block 3: want Standalone(exec)")
	}
	if !blocks[4].IsGroup || len(blocks[4].Group.Tools) != 1 {
		t.Errorf("block 4: want Group(1)")
	}
}

func TestCollapseToolsPreservesOrder(t *testing.T) {
	tools := []ToolCall{
		{ID: "1", Name: "read_file", Input: `{"path": "first.go"}`, Status: "done"},
		{ID: "2", Name: "read_file", Input: `{"path": "second.go"}`, Status: "done"},
	}

	blocks := CollapseTools(tools)
	if len(blocks) != 1 || !blocks[0].IsGroup {
		t.Fatal("expected 1 group")
	}

	group := blocks[0].Group
	if group.Tools[0].ID != "1" {
		t.Error("tools not in original order")
	}
	if group.Tools[1].ID != "2" {
		t.Error("tools not in original order")
	}
}

func TestCollapseToolsStatusPropagation(t *testing.T) {
	tools := []ToolCall{
		{ID: "1", Name: "read_file", Input: `{"path": "a.go"}`, Status: "done"},
		{ID: "2", Name: "read_file", Input: `{"path": "b.go"}`, Status: "running"},
		{ID: "3", Name: "read_file", Input: `{"path": "c.go"}`, Status: "error"},
	}

	blocks := CollapseTools(tools)
	group := blocks[0].Group

	if !group.IsActive {
		t.Error("group should be active (has running tool)")
	}
	if !group.HasError {
		t.Error("group should have error flag (has errored tool)")
	}
}

// --- Hint line tests (Phase 10) ---

func TestToolGroupHintLine(t *testing.T) {
	g := NewToolGroup()

	g.Add(ToolCall{ID: "1", Name: "read_file", Input: `{"path": "a.go"}`, Status: "done"}, ToolKindRead)

	if hint := g.CurrentOperationForHint(); hint != nil {
		t.Error("no hint expected when all tools done")
	}

	g.Add(ToolCall{ID: "2", Name: "read_file", Input: `{"path": "b.go"}`, Status: "running"}, ToolKindRead)

	hint := g.CurrentOperationForHint()
	if hint == nil || hint.ID != "2" {
		t.Error("expected hint for running tool")
	}
}
