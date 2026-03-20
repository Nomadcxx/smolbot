package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Nomadcxx/nanobot-go/pkg/agent"
	"github.com/Nomadcxx/nanobot-go/pkg/tool"
	"github.com/google/uuid"
)

type Processor interface {
	ProcessDirect(ctx context.Context, req agent.Request, cb agent.EventCallback) (string, error)
}

type Evaluator interface {
	ShouldDeliver(ctx context.Context, content string) bool
}

type Router interface {
	Route(ctx context.Context, channel, chatID, content string) error
}

type ServiceDeps struct {
	JobsFile  string
	Processor Processor
	Evaluator Evaluator
	Router    Router
	Now       func() time.Time
}

type Service struct {
	mu        sync.Mutex
	jobsFile  string
	processor Processor
	evaluator Evaluator
	router    Router
	now       func() time.Time
	jobs      []Job
}

type Job struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Schedule   string      `json:"schedule"`
	Timezone   string      `json:"timezone"`
	Reminder   string      `json:"reminder"`
	Channel    string      `json:"channel"`
	ChatID     string      `json:"chatID"`
	Enabled    bool        `json:"enabled"`
	NextRun    time.Time   `json:"nextRun"`
	RecentRuns []RunRecord `json:"recentRuns,omitempty"`
}

type RunRecord struct {
	At      time.Time `json:"at"`
	Success bool      `json:"success"`
}

func NewService(deps ServiceDeps) *Service {
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	s := &Service{
		jobsFile:  deps.JobsFile,
		processor: deps.Processor,
		evaluator: deps.Evaluator,
		router:    deps.Router,
		now:       now,
	}
	_ = s.load()
	return s
}

func (s *Service) Handle(ctx context.Context, req tool.CronRequest) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch req.Action {
	case "create":
		job, err := s.newJob(req)
		if err != nil {
			return "", err
		}
		s.jobs = append(s.jobs, job)
		return "created", s.persist()
	case "list":
		data, err := json.Marshal(s.jobs)
		return string(data), err
	case "enable":
		job := s.jobByIDLocked(req.ID)
		if job == nil {
			return "", fmt.Errorf("job %q not found", req.ID)
		}
		job.Enabled = true
		return "enabled", s.persist()
	case "disable":
		job := s.jobByIDLocked(req.ID)
		if job == nil {
			return "", fmt.Errorf("job %q not found", req.ID)
		}
		job.Enabled = false
		return "disabled", s.persist()
	case "update":
		job := s.jobByIDLocked(req.ID)
		if job == nil {
			return "", fmt.Errorf("job %q not found", req.ID)
		}
		updated, err := s.newJob(req)
		if err != nil {
			return "", err
		}
		updated.ID = job.ID
		updated.RecentRuns = job.RecentRuns
		*job = updated
		return "updated", s.persist()
	case "delete":
		for i, job := range s.jobs {
			if job.ID == req.ID {
				s.jobs = append(s.jobs[:i], s.jobs[i+1:]...)
				return "deleted", s.persist()
			}
		}
		return "", fmt.Errorf("job %q not found", req.ID)
	default:
		return "", fmt.Errorf("unsupported action %q", req.Action)
	}
}

func (s *Service) ListJobs() []Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]Job(nil), s.jobs...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Service) RunDue(ctx context.Context, now time.Time) error {
	s.mu.Lock()
	jobs := make([]Job, len(s.jobs))
	copy(jobs, s.jobs)
	s.mu.Unlock()

	for _, job := range jobs {
		if !job.Enabled || job.NextRun.After(now) {
			continue
		}
		if err := s.executeJob(ctx, job, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) executeJob(ctx context.Context, job Job, now time.Time) error {
	if s.processor == nil {
		return nil
	}

	deliveredToTarget := false
	result, err := s.processor.ProcessDirect(ctx, agent.Request{
		Content:       fmt.Sprintf("[Scheduled Task: %s] %s", job.Name, job.Reminder),
		SessionKey:    "cron:" + job.ID,
		Channel:       job.Channel,
		ChatID:        job.ChatID,
		IsCronContext: true,
	}, func(event agent.Event) {
		if event.Type != agent.EventToolDone {
			return
		}
		if flag, ok := event.Data["deliveredToRequestTarget"].(bool); ok && flag {
			deliveredToTarget = true
		}
	})

	s.mu.Lock()
	defer s.mu.Unlock()
	stored := s.jobByIDLocked(job.ID)
	if stored == nil {
		return nil
	}
	stored.RecentRuns = append(stored.RecentRuns, RunRecord{At: now, Success: err == nil})
	if len(stored.RecentRuns) > 10 {
		stored.RecentRuns = stored.RecentRuns[len(stored.RecentRuns)-10:]
	}
	nextRun, nextEnabled := nextRunForSchedule(stored.Schedule, stored.Timezone, now)
	stored.NextRun = nextRun
	if !nextEnabled {
		stored.Enabled = false
	}
	if err := s.persist(); err != nil {
		return err
	}
	if err != nil || deliveredToTarget || s.router == nil {
		return err
	}
	if s.evaluator != nil && !s.evaluator.ShouldDeliver(ctx, result) {
		return nil
	}
	return s.router.Route(ctx, stored.Channel, stored.ChatID, result)
}

func (s *Service) newJob(req tool.CronRequest) (Job, error) {
	nextRun, enabled := nextRunForSchedule(req.Schedule, req.Timezone, s.now())
	if req.Action == "create" && !enabled {
		enabled = req.Enabled
	}
	return Job{
		ID:       firstNonEmpty(req.ID, uuid.NewString()),
		Name:     req.Name,
		Schedule: req.Schedule,
		Timezone: req.Timezone,
		Reminder: req.Reminder,
		Channel:  req.Channel,
		ChatID:   req.ChatID,
		Enabled:  enabled,
		NextRun:  nextRun,
	}, nil
}

func nextRunForSchedule(schedule, timezone string, now time.Time) (time.Time, bool) {
	location := time.UTC
	if timezone != "" {
		if parsed, err := time.LoadLocation(timezone); err == nil {
			location = parsed
		}
	}
	localNow := now.In(location)
	schedule = strings.TrimSpace(schedule)

	if schedule == "" {
		return now, false
	}
	if strings.HasPrefix(strings.ToLower(schedule), "every ") {
		duration, err := time.ParseDuration(strings.TrimSpace(schedule[6:]))
		if err != nil {
			return now, false
		}
		return localNow.Add(duration), true
	}
	if at, err := time.Parse(time.RFC3339, schedule); err == nil {
		return at.In(location), true
	}

	fields := strings.Fields(schedule)
	if len(fields) == 5 {
		minute := parseCronField(fields[0], localNow.Minute())
		hour := parseCronField(fields[1], localNow.Hour())
		candidate := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), hour, minute, 0, 0, location)
		if !candidate.After(localNow) {
			candidate = candidate.Add(24 * time.Hour)
		}
		return candidate, true
	}
	return now, false
}

func parseCronField(field string, fallback int) int {
	if field == "*" {
		return fallback
	}
	var value int
	if _, err := fmt.Sscanf(field, "%d", &value); err == nil {
		return value
	}
	return fallback
}

func (s *Service) load() error {
	if s.jobsFile == "" {
		return nil
	}
	data, err := os.ReadFile(s.jobsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var jobs []Job
	if err := json.Unmarshal(data, &jobs); err != nil {
		return err
	}
	s.jobs = jobs
	return nil
}

func (s *Service) persist() error {
	if s.jobsFile == "" {
		return nil
	}
	if err := os.MkdirAll(filepathDir(s.jobsFile), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.jobs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.jobsFile, data, 0o644)
}

func (s *Service) jobByID(id string) *Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.jobByIDLocked(id)
}

func (s *Service) jobByIDLocked(id string) *Job {
	for i := range s.jobs {
		if s.jobs[i].ID == id {
			return &s.jobs[i]
		}
	}
	return nil
}

func filepathDir(path string) string {
	lastSlash := strings.LastIndex(path, string(os.PathSeparator))
	if lastSlash < 0 {
		return "."
	}
	return path[:lastSlash]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
