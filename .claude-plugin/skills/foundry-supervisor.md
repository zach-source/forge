---
name: foundry:supervisor
description: Automated orchestration that coordinates workers and leaders through the kanban workflow. Use for autonomous development.
triggers:
  - "supervisor"
  - "orchestrate"
  - "automated workflow"
  - "autonomous development"
  - "run supervisor"
---

# Foundry Supervisor - Automated Orchestration

The supervisor coordinates workers and leaders through the kanban workflow.

## Commands

```bash
foundry supervisor                    # Default 2 minute interval
foundry supervisor --interval 30s     # Faster polling
foundry supervisor --leaders          # Enable all leader agents
foundry supervisor --max-workers 4    # Max concurrent workers (default 4)
foundry supervisor --no-auto-assign   # Only monitor, don't assign
foundry supervisor -d /path           # Custom working directory

# Task analysis and requeue
foundry supervisor --auto-requeue     # Auto-requeue stuck tasks
foundry supervisor --analyze-interval 10m  # Task analysis frequency
foundry supervisor --stuck 30m        # Stuck task threshold

# Health checks and cleanup
foundry supervisor --cleanup-orphans  # Clean up orphaned tmux sessions
foundry supervisor --dry-run          # Preview cleanup without taking action
```

## Workflow

```
todo → in_progress (worker) → review (reviewer) → done → merge → deploy
```

The supervisor:
1. **Health checks** - Detects stale workers (session gone or Claude exited)
2. **Assigns tasks** - Up to `--max-workers` (default 4) can run in parallel
3. **Monitors progress** - Pokes active workers periodically
4. **Handles completion** - Moves finished tasks to review
5. **Analyzes stuck tasks** - Requeues tasks stuck too long (--auto-requeue)
6. **Coordinates leaders** - Reviewer runs alongside workers; merge/deploy after completion

## Health Checks

Each cycle, the supervisor:
- Verifies all active workers have running tmux sessions
- Detects when Claude has exited (shell prompt visible)
- Resets stale workers to idle and clears leader state
- Restores leader state from registry on startup

### Orphan Cleanup

On startup with `--cleanup-orphans`:
- Finds tmux sessions not tracked in worker registry
- Covers legacy prefixes: `forge-`, `mforge-`, `mf-`
- Use `--dry-run` to preview without taking action

## Setup

```bash
# 1. Create kanban tasks
foundry kanban add "Task 1" -p high -s todo
foundry kanban add "Task 2" -p medium -s todo

# 2. Create workers
foundry worker create                    # alpha (worker)
foundry worker create                    # bravo (worker)
foundry worker create --role groomer     # charlie (groomer)
foundry worker create --role reviewer    # delta (reviewer)
foundry worker create --role merge       # echo (merge)
foundry worker create --role deploy      # foxtrot (deploy)

# 3. Run supervisor (up to 4 workers in parallel)
foundry supervisor --leaders --interval 30s --max-workers 4
```

## Leader Workflow

When `--leaders` is enabled:

| Condition | Action |
|-----------|--------|
| Items in backlog | Launch groomer (runs parallel, researches and details items) |
| Tasks in review | Launch reviewer (runs alongside workers) |
| Backlog needs prioritization | Launch planner |
| All tasks done | Launch merge leader |
| Merge complete | Launch deploy leader |

### Groomer Role

The groomer is a new leader that runs in parallel with workers and other leaders. It:
- Researches backlog items using codebase exploration
- Adds detailed descriptions and acceptance criteria
- Breaks down large items into smaller tasks
- Moves well-defined items from backlog → todo
- Prioritizes items based on dependencies and value

Create a groomer worker:
```bash
foundry worker create --role groomer
```

## Parallel Execution

The supervisor supports parallel worker execution (default: 4 concurrent workers):

```bash
foundry supervisor --max-workers 8    # Allow up to 8 parallel workers
```

| Setting | Default | Purpose |
|---------|---------|---------|
| `--max-workers` | 4 | Maximum concurrent development workers |

### Task Isolation via Worktrees

Each task gets its own git worktree and branch:
- Worktrees created in `.forge/worktrees/<task-id>/`
- Branches follow pattern `task/<task-id>`
- Workers work in isolation, no file conflicts
- Reviewer checks branches: `git log main..task/<id>`
- Merge leader combines branches to main

The reviewer can launch while workers are active, enabling parallel review.

## Poke System

The supervisor sends periodic nudges to active workers:

| Poke Count | Message |
|------------|---------|
| 1-2 | "How's it going?" |
| 3-4 | "Please wrap up soon." |
| 5-7 | "Time to finish - output your completion promise now." |
| 8+ | "URGENT: Please complete immediately." |

Use `--max-pokes 10` to limit pokes before escalating.

## Example Session

```bash
$ foundry supervisor --leaders --interval 30s

🎯 Supervisor starting
   Interval: 30s
   Work dir: /path/to/project
   Max workers: 4
   Auto-assign: true
   Leaders: true

━━━ Cycle 14:30:00 ━━━
🚀 Assigning abc123 to worker alpha
   Task: Implement user authentication

📊 Tasks: 0 backlog, 2 todo, 1 in-progress, 0 review, 0 done

━━━ Cycle 14:30:30 ━━━
📣 👷 alpha: poked (#1)

━━━ Cycle 14:31:00 ━━━
✅ Worker alpha finished task abc123
   📋 Moved task to Review
🔍 Starting reviewer charlie
```

## Task Analysis & Requeue

When `--auto-requeue` is enabled, the supervisor periodically analyzes tasks:

```bash
foundry supervisor --auto-requeue --stuck 30m --analyze-interval 10m
```

| Option | Default | Purpose |
|--------|---------|---------|
| `--auto-requeue` | false | Enable automatic requeue |
| `--stuck` | 30m | Time before task is considered stuck |
| `--analyze-interval` | 15m | How often to run analysis |

The analyzer:
- Identifies tasks stuck in `in_progress` too long
- Checks for abandoned work (worker stopped but task not moved)
- Requeues stuck tasks back to `todo` for reassignment
- Suggests new tasks based on patterns

## Stopping

Press `Ctrl+C` to gracefully stop the supervisor. Active workers will continue running independently.

To stop all agents including workers, use:
```bash
foundry shutdown
```
