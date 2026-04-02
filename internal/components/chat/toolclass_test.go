package chat

import "testing"

func TestClassifyTool(t *testing.T) {
	tests := []struct {
		name      string
		wantClass ToolClass
		wantKind  ToolKind
	}{
		{"read_file", ToolClassCollapsible, ToolKindRead},
		{"list_dir", ToolClassCollapsible, ToolKindList},
		{"web_search", ToolClassCollapsible, ToolKindSearch},
		{"grep", ToolClassCollapsible, ToolKindSearch},
		{"write_file", ToolClassStandalone, ToolKindWrite},
		{"edit_file", ToolClassStandalone, ToolKindEdit},
		{"exec", ToolClassStandalone, ToolKindBash},
		{"bash", ToolClassStandalone, ToolKindBash},
		{"web_fetch", ToolClassStandalone, ToolKindFetch},
		{"message", ToolClassStandalone, ToolKindMessage},
		{"spawn", ToolClassStandalone, ToolKindSpawn},
		{"unknown_tool", ToolClassStandalone, ToolKindOther},
		{"", ToolClassStandalone, ToolKindOther},
		// Case insensitive
		{"READ_FILE", ToolClassCollapsible, ToolKindRead},
		{"List_Dir", ToolClassCollapsible, ToolKindList},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotClass, gotKind := ClassifyTool(tc.name)
			if gotClass != tc.wantClass {
				t.Errorf("ClassifyTool(%q) class = %v, want %v", tc.name, gotClass, tc.wantClass)
			}
			if gotKind != tc.wantKind {
				t.Errorf("ClassifyTool(%q) kind = %v, want %v", tc.name, gotKind, tc.wantKind)
			}
		})
	}
}

func TestIsCollapsible(t *testing.T) {
	if !IsCollapsible("read_file") {
		t.Error("read_file should be collapsible")
	}
	if IsCollapsible("exec") {
		t.Error("exec should not be collapsible")
	}
}
