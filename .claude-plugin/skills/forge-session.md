---
name: forge-session
description: Manage autonomous Claude sessions in tmux. Use when starting, monitoring, or controlling Claude execution.
triggers:
  - "start forge"
  - "forge session"
  - "attach session"
  - "monitor forge"
  - "autonomous claude"
---

# Forge Sessions - Autonomous Claude Execution

Run Claude autonomously in tmux sessions with completion detection.

## Core Commands

### Start Session
```bash
forge start "implement feature X"
forge start -p "detailed prompt here"
forge start --max-iterations 20
forge start --id my-session
```

Options:
- `-p, --prompt`: Prompt text or file
- `--max-iterations`: Max agent iterations (default: 50)
- `--id`: Custom session ID
- `--no-skip`: Don't use --dangerously-skip-permissions

### Session Management
```bash
forge list               # List all sessions
forge status [id]        # Session status
forge attach [id]        # Attach to tmux
forge cancel [id]        # Cancel session
forge log [id]           # View session log
```

### Monitoring
```bash
forge monitor            # TUI dashboard for all sessions
```

## Session Lifecycle

1. **Starting**: Session created, Claude launching
2. **Running**: Claude executing autonomously
3. **Completed**: Promise detected, session finished
4. **Failed**: Error or max iterations reached
5. **Cancelled**: Manually stopped

## Completion Detection

Sessions detect completion via "promise" text in output:
- Default: `TASK_COMPLETE`
- Custom: Set via `--promise` flag

## Leader Sessions

Specialized leader agents for workflow stages:

```bash
forge planner            # Planning and task breakdown
forge reviewer           # Code review
forge merge              # PR and merge management
forge deploy             # Deployment coordination
```

## State Storage

Session state stored at:
- `~/.forge/sessions/{id}.state.md`

Contains:
- Session ID and status
- Start/end times
- Prompt and promise
- Output summary

## Example Workflow

```bash
# Start autonomous task
forge start "implement user authentication with JWT"

# Monitor progress
forge monitor

# Check status
forge status

# Attach if needed
forge attach

# View full log
forge log
```
