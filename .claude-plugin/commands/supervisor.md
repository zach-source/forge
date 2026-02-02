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
/supervisor              # Start with defaults (2min interval)
/supervisor fast         # 30s interval
/supervisor leaders      # Enable leader agents
/supervisor full         # Leaders + 30s interval
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

5. **"cleanup"**: Clean up orphaned tmux sessions
   ```bash
   foundry supervisor --cleanup-orphans --dry-run
   ```

## Workflow

```
todo → in_progress (worker) → review (reviewer) → done → merge → deploy
```

The supervisor:
1. Health checks - Detects stale workers (session gone or Claude exited)
2. Assigns idle workers to todo tasks
3. Pokes active workers periodically
4. Moves finished tasks to review
5. Launches leader agents when appropriate

## Setup Example

```bash
# 1. Create tasks
foundry kanban add "Task 1" -p high -s todo
foundry kanban add "Task 2" -p medium -s todo

# 2. Create workers
foundry worker create                    # alpha (worker)
foundry worker create --role reviewer    # bravo (reviewer)

# 3. Run supervisor
foundry supervisor --leaders --interval 30s
```

## Stop

Press `Ctrl+C` to gracefully stop. Active workers continue independently.
