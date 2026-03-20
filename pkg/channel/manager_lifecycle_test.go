package channel

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestManagerStartRollbackUsesBoundedContext(t *testing.T) {
	manager := NewManager()
	first := &fakeChannel{name: "signal"}
	second := &fakeChannel{name: "whatsapp", startErr: errors.New("boom")}
	manager.Register(first)
	manager.Register(second)

	err := manager.Start(context.Background())
	if err == nil {
		t.Fatal("expected Start to fail")
	}
	if first.stops == 0 && second.stops == 0 {
		t.Fatal("expected at least one channel to be stopped on rollback")
	}
	if first.lastStopCtx == nil && second.lastStopCtx == nil {
		t.Fatal("expected stopped channels to receive non-nil context")
	}
}

func TestManagerStopDeliversBoundedContext(t *testing.T) {
	manager := NewManager()
	fake := &fakeChannel{name: "signal"}
	manager.Register(fake)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := manager.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if fake.lastStopCtx == nil {
		t.Fatal("expected Stop to be called with non-nil context")
	}
	if fake.lastStopCtx.Err() != nil {
		t.Fatalf("expected stop context not to be canceled yet, got %v", fake.lastStopCtx.Err())
	}
}
