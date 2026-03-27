package tool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRoutingTools(t *testing.T) {
	t.Run("message routes and preserves structured metadata", func(t *testing.T) {
		router := &fakeMessageRouter{}
		tool := NewMessageTool()
		raw, _ := json.Marshal(map[string]any{
			"channel": "slack",
			"chat_id": "C123",
			"content": "hello",
		})

		result, err := tool.Execute(context.Background(), raw, ToolContext{MessageRouter: router})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if router.calls != 1 {
			t.Fatalf("expected router to be called once, got %d", router.calls)
		}
		if router.channel != "slack" || router.chatID != "C123" || router.content != "hello" {
			t.Fatalf("unexpected routed payload: %#v", router)
		}
		if result.Metadata["channel"] != "slack" || result.Metadata["chatID"] != "C123" {
			t.Fatalf("expected structured metadata, got %#v", result.Metadata)
		}
	})

	t.Run("spawn delegates with reduced iteration limit and child session key", func(t *testing.T) {
		spawner := &fakeSpawner{}
		tool := NewSpawnTool(func() string { return "abc123" })
		raw, _ := json.Marshal(map[string]any{"message": "do the side quest"})

		result, err := tool.Execute(context.Background(), raw, ToolContext{SessionKey: "parent-session", Spawner: spawner})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if spawner.calls != 1 {
			t.Fatalf("expected spawner call, got %d", spawner.calls)
		}
		if spawner.req.ParentSessionKey != "parent-session" {
			t.Fatalf("unexpected parent session key %#v", spawner.req)
		}
		if spawner.req.ChildSessionKey != "spawn:parent-session:abc123" {
			t.Fatalf("unexpected child session key %#v", spawner.req)
		}
		if spawner.req.Prompt != "do the side quest" {
			t.Fatalf("unexpected prompt %#v", spawner.req)
		}
		if spawner.req.MaxIterations != 15 {
			t.Fatalf("expected reduced max iterations, got %d", spawner.req.MaxIterations)
		}
		if !containsString(spawner.req.DisabledTools, "message") || !containsString(spawner.req.DisabledTools, "spawn") {
			t.Fatalf("expected recursive tools to be disabled, got %#v", spawner.req.DisabledTools)
		}
		if result.Metadata["description"] != "do the side quest" {
			t.Fatalf("expected compatibility metadata, got %#v", result.Metadata)
		}
		if firstNonEmpty(result.Output, result.Content) != "child finished" {
			t.Fatalf("expected synchronous child output, got %#v", result)
		}
	})

	t.Run("task delegates with structured description and agent type", func(t *testing.T) {
		spawner := &fakeSpawner{}
		tool := NewTaskTool(func() string { return "child1" })
		raw, _ := json.Marshal(map[string]any{
			"description":      "Spec review Gate 6",
			"prompt":           "Review ONLY the Gate 6 changes in the current working tree.",
			"agent_type":       "explorer",
			"model":            "gpt-5.4",
			"reasoning_effort": "high",
		})

		result, err := tool.Execute(context.Background(), raw, ToolContext{SessionKey: "parent-session", Spawner: spawner})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if spawner.calls != 1 {
			t.Fatalf("expected spawner call, got %d", spawner.calls)
		}
		if spawner.req.Description != "Spec review Gate 6" {
			t.Fatalf("unexpected description %#v", spawner.req)
		}
		if spawner.req.AgentType != "explorer" {
			t.Fatalf("unexpected agent type %#v", spawner.req)
		}
		if spawner.req.Model != "gpt-5.4" || spawner.req.ReasoningEffort != "high" {
			t.Fatalf("unexpected task model metadata %#v", spawner.req)
		}
		if got := result.Metadata["agentType"]; got != "explorer" {
			t.Fatalf("expected agentType metadata, got %#v", result.Metadata)
		}
	})

	t.Run("wait delegates to spawner and preserves requested agent ids", func(t *testing.T) {
		spawner := &fakeSpawner{}
		tool := NewWaitTool()
		raw, _ := json.Marshal(map[string]any{
			"agent_ids": []string{"child-b", "child-a"},
		})

		result, err := tool.Execute(context.Background(), raw, ToolContext{SessionKey: "parent-session", Spawner: spawner})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if spawner.waitCalls != 1 {
			t.Fatalf("expected wait call, got %d", spawner.waitCalls)
		}
		if len(spawner.waitReq.AgentIDs) != 2 || spawner.waitReq.AgentIDs[0] != "child-b" {
			t.Fatalf("unexpected wait request %#v", spawner.waitReq)
		}
		if result.Metadata["count"] != 2 {
			t.Fatalf("expected wait count metadata, got %#v", result.Metadata)
		}
	})

	t.Run("cron rejects create inside cron context", func(t *testing.T) {
		service := &fakeCronService{}
		tool := NewCronTool(service)
		raw, _ := json.Marshal(map[string]any{
			"action":   "create",
			"name":     "Daily reminder",
			"timezone": "Australia/Melbourne",
		})

		result, err := tool.Execute(context.Background(), raw, ToolContext{IsCronContext: true})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if !strings.Contains(result.Error, "cron context") {
			t.Fatalf("expected cron-context guard, got %#v", result)
		}
		if service.calls != 0 {
			t.Fatalf("service should not be called, got %d", service.calls)
		}
	})

	t.Run("cron validates timezone and forwards crud actions", func(t *testing.T) {
		service := &fakeCronService{response: "ok"}
		tool := NewCronTool(service)

		invalidRaw, _ := json.Marshal(map[string]any{
			"action":   "create",
			"name":     "bad tz",
			"timezone": "Mars/Phobos",
		})
		result, err := tool.Execute(context.Background(), invalidRaw, ToolContext{})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if !strings.Contains(result.Error, "timezone") {
			t.Fatalf("expected timezone validation error, got %#v", result)
		}

		actions := []string{"create", "list", "enable", "disable", "update", "delete"}
		for _, action := range actions {
			raw, _ := json.Marshal(map[string]any{
				"action":    action,
				"id":        "job-1",
				"name":      "Daily reminder",
				"schedule":  "0 9 * * *",
				"timezone":  "Australia/Melbourne",
				"reminder":  "Check the queue",
				"channel":   "discord",
				"chat_id":   "channel-123",
				"isEnabled": true,
			})
			result, err := tool.Execute(context.Background(), raw, ToolContext{})
			if err != nil {
				t.Fatalf("Execute %s: %v", action, err)
			}
			if result.Error != "" {
				t.Fatalf("unexpected cron error for %s: %#v", action, result)
			}
			if service.last.Action != action {
				t.Fatalf("expected action %s, got %#v", action, service.last)
			}
			if service.last.Name != "Daily reminder" || service.last.Reminder != "Check the queue" {
				t.Fatalf("expected preserved cron payload, got %#v", service.last)
			}
		}
	})
}

func TestWaitToolDelegatesToSpawner(t *testing.T) {
	spawner := &fakeSpawner{}
	tool := NewWaitTool()
	raw, _ := json.Marshal(map[string]any{
		"agent_ids": []string{"child-b", "child-a"},
	})

	result, err := tool.Execute(context.Background(), raw, ToolContext{SessionKey: "parent-session", Spawner: spawner})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if spawner.waitCalls != 1 {
		t.Fatalf("expected wait call, got %d", spawner.waitCalls)
	}
	if len(spawner.waitReq.AgentIDs) != 2 || spawner.waitReq.AgentIDs[0] != "child-b" {
		t.Fatalf("unexpected wait request %#v", spawner.waitReq)
	}
	if result.Metadata["count"] != 2 {
		t.Fatalf("expected wait count metadata, got %#v", result.Metadata)
	}
}

type fakeMessageRouter struct {
	calls   int
	channel string
	chatID  string
	content string
}

func (f *fakeMessageRouter) Route(_ context.Context, channel, chatID, content string) error {
	f.calls++
	f.channel = channel
	f.chatID = chatID
	f.content = content
	return nil
}

type fakeSpawner struct {
	calls     int
	req       SpawnRequest
	waitCalls int
	waitReq   WaitRequest
}

func (f *fakeSpawner) Spawn(_ context.Context, req SpawnRequest) (*SpawnResult, error) {
	f.calls++
	f.req = req
	return &SpawnResult{
		ID:              req.ChildSessionKey,
		SessionKey:      req.ChildSessionKey,
		Name:            "Bernoulli",
		AgentType:       firstNonEmpty(req.AgentType, "explorer"),
		Model:           req.Model,
		ReasoningEffort: req.ReasoningEffort,
		Description:     req.Description,
		PromptPreview:   req.Prompt,
	}, nil
}

func (f *fakeSpawner) ProcessDirect(_ context.Context, req SpawnRequest) (string, error) {
	f.calls++
	f.req = req
	return "child finished", nil
}

func (f *fakeSpawner) Wait(_ context.Context, req WaitRequest) (*WaitResult, error) {
	f.waitCalls++
	f.waitReq = req
	return &WaitResult{
		Count: 2,
		Results: []WaitResultItem{
			{ID: "child-a", Name: "Bernoulli", AgentType: "explorer", Status: "completed", Summary: "ok"},
			{ID: "child-b", Name: "Averroes", AgentType: "explorer", Status: "completed", Summary: "ok"},
		},
	}, nil
}

type fakeCronService struct {
	calls    int
	last     CronRequest
	response string
}

func (f *fakeCronService) Handle(_ context.Context, req CronRequest) (string, error) {
	f.calls++
	f.last = req
	return f.response, nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
