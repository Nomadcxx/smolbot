package client

import "testing"

func TestCloseResetsLastSequence(t *testing.T) {
	c := New("ws://127.0.0.1/ws")
	c.lastSeq = 9

	c.Close()

	if c.lastSeq != 0 {
		t.Fatalf("expected close to reset sequence tracking, got %d", c.lastSeq)
	}
}
