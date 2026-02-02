---
name: foundry-shutdown
description: |
  Shutdown all forge sessions and foundry workers. Use when: (1) User says "shutdown all agents",
  (2) "stop all workers", (3) "kill all sessions", (4) "foundry shutdown", (5) "stop everything",
  (6) "clean up agents", (7) User wants to stop all running Claude sessions managed by forge/foundry.
---

# Foundry Shutdown

Stop all running forge sessions and foundry workers.

## CLI Command

The preferred method is to use the foundry CLI:

```bash
foundry shutdown              # Graceful shutdown of all agents
foundry shutdown --force      # Force kill all sessions
foundry stop-all              # Alias
foundry killall               # Alias
```

## Manual Shutdown

If the CLI isn't available, run these commands in sequence:

```bash
# 1. Cancel all forge sessions (non-interactive)
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

After shutdown, verify all agents are stopped:

```bash
echo "=== Status Check ==="
forge list 2>/dev/null | grep -v "No active" || echo "Forge: all stopped"
foundry worker list --active 2>/dev/null | grep -v "No active" || echo "Workers: all stopped"
tmux list-sessions 2>/dev/null | grep -E '(forge|foundry|worker|you-are)' || echo "Tmux: clean"
```

## What Gets Stopped

| Component | Description |
|-----------|-------------|
| Forge sessions | Claude sessions running in tmux (started by `forge start`) |
| Foundry workers | Named workers (alpha, bravo, etc.) from `foundry worker` |
| Leader sessions | Planner, reviewer, merge, deploy agents |
| Sync sessions | GitHub/Notion board sync sessions |
| Orphaned tmux | Any remaining forge-related tmux sessions (`forge-`, `mforge-`, `mf-`) |

## Alternative: Supervisor Cleanup

For cleanup without full shutdown, use the supervisor's orphan cleanup:

```bash
foundry supervisor --cleanup-orphans --dry-run  # Preview what would be cleaned
foundry supervisor --cleanup-orphans            # Clean up and continue
```

This cleans orphaned sessions but keeps the supervisor running.

## Notes

- Sessions are cancelled gracefully (workers can save state)
- Use `--force` flags if graceful shutdown fails
- Supervisor will restart workers if still running - stop it first with Ctrl+C
