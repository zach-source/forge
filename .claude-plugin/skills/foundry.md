---
name: foundry:main
description: Development orchestration platform. Use for supervisor, kanban, workers, board sync, leaders, and workspace management.
triggers:
  - "foundry"
  - "orchestration"
  - "development workflow"
  - "workspace"
  - "leader agent"
  - "supervisor"
---

# Foundry - Development Orchestration Platform

Foundry provides high-level development automation on top of forge.

## Features

| Feature | Command | Purpose |
|---------|---------|---------|
| Supervisor | `foundry supervisor` | Automated orchestration |
| Kanban | `foundry kanban` | Local issue tracking |
| Workers | `foundry worker` | Parallel Claude instances |
| Board Sync | `foundry board` | Notion/GitHub sync |
| Leaders | `foundry planner/reviewer/merge/deploy` | Specialized agents |
| Shutdown | `foundry shutdown` | Stop all agents |
| Workspace | `foundry init/repo/work` | Multi-repo management |

## Quick Start

```bash
# Supervisor - automated workflow
foundry supervisor --leaders --interval 30s

# Local kanban
foundry kanban add "Fix bug" -p high
foundry kanban move abc123 done

# Workers
foundry worker create
foundry worker start alpha --task "feature"

# Board sync
foundry board --sync           # Notion
foundry board --github --sync  # GitHub

# Leaders
foundry planner
foundry reviewer
```

## Automated Workflow (Supervisor)

```bash
# Setup
foundry kanban add "Task 1" -s todo
foundry worker create                    # alpha
foundry worker create --role reviewer    # bravo

# Run
foundry supervisor --leaders

# Workflow: todo → in_progress → review → done → merge → deploy
```

## Manual Workflow

```
foundry planner     → Plan tasks
foundry work start  → Create worktree
foundry worker      → Execute in parallel
foundry reviewer    → Review code
foundry merge       → Merge to main
foundry deploy      → Deploy & verify
```

## Relationship to Forge

- **Forge** = Run individual Claude sessions
- **Foundry** = Orchestrate multiple forge sessions

Foundry calls forge internally for session management.

## Data Storage

- `.beads/` - Issue database (used by kanban view)
- `.foundry/workspace.yaml` - Workspace config
- `~/.forge/workers/registry.yaml` - Worker registry
- `~/.forge/workers/locks/` - Resource locks
