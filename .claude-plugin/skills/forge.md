---
name: forge
description: Minimal Claude session runner. Use when starting, attaching, or managing Claude sessions in tmux.
triggers:
  - "start claude session"
  - "run claude"
  - "attach to session"
  - "session status"
  - "forge start"
---

# Forge - Claude Session Runner

Forge is a minimal tool for running Claude sessions in tmux.

## Commands

```bash
forge start "<prompt>"   # Start Claude session
forge attach [id]        # Attach to tmux session
forge status [id]        # Show session status
forge cancel [id]        # Cancel session
forge list               # List all sessions
forge log [id]           # View session output
```

## Examples

```bash
# Start autonomous task
forge start "implement user authentication with JWT"

# Check what's running
forge list
forge status

# Attach to watch
forge attach

# View output without attaching
forge log

# Stop a session
forge cancel
```

## Session Lifecycle

1. `forge start` → Creates tmux session, runs Claude
2. Claude executes until completion promise detected
3. Session state saved to `~/.forge/sessions/`

## Completion Detection

Sessions detect completion via promise text in output:
- Default: `TASK_COMPLETE`
- Custom: `forge start "task" --promise "DONE"`

## Notes

- Forge is intentionally minimal
- For orchestration (workers, boards, kanban), use **foundry**
- Sessions run with `--dangerously-skip-permissions` by default
