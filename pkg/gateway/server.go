package gateway

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Nomadcxx/smolbot/pkg/agent"
	"github.com/Nomadcxx/smolbot/pkg/channel"
	"github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/Nomadcxx/smolbot/pkg/cron"
	"github.com/Nomadcxx/smolbot/pkg/provider"
	"github.com/Nomadcxx/smolbot/pkg/session"
	"github.com/Nomadcxx/smolbot/pkg/skill"
	"github.com/Nomadcxx/smolbot/pkg/usage"
	"github.com/gorilla/websocket"
)

type CronLister interface {
	ListJobs() []cron.Job
}

type AgentProcessor interface {
	ProcessDirect(ctx context.Context, req agent.Request, cb agent.EventCallback) (string, error)
	CancelSession(sessionKey string)
}

type UsageSummaryReader interface {
	CurrentProviderSummary(sessionKey, providerID, modelName string, now time.Time) (usage.ProviderSummary, error)
}

type ServerDeps struct {
	Agent            AgentProcessor
	Cron             CronLister
	Sessions         *session.Store
	Channels         *channel.Manager
	Config           *config.Config
	Usage            UsageSummaryReader
	Skills           *skill.Registry
	Version          string
	StartedAt        time.Time
	SetModelCallback func(model string) error
}

type Server struct {
	agent            AgentProcessor
	cron             CronLister
	sessions         *session.Store
	channels         *channel.Manager
	config           *config.Config
	usage            UsageSummaryReader
	skills           *skill.Registry
	version          string
	started          time.Time
	setModelCallback func(model string) error

	upgrader          websocket.Upgrader
	connectedClients  atomic.Int64
	mu                sync.Mutex
	startingSessions  map[string]struct{}
	activeRuns        map[string]*runState
	sessionRuns       map[string]string
	wsTasks           map[*websocket.Conn]map[string]struct{}
	completedDelivery map[string]bool
	clients           map[*websocket.Conn]*clientState
	ollamaMu          sync.Mutex
	ollamaContext     map[string]ollamaContextCacheEntry
	lastUsage         struct {
		PromptTokens     int
		CompletionTokens int
		TotalTokens      int
	}
}

type clientState struct {
	conn       *websocket.Conn
	mu         sync.Mutex
	seq        int64
	isLegacy   bool
	sessionKey string
}

type runState struct {
	runID      string
	sessionKey string
	cancel     context.CancelFunc
	owner      *clientState
}

type ollamaContextCacheEntry struct {
	value     int
	found     bool
	expiresAt time.Time
}

type chatSendParams struct {
	Session string       `json:"session"`
	Message string       `json:"message"`
	Channel string       `json:"channel"`
	ChatID  string       `json:"chatID"`
	Media   []mediaInput `json:"media"`
}

type mediaInput struct {
	Data     string `json:"data"`
	MimeType string `json:"mimeType"`
}

func NewServer(deps ServerDeps) *Server {
	started := deps.StartedAt
	if started.IsZero() {
		started = time.Now()
	}
	return &Server{
		agent:            deps.Agent,
		cron:             deps.Cron,
		sessions:         deps.Sessions,
		channels:         deps.Channels,
		config:           deps.Config,
		usage:            deps.Usage,
		skills:           deps.Skills,
		version:          firstNonEmptyString(deps.Version, "dev"),
		started:          started,
		setModelCallback: deps.SetModelCallback,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(*http.Request) bool { return true },
		},
		startingSessions:  make(map[string]struct{}),
		activeRuns:        make(map[string]*runState),
		sessionRuns:       make(map[string]string),
		wsTasks:           make(map[*websocket.Conn]map[string]struct{}),
		completedDelivery: make(map[string]bool),
		clients:           make(map[*websocket.Conn]*clientState),
		ollamaContext:     make(map[string]ollamaContextCacheEntry),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ws", s.handleWS)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer conn.Close()
	s.connectedClients.Add(1)
	defer s.connectedClients.Add(-1)
	state := &clientState{conn: conn}
	s.mu.Lock()
	s.clients[conn] = state
	s.wsTasks[conn] = make(map[string]struct{})
	s.mu.Unlock()
	defer func() {
		s.cancelWsTasks(conn)
		s.mu.Lock()
		delete(s.clients, conn)
		delete(s.wsTasks, conn)
		s.mu.Unlock()
	}()

	// Enable ping/pong keepalive
	conn.SetPongHandler(func(string) error {
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start ping goroutine to keep connection alive
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		frame, err := DecodeFrame(data)
		if err != nil {
			_ = s.writeError(state, "", "bad_request", err.Error())
			continue
		}
		if frame.Kind != FrameRequest {
			_ = s.writeError(state, "", "bad_request", "expected request frame")
			continue
		}
		if frame.IsLegacy {
			state.isLegacy = true
		}

		resp, err := s.handleRequest(r.Context(), state, frame.Request)
		if err != nil {
			_ = s.writeError(state, frame.Request.ID, "bad_request", err.Error())
			continue
		}
		if resp == nil {
			continue
		}
		if err := s.writeResponse(state, frame.Request.ID, resp); err != nil {
			return
		}
	}
}

func (s *Server) handleRequest(ctx context.Context, client *clientState, req RequestFrame) (any, error) {
	switch req.Method {
	case "hello":
		return map[string]any{
			"server":   "smolbot",
			"version":  s.version,
			"protocol": 1,
			"methods": []string{
				"hello", "status", "chat.send", "chat.history", "sessions.list", "sessions.reset", "models.list", "models.set", "compact", "skills.list", "mcps.list", "cron.list",
			},
			"events": []string{"chat.progress", "chat.done", "chat.error", "chat.tool.start", "chat.tool.done", "chat.thinking", "chat.thinking.done", "chat.usage", "agent.spawned", "agent.completed", "agent.wait.started", "agent.wait.completed", "compact.start", "compact.done", "context.compressed"},
		}, nil
	case "status":
		params := struct {
			Session string `json:"session"`
		}{}
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				return nil, fmt.Errorf("parse status params: %w", err)
			}
		}
		sessionKey := params.Session
		if sessionKey == "" && client != nil {
			sessionKey = client.sessionKey
		}

		var channels []map[string]string
		if s.channels != nil {
			for name, status := range s.channels.Statuses(ctx) {
				channels = append(channels, map[string]string{
					"name":   name,
					"status": status.State,
				})
			}
		}
		payload := map[string]any{
			"model":    s.currentModel(),
			"provider": s.currentProvider(),
			"uptime":   int(time.Since(s.started).Seconds()),
			"channels": channels,
			"usage": map[string]any{
				"promptTokens":     s.lastUsage.PromptTokens,
				"completionTokens": s.lastUsage.CompletionTokens,
				"totalTokens":      s.lastUsage.TotalTokens,
				"contextWindow":    s.contextWindowTokens(ctx),
			},
		}
		if sessionKey != "" {
			payload["session"] = sessionKey
		}
		if s.usage != nil && sessionKey != "" {
			providerID := s.currentProvider()
			summary, err := s.usage.CurrentProviderSummary(
				sessionKey,
				providerID,
				statusSummaryModel(providerID, s.currentModel()),
				time.Now().UTC(),
			)
			if err == nil && summary.ProviderID != "" {
				payload["persistedUsage"] = map[string]any{
					"providerId":    summary.ProviderID,
					"modelName":     summary.ModelName,
					"sessionTokens": summary.SessionTokens,
					"todayTokens":   summary.TodayTokens,
					"weeklyTokens":  summary.WeeklyTokens,
					"budgetStatus":  summary.BudgetStatus,
					"warningLevel":  summary.WarningLevel,
				}
				if alert, ok := usageAlertPayload(summary); ok {
					payload["usageAlert"] = alert
				}
			}
		}
		return payload, nil
	case "cron.list":
		jobs := make([]map[string]any, 0)
		if s.cron != nil {
			for _, job := range s.cron.ListJobs() {
				status := "paused"
				if job.Enabled {
					status = "active"
				}
				nextRun := ""
				if !job.NextRun.IsZero() {
					nextRun = job.NextRun.Format(time.RFC3339)
				}
				jobs = append(jobs, map[string]any{
					"id":       job.ID,
					"name":     job.Name,
					"schedule": job.Schedule,
					"status":   status,
					"nextRun":  nextRun,
				})
			}
		}
		return map[string]any{"jobs": jobs}, nil
	case "chat.history":
		params := struct {
			Session string `json:"session"`
			Limit   int    `json:"limit"`
		}{}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, fmt.Errorf("parse chat.history params: %w", err)
		}
		limit := params.Limit
		if limit <= 0 {
			limit = 200
		}
		history, err := s.sessions.GetHistory(params.Session, limit)
		if err != nil {
			return nil, err
		}
		items := make([]map[string]any, 0, len(history))
		for _, msg := range history {
			items = append(items, map[string]any{
				"role":    msg.Role,
				"content": messageText(msg),
			})
		}
		return map[string]any{"messages": items}, nil
	case "chat.send":
		params := chatSendParams{}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, fmt.Errorf("parse chat.send params: %w", err)
		}
		if s.agent == nil {
			return nil, fmt.Errorf("agent unavailable")
		}
		request, err := buildAgentRequest(params)
		if err != nil {
			return nil, err
		}
		client.sessionKey = request.SessionKey
		runID, err := s.startRun(request, client)
		if err != nil {
			return nil, err
		}
		return map[string]any{"runId": runID}, nil
	case "chat.abort":
		params := struct {
			RunID string `json:"runId"`
		}{}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, fmt.Errorf("parse chat.abort params: %w", err)
		}
		if err := s.abortRun(params.RunID); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	case "sessions.list":
		if s.sessions == nil {
			return []any{}, nil
		}
		sessions, err := s.sessions.ListSessions()
		if err != nil {
			return nil, err
		}
		items := make([]map[string]any, 0, len(sessions))
		for _, item := range sessions {
			items = append(items, map[string]any{
				"key":       item.Key,
				"updatedAt": item.UpdatedAt.Format(time.RFC3339),
			})
		}
		return map[string]any{"sessions": items}, nil
	case "sessions.reset":
		params := struct {
			Session string `json:"session"`
		}{}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, fmt.Errorf("parse sessions.reset params: %w", err)
		}
		if err := s.sessions.ClearSession(params.Session); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	case "models.list":
		models, err := provider.GetAvailableModels(s.config)
		if err != nil {
			return nil, fmt.Errorf("get available models: %w", err)
		}
		modelList := make([]map[string]any, len(models))
		for i, m := range models {
			modelList[i] = map[string]any{
				"id":       m.ID,
				"name":     m.Name,
				"provider": m.Provider,
			}
		}
		return map[string]any{
			"models":  modelList,
			"current": s.currentModel(),
		}, nil
	case "compact":
		params := struct {
			Session string `json:"session"`
		}{}
		if len(req.Params) != 0 {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				return nil, fmt.Errorf("parse compact params: %w", err)
			}
		}
		sessionKey := params.Session
		if sessionKey == "" {
			sessionKey = client.sessionKey
		}
		if sessionKey == "" {
			return nil, fmt.Errorf("compact requires session")
		}
		if s.sessions == nil {
			return nil, fmt.Errorf("session store unavailable")
		}
		history, err := s.sessions.GetHistory(sessionKey, 500)
		if err != nil {
			return nil, err
		}
		result := map[string]any{
			"session":          sessionKey,
			"compacted":        false,
			"originalTokens":   0,
			"compressedTokens": 0,
			"reductionPercent": 0,
		}
		if s.config != nil && !s.config.Agents.Defaults.Compression.Enabled {
			result["reason"] = "compression disabled"
			return result, nil
		}
		if len(history) < 4 {
			result["reason"] = "not enough history"
			return result, nil
		}
		compactor, ok := s.agent.(interface {
			CompactNow(context.Context, string) (int, int, float64, error)
		})
		if !ok {
			return nil, fmt.Errorf("agent does not support manual compaction")
		}
		client.sessionKey = sessionKey
		s.emitEvent(client, "compact.start", map[string]any{"session": sessionKey})
		original, compressed, pct, err := compactor.CompactNow(ctx, sessionKey)
		if err != nil {
			return nil, err
		}
		if original == 0 || compressed == 0 || pct <= 0 {
			result["reason"] = "no reduction achieved"
			s.emitEvent(client, "compact.done", result)
			return result, nil
		}
		payload := map[string]any{
			"session":          sessionKey,
			"compacted":        true,
			"originalTokens":   original,
			"compressedTokens": compressed,
			"reductionPercent": pct,
		}
		s.emitEvent(client, "compact.done", payload)
		return payload, nil
	case "skills.list":
		skills := make([]map[string]any, 0)
		if s.skills != nil {
			for _, name := range s.skills.Names() {
				entry, ok := s.skills.Get(name)
				if !ok {
					continue
				}
				status := "available"
				if entry.Always {
					status = "always"
				}
				if !entry.Available {
					status = "unavailable"
				}
				skills = append(skills, map[string]any{
					"name":        entry.Name,
					"description": entry.Description,
					"status":      status,
				})
			}
		}
		return map[string]any{"skills": skills}, nil
	case "mcps.list":
		servers := make([]map[string]any, 0)
		if s.config != nil {
			names := make([]string, 0, len(s.config.Tools.MCPServers))
			for name := range s.config.Tools.MCPServers {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				cfg := s.config.Tools.MCPServers[name]
				command := strings.TrimSpace(strings.Join(append([]string{cfg.Command}, cfg.Args...), " "))
				servers = append(servers, map[string]any{
					"name":    name,
					"command": command,
					"status":  "configured",
				})
			}
		}
		return map[string]any{"servers": servers}, nil
	case "models.set":
		params := struct {
			Model string `json:"model"`
		}{}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, fmt.Errorf("parse models.set params: %w", err)
		}
		previous := s.currentModel()
		if s.config != nil {
			s.config.Agents.Defaults.Model = params.Model
		}
		if s.setModelCallback != nil {
			if err := s.setModelCallback(params.Model); err != nil {
				return nil, err
			}
		}
		return map[string]any{"previous": previous}, nil
	default:
		return nil, fmt.Errorf("unknown method %q", req.Method)
	}
}

func (s *Server) writeResponse(client *clientState, id string, payload any) error {
	result, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if client.isLegacy {
		frame, err := EncodeLegacyResponse(ResponseFrame{ID: id, Result: result, OK: true})
		if err != nil {
			return err
		}
		return client.write(frame)
	}
	frame, err := EncodeResponse(ResponseFrame{ID: id, Result: result})
	if err != nil {
		return err
	}
	return client.write(frame)
}

func (s *Server) writeError(client *clientState, id, code, message string) error {
	frame, err := EncodeError(ResponseFrame{
		ID: id,
		Error: &ErrorFrame{
			Code:    code,
			Message: message,
		},
	})
	if err != nil {
		return err
	}
	if client.isLegacy {
		legacyFrame, err := EncodeLegacyResponse(ResponseFrame{
			ID: id,
			Error: &ErrorFrame{
				Code:    code,
				Message: message,
			},
		})
		if err != nil {
			return err
		}
		return client.write(legacyFrame)
	}
	return client.write(frame)
}

func buildAgentRequest(params chatSendParams) (agent.Request, error) {
	req := agent.Request{
		Content:    params.Message,
		SessionKey: params.Session,
		Channel:    params.Channel,
		ChatID:     params.ChatID,
	}
	for _, media := range params.Media {
		data, err := base64.StdEncoding.DecodeString(media.Data)
		if err != nil {
			return req, fmt.Errorf("decode media: %w", err)
		}
		req.Media = append(req.Media, agent.MediaAttachment{
			Data:     data,
			MimeType: media.MimeType,
		})
	}
	return req, nil
}

func messageText(msg provider.Message) string {
	switch value := msg.Content.(type) {
	case string:
		return value
	case []provider.ContentBlock:
		var parts []string
		for _, item := range value {
			if strings.TrimSpace(item.Text) != "" {
				parts = append(parts, item.Text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return fmt.Sprint(value)
	}
}

func (s *Server) currentModel() string {
	if s.config == nil {
		return ""
	}
	return s.config.Agents.Defaults.Model
}

func (s *Server) currentProvider() string {
	if s.config == nil {
		return ""
	}
	model := s.config.Agents.Defaults.Model
	if model == "" {
		return s.config.Agents.Defaults.Provider
	}
	detected := detectProviderFromModel(model, s.config.Agents.Defaults.Provider)
	return detected
}

func detectProviderFromModel(model, fallbackProvider string) string {
	lower := strings.ToLower(model)
	if strings.HasPrefix(lower, "claude-") || strings.Contains(lower, "anthropic") {
		return "anthropic"
	}
	if strings.HasPrefix(lower, "gpt-") || strings.Contains(lower, "openai") {
		return "openai"
	}
	if strings.HasPrefix(lower, "azure/") {
		return "azure_openai"
	}
	if strings.Contains(lower, "ollama") {
		return "ollama"
	}
	return fallbackProvider
}

func (s *Server) contextWindowTokens(ctx context.Context) int {
	fallback := s.configContextWindowTokens()
	if s.currentProvider() != "ollama" {
		return fallback
	}

	model := normalizeOllamaModelID(s.currentModel())
	if model == "" {
		return fallback
	}

	if window, ok := s.cachedOllamaContextWindow(model); ok {
		if window.found && window.value > 0 {
			return window.value
		}
		return fallback
	}

	lookupCtx, cancel := context.WithTimeout(ctx, ollamaMetadataLookupTimeout)
	defer cancel()

	client := provider.NewOllamaClient(s.ollamaBaseURL())
	window, err := client.ContextWindow(lookupCtx, model)
	if err != nil || !window.Found || window.Value <= 0 {
		return fallback
	}
	s.storeOllamaContextWindow(model, window)
	return window.Value
}

const ollamaMetadataLookupTimeout = 250 * time.Millisecond
const ollamaContextCacheTTL = time.Minute

func (s *Server) cachedOllamaContextWindow(model string) (ollamaContextCacheEntry, bool) {
	key := s.ollamaCacheKey(model)

	s.ollamaMu.Lock()
	defer s.ollamaMu.Unlock()

	entry, ok := s.ollamaContext[key]
	if !ok {
		return ollamaContextCacheEntry{}, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(s.ollamaContext, key)
		return ollamaContextCacheEntry{}, false
	}
	return entry, true
}

func (s *Server) storeOllamaContextWindow(model string, window provider.OllamaContextWindow) {
	key := s.ollamaCacheKey(model)
	entry := ollamaContextCacheEntry{
		value:     window.Value,
		found:     true,
		expiresAt: time.Now().Add(ollamaContextCacheTTL),
	}

	s.ollamaMu.Lock()
	if s.ollamaContext == nil {
		s.ollamaContext = make(map[string]ollamaContextCacheEntry)
	}
	s.ollamaContext[key] = entry
	s.ollamaMu.Unlock()
}

func (s *Server) ollamaCacheKey(model string) string {
	return strings.TrimSpace(s.ollamaBaseURL()) + "\x00" + model
}

func (s *Server) configContextWindowTokens() int {
	if s.config == nil {
		return 0
	}
	return s.config.Agents.Defaults.ContextWindowTokens
}

func (s *Server) ollamaBaseURL() string {
	if s.config == nil {
		return ""
	}
	if providerCfg, ok := s.config.Providers["ollama"]; ok {
		return providerCfg.APIBase
	}
	return ""
}

func normalizeOllamaModelID(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return ""
	}
	prefix, rest, ok := strings.Cut(model, "/")
	if !ok {
		return model
	}
	if strings.Contains(strings.ToLower(prefix), "ollama") {
		return rest
	}
	return model
}

func statusSummaryModel(providerID, model string) string {
	if providerID == "ollama" {
		return normalizeOllamaModelID(model)
	}
	return model
}

func usageAlertPayload(summary usage.ProviderSummary) (map[string]any, bool) {
	if strings.TrimSpace(summary.WarningLevel) == "" {
		return nil, false
	}
	return map[string]any{
		"providerId":   summary.ProviderID,
		"modelName":    summary.ModelName,
		"budgetStatus": summary.BudgetStatus,
		"warningLevel": summary.WarningLevel,
		"message":      usageAlertMessage(summary.ProviderID, summary.ModelName, summary.WarningLevel),
	}, true
}

func usageAlertMessage(providerID, modelName, warningLevel string) string {
	label := strings.TrimSpace(modelName)
	providerID = strings.TrimSpace(providerID)
	if label == "" {
		label = providerID
	} else if providerID != "" && !strings.HasPrefix(label, providerID+"/") {
		label = providerID + "/" + label
	}
	if label == "" {
		label = "usage"
	}
	level := strings.TrimSpace(warningLevel)
	if level == "" {
		return "Usage warning for " + label + "."
	}
	return fmt.Sprintf("Usage warning for %s: %s budget threshold reached.", label, level)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (s *Server) startRun(req agent.Request, client *clientState) (string, error) {
	if s.agent == nil {
		return "", fmt.Errorf("agent unavailable")
	}
	runID := "run-" + req.SessionKey

	s.mu.Lock()
	if _, starting := s.startingSessions[req.SessionKey]; starting {
		s.mu.Unlock()
		return "", fmt.Errorf("session %q already active", req.SessionKey)
	}
	if _, active := s.sessionRuns[req.SessionKey]; active {
		s.mu.Unlock()
		return "", fmt.Errorf("session %q already active", req.SessionKey)
	}
	s.startingSessions[req.SessionKey] = struct{}{}
	runCtx, cancel := context.WithCancel(context.Background())
	state := &runState{
		runID:      runID,
		sessionKey: req.SessionKey,
		cancel:     cancel,
		owner:      client,
	}
	s.activeRuns[runID] = state
	s.sessionRuns[req.SessionKey] = runID
	s.wsTasks[client.conn][runID] = struct{}{}
	delete(s.startingSessions, req.SessionKey)
	s.mu.Unlock()

	go s.executeRun(runCtx, state, req)
	return runID, nil
}

func (s *Server) executeRun(ctx context.Context, state *runState, req agent.Request) {
	var thinking strings.Builder
	var lastUsage struct {
		PromptTokens     int
		CompletionTokens int
		TotalTokens      int
	}
	delivered := false

	result, err := s.agent.ProcessDirect(ctx, req, func(event agent.Event) {
		switch event.Type {
		case agent.EventContextCompacting:
			s.emitEvent(state.owner, "compact.start", map[string]any{"session": req.SessionKey})
		case agent.EventThinking:
			thinking.WriteString(event.Content)
			s.emitEvent(state.owner, "chat.thinking", map[string]any{"content": event.Content})
		case agent.EventProgress:
			s.emitEvent(state.owner, "chat.progress", map[string]any{"content": event.Content})
		case agent.EventToolStart:
			if delegatedToolEvent(event.Content) {
				break
			}
			input, _ := event.Data["input"].(string)
			toolID, _ := event.Data["id"].(string)
			s.emitEvent(state.owner, "chat.tool.start", map[string]any{
				"name":  event.Content,
				"input": input,
				"id":    toolID,
			})
		case agent.EventToolDone:
			if delegatedToolEvent(event.Content) {
				break
			}
			if flag, ok := event.Data["deliveredToRequestTarget"].(bool); ok && flag {
				delivered = true
			}
			output, _ := event.Data["output"].(string)
			errStr, _ := event.Data["error"].(string)
			toolID, _ := event.Data["id"].(string)
			s.emitEvent(state.owner, "chat.tool.done", map[string]any{
				"name":                     event.Content,
				"deliveredToRequestTarget": delivered,
				"output":                   output,
				"error":                    errStr,
				"id":                       toolID,
			})
		case agent.EventError:
			s.emitEvent(state.owner, "chat.error", map[string]any{"message": event.Content})
		case agent.EventDone:
		case agent.EventContextCompressed:
			originalTokens, _ := event.Data["originalTokens"].(int)
			compressedTokens, _ := event.Data["compressedTokens"].(int)
			reductionPercent, _ := event.Data["reductionPercent"].(float64)
			s.emitEvent(state.owner, "compact.done", map[string]any{
				"originalTokens":   originalTokens,
				"compressedTokens": compressedTokens,
				"reductionPercent": reductionPercent,
			})
			s.emitEvent(state.owner, "context.compressed", map[string]any{
				"enabled":          true,
				"originalTokens":   originalTokens,
				"compressedTokens": compressedTokens,
				"reductionPercent": reductionPercent,
			})
		case agent.EventUsage:
			pt, _ := event.Data["promptTokens"].(int)
			ct, _ := event.Data["completionTokens"].(int)
			tt, _ := event.Data["totalTokens"].(int)
			lastUsage.PromptTokens = pt
			lastUsage.CompletionTokens = ct
			lastUsage.TotalTokens = tt
			s.emitEvent(state.owner, "chat.usage", map[string]any{
				"promptTokens":     pt,
				"completionTokens": ct,
				"totalTokens":      tt,
				"contextWindow":    s.contextWindowTokens(ctx),
			})
		case agent.EventAgentSpawned:
			s.emitEvent(state.owner, "agent.spawned", cloneEventData(event.Data))
		case agent.EventAgentCompleted:
			s.emitEvent(state.owner, "agent.completed", cloneEventData(event.Data))
		case agent.EventAgentWaitStarted:
			s.emitEvent(state.owner, "agent.wait.started", cloneEventData(event.Data))
		case agent.EventAgentWaitCompleted:
			s.emitEvent(state.owner, "agent.wait.completed", cloneEventData(event.Data))
		}
	})

	if thinking.Len() > 0 {
		s.emitEvent(state.owner, "chat.thinking.done", map[string]any{"content": thinking.String()})
	}
	if err != nil {
		s.emitEvent(state.owner, "chat.error", map[string]any{"message": err.Error()})
	} else {
		s.emitEvent(state.owner, "chat.done", map[string]any{"content": result})
	}

	s.mu.Lock()
	delete(s.activeRuns, state.runID)
	delete(s.sessionRuns, state.sessionKey)
	delete(s.wsTasks[state.owner.conn], state.runID)
	s.completedDelivery[state.runID] = delivered
	if lastUsage.TotalTokens > 0 {
		s.lastUsage = lastUsage
	}
	s.mu.Unlock()
}

func cloneEventData(data map[string]any) map[string]any {
	if len(data) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(data))
	for k, v := range data {
		cloned[k] = v
	}
	return cloned
}

func delegatedToolEvent(name string) bool {
	switch strings.TrimSpace(name) {
	case "task", "wait":
		return true
	default:
		return false
	}
}

func (s *Server) abortRun(runID string) error {
	s.mu.Lock()
	run, ok := s.activeRuns[runID]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("run %q not found", runID)
	}
	run.cancel()
	if s.agent != nil {
		s.agent.CancelSession(run.sessionKey)
	}
	return nil
}

func (s *Server) cancelWsTasks(conn *websocket.Conn) {
	s.mu.Lock()
	runIDs := make([]string, 0, len(s.wsTasks[conn]))
	for runID := range s.wsTasks[conn] {
		runIDs = append(runIDs, runID)
	}
	s.mu.Unlock()
	for _, runID := range runIDs {
		_ = s.abortRun(runID)
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	runIDs := make([]string, 0, len(s.activeRuns))
	for runID := range s.activeRuns {
		runIDs = append(runIDs, runID)
	}
	clients := make([]*websocket.Conn, 0, len(s.clients))
	for conn := range s.clients {
		clients = append(clients, conn)
	}
	s.mu.Unlock()

	for _, runID := range runIDs {
		_ = s.abortRun(runID)
	}

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		s.mu.Lock()
		remaining := len(s.activeRuns)
		s.mu.Unlock()
		if remaining == 0 {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}

	for _, conn := range clients {
		_ = conn.Close()
	}
	return nil
}

func (s *Server) writeEvent(client *clientState, name string, payload any) error {
	if client == nil || client.conn == nil {
		return nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	frame, err := EncodeEvent(EventFrame{
		EventName: name,
		Seq:       client.nextSeq(),
		Payload:   data,
	})
	if err != nil {
		return err
	}
	return client.write(frame)
}

func (s *Server) emitEvent(client *clientState, name string, payload map[string]any) {
	if client == nil || client.conn == nil {
		return
	}
	if err := s.writeEvent(client, name, payload); err != nil {
		log.Printf("[gateway] write %s event failed: %v", name, err)
	}
}

// BroadcastEvent sends an event to all connected WebSocket clients.
func (s *Server) BroadcastEvent(name string, payload map[string]any) {
	s.mu.Lock()
	snapshot := make([]*clientState, 0, len(s.clients))
	for _, cs := range s.clients {
		snapshot = append(snapshot, cs)
	}
	s.mu.Unlock()
	for _, cs := range snapshot {
		s.emitEvent(cs, name, payload)
	}
}

func (c *clientState) write(frame []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteMessage(websocket.TextMessage, frame)
}

func (c *clientState) nextSeq() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seq++
	return c.seq
}
