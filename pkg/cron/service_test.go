package cron

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
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

	t.Run("RunDue continues executing remaining jobs after one fails", func(t *testing.T) {
		now2 := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
		errProcessor := &fakeCronProcessor{
			result:    "ok",
			errOnCall: 1,
			callErr:   errors.New("job-a exploded"),
		}
		svc2 := NewService(ServiceDeps{
			JobsFile:  filepath.Join(t.TempDir(), "jobs2.json"),
			Processor: errProcessor,
			Router:    &fakeCronRouter{},
			Evaluator: &fakeCronEvaluator{deliver: false},
			Now:       func() time.Time { return now2 },
		})
		for _, name := range []string{"job-a", "job-b"} {
			if _, err := svc2.Handle(context.Background(), toolpkg.CronRequest{
				Action:   "create",
				Name:     name,
				Schedule: now2.Add(-time.Minute).Format(time.RFC3339),
				Timezone: "UTC",
				Reminder: "ping",
				Enabled:  true,
			}); err != nil {
				t.Fatalf("create %s: %v", name, err)
			}
		}
		_ = svc2.RunDue(context.Background(), now2)
		if errProcessor.calls != 2 {
			t.Fatalf("expected 2 processor calls (both jobs run despite first error), got %d", errProcessor.calls)
		}
	})

	t.Run("RunDue skips a job that is already running from a prior cycle", func(t *testing.T) {
		now3 := time.Date(2026, 3, 20, 11, 0, 0, 0, time.UTC)
		started := make(chan struct{}, 1)
		unblock := make(chan struct{})
		blockP := &blockingCronProcessor{started: started, wait: unblock}
		svc3 := NewService(ServiceDeps{
			JobsFile:  filepath.Join(t.TempDir(), "jobs3.json"),
			Processor: blockP,
			Router:    &fakeCronRouter{},
			Evaluator: &fakeCronEvaluator{deliver: false},
			Now:       func() time.Time { return now3 },
		})
		if _, err := svc3.Handle(context.Background(), toolpkg.CronRequest{
			Action:   "create",
			Name:     "slow-job",
			Schedule: now3.Add(-time.Minute).Format(time.RFC3339),
			Timezone: "UTC",
			Reminder: "slow",
			Enabled:  true,
		}); err != nil {
			t.Fatalf("create: %v", err)
		}

		done1 := make(chan error, 1)
		go func() { done1 <- svc3.RunDue(context.Background(), now3) }()
		<-started

		if err := svc3.RunDue(context.Background(), now3); err != nil {
			t.Fatalf("second RunDue: %v", err)
		}
		if blockP.calls != 1 {
			t.Fatalf("expected 1 processor call after second RunDue, got %d", blockP.calls)
		}

		close(unblock)
		if err := <-done1; err != nil {
			t.Fatalf("first RunDue: %v", err)
		}
	})
}

type fakeCronProcessor struct {
	calls     int
	lastReq   agent.Request
	result    string
	cbEvent   *agent.Event
	errOnCall int
	callErr   error
}

func (f *fakeCronProcessor) ProcessDirect(ctx context.Context, req agent.Request, cb agent.EventCallback) (string, error) {
	f.calls++
	f.lastReq = req
	if f.cbEvent != nil && cb != nil {
		cb(*f.cbEvent)
	}
	if f.errOnCall > 0 && f.calls == f.errOnCall {
		return "", f.callErr
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

type blockingCronProcessor struct {
	mu      sync.Mutex
	calls   int
	started chan struct{}
	wait    chan struct{}
}

func (b *blockingCronProcessor) ProcessDirect(_ context.Context, _ agent.Request, _ agent.EventCallback) (string, error) {
	b.mu.Lock()
	b.calls++
	b.mu.Unlock()
	b.started <- struct{}{}
	<-b.wait
	return "done", nil
}
