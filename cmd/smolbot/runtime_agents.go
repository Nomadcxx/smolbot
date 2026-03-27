package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Nomadcxx/smolbot/pkg/agent"
	"github.com/Nomadcxx/smolbot/pkg/tool"
)

var delegatedAgentNames = []string{
	"Bernoulli",
	"Averroes",
	"Curie",
	"Hopper",
	"Lovelace",
	"Noether",
	"Ramanujan",
	"Turing",
}

type runtimeChildRun struct {
	id               string
	parentSessionKey string
	sessionKey       string
	order            int
	name             string
	agentType        string
	model            string
	reasoningEffort  string
	description      string
	promptPreview    string
	summary          string
	err              error
	done             bool
	completed        chan struct{}
	cancel           context.CancelFunc
}

type runtimeChildSnapshot struct {
	ID              string
	Name            string
	AgentType       string
	Model           string
	ReasoningEffort string
	Description     string
	PromptPreview   string
	Summary         string
	Done            bool
	Error           string
}

func (s *runtimeSpawner) Spawn(ctx context.Context, req tool.SpawnRequest) (*tool.SpawnResult, error) {
	if s == nil {
		return nil, errors.New("spawner unavailable")
	}
	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	childID := firstNonEmpty(req.ChildSessionKey, req.ParentSessionKey)
	childSessionKey := firstNonEmpty(req.ChildSessionKey, childID)
	runChild, err := s.childProcessor()
	if err != nil {
		return nil, err
	}
	baseCtx := s.baseContext()
	childCtx, cancel := context.WithCancel(baseCtx)

	run := &runtimeChildRun{
		id:               childID,
		parentSessionKey: req.ParentSessionKey,
		sessionKey:       childSessionKey,
		order:            s.allocateChildOrder(req.ParentSessionKey),
		name:             s.allocateChildName(req.ParentSessionKey),
		agentType:        firstNonEmpty(strings.TrimSpace(req.AgentType), "explorer"),
		model:            strings.TrimSpace(req.Model),
		reasoningEffort:  strings.TrimSpace(req.ReasoningEffort),
		description:      normalizeChildSummary(req.Description),
		promptPreview:    summarizePrompt(req.Prompt),
		completed:        make(chan struct{}),
		cancel:           cancel,
	}
	s.registerRun(run)

	go func() {
		summary, err := runChild(childCtx, s.buildChildRequest(req, childSessionKey, run))
		s.finishRun(run, summary, err)
	}()

	return &tool.SpawnResult{
		ID:              run.id,
		SessionKey:      run.sessionKey,
		Name:            run.name,
		AgentType:       run.agentType,
		Model:           run.model,
		ReasoningEffort: run.reasoningEffort,
		Description:     firstNonEmpty(run.description, run.promptPreview),
		PromptPreview:   run.promptPreview,
	}, nil
}

func (s *runtimeSpawner) ProcessDirect(ctx context.Context, req tool.SpawnRequest) (string, error) {
	runChild, err := s.childProcessor()
	if err != nil {
		return "", err
	}
	summary, err := runChild(ctx, s.buildChildRequest(req, req.ChildSessionKey, &runtimeChildRun{
		model:           strings.TrimSpace(req.Model),
		reasoningEffort: strings.TrimSpace(req.ReasoningEffort),
	}))
	return strings.TrimSpace(summary), err
}

func (s *runtimeSpawner) Wait(ctx context.Context, req tool.WaitRequest) (*tool.WaitResult, error) {
	targets := s.waitTargets(req.ParentSessionKey, req.AgentIDs)
	if len(targets) == 0 {
		return &tool.WaitResult{}, nil
	}

	ids := make([]string, 0, len(targets))
	for _, run := range targets {
		ids = append(ids, run.id)
	}
	s.SetWaiting(req.ParentSessionKey, ids)
	defer s.ClearWaiting(req.ParentSessionKey, ids)

	for _, run := range targets {
		if run.done {
			continue
		}
		select {
		case <-run.completed:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	results := make([]tool.WaitResultItem, 0, len(targets))
	for _, run := range targets {
		results = append(results, tool.WaitResultItem{
			ID:            run.id,
			Name:          run.name,
			AgentType:     run.agentType,
			Status:        childRunStatus(run),
			Description:   run.description,
			PromptPreview: run.promptPreview,
			Summary:       run.summary,
			Error:         firstErrString(run.err),
		})
	}

	return &tool.WaitResult{
		Count:   len(results),
		Results: results,
	}, nil
}

func (s *runtimeSpawner) CleanupParent(parentSessionKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	children := s.childrenByParent[parentSessionKey]
	for id, run := range children {
		if run.cancel != nil && !run.done {
			run.cancel()
		}
		delete(s.runs, id)
	}
	delete(s.childrenByParent, parentSessionKey)
	delete(s.nameIdx, parentSessionKey)
	delete(s.orderIdx, parentSessionKey)
	delete(s.waiting, parentSessionKey)
}

func (s *runtimeSpawner) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, run := range s.runs {
		if run.cancel != nil && !run.done {
			run.cancel()
		}
	}
	s.runs = nil
	s.childrenByParent = nil
	s.nameIdx = nil
	s.orderIdx = nil
	s.waiting = nil
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
		s.ctx = nil
	}
}

func (s *runtimeSpawner) Outstanding(parentSessionKey string) []runtimeChildSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	children := s.childrenByParent[parentSessionKey]
	out := make([]runtimeChildSnapshot, 0, len(children))
	for _, run := range children {
		if run.done {
			continue
		}
		out = append(out, snapshotRun(run))
	}
	return out
}

func (s *runtimeSpawner) SetWaiting(parentSessionKey string, childIDs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.waiting == nil {
		s.waiting = make(map[string]map[string]int)
	}
	if len(childIDs) == 0 {
		delete(s.waiting, parentSessionKey)
		return
	}
	waiting := s.waiting[parentSessionKey]
	if waiting == nil {
		waiting = make(map[string]int, len(childIDs))
		s.waiting[parentSessionKey] = waiting
	}
	for _, id := range childIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		waiting[id]++
	}
}

func (s *runtimeSpawner) ClearWaiting(parentSessionKey string, childIDs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	waiting := s.waiting[parentSessionKey]
	if len(waiting) == 0 {
		delete(s.waiting, parentSessionKey)
		return
	}
	for _, id := range childIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if waiting[id] <= 1 {
			delete(waiting, id)
			continue
		}
		waiting[id]--
	}
	if len(waiting) == 0 {
		delete(s.waiting, parentSessionKey)
	}
}

func (s *runtimeSpawner) allocateChildName(parentSessionKey string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.nameIdx == nil {
		s.nameIdx = make(map[string]int)
	}
	if len(delegatedAgentNames) == 0 {
		s.nameIdx[parentSessionKey]++
		return fmt.Sprintf("Agent-%d", s.nameIdx[parentSessionKey])
	}
	idx := s.nameIdx[parentSessionKey]
	name := delegatedAgentNames[idx%len(delegatedAgentNames)]
	s.nameIdx[parentSessionKey] = idx + 1
	return name
}

func (s *runtimeSpawner) allocateChildOrder(parentSessionKey string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.orderIdx == nil {
		s.orderIdx = make(map[string]int)
	}
	order := s.orderIdx[parentSessionKey]
	s.orderIdx[parentSessionKey] = order + 1
	return order
}

func summarizePrompt(prompt string) string {
	prompt = normalizeChildSummary(prompt)
	if len(prompt) <= 160 {
		return prompt
	}
	return strings.TrimSpace(prompt[:157]) + "..."
}

func normalizeChildResult(summary string, err error) string {
	if err != nil {
		return normalizeChildSummary(err.Error())
	}
	return normalizeChildSummary(summary)
}

func normalizeChildSummary(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func (s *runtimeSpawner) childProcessor() (func(context.Context, agent.Request) (string, error), error) {
	if s == nil {
		return nil, errors.New("spawner unavailable")
	}
	if s.process != nil {
		return s.process, nil
	}
	if s.loop == nil {
		return nil, errors.New("spawner unavailable")
	}
	return func(ctx context.Context, req agent.Request) (string, error) {
		return s.loop.ProcessDirect(ctx, req, nil)
	}, nil
}

func (s *runtimeSpawner) baseContext() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ctx != nil {
		return s.ctx
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	return s.ctx
}

func (s *runtimeSpawner) buildChildRequest(req tool.SpawnRequest, sessionKey string, run *runtimeChildRun) agent.Request {
	return agent.Request{
		Content:         req.Prompt,
		SessionKey:      firstNonEmpty(sessionKey, req.ChildSessionKey),
		Model:           run.model,
		ReasoningEffort: run.reasoningEffort,
		MaxIterations:   req.MaxIterations,
		DisabledTools:   append([]string(nil), req.DisabledTools...),
	}
}

func (s *runtimeSpawner) registerRun(run *runtimeChildRun) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.runs == nil {
		s.runs = make(map[string]*runtimeChildRun)
	}
	if s.childrenByParent == nil {
		s.childrenByParent = make(map[string]map[string]*runtimeChildRun)
	}
	s.runs[run.id] = run
	children := s.childrenByParent[run.parentSessionKey]
	if children == nil {
		children = make(map[string]*runtimeChildRun)
		s.childrenByParent[run.parentSessionKey] = children
	}
	children[run.id] = run
}

func (s *runtimeSpawner) finishRun(run *runtimeChildRun, summary string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	run.summary = normalizeChildResult(summary, err)
	run.err = err
	run.done = true
	if run.cancel != nil {
		run.cancel()
		run.cancel = nil
	}
	close(run.completed)
}

func (s *runtimeSpawner) waitTargets(parentSessionKey string, ids []string) []*runtimeChildRun {
	s.mu.Lock()
	defer s.mu.Unlock()

	children := s.childrenByParent[parentSessionKey]
	if len(children) == 0 {
		return nil
	}

	var targets []*runtimeChildRun
	if len(ids) == 0 {
		for _, run := range children {
			if run.done {
				continue
			}
			targets = append(targets, run)
		}
	} else {
		seen := make(map[string]struct{}, len(ids))
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			run := children[id]
			if run == nil || run.done {
				continue
			}
			targets = append(targets, run)
		}
	}

	sort.Slice(targets, func(i, j int) bool {
		return targets[i].order < targets[j].order
	})
	return targets
}

func snapshotRun(run *runtimeChildRun) runtimeChildSnapshot {
	snap := runtimeChildSnapshot{
		ID:              run.id,
		Name:            run.name,
		AgentType:       run.agentType,
		Model:           run.model,
		ReasoningEffort: run.reasoningEffort,
		Description:     run.description,
		PromptPreview:   run.promptPreview,
		Summary:         run.summary,
		Done:            run.done,
	}
	if run.err != nil {
		snap.Error = run.err.Error()
	}
	return snap
}

func childRunStatus(run *runtimeChildRun) string {
	if run.err != nil {
		return "error"
	}
	if run.done {
		return "completed"
	}
	return "running"
}

func firstErrString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
