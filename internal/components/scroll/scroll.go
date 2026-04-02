// Package scroll provides scroll state management with sticky scroll behavior.
package scroll

// ScrollState tracks scroll position and sticky behavior.
// When StickyScroll is true, new content automatically scrolls to bottom.
// Scrolling up breaks sticky mode; scrolling back to bottom restores it.
type ScrollState struct {
	Offset        int  // Current scroll offset from top
	ViewHeight    int  // Visible viewport height
	ContentHeight int  // Total content height
	StickyScroll  bool // Auto-follow new content when true
}

// New creates a ScrollState with sticky scroll enabled.
func New(viewHeight int) *ScrollState {
	return &ScrollState{
		ViewHeight:   viewHeight,
		StickyScroll: true,
	}
}

// SetViewHeight updates the viewport height.
func (s *ScrollState) SetViewHeight(height int) {
	s.ViewHeight = height
	s.clampOffset()
}

// SetContent updates content height, auto-scrolling if sticky.
func (s *ScrollState) SetContent(height int) {
	s.ContentHeight = height

	if s.StickyScroll {
		// Scroll to bottom
		s.Offset = s.maxOffset()
	} else {
		s.clampOffset()
	}
}

// ScrollBy moves offset by delta, managing sticky state.
// Negative delta = scroll up (breaks sticky).
// Positive delta = scroll down (may restore sticky at bottom).
func (s *ScrollState) ScrollBy(delta int) {
	if delta == 0 {
		return
	}

	// Scrolling up breaks sticky
	if delta < 0 {
		s.StickyScroll = false
	}

	s.Offset += delta
	s.clampOffset()

	// Scrolling to bottom restores sticky
	if s.Offset >= s.maxOffset() {
		s.StickyScroll = true
	}
}

// ScrollTo sets absolute offset position.
func (s *ScrollState) ScrollTo(offset int) {
	oldOffset := s.Offset
	s.Offset = offset
	s.clampOffset()

	// Determine if this was up or down movement
	if s.Offset < oldOffset {
		s.StickyScroll = false
	} else if s.Offset >= s.maxOffset() {
		s.StickyScroll = true
	}
}

// PageUp scrolls up by one page.
func (s *ScrollState) PageUp() {
	s.ScrollBy(-s.ViewHeight)
}

// PageDown scrolls down by one page.
func (s *ScrollState) PageDown() {
	s.ScrollBy(s.ViewHeight)
}

// ScrollToTop jumps to top, breaking sticky.
func (s *ScrollState) ScrollToTop() {
	s.Offset = 0
	s.StickyScroll = false
}

// ScrollToBottom jumps to bottom, enabling sticky.
func (s *ScrollState) ScrollToBottom() {
	s.Offset = s.maxOffset()
	s.StickyScroll = true
}

// AtTop returns true if scrolled to top.
func (s *ScrollState) AtTop() bool {
	return s.Offset <= 0
}

// AtBottom returns true if scrolled to bottom.
func (s *ScrollState) AtBottom() bool {
	return s.Offset >= s.maxOffset()
}

// IsSticky returns whether auto-follow is enabled.
func (s *ScrollState) IsSticky() bool {
	return s.StickyScroll
}

// VisibleRange returns the range of content lines visible.
// Returns (start, end) where end is exclusive.
func (s *ScrollState) VisibleRange() (int, int) {
	start := s.Offset
	end := s.Offset + s.ViewHeight
	if end > s.ContentHeight {
		end = s.ContentHeight
	}
	return start, end
}

// maxOffset returns the maximum valid scroll offset.
func (s *ScrollState) maxOffset() int {
	max := s.ContentHeight - s.ViewHeight
	if max < 0 {
		return 0
	}
	return max
}

// clampOffset ensures offset is within valid bounds.
func (s *ScrollState) clampOffset() {
	if s.Offset < 0 {
		s.Offset = 0
	}
	max := s.maxOffset()
	if s.Offset > max {
		s.Offset = max
	}
}
