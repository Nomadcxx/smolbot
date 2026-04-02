package scroll

import "testing"

func TestNew(t *testing.T) {
	s := New(20)
	if s.ViewHeight != 20 {
		t.Errorf("ViewHeight = %d, want 20", s.ViewHeight)
	}
	if !s.StickyScroll {
		t.Error("StickyScroll should be true by default")
	}
	if s.Offset != 0 {
		t.Errorf("Offset = %d, want 0", s.Offset)
	}
}

func TestSetContent_StickyScrollsToBottom(t *testing.T) {
	s := New(10)
	s.SetContent(50)

	// Should scroll to bottom (50 - 10 = 40)
	if s.Offset != 40 {
		t.Errorf("Offset = %d, want 40", s.Offset)
	}
	if !s.StickyScroll {
		t.Error("StickyScroll should remain true")
	}
}

func TestSetContent_NonStickyPreservesOffset(t *testing.T) {
	s := New(10)
	s.SetContent(50)
	s.ScrollBy(-10) // Scroll up, breaks sticky
	s.SetContent(60) // Add more content

	// Should NOT auto-scroll since sticky is broken
	if s.Offset != 30 { // Was at 40-10=30
		t.Errorf("Offset = %d, want 30", s.Offset)
	}
	if s.StickyScroll {
		t.Error("StickyScroll should be false after scrolling up")
	}
}

func TestScrollBy_UpBreaksSticky(t *testing.T) {
	s := New(10)
	s.SetContent(50)

	s.ScrollBy(-5) // Scroll up

	if s.StickyScroll {
		t.Error("Scrolling up should break sticky")
	}
	if s.Offset != 35 {
		t.Errorf("Offset = %d, want 35", s.Offset)
	}
}

func TestScrollBy_DownToBottomRestoresSticky(t *testing.T) {
	s := New(10)
	s.SetContent(50)
	s.ScrollBy(-20) // Scroll up, breaks sticky

	if s.StickyScroll {
		t.Error("Should not be sticky after scroll up")
	}

	s.ScrollBy(20) // Scroll back to bottom

	if !s.StickyScroll {
		t.Error("Scrolling to bottom should restore sticky")
	}
	if s.Offset != 40 {
		t.Errorf("Offset = %d, want 40", s.Offset)
	}
}

func TestScrollBy_ClampsOffset(t *testing.T) {
	s := New(10)
	s.SetContent(50)

	s.ScrollBy(-1000) // Try to scroll way past top
	if s.Offset != 0 {
		t.Errorf("Offset = %d, want 0 (clamped)", s.Offset)
	}

	s.ScrollBy(1000) // Try to scroll way past bottom
	if s.Offset != 40 {
		t.Errorf("Offset = %d, want 40 (clamped)", s.Offset)
	}
}

func TestPageUpDown(t *testing.T) {
	s := New(10)
	s.SetContent(50)

	s.PageUp()
	if s.Offset != 30 {
		t.Errorf("After PageUp, Offset = %d, want 30", s.Offset)
	}

	s.PageDown()
	if s.Offset != 40 {
		t.Errorf("After PageDown, Offset = %d, want 40", s.Offset)
	}
}

func TestScrollToTopBottom(t *testing.T) {
	s := New(10)
	s.SetContent(50)

	s.ScrollToTop()
	if s.Offset != 0 {
		t.Errorf("Offset = %d, want 0", s.Offset)
	}
	if s.StickyScroll {
		t.Error("ScrollToTop should break sticky")
	}

	s.ScrollToBottom()
	if s.Offset != 40 {
		t.Errorf("Offset = %d, want 40", s.Offset)
	}
	if !s.StickyScroll {
		t.Error("ScrollToBottom should enable sticky")
	}
}

func TestAtTopBottom(t *testing.T) {
	s := New(10)
	s.SetContent(50)

	if s.AtTop() {
		t.Error("Should not be at top when at bottom")
	}
	if !s.AtBottom() {
		t.Error("Should be at bottom")
	}

	s.ScrollToTop()

	if !s.AtTop() {
		t.Error("Should be at top")
	}
	if s.AtBottom() {
		t.Error("Should not be at bottom when at top")
	}
}

func TestVisibleRange(t *testing.T) {
	s := New(10)
	s.SetContent(50)
	s.ScrollToTop()

	start, end := s.VisibleRange()
	if start != 0 || end != 10 {
		t.Errorf("VisibleRange = (%d, %d), want (0, 10)", start, end)
	}

	s.ScrollToBottom()
	start, end = s.VisibleRange()
	if start != 40 || end != 50 {
		t.Errorf("VisibleRange = (%d, %d), want (40, 50)", start, end)
	}
}

func TestSmallContent(t *testing.T) {
	s := New(20)
	s.SetContent(10) // Content smaller than viewport

	if s.Offset != 0 {
		t.Errorf("Offset = %d, want 0 for small content", s.Offset)
	}

	s.ScrollBy(100)
	if s.Offset != 0 {
		t.Errorf("Offset = %d, want 0 (can't scroll small content)", s.Offset)
	}
}

func TestScrollTo(t *testing.T) {
	s := New(10)
	s.SetContent(50)

	s.ScrollTo(20)
	if s.Offset != 20 {
		t.Errorf("Offset = %d, want 20", s.Offset)
	}
	if s.StickyScroll {
		t.Error("ScrollTo middle should break sticky")
	}

	s.ScrollTo(40) // To bottom
	if !s.StickyScroll {
		t.Error("ScrollTo bottom should restore sticky")
	}
}
