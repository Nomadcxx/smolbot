package chat

import (
	"strings"
	"testing"

	"github.com/Nomadcxx/smolbot/internal/theme"
	_ "github.com/Nomadcxx/smolbot/internal/theme/themes"
)

func TestParseUnifiedDiffTracksLineKindsAndNumbers(t *testing.T) {
	parsed := ParseUnifiedDiff(strings.Join([]string{
		"diff --git a/foo.txt b/foo.txt",
		"index 1111111..2222222 100644",
		"--- a/foo.txt",
		"+++ b/foo.txt",
		"@@ -1,3 +1,3 @@",
		" line one",
		"-old line",
		"+new line",
		" same",
	}, "\n"))

	if len(parsed.Lines) != 9 {
		t.Fatalf("expected 9 parsed lines, got %d", len(parsed.Lines))
	}

	if parsed.Lines[2].Kind != DiffLineFileHeader || parsed.Lines[3].Kind != DiffLineFileHeader {
		t.Fatalf("expected file headers to be preserved, got %#v %#v", parsed.Lines[2], parsed.Lines[3])
	}
	if parsed.Lines[4].Kind != DiffLineHunkHeader {
		t.Fatalf("expected hunk header, got %#v", parsed.Lines[4])
	}
	if parsed.Lines[5].Kind != DiffLineContext || parsed.Lines[5].OldLine != 1 || parsed.Lines[5].NewLine != 1 {
		t.Fatalf("expected first context line numbers to start at 1, got %#v", parsed.Lines[5])
	}
	if parsed.Lines[6].Kind != DiffLineRemoved || parsed.Lines[6].OldLine != 2 {
		t.Fatalf("expected removed line to preserve old line number, got %#v", parsed.Lines[6])
	}
	if parsed.Lines[7].Kind != DiffLineAdded || parsed.Lines[7].NewLine != 2 {
		t.Fatalf("expected added line to preserve new line number, got %#v", parsed.Lines[7])
	}
}

func TestRenderDiffSwitchesLayoutsByWidth(t *testing.T) {
	useTheme(t)
	current := theme.Current()
	if current == nil {
		t.Fatal("expected current theme")
	}

	diff := strings.Join([]string{
		"diff --git a/foo.txt b/foo.txt",
		"--- a/foo.txt",
		"+++ b/foo.txt",
		"@@ -1,3 +1,3 @@",
		" keep line",
		"-old line",
		"+new line",
		" keep again",
	}, "\n")

	narrow := RenderDiff(diff, 80, current)
	wide := RenderDiff(diff, 140, current)

	if !strings.Contains(stripANSI(narrow), "- old line") || !strings.Contains(stripANSI(narrow), "+ new line") {
		t.Fatalf("expected unified rendering at narrow widths, got %q", narrow)
	}
	if !strings.Contains(wide, "│") {
		t.Fatalf("expected wide rendering to introduce a split column, got %q", wide)
	}
	if !strings.Contains(wide, "old line") || !strings.Contains(wide, "new line") {
		t.Fatalf("expected wide rendering to preserve both sides of the change, got %q", wide)
	}
}
