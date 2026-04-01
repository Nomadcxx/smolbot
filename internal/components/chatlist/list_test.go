package chatlist

import (
	"strings"
	"testing"
)

type textItem struct {
	content string
}

func (t *textItem) Render(width int) string {
	return t.content
}

func itemHeights(items []Item, width int) []int {
	heights := make([]int, len(items))
	for i, item := range items {
		rendered := item.Render(width)
		heights[i] = strings.Count(rendered, "\n") + 1
	}
	return heights
}

func TestListRenderOnlyVisibleItems(t *testing.T) {
	items := make([]Item, 20)
	for i := range items {
		items[i] = &textItem{content: "item"}
	}

	l := NewList(items...)
	l.SetSize(80, 5)
	l.SetGap(1)

	l.ScrollToTop()
	view := l.Render()
	lineCount := strings.Count(view, "\n") + 1
	if lineCount != 5 {
		t.Errorf("expected 5 lines at top, got %d", lineCount)
	}
}

func TestListScrollBy(t *testing.T) {
	items := make([]Item, 20)
	for i := range items {
		items[i] = &textItem{content: "item"}
	}

	l := NewList(items...)
	l.SetSize(80, 5)
	l.SetGap(1)

	initialIdx := l.offsetIdx
	l.ScrollBy(1)
	if l.offsetIdx <= initialIdx {
		t.Errorf("expected offsetIdx to advance, got %d", l.offsetIdx)
	}
}

func TestListScrollToBottom(t *testing.T) {
	items := make([]Item, 20)
	for i := range items {
		items[i] = &textItem{content: "item"}
	}

	l := NewList(items...)
	l.SetSize(80, 5)
	l.SetGap(1)

	l.ScrollToBottom()
	if !l.AtBottom() {
		t.Errorf("expected AtBottom to be true after ScrollToBottom")
	}
}

func TestListScrollToTop(t *testing.T) {
	items := make([]Item, 20)
	for i := range items {
		items[i] = &textItem{content: "item"}
	}

	l := NewList(items...)
	l.SetSize(80, 5)
	l.SetGap(1)

	l.ScrollToBottom()
	l.ScrollToTop()
	if l.offsetIdx != 0 || l.offsetLine != 0 {
		t.Errorf("expected offsetIdx=0 offsetLine=0, got %d %d", l.offsetIdx, l.offsetLine)
	}
}

func TestListAtBottom(t *testing.T) {
	items := make([]Item, 5)
	for i := range items {
		items[i] = &textItem{content: "item"}
	}

	l := NewList(items...)
	l.SetSize(80, 10)
	l.SetGap(0)

	if !l.AtBottom() {
		t.Errorf("expected AtBottom to be true when all items fit")
	}
}

func TestListAppendWithFollow(t *testing.T) {
	items := make([]Item, 3)
	for i := range items {
		items[i] = &textItem{content: "item"}
	}

	l := NewList(items...)
	l.SetSize(80, 5)
	l.SetGap(1)
	l.SetFollow(true)

	initialIdx := l.offsetIdx
	_ = initialIdx
	l.ScrollBy(2)
	l.AppendItem(&textItem{content: "new item"})

	if !l.AtBottom() {
		t.Errorf("expected AtBottom after append with follow=true")
	}
}

func TestListGap(t *testing.T) {
	items := []Item{
		&textItem{content: "a"},
		&textItem{content: "b"},
		&textItem{content: "c"},
	}

	l := NewList(items...)
	l.SetSize(80, 10)
	l.SetGap(2)

	rendered := l.Render()
	lines := strings.Split(rendered, "\n")
	gapLines := 0
	for i, line := range lines {
		if i > 0 && line == "" {
			gapLines++
		}
	}

	if gapLines < 2 {
		t.Errorf("expected at least 2 gap lines, got %d", gapLines)
	}
}

func TestListPartialItem(t *testing.T) {
	items := []Item{
		&textItem{content: "line1\nline2\nline3\nline4\nline5"},
		&textItem{content: "other"},
	}

	l := NewList(items...)
	l.SetSize(80, 3)
	l.SetGap(0)

	rendered := l.Render()
	lineCount := strings.Count(rendered, "\n") + 1
	if lineCount != 3 {
		t.Errorf("expected 3 lines, got %d", lineCount)
	}
}

func TestListAtBottomState(t *testing.T) {
	items := make([]Item, 20)
	for i := range items {
		items[i] = &textItem{content: "item"}
	}

	l := NewList(items...)
	l.SetSize(80, 5)
	l.SetGap(1)

	l.ScrollToBottom()
	t.Logf("After ScrollToBottom: offsetIdx=%d offsetLine=%d AtBottom=%v remainingContent=%d lastVisible=%d",
		l.offsetIdx, l.offsetLine, l.AtBottom(), l.remainingContent(), l.lastVisibleContent())

	// Check AtBottom is true
	if !l.AtBottom() {
		t.Errorf("AtBottom() should be true after ScrollToBottom, got false")
	}

	// Scroll up
	l.ScrollBy(-1)
	t.Logf("After ScrollBy(-1): offsetIdx=%d offsetLine=%d AtBottom=%v",
		l.offsetIdx, l.offsetLine, l.AtBottom())

	if l.AtBottom() {
		t.Errorf("AtBottom() should be false after scrolling up")
	}
}
