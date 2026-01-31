---
name: foundry
description: Local-first development toolkit with kanban tracking. Use when tracking issues, viewing project board, or managing local development tasks.
triggers:
  - "foundry"
  - "local kanban"
  - "issue tracker"
  - "track tasks"
  - "project board"
---

# Foundry - Local-First Development Toolkit

Foundry is a lightweight, local-first toolkit for developers. All data is stored in `.foundry/` using SQLite - no external dependencies.

## Kanban Issue Tracker

### View Board
```bash
foundry kanban              # Show board
foundry kb                  # Alias
foundry i                   # Short alias
```

### Quick Add
```bash
foundry kanban add "Fix bug"
foundry kanban add "Feature" -p high -l "feature,api"
```

Options:
- `-p`: Priority (low, medium, high, critical)
- `-s`: Initial status (backlog, todo, in_progress, review, done)
- `-l`: Labels (comma-separated)
- `-a`: Assignee
- `-d`: Description

### Move with Shortcuts
```bash
foundry kanban move <id> <status>
```

Status shortcuts:
- `b` = backlog
- `t` = todo
- `p` = in_progress
- `r` = review
- `d` = done

Examples:
```bash
foundry kanban move abc123 t    # → todo
foundry kanban move abc123 p    # → in_progress
foundry kanban move abc123 d    # → done
```

### List & Filter
```bash
foundry kanban list              # All issues
foundry kanban ls -s todo        # Filter by status
```

### Show Details
```bash
foundry kanban show <id>
```

### Edit & Delete
```bash
foundry kanban edit <id> -t "New title" -p critical
foundry kanban delete <id> -f
```

## Workflow Example

```bash
# Start a task
foundry kanban add "Implement auth" -p high

# Work on it
foundry kanban move abc123 p

# Review
foundry kanban move abc123 r

# Complete
foundry kanban move abc123 d

# View progress
foundry kanban
```

## Data Storage

- Location: `.foundry/kanban.db`
- Format: SQLite (no CGO required)
- IDs: Short 8-char hex (e.g., `abc12345`)
