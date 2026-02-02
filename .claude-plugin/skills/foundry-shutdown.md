---
name: foundry:shutdown
description: |
  Stop all forge sessions and foundry workers. Use when: "shutdown all agents",
  "stop all workers", "kill all sessions", "stop everything", "clean up agents".
triggers:
  - "shutdown agents"
  - "stop all workers"
  - "kill all sessions"
  - "stop everything"
  - "clean up agents"
  - "foundry shutdown"
---

# Foundry Shutdown - Stop All Agents

Stop all running forge sessions, foundry workers, and related processes.

## CLI Command

```bash
foundry shutdown              # Graceful shutdown of all agents
foundry shutdown --force      # Force kill all sessions
foundry stop-all              # Alias
foundry killall               # Alias
```

## What Gets Stopped

| Component | Description |
|-----------|-------------|
| Foundry workers | Named workers (alpha, bravo, etc.) from `foundry worker` |
| Forge sessions | Claude sessions running in tmux (started by `forge start`) |
| Leader sessions | Planner, reviewer, merge, deploy agents |
| Sync sessions | GitHub/Notion board sync sessions |
| Orphaned tmux | Any remaining forge-related tmux sessions |

## Manual Shutdown

If the CLI isn't available:

```bash
# 1. Cancel all forge sessions
for session in $(forge list 2>/dev/null | tail -n +2 | awk '{print $1}'); do
  echo "y" | forge cancel "$session" 2>/dev/null && echo "Cancelled: $session"
done

# 2. Stop all foundry workers
foundry worker list --active 2>/dev/null | tail -n +2 | awk '{print $1}' | while read worker; do
  foundry worker stop "$worker" 2>/dev/null && echo "Stopped worker: $worker"
done

# 3. Kill any remaining tmux sessions
tmux list-sessions 2>/dev/null | grep -E '^(forge-|you-are-|github-sync|foundry-|worker-)' | cut -d: -f1 | while read s; do
  tmux kill-session -t "$s" 2>/dev/null && echo "Killed tmux: $s"
done
```

## Verification

```bash
echo "=== Status Check ==="
forge list 2>/dev/null | grep -v "No active" || echo "Forge: all stopped"
foundry worker list --active 2>/dev/null | grep -v "No active" || echo "Workers: all stopped"
tmux list-sessions 2>/dev/null | grep -E '(forge|foundry|worker|you-are)' || echo "Tmux: clean"
```

## Notes

- Sessions are cancelled gracefully (workers can save state)
- Use `--force` if graceful shutdown fails
- Stop the supervisor first (Ctrl+C) before running shutdown, or it will restart workers
