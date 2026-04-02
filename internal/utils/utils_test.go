package utils

import (
	"testing"
	"time"
)

func TestCircularBufferBasic(t *testing.T) {
	cb := NewCircularBuffer[int](3)

	cb.Push(1)
	cb.Push(2)
	cb.Push(3)

	got := cb.ToSlice()
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Errorf("expected [1 2 3], got %v", got)
	}
}

func TestCircularBufferEviction(t *testing.T) {
	cb := NewCircularBuffer[int](3)
	cb.Push(1)
	cb.Push(2)
	cb.Push(3)
	cb.Push(4) // evicts 1

	got := cb.ToSlice()
	if len(got) != 3 || got[0] != 2 || got[1] != 3 || got[2] != 4 {
		t.Errorf("expected [2 3 4], got %v", got)
	}
}

func TestCircularBufferLast(t *testing.T) {
	cb := NewCircularBuffer[int](5)
	for i := 1; i <= 5; i++ {
		cb.Push(i)
	}

	last2 := cb.Last(2)
	if len(last2) != 2 || last2[0] != 4 || last2[1] != 5 {
		t.Errorf("expected [4 5], got %v", last2)
	}

	all := cb.Last(10) // more than stored
	if len(all) != 5 {
		t.Errorf("expected 5 items, got %d", len(all))
	}
}

func TestBufferedWriterFlushOnSize(t *testing.T) {
	received := make([][]int, 0)
	bw := NewBufferedWriter(func(items []int) {
		received = append(received, items)
	})
	bw.maxSize = 3
	bw.flushInterval = 10 * time.Second // large so timer never fires

	bw.Write(1)
	bw.Write(2)
	bw.Write(3) // triggers size flush

	time.Sleep(20 * time.Millisecond) // let goroutine run
	if len(received) == 0 {
		t.Error("expected flush to have been called")
	}
}

func TestBufferedWriterManualFlush(t *testing.T) {
	var got []int
	bw := NewBufferedWriter(func(items []int) {
		got = append(got, items...)
	})
	bw.Write(10)
	bw.Write(20)
	bw.Flush()
	time.Sleep(10 * time.Millisecond)
	if len(got) != 2 || got[0] != 10 || got[1] != 20 {
		t.Errorf("expected [10 20], got %v", got)
	}
}
