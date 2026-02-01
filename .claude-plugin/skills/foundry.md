---
name: foundry
description: Development orchestration platform. Use for kanban, workers, board sync, leaders, and workspace management.
triggers:
  - "foundry"
  - "orchestration"
  - "development workflow"
  - "workspace"
  - "leader agent"
---

# Foundry - Development Orchestration Platform

Foundry provides high-level development automation on top of forge.

## Features

| Feature | Command | Purpose |
|---------|---------|---------|
| Kanban | `foundry kanban` | Local issue tracking |
| Workers | `foundry worker` | Parallel Claude instances |
| Board Sync | `foundry board` | Notion/GitHub sync |
| Leaders | `foundry planner/reviewer/merge/deploy` | Specialized agents |
| Workspace | `foundry init/repo/work` | Multi-repo management |

## Quick Start

```bash
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

## Workflow

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

- `.foundry/kanban.db` - Local issues
- `.foundry/workspace.yaml` - Workspace config
- `~/.forge/workers/` - Worker registry
