package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Nomadcxx/smolbot/pkg/agent/compression"
	"github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/Nomadcxx/smolbot/pkg/provider"
	"github.com/Nomadcxx/smolbot/pkg/session"
	"github.com/Nomadcxx/smolbot/pkg/skill"
	"github.com/Nomadcxx/smolbot/pkg/tokenizer"
	"github.com/Nomadcxx/smolbot/pkg/tool"
	"github.com/Nomadcxx/smolbot/pkg/usage"
)

var thinkBlockPattern = regexp.MustCompile(`(?s)<think>.*?</think>`)

type memoryRunner interface {
	MaybeConsolidate(ctx context.Context, sessionKey string) error
}

type LoopDeps struct {
	Provider      provider.Provider
	Tools         *tool.Registry
	Sessions      *session.Store
	Config        *config.Config
	Skills        *skill.Registry
	Tokenizer     *tokenizer.Tokenizer
	UsageRecorder usage.Recorder
	Memory        memoryRunner
	Workspace     string
	MessageRouter tool.MessageRouter
	Spawner       tool.Spawner
}

type AgentLoop struct {
	provider      provider.Provider
	tools         *tool.Registry
	sessions      *session.Store
	config        *config.Config
	skills        *skill.Registry
	memory        memoryRunner
	tokenizer     *tokenizer.Tokenizer
	usageRecorder usage.Recorder
	workspace     string

	messageRouter tool.MessageRouter
	spawner       tool.Spawner

	mu         sync.Mutex
	activeTask map[string]context.CancelFunc
	bgTasks    sync.WaitGroup
}

func NewAgentLoop(deps LoopDeps) *AgentLoop {
	return &AgentLoop{
		provider:      deps.Provider,
		tools:         deps.Tools,
		sessions:      deps.Sessions,
		config:        deps.Config,
		skills:        deps.Skills,
		memory:        deps.Memory,
		tokenizer:     deps.Tokenizer,
		usageRecorder: deps.UsageRecorder,
		workspace:     deps.Workspace,
		messageRouter: deps.MessageRouter,
		spawner:       deps.Spawner,
		activeTask:    make(map[string]context.CancelFunc),
	}
}

func (a *AgentLoop) EffectiveModel() string {
	if a == nil || a.config == nil {
		return ""
	}
	return a.config.Agents.Defaults.Model
}

func (a *AgentLoop) SetActiveModel(model string) {
	if a == nil || a.config == nil {
		return
	}
	a.config.Agents.Defaults.Model = model
}

func (a *AgentLoop) ProcessDirect(ctx context.Context, req Request, cb EventCallback) (string, error) {
	if strings.HasPrefix(req.Content, "/") {
		return a.handleSlashCommand(ctx, req)
	}

	runCtx, cancel, err := a.beginSession(ctx, req.SessionKey)
	if err != nil {
		return "", err
	}
	defer a.endSession(req.SessionKey)
	defer cancel()
	defer func() {
		if cleaner, ok := a.spawner.(interface{ CleanupParent(string) }); ok {
			cleaner.CleanupParent(req.SessionKey)
		}
	}()

	if _, err := a.sessions.GetOrCreateSession(req.SessionKey); err != nil {
		return "", err
	}

	history, err := a.sessions.GetHistory(req.SessionKey, 500)
	if err != nil {
		return "", err
	}

	systemPrompt, err := BuildSystemPrompt(BuildContext{
		Workspace: a.workspace,
		Skills:    a.skills,
	})
	if err != nil {
		return "", err
	}

	userMessage := a.buildUserMessage(req)

	compressedHistory, compressed, _, _, _, err := a.compressSessionHistory(req.SessionKey, history, false, cb)
	if err != nil {
		return "", err
	}
	if compressed {
		history = compressedHistory
	}

	conversation := make([]provider.Message, 0, len(history)+2)
	conversation = append(conversation, provider.Message{Role: "system", Content: systemPrompt})
	conversation = append(conversation, history...)
	conversation = append(conversation, userMessage)

	newMessages := []provider.Message{userMessage}
	finalResponse := ""
	suppressFinalResponse := false
	maxIterations := req.MaxIterations
	if maxIterations <= 0 {
		maxIterations = a.config.Agents.Defaults.MaxToolIterations
	}
	if maxIterations <= 0 {
		maxIterations = 40
	}
	activeModel := firstNonEmptyString(req.Model, a.config.Agents.Defaults.Model)
	reasoningEffort := firstNonEmptyString(req.ReasoningEffort, a.config.Agents.Defaults.ReasoningEffort)
	toolDefs := a.tools.DefinitionsExcluding(req.DisabledTools)

	for i := 0; i < maxIterations; i++ {
		iterationStart := time.Now()
		sanitized := provider.SanitizeMessages(conversation, a.provider.Name())
		stream, err := a.provider.ChatStream(runCtx, provider.ChatRequest{
			Model:           activeModel,
			Messages:        sanitized,
			Tools:           toolDefs,
			MaxTokens:       a.config.Agents.Defaults.MaxTokens,
			Temperature:     a.config.Agents.Defaults.Temperature,
			ReasoningEffort: reasoningEffort,
		})
		if err != nil {
			return "", err
		}

		resp, err := a.consumeStream(stream, cb, suppressFinalResponse)
		if err != nil {
			return "", err
		}
		a.recordUsage(req.SessionKey, activeModel, sanitized, resp, time.Since(iterationStart))
		if resp.FinishReason == "error" {
			emit(cb, Event{Type: EventError, Content: "provider returned error finish"})
			return "", errors.New("provider returned error finish")
		}

		if resp.Usage.TotalTokens > 0 {
			emit(cb, Event{Type: EventUsage, Data: map[string]any{
				"promptTokens":     resp.Usage.PromptTokens,
				"completionTokens": resp.Usage.CompletionTokens,
				"totalTokens":      resp.Usage.TotalTokens,
			}})
		}

		if len(resp.ToolCalls) > 0 {
			assistantMsg := provider.Message{
				Role:             "assistant",
				Content:          resp.Content,
				ToolCalls:        resp.ToolCalls,
				ReasoningContent: resp.ReasoningContent,
			}
			conversation = append(conversation, assistantMsg)
			newMessages = append(newMessages, assistantMsg)

			for _, toolCall := range resp.ToolCalls {
				emit(cb, Event{Type: EventToolHint, Content: toolCall.Function.Name})
				emit(cb, Event{Type: EventToolStart, Content: toolCall.Function.Name, Data: map[string]any{
					"input": toolCall.Function.Arguments,
					"id":    toolCall.ID,
				}})

				result, err := a.tools.Execute(runCtx, toolCall.Function.Name, json.RawMessage(toolCall.Function.Arguments), tool.ToolContext{
					SessionKey:    req.SessionKey,
					Channel:       req.Channel,
					ChatID:        req.ChatID,
					Workspace:     a.workspace,
					Spawner:       a.spawner,
					MessageRouter: a.messageRouter,
					IsCronContext: req.IsCronContext,
					EmitEvent: func(name string, payload map[string]any) {
						switch name {
						case string(EventAgentSpawned):
							emit(cb, Event{Type: EventAgentSpawned, Data: payload})
						case string(EventAgentCompleted):
							emit(cb, Event{Type: EventAgentCompleted, Data: payload})
						case string(EventAgentWaitStarted):
							emit(cb, Event{Type: EventAgentWaitStarted, Data: payload})
						case string(EventAgentWaitCompleted):
							emit(cb, Event{Type: EventAgentWaitCompleted, Data: payload})
						}
					},
				})
				if err != nil {
					return "", err
				}

				output := truncateString(firstNonEmptyString(result.Output, result.Content), 16000)
				errText := truncateString(result.Error, 16000)
				content := firstNonEmptyString(output, errText)
				toolMsg := provider.Message{
					Role:       "tool",
					Content:    content,
					ToolCallID: toolCall.ID,
					Name:       toolCall.Function.Name,
				}
				conversation = append(conversation, toolMsg)
				newMessages = append(newMessages, toolMsg)

				delivered := false
				if toolCall.Function.Name == "message" && result != nil {
					delivered = sameTargetDelivery(req, result.Metadata)
					if delivered {
						suppressFinalResponse = true
					}
				}
				emit(cb, Event{
					Type:    EventToolDone,
					Content: toolCall.Function.Name,
					Data: map[string]any{
						"deliveredToRequestTarget": delivered,
						"id":                       toolCall.ID,
						"output":                   output,
						"error":                    errText,
					},
				})
			}
			continue
		}

		finalResponse = stripThinkBlocks(resp.Content)
		assistantMsg := provider.Message{
			Role:             "assistant",
			Content:          finalResponse,
			ReasoningContent: resp.ReasoningContent,
		}
		conversation = append(conversation, assistantMsg)
		newMessages = append(newMessages, assistantMsg)
		if !suppressFinalResponse {
			emit(cb, Event{Type: EventDone, Content: finalResponse})
		}
		break
	}

	if err := a.sessions.SaveMessages(req.SessionKey, normalizeMessagesForSave(newMessages)); err != nil {
		return "", err
	}

	if a.memory != nil && a.tokenizer != nil && a.config.Agents.Defaults.ContextWindowTokens > 0 && a.tokenizer.EstimatePromptTokens(conversation) > a.config.Agents.Defaults.ContextWindowTokens {
		a.bgTasks.Add(1)
		go func() {
			defer a.bgTasks.Done()
			_ = a.memory.MaybeConsolidate(context.Background(), req.SessionKey)
		}()
	}

	if suppressFinalResponse {
		return "", nil
	}
	return finalResponse, nil
}

func (a *AgentLoop) recordUsage(sessionKey, modelName string, sanitized []provider.Message, resp *provider.Response, duration time.Duration) {
	if a == nil || a.usageRecorder == nil || a.config == nil || a.provider == nil || resp == nil {
		return
	}

	tok := a.tokenizer
	if tok == nil {
		tok = tokenizer.New()
	}

	record := usage.CompletionRecord{
		SessionKey:  sessionKey,
		ProviderID:  a.provider.Name(),
		ModelName:   modelName,
		RequestType: "chat",
		Status:      "success",
		UsageSource: "reported",
	}
	if len(sanitized) > 0 {
		record.PromptTokens = tok.EstimatePromptTokens(sanitized)
	}

	if resp.Usage.TotalTokens > 0 {
		record.PromptTokens = resp.Usage.PromptTokens
		record.CompletionTokens = resp.Usage.CompletionTokens
		record.TotalTokens = resp.Usage.TotalTokens
	} else {
		record.UsageSource = "estimated"
		completionEstimate := resp.Content + resp.ReasoningContent
		for _, toolCall := range resp.ToolCalls {
			completionEstimate += toolCall.ID
			completionEstimate += toolCall.Function.Name
			completionEstimate += toolCall.Function.Arguments
		}
		record.CompletionTokens = tok.EstimateTokens(completionEstimate)
		record.TotalTokens = record.PromptTokens + record.CompletionTokens
	}

	if duration > 0 {
		record.DurationMS = int(duration / time.Millisecond)
	}
	if resp.FinishReason == "error" {
		record.Status = "error"
	}
	if record.TotalTokens <= 0 {
		record.TotalTokens = record.PromptTokens + record.CompletionTokens
	}
	if record.TotalTokens <= 0 {
		return
	}

	if err := a.usageRecorder.RecordCompletion(context.Background(), record); err != nil {
		log.Printf("[agent] usage record failed: %v", err)
	}
}

func (a *AgentLoop) handleSlashCommand(ctx context.Context, req Request) (string, error) {
	switch strings.TrimSpace(req.Content) {
	case "/help":
		return "/help, /new, /stop", nil
	case "/new":
		if a.memory != nil {
			if err := a.memory.MaybeConsolidate(ctx, req.SessionKey); err != nil {
				log.Printf("[agent] memory consolidation failed on /new for session %s: %v", req.SessionKey, err)
			}
		}
		if err := a.sessions.ClearSession(req.SessionKey); err != nil {
			return "", err
		}
		return "Started a new session.", nil
	case "/stop":
		a.cancelSession(req.SessionKey)
		if a.tools != nil {
			a.tools.CancelSession(req.SessionKey)
		}
		return "stopped active session", nil
	default:
		return "Unknown command.", nil
	}
}

func (a *AgentLoop) CancelSession(sessionKey string) {
	a.cancelSession(sessionKey)
}

func (a *AgentLoop) CompactNow(ctx context.Context, sessionKey string) (originalTokens, compressedTokens int, reductionPct float64, err error) {
	history, err := a.sessions.GetHistory(sessionKey, 500)
	if err != nil {
		return 0, 0, 0, err
	}
	_, compressed, originalTokens, compressedTokens, reductionPct, err := a.compressSessionHistory(sessionKey, history, true, nil)
	if err != nil {
		return 0, 0, 0, err
	}
	if !compressed {
		return 0, 0, 0, nil
	}
	return originalTokens, compressedTokens, reductionPct, nil
}

func (a *AgentLoop) beginSession(parent context.Context, sessionKey string) (context.Context, context.CancelFunc, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.activeTask[sessionKey]; ok {
		return nil, nil, fmt.Errorf("session %q is busy", sessionKey)
	}
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	a.activeTask[sessionKey] = cancel
	return ctx, cancel, nil
}

func (a *AgentLoop) endSession(sessionKey string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.activeTask, sessionKey)
}

func (a *AgentLoop) cancelSession(sessionKey string) {
	a.mu.Lock()
	cancel := a.activeTask[sessionKey]
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *AgentLoop) buildUserMessage(req Request) provider.Message {
	prefix := BuildRuntimeContextPrefix(timeNow(), req.Channel, req.ChatID)
	if len(req.Media) == 0 {
		return provider.Message{Role: "user", Content: prefix + req.Content}
	}

	blocks := []provider.ContentBlock{{Type: "text", Text: prefix + req.Content}}
	for _, media := range req.Media {
		mime := media.MimeType
		if mime == "" {
			mime = detectMime(media.Data)
		}
		dataURL := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(media.Data)
		blocks = append(blocks, provider.ContentBlock{
			Type:     "image_url",
			ImageURL: &provider.ImageURL{URL: dataURL, Detail: "auto"},
		})
	}
	return provider.Message{Role: "user", Content: blocks}
}

func (a *AgentLoop) compressSessionHistory(sessionKey string, history []provider.Message, force bool, cb EventCallback) ([]provider.Message, bool, int, int, float64, error) {
	if a == nil || a.config == nil || a.sessions == nil || a.tokenizer == nil {
		return history, false, 0, 0, 0, nil
	}
	compConfig := a.config.Agents.Defaults.Compression
	if !compConfig.Enabled || len(history) < 4 {
		return history, false, 0, 0, 0, nil
	}

	compressor := compression.NewCompressor(compression.Config{
		Enabled:            compConfig.Enabled,
		Mode:               compression.Mode(compConfig.Mode),
		ThresholdPercent:   compConfig.ThresholdPercent,
		KeepRecentMessages: compression.DefaultKeepRecentMessages,
	})
	tokenTracker := compression.NewTokenTracker(a.tokenizer)

	if !force {
		threshold := compConfig.ThresholdPercent
		if threshold == 0 {
			threshold = compression.DefaultThresholdPercent
		}
		if !tokenTracker.ShouldCompress(historyToAny(history), a.config.Agents.Defaults.ContextWindowTokens, threshold) {
			return history, false, 0, 0, 0, nil
		}
	}

	emit(cb, Event{Type: EventContextCompacting})
	result := compressor.Compress(historyToAny(history))
	orig, comp, reduction := tokenTracker.CalculateStats(historyToAny(history), result.CompressedMessages)
	if orig == 0 || comp == 0 || reduction <= 0 {
		return history, false, 0, 0, 0, nil
	}

	result.OriginalTokenCount = orig
	result.CompressedTokenCount = comp
	result.ReductionPercentage = reduction
	compression.RecordCompression(sessionKey, &result)

	compressedHistory := anyToMessages(result.CompressedMessages)
	if err := a.sessions.ReplaceMessages(sessionKey, compressedHistory); err != nil {
		return history, false, 0, 0, 0, err
	}

	emit(cb, Event{
		Type:    EventContextCompressed,
		Content: fmt.Sprintf("Context compressed: %d→%d tokens (%.0f%% reduction)", orig, comp, reduction),
		Data: map[string]any{
			"originalTokens":   orig,
			"compressedTokens": comp,
			"reductionPercent": reduction,
		},
	})
	return compressedHistory, true, orig, comp, reduction, nil
}

func (a *AgentLoop) consumeStream(stream *provider.Stream, cb EventCallback, suppressOutput bool) (*provider.Response, error) {
	defer stream.Close()

	resp := &provider.Response{}
	toolCalls := map[int]*provider.ToolCall{}
	for {
		delta, err := stream.Recv()
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			if err.Error() == "EOF" {
				break
			}
			if err == context.Canceled {
				return nil, err
			}
			if err.Error() == "context canceled" {
				return nil, err
			}
			if strings.Contains(err.Error(), "EOF") {
				break
			}
			return nil, err
		}
		if delta == nil {
			continue
		}
		if delta.Content != "" {
			resp.Content += delta.Content
			if !suppressOutput {
				emit(cb, Event{Type: EventProgress, Content: delta.Content})
			}
		}
		if delta.ReasoningContent != "" {
			resp.ReasoningContent += delta.ReasoningContent
			if !suppressOutput {
				emit(cb, Event{Type: EventThinking, Content: delta.ReasoningContent})
			}
		}
		for _, toolCall := range delta.ToolCalls {
			existing, ok := toolCalls[toolCall.Index]
			if !ok {
				copyCall := toolCall
				toolCalls[toolCall.Index] = &copyCall
				continue
			}
			if toolCall.ID != "" {
				existing.ID = toolCall.ID
			}
			if toolCall.Function.Name != "" {
				existing.Function.Name = toolCall.Function.Name
			}
			existing.Function.Arguments += toolCall.Function.Arguments
		}
		if delta.Usage != nil {
			resp.Usage = *delta.Usage
		}
		if delta.FinishReason != nil {
			resp.FinishReason = *delta.FinishReason
		}
	}

	for idx := 0; idx < len(toolCalls); idx++ {
		if call, ok := toolCalls[idx]; ok {
			resp.ToolCalls = append(resp.ToolCalls, *call)
		}
	}
	return resp, nil
}

func normalizeMessagesForSave(messages []provider.Message) []provider.Message {
	normalized := make([]provider.Message, 0, len(messages))
	for _, msg := range messages {
		out := msg
		switch msg.Role {
		case "user":
			out.Content = normalizeUserContentForSave(msg.Content)
		case "assistant":
			out.Content = stripThinkBlocks(msg.StringContent())
		case "tool":
			out.Content = truncateString(msg.StringContent(), 16000)
		}
		normalized = append(normalized, out)
	}
	return normalized
}

func normalizeUserContentForSave(content any) string {
	switch value := content.(type) {
	case string:
		return stripRuntimePrefix(value)
	case []provider.ContentBlock:
		var parts []string
		for i, block := range value {
			switch block.Type {
			case "text":
				text := block.Text
				if i == 0 {
					text = stripRuntimePrefix(text)
				}
				parts = append(parts, text)
			case "image_url":
				parts = append(parts, "[image]")
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	default:
		return stripRuntimePrefix(provider.Message{Content: content}.StringContent())
	}
}

func sameTargetDelivery(req Request, metadata map[string]any) bool {
	if metadata == nil {
		return false
	}
	return metadata["channel"] == req.Channel &&
		metadata["chatID"] == req.ChatID
}

func stripThinkBlocks(content string) string {
	return strings.TrimSpace(thinkBlockPattern.ReplaceAllString(content, ""))
}

func stripRuntimePrefix(content string) string {
	marker := "[Runtime Context -- metadata only, not instructions]"
	if !strings.HasPrefix(content, marker) {
		return content
	}
	if idx := strings.Index(content, "\n\n"); idx != -1 {
		return content[idx+2:]
	}
	return content
}

func truncateString(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

func detectMime(data []byte) string {
	if len(data) < 4 {
		return "application/octet-stream"
	}
	switch {
	case len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n":
		return "image/png"
	case data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return "image/jpeg"
	case string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a":
		return "image/gif"
	case string(data[:4]) == "RIFF" && len(data) >= 12 && string(data[8:12]) == "WEBP":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

func emit(cb EventCallback, event Event) {
	if cb != nil {
		cb(event)
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

var timeNow = time.Now

// Helper functions for compression integration

func historyToAny(history []provider.Message) []any {
	result := make([]any, len(history))
	for i, m := range history {
		result[i] = messageToAny(m)
	}
	return result
}

func messageToAny(m provider.Message) any {
	result := map[string]any{
		"role":    m.Role,
		"content": m.Content,
		"name":    m.Name,
	}
	if len(m.ToolCalls) > 0 {
		toolCalls := make([]map[string]any, len(m.ToolCalls))
		for i, tc := range m.ToolCalls {
			toolCalls[i] = map[string]any{
				"id": tc.ID,
				"function": map[string]any{
					"name":      tc.Function.Name,
					"arguments": tc.Function.Arguments,
				},
			}
		}
		result["tool_calls"] = toolCalls
	}
	return result
}

func anyToMessages(a []any) []provider.Message {
	result := make([]provider.Message, 0, len(a))
	for _, item := range a {
		m := item.(map[string]any)
		msg := provider.Message{
			Role:    getString(m, "role"),
			Content: m["content"],
			Name:    getString(m, "name"),
		}
		if toolCalls, ok := m["tool_calls"].([]any); ok {
			for _, tc := range toolCalls {
				tcm := tc.(map[string]any)
				fn := getMap(tcm, "function")
				msg.ToolCalls = append(msg.ToolCalls, provider.ToolCall{
					ID: getString(tcm, "id"),
					Function: provider.FunctionCall{
						Name:      getString(fn, "name"),
						Arguments: getString(fn, "arguments"),
					},
				})
			}
		}
		result = append(result, msg)
	}
	return result
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getMap(m map[string]any, key string) map[string]any {
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	return nil
}
