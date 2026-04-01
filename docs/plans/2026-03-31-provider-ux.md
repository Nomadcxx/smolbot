# Interactive Provider Management — Comprehensive Implementation Plan

> **Plan B of 2.** Transform the F1 Providers panel from a read-only display into a fully
> interactive configure/remove flow, backed by a static model catalogue, atomic config
> write-back, and live hot-reload of the model list without daemon restart.
>
> **For Claude:** implement task-by-task in order. Each task has a checkpoint commit.
> Do not start Task N+1 until Task N's verification tests pass.

---

## 1. Architecture Overview

### Current state

```
TUI (F1 → Providers)
  └─ ProvidersModel — READ-ONLY, Esc only
       │
       └─ NewProvidersFromData(models, current, status, cfg)
            │
            └─ buildProviderInfoList  ← only shows cfg.Providers entries
                                        does NOT show known-but-unconfigured providers

Gateway models.list
  └─ GetAvailableModels(cfg)
       ├─ Ollama   → live discovery
       ├─ anthropic → EXCLUDED from compatible path, NOT returned
       └─ others   → single stub ModelInfo (Selectable: false)

writeConfigFile  ← os.WriteFile directly, 0o644, non-atomic
```

### Target state

```
TUI (F1 → Providers)
  ├─ Browse mode: cursor nav over ALL known providers (configured + unconfigured)
  └─ Configure mode: inline masked API key form
       │ ConfigureProviderMsg / RemoveProviderMsg / SwitchProviderMsg
       ▼
  tui.go Update  ──provider.configure──►  Gateway
                                           ├─ validate
                                           ├─ update cfg.Providers in memory
                                           ├─ AtomicWriteConfig
                                           ├─ registry.UpdateProviderConfig / RemoveProviderConfig
                                           ├─ GetAvailableModels → now returns catalogue entries
                                           └─ BroadcastEvent("models.updated", {...})
                                                     │
                                          ◄──────────┘
  TUI EventMsg "models.updated"
    ├─ m.models refreshed
    ├─ open model picker rebuilt in place
    └─ open providers dialog rebuilt in place
```

### End result

```
GetAvailableModels:
  ├─ Ollama → live (unchanged)
  ├─ anthropic configured → claude-opus-4-5, claude-sonnet-4-5, etc. (Selectable: true)
  ├─ openai configured    → gpt-4o, gpt-4o-mini, o1, o3-mini, etc. (Selectable: true)
  ├─ gemini configured    → gemini-2.5-pro, gemini-2.0-flash, etc. (Selectable: true)
  ├─ groq configured      → llama-3.3-70b, mixtral-8x7b, etc. (Selectable: true)
  ├─ deepseek configured  → deepseek-chat, deepseek-reasoner (Selectable: true)
  └─ minimax configured   → MiniMax-Text-01, abab6.5s-chat (Selectable: true)
```

---

## 2. Non-Negotiable Invariants

1. **Atomic config writes.** `writeConfigFile` in `cmd/smolbot/runtime.go` uses `os.WriteFile`
   with `0o644`. The new `AtomicWriteConfig` must use `os.CreateTemp` + `os.Rename` with
   `0o600`. The existing var will be replaced.

2. **API keys never appear in logs.** Gateway handlers log provider ID and operation result,
   never `ProviderConfig.APIKey`. The `ProviderConfigureParams` struct must not be logged raw.

3. **Hot-reload does not interrupt active sessions.** `registry.UpdateProviderConfig` /
   `RemoveProviderConfig` hold `r.mu` only during the cache eviction, not during any network
   call. In-flight `agent.ProcessDirect` calls on other providers are unaffected.

4. **Static catalogue is a single source of truth.** All hardcoded model IDs live in
   `pkg/provider/catalogue.go` only.

5. **Unconfigured providers are visible in browse mode but invisible to the model picker.**
   Catalogue models for a provider only flow into `GetAvailableModels` when that provider has
   a non-empty `APIKey` (or OAuth `AuthType`) in `cfg.Providers`.

6. **`models.updated` broadcast is fire-and-forget.** TUI handles it by refreshing its local
   `m.models` slice. It does not reset session state, scroll position, or clear pending input.

---

## 3. Task 1 — Static Model Catalogue

### Files changed

| Action | Path |
|--------|------|
| **Create** | `pkg/provider/catalogue.go` |
| **Modify** | `pkg/provider/discovery.go` |
| **Create** | `pkg/provider/catalogue_test.go` |

### 3.1 Create `pkg/provider/catalogue.go`

```go
package provider

// CatalogueEntry is a statically known model for a provider.
type CatalogueEntry struct {
    ID          string // model ID passed to the API
    Name        string // display name
    Capability  string // "chat", "reasoning", "vision"
}

// providerCatalogue maps provider ID → ordered slice of known models.
// Order within each slice: newest / most capable first.
// Only API-key providers are listed; Ollama uses live discovery.
var providerCatalogue = map[string][]CatalogueEntry{
    "anthropic": {
        {ID: "claude-opus-4-5",           Name: "Claude Opus 4.5",    Capability: "chat"},
        {ID: "claude-sonnet-4-5-20251001", Name: "Claude Sonnet 4.5",  Capability: "chat"},
        {ID: "claude-haiku-4-5-20251001",  Name: "Claude Haiku 4.5",   Capability: "chat"},
        {ID: "claude-opus-4",             Name: "Claude Opus 4",      Capability: "chat"},
        {ID: "claude-sonnet-4",           Name: "Claude Sonnet 4",    Capability: "chat"},
        {ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet", Capability: "chat"},
        {ID: "claude-3-5-haiku-20241022",  Name: "Claude 3.5 Haiku",  Capability: "chat"},
    },
    "openai": {
        {ID: "gpt-4o",       Name: "GPT-4o",       Capability: "chat"},
        {ID: "gpt-4o-mini",  Name: "GPT-4o mini",  Capability: "chat"},
        {ID: "gpt-4-turbo",  Name: "GPT-4 Turbo",  Capability: "chat"},
        {ID: "o1",           Name: "o1",            Capability: "reasoning"},
        {ID: "o1-mini",      Name: "o1-mini",       Capability: "reasoning"},
        {ID: "o3-mini",      Name: "o3-mini",       Capability: "reasoning"},
        {ID: "o3",           Name: "o3",            Capability: "reasoning"},
    },
    "gemini": {
        {ID: "gemini-2.5-pro-preview-03-25", Name: "Gemini 2.5 Pro",    Capability: "chat"},
        {ID: "gemini-2.0-flash",             Name: "Gemini 2.0 Flash",  Capability: "chat"},
        {ID: "gemini-2.0-flash-lite",        Name: "Gemini 2.0 Flash Lite", Capability: "chat"},
        {ID: "gemini-1.5-pro-002",           Name: "Gemini 1.5 Pro",    Capability: "chat"},
        {ID: "gemini-1.5-flash-002",         Name: "Gemini 1.5 Flash",  Capability: "chat"},
    },
    "groq": {
        {ID: "llama-3.3-70b-versatile", Name: "Llama 3.3 70B",  Capability: "chat"},
        {ID: "llama-3.1-8b-instant",    Name: "Llama 3.1 8B",   Capability: "chat"},
        {ID: "mixtral-8x7b-32768",      Name: "Mixtral 8x7B",   Capability: "chat"},
        {ID: "gemma2-9b-it",            Name: "Gemma 2 9B",     Capability: "chat"},
    },
    "deepseek": {
        {ID: "deepseek-chat",     Name: "DeepSeek Chat",     Capability: "chat"},
        {ID: "deepseek-reasoner", Name: "DeepSeek Reasoner", Capability: "reasoning"},
    },
    "minimax": {
        {ID: "MiniMax-Text-01", Name: "MiniMax Text-01", Capability: "chat"},
        {ID: "abab6.5s-chat",   Name: "ABAB 6.5s",       Capability: "chat"},
    },
}

// CatalogueModels returns the static model list for providerID, or nil if unknown.
func CatalogueModels(providerID string) []CatalogueEntry {
    return providerCatalogue[providerID]
}

// KnownProviderIDs returns all provider IDs that have catalogue entries, sorted.
func KnownProviderIDs() []string {
    ids := make([]string, 0, len(providerCatalogue))
    for id := range providerCatalogue {
        ids = append(ids, id)
    }
    sort.Strings(ids)
    return ids
}
```

Add `"sort"` to the import if needed.

### 3.2 Modify `pkg/provider/discovery.go`

**Change 1** — replace the `isConfiguredCompatibleProvider` function body. Currently it
excludes `"anthropic"`. The new version routes anthropic (and all other known-catalogue
providers) through the catalogue path instead:

```go
// isConfiguredCompatibleProvider returns true for providers that should receive
// a single OpenAI-compatible stub when they have no catalogue entry.
// Providers with catalogue entries are handled separately in GetAvailableModels.
func isConfiguredCompatibleProvider(providerID string) bool {
    switch strings.TrimSpace(providerID) {
    case "", "ollama":
        return false
    default:
        // If the provider has a catalogue, it is handled by the catalogue path.
        if len(CatalogueModels(providerID)) > 0 {
            return false
        }
        return true
    }
}
```

**Change 2** — in `GetAvailableModels`, after the Ollama block and before the
`configuredCompatibleProviders` loop, add a catalogue block:

```go
// Catalogue providers: emit one ModelInfo per catalogue entry for each
// provider that is configured with a non-empty API key.
for _, providerID := range KnownProviderIDs() {
    pc, ok := cfg.Providers[providerID]
    if !ok {
        continue
    }
    // OAuth providers (minimax-portal) skip the catalogue; they have their own flow.
    if pc.AuthType == "oauth" {
        continue
    }
    if strings.TrimSpace(pc.APIKey) == "" {
        continue
    }
    for _, entry := range CatalogueModels(providerID) {
        models = appendUniqueModel(models, seen, ModelInfo{
            ID:          entry.ID,
            Name:        entry.Name,
            Provider:    providerID,
            Capability:  entry.Capability,
            Source:      "catalogue",
            Selectable:  true,
        })
    }
}
```

**Change 3** — the fallback block at the end of `GetAvailableModels` adds the current
configured model if it is not already seen. This stays unchanged — it ensures the active
model always appears even if the catalogue doesn't know it.

**No other changes** to `discovery.go`.

### 3.3 Create `pkg/provider/catalogue_test.go`

```go
package provider_test

import (
    "testing"
    "github.com/Nomadcxx/smolbot/pkg/config"
    "github.com/Nomadcxx/smolbot/pkg/provider"
)

func TestCatalogueModels_AnthropicReturnsEntries(t *testing.T) {
    entries := provider.CatalogueModels("anthropic")
    if len(entries) == 0 {
        t.Fatal("expected anthropic catalogue entries, got none")
    }
    // First entry should be the most capable model.
    for _, e := range entries {
        if e.ID == "" || e.Name == "" {
            t.Errorf("entry has empty ID or Name: %+v", e)
        }
    }
}

func TestCatalogueModels_UnknownProviderReturnsNil(t *testing.T) {
    if provider.CatalogueModels("nonexistent") != nil {
        t.Fatal("expected nil for unknown provider")
    }
}

func TestGetAvailableModels_AnthropicCatalogueWhenConfigured(t *testing.T) {
    cfg := &config.Config{
        Providers: map[string]config.ProviderConfig{
            "anthropic": {APIKey: "sk-ant-test"},
        },
    }
    models, err := provider.GetAvailableModels(cfg)
    if err != nil {
        t.Fatal(err)
    }
    // Must contain at least one anthropic catalogue model.
    var found bool
    for _, m := range models {
        if m.Provider == "anthropic" && m.Source == "catalogue" && m.Selectable {
            found = true
            break
        }
    }
    if !found {
        t.Fatalf("expected selectable anthropic catalogue model, got: %+v", models)
    }
}

func TestGetAvailableModels_AnthropicCatalogueAbsentWhenNotConfigured(t *testing.T) {
    cfg := &config.Config{
        Providers: map[string]config.ProviderConfig{},
    }
    models, err := provider.GetAvailableModels(cfg)
    if err != nil {
        t.Fatal(err)
    }
    for _, m := range models {
        if m.Provider == "anthropic" && m.Source == "catalogue" {
            t.Fatalf("unexpected anthropic catalogue model when not configured: %+v", m)
        }
    }
}

func TestGetAvailableModels_GroqCatalogueWhenConfigured(t *testing.T) {
    cfg := &config.Config{
        Providers: map[string]config.ProviderConfig{
            "groq": {APIKey: "gsk_test"},
        },
    }
    models, _ := provider.GetAvailableModels(cfg)
    var count int
    for _, m := range models {
        if m.Provider == "groq" && m.Source == "catalogue" {
            count++
        }
    }
    if count == 0 {
        t.Fatal("expected groq catalogue models")
    }
}

func TestGetAvailableModels_DeepSeekCatalogueWhenConfigured(t *testing.T) {
    cfg := &config.Config{
        Providers: map[string]config.ProviderConfig{
            "deepseek": {APIKey: "ds_test"},
        },
    }
    models, _ := provider.GetAvailableModels(cfg)
    var count int
    for _, m := range models {
        if m.Provider == "deepseek" && m.Source == "catalogue" {
            count++
        }
    }
    if count == 0 {
        t.Fatal("expected deepseek catalogue models")
    }
}

func TestKnownProviderIDs_IsSorted(t *testing.T) {
    ids := provider.KnownProviderIDs()
    for i := 1; i < len(ids); i++ {
        if ids[i] < ids[i-1] {
            t.Fatalf("KnownProviderIDs not sorted: %v", ids)
        }
    }
}
```

### Checkpoint commit

```
feat(provider): add static model catalogue for anthropic/openai/gemini/groq/deepseek/minimax
```

---

## 4. Task 2 — Config Write-Back and Hot-Reload

### Files changed

| Action | Path |
|--------|------|
| **Modify** | `cmd/smolbot/runtime.go` — replace `writeConfigFile` var with `AtomicWriteConfig` call |
| **Modify** | `pkg/provider/registry.go` — add `Invalidate`, `UpdateProviderConfig`, `RemoveProviderConfig` |
| **Modify** | `pkg/gateway/server.go` — add `ServerDeps.ConfigPath`, `ServerDeps.Registry`, `ProviderRegistry` interface, gateway handlers |
| **Modify** | `cmd/smolbot/runtime.go` — pass `ConfigPath` and `Registry` in `ServerDeps` |
| **Modify** | `internal/client/protocol.go` — add new param/payload types |
| **Modify** | `internal/client/messages.go` — add `ProviderConfigure`, `ProviderRemove` |
| **Create** | `pkg/config/writeback.go` |
| **Create** | `pkg/config/writeback_test.go` |
| **Create** | `pkg/provider/registry_invalidate_test.go` |
| **Create** | `pkg/gateway/provider_handler_test.go` |

### 4.1 Create `pkg/config/writeback.go`

```go
package config

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
)

// AtomicWriteConfig serialises cfg to a temp file in the same directory as path,
// then renames it atomically. This prevents partial writes from corrupting config.
// The file is written with 0o600 permissions (owner read/write only).
func AtomicWriteConfig(path string, cfg *Config) error {
    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
        return fmt.Errorf("config: mkdir: %w", err)
    }
    data, err := json.MarshalIndent(cfg, "", "  ")
    if err != nil {
        return fmt.Errorf("config: marshal: %w", err)
    }
    tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.json.tmp")
    if err != nil {
        return fmt.Errorf("config: create temp: %w", err)
    }
    tmpName := tmp.Name()
    if _, err := tmp.Write(data); err != nil {
        tmp.Close()
        os.Remove(tmpName)
        return fmt.Errorf("config: write temp: %w", err)
    }
    if err := tmp.Chmod(0o600); err != nil {
        tmp.Close()
        os.Remove(tmpName)
        return fmt.Errorf("config: chmod: %w", err)
    }
    if err := tmp.Close(); err != nil {
        os.Remove(tmpName)
        return fmt.Errorf("config: close temp: %w", err)
    }
    if err := os.Rename(tmpName, path); err != nil {
        os.Remove(tmpName)
        return fmt.Errorf("config: rename: %w", err)
    }
    return nil
}
```

### 4.2 Create `pkg/config/writeback_test.go`

```go
package config_test

import (
    "encoding/json"
    "os"
    "path/filepath"
    "testing"
    "github.com/Nomadcxx/smolbot/pkg/config"
)

func TestAtomicWriteConfig_WritesValidJSON(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "config.json")
    cfg := config.DefaultConfig()
    cfg.Agents.Defaults.Model = "test-model"

    if err := config.AtomicWriteConfig(path, &cfg); err != nil {
        t.Fatal(err)
    }
    data, err := os.ReadFile(path)
    if err != nil {
        t.Fatal(err)
    }
    var out config.Config
    if err := json.Unmarshal(data, &out); err != nil {
        t.Fatalf("written file is not valid JSON: %v", err)
    }
    if out.Agents.Defaults.Model != "test-model" {
        t.Fatalf("model mismatch: got %q", out.Agents.Defaults.Model)
    }
}

func TestAtomicWriteConfig_CreatesFileWhenAbsent(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "sub", "config.json")
    cfg := config.DefaultConfig()

    if err := config.AtomicWriteConfig(path, &cfg); err != nil {
        t.Fatal(err)
    }
    if _, err := os.Stat(path); os.IsNotExist(err) {
        t.Fatal("file not created")
    }
}

func TestAtomicWriteConfig_FilePermissions(t *testing.T) {
    if os.Getuid() == 0 {
        t.Skip("root ignores file permissions")
    }
    dir := t.TempDir()
    path := filepath.Join(dir, "config.json")
    cfg := config.DefaultConfig()

    if err := config.AtomicWriteConfig(path, &cfg); err != nil {
        t.Fatal(err)
    }
    info, err := os.Stat(path)
    if err != nil {
        t.Fatal(err)
    }
    if got := info.Mode().Perm(); got != 0o600 {
        t.Fatalf("expected 0o600, got %04o", got)
    }
}
```

### 4.3 Modify `pkg/provider/registry.go` — add invalidation methods

Add three methods to `*Registry`. They go immediately after `CurrentModel()`:

```go
// Invalidate removes the cached provider instance for providerID so that the
// next ForModel call rebuilds it from current config. Safe to call concurrently
// with in-flight ForModel calls on other providers — the lock is held only
// during cache eviction.
func (r *Registry) Invalidate(providerID string) {
    r.mu.Lock()
    defer r.mu.Unlock()
    delete(r.cache, providerID)
    // Also evict OAuth cache variants keyed as "providerID:profileID".
    for k := range r.cache {
        if strings.HasPrefix(k, providerID+":") {
            delete(r.cache, k)
        }
    }
}

// UpdateProviderConfig writes a new ProviderConfig into the registry's config
// under the registry lock, then evicts the cached provider instance.
// The next ForModel call for this provider will use the new config.
func (r *Registry) UpdateProviderConfig(providerID string, pc config.ProviderConfig) {
    r.mu.Lock()
    defer r.mu.Unlock()
    if r.cfg.Providers == nil {
        r.cfg.Providers = make(map[string]config.ProviderConfig)
    }
    r.cfg.Providers[providerID] = pc
    delete(r.cache, providerID)
    for k := range r.cache {
        if strings.HasPrefix(k, providerID+":") {
            delete(r.cache, k)
        }
    }
}

// RemoveProviderConfig removes the ProviderConfig entry for providerID and
// evicts the cached provider instance.
func (r *Registry) RemoveProviderConfig(providerID string) {
    r.mu.Lock()
    defer r.mu.Unlock()
    delete(r.cfg.Providers, providerID)
    delete(r.cache, providerID)
    for k := range r.cache {
        if strings.HasPrefix(k, providerID+":") {
            delete(r.cache, k)
        }
    }
}
```

`"strings"` is already imported in `registry.go`.

### 4.4 Create `pkg/provider/registry_invalidate_test.go`

```go
package provider_test

import (
    "testing"
    "github.com/Nomadcxx/smolbot/pkg/config"
    "github.com/Nomadcxx/smolbot/pkg/provider"
)

func TestRegistry_Invalidate_ClearsCache(t *testing.T) {
    cfg := &config.Config{
        Agents:    config.AgentsConfig{Defaults: config.AgentDefaults{Model: "gpt-4o", Provider: "openai"}},
        Providers: map[string]config.ProviderConfig{"openai": {APIKey: "k1"}},
    }
    r := provider.NewRegistryWithDefaults(cfg)
    // Warm the cache.
    p1, err := r.ForModel("gpt-4o")
    if err != nil {
        t.Fatal(err)
    }
    r.Invalidate("openai")
    p2, err := r.ForModel("gpt-4o")
    if err != nil {
        t.Fatal(err)
    }
    // After invalidation a new instance is returned.
    if p1 == p2 {
        t.Fatal("expected new provider instance after Invalidate")
    }
}

func TestRegistry_UpdateProviderConfig_ReflectsNewKey(t *testing.T) {
    cfg := &config.Config{
        Agents:    config.AgentsConfig{Defaults: config.AgentDefaults{Model: "gpt-4o", Provider: "openai"}},
        Providers: map[string]config.ProviderConfig{"openai": {APIKey: "k1"}},
    }
    r := provider.NewRegistryWithDefaults(cfg)
    r.UpdateProviderConfig("openai", config.ProviderConfig{APIKey: "k2"})
    // The config map should reflect the new key.
    if cfg.Providers["openai"].APIKey != "k2" {
        t.Fatalf("expected APIKey=k2, got %q", cfg.Providers["openai"].APIKey)
    }
}

func TestRegistry_RemoveProviderConfig_RemovesEntry(t *testing.T) {
    cfg := &config.Config{
        Providers: map[string]config.ProviderConfig{"openai": {APIKey: "k1"}},
    }
    r := provider.NewRegistryWithDefaults(cfg)
    r.RemoveProviderConfig("openai")
    if _, ok := cfg.Providers["openai"]; ok {
        t.Fatal("expected openai to be removed from config.Providers")
    }
}
```

### 4.5 Modify `pkg/gateway/server.go`

**Step A — add `ProviderRegistry` interface** near the top of the file (after package + imports):

```go
// ProviderRegistry is the subset of *provider.Registry used by the gateway
// for hot-reload after provider configuration changes.
type ProviderRegistry interface {
    UpdateProviderConfig(providerID string, pc config.ProviderConfig)
    RemoveProviderConfig(providerID string)
}
```

Note: `config` here refers to `github.com/Nomadcxx/smolbot/pkg/config`. This import is
already present in `server.go`.

**Step B — extend `ServerDeps`:**

```go
type ServerDeps struct {
    Agent            AgentProcessor
    Cron             CronLister
    Sessions         *session.Store
    Channels         *channel.Manager
    Config           *config.Config
    Usage            UsageSummaryReader
    Skills           *skill.Registry
    MCPTools         MCPToolCounter
    Version          string
    StartedAt        time.Time
    SetModelCallback func(model string) (string, error)
    // New fields:
    ConfigPath string           // absolute path to config.json; enables write-back
    Registry   ProviderRegistry // enables hot-reload after configure/remove
}
```

**Step C — extend `Server` struct** (add two fields to the existing struct, after `setModelCallback`):

```go
configPath string
registry   ProviderRegistry
```

**Step D — wire new fields in `NewServer`:**

```go
func NewServer(deps ServerDeps) *Server {
    s := &Server{
        // ...existing assignments...
        configPath: deps.ConfigPath,
        registry:   deps.Registry,
    }
    // ...rest of NewServer...
}
```

**Step E — add a `currentModel()` helper** (if not already present):

```go
func (s *Server) currentModel() string {
    if s.config == nil {
        return ""
    }
    return s.config.Agents.Defaults.Model
}
```

**Step F — add `clientModels` conversion helper:**

```go
// clientModels converts []provider.ModelInfo to []clientproto.ModelInfo.
// Both structs have identical fields; this avoids a JSON round-trip.
func clientModels(in []provider.ModelInfo) []clientproto.ModelInfo {
    out := make([]clientproto.ModelInfo, len(in))
    for i, m := range in {
        out[i] = clientproto.ModelInfo{
            ID:          m.ID,
            Name:        m.Name,
            Provider:    m.Provider,
            Description: m.Description,
            Source:      m.Source,
            Capability:  m.Capability,
            Selectable:  m.Selectable,
        }
    }
    return out
}
```

Note: `clientproto` is the import alias for `internal/client`. Check existing import alias
in `server.go` and use whatever it is (may be `client` or `clientproto`).

**Step G — add `provider.configure` and `provider.remove` cases** in the `handleRequest`
switch (in `server.go`). Add them after the existing `models.set` case:

```go
case "provider.configure":
    var params struct {
        Provider string `json:"provider"`
        APIKey   string `json:"apiKey"`
        APIBase  string `json:"apiBase,omitempty"`
    }
    if err := json.Unmarshal(req.Params, &params); err != nil {
        return nil, fmt.Errorf("provider.configure: bad params: %w", err)
    }
    if strings.TrimSpace(params.Provider) == "" {
        return nil, fmt.Errorf("provider.configure: missing provider")
    }
    if strings.TrimSpace(params.APIKey) == "" {
        return nil, fmt.Errorf("provider.configure: missing apiKey")
    }
    // NEVER log params.APIKey
    pc := config.ProviderConfig{
        APIKey:  strings.TrimSpace(params.APIKey),
        APIBase: strings.TrimSpace(params.APIBase),
    }
    if s.config.Providers == nil {
        s.config.Providers = make(map[string]config.ProviderConfig)
    }
    s.config.Providers[params.Provider] = pc
    if s.registry != nil {
        s.registry.UpdateProviderConfig(params.Provider, pc)
    }
    if s.configPath != "" {
        if err := config.AtomicWriteConfig(s.configPath, s.config); err != nil {
            return nil, fmt.Errorf("provider.configure: write config: %w", err)
        }
    }
    models, _ := provider.GetAvailableModels(s.config)
    current := s.currentModel()
    s.BroadcastEvent("models.updated", map[string]any{
        "models":  clientModels(models),
        "current": current,
    })
    return map[string]any{
        "provider": params.Provider,
        "models":   clientModels(models),
        "current":  current,
    }, nil

case "provider.remove":
    var params struct {
        Provider string `json:"provider"`
    }
    if err := json.Unmarshal(req.Params, &params); err != nil {
        return nil, fmt.Errorf("provider.remove: bad params: %w", err)
    }
    if strings.TrimSpace(params.Provider) == "" {
        return nil, fmt.Errorf("provider.remove: missing provider")
    }
    if s.config.Providers != nil {
        delete(s.config.Providers, params.Provider)
    }
    if s.registry != nil {
        s.registry.RemoveProviderConfig(params.Provider)
    }
    if s.configPath != "" {
        if err := config.AtomicWriteConfig(s.configPath, s.config); err != nil {
            return nil, fmt.Errorf("provider.remove: write config: %w", err)
        }
    }
    models, _ := provider.GetAvailableModels(s.config)
    current := s.currentModel()
    s.BroadcastEvent("models.updated", map[string]any{
        "models":  clientModels(models),
        "current": current,
    })
    return map[string]any{
        "provider": params.Provider,
        "models":   clientModels(models),
        "current":  current,
    }, nil
```

Required imports in `server.go` (add if missing):
- `"github.com/Nomadcxx/smolbot/pkg/config"` — already present
- `"github.com/Nomadcxx/smolbot/pkg/provider"` — likely already present (used in models.list)

**Step H — update the `hello` method** response to include the new method and event names:

```go
// In methods list, add:
"provider.configure", "provider.remove",

// In events list, add:
"models.updated",
```

### 4.6 Modify `cmd/smolbot/runtime.go`

**Replace the `writeConfigFile` var** with a call to `AtomicWriteConfig`. The var is:

```go
var writeConfigFile = func(path string, cfg *config.Config) error {
    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
        return err
    }
    data, err := json.MarshalIndent(cfg, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(path, data, 0o644)
}
```

Replace the body so it delegates to `AtomicWriteConfig`:

```go
var writeConfigFile = func(path string, cfg *config.Config) error {
    return config.AtomicWriteConfig(path, cfg)
}
```

This preserves the var (still overridable in tests) while using the atomic + 0o600 path.

**Add `ConfigPath` and `Registry` to `ServerDeps`** in the `gateway.NewServer(gateway.ServerDeps{...})` call. The config path comes from wherever `Load(path)` was called — pass it through `buildRuntime` or store it in `runtimeApp`. The exact mechanism depends on how `buildRuntime` receives the path; inspect the call site and pass it through. Example:

```go
gateway: gateway.NewServer(gateway.ServerDeps{
    // ...existing fields...
    ConfigPath: configPath,          // the path passed to config.Load()
    Registry:   providerRegistry,    // *provider.Registry created above
    SetModelCallback: func(model string) (string, error) {
        // ...unchanged...
    },
}),
```

`providerRegistry` is already assigned earlier in `buildRuntime` (see the existing code:
`providerRegistry := deps.ProviderRegistry` / `provider.NewRegistryWithOAuthStore(...)`).
It is of type `*provider.Registry` which satisfies `gateway.ProviderRegistry` after Step 4.3.

### 4.7 Modify `internal/client/protocol.go` — add new types

Append at the end of the file:

```go
// ProviderConfigureParams are the parameters for the provider.configure method.
type ProviderConfigureParams struct {
    Provider string `json:"provider"`
    APIKey   string `json:"apiKey"`
    APIBase  string `json:"apiBase,omitempty"`
}

// ProviderConfigurePayload is returned by provider.configure and broadcast via models.updated.
type ProviderConfigurePayload struct {
    Provider string      `json:"provider"`
    Models   []ModelInfo `json:"models"`
    Current  string      `json:"current"`
}

// ProviderRemoveParams are the parameters for the provider.remove method.
type ProviderRemoveParams struct {
    Provider string `json:"provider"`
}

// ProviderRemovePayload is returned by provider.remove.
type ProviderRemovePayload struct {
    Provider string      `json:"provider"`
    Models   []ModelInfo `json:"models"`
    Current  string      `json:"current"`
}

// ModelsUpdatedPayload is the payload of the "models.updated" broadcast event.
type ModelsUpdatedPayload struct {
    Models  []ModelInfo `json:"models"`
    Current string      `json:"current"`
}
```

### 4.8 Modify `internal/client/messages.go` — add client methods

Add after the existing `CronJobs` method:

```go
// ProviderConfigure calls provider.configure on the daemon, updating the API key
// for providerID and triggering a hot-reload of the model list.
func (c *Client) ProviderConfigure(providerID, apiKey, apiBase string) (ProviderConfigurePayload, error) {
    res, err := c.sendRequest("provider.configure", ProviderConfigureParams{
        Provider: providerID,
        APIKey:   apiKey,
        APIBase:  apiBase,
    })
    if err != nil {
        return ProviderConfigurePayload{}, err
    }
    var payload ProviderConfigurePayload
    if err := json.Unmarshal(res.Payload, &payload); err != nil {
        return ProviderConfigurePayload{}, fmt.Errorf("provider.configure: decode response: %w", err)
    }
    return payload, nil
}

// ProviderRemove calls provider.remove on the daemon, removing the API key for
// providerID and triggering a hot-reload of the model list.
func (c *Client) ProviderRemove(providerID string) (ProviderRemovePayload, error) {
    res, err := c.sendRequest("provider.remove", ProviderRemoveParams{
        Provider: providerID,
    })
    if err != nil {
        return ProviderRemovePayload{}, err
    }
    var payload ProviderRemovePayload
    if err := json.Unmarshal(res.Payload, &payload); err != nil {
        return ProviderRemovePayload{}, fmt.Errorf("provider.remove: decode response: %w", err)
    }
    return payload, nil
}
```

### Checkpoint commit

```
feat(gateway): provider.configure and provider.remove with atomic write-back and hot-reload
```

---

## 5. Task 3 — Interactive Providers Component

### Files changed

| Action | Path |
|--------|------|
| **Modify** | `internal/components/dialog/providers.go` |
| **Create** | `internal/components/dialog/providers_test.go` |

### 5.1 New exported message types

Add to `providers.go` (after the imports block, before `ProviderInfo`):

```go
// ConfigureProviderMsg is emitted when the user submits a valid API key form.
// Handled by tui.go which calls client.ProviderConfigure.
type ConfigureProviderMsg struct {
    ProviderID string
    APIKey     string
    APIBase    string
}

// RemoveProviderMsg is emitted when the user confirms provider removal.
type RemoveProviderMsg struct {
    ProviderID string
}

// SwitchProviderMsg is emitted when Enter is pressed on an already-configured
// provider row. tui.go handles it the same way as ModelChosenMsg.
type SwitchProviderMsg struct {
    ProviderID string
}
```

### 5.2 Add `providerMode` and `providerMeta`

```go
type providerMode int

const (
    providerModeBrowse     providerMode = iota // cursor navigation
    providerModeConfigure                      // inline API key form
    providerModeConfirming                     // "Remove X?" confirmation
)

// providerMeta holds display metadata for known providers.
// Used in browse mode to show ALL providers, even unconfigured ones.
type providerMetaEntry struct {
    DisplayName string
    Description string
}

var providerMeta = map[string]providerMetaEntry{
    "anthropic": {"Anthropic",      "Claude models — console.anthropic.com"},
    "openai":    {"OpenAI",         "GPT & o-series — platform.openai.com"},
    "gemini":    {"Google Gemini",  "Gemini models — aistudio.google.com"},
    "groq":      {"Groq",           "Fast inference — console.groq.com"},
    "deepseek":  {"DeepSeek",       "DeepSeek models — platform.deepseek.com"},
    "minimax":   {"MiniMax",        "MiniMax models — platform.minimax.io"},
    "ollama":    {"Ollama",         "Local models — no API key needed"},
}
```

### 5.3 Extend `ProviderInfo` struct

Add `Description string` to the existing `ProviderInfo` struct:

```go
type ProviderInfo struct {
    Name        string
    Type        string
    APIBase     string
    HasAuth     bool
    IsOAuth     bool
    IsActive    bool
    IsPartial   bool
    Description string // from providerMeta
}
```

### 5.4 Extend `ProvidersModel` struct

Replace the existing struct definition:

```go
type ProvidersModel struct {
    // existing fields
    rows           []providerRenderRow
    activeProvider string
    activeModel    string
    termWidth      int

    // new navigation fields
    mode           providerMode
    cursor         int    // index into selectableRows
    selectableRows []int  // indexes into rows[] that represent navigable providers

    // configure mode fields
    configTarget   string // provider ID being configured
    configAPIKey   string // current text in API key input
    configAPIBase  string // current text in API base input (openai only)
    configFocused  int    // 0 = API key field, 1 = API base field
    configError    string // error from last configure attempt
    configWorking  bool   // true while RPC is in flight

    // confirm mode
    confirmTarget  string // provider ID pending removal confirmation
}
```

### 5.5 Extend `buildProviderInfoList` to include ALL known providers

Currently `buildProviderInfoList` only shows providers from `cfg.Providers` (configured) plus
the active one. Rewrite it to merge ALL providers from `providerMeta` so unconfigured ones
appear in browse mode:

```go
func buildProviderInfoList(models []client.ModelInfo, currentModel, activeProvider string, cfg *cfgpkg.Config) []ProviderInfo {
    configuredProviders := make(map[string]cfgpkg.ProviderConfig)
    if cfg != nil {
        for name, pc := range cfg.Providers {
            configuredProviders[name] = pc
        }
    }

    seen := make(map[string]bool)
    var infoList []ProviderInfo

    // Active provider first.
    if activeProvider != "" {
        seen[activeProvider] = true
        pc := configuredProviders[activeProvider]
        meta := providerMeta[activeProvider]
        infoList = append(infoList, ProviderInfo{
            Name:        activeProvider,
            Type:        providerTypeName(activeProvider),
            APIBase:     pc.APIBase,
            HasAuth:     pc.APIKey != "" || pc.AuthType == "oauth",
            IsOAuth:     pc.AuthType == "oauth",
            IsActive:    true,
            Description: meta.Description,
        })
    }

    // All providers from providerMeta (known catalogue + ollama), sorted.
    knownIDs := make([]string, 0, len(providerMeta))
    for id := range providerMeta {
        knownIDs = append(knownIDs, id)
    }
    sort.Strings(knownIDs)
    for _, name := range knownIDs {
        if seen[name] {
            continue
        }
        seen[name] = true
        pc := configuredProviders[name]
        meta := providerMeta[name]
        hasAuth := pc.APIKey != "" || pc.AuthType == "oauth"
        isOAuth := pc.AuthType == "oauth"
        isPartial := !hasAuth && pc.APIBase == ""
        infoList = append(infoList, ProviderInfo{
            Name:        name,
            Type:        providerTypeName(name),
            APIBase:     pc.APIBase,
            HasAuth:     hasAuth,
            IsOAuth:     isOAuth,
            IsPartial:   isPartial,
            Description: meta.Description,
        })
    }

    // Any additional configured providers not in providerMeta.
    extras := make([]string, 0)
    for name := range configuredProviders {
        if !seen[name] {
            extras = append(extras, name)
        }
    }
    sort.Strings(extras)
    for _, name := range extras {
        pc := configuredProviders[name]
        hasAuth := pc.APIKey != "" || pc.AuthType == "oauth"
        infoList = append(infoList, ProviderInfo{
            Name:    name,
            Type:    providerTypeName(name),
            APIBase: pc.APIBase,
            HasAuth: hasAuth,
            IsOAuth: pc.AuthType == "oauth",
        })
    }

    return infoList
}

func providerTypeName(providerID string) string {
    switch providerID {
    case "anthropic":    return "Anthropic"
    case "openai":       return "OpenAI Compatible"
    case "gemini":       return "Google Gemini"
    case "groq":         return "Groq"
    case "deepseek":     return "DeepSeek"
    case "minimax":      return "MiniMax"
    case "minimax-portal": return "MiniMax OAuth"
    case "ollama":       return "Ollama"
    case "azure_openai": return "Azure OpenAI"
    default:             return "OpenAI Compatible"
    }
}
```

Add `"sort"` to imports if not present.

### 5.6 Update `NewProvidersFromData` to compute `selectableRows`

After building `rows`, compute `selectableRows` and set initial cursor:

```go
func NewProvidersFromData(models []client.ModelInfo, current string, status client.StatusPayload, cfg *cfgpkg.Config) ProvidersModel {
    currentModel := firstNonEmptyString(current, status.Model, "")
    activeProvider := providerNameForModel(models, currentModel)
    info := buildProviderInfoList(models, currentModel, activeProvider, cfg)
    rows := buildProviderRows(info, activeProvider, currentModel)

    selectable := make([]int, 0)
    for i, row := range rows {
        if row.kind == "provider" {
            selectable = append(selectable, i)
        }
    }

    // Start cursor on the active provider.
    cursor := 0
    for i, rowIdx := range selectable {
        if rows[rowIdx].isActive {
            cursor = i
            break
        }
    }

    return ProvidersModel{
        rows:           rows,
        activeProvider: activeProvider,
        activeModel:    currentModel,
        selectableRows: selectable,
        cursor:         cursor,
    }
}
```

### 5.7 Rewrite `Update` to handle all modes

Replace the existing `Update` method (currently just handles Esc):

```go
func (m ProvidersModel) Update(msg tea.Msg) (ProvidersModel, tea.Cmd) {
    switch m.mode {
    case providerModeBrowse:
        return m.updateBrowse(msg)
    case providerModeConfigure:
        return m.updateConfigure(msg)
    case providerModeConfirming:
        return m.updateConfirm(msg)
    }
    return m, nil
}

func (m ProvidersModel) updateBrowse(msg tea.Msg) (ProvidersModel, tea.Cmd) {
    key, ok := msg.(tea.KeyMsg)
    if !ok {
        return m, nil
    }
    switch key.String() {
    case "up", "k", "ctrl+p":
        if len(m.selectableRows) > 0 {
            m.cursor = (m.cursor - 1 + len(m.selectableRows)) % len(m.selectableRows)
        }
    case "down", "j", "ctrl+n":
        if len(m.selectableRows) > 0 {
            m.cursor = (m.cursor + 1) % len(m.selectableRows)
        }
    case "enter":
        return m.handleBrowseEnter()
    case "d":
        return m.handleBrowseDelete()
    case "esc":
        return m, func() tea.Msg { return CloseDialogMsg{} }
    }
    return m, nil
}

func (m ProvidersModel) handleBrowseEnter() (ProvidersModel, tea.Cmd) {
    if len(m.selectableRows) == 0 {
        return m, nil
    }
    rowIdx := m.selectableRows[m.cursor]
    row := m.rows[rowIdx]
    providerID := row.label // for active row, label is "Provider"; for others, label is the name
    if row.isActive {
        // Pressing Enter on active provider: switch to it (no-op but emit msg for consistency)
        return m, func() tea.Msg { return SwitchProviderMsg{ProviderID: m.activeProvider} }
    }
    // Determine the provider name from the row.
    name := providerIDForRow(row)
    if row.hasAuth || row.isOAuth {
        // Already configured — switch to it.
        _ = providerID
        return m, func() tea.Msg { return SwitchProviderMsg{ProviderID: name} }
    }
    // Not configured — enter configure mode.
    m.mode = providerModeConfigure
    m.configTarget = name
    m.configAPIKey = ""
    m.configAPIBase = defaultAPIBase(name)
    m.configFocused = 0
    m.configError = ""
    m.configWorking = false
    return m, nil
}

func (m ProvidersModel) handleBrowseDelete() (ProvidersModel, tea.Cmd) {
    if len(m.selectableRows) == 0 {
        return m, nil
    }
    rowIdx := m.selectableRows[m.cursor]
    row := m.rows[rowIdx]
    if row.isActive {
        // Cannot remove the active provider.
        return m, nil
    }
    name := providerIDForRow(row)
    if !row.hasAuth && !row.isOAuth {
        // Not configured — nothing to remove.
        return m, nil
    }
    m.mode = providerModeConfirming
    m.confirmTarget = name
    return m, nil
}

func (m ProvidersModel) updateConfigure(msg tea.Msg) (ProvidersModel, tea.Cmd) {
    key, ok := msg.(tea.KeyMsg)
    if !ok {
        return m, nil
    }
    switch key.String() {
    case "esc":
        m.mode = providerModeBrowse
        m.configTarget = ""
        m.configAPIKey = ""
        m.configAPIBase = ""
        m.configError = ""
        m.configWorking = false
    case "tab", "shift+tab":
        // Only toggle for providers that have an API base field (openai).
        if m.configTarget == "openai" {
            m.configFocused = 1 - m.configFocused
        }
    case "backspace":
        if m.configFocused == 0 && len(m.configAPIKey) > 0 {
            runes := []rune(m.configAPIKey)
            m.configAPIKey = string(runes[:len(runes)-1])
        } else if m.configFocused == 1 && len(m.configAPIBase) > 0 {
            runes := []rune(m.configAPIBase)
            m.configAPIBase = string(runes[:len(runes)-1])
        }
        m.configError = ""
    case "enter":
        if m.configWorking {
            return m, nil
        }
        if strings.TrimSpace(m.configAPIKey) == "" {
            m.configError = "API key is required"
            return m, nil
        }
        m.configWorking = true
        providerID := m.configTarget
        apiKey := strings.TrimSpace(m.configAPIKey)
        apiBase := strings.TrimSpace(m.configAPIBase)
        return m, func() tea.Msg {
            return ConfigureProviderMsg{
                ProviderID: providerID,
                APIKey:     apiKey,
                APIBase:    apiBase,
            }
        }
    default:
        // Append printable rune to the focused field.
        if r := []rune(key.String()); len(r) == 1 && r[0] >= 32 {
            if m.configFocused == 0 {
                m.configAPIKey += string(r)
            } else {
                m.configAPIBase += string(r)
            }
            m.configError = ""
        }
    }
    return m, nil
}

func (m ProvidersModel) updateConfirm(msg tea.Msg) (ProvidersModel, tea.Cmd) {
    key, ok := msg.(tea.KeyMsg)
    if !ok {
        return m, nil
    }
    switch key.String() {
    case "y", "enter":
        target := m.confirmTarget
        m.mode = providerModeBrowse
        m.confirmTarget = ""
        return m, func() tea.Msg { return RemoveProviderMsg{ProviderID: target} }
    case "n", "esc":
        m.mode = providerModeBrowse
        m.confirmTarget = ""
    }
    return m, nil
}

// WithConfigureResult updates the model after a provider.configure RPC completes.
// Called by tui.go with the error from client.ProviderConfigure.
func (m ProvidersModel) WithConfigureResult(err error) ProvidersModel {
    m.configWorking = false
    if err != nil {
        m.configError = err.Error()
        return m
    }
    m.mode = providerModeBrowse
    m.configTarget = ""
    m.configAPIKey = ""
    m.configAPIBase = ""
    m.configError = ""
    return m
}
```

**Helper functions** to add:

```go
// providerIDForRow extracts the provider ID from a providerRenderRow.
// For the active provider row, label is "Provider" and value is the name.
// For other rows, label IS the provider name.
func providerIDForRow(row providerRenderRow) string {
    if row.isActive {
        return row.value // active row: label="Provider", value=providerName
    }
    return row.label // other rows: label=providerName, value=type
}

// defaultAPIBase returns the default API base URL for providers that need one.
func defaultAPIBase(providerID string) string {
    switch providerID {
    case "openai":
        return "https://api.openai.com/v1"
    default:
        return ""
    }
}

// maskAPIKey returns key with all but the last 4 chars replaced with '*'.
// Keys shorter than 8 chars are fully masked.
func maskAPIKey(key string) string {
    runes := []rune(key)
    if len(runes) <= 8 {
        return strings.Repeat("*", len(runes))
    }
    return strings.Repeat("*", len(runes)-4) + string(runes[len(runes)-4:])
}
```

### 5.8 Rewrite `View` to render all three modes

Replace `View()` entirely:

```go
func (m ProvidersModel) View() string {
    t := theme.Current()
    if t == nil {
        return "providers"
    }
    var content string
    switch m.mode {
    case providerModeBrowse:
        content = m.viewBrowse(t)
    case providerModeConfigure:
        content = m.viewConfigure(t)
    case providerModeConfirming:
        content = m.viewConfirm(t)
    }
    return lipgloss.NewStyle().
        Background(t.Panel).
        Border(lipgloss.RoundedBorder()).
        BorderForeground(t.BorderFocus).
        Padding(1, 2).
        Width(dialogWidth(m.termWidth, 72)).
        Render(content)
}

func (m ProvidersModel) viewBrowse(t *theme.Theme) string {
    lines := []string{
        lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render("Providers"),
        "",
    }
    if len(m.rows) == 0 {
        lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true).Render("No providers found"))
    } else {
        // Build a set of currently selected row indexes.
        selectedRowIdx := -1
        if len(m.selectableRows) > 0 && m.cursor < len(m.selectableRows) {
            selectedRowIdx = m.selectableRows[m.cursor]
        }
        for i, row := range m.rows {
            isCursor := i == selectedRowIdx
            lines = append(lines, m.renderRowBrowse(row, isCursor, t)...)
        }
    }
    lines = append(lines, "",
        lipgloss.NewStyle().Foreground(t.TextMuted).Render("↑/↓ navigate • Enter select/configure • d remove • Esc close"),
    )
    return strings.Join(lines, "\n")
}

func (m ProvidersModel) renderRowBrowse(row providerRenderRow, isCursor bool, t *theme.Theme) []string {
    switch row.kind {
    case "section":
        style := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
        return []string{"", style.Render(row.section)}
    case "provider":
        prefix := "  "
        if isCursor {
            prefix = "> "
        }
        name := row.label
        if row.isActive {
            name = row.value + " (active)"
        }
        var style lipgloss.Style
        switch {
        case row.isActive:
            style = lipgloss.NewStyle().Foreground(t.Success).Bold(true)
        case row.isPartial || (!row.hasAuth && !row.isOAuth):
            style = lipgloss.NewStyle().Foreground(t.TextMuted)
        default:
            style = lipgloss.NewStyle().Foreground(t.Text)
        }
        hint := ""
        if isCursor && !row.isActive && !row.hasAuth && !row.isOAuth {
            hint = "  " + lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true).Render("Press Enter to configure")
        }
        return []string{prefix + style.Render(name) + hint}
    default:
        labelStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
        valueStyle := lipgloss.NewStyle().Foreground(t.Text)
        return []string{"  " + labelStyle.Render(row.label+":") + " " + valueStyle.Render(row.value)}
    }
}

func (m ProvidersModel) viewConfigure(t *theme.Theme) string {
    meta := providerMeta[m.configTarget]
    displayName := meta.DisplayName
    if displayName == "" {
        displayName = m.configTarget
    }
    lines := []string{
        lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render("Configure " + displayName),
        "",
    }
    if meta.Description != "" {
        lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Render(meta.Description), "")
    }

    // API Key field.
    keyLabel := lipgloss.NewStyle().Foreground(t.Text).Render("API Key: ")
    keyValue := maskAPIKey(m.configAPIKey)
    if m.configFocused == 0 {
        keyValue += "█" // cursor indicator
    }
    keyStyle := lipgloss.NewStyle().Foreground(t.Primary)
    if m.configFocused == 0 {
        keyStyle = keyStyle.Bold(true)
    }
    lines = append(lines, keyLabel+keyStyle.Render(keyValue))

    // API Base field (openai only).
    if m.configTarget == "openai" {
        baseLabel := lipgloss.NewStyle().Foreground(t.Text).Render("API Base: ")
        baseValue := m.configAPIBase
        if m.configFocused == 1 {
            baseValue += "█"
        }
        baseStyle := lipgloss.NewStyle().Foreground(t.Primary)
        if m.configFocused == 1 {
            baseStyle = baseStyle.Bold(true)
        }
        lines = append(lines, baseLabel+baseStyle.Render(baseValue))
    }

    if m.configWorking {
        lines = append(lines, "", lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true).Render("Configuring..."))
    } else if m.configError != "" {
        lines = append(lines, "", lipgloss.NewStyle().Foreground(t.Error).Render("✗ "+m.configError))
    }

    lines = append(lines, "",
        lipgloss.NewStyle().Foreground(t.TextMuted).Render("Enter save • Esc cancel"),
    )
    return strings.Join(lines, "\n")
}

func (m ProvidersModel) viewConfirm(t *theme.Theme) string {
    meta := providerMeta[m.confirmTarget]
    displayName := meta.DisplayName
    if displayName == "" {
        displayName = m.confirmTarget
    }
    return strings.Join([]string{
        lipgloss.NewStyle().Foreground(t.Warning).Bold(true).Render("Remove " + displayName + "?"),
        "",
        lipgloss.NewStyle().Foreground(t.Text).Render("All models for this provider will be hidden from the picker."),
        "",
        lipgloss.NewStyle().Foreground(t.TextMuted).Render("y/Enter confirm • n/Esc cancel"),
    }, "\n")
}
```

### Checkpoint commit

```
feat(tui): make providers panel interactive — browse, configure, remove
```

---

## 6. Task 4 — TUI Wiring

### Files changed

| Action | Path |
|--------|------|
| **Modify** | `internal/tui/tui.go` |

This task has no new files — all wiring is in `tui.go`. Read the current file carefully before
editing; match existing patterns for dialog wrappers, message types, and `Update` switch cases.

### 6.1 Add new message types

Add to `tui.go` alongside the existing `ModelChosenMsg`, `ModelsLoadedMsg`, etc.:

```go
// ModelsUpdatedMsg is dispatched when the "models.updated" gateway event arrives.
type ModelsUpdatedMsg struct {
    Models  []client.ModelInfo
    Current string
}

// ProviderConfigureResultMsg is dispatched after client.ProviderConfigure returns.
type ProviderConfigureResultMsg struct {
    ProviderID string
    Models     []client.ModelInfo
    Current    string
    Err        error
}

// ProviderRemoveResultMsg is dispatched after client.ProviderRemove returns.
type ProviderRemoveResultMsg struct {
    ProviderID string
    Models     []client.ModelInfo
    Current    string
    Err        error
}
```

### 6.2 Extend `gatewayClient` interface

Find the `gatewayClient` interface in `tui.go` (or wherever the client interface is defined)
and add two methods:

```go
ProviderConfigure(providerID, apiKey, apiBase string) (client.ProviderConfigurePayload, error)
ProviderRemove(providerID string) (client.ProviderRemovePayload, error)
```

`*client.Client` already satisfies this after Task 2 changes.

### 6.3 Store `lastStatus` on `tuiModel`

Check whether `tuiModel` already has a `lastStatus client.StatusPayload` field. If not, add it
and populate it in the `StatusLoadedMsg` (or equivalent) handler.

Also confirm that `m.providerCfg` (or however the `*config.Config` is stored in `tuiModel`)
is populated and accessible. It is passed to `NewProvidersFromData` as the last argument.

### 6.4 Handle `"models.updated"` broadcast event

In the `EventMsg` switch in `Update` (where other event names like `"chat.progress"`,
`"agent.spawned"`, etc. are handled), add:

```go
case "models.updated":
    var p client.ModelsUpdatedPayload
    if err := json.Unmarshal(msg.Event.Payload, &p); err != nil {
        break
    }
    mapped = ModelsUpdatedMsg{Models: p.Models, Current: p.Current}
```

`"models.updated"` arrives as an event push (not a response), so it comes through the event
loop, not `sendRequest`.

### 6.5 Handle `ModelsUpdatedMsg`

In the main `Update` switch, add:

```go
case ModelsUpdatedMsg:
    m.models = msg.Models
    if msg.Current != "" && m.app.Model != msg.Current {
        m.app.Model = msg.Current
        // Update footer and sidebar if they have SetModel methods.
        // Follow whatever pattern ModelSetMsg uses.
    }
    // If model picker dialog is open, rebuild it with the new model list.
    if d, ok := m.dialog.(modelsDialog); ok {
        _ = d
        current := m.app.Model
        m.dialog = modelsDialog{dialogcmp.NewModels(m.providerCfg, msg.Models, current)}
        m.dialog = m.dialog.SetTerminalWidth(m.width)
    }
    // If providers dialog is open, rebuild it with the new model list.
    if d, ok := m.dialog.(providersDialog); ok {
        _ = d
        m.dialog = providersDialog{
            dialogcmp.NewProvidersFromData(msg.Models, m.app.Model, m.lastStatus, m.providerCfg),
        }
        m.dialog = m.dialog.SetTerminalWidth(m.width)
    }
    return m, nil
```

### 6.6 Handle `dialogcmp.ConfigureProviderMsg`

In the `Update` switch, add:

```go
case dialogcmp.ConfigureProviderMsg:
    providerID := msg.ProviderID
    apiKey := msg.APIKey
    apiBase := msg.APIBase
    return m, func() tea.Msg {
        payload, err := m.client.ProviderConfigure(providerID, apiKey, apiBase)
        if err != nil {
            return ProviderConfigureResultMsg{ProviderID: providerID, Err: err}
        }
        return ProviderConfigureResultMsg{
            ProviderID: providerID,
            Models:     payload.Models,
            Current:    payload.Current,
        }
    }
```

### 6.7 Handle `ProviderConfigureResultMsg`

```go
case ProviderConfigureResultMsg:
    // Update the providers dialog with the result (success or error).
    if d, ok := m.dialog.(providersDialog); ok {
        next := d.ProvidersModel.WithConfigureResult(msg.Err)
        m.dialog = providersDialog{next}
        m.dialog = m.dialog.SetTerminalWidth(m.width)
    }
    if msg.Err == nil && len(msg.Models) > 0 {
        m.models = msg.Models
        if msg.Current != "" && m.app.Model != msg.Current {
            m.app.Model = msg.Current
            // follow ModelSetMsg pattern for footer/sidebar update
        }
    }
    return m, nil
```

### 6.8 Handle `dialogcmp.RemoveProviderMsg`

```go
case dialogcmp.RemoveProviderMsg:
    providerID := msg.ProviderID
    return m, func() tea.Msg {
        payload, err := m.client.ProviderRemove(providerID)
        if err != nil {
            return ProviderRemoveResultMsg{ProviderID: providerID, Err: err}
        }
        return ProviderRemoveResultMsg{
            ProviderID: providerID,
            Models:     payload.Models,
            Current:    payload.Current,
        }
    }
```

### 6.9 Handle `ProviderRemoveResultMsg`

```go
case ProviderRemoveResultMsg:
    if msg.Err == nil && len(msg.Models) > 0 {
        m.models = msg.Models
        // Rebuild providers dialog with updated list.
        if _, ok := m.dialog.(providersDialog); ok {
            m.dialog = providersDialog{
                dialogcmp.NewProvidersFromData(msg.Models, m.app.Model, m.lastStatus, m.providerCfg),
            }
            m.dialog = m.dialog.SetTerminalWidth(m.width)
        }
        // If removed provider was the active one, clear the model.
        if msg.Current != "" && m.app.Model != msg.Current {
            m.app.Model = msg.Current
        }
    }
    return m, nil
```

### 6.10 Handle `dialogcmp.SwitchProviderMsg`

```go
case dialogcmp.SwitchProviderMsg:
    // Find the first selectable model for this provider.
    for _, model := range m.models {
        if model.Provider == msg.ProviderID && model.Selectable {
            m.dialog = nil
            modelID := model.ID
            return m, func() tea.Msg {
                current, err := m.client.ModelsSet(modelID)
                if err != nil {
                    return ChatErrorMsg{Message: err.Error()}
                }
                return ModelSetMsg{ID: current}
            }
        }
    }
    // No selectable model — show inline flash or status message.
    // Use whatever flash/status mechanism is already in tui.go.
    return m, nil
```

### 6.11 Verify `providersDialog.Update` delegation

Find `providersDialog` in `tui.go`. Its `Update` method must correctly return `(Dialog, tea.Cmd)`:

```go
func (d providersDialog) Update(msg tea.Msg) (Dialog, tea.Cmd) {
    next, cmd := d.ProvidersModel.Update(msg)
    return providersDialog{next}, cmd
}
```

If it already does this, no change needed. If it uses an older pattern, fix it.

### Checkpoint commit

```
feat(tui): wire provider configure/remove/switch and hot-reload on models.updated
```

---

## 7. Security Considerations

### API key storage

- `AtomicWriteConfig` writes with `0o600` (owner read/write only). Existing configs written
  at `0o644` are not retroactively fixed — document this in the commit message.
- `ProviderConfig.APIKey` must not appear in any `log.Printf`, `slog`, or debug output.
  The gateway handlers log `"provider.configure"` + provider ID, never the key.

### API key in TUI

- `maskAPIKey(key string) string` shows only the last 4 runes; fully masks keys shorter than 8.
- `configAPIKey` lives in memory only while the form is open. After `ConfigureProviderMsg` is
  emitted, the struct is cleared in `WithConfigureResult` on success.
- TUI state is not persisted to disk; only `config.json` stores the key.

### No new attack surface

- `provider.configure` binds to `127.0.0.1` only (existing gateway behaviour).
- No authentication is added between TUI and daemon — consistent with existing model.

---

## 8. Final Integration Gate

Run after all 4 tasks are complete:

```sh
# Unit tests for changed packages
go test ./pkg/config/... ./pkg/provider/... ./pkg/gateway/... \
        ./internal/components/dialog/... ./internal/tui/... \
        ./cmd/smolbot/... \
        -run 'Test.*Provider|Test.*Catalogue|Test.*Atomic|Test.*Invalidate|Test.*Writeback|Test.*Model'

# Full build
go build ./cmd/smolbot ./cmd/smolbot-tui

# Manual smoke test
# 1. Start daemon: smolbot daemon
# 2. Open TUI: smolbot-tui
# 3. Press F1 → navigate to Providers
# 4. Browse shows all known providers (anthropic, openai, gemini, groq, deepseek, minimax, ollama)
# 5. Press Enter on an unconfigured provider → configure form appears
# 6. Type an API key → Enter → daemon confirms, model picker immediately shows new models
# 7. Press d on a configured (non-active) provider → confirmation → y → provider disappears from picker
```

### Definition of done

- [ ] `go build ./...` passes with no errors
- [ ] All `TestCatalogue*`, `TestGetAvailableModels_*Catalogue*` tests pass
- [ ] `TestAtomicWriteConfig_*` passes including 0o600 permission check
- [ ] `TestRegistry_Invalidate*`, `TestRegistry_UpdateProviderConfig*`, `TestRegistry_RemoveProviderConfig*` pass
- [ ] Typing an API key in the TUI and pressing Enter causes `models.list` to immediately return the provider's catalogue models
- [ ] The active session is not interrupted when a second provider is configured
- [ ] API keys do not appear in daemon logs (`journalctl -u smolbot | grep -i apikey` is empty)
