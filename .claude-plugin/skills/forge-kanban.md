---
name: forge-kanban
description: Local kanban issue tracker with SQLite storage. Use when tracking tasks, managing issues, or viewing project board.
triggers:
  - "track issue"
  - "add task"
  - "kanban board"
  - "issue tracker"
  - "move issue"
  - "list issues"
  - "show board"
---

# Forge Kanban - Local Issue Tracker

A lightweight, local-first kanban board stored in `.forge/kanban.db` using SQLite.

## Commands

### View Board
```bash
forge kanban              # Show kanban board view
forge kanban board        # Same as above
```

### List Issues
```bash
forge kanban list                    # All issues
forge kanban list -s todo            # Filter by status
forge kanban list -s in_progress     # Active work
```

### Add Issue
```bash
forge kanban add "Issue title"
forge kanban add "Bug fix" -p high -l "bug,urgent"
forge kanban add "Feature" -s todo -a "username"
```

Options:
- `-p, --priority`: low, medium, high, critical
- `-s, --status`: backlog, todo, in_progress, review, done
- `-l, --labels`: Comma-separated labels
- `-a, --assignee`: Assignee name
- `-d, --description`: Full description
- `--parent`: Parent issue ID for subtasks

### Move Issue
```bash
forge kanban move <id> <status>
forge kanban move abc123 in_progress
forge kanban move abc123 done
```

Valid statuses: `backlog`, `todo`, `in_progress`, `review`, `done`

### Show Issue
```bash
forge kanban show <id>
forge kanban show abc123
```

### Edit Issue
```bash
forge kanban edit <id> -t "New title"
forge kanban edit <id> -p critical
forge kanban edit <id> -l "new,labels"
```

### Delete Issue
```bash
forge kanban delete <id> -f    # Force delete
```

## Workflow Example

```bash
# Create issues for a feature
forge kanban add "Implement auth" -p high -l "feature"
forge kanban add "Write tests" --parent abc123

# Work on issues
forge kanban move abc123 in_progress
forge kanban move abc123 review
forge kanban move abc123 done

# View progress
forge kanban
```

## Issue IDs

Issues use short 8-character hex IDs (e.g., `abc12345`) for easy reference.

## Storage

Data is stored locally in `.forge/kanban.db` - no external dependencies required.
