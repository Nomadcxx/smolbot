package utils

// CircularBuffer is a fixed-capacity ring buffer that evicts the oldest
// item when full. Thread-safety is the caller's responsibility.
type CircularBuffer[T any] struct {
	items    []T
	capacity int
	head     int
	size     int
}

// NewCircularBuffer creates an empty circular buffer with the given capacity.
func NewCircularBuffer[T any](capacity int) *CircularBuffer[T] {
	if capacity <= 0 {
		capacity = 1
	}
	return &CircularBuffer[T]{
		items:    make([]T, capacity),
		capacity: capacity,
	}
}

// Push adds an item to the buffer. If the buffer is full the oldest item is
// evicted to make room.
func (c *CircularBuffer[T]) Push(item T) {
	idx := (c.head + c.size) % c.capacity
	c.items[idx] = item

	if c.size < c.capacity {
		c.size++
	} else {
		c.head = (c.head + 1) % c.capacity
	}
}

// Len returns the number of items currently stored.
func (c *CircularBuffer[T]) Len() int { return c.size }

// Cap returns the maximum number of items the buffer can hold.
func (c *CircularBuffer[T]) Cap() int { return c.capacity }

// ToSlice returns all stored items in insertion order (oldest first).
func (c *CircularBuffer[T]) ToSlice() []T {
	result := make([]T, c.size)
	for i := range c.size {
		result[i] = c.items[(c.head+i)%c.capacity]
	}
	return result
}

// Last returns the n most recently added items. If n > Len(), all items are
// returned.
func (c *CircularBuffer[T]) Last(n int) []T {
	if n > c.size {
		n = c.size
	}
	result := make([]T, n)
	start := c.size - n
	for i := range n {
		result[i] = c.items[(c.head+start+i)%c.capacity]
	}
	return result
}
