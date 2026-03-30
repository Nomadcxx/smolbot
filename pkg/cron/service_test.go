package cron

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Nomadcxx/smolbot/pkg/agent"
	toolpkg "github.com/Nomadcxx/smolbot/pkg/tool"
)

func TestService(t *testing.T) {
	now := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
	jobsFile := filepath.Join(t.TempDir(), "jobs.json")
	processor := &fakeCronProcessor{result: "job output"}
	router := &fakeCronRouter{}
	evaluator := &fakeCronEvaluator{deliver: true}
	service := NewService(ServiceDeps{
		JobsFile:  jobsFile,
		Processor: processor,
		Evaluator: evaluator,
		Router:    router,
		Now:       func() time.Time { return now },
	})

	t.Run("create persists and list returns job", func(t *testing.T) {
		_, err := service.Handle(context.Background(), toolpkg.CronRequest{
			Action:   "create",
			Name:     "Daily reminder",
			Schedule: now.Add(-time.Minute).Format(time.RFC3339),
			Timezone: "UTC",
			Reminder: "Check the queue",
			Channel:  "slack",
			ChatID:   "C1",
			Enabled:  true,
		})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		data, err := os.ReadFile(jobsFile)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if !json.Valid(data) || len(service.ListJobs()) != 1 {
			t.Fatalf("expected persisted job list, got %s", data)
		}
	})

	t.Run("run due executes with cron context and session key", func(t *testing.T) {
		if err := service.RunDue(context.Background(), now); err != nil {
			t.Fatalf("RunDue: %v", err)
		}
		if processor.calls == 0 {
			t.Fatal("expected processor to run")
		}
		if processor.lastReq.SessionKey == "" || processor.lastReq.IsCronContext != true {
			t.Fatalf("expected cron session and context, got %#v", processor.lastReq)
		}
		if processor.lastReq.Content != "[Scheduled Task: Daily reminder] Check the queue" {
			t.Fatalf("unexpected reminder format %q", processor.lastReq.Content)
		}
		if len(service.ListJobs()[0].RecentRuns) == 0 {
			t.Fatalf("expected recent run history %#v", service.ListJobs()[0])
		}
		if router.calls != 1 || router.lastContent != "job output" {
			t.Fatalf("expected routed approved output, got %#v", router)
		}
	})

	t.Run("every schedule advances next run", func(t *testing.T) {
		_, err := service.Handle(context.Background(), toolpkg.CronRequest{
			Action:   "create",
			Name:     "Every minute",
			Schedule: "every 1m",
			Timezone: "UTC",
			Reminder: "Keep going",
			Channel:  "slack",
			ChatID:   "C2",
			Enabled:  true,
		})
		if err != nil {
			t.Fatalf("create every: %v", err)
		}
		job := findJobByName(service.ListJobs(), "Every minute")
		if !job.NextRun.After(now) {
			t.Fatalf("expected future next run, got %#v", job)
		}
	})

	t.Run("cron expression scheduling computes next run", func(t *testing.T) {
		_, err := service.Handle(context.Background(), toolpkg.CronRequest{
			Action:   "create",
			Name:     "Lunch",
			Schedule: "30 12 * * *",
			Timezone: "UTC",
			Reminder: "Lunch check",
			Enabled:  true,
		})
		if err != nil {
			t.Fatalf("create cron: %v", err)
		}
		job := findJobByName(service.ListJobs(), "Lunch")
		if job.NextRun.Hour() != 12 || job.NextRun.Minute() != 30 {
			t.Fatalf("unexpected cron next run %#v", job.NextRun)
		}
	})

	t.Run("same-target delivery skips evaluator and router", func(t *testing.T) {
		skipProcessor := &fakeCronProcessor{
			result: "already delivered",
			cbEvent: &agent.Event{
				Type:    agent.EventToolDone,
				Content: "message",
				Data: map[string]any{
					"deliveredToRequestTarget": true,
				},
			},
		}
		skipEvaluator := &fakeCronEvaluator{deliver: true}
		skipRouter := &fakeCronRouter{}
		skipService := NewService(ServiceDeps{
			JobsFile:  filepath.Join(t.TempDir(), "jobs.json"),
			Processor: skipProcessor,
			Evaluator: skipEvaluator,
			Router:    skipRouter,
			Now:       func() time.Time { return now },
		})
		_, err := skipService.Handle(context.Background(), toolpkg.CronRequest{
			Action:   "create",
			Name:     "Direct message",
			Schedule: now.Add(-time.Minute).Format(time.RFC3339),
			Timezone: "UTC",
			Reminder: "Ping directly",
			Channel:  "discord",
			ChatID:   "chan",
			Enabled:  true,
		})
		if err != nil {
			t.Fatalf("create direct: %v", err)
		}
		if err := skipService.RunDue(context.Background(), now); err != nil {
			t.Fatalf("RunDue: %v", err)
		}
		if skipEvaluator.calls != 0 || skipRouter.calls != 0 {
			t.Fatalf("expected evaluator/router skip, got %#v %#v", skipEvaluator, skipRouter)
		}
	})

	t.Run("crud update disable enable delete", func(t *testing.T) {
		jobs := service.ListJobs()
		id := jobs[0].ID
		if _, err := service.Handle(context.Background(), toolpkg.CronRequest{Action: "disable", ID: id}); err != nil {
			t.Fatalf("disable: %v", err)
		}
		if service.jobByID(id).Enabled {
			t.Fatal("expected disabled job")
		}
		if _, err := service.Handle(context.Background(), toolpkg.CronRequest{Action: "enable", ID: id}); err != nil {
			t.Fatalf("enable: %v", err)
		}
		if !service.jobByID(id).Enabled {
			t.Fatal("expected enabled job")
		}
		if _, err := service.Handle(context.Background(), toolpkg.CronRequest{
			Action:   "update",
			ID:       id,
			Name:     "Updated",
			Schedule: "every 5m",
			Timezone: "UTC",
			Reminder: "Updated reminder",
		}); err != nil {
			t.Fatalf("update: %v", err)
		}
		if service.jobByID(id).Name != "Updated" {
			t.Fatalf("expected updated job %#v", service.jobByID(id))
		}
		if _, err := service.Handle(context.Background(), toolpkg.CronRequest{Action: "delete", ID: id}); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if service.jobByID(id) != nil {
			t.Fatal("expected deleted job")
		}
	})
}

type fakeCronProcessor struct {
	calls   int
	lastReq agent.Request
	result  string
	cbEvent *agent.Event
}

func (f *fakeCronProcessor) ProcessDirect(ctx context.Context, req agent.Request, cb agent.EventCallback) (string, error) {
	f.calls++
	f.lastReq = req
	if f.cbEvent != nil && cb != nil {
		cb(*f.cbEvent)
	}
	return f.result, nil
}

type fakeCronEvaluator struct {
	calls   int
	deliver bool
}

func (f *fakeCronEvaluator) ShouldDeliver(context.Context, string) bool {
	f.calls++
	return f.deliver
}

type fakeCronRouter struct {
	calls       int
	lastChannel string
	lastChatID  string
	lastContent string
}

func (f *fakeCronRouter) Route(_ context.Context, channel, chatID, content string) error {
	f.calls++
	f.lastChannel = channel
	f.lastChatID = chatID
	f.lastContent = content
	return nil
}

func findJobByName(jobs []Job, name string) Job {
	for _, job := range jobs {
		if job.Name == name {
			return job
		}
	}
	return Job{}
}

func TestPersistIsAtomicNoTempFileLeftOnSuccess(t *testing.T) {
	dir := t.TempDir()
	jobsFile := filepath.Join(dir, "jobs.json")

	svc := &Service{
		jobsFile: jobsFile,
		jobs: []Job{
			{ID: "j1", Name: "daily-report", Schedule: "0 9 * * *", Enabled: true},
		},
	}

	if err := svc.persist(); err != nil {
		t.Fatalf("persist: %v", err)
	}

	if _, err := os.Stat(jobsFile + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("expected .tmp file to be cleaned up after successful persist")
	}

	data, err := os.ReadFile(jobsFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var jobs []Job
	if err := json.Unmarshal(data, &jobs); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(jobs) != 1 || jobs[0].ID != "j1" {
		t.Fatalf("unexpected jobs: %#v", jobs)
	}
}
