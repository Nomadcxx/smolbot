---
---
name: tmux
description: "Manage tmux terminal sessions for persistent and multi-window terminal workflows. Use when creating detached terminal sessions, managing multiple terminal panes, or setting up persistent development environments."
---
---

Use this skill when working with tmux terminal sessions.

## When to Use

- Create a persistent tmux session for long-running tasks
- Attach to an existing tmux session
- Create windows and panes for multi-terminal workflows
- List available tmux sessions
- Send commands to tmux sessions
- Kill or manage tmux sessions

## Common Commands

### Session Management

```bash
tmux new-session -d -s <session-name>    # Create detached session
tmux attach -t <session-name>            # Attach to session
tmux list-sessions                        # List all sessions
tmux kill-session -t <session-name>       # Kill a session
```

### Window Management

```bash
tmux new-window -t <session>              # Create new window
tmux select-window -t <session>:<n>       # Switch to window n
tmux list-windows -t <session>             # List windows in session
```

### Pane Management

```bash
tmux split-window -h                      # Split horizontally
tmux split-window -v                      # Split vertically
tmux select-pane -t <session>:<window>.<pane>  # Select pane
```

### Sending Commands

```bash
tmux send-keys -t <session>:<window>.<pane> "command" Enter
```

## Use Cases

1. **Persistent dev servers** - Keep services running after disconnect
2. **Multi-terminal workflows** - Split panes for editor, tests, logs
3. **Remote sessions** - Reconnect to ongoing work via SSH
4. **Background tasks** - Run long jobs without tying up terminal
