package utils

import (
	"sync"
	"time"
)

// BufferedWriter batches writes and flushes them in bulk either when
// the buffer reaches maxSize or flushInterval elapses since the first write.
type BufferedWriter[T any] struct {
	buffer        []T
	maxSize       int
	flushInterval time.Duration
	flushFn       func([]T)

	mu    sync.Mutex
	timer *time.Timer
}

// NewBufferedWriter creates a BufferedWriter with 100-item capacity and a
// 1000 ms flush window — matching Claude Code's bufferedWriter defaults.
func NewBufferedWriter[T any](flushFn func([]T)) *BufferedWriter[T] {
	return &BufferedWriter[T]{
		maxSize:       100,
		flushInterval: 1000 * time.Millisecond,
		flushFn:       flushFn,
	}
}

// Write adds an item to the buffer. If the buffer reaches maxSize it is
// flushed immediately; otherwise a timer is started to flush after flushInterval.
func (bw *BufferedWriter[T]) Write(item T) {
	bw.mu.Lock()
	defer bw.mu.Unlock()

	bw.buffer = append(bw.buffer, item)

	if len(bw.buffer) >= bw.maxSize {
		bw.flushLocked()
		return
	}

	if bw.timer == nil {
		bw.timer = time.AfterFunc(bw.flushInterval, func() {
			bw.mu.Lock()
			defer bw.mu.Unlock()
			bw.flushLocked()
		})
	}
}

// Flush forces an immediate flush of any buffered items.
func (bw *BufferedWriter[T]) Flush() {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	bw.flushLocked()
}

func (bw *BufferedWriter[T]) flushLocked() {
	if len(bw.buffer) == 0 {
		return
	}

	items := bw.buffer
	bw.buffer = nil

	if bw.timer != nil {
		bw.timer.Stop()
		bw.timer = nil
	}

	go bw.flushFn(items)
}
