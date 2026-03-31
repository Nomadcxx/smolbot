package heartbeat

import (
	"context"
	"errors"
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/agent"
	"github.com/Nomadcxx/smolbot/pkg/provider"
)

func TestHeartbeat(t *testing.T) {
	t.Run("skip decision", func(t *testing.T) {
		service := NewService(ServiceDeps{
			Decider:   fakeDecider{decision: "skip"},
			Processor: &fakeProcessor{},
			Evaluator: fakeEvaluator{deliver: true},
			Router:    &fakeRouter{},
		})
		if err := service.RunOnce(context.Background()); err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		if service.processor.(*fakeProcessor).calls != 0 {
			t.Fatal("processor should not run on skip")
		}
	})

	t.Run("run decision executes agent loop", func(t *testing.T) {
		processor := &fakeProcessor{result: "heartbeat output"}
		router := &fakeRouter{}
		service := NewService(ServiceDeps{
			Decider:   fakeDecider{decision: "run"},
			Processor: processor,
			Evaluator: fakeEvaluator{deliver: true},
			Router:    router,
			Channel:   "slack",
			ChatID:    "C1",
		})
		if err := service.RunOnce(context.Background()); err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		if processor.calls != 1 {
			t.Fatalf("expected processor to run once, got %d", processor.calls)
		}
		if router.calls != 1 || router.lastContent != "heartbeat output" {
			t.Fatalf("expected routed heartbeat output, got %#v", router)
		}
	})

	t.Run("delivery suppressed when evaluator rejects", func(t *testing.T) {
		router := &fakeRouter{}
		service := NewService(ServiceDeps{
			Decider:   fakeDecider{decision: "run"},
			Processor: &fakeProcessor{result: "heartbeat output"},
			Evaluator: fakeEvaluator{deliver: false},
			Router:    router,
			Channel:   "slack",
			ChatID:    "C1",
		})
		if err := service.RunOnce(context.Background()); err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		if router.calls != 0 {
			t.Fatalf("router should not be called when evaluator rejects, got %#v", router)
		}
	})

	t.Run("fails open on decider error by skipping run", func(t *testing.T) {
		service := NewService(ServiceDeps{
			Decider:   fakeDecider{err: errors.New("boom")},
			Processor: &fakeProcessor{},
			Evaluator: fakeEvaluator{deliver: true},
			Router:    &fakeRouter{},
		})
		if err := service.RunOnce(context.Background()); err == nil {
			t.Fatal("expected error when decider fails, got nil")
		}
	})

	t.Run("provider decider uses provider chat", func(t *testing.T) {
		fakeProvider := &fakeHeartbeatChatProvider{
			resp: &provider.Response{Content: "run"},
		}
		decider := ProviderDecider{
			Provider:     fakeProvider,
			Model:        "gpt-test",
			SystemPrompt: "Heartbeat policy",
		}
		value, err := decider.Decide(context.Background())
		if err != nil {
			t.Fatalf("Decide: %v", err)
		}
		if value != "run" {
			t.Fatalf("unexpected decision %q", value)
		}
		if len(fakeProvider.requests) != 1 {
			t.Fatalf("expected one provider request, got %d", len(fakeProvider.requests))
		}
	})
}

func TestService_StructuredDecision(t *testing.T) {
	t.Run("malformed free text decision skips instead of running", func(t *testing.T) {
		processor := &fakeProcessor{}
		service := NewService(ServiceDeps{
			Decider:   fakeDecider{decision: "probably run"},
			Processor: processor,
			Evaluator: fakeEvaluator{deliver: true},
			Router:    &fakeRouter{},
		})
		if err := service.RunOnce(context.Background()); err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		if processor.calls != 0 {
			t.Fatal("processor should not run on malformed free text decision")
		}
	})

	t.Run("garbage decider output skips run", func(t *testing.T) {
		processor := &fakeProcessor{}
		service := NewService(ServiceDeps{
			Decider:   fakeDecider{decision: "∆∫∂ƒ"},
			Processor: processor,
			Evaluator: fakeEvaluator{deliver: true},
			Router:    &fakeRouter{},
		})
		if err := service.RunOnce(context.Background()); err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		if processor.calls != 0 {
			t.Fatal("processor should not run on garbage decision")
		}
	})

	t.Run("empty decider output skips run", func(t *testing.T) {
		processor := &fakeProcessor{}
		service := NewService(ServiceDeps{
			Decider:   fakeDecider{decision: ""},
			Processor: processor,
			Evaluator: fakeEvaluator{deliver: true},
			Router:    &fakeRouter{},
		})
		if err := service.RunOnce(context.Background()); err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		if processor.calls != 0 {
			t.Fatal("processor should not run on empty decision")
		}
	})

	t.Run("structured JSON run decision executes agent loop", func(t *testing.T) {
		fakeProvider := &fakeHeartbeatChatProvider{
			resp: &provider.Response{Content: `{"action":"run"}`},
		}
		decider := ProviderDecider{
			Provider:     fakeProvider,
			Model:        "claude-test",
			SystemPrompt: "Decide whether to run heartbeat.",
		}
		processor := &fakeProcessor{result: "heartbeat output"}
		router := &fakeRouter{}
		service := NewService(ServiceDeps{
			Decider:   decider,
			Processor: processor,
			Evaluator: fakeEvaluator{deliver: true},
			Router:    router,
			Channel:   "slack",
			ChatID:    "C1",
		})
		if err := service.RunOnce(context.Background()); err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		if processor.calls != 1 {
			t.Fatal("expected processor to run on structured run")
		}
		if router.calls != 1 {
			t.Fatal("expected router to deliver on structured run")
		}
	})

	t.Run("structured JSON skip decision suppresses run", func(t *testing.T) {
		fakeProvider := &fakeHeartbeatChatProvider{
			resp: &provider.Response{Content: `{"action":"skip"}`},
		}
		decider := ProviderDecider{
			Provider:     fakeProvider,
			Model:        "claude-test",
			SystemPrompt: "Decide.",
		}
		processor := &fakeProcessor{}
		router := &fakeRouter{}
		service := NewService(ServiceDeps{
			Decider:   decider,
			Processor: processor,
			Evaluator: fakeEvaluator{deliver: true},
			Router:    router,
			Channel:   "slack",
			ChatID:    "C1",
		})
		if err := service.RunOnce(context.Background()); err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		if processor.calls != 0 {
			t.Fatal("expected processor skip on structured skip")
		}
		if router.calls != 0 {
			t.Fatal("expected router skip on structured skip")
		}
	})

	t.Run("malformed JSON decision skips instead of running", func(t *testing.T) {
		fakeProvider := &fakeHeartbeatChatProvider{
			resp: &provider.Response{Content: "lol idk maybe run"},
		}
		decider := ProviderDecider{
			Provider:     fakeProvider,
			Model:        "claude-test",
			SystemPrompt: "Decide.",
		}
		processor := &fakeProcessor{result: "output"}
		router := &fakeRouter{}
		service := NewService(ServiceDeps{
			Decider:   decider,
			Processor: processor,
			Evaluator: fakeEvaluator{deliver: true},
			Router:    router,
			Channel:   "slack",
			ChatID:    "C1",
		})
		if err := service.RunOnce(context.Background()); err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		if processor.calls != 0 {
			t.Fatal("expected processor not to run on malformed decision")
		}
	})

	t.Run("provider decider non-boolean action value skips", func(t *testing.T) {
		fakeProvider := &fakeHeartbeatChatProvider{
			resp: &provider.Response{Content: `{"action":"maybe"}`},
		}
		decider := ProviderDecider{
			Provider:     fakeProvider,
			Model:        "claude-test",
			SystemPrompt: "Decide.",
		}
		processor := &fakeProcessor{}
		service := NewService(ServiceDeps{
			Decider:   decider,
			Processor: processor,
			Evaluator: fakeEvaluator{deliver: true},
			Router:    &fakeRouter{},
		})
		if err := service.RunOnce(context.Background()); err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		if processor.calls != 0 {
			t.Fatal("expected processor skip on non-boolean action")
		}
	})

	t.Run("provider decider provider error skips run", func(t *testing.T) {
		fakeProvider := &fakeHeartbeatChatProvider{
			err: errors.New("network failure"),
		}
		decider := ProviderDecider{
			Provider:     fakeProvider,
			Model:        "claude-test",
			SystemPrompt: "Decide.",
		}
		processor := &fakeProcessor{}
		service := NewService(ServiceDeps{
			Decider:   decider,
			Processor: processor,
			Evaluator: fakeEvaluator{deliver: true},
			Router:    &fakeRouter{},
		})
		if err := service.RunOnce(context.Background()); err == nil {
			t.Fatal("expected error when provider decider fails")
		}
		if processor.calls != 0 {
			t.Fatal("expected processor skip on provider error")
		}
	})
}

type fakeDecider struct {
	decision string
	err      error
}

func (f fakeDecider) Decide(context.Context) (string, error) {
	return f.decision, f.err
}

type fakeProcessor struct {
	calls  int
	result string
}

func (f *fakeProcessor) ProcessDirect(context.Context, agent.Request, agent.EventCallback) (string, error) {
	f.calls++
	return f.result, nil
}

type fakeEvaluator struct{ deliver bool }

func (f fakeEvaluator) ShouldDeliver(context.Context, string) bool { return f.deliver }

type fakeRouter struct {
	calls       int
	lastChannel string
	lastChatID  string
	lastContent string
}

func (f *fakeRouter) Route(_ context.Context, channel, chatID, content string) error {
	f.calls++
	f.lastChannel = channel
	f.lastChatID = chatID
	f.lastContent = content
	return nil
}

type fakeHeartbeatChatProvider struct {
	requests []provider.ChatRequest
	resp     *provider.Response
	err      error
}

func (f *fakeHeartbeatChatProvider) Chat(_ context.Context, req provider.ChatRequest) (*provider.Response, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func (f *fakeHeartbeatChatProvider) ChatStream(context.Context, provider.ChatRequest) (*provider.Stream, error) {
	return nil, errors.New("unexpected ChatStream call")
}

func (f *fakeHeartbeatChatProvider) Name() string { return "openai" }
