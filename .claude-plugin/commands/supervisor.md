---
name: foundry:supervisor
description: Run foundry supervisor for automated orchestration
aliases: ["supervisor", "sup", "sv"]
user_invocable: true
---

# /foundry:supervisor Command

Automated orchestration that coordinates workers and leaders through the kanban workflow.

## Usage

```
/supervisor              # Start with defaults (2min interval, 4 workers)
/supervisor fast         # 30s interval
/supervisor leaders      # Enable leader agents
/supervisor full         # Leaders + 30s interval
/supervisor parallel     # Max workers (8)
/supervisor cleanup      # Clean up orphaned sessions
```

## Behavior

1. **No arguments**: Start supervisor with defaults
   ```bash
   foundry supervisor
   ```

2. **"fast"**: Fast polling (30s interval)
   ```bash
   foundry supervisor --interval 30s
   ```

3. **"leaders"**: Enable all leader agents
   ```bash
   foundry supervisor --leaders
   ```

4. **"full"**: Full automation (leaders + fast)
   ```bash
   foundry supervisor --leaders --interval 30s
   ```

5. **"parallel"**: Maximum parallel workers (8)
   ```bash
   foundry supervisor --max-workers 8
   ```

6. **"cleanup"**: Clean up orphaned tmux sessions
   ```bash
   foundry supervisor --cleanup-orphans --dry-run
   ```

## Workflow

```
backlog → (groomer details) → todo → in_progress (worker) → review (reviewer) → done → merge → deploy
```

The supervisor:
1. Health checks - Detects stale workers (session gone or Claude exited)
2. Assigns up to `--max-workers` (default 4) in parallel
3. Creates isolated worktree per task (`.forge/worktrees/<id>/`, branch `task/<id>`)
4. Pokes active workers periodically
5. Moves finished tasks to review, cleans up worktrees
6. Launches groomer to research and detail backlog items (runs parallel with workers)
7. Launches leader agents (reviewer runs alongside workers)

## Setup Example

```bash
# 1. Create tasks
foundry kanban add "Task 1" -p high -s todo
foundry kanban add "Task 2" -p medium -s todo

# 2. Create workers
foundry worker create                    # alpha (worker)
foundry worker create --role groomer     # bravo (groomer - researches backlog)
foundry worker create --role reviewer    # charlie (reviewer)

# 3. Run supervisor
foundry supervisor --leaders --interval 30s
```

## Stop

Press `Ctrl+C` to gracefully stop. Active workers continue independently.
