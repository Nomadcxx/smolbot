package utils

import (
	"sync"
	"time"
)

// MinDisplayValue ensures a value is displayed for a minimum duration
// before being replaced. This prevents visual flicker when values change
// rapidly (e.g., progress indicators, status text).
//
// Example usage:
//
//	mdv := NewMinDisplayValue("Loading...", 500*time.Millisecond)
//	mdv.Set("Processing...")  // If called within 500ms, queued
//	current := mdv.Get()      // Returns "Loading..." until 500ms elapsed
type MinDisplayValue[T any] struct {
	current     T
	pending     *T
	displayedAt time.Time
	minDuration time.Duration
	mu          sync.RWMutex
}

// NewMinDisplayValue creates a MinDisplayValue with the given initial value
// and minimum display duration.
func NewMinDisplayValue[T any](initial T, minDuration time.Duration) *MinDisplayValue[T] {
	return &MinDisplayValue[T]{
		current:     initial,
		displayedAt: time.Now(),
		minDuration: minDuration,
	}
}

// Set updates the value. If the minimum display time hasn't elapsed,
// the new value is queued and will be applied on the next Get() call
// after the duration has passed.
func (m *MinDisplayValue[T]) Set(value T) {
	m.mu.Lock()
	defer m.mu.Unlock()

	elapsed := time.Since(m.displayedAt)

	if elapsed >= m.minDuration {
		// Enough time passed, update immediately
		m.current = value
		m.displayedAt = time.Now()
		m.pending = nil
	} else {
		// Queue for later
		m.pending = &value
	}
}

// Get returns the current display value. If a pending value exists and
// the minimum duration has elapsed, the pending value becomes current.
func (m *MinDisplayValue[T]) Get() T {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if pending value can now be shown
	if m.pending != nil && time.Since(m.displayedAt) >= m.minDuration {
		m.current = *m.pending
		m.displayedAt = time.Now()
		m.pending = nil
	}

	return m.current
}

// ForceSet immediately sets the value, ignoring the minimum duration.
// Use this for critical updates that must be shown immediately.
func (m *MinDisplayValue[T]) ForceSet(value T) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.current = value
	m.displayedAt = time.Now()
	m.pending = nil
}

// HasPending returns true if there's a queued value waiting to be displayed.
func (m *MinDisplayValue[T]) HasPending() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pending != nil
}

// TimeUntilUpdate returns the duration until a pending value can be shown.
// Returns 0 if no pending value or if it can be shown now.
func (m *MinDisplayValue[T]) TimeUntilUpdate() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.pending == nil {
		return 0
	}

	elapsed := time.Since(m.displayedAt)
	if elapsed >= m.minDuration {
		return 0
	}

	return m.minDuration - elapsed
}

// Reset clears any pending value and resets the display timer.
func (m *MinDisplayValue[T]) Reset(value T) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.current = value
	m.displayedAt = time.Now()
	m.pending = nil
}
