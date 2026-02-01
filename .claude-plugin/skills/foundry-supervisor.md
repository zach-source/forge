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
foundry supervisor --no-auto-assign   # Only monitor, don't assign
foundry supervisor -d /path           # Custom working directory
```

## Workflow

```
todo → in_progress (worker) → review (reviewer) → done → merge → deploy
```

The supervisor:
1. **Assigns tasks** - Idle workers get todo tasks
2. **Monitors progress** - Pokes active workers periodically
3. **Handles completion** - Moves finished tasks to review
4. **Coordinates leaders** - Launches reviewer/merge/deploy when appropriate

## Setup

```bash
# 1. Create kanban tasks
foundry kanban add "Task 1" -p high -s todo
foundry kanban add "Task 2" -p medium -s todo

# 2. Create workers
foundry worker create                    # alpha (worker)
foundry worker create                    # bravo (worker)
foundry worker create --role reviewer    # charlie (reviewer)
foundry worker create --role merge       # delta (merge)
foundry worker create --role deploy      # echo (deploy)

# 3. Run supervisor
foundry supervisor --leaders --interval 30s
```

## Leader Workflow

When `--leaders` is enabled:

| Condition | Action |
|-----------|--------|
| Tasks in review | Launch reviewer |
| Backlog needs prioritization | Launch planner |
| All tasks done | Launch merge leader |
| Merge complete | Launch deploy leader |

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

## Stopping

Press `Ctrl+C` to gracefully stop the supervisor. Active workers will continue running independently.
