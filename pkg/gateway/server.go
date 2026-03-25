package gateway

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Nomadcxx/smolbot/pkg/agent"
	"github.com/Nomadcxx/smolbot/pkg/channel"
	"github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/Nomadcxx/smolbot/pkg/provider"
	"github.com/Nomadcxx/smolbot/pkg/session"
	"github.com/gorilla/websocket"
)

type AgentProcessor interface {
	ProcessDirect(ctx context.Context, req agent.Request, cb agent.EventCallback) (string, error)
	CancelSession(sessionKey string)
}

type ServerDeps struct {
	Agent            AgentProcessor
	Sessions         *session.Store
	Channels         *channel.Manager
	Config           *config.Config
	Version          string
	StartedAt        time.Time
	SetModelCallback func(model string) error
}

type Server struct {
	agent            AgentProcessor
	sessions         *session.Store
	channels         *channel.Manager
	config           *config.Config
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
	lastUsage         struct {
		PromptTokens     int
		CompletionTokens int
		TotalTokens      int
	}
}

type clientState struct {
	conn        *websocket.Conn
	mu          sync.Mutex
	seq         int64
	isLegacy    bool
}

type runState struct {
	runID      string
	sessionKey string
	cancel     context.CancelFunc
	owner      *clientState
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
		agent:    deps.Agent,
		sessions: deps.Sessions,
		channels: deps.Channels,
		config:   deps.Config,
		version:  firstNonEmptyString(deps.Version, "dev"),
		started:  started,
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
				"hello", "status", "chat.send", "chat.history", "sessions.list", "sessions.reset", "models.list", "models.set",
			},
			"events": []string{"chat.progress", "chat.done", "chat.error", "chat.tool.start", "chat.tool.done", "chat.thinking", "chat.thinking.done", "chat.usage", "context.compressed"},
		}, nil
	case "status":
		var channels []map[string]string
		if s.channels != nil {
			for name, status := range s.channels.Statuses(ctx) {
				channels = append(channels, map[string]string{
					"name":   name,
					"status": status.State,
				})
			}
		}
		return map[string]any{
			"model":  s.currentModel(),
			"uptime": int(time.Since(s.started).Seconds()),
			"channels": channels,
			"usage": map[string]any{
				"promptTokens":     s.lastUsage.PromptTokens,
				"completionTokens": s.lastUsage.CompletionTokens,
				"totalTokens":      s.lastUsage.TotalTokens,
				"contextWindow":    s.config.Agents.Defaults.ContextWindowTokens,
			},
		}, nil
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
	return fallbackProvider
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
		case agent.EventThinking:
			thinking.WriteString(event.Content)
			s.emitEvent(state.owner, "chat.thinking", map[string]any{"content": event.Content})
		case agent.EventProgress:
			s.emitEvent(state.owner, "chat.progress", map[string]any{"content": event.Content})
		case agent.EventToolStart:
			input, _ := event.Data["input"].(string)
			toolID, _ := event.Data["id"].(string)
			s.emitEvent(state.owner, "chat.tool.start", map[string]any{
				"name":  event.Content,
				"input": input,
				"id":    toolID,
			})
		case agent.EventToolDone:
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
				"contextWindow":    s.config.Agents.Defaults.ContextWindowTokens,
			})
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
	if err := s.writeEvent(client, name, payload); err != nil {
		log.Printf("[gateway] write %s event failed: %v", name, err)
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
