# Provider & Storage Bugfix Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix 9 bugs across providers, config, storage, and skills: provider prefix stripping for all providers (not just MiniMax), OAuth store silent failure, config startup validation, cron crash-safety, web_search output flood, user skills never loading, HasResource nil-pointer panic, Azure streaming usage loss, and tokenizer inaccuracy for non-OpenAI models.

**Architecture:** All fixes are isolated to their respective packages with no cross-package dependencies introduced. Three fixes share `pkg/provider/sanitize.go` as the export point for the shared `StripProviderPrefix` helper. Two fixes are in `pkg/skill/registry.go`. All other fixes are single-file changes.

**Tech Stack:** Go standard library, `io/fs`, `os`, `github.com/pkoukk/tiktoken-go`

**⚠️ Important notes:**
- **Line numbers are approximate (±5 lines)** — verify exact locations before editing
- **Task 1 and Task 2 both modify `azure.go`** — merge conflicts possible if working in parallel
- **`NewOAuthTokenStore` is an exported API** — this plan changes its signature (see Task 3)

---

## Pre-Execution Verification

**CRITICAL: Run these before starting any task to establish baseline.**

- [ ] **Step 0.1: Verify test suite passes**

```bash
go test ./... -count=1 2>&1 | tee /tmp/baseline-tests.txt
# Must show all PASS, no FAIL
```

- [ ] **Step 0.2: Verify build succeeds**

```bash
go build ./cmd/smolbot/ ./cmd/installer/
# Must complete without errors
```

- [ ] **Step 0.3: Verify race detector passes** (optional but recommended)

```bash
go test -race ./pkg/provider/ ./pkg/config/ ./pkg/cron/ ./pkg/tool/ ./pkg/skill/ ./pkg/tokenizer/ 2>&1 | tail -30
# Must show no race conditions
```

---

## File Map

| File | What changes |
|---|---|
| `pkg/provider/sanitize.go` | Add exported `StripProviderPrefix(model string) string` |
| `pkg/provider/sanitize_test.go` | Add test for `StripProviderPrefix` |
| `pkg/provider/anthropic.go` | Call `StripProviderPrefix` in `buildRequest` |
| `pkg/provider/openai.go` | Call `StripProviderPrefix` in `buildWireRequest` |
| `pkg/provider/azure.go` | Call `StripProviderPrefix` at start of `Chat` and `ChatStream`; add `StreamOptions` to `azureRequest`; fix empty-choices usage check in stream |
| `pkg/provider/minimax_oauth.go` | Remove private `stripProviderPrefix` (replaced by `StripProviderPrefix`) |
| `pkg/provider/azure_test.go` | Add tests: prefixed model stripped, streaming returns usage, stream_options sent |
| `pkg/provider/anthropic_test.go` | Add test: prefixed model is stripped from request |
| `pkg/provider/openai_test.go` | Add test: prefixed model is stripped from request |
| `pkg/config/oauth_store.go` | Change `NewOAuthTokenStore` to return `(OAuthTokenStore, error)`; update internal impl |
| `pkg/config/oauth_store_test.go` | Update all 5 call sites to handle `(OAuthTokenStore, error)`; add test for error case |
| `cmd/smolbot/runtime.go` | Handle error from `NewOAuthTokenStore`; add model non-empty check |
| `pkg/cron/service.go` | Replace `os.WriteFile` with atomic write + `f.Sync()` in `persist()` |
| `pkg/cron/service_test.go` | Add test verifying persist is crash-safe |
| `pkg/tool/web.go` | Add `maxSearchOutputBytes` cap in `WebSearchTool.Execute` |
| `pkg/tool/web_test.go` | Add `stubSearchBackend` type; add test verifying output is capped |
| `pkg/skill/registry.go` | Fix user skills dir check (K3): use `os.Stat`; fix `HasResource` (C14): use `os.Stat` |
| `pkg/skill/registry_test.go` | Add tests for user skills loading and HasResource |
| `pkg/tokenizer/tokenizer.go` | Add `NewForModel(model string)` with model-aware encoding selection; fix `init()` for empty model |
| `pkg/tokenizer/tokenizer_test.go` | Add tests: GPT models use cl100k_base, Ollama models use fallback, empty model works |
| `cmd/smolbot/runtime.go` | Use `tokenizer.NewForModel(cfg.Agents.Defaults.Model)` at startup |

---

## Root Cause Validation

Execute these commands to verify each bug's root cause exists before implementing fixes:

### C8/C9: Provider prefix stripping
```bash
# Verify stripProviderPrefix is private (lowercase) and only in minimax_oauth.go
grep -n "func stripProviderPrefix\|func StripProviderPrefix" pkg/provider/*.go
# Expected: only private version in minimax_oauth.go

# Verify anthropic.go and openai.go set Model directly without stripping
grep -n "Model:.*req\.Model" pkg/provider/anthropic.go pkg/provider/openai.go
# Expected: lines setting Model without StripProviderPrefix
```

### M2: Azure streaming usage loss
```bash
# Verify empty-choices skip without usage check
grep -n "len(chunk.Choices) == 0" pkg/provider/azure.go
# Expected: line with `continue` (no usage handling)

# Verify no StreamOptions in azureRequest
grep -n "StreamOptions" pkg/provider/azure.go
# Expected: no matches
```

### C13: OAuth store silent failure
```bash
# Verify NewOAuthTokenStore returns one value (OAuthTokenStore)
grep -n "func NewOAuthTokenStore" pkg/config/oauth_store.go
# Expected: `func NewOAuthTokenStore(paths *Paths) OAuthTokenStore`

# Verify MkdirAll error path returns store (not error)
sed -n '41,52p' pkg/config/oauth_store.go
# Expected: return store even on MkdirAll error
```

### C14: HasResource nil panic
```bash
# Verify fs.Stat(nil, ...) in HasResource
grep -n "fs.Stat(nil" pkg/skill/registry.go
# Expected: line 135 with nil as first argument
```

### K3: User skills wrong FS
```bash
# Verify user skills dir checked against embedded FS
grep -n "fs.Stat(nanobotgo.EmbeddedAssets" pkg/skill/registry.go
# Expected: line 45 checking userSkillsDir against EmbeddedAssets
```

### C16: No model validation
```bash
# Verify no model validation in buildRuntime
grep -n "Agents.Defaults.Model.*empty\|model.*required" cmd/smolbot/runtime.go
# Expected: no matches
```

### H4: Cron no fsync
```bash
# Verify persist uses os.WriteFile (not atomic write)
grep -n "os.WriteFile" pkg/cron/service.go
# Expected: line ~307 with os.WriteFile

# Verify no Sync() call in persist
grep -n "Sync()" pkg/cron/service.go
# Expected: no matches
```

### H5: web_search uncapped output
```bash
# Verify unlimited builder in Execute
sed -n '149,161p' pkg/tool/web.go
# Expected: loop appending to strings.Builder without size check
```

### M10: Tokenizer ignores model
```bash
# Verify hardcoded cl100k_base
grep -n 'tiktoken.GetEncoding\|"cl100k_base"' pkg/tokenizer/tokenizer.go
# Expected: line 71 with hardcoded "cl100k_base"

# Verify no model parameter
grep -n "func New\(\)" pkg/tokenizer/tokenizer.go
# Expected: `func New() *Tokenizer` with no parameters
```

---

## Task 1: Provider prefix stripping for all providers (C8/C9)

**Root cause:** `stripProviderPrefix` exists only in `minimax_oauth.go:381-389` as a private function. `anthropic.go` sets `Model: req.Model` directly without stripping, so `"anthropic/claude-3-opus"` hits the Anthropic API and gets a 400. Same for `openai.go`. Azure passes `req.Model` to `p.endpoint()` building `"/openai/deployments/azure/gpt-4o/chat/completions"` — a 404.

**Files:**
- Modify: `pkg/provider/sanitize.go`
- Modify: `pkg/provider/sanitize_test.go`
- Modify: `pkg/provider/anthropic.go` (≈line 201 and 238)
- Modify: `pkg/provider/openai.go` (≈line 182)
- Modify: `pkg/provider/azure.go` (start of `Chat` and `ChatStream`)
- Modify: `pkg/provider/minimax_oauth.go` (remove private duplicate)

- [ ] **Step 1: Validate root cause** (see Root Cause Validation section above)

- [ ] **Step 2: Write failing tests for prefix stripping**

In `pkg/provider/sanitize_test.go`, add:

```go
func TestStripProviderPrefix(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"anthropic/claude-3-opus", "claude-3-opus"},
		{"openai/gpt-4o", "gpt-4o"},
		{"azure/gpt-4", "gpt-4"},
		{"minimax-portal/MiniMax-M2.7", "MiniMax-M2.7"},
		{"gpt-4o", "gpt-4o"},
		{"claude-3-opus", "claude-3-opus"},
		{"", ""},
	}
	for _, tc := range cases {
		got := StripProviderPrefix(tc.input)
		if got != tc.want {
			t.Errorf("StripProviderPrefix(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
```

In `pkg/provider/openai_test.go`, add:

```go
func TestOpenAIProviderStripsPrefixFromModel(t *testing.T) {
	var gotModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			gotModel = body.Model
		}
		resp := map[string]any{
			"id":      "chatcmpl-1",
			"choices": []map[string]any{{"index": 0, "finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": "ok"}}},
			"usage":   map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewOpenAIProvider("openai", "key", server.URL+"/v1", nil)
	p.sleep = func(context.Context, int) error { return nil }

	_, err := p.Chat(context.Background(), ChatRequest{
		Model:    "openai/gpt-4o",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if gotModel != "gpt-4o" {
		t.Fatalf("model sent to API = %q, want %q", gotModel, "gpt-4o")
	}
}
```

In `pkg/provider/anthropic_test.go`, add:

```go
func TestAnthropicProviderStripsPrefixFromModel(t *testing.T) {
	var gotModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			gotModel = body.Model
		}
		resp := map[string]any{
			"id":          "msg-1",
			"type":        "message",
			"role":        "assistant",
			"model":       "claude-3-opus-20240229",
			"stop_reason": "end_turn",
			"content":     []map[string]any{{"type": "text", "text": "ok"}},
			"usage":       map[string]any{"input_tokens": 10, "output_tokens": 5},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewAnthropicProvider("test-key", server.URL)
	_, err := p.Chat(context.Background(), ChatRequest{
		Model:    "anthropic/claude-3-opus-20240229",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if gotModel != "claude-3-opus-20240229" {
		t.Fatalf("model sent to API = %q, want %q", gotModel, "claude-3-opus-20240229")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./pkg/provider/ -run "TestStripProviderPrefix|TestOpenAIProviderStripsPrefixFromModel|TestAnthropicProviderStripsPrefixFromModel" -v
```

Expected: `TestStripProviderPrefix` fails (symbol not found); model tests fail (model not stripped).

- [ ] **Step 4: Add StripProviderPrefix to sanitize.go**

Add to `pkg/provider/sanitize.go` (before existing functions):

```go
// StripProviderPrefix removes a "provider/" prefix from a model name.
// "anthropic/claude-3-opus" → "claude-3-opus". No-op if no slash present.
func StripProviderPrefix(model string) string {
	if idx := strings.LastIndex(model, "/"); idx >= 0 {
		return model[idx+1:]
	}
	return model
}
```

- [ ] **Step 5: Remove private duplicate from minimax_oauth.go**

In `pkg/provider/minimax_oauth.go`, delete the private `stripProviderPrefix` function (≈lines 381-389). Replace call sites with `StripProviderPrefix(req.Model)`.

- [ ] **Step 6: Call StripProviderPrefix in anthropic.go**

In `pkg/provider/anthropic.go`, find both locations where `Model: req.Model` is set (≈lines 201 and 238). Change both to:

```go
Model: StripProviderPrefix(req.Model),
```

- [ ] **Step 7: Call StripProviderPrefix in openai.go**

In `pkg/provider/openai.go`, find `buildWireRequest` (≈line 182) and change:

```go
Model: req.Model,
```

to:

```go
Model: StripProviderPrefix(req.Model),
```

- [ ] **Step 8: Fix Azure to strip prefix at method entry**

In `pkg/provider/azure.go`, at the start of `Chat` (≈line 48):

```go
func (p *AzureProvider) Chat(ctx context.Context, req ChatRequest) (*Response, error) {
	req.Model = StripProviderPrefix(req.Model)
	// ... rest of method
}
```

At the start of `ChatStream` (≈line 62):

```go
func (p *AzureProvider) ChatStream(ctx context.Context, req ChatRequest) (*Stream, error) {
	req.Model = StripProviderPrefix(req.Model)
	// ... rest of method
}
```

- [ ] **Step 9: Run all tests to verify they pass**

```bash
go test ./pkg/provider/ -run "TestStripProviderPrefix|TestOpenAIProviderStripsPrefixFromModel|TestAnthropicProviderStripsPrefixFromModel" -v
go test ./pkg/provider/ -v -count=1 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 10: Commit**

```bash
git add pkg/provider/sanitize.go pkg/provider/sanitize_test.go \
        pkg/provider/anthropic.go pkg/provider/anthropic_test.go \
        pkg/provider/openai.go pkg/provider/openai_test.go \
        pkg/provider/azure.go \
        pkg/provider/minimax_oauth.go
git commit -m "fix(provider): strip provider prefix from model name for all providers"
```

---

## Task 2: Azure streaming usage loss (M2)

**Root cause:** `azure.go` streaming loop (≈line 103) does `if len(chunk.Choices) == 0 { continue }`, discarding the final chunk which Azure sends with empty Choices but with `usage` populated. OpenAI streaming handles this correctly.

**Files:**
- Modify: `pkg/provider/azure.go`
- Modify: `pkg/provider/azure_test.go`

- [ ] **Step 1: Validate root cause** (see Root Cause Validation section above)

- [ ] **Step 2: Write failing tests**

In `pkg/provider/azure_test.go`, add:

```go
func TestAzureStreamingReturnsUsage(t *testing.T) {
	streamChunks := []string{
		`data: {"id":"1","choices":[{"index":0,"delta":{"role":"assistant","content":"hi"},"finish_reason":null}]}`,
		`data: {"id":"1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		`data: {"id":"1","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":3,"total_tokens":13}}`,
		`data: [DONE]`,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, chunk := range streamChunks {
			fmt.Fprintln(w, chunk)
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer server.Close()

	p := NewAzureProvider("key", server.URL)
	p.sleep = func(context.Context, int) error { return nil }

	stream, err := p.ChatStream(context.Background(), ChatRequest{
		Model:    "gpt-4",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	defer stream.Close()

	var lastUsage *Usage
	for {
		delta, err := stream.Next()
		if err != nil {
			break
		}
		if delta != nil && delta.Usage != nil {
			lastUsage = delta.Usage
		}
	}
	if lastUsage == nil {
		t.Fatal("expected usage in stream, got none — empty-choices chunk was skipped")
	}
	if lastUsage.PromptTokens != 10 || lastUsage.CompletionTokens != 3 {
		t.Fatalf("usage = %+v, want {10 3 13}", lastUsage)
	}
}

func TestAzureStreamingRequestIncludesStreamOptions(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, "data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}")
		fmt.Fprintln(w, "data: [DONE]")
	}))
	defer server.Close()

	p := NewAzureProvider("key", server.URL)
	p.sleep = func(context.Context, int) error { return nil }

	stream, _ := p.ChatStream(context.Background(), ChatRequest{
		Model:    "gpt-4",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	stream.Close()

	if gotBody == nil {
		t.Fatal("no request body captured")
	}
	so, ok := gotBody["stream_options"].(map[string]any)
	if !ok {
		t.Fatal("stream_options not present in request body")
	}
	if so["include_usage"] != true {
		t.Fatalf("stream_options.include_usage = %v, want true", so["include_usage"])
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./pkg/provider/ -run "TestAzureStreaming" -v
```

Expected: `TestAzureStreamingReturnsUsage` FAIL (lastUsage nil), `TestAzureStreamingRequestIncludesStreamOptions` FAIL (stream_options missing).

- [ ] **Step 4: Add StreamOptions to azureRequest**

In `pkg/provider/azure.go`, in the `azureRequest` struct (≈line 189), add:

```go
type azureRequest struct {
	Messages            []openAIMessage   `json:"messages"`
	Tools               []openAITool      `json:"tools,omitempty"`
	ToolChoice          any               `json:"tool_choice,omitempty"`
	MaxCompletionTokens int               `json:"max_completion_tokens,omitempty"`
	Temperature         *float64          `json:"temperature,omitempty"`
	Reasoning           *openAIReasoning  `json:"reasoning,omitempty"`
	Stream              bool              `json:"stream,omitempty"`
	StreamOptions       *openAIStreamOpts `json:"stream_options,omitempty"`
}
```

- [ ] **Step 5: Set StreamOptions in buildRequest when streaming**

In `buildRequest`, after creating `wireReq`, add:

```go
func (p *AzureProvider) buildRequest(req ChatRequest, stream bool) azureRequest {
	messages := SanitizeMessages(req.Messages, p.Name())
	wireReq := azureRequest{
		Messages:            convertOpenAIMessages(messages, false),
		Tools:               convertOpenAITools(req.Tools, false),
		ToolChoice:          req.ToolChoice,
		MaxCompletionTokens: req.MaxTokens,
		Stream:              stream,
	}
	if stream {
		wireReq.StreamOptions = &openAIStreamOpts{IncludeUsage: true}
	}
	// ... rest of method
}
```

- [ ] **Step 6: Fix empty-choices skip in stream loop**

In `pkg/provider/azure.go` (≈line 103), change:

```go
if len(chunk.Choices) == 0 {
    continue
}
```

to:

```go
if len(chunk.Choices) == 0 {
    if chunk.Usage != nil {
        return &StreamDelta{Usage: chunk.Usage}, nil
    }
    continue
}
```

- [ ] **Step 7: Run tests to verify they pass**

```bash
go test ./pkg/provider/ -run "TestAzureStreaming" -v
```

Expected: both PASS

- [ ] **Step 8: Run all provider tests**

```bash
go test ./pkg/provider/ -v -count=1 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 9: Commit**

```bash
git add pkg/provider/azure.go pkg/provider/azure_test.go
git commit -m "fix(azure): capture usage from empty-choices final stream chunk"
```

---

## Task 3: OAuth store MkdirAll error handling (C13)

**Root cause:** `NewOAuthTokenStore` in `config/oauth_store.go:41` returns `OAuthTokenStore` (one value). When `os.MkdirAll` fails, it returns a valid in-memory store, but every subsequent `Save()` call fails trying to write to a non-existent directory. **Breaking change:** This is an exported API.

**Fix:** Change `NewOAuthTokenStore` signature to return `(OAuthTokenStore, error)`. Update all call sites to handle the error.

**Files:**
- Modify: `pkg/config/oauth_store.go`
- Modify: `pkg/config/oauth_store_test.go` (5 call sites)
- Modify: `cmd/smolbot/runtime.go`

- [ ] **Step 1: Validate root cause** (see Root Cause Validation section above)

- [ ] **Step 2: Write failing test**

In `pkg/config/oauth_store_test.go`, add:

```go
func TestNewOAuthTokenStoreReturnsErrorOnBadRoot(t *testing.T) {
	// Create a blocking file (can't create directory inside a file)
	tmp := t.TempDir()
	blockingFile := filepath.Join(tmp, "blocking")
	if err := os.WriteFile(blockingFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// NewPaths creates Root() as blockingFile/smolbot — can't create
	paths := NewPaths(filepath.Join(blockingFile, "smolbot"))
	_, err := NewOAuthTokenStore(paths)
	if err == nil {
		t.Fatal("expected error from NewOAuthTokenStore with uncreateable root, got nil")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
go test ./pkg/config/ -run TestNewOAuthTokenStoreReturnsErrorOnBadRoot -v
```

Expected: FAIL — `NewOAuthTokenStore` returns one value (compilation error if we keep old signature, or returns store without error).

- [ ] **Step 4: Change NewOAuthTokenStore signature to return error**

In `pkg/config/oauth_store.go`, replace the `NewOAuthTokenStore` function:

```go
// NewOAuthTokenStore creates an OAuth token store.
// Returns an error if the storage directory cannot be created.
func NewOAuthTokenStore(paths *Paths) (OAuthTokenStore, error) {
	if err := os.MkdirAll(paths.Root(), 0700); err != nil {
		return nil, fmt.Errorf("create oauth token store directory: %w", err)
	}
	return &tokenStore{
		pathFn:  func() string { return filepath.Join(paths.Root(), "oauth_tokens.json") },
		entries: make(map[string]map[string]TokenStoreEntry),
	}, nil
}
```

Add `"fmt"` to imports if not present.

- [ ] **Step 5: Update all call sites in oauth_store_test.go**

In `pkg/config/oauth_store_test.go`, find all 5 existing call sites of the form:
```go
store := NewOAuthTokenStore(paths)
```
or
```go
store := NewOAuthTokenStore(...)
```

Change each to:
```go
store, err := NewOAuthTokenStore(paths)
if err != nil {
    t.Fatalf("NewOAuthTokenStore: %v", err)
}
```

- [ ] **Step 6: Update runtime.go call site**

In `cmd/smolbot/runtime.go` (≈line 634), change:

```go
providerRegistry = provider.NewRegistryWithOAuthStore(cfg, config.NewOAuthTokenStore(paths))
```

to:

```go
oauthStore, err := config.NewOAuthTokenStore(paths)
if err != nil {
    _ = sessions.Close()
    _ = usageStore.Close()
    return nil, fmt.Errorf("create oauth token store: %w", err)
}
providerRegistry = provider.NewRegistryWithOAuthStore(cfg, oauthStore)
```

- [ ] **Step 7: Run tests to verify they pass**

```bash
go test ./pkg/config/ -run TestNewOAuthTokenStore -v
go build ./cmd/smolbot/
```

Expected: all tests PASS, build succeeds.

- [ ] **Step 8: Run all config tests**

```bash
go test ./pkg/config/ -v -count=1 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 9: Commit**

```bash
git add pkg/config/oauth_store.go pkg/config/oauth_store_test.go cmd/smolbot/runtime.go
git commit -m "fix(config): NewOAuthTokenStore returns error on directory creation failure"
```

---

## Task 4: Config model validation at startup (C16)

**Root cause:** `buildRuntime` starts the daemon with no check that `cfg.Agents.Defaults.Model` is set. First chat request fails with cryptic provider error.

**Files:**
- Modify: `cmd/smolbot/runtime.go`
- Modify: `cmd/smolbot/runtime_test.go`

- [ ] **Step 1: Validate root cause** (see Root Cause Validation section above)

- [ ] **Step 2: Write failing test**

In `cmd/smolbot/runtime_test.go`, add:

```go
func TestBuildRuntimeRejectsEmptyModel(t *testing.T) {
	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = ""
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	_, err := buildRuntime(daemonLaunchOptions{
		ConfigPath: cfgPath,
		Port:       port,
	}, runtimeDeps{
		Provider: &fakeRuntimeProvider{},
	})
	if err == nil {
		t.Fatal("expected error for empty model, got nil")
	}
	if !strings.Contains(err.Error(), "model") {
		t.Fatalf("expected error to mention 'model', got: %v", err)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
go test ./cmd/smolbot/ -run TestBuildRuntimeRejectsEmptyModel -v
```

Expected: FAIL

- [ ] **Step 4: Add validation**

In `cmd/smolbot/runtime.go` in `buildRuntime`, find where other config validation occurs (≈line 610-620) and add:

```go
if strings.TrimSpace(cfg.Agents.Defaults.Model) == "" {
	return nil, fmt.Errorf("config: agents.defaults.model is required — run 'smolbot onboard' to configure")
}
```

- [ ] **Step 5: Run test to verify it passes**

```bash
go test ./cmd/smolbot/ -run TestBuildRuntimeRejectsEmptyModel -v
```

Expected: PASS

- [ ] **Step 6: Run all smolbot tests**

```bash
go test ./cmd/smolbot/ -count=1 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 7: Commit**

```bash
git add cmd/smolbot/runtime.go cmd/smolbot/runtime_test.go
git commit -m "fix(runtime): reject empty model at daemon startup with clear error message"
```

---

## Task 5: Cron fsync (H4)

**Root cause:** `cron/service.go:persist()` uses `os.WriteFile` without `fsync`. Kernel crash between write and flush loses all job definitions.

**Files:**
- Modify: `pkg/cron/service.go`

- [ ] **Step 1: Validate root cause** (see Root Cause Validation section above)

- [ ] **Step 2: Write failing test**

In `pkg/cron/service_test.go`, add:

```go
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

	// No .tmp file should remain after successful persist
	if _, err := os.Stat(jobsFile + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("expected .tmp file to be cleaned up after successful persist")
	}

	// jobs.json should exist and contain valid data
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
```

- [ ] **Step 3: Run test — should pass before (no .tmp with current impl)**

```bash
go test ./pkg/cron/ -run TestPersistIsAtomic -v
```

This test verifies atomic rename behavior. The implementation change adds `fsync` which is harder to test directly.

- [ ] **Step 4: Replace os.WriteFile with atomic write + Sync**

In `pkg/cron/service.go`, find `persist()` (≈line 295-307) and replace:

```go
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
	tmp := s.jobsFile + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, s.jobsFile)
}
```

- [ ] **Step 5: Verify tests pass**

```bash
go test ./pkg/cron/ -v -count=1 2>&1 | tail -15
```

Expected: all PASS (including new atomic test)

- [ ] **Step 6: Commit**

```bash
git add pkg/cron/service.go pkg/cron/service_test.go
git commit -m "fix(cron): use atomic write with fsync in persist() to prevent job loss on crash"
```

---

## Task 6: web_search output size cap (H5)

**Root cause:** `WebSearchTool.Execute` (≈line 149-161) has no output size limit. Large results consume context window.

**Files:**
- Modify: `pkg/tool/web.go`
- Modify: `pkg/tool/web_test.go`

- [ ] **Step 1: Validate root cause** (see Root Cause Validation section above)

- [ ] **Step 2: Add test scaffolding**

In `pkg/tool/web_test.go`, add `stubSearchBackend` if not already defined:

```go
type stubSearchBackend struct {
	results []SearchResult
	name    string
}

func (b *stubSearchBackend) Name() string {
	if b.name != "" {
		return b.name
	}
	return "stub"
}
func (b *stubSearchBackend) Available() bool { return true }
func (b *stubSearchBackend) Search(_ context.Context, _ *http.Client, _ string, _ int) ([]SearchResult, error) {
	return b.results, nil
}
```

- [ ] **Step 3: Write failing test**

In `pkg/tool/web_test.go`, add:

```go
func TestWebSearchOutputIsCapped(t *testing.T) {
	longSnippet := strings.Repeat("x", 10_000) // 10KB per result
	results := make([]SearchResult, 10)
	for i := range results {
		results[i] = SearchResult{
			Title:   fmt.Sprintf("Result %d", i),
			URL:     fmt.Sprintf("https://example.com/%d", i),
			Snippet: longSnippet,
		}
	}

	backend := &stubSearchBackend{results: results}
	tool := &WebSearchTool{
		deps: WebDependencies{
			Backends: map[string]SearchBackend{"duckduckgo": backend},
		},
	}

	raw, _ := json.Marshal(map[string]any{"query": "test"})
	result, err := tool.Execute(context.Background(), raw, ToolContext{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// maxSearchOutputBytes + small overhead for partial entry
	if len(result.Output) > maxSearchOutputBytes+1024 {
		t.Fatalf("output length = %d, want <= %d", len(result.Output), maxSearchOutputBytes+1024)
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

```bash
go test ./pkg/tool/ -run TestWebSearchOutputIsCapped -v
```

Expected: FAIL (constant not defined, output too large)

- [ ] **Step 5: Add constant and cap**

In `pkg/tool/web.go` (≈line 21), add:

```go
const maxSearchOutputBytes = 32 * 1024
```

Replace the result-building loop (≈lines 149-161):

```go
var builder strings.Builder
for i, result := range results {
	entry := "Title: " + result.Title + "\nURL: " + result.URL + "\nSnippet: " + result.Snippet
	if builder.Len() > 0 {
		if builder.Len()+len(entry)+2 > maxSearchOutputBytes {
			break
		}
		builder.WriteString("\n\n")
	} else if len(entry) > maxSearchOutputBytes {
		entry = entry[:maxSearchOutputBytes]
	}
	builder.WriteString(entry)
}
return &Result{Output: builder.String()}, nil
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
go test ./pkg/tool/ -run TestWebSearchOutputIsCapped -v
go test ./pkg/tool/ -v -count=1 2>&1 | tail -15
```

Expected: all PASS

- [ ] **Step 7: Commit**

```bash
git add pkg/tool/web.go pkg/tool/web_test.go
git commit -m "fix(tool): cap web_search output at 32KB to prevent context window exhaustion"
```

---

## Task 7: User skills FS bug + HasResource nil panic (K3, C14)

**Root cause (K3):** `registry.go:45` uses `fs.Stat(nanobotgo.EmbeddedAssets, userSkillsDir)` for a real filesystem path. Embedded FS can't contain user paths.

**Root cause (C14):** `registry.go:135` does `fs.Stat(nil, resourcePath)` passing nil FS, which panics.

**Files:**
- Modify: `pkg/skill/registry.go`
- Modify: `pkg/skill/registry_test.go`

- [ ] **Step 1: Validate root cause** (see Root Cause Validation section above)

- [ ] **Step 2: Write failing test for K3**

In `pkg/skill/registry_test.go`:

```go
func TestNewRegistryLoadsUserSkills(t *testing.T) {
	// Create a temp directory with a skill
	tmp := t.TempDir()
	skillsDir := filepath.Join(tmp, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	skillContent := `<?xml version="1.0"?>
<skill>
  <name>my-user-skill</name>
  <description>A test skill</description>
</skill>`
	if err := os.WriteFile(filepath.Join(skillsDir, "my-user-skill.xml"), []byte(skillContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	paths := config.NewPaths(tmp)
	reg, err := NewRegistry(paths)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	if _, ok := reg.Get("my-user-skill"); !ok {
		t.Fatal("user skill 'my-user-skill' not loaded — K3: user skills dir checked against embedded FS")
	}
}
```

- [ ] **Step 3: Write failing test for C14**

```go
func TestHasResourceNonBuiltinDoesNotPanic(t *testing.T) {
	tmp := t.TempDir()
	skillFile := filepath.Join(tmp, "my-skill.xml")
	os.WriteFile(skillFile, []byte(`<?xml version="1.0"?><skill><name>my-skill</name><description>test</description></skill>`), 0o644)
	resourceFile := filepath.Join(tmp, "data.txt")
	os.WriteFile(resourceFile, []byte("resource"), 0o644)

	reg := &Registry{
		skills: map[string]*Skill{
			"my-skill": {
				Name:        "my-skill",
				Path:        skillFile,
				Source:      "user",
				Description: "test",
			},
		},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("HasResource panicked: %v — C14: fs.Stat(nil, path) panics", r)
		}
	}()
	if !reg.HasResource("my-skill", "data.txt") {
		t.Fatal("HasResource returned false for existing resource")
	}
}
```

- [ ] **Step 4: Run tests to verify they fail**

```bash
go test ./pkg/skill/ -run "TestNewRegistryLoadsUserSkills|TestHasResourceNonBuiltinDoesNotPanic" -v
```

Expected: K3 test FAIL (skill not loaded), C14 test FAIL (panic)

- [ ] **Step 5: Fix registry.go**

Add `"os"` to imports. Then fix both issues:

**K3 fix** (line 45): Change:
```go
if _, err := fs.Stat(nanobotgo.EmbeddedAssets, userSkillsDir); err == nil {
```
to:
```go
if _, err := os.Stat(userSkillsDir); err == nil {
```

**C14 fix** (line 135): Change:
```go
_, err := fs.Stat(nil, resourcePath)
```
to:
```go
_, err := os.Stat(resourcePath)
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
go test ./pkg/skill/ -run "TestNewRegistryLoadsUserSkills|TestHasResourceNonBuiltinDoesNotPanic" -v
```

Expected: both PASS

- [ ] **Step 7: Run all skill tests**

```bash
go test ./pkg/skill/ -v -count=1 2>&1 | tail -15
```

Expected: all PASS

- [ ] **Step 8: Commit**

```bash
git add pkg/skill/registry.go pkg/skill/registry_test.go
git commit -m "fix(skill): use os.Stat for user skills dir and HasResource non-builtin path"
```

---

## Task 8: Tokenizer accuracy for non-OpenAI models (M10)

**Root cause:** `tokenizer.New()` uses hardcoded `"cl100k_base"`. Llama/Mistral/Ollama use SentencePiece; cl100k_base overestimates by 20-50%.

**Files:**
- Modify: `pkg/tokenizer/tokenizer.go`
- Modify: `pkg/tokenizer/tokenizer_test.go`
- Modify: `cmd/smolbot/runtime.go`

- [ ] **Step 1: Validate root cause** (see Root Cause Validation section above)

- [ ] **Step 2: Write failing tests**

In `pkg/tokenizer/tokenizer_test.go`:

```go
func TestNewForModelUsesCl100kForGPTModels(t *testing.T) {
	tok := NewForModel("gpt-4o")
	tokens := tok.EstimateTokens("Hello world")
	if tokens < 2 || tokens > 4 {
		t.Fatalf("expected 2-4 tokens for 'Hello world' with gpt-4o, got %d", tokens)
	}
}

func TestNewForModelUsesFallbackForOllamaModels(t *testing.T) {
	tok := NewForModel("ollama/llama3.2")
	tokens := tok.EstimateTokens("Hello world")
	// fallbackEstimate: len("Hello world")/4 = ~3
	if tokens < 1 || tokens > 5 {
		t.Fatalf("expected 1-5 tokens for ollama model, got %d", tokens)
	}
}

func TestNewForEmptyModelUsesCl100k(t *testing.T) {
	// Backward compatibility: empty model should use cl100k_base
	tok := NewForModel("")
	tokens := tok.EstimateTokens("Hello world")
	if tokens < 2 || tokens > 4 {
		t.Fatalf("expected 2-4 tokens for empty model (legacy), got %d", tokens)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./pkg/tokenizer/ -run "TestNewForModel" -v
```

Expected: FAIL (symbol not found)

- [ ] **Step 4: Add NewForModel and fix init()**

In `pkg/tokenizer/tokenizer.go`:

Add `model` field to `Tokenizer`:
```go
type Tokenizer struct {
	model string
	enc   *tiktoken.Tiktoken
	once  sync.Once
	err   error
}
```

Add new constructor:
```go
func New() *Tokenizer {
	return &Tokenizer{}
}

func NewForModel(model string) *Tokenizer {
	return &Tokenizer{model: model}
}
```

Add model-aware encoding selection:
```go
func encodingForModel(model string) string {
	lower := strings.ToLower(model)
	// Strip provider prefix
	if idx := strings.LastIndex(lower, "/"); idx >= 0 {
		lower = lower[idx+1:]
	}
	// cl100k_base families
	cl100kFamilies := []string{"gpt-", "claude", "o1", "o3", "o4", "chatgpt", "text-embedding"}
	for _, prefix := range cl100kFamilies {
		if strings.HasPrefix(lower, prefix) {
			return "cl100k_base"
		}
	}
	// Non-cl100k models (Llama, Mistral, Gemma, Qwen, Ollama) use fallback
	return ""
}

func (t *Tokenizer) init() {
	t.once.Do(func() {
		enc := encodingForModel(t.model)
		if enc == "" {
			if t.model == "" {
				// Backward compatibility: empty model defaults to cl100k_base
				enc = "cl100k_base"
			} else {
				// Non-cl100k model: use fallback estimator
				t.err = fmt.Errorf("no cl100k encoding for model %q", t.model)
				return
			}
		}
		t.enc, t.err = tiktoken.GetEncoding(enc)
	})
}
```

Add imports: `"strings"`, `"fmt"` if not present.

- [ ] **Step 5: Update runtime.go**

In `cmd/smolbot/runtime.go` (≈line 695), change:
```go
Tokenizer: tokenizer.New(),
```
to:
```go
Tokenizer: tokenizer.NewForModel(cfg.Agents.Defaults.Model),
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
go test ./pkg/tokenizer/ -run "TestNewForModel" -v
```

Expected: all PASS

- [ ] **Step 7: Run full test suite**

```bash
go test ./pkg/tokenizer/ ./cmd/smolbot/ -count=1 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 8: Commit**

```bash
git add pkg/tokenizer/tokenizer.go pkg/tokenizer/tokenizer_test.go cmd/smolbot/runtime.go
git commit -m "fix(tokenizer): use fallback estimator for non-cl100k models, add NewForModel"
```

---

## Final Verification

- [ ] **Full test suite**

```bash
go test ./... -count=1 2>&1 | grep -E "FAIL|ok"
```

Expected: all `ok`, zero `FAIL`

- [ ] **Race detector**

```bash
go test -race ./pkg/provider/ ./pkg/config/ ./pkg/cron/ ./pkg/tool/ ./pkg/skill/ ./pkg/tokenizer/ 2>&1 | tail -20
```

Expected: no race conditions detected

- [ ] **Build check**

```bash
go build ./cmd/smolbot/ ./cmd/installer/
```

Expected: no errors

- [ ] **Vet check**

```bash
go vet ./...
```

Expected: no issues

---

## Summary

| Bug ID | Issue | Fix | Task |
|--------|-------|-----|------|
| C8/C9 | Provider prefix stripping | Export `StripProviderPrefix`, call in all providers | Task 1 |
| M2 | Azure streaming usage loss | Handle empty-choices usage chunk, add StreamOptions | Task 2 |
| C13 | OAuth store silent failure | Change `NewOAuthTokenStore` to return `(OAuthTokenStore, error)` | Task 3 |
| C16 | No model validation | Add startup validation | Task 4 |
| H4 | Cron no fsync | Atomic write with Sync | Task 5 |
| H5 | web_search uncapped | Cap output at 32KB | Task 6 |
| K3 | User skills wrong FS | Use `os.Stat` for real paths | Task 7 |
| C14 | HasResource nil panic | Use `os.Stat` instead of `fs.Stat(nil, ...)` | Task 7 |
| M10 | Tokenizer ignores model | Add `NewForModel`, fallback estimator | Task 8 |