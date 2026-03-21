# nanobot-go Installer Design Document

**Date:** 2026-03-21  
**Status:** Design Complete - REVISED  
**Based on:** sysc-greet + jellywatch patterns
**Review:** Critical issues addressed (see Revision History)

---

## Executive Summary

This document specifies the TUI-based installer for nanobot-go, designed for user-local installation with systemd user service integration. The installer follows the proven patterns from sysc-greet and jellywatch while being adapted for nanobot-go's simpler requirements (no root, no *arr integrations, minimal configuration).

---

## Architecture

### Two-Stage Installation

```
Stage 1: Bootstrap (install.sh)
├── Download one-liner script
├── Check prerequisites (Go, git)
├── Clone repository to temp directory
├── Build TUI installer binary
└── Execute Stage 2

Stage 2: TUI Installer (cmd/installer/main.go)
├── Welcome screen with SMOLBOT ASCII art
├── Prerequisites validation
├── Configuration wizard
├── Installation tasks
├── Systemd user service setup
└── Completion summary
```

### Installation Flow

```
┌─────────────────────────────────────────────┐
│            SMOLBOT ASCII Art              │
│         Welcome to nanobot-go             │
└─────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────┐
│         Prerequisites Check                 │
│  ✓ Go 1.21+ detected                       │
│  ✓ Git available                           │
│  ✓ Ollama detected at localhost:11434    │
└─────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────┐
│          Configuration Wizard               │
│  ┌─────────────────────────────────────────┐ │
│  │ Select Default Model                  │ │
│  │ ○ llama2:7b                            │ │
│  │ ● mixtral:8x7b                         │ │
│  │ ○ codellama:7b                         │ │
│  │ ○ [Refresh List]                       │ │
│  └─────────────────────────────────────────┘ │
│  ┌─────────────────────────────────────────┐ │
│  │ Workspace Path: ~/.nanobot-go/          │ │
│  │ [Advanced Options ▼]                   │ │
│  │   Config path, Port, etc.              │ │
│  └─────────────────────────────────────────┘ │
└─────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────┐
│           Installation Progress             │
│  [OK] Clone repository                      │
│  [OK] Build nanobot binary                  │
│  [OK] Build nanobot-tui binary            │
│  [OK] Install to ~/.local/bin/            │
│  [OK] Create config structure             │
│  [OK] Setup systemd user service          │
│  [RUNNING] Start service                  │
└─────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────┐
│           Installation Complete!            │
│                                             │
│  System Commands:                           │
│    systemctl --user status nanobot-go     │
│    systemctl --user stop nanobot-go       │
│    nanobot --help                         │
│    nanobot-tui                            │
│                                             │
│  [Close] [Open TUI Now]                     │
└─────────────────────────────────────────────┘
```

---

## Technical Specification

### File Structure

```
nanobot-go/
├── install.sh                    # One-line bootstrap script
├── cmd/
│   ├── nanobot/                  # Main daemon + CLI
│   ├── nanobot-tui/             # Bubble Tea TUI client
│   └── installer/               # TUI installer
│       ├── main.go
│       ├── types.go             # InstallStep, TaskStatus, etc.
│       ├── tasks.go             # Installation task functions
│       ├── views.go             # Bubble Tea views
│       ├── update.go            # Update mode logic
│       └── theme.go             # Colors & styles (match nanobot-tui)
└── systemd/
    └── nanobot-go.service       # User service template
```

### Core Types (from jellywatch pattern)

```go
// Install steps in the wizard flow
type installStep int

const (
    stepWelcome installStep = iota
    stepPrerequisites
    stepConfiguration
    stepInstalling
    stepComplete
)

// Task execution status
type taskStatus int

const (
    statusPending taskStatus = iota
    statusRunning
    statusComplete
    statusFailed
    statusSkipped
)

// Install task with error handling
type installTask struct {
    name         string
    description  string
    execute      func(*model) error
    optional     bool
    status       taskStatus
    errorDetails *errorInfo
}

type errorInfo struct {
    message string
    command string
    logFile string
}
```

### Configuration Model

```go
type model struct {
    // Wizard state
    step             installStep
    tasks            []installTask
    currentTaskIndex int
    width, height    int
    
    // Visual elements
    spinner     spinner.Model
    inputs      []textinput.Model
    focusedInput int
    
    // Configuration
    selectedModel      string
    ollamaURL          string
    workspacePath      string
    configPath         string
    port               int
    
    // Ollama detection
    ollamaDetected     bool
    ollamaModels       []string
    ollamaModelIndex   int
    
    // Service options
    enableService      bool
    startNow           bool
    
    // Upgrade detection
    existingInstall    bool
    existingVersion    string
    
    // Error tracking
    errors             []string
    debugMode          bool
    logFile           *os.File
}
```

---

## Step-by-Step Specification

### Step 1: Welcome

**Purpose:** Branding and introduction

**Content:**
- SMOLBOT ASCII art header
- Version information
- Brief description: "AI-powered coding assistant for your terminal"
- [Continue] [Exit]

**Design Notes:**
- Match nanobot-tui theme colors
- Use typewriter effect for tagline (from sysc-greet)

### Step 2: Prerequisites

**Purpose:** Validate system requirements

**Checks:**
1. Go 1.21+ installed (`go version`)
2. Git available (`git --version`)
3. Ollama detected (`curl http://localhost:11434/api/tags`)

**Visual:**
```
Prerequisites Check
────────────────────

[OK]  Go 1.21.5 detected
[OK]  Git 2.43.0 detected
[OK]  Ollama running at localhost:11434
      Found 12 models

All prerequisites met! Continue to configuration...
```

**Error Handling:**
- If Go missing: Show install instructions + exit
- If Git missing: Show install instructions + exit
- If Ollama not detected: Warning + continue (user can configure later)

### Step 3: Configuration Wizard

**Purpose:** Collect user preferences

**3a. Model Selection (if Ollama detected)**

```go
// Auto-fetch from Ollama API
func fetchOllamaModels(url string) ([]string, error) {
    resp, err := http.Get(url + "/api/tags")
    // Parse JSON response for model names
}
```

**UI:**
```
Select Default Model
────────────────────

Ollama detected at localhost:11434

  ○ llama2:7b                              
  ● mixtral:8x7b          ← Cursor here     
  ○ codellama:7b                           
  ○ mistral:7b                             
  ○ neural-chat:7b                         
  ○ starcoder:7b                           
  
  [← Prev]  [Next →]  [Refresh]
```

**3b. Workspace Path**

```
Configure Workspace
────────────────────

Workspace path (where sessions/cache stored):
┌────────────────────────────────────────┐
│ ~/.nanobot-go                          │
└────────────────────────────────────────┘

Config file location:
┌────────────────────────────────────────┐
│ ~/.nanobot-go/config.json            │
└────────────────────────────────────────┘

[Advanced Options ▼]
  Gateway port: 18791
  Log level: info

  [← Prev]  [Install →]
```

### Step 4: Installation

**Purpose:** Execute installation tasks with visual feedback

**Task List:**
```go
func (m model) startInstallation() (tea.Model, tea.Cmd) {
    m.step = stepInstalling
    
    if m.existingInstall {
        // Upgrade mode
        m.tasks = []installTask{
            {name: "Backup config", description: "Backing up existing config", execute: backupConfig},
            {name: "Stop service", description: "Stopping nanobot-go", execute: stopService},
            {name: "Build nanobot", description: "Building daemon binary", execute: buildNanobot},
            {name: "Build nanobot-tui", description: "Building TUI binary", execute: buildNanobotTUI},
            {name: "Install binaries", description: "Installing to ~/.local/bin", execute: installBinaries},
            {name: "Migrate config", description: "Migrating configuration", execute: migrateConfig},
            {name: "Start service", description: "Starting nanobot-go", execute: startService, optional: true},
        }
    } else {
        // Fresh install
        m.tasks = []installTask{
            {name: "Clone repository", description: "Cloning nanobot-go", execute: cloneRepository},
            {name: "Build nanobot", description: "Building daemon binary", execute: buildNanobot},
            {name: "Build nanobot-tui", description: "Building TUI binary", execute: buildNanobotTUI},
            {name: "Install binaries", description: "Installing to ~/.local/bin", execute: installBinaries},
            {name: "Create workspace", description: "Creating ~/.nanobot-go", execute: createWorkspace},
            {name: "Write config", description: "Writing config.json", execute: writeConfig},
            {name: "Setup systemd", description: "Installing user service", execute: setupSystemd},
            {name: "Start service", description: "Starting nanobot-go", execute: startService, optional: true},
        }
    }
    
    m.currentTaskIndex = 0
    m.tasks[0].status = statusRunning
    return m, tea.Batch(m.spinner.Tick, executeTaskCmd(0, &m))
}
```

**Visual:**
```
Installing nanobot-go v1.0.0
────────────────────────────────

[OK]    Clone repository                        2s
[OK]    Build nanobot                          15s
[RUN]   Build nanobot-tui                      ⠋  (spinner)
[PEND]  Install binaries
[PEND]  Create workspace
[PEND]  Write config
[PEND]  Setup systemd
[PEND]  Start service

Details:
  Building nanobot-tui binary...
  go build -o nanobot-tui ./cmd/nanobot-tui

[View Log] [Cancel]
```

### Step 5: Complete

**Purpose:** Show success message and next steps

**Fresh Install:**
```
Installation Complete! ✓
────────────────────────────────

nanobot-go v1.0.0 is installed and running!

Installation Summary:
  Binaries: ~/.local/bin/nanobot, ~/.local/bin/nanobot-tui
  Config: ~/.nanobot-go/config.json
  Service: systemd user service enabled

Quick Start:
  nanobot-tui              # Launch TUI
  nanobot chat "hello"     # Quick CLI chat
  
Systemd Commands:
  systemctl --user status nanobot-go
  systemctl --user stop nanobot-go
  systemctl --user restart nanobot-go

[Close] [Open TUI Now]
```

**Upgrade:**
```
Upgrade Complete! ✓
────────────────────────────────

Successfully upgraded nanobot-go
  From: v0.9.5
  To:   v1.0.0

Backup created: ~/.nanobot-go/config.json.backup.20260321

[Close]
```

---

## Revision History

**v1.1 (Post-Review)** - Addressed critical issues:
- Fixed Bootstrap Script with trap for proper cleanup
- Added async task execution pattern (tea.Cmd)
- Added context cancellation support
- Added command logging infrastructure
- Added rich error types with CommandError
- Added daemonWasRunning detection for upgrades
- Added port availability validation
- Added systemd daemon-reload step

---

## Key Implementation Details

### Bootstrap Script (install.sh) - REVISED

```bash
#!/bin/bash
# nanobot-go one-line installer
# Usage: curl -fsSL https://raw.githubusercontent.com/Nomadcxx/nanobot-go/main/install.sh | bash

set -e

echo "SMOLBOT"
echo "nanobot-go installer"
echo ""

# Check prerequisites
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed"
    echo "Install Go 1.21+: https://golang.org/dl/"
    exit 1
fi

if ! command -v git &> /dev/null; then
    echo "Error: Git is not installed"
    exit 1
fi

# Create temp directory with cleanup trap
TEMP_DIR=$(mktemp -d)
trap "cd / && rm -rf '$TEMP_DIR'" EXIT

cd "$TEMP_DIR"

echo "Cloning nanobot-go..."
git clone --depth 1 https://github.com/Nomadcxx/nanobot-go.git
cd nanobot-go

echo "Building installer..."
go build -o install-smolbot ./cmd/installer/

echo "Starting installer..."
./install-smolbot

# Cleanup happens via trap on exit
```

### Task Functions (from jellywatch pattern)

```go
// Clone from GitHub
func cloneRepository(m *model) error {
    repoURL := "https://github.com/Nomadcxx/nanobot-go.git"
    tempDir := os.TempDir()
    
    cmd := exec.Command("git", "clone", "--depth", "1", repoURL, tempDir)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("git clone failed: %w\nOutput: %s", err, output)
    }
    
    m.projectDir = tempDir
    return nil
}

// Build both binaries
func buildBinaries(m *model) error {
    // Build nanobot (daemon + CLI)
    cmd := exec.Command("go", "build", "-o", "nanobot", "./cmd/nanobot")
    cmd.Dir = m.projectDir
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("build nanobot failed: %w\n%s", err, output)
    }
    
    // Build nanobot-tui
    cmd = exec.Command("go", "build", "-o", "nanobot-tui", "./cmd/nanobot-tui")
    cmd.Dir = m.projectDir
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("build nanobot-tui failed: %w\n%s", err, output)
    }
    
    return nil
}

// Install to ~/.local/bin
func installBinaries(m *model) error {
    binDir := filepath.Join(os.Getenv("HOME"), ".local", "bin")
    
    // Ensure directory exists
    if err := os.MkdirAll(binDir, 0755); err != nil {
        return fmt.Errorf("create bin dir: %w", err)
    }
    
    // Copy binaries
    for _, binary := range []string{"nanobot", "nanobot-tui"} {
        src := filepath.Join(m.projectDir, binary)
        dst := filepath.Join(binDir, binary)
        
        // Read source
        data, err := os.ReadFile(src)
        if err != nil {
            return fmt.Errorf("read %s: %w", binary, err)
        }
        
        // Write destination
        if err := os.WriteFile(dst, data, 0755); err != nil {
            return fmt.Errorf("install %s: %w", binary, err)
        }
    }
    
    return nil
}

// Create systemd user service
func setupSystemd(m *model) error {
    serviceDir := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user")
    if err := os.MkdirAll(serviceDir, 0755); err != nil {
        return fmt.Errorf("create systemd dir: %w", err)
    }
    
    serviceContent := fmt.Sprintf(`[Unit]
Description=nanobot-go - AI coding assistant
After=network.target

[Service]
Type=simple
ExecStart=%s/.local/bin/nanobot run --config %s --port %d
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`, os.Getenv("HOME"), m.configPath, m.port)
    
    servicePath := filepath.Join(serviceDir, "nanobot-go.service")
    if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
        return fmt.Errorf("write service file: %w", err)
    }
    
    // Reload systemd
    cmd := exec.Command("systemctl", "--user", "daemon-reload")
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("daemon-reload: %w", err)
    }
    
    // Enable service
    cmd = exec.Command("systemctl", "--user", "enable", "nanobot-go")
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("enable service: %w", err)
    }
    
    return nil
}

---

## Critical Implementation Patterns (Post-Review Additions)

### 1. Async Task Execution with tea.Cmd

Following jellywatch's pattern for proper Bubble Tea async execution:

```go
// Message types for task completion
type taskCompleteMsg struct {
    index   int
    success bool
    err     error
}

// Execute task asynchronously
func executeTaskCmd(index int, m *model) tea.Cmd {
    return func() tea.Msg {
        // Run task with context cancellation support
        err := m.tasks[index].execute(m)
        return taskCompleteMsg{index: index, success: err == nil, err: err}
    }
}

// Update function handles task completion
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case taskCompleteMsg:
        // Update task status based on result
        if msg.success {
            m.tasks[msg.index].status = statusComplete
            m.currentTaskIndex++
            // Start next task if available
            if m.currentTaskIndex < len(m.tasks) {
                m.tasks[m.currentTaskIndex].status = statusRunning
                return m, tea.Batch(m.spinner.Tick, executeTaskCmd(m.currentTaskIndex, &m))
            }
        } else {
            m.tasks[msg.index].status = statusFailed
            m.tasks[msg.index].errorDetails = &errorInfo{
                message: msg.err.Error(),
                command: getLastCommand(),
            }
            // For optional tasks, continue; for required, stop
            if m.tasks[msg.index].optional {
                m.currentTaskIndex++
                if m.currentTaskIndex < len(m.tasks) {
                    m.tasks[m.currentTaskIndex].status = statusRunning
                    return m, executeTaskCmd(m.currentTaskIndex, &m)
                }
            }
        }
    }
    return m, nil
}
```

### 2. Context Cancellation Support

```go
type model struct {
    // ... other fields ...
    ctx    context.Context
    cancel context.CancelFunc
}

func newModel() model {
    ctx, cancel := context.WithCancel(context.Background())
    return model{
        ctx:    ctx,
        cancel: cancel,
        // ... other fields ...
    }
}

// Check for cancellation in long-running tasks
func cloneRepository(m *model) error {
    select {
    case <-m.ctx.Done():
        return m.ctx.Err()
    default:
    }
    
    cmd := exec.CommandContext(m.ctx, "git", "clone", "--depth", "1", repoURL, tempDir)
    // ... rest of implementation
}
```

### 3. Command Logging Infrastructure

From sysc-greet's runCommand pattern:

```go
// runCommand executes a command with logging
type commandResult struct {
    output   string
    exitCode int
    duration time.Duration
    err      error
}

func runCommand(m *model, name string, args ...string) commandResult {
    start := time.Now()
    cmd := exec.Command(name, args...)
    
    // Log command
    if m.logFile != nil {
        fmt.Fprintf(m.logFile, "[CMD] %s %s\n", name, strings.Join(args, " "))
    }
    
    output, err := cmd.CombinedOutput()
    duration := time.Since(start)
    
    result := commandResult{
        output:   string(output),
        duration: duration,
        err:      err,
    }
    
    if cmd.ProcessState != nil {
        result.exitCode = cmd.ProcessState.ExitCode()
    }
    
    // Log result
    if m.logFile != nil {
        if err != nil {
            fmt.Fprintf(m.logFile, "[FAIL] Exit code %d, duration %v\n%s\n\n", 
                result.exitCode, duration, output)
        } else {
            fmt.Fprintf(m.logFile, "[OK] Duration %v\n\n", duration)
        }
    }
    
    return result
}
```

### 4. Rich Error Types (CommandError)

From jellywatch's error handling:

```go
// CommandError provides detailed error information
type CommandError struct {
    Command  string
    ExitCode int
    Output   string
    Duration time.Duration
    Err      error
}

func (e CommandError) Error() string {
    return fmt.Sprintf("command failed: %s (exit %d): %v", e.Command, e.ExitCode, e.Err)
}

func (e CommandError) DetailedError() string {
    var b strings.Builder
    b.WriteString(fmt.Sprintf("Command: %s\n", e.Command))
    b.WriteString(fmt.Sprintf("Exit Code: %d\n", e.ExitCode))
    b.WriteString(fmt.Sprintf("Duration: %v\n", e.Duration))
    if e.Output != "" {
        b.WriteString(fmt.Sprintf("Output:\n%s\n", e.Output))
    }
    return b.String()
}

// Usage in tasks
func buildBinaries(m *model) error {
    result := runCommand(m, "go", "build", "-o", "nanobot", "./cmd/nanobot")
    if result.err != nil {
        return CommandError{
            Command:  "go build",
            ExitCode: result.exitCode,
            Output:   result.output,
            Duration: result.duration,
            Err:      result.err,
        }
    }
    return nil
}
```

### 5. Port Availability Validation

```go
func validatePort(port int) error {
    // Try to listen on the port
    listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
    if err != nil {
        return fmt.Errorf("port %d is already in use: %w", port, err)
    }
    listener.Close()
    return nil
}

// In configuration step
func (m model) validateConfiguration() error {
    if err := validatePort(m.port); err != nil {
        return fmt.Errorf("port validation failed: %w", err)
    }
    return nil
}
```

### 6. Upgrade Detection with Service State

```go
func detectExistingInstall() (struct {
    exists         bool
    version        string
    daemonRunning  bool
    configExists   bool
}, error) {
    result := struct {
        exists         bool
        version        string
        daemonRunning  bool
        configExists   bool
    }{}
    
    // Check binary
    binPath := filepath.Join(os.Getenv("HOME"), ".local", "bin", "nanobot")
    if _, err := os.Stat(binPath); os.IsNotExist(err) {
        return result, nil
    }
    result.exists = true
    
    // Get version
    cmd := exec.Command(binPath, "--version")
    output, err := cmd.Output()
    if err == nil {
        result.version = strings.TrimSpace(string(output))
    }
    
    // Check if service is running
    cmd = exec.Command("systemctl", "--user", "is-active", "nanobot-go")
    if err := cmd.Run(); err == nil {
        result.daemonRunning = true
    }
    
    // Check config
    configPath := filepath.Join(os.Getenv("HOME"), ".nanobot-go", "config.json")
    if _, err := os.Stat(configPath); err == nil {
        result.configExists = true
    }
    
    return result, nil
}

// Store in model for upgrade workflow
type model struct {
    // ... other fields ...
    existingInstall   bool
    existingVersion   string
    daemonWasRunning  bool  // Remember to restart after upgrade
    configExists      bool
}
```

---

## Theme Integration

The installer should match nanobot-tui's existing theme system:

```go
// From nanobot-tui/internal/theme/theme.go
var colors = map[string]ThemeColors{
    "catppuccin": {BgBase: "#1e1e2e", Primary: "#cba6f7", Accent: "#f38ba8"},
    "dracula":    {BgBase: "#282a36", Primary: "#bd93f9", Accent: "#ff79c6"},
    "gruvbox":    {BgBase: "#282828", Primary: "#d79921", Accent: "#cc241d"},
    "nord":       {BgBase: "#2e3440", Primary: "#88c0d0", Accent: "#bf616a"},
    "tokyo_night": {BgBase: "#1a1b26", Primary: "#7aa2f7", Accent: "#f7768e"},
}
```

Use the same color palette for consistency.

---

## Upgrade Mode Detection

```go
func detectExistingInstall() (bool, string, error) {
    // Check for existing binary
    binPath := filepath.Join(os.Getenv("HOME"), ".local", "bin", "nanobot")
    if _, err := os.Stat(binPath); os.IsNotExist(err) {
        return false, "", nil
    }
    
    // Get version from binary
    cmd := exec.Command(binPath, "--version")
    output, err := cmd.Output()
    if err != nil {
        return true, "unknown", nil
    }
    
    version := strings.TrimSpace(string(output))
    return true, version, nil
}
```

---

## Error Handling

**Task Failure Strategy:**
1. Mark task as failed with red [FAIL]
2. Show error details in expandable section
3. Offer [Retry], [Skip], or [Abort]
4. Log full error to temp file

```go
type errorInfo struct {
    message string
    command string
    logFile string
}

func handleTaskError(m *model, taskIdx int, err error) {
    m.tasks[taskIdx].status = statusFailed
    m.tasks[taskIdx].errorDetails = &errorInfo{
        message: err.Error(),
        command: getLastCommand(),
        logFile: m.logFile.Name(),
    }
    
    // If task is optional, offer skip
    if m.tasks[taskIdx].optional {
        // Show [Skip] option
    }
}
```

---

## Testing Strategy

1. **Fresh Install Test:**
   - Clean VM with Go/git
   - Run install.sh
   - Verify binaries installed
   - Verify service started
   - Test nanobot-tui connects

2. **Upgrade Test:**
   - Install older version
   - Run installer again
   - Verify config migrated
   - Verify service restarted

3. **Error Recovery Test:**
   - Simulate network failure during clone
   - Verify retry works
   - Simulate build failure
   - Verify graceful exit

---

## Future Enhancements

1. **Package Manager Integration:**
   - AUR PKGBUILD
   - Homebrew formula
   - Nix derivation

2. **Configuration Templates:**
   - Preset configs (minimal, full, etc.)
   - Import from existing nanobot-python

3. **Plugin Discovery:**
   - Detect installed skills
   - Offer to install popular ones

---

## References

- **sysc-greet installer:** `/home/nomadx/Documents/sysc-greet-dev/cmd/installer/main.go`
- **jellywatch installer:** `/home/nomadx/Documents/jellywatch/cmd/installer/`
- **nanobot-tui themes:** `/home/nomadx/nanobot-tui/internal/theme/theme.go`
- **systemd user services:** https://wiki.archlinux.org/title/Systemd/User
