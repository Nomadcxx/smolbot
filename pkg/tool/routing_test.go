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
		if spawner.req.MaxIterations != 15 {
			t.Fatalf("expected reduced max iterations, got %d", spawner.req.MaxIterations)
		}
		if !containsString(spawner.req.DisabledTools, "message") || !containsString(spawner.req.DisabledTools, "spawn") {
			t.Fatalf("expected recursive tools to be disabled, got %#v", spawner.req.DisabledTools)
		}
		if !strings.Contains(firstNonEmpty(result.Output, result.Content), "spawn:parent-session:abc123") {
			t.Fatalf("expected child session key in output, got %#v", result)
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
	calls int
	req   SpawnRequest
}

func (f *fakeSpawner) ProcessDirect(_ context.Context, req SpawnRequest) (string, error) {
	f.calls++
	f.req = req
	return "child finished", nil
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
