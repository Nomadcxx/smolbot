# Context Management Enhancement - FINAL Revised Implementation Plan

**Date:** March 22, 2026
**Status:** FINAL REVISED - Ready for Agent Handoff
**Focus Area:** Area 2 - Context Management
**Repository:** `/home/nomadx/Documents/smolbot`

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| REVISED | 2026-03-22 | Initial revision with gate structure |
| FINAL | 2026-03-22 | Added crush- inspired patterns, improved architecture |

## Reference Projects

- **Crush** (`/home/nomadx/crush`): Go-based AI coding assistant by Charm
  - Key insight: Uses AI-powered summarization via separate `Summarize()` method
  - Key insight: Thread-safe state with `csync` package
  - Key insight: `preparePrompt` converts messages for LLM
- **Nanocoder** (`/home/nomadx/nanocoder`): TypeScript/React CLI
  - Key insight: Token-aware compression with 3 modes (conservative/default/aggressive)
  - Key insight: Compression preserves tool call structure

---

## ARCHITECTURE OVERVIEW

```
pkg/agent/
├── compression/           # NEW PACKAGE
│   ├── types.go           # Compression config, modes, results
│   ├── compressor.go      # Core compression algorithm
│   ├── token_tracker.go  # Token counting and context tracking
│   └── stats.go           # Per-session compression stats (thread-safe)
├── loop.go                # MODIFY - integrate compression
└── context.go             # READ ONLY - understand for integration
```

**Key Design Decisions (from crush analysis):**

1. **Thread-safe stats**: Use mutex-protected map like crush's `csync` pattern
2. **Context tracking**: Track remaining context, not just threshold
3. **Compression modes**: 3 modes inspired by nanocoder (conservative/default/aggressive)
4. **Future enhancement**: AI summarization (like crush) can be added later

---

## GATE 0: Foundation Types (BLOCKING - No Dependencies)

**Purpose:** Create the compression package with types. All other gates depend on these types.

**Entry Gate:** Clean git state, `go mod tidy` passes
**Exit Gate:** `go build ./pkg/agent/compression/...` succeeds

### Task 0.1: Create Compression Types

**File:** `pkg/agent/compression/types.go` (CREATE)

```go
package compression

import "time"

type Mode string

const (
    ModeConservative Mode = "conservative" // Minimal compression, preserve detail
    ModeDefault      Mode = "default"       // Balanced compression
    ModeAggressive   Mode = "aggressive"   // Maximum compression, may lose nuance
)

type Config struct {
    Enabled            bool  `json:"enabled"`
    Mode               Mode  `json:"mode"`
    ThresholdPercent   int   `json:"thresholdPercent"`   // Trigger at % of context window used
    KeepRecentMessages int   `json:"keepRecentMessages"` // Messages to preserve at full detail
}

type Result struct {
    CompressedMessages   []any              `json:"-"`
    OriginalTokenCount   int                `json:"originalTokenCount"`
    CompressedTokenCount int               `json:"compressedTokenCount"`
    ReductionPercentage  float64            `json:"reductionPercentage"`
    PreservedInfo        PreservedInfo     `json:"preservedInfo"`
}

type PreservedInfo struct {
    KeyDecisions       int `json:"keyDecisions"`       // Assistant msgs with tool calls
    FileModifications  int `json:"fileModifications"` // write/edit/create tools
    ToolResults        int `json:"toolResults"`        // Tool response count
    RecentMessages     int `json:"recentMessages"`     // Messages kept uncompressed
}

type Stats struct {
    LastCompression      *Result  `json:"lastCompression"`
    LastCompressionTime time.Time `json:"lastCompressionTime"`
    TotalCompressions   int      `json:"totalCompressions"`
    TotalTokensSaved    int      `json:"totalTokensSaved"`
}

// ContextState tracks current context usage for compression decisions
type ContextState struct {
    TotalTokens     int `json:"totalTokens"`
    ContextWindow   int `json:"contextWindow"`
    RemainingTokens int `json:"remainingTokens"`
    UsagePercent    int `json:"usagePercent"` // 0-100
}

// Compression thresholds (inspired by nanocoder)
const (
    DefaultKeepRecentMessages         = 2
    DefaultThresholdPercent          = 60  // Trigger at 60% context usage
    MinThresholdPercent             = 50
    MaxThresholdPercent             = 95
    
    // Character limits per message type (soft limits for truncation)
    UserMessageThresholdDefault      = 500
    UserMessageThresholdConservative = 1000
    AssistantWithToolsThreshold     = 300
    
    // Hard truncation limits
    TruncationLimitAggressive      = 100
    TruncationLimitDefault         = 200
    TruncationLimitConservative    = 500
)
```

**Verification:** `cd /home/nomadx/Documents/smolbot && go build ./pkg/agent/compression/...`
**Expected:** SUCCESS

**Commit:** `git add pkg/agent/compression/types.go && git commit -m "feat(agent): add compression types"`

---

### Task 0.2: Add Compression Config to AgentDefaults

**File:** `pkg/config/config.go` (MODIFY)

**Add to AgentDefaults struct:**
```go
type CompressionConfig struct {
    Enabled          bool   `json:"enabled"`
    Mode             string `json:"mode"`
    ThresholdPercent int    `json:"thresholdPercent"`
}
```

**Add field to AgentDefaults:**
```go
type AgentDefaults struct {
    // ... existing fields ...
    Compression CompressionConfig `json:"compression"`
}
```

**Add default in DefaultConfig():**
```go
Compression: CompressionConfig{
    Enabled:          true,
    Mode:             "default",
    ThresholdPercent: 60,
},
```

**Verification:** `cd /home/nomadx/Documents/smolbot && go build ./pkg/config/...`
**Expected:** SUCCESS

**Commit:** `git add pkg/config/config.go && git commit -m "feat(config): add compression configuration"`

---

## GATE 1: Core Compression Algorithm (PARALLEL with Gate 2)

**Purpose:** Implement the compression algorithm with proper message type handling.

**Entry Gate:** Gate 0 complete
**Exit Gate:** `go test ./pkg/agent/compression/... -v` all tests pass

### Task 1.1: Write Compressor Tests

**File:** `pkg/agent/compression/compressor_test.go` (CREATE)

```go
package compression

import (
    "strings"
    "testing"
)

func TestCompressorPreservesSystemMessages(t *testing.T) {
    c := NewCompressor(Config{Mode: ModeAggressive})
    messages := []any{
        map[string]any{"role": "system", "content": "You are helpful"},
        map[string]any{"role": "user", "content": "Hello"},
    }
    
    result := c.Compress(messages)
    
    require.Equal(t, 2, len(result.CompressedMessages))
    first := result.CompressedMessages[0].(map[string]any)
    require.Equal(t, "system", first["role"])
    require.Equal(t, "You are helpful", first["content"])
}

func TestCompressorCompressesLongUserMessage(t *testing.T) {
    c := NewCompressor(Config{Mode: ModeDefault})
    longText := strings.Repeat("A", 600)
    messages := []any{
        map[string]any{"role": "user", "content": longText},
    }
    
    result := c.Compress(messages)
    
    first := result.CompressedMessages[0].(map[string]any)
    content := first["content"].(string)
    require.Less(t, len(content), 600)
    require.True(t, strings.HasSuffix(content, "...") || len(content) < 600)
}

func TestCompressorKeepsRecentMessages(t *testing.T) {
    c := NewCompressor(Config{Mode: ModeDefault, KeepRecentMessages: 2})
    messages := []any{
        map[string]any{"role": "user", "content": "First"},
        map[string]any{"role": "assistant", "content": "Second"},
        map[string]any{"role": "user", "content": "Third"},
        map[string]any{"role": "assistant", "content": "Fourth"},
    }
    
    result := c.Compress(messages)
    
    // Should keep all 4, recent 2 at full detail
    require.Equal(t, 4, len(result.CompressedMessages))
    last := result.CompressedMessages[3].(map[string]any)
    require.Equal(t, "Fourth", last["content"])
}

func TestCompressorPreservesToolCallStructure(t *testing.T) {
    c := NewCompressor(Config{Mode: ModeAggressive})
    messages := []any{
        map[string]any{
            "role": "assistant",
            "content": "Let me help",
            "tool_calls": []map[string]any{
                {"id": "call_123", "function": map[string]any{"name": "read_file"}},
            },
        },
    }
    
    result := c.Compress(messages)
    
    first := result.CompressedMessages[0].(map[string]any)
    toolCalls, ok := first["tool_calls"].([]map[string]any)
    require.True(t, ok)
    require.Equal(t, 1, len(toolCalls))
    require.Equal(t, "call_123", toolCalls[0]["id"])
}

func TestCompressorModeAggressive(t *testing.T) {
    c := NewCompressor(Config{Mode: ModeAggressive})
    messages := []any{
        map[string]any{"role": "user", "content": strings.Repeat("B", 1000)},
    }
    
    result := c.Compress(messages)
    
    first := result.CompressedMessages[0].(map[string]any)
    content := first["content"].(string)
    require.LessOrEqual(t, len(content), 103) // TruncationLimitAggressive + "..."
}

func TestCompressorModeConservative(t *testing.T) {
    c := NewCompressor(Config{Mode: ModeConservative})
    messages := []any{
        map[string]any{"role": "assistant", "content": "Normal response without truncation needed"},
    }
    
    result := c.Compress(messages)
    
    first := result.CompressedMessages[0].(map[string]any)
    require.Equal(t, "Normal response without truncation needed", first["content"])
}

func TestCompressorTracksPreservedInfo(t *testing.T) {
    c := NewCompressor(Config{Mode: ModeDefault})
    messages := []any{
        map[string]any{"role": "system", "content": "System"},
        map[string]any{"role": "assistant", "content": "Thinking...", "tool_calls": []any{
            map[string]any{"id": "1", "function": map[string]any{"name": "read_file"}},
        }},
        map[string]any{"role": "tool", "content": "file content here", "name": "read_file"},
    }
    
    result := c.Compress(messages)
    
    require.Equal(t, 1, result.PreservedInfo.KeyDecisions)    // tool_call in assistant
    require.Equal(t, 1, result.PreservedInfo.ToolResults)    // tool response
}
```

**Run:** `cd /home/nomadx/Documents/smolbot && go test ./pkg/agent/compression -v`
**Expected:** Tests FAIL (Compressor not defined yet) - This is TDD

---

### Task 1.2: Implement Compressor

**File:** `pkg/agent/compression/compressor.go` (CREATE)

```go
package compression

import (
    "strings"
)

type Compressor struct {
    config Config
}

func NewCompressor(config Config) *Compressor {
    if config.KeepRecentMessages == 0 {
        config.KeepRecentMessages = DefaultKeepRecentMessages
    }
    return &Compressor{config: config}
}

func (c *Compressor) Compress(messages []any) Result {
    if !c.config.Enabled || len(messages) == 0 {
        return Result{CompressedMessages: messages}
    }
    
    keepRecent := c.config.KeepRecentMessages
    
    // Separate messages by type and position
    var systemMessages []any
    var compressible []any
    var recentMessages []any
    
    for i, msg := range messages {
        m := msg.(map[string]any)
        role := getRole(m)
        
        if role == "system" {
            systemMessages = append(systemMessages, msg)
        } else if i >= len(messages)-keepRecent {
            recentMessages = append(recentMessages, msg)
        } else {
            compressible = append(compressible, msg)
        }
    }
    
    // Compress messages in the middle
    compressed := make([]any, 0, len(compressible))
    preservedInfo := PreservedInfo{RecentMessages: len(recentMessages)}
    
    for _, msg := range compressible {
        compressedMsg := c.compressMessage(msg, &preservedInfo)
        compressed = append(compressed, compressedMsg)
    }
    
    // Combine: system + compressed middle + recent (uncompressed)
    result := make([]any, 0, len(messages))
    result = append(result, systemMessages...)
    result = append(result, compressed...)
    result = append(result, recentMessages...)
    
    return Result{
        CompressedMessages: result,
        PreservedInfo:     preservedInfo,
    }
}

func (c *Compressor) compressMessage(msg any, info *PreservedInfo) any {
    m := msg.(map[string]any)
    role := getRole(m)
    
    switch role {
    case "user":
        return c.compressUserMessage(msg)
    case "assistant":
        return c.compressAssistantMessage(msg, info)
    case "tool":
        return c.compressToolMessage(msg, info)
    default:
        return msg
    }
}

func (c *Compressor) compressUserMessage(msg any) any {
    m := msg.(map[string]any)
    content := getContent(m)
    
    threshold := UserMessageThresholdDefault
    limit := TruncationLimitDefault
    
    if c.config.Mode == ModeConservative {
        threshold = UserMessageThresholdConservative
        limit = TruncationLimitConservative
    } else if c.config.Mode == ModeAggressive {
        limit = TruncationLimitAggressive
    }
    
    if len(content) <= threshold {
        return msg
    }
    
    m["content"] = truncateAtSentence(content, limit)
    return m
}

func (c *Compressor) compressAssistantMessage(msg any, info *PreservedInfo) any {
    m := msg.(map[string]any)
    
    // Always preserve tool_calls structure - mark as key decision
    if toolCalls, ok := m["tool_calls"].([]map[string]any); ok && len(toolCalls) > 0 {
        info.KeyDecisions++
        // Only truncate content if very long and not conservative
        if c.config.Mode != ModeConservative {
            content := getContent(m)
            if len(content) > AssistantWithToolsThreshold {
                m["content"] = truncateAtSentence(content, TruncationLimitDefault)
            }
        }
        return msg
    }
    
    // For regular assistant messages
    if c.config.Mode == ModeConservative {
        return msg // Conservative: don't compress assistant
    }
    
    content := getContent(m)
    limit := TruncationLimitDefault
    if c.config.Mode == ModeAggressive {
        limit = TruncationLimitAggressive
    }
    
    if len(content) > UserMessageThresholdDefault {
        m["content"] = truncateAtSentence(content, limit)
    }
    return msg
}

func (c *Compressor) compressToolMessage(msg any, info *PreservedInfo) any {
    m := msg.(map[string]any)
    info.ToolResults++
    
    // Track file modifications
    if name, ok := m["name"].(string); ok {
        if isFileModificationTool(name) {
            info.FileModifications++
        }
    }
    
    content := getContent(m)
    toolName, _ := m["name"].(string)
    
    switch c.config.Mode {
    case ModeAggressive:
        m["content"] = compressToolAggressive(content)
    case ModeConservative:
        m["content"] = compressToolConservative(content)
    default:
        m["content"] = compressToolDefault(content, toolName)
    }
    
    return msg
}

// Helper functions

func getRole(m map[string]any) string {
    if role, ok := m["role"].(string); ok {
        return role
    }
    return ""
}

func getContent(m map[string]any) string {
    switch content := m["content"].(type) {
    case string:
        return content
    case []any:
        var builder strings.Builder
        for _, item := range content {
            if block, ok := item.(map[string]any); ok {
                if text, ok := block["text"].(string); ok {
                    builder.WriteString(text)
                }
            }
        }
        return builder.String()
    }
    return ""
}

func truncateAtSentence(text string, limit int) string {
    if len(text) <= limit {
        return text
    }
    
    // Try to truncate at sentence boundary
    sentences := strings.Split(text, ". ")
    var result strings.Builder
    for i, sentence := range sentences {
        if result.Len()+len(sentence) > limit-3 {
            break
        }
        if result.Len() > 0 {
            result.WriteString(". ")
        }
        result.WriteString(sentence)
    }
    
    summary := result.String()
    if len(summary) < limit/2 {
        // Fallback: character truncation
        summary = text[:limit-3] + "..."
    } else if !strings.HasSuffix(summary, "...") {
        summary += "..."
    }
    return summary
}

func isFileModificationTool(name string) bool {
    name = strings.ToLower(name)
    patterns := []string{"write", "edit", "create", "modify", "delete", "move", "copy"}
    for _, p := range patterns {
        if strings.Contains(name, p) {
            return true
        }
    }
    return false
}

func compressToolAggressive(content string) string {
    lower := strings.ToLower(content)
    
    // Preserve success indicators
    if strings.Contains(lower, "success") || strings.Contains(lower, "created") ||
       strings.Contains(lower, "updated") || strings.Contains(lower, "saved") {
        return "Result: success"
    }
    
    // Extract error line
    if strings.Contains(lower, "error") {
        for _, line := range strings.Split(content, "\n") {
            if strings.Contains(strings.ToLower(line), "error") {
                return strings.TrimSpace(line)
            }
        }
    }
    
    return truncateAtSentence(content, TruncationLimitAggressive)
}

func compressToolConservative(content string) string {
    lines := strings.Split(content, "\n")
    if len(lines) <= 3 {
        return content
    }
    return strings.Join(lines[:3], "\n") + "\n[+ " + fmt.Sprintf("%d more lines", len(lines)-3) + "]"
}

func compressToolDefault(content, toolName string) string {
    lower := strings.ToLower(content)
    
    // Preserve success indicators
    if strings.Contains(lower, "success") || strings.Contains(lower, "created") ||
       strings.Contains(lower, "updated") || strings.Contains(lower, "saved") {
        return "Result: success"
    }
    
    // Extract error line
    if strings.Contains(lower, "error") {
        for _, line := range strings.Split(content, "\n") {
            if strings.Contains(strings.ToLower(line), "error") {
                return strings.TrimSpace(line)
            }
        }
    }
    
    // Keep first significant line if short
    lines := strings.Split(content, "\n")
    if len(lines) > 0 && len(lines[0]) < 100 {
        return lines[0]
    }
    
    return truncateAtSentence(content, TruncationLimitDefault)
}
```

**Note:** Add `import "fmt"` if not present.

**Run:** `cd /home/nomadx/Documents/smolbot && go test ./pkg/agent/compression -v`
**Expected:** All 6 tests PASS

**Commit:** `git add pkg/agent/compression/compressor.go pkg/agent/compression/compressor_test.go && git commit -m "feat(agent): implement compression algorithm with 3 modes"`

---

## GATE 2: Token Tracking (PARALLEL with Gate 1)

**Purpose:** Integrate with existing tokenizer and track context state.

**Entry Gate:** Gate 0 complete
**Exit Gate:** `go build ./pkg/agent/compression/...` succeeds

### Task 2.1: Review Tokenizer Interface

**File:** `pkg/tokenizer/tokenizer.go` (READ ONLY)

**Check for interface methods:**
- `EstimatePromptTokens(messages []provider.Message) int`
- Any context window or limit methods

**Adjust Task 2.2 if interface differs.**

---

### Task 2.2: Create Token Tracker

**File:** `pkg/agent/compression/token_tracker.go` (CREATE)

```go
package compression

import (
    "github.com/Nomadcxx/smolbot/pkg/provider"
    "github.com/Nomadcxx/smolbot/pkg/tokenizer"
)

type TokenTracker struct {
    tok *tokenizer.Tokenizer
}

func NewTokenTracker(t *tokenizer.Tokenizer) *TokenTracker {
    return &TokenTracker{tok: t}
}

// ShouldCompress returns true if compression threshold is exceeded
func (tt *TokenTracker) ShouldCompress(messages []any, contextWindowTokens, thresholdPercent int) bool {
    if contextWindowTokens == 0 || thresholdPercent == 0 {
        return false
    }
    
    totalTokens := tt.CountTokens(messages)
    usagePercent := (totalTokens * 100) / contextWindowTokens
    
    return usagePercent >= thresholdPercent
}

// CalculateStats computes compression statistics
func (tt *TokenTracker) CalculateStats(original, compressed []any) (orig, comp int, reduction float64) {
    orig = tt.CountTokens(original)
    comp = tt.CountTokens(compressed)
    
    if orig > 0 {
        reduction = float64(orig-comp) * 100 / float64(orig)
    }
    
    return orig, comp, reduction
}

// GetContextState returns current context usage
func (tt *TokenTracker) GetContextState(messages []any, contextWindow int) ContextState {
    total := tt.CountTokens(messages)
    remaining := contextWindow - total
    if remaining < 0 {
        remaining = 0
    }
    
    usagePercent := 0
    if contextWindow > 0 {
        usagePercent = (total * 100) / contextWindow
    }
    
    return ContextState{
        TotalTokens:     total,
        ContextWindow:   contextWindow,
        RemainingTokens: remaining,
        UsagePercent:    usagePercent,
    }
}

// CountTokens estimates token count for messages
func (tt *TokenTracker) CountTokens(messages []any) int {
    if tt.tok == nil {
        return tt.fallbackCount(messages)
    }
    
    // Convert []any to []provider.Message
    providerMsgs := toProviderMessages(messages)
    return tt.tok.EstimatePromptTokens(providerMsgs)
}

func (tt *TokenTracker) fallbackCount(messages []any) int {
    // Rough estimate: ~4 chars per token average
    total := 0
    for _, msg := range messages {
        m := msg.(map[string]any)
        content := getContent(m)
        total += len(content) / 4
        total += 10 // overhead per message
    }
    return total
}

func toProviderMessages(a []any) []provider.Message {
    result := make([]provider.Message, 0, len(a))
    for _, item := range a {
        m := item.(map[string]any)
        msg := provider.Message{
            Role:    getString(m, "role"),
            Content: m["content"],
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
```

**Run:** `cd /home/nomadx/Documents/smolbot && go build ./pkg/agent/compression/...`
**Expected:** SUCCESS

**Commit:** `git add pkg/agent/compression/token_tracker.go && git commit -m "feat(agent): add token tracking for compression"`

---

### Task 2.3: Add Thread-Safe Compression Stats

**File:** `pkg/agent/compression/stats.go` (CREATE)

```go
package compression

import (
    "sync"
    "time"
)

var (
    sessionStats = make(map[string]*Stats)
    statsMu      sync.RWMutex
)

// RecordCompression stores compression result for a session
// Thread-safe using mutex (inspired by crush's csync pattern)
func RecordCompression(sessionKey string, result *Result) {
    statsMu.Lock()
    defer statsMu.Unlock()
    
    stats, exists := sessionStats[sessionKey]
    if !exists {
        stats = &Stats{}
        sessionStats[sessionKey] = stats
    }
    
    stats.LastCompression = result
    stats.LastCompressionTime = time.Now()
    stats.TotalCompressions++
    
    if result != nil {
        stats.TotalTokensSaved += result.OriginalTokenCount - result.CompressedTokenCount
    }
}

// GetStats retrieves compression stats for a session
func GetStats(sessionKey string) *Stats {
    statsMu.RLock()
    defer statsMu.RUnlock()
    return sessionStats[sessionKey]
}

// ClearStats removes compression stats for a session
func ClearStats(sessionKey string) {
    statsMu.Lock()
    defer statsMu.Unlock()
    delete(sessionStats, sessionKey)
}

// GetAllStats returns a copy of all session stats (for debugging/admin)
func GetAllStats() map[string]*Stats {
    statsMu.RLock()
    defer statsMu.RUnlock()
    
    result := make(map[string]*Stats, len(sessionStats))
    for k, v := range sessionStats {
        result[k] = v
    }
    return result
}
```

**Run:** `cd /home/nomadx/Documents/smolbot && go build ./pkg/agent/compression/...`
**Expected:** SUCCESS

**Commit:** `git add pkg/agent/compression/stats.go && git commit -m "feat(agent): add thread-safe per-session compression statistics"`

---

## GATE 3: Agent Loop Integration (BLOCKING - Depends on Gates 1 & 2)

**Purpose:** Integrate compression into the agent processing loop.

**Entry Gate:** Gates 1 and 2 complete, all tests pass
**Exit Gate:** `go build ./pkg/agent/...` succeeds

### Task 3.1: Review Agent Loop Structure

**File:** `pkg/agent/loop.go` (READ - understand integration point)

**Find:**
1. Where history is retrieved: `sessions.GetHistory`
2. Where messages are built for LLM call
3. How config is accessed

**Integration point:** AFTER history retrieval, BEFORE building messages for LLM.

---

### Task 3.2: Add Compressor to Agent

**File:** `pkg/agent/loop.go` (MODIFY)

**Add to Agent struct:**
```go
type Agent struct {
    // ... existing fields ...
    compressor   *compression.Compressor
    tokenTracker *compression.TokenTracker
}
```

**In NewAgent or initialization:**
```go
agent.compressor = compression.NewCompressor(compression.Config{
    Enabled:          cfg.Agents.Defaults.Compression.Enabled,
    Mode:             compression.Mode(cfg.Agents.Defaults.Compression.Mode),
    ThresholdPercent: cfg.Agents.Defaults.Compression.ThresholdPercent,
    KeepRecentMessages: 2,
})
if agent.tokenizer != nil {
    agent.tokenTracker = compression.NewTokenTracker(agent.tokenizer)
}
```

---

### Task 3.3: Integrate Compression in ProcessDirect

**File:** `pkg/agent/loop.go` (MODIFY)

**Find section AFTER `history, err := a.sessions.GetHistory(...)` and BEFORE LLM call.**

**Add:**
```go
// Compress history if threshold exceeded
if a.compressor != nil && a.tokenTracker != nil {
    threshold := cfg.Agents.Defaults.Compression.ThresholdPercent
    if threshold == 0 {
        threshold = 60 // default
    }
    
    if a.tokenTracker.ShouldCompress(
        historyToAny(history),
        cfg.Agents.Defaults.ContextWindowTokens,
        threshold,
    ) {
        result := a.compressor.Compress(historyToAny(history))
        
        // Calculate token stats
        orig, comp, reduction := a.tokenTracker.CalculateStats(
            historyToAny(history),
            result.CompressedMessages,
        )
        result.OriginalTokenCount = orig
        result.CompressedTokenCount = comp
        result.ReductionPercentage = reduction
        
        // Record stats for session
        compression.RecordCompression(req.SessionKey, &result)
        
        // Convert back and log event
        history = anyToMessages(result.CompressedMessages)
        
        cb(Event{
            Type:    "context.compressed",
            Content: fmt.Sprintf("Context compressed: %d→%d tokens (%.0f%% reduction)", 
                orig, comp, reduction),
        })
    }
}
```

**Add helper functions (can be in a new file or at bottom of loop.go):**

```go
func historyToAny(h []provider.Message) []any {
    result := make([]any, len(h))
    for i, m := range h {
        result[i] = messageToAny(m)
    }
    return result
}

func messageToAny(m provider.Message) any {
    result := map[string]any{
        "role":    m.Role,
        "content": m.Content,
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
    if m.Name != "" {
        result["name"] = m.Name
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
```

**Run:** `cd /home/nomadx/Documents/smolbot && go build ./pkg/agent/...`
**Expected:** SUCCESS

**Commit:** `git add pkg/agent/loop.go && git commit -m "feat(agent): integrate compression into processing loop"`

---

## GATE 4: Verification (PARALLEL with Gate 3)

**Purpose:** Final verification and integration tests.

**Entry Gate:** Gate 3 code written
**Exit Gate:** All tests pass, `go vet` clean

### Task 4.1: Integration Test

**File:** `pkg/agent/compression/integration_test.go` (CREATE)

```go
package compression

import (
    "strings"
    "testing"
)

func TestIntegrationFullCompressionFlow(t *testing.T) {
    cfg := Config{
        Enabled:          true,
        Mode:             ModeDefault,
        KeepRecentMessages: 2,
    }
    
    compressor := NewCompressor(cfg)
    
    messages := []any{
        map[string]any{"role": "system", "content": "You are helpful"},
        map[string]any{"role": "user", "content": strings.Repeat("A", 600)},
        map[string]any{"role": "assistant", "content": "Response"},
        map[string]any{"role": "user", "content": "Short"},
        map[string]any{"role": "assistant", "content": "Last"},
    }
    
    result := compressor.Compress(messages)
    
    // System preserved
    first := result.CompressedMessages[0].(map[string]any)
    require.Equal(t, "system", first["role"])
    
    // Recent messages preserved
    last := result.CompressedMessages[len(result.CompressedMessages)-1].(map[string]any)
    require.Equal(t, "Last", last["content"])
    
    // Middle compressed
    second := result.CompressedMessages[1].(map[string]any)
    content := second["content"].(string)
    require.Less(t, len(content), 600)
}

func TestIntegrationPreservesToolCalls(t *testing.T) {
    cfg := Config{Mode: ModeAggressive}
    compressor := NewCompressor(cfg)
    
    messages := []any{
        map[string]any{
            "role": "assistant",
            "content": strings.Repeat("X", 1000),
            "tool_calls": []map[string]any{
                {"id": "1", "function": map[string]any{"name": "write_file"}},
                {"id": "2", "function": map[string]any{"name": "read_file"}},
            },
        },
    }
    
    result := compressor.Compress(messages)
    
    first := result.CompressedMessages[0].(map[string]any)
    toolCalls := first["tool_calls"].([]map[string]any)
    
    require.Equal(t, 2, len(toolCalls))
    require.Equal(t, "1", toolCalls[0]["id"])
    require.Equal(t, "2", toolCalls[1]["id"])
}

func TestIntegrationThresholdDetection(t *testing.T) {
    tracker := &TokenTracker{}
    
    messages := []any{
        map[string]any{"role": "user", "content": "Test"},
    }
    
    // Should not compress at 10% of 1000 token window
    require.False(t, tracker.ShouldCompress(messages, 1000, 60))
    
    // Would compress at 70%
    largeMessages := make([]any, 70)
    for i := range largeMessages {
        largeMessages[i] = map[string]any{"role": "user", "content": strings.Repeat("A", 10)}
    }
    // Note: This is a simplified test; real test would need accurate token counting
}
```

**Run:** `cd /home/nomadx/Documents/smolbot && go test ./pkg/agent/compression -v`
**Expected:** All tests PASS

---

### Task 4.2: Final Verification

```bash
cd /home/nomadx/Documents/smolbot
go vet ./pkg/agent/compression/...
go build ./pkg/agent/...
go test ./pkg/agent/compression/... -v
```

**Expected:** No errors, all tests pass

---

### Task 4.3: Final Commit

```bash
git add -A
git commit -m "feat(agent): implement context compression system

Implements Area 2 (Context Management) with:

GATE 0 - Foundation:
- Compression types (Mode, Config, Result, Stats, ContextState)
- Compression configuration in AgentDefaults

GATE 1 - Core Algorithm:
- 3 compression modes: conservative, default, aggressive
- Smart truncation at sentence boundaries
- Preserves tool call structure
- Tracks preserved info (keyDecisions, fileModifications, etc.)

GATE 2 - Token Integration:
- TokenTracker for context state monitoring
- Thread-safe Stats storage (mutex-protected map)
- Fallback token estimation

GATE 3 - Agent Integration:
- Compression triggers at configurable threshold (default 60%)
- Automatic compression before LLM calls
- Compression event logged for UI display

Key features:
- Preserves system messages always
- Preserves recent N messages at full detail
- Compresses middle messages based on mode
- Thread-safe stats per session

Future enhancements (from crush analysis):
- AI-powered summarization (crush-style)
- Better context window tracking"
```

---

## SUMMARY

| Gate | Tasks | Dependencies | Duration | Files |
|------|-------|-------------|----------|-------|
| **GATE 0** | 2 | None | 15 min | types.go, config.go |
| **GATE 1** | 2 | Gate 0 | 45 min | compressor.go, *_test.go |
| **GATE 2** | 3 | Gate 0 | 30 min | token_tracker.go, stats.go |
| **GATE 3** | 3 | Gates 1 & 2 | 45 min | loop.go |
| **GATE 4** | 3 | Gate 3 | 20 min | integration_test.go |

**Total:** 13 tasks, ~2.5 hours
**Max Parallel:** Gates 1 & 2 run in parallel after Gate 0

**New Files (6):**
- `pkg/agent/compression/types.go`
- `pkg/agent/compression/compressor.go`
- `pkg/agent/compression/token_tracker.go`
- `pkg/agent/compression/stats.go`
- `pkg/agent/compression/compressor_test.go`
- `pkg/agent/compression/integration_test.go`

**Modified Files (2):**
- `pkg/config/config.go`
- `pkg/agent/loop.go`

---

## DELIVERABLES CHECKLIST

- [ ] `go build ./pkg/agent/compression/...` succeeds
- [ ] `go test ./pkg/agent/compression/... -v` (6+ tests passing)
- [ ] `go build ./pkg/agent/...` succeeds
- [ ] `go vet ./pkg/agent/compression/...` clean
- [ ] 3 compression modes work differently
- [ ] System messages preserved
- [ ] Recent messages preserved per config
- [ ] Tool call structure preserved
- [ ] Stats recorded per session
- [ ] Compression event emitted

---

## INSPIRATION CREDITS

- **Nanocoder**: 3-mode compression (conservative/default/aggressive), token threshold triggering
- **Crush**: Thread-safe stats (csync pattern), preparePrompt conversion, AI summarization future enhancement
