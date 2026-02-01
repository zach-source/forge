---
name: foundry:kanban
description: Local kanban issue tracker with SQLite. Use when tracking tasks, managing issues, or viewing project board.
triggers:
  - "kanban"
  - "add issue"
  - "track task"
  - "issue board"
  - "move issue"
---

# Foundry Kanban - Local Issue Tracker

SQLite-based issue tracker stored in `.foundry/kanban.db`.

## Commands

```bash
foundry kanban              # View board
foundry kanban list         # List issues
foundry kanban add "Title"  # Add issue
foundry kanban show <id>    # Show details
foundry kanban move <id> <status>
foundry kanban edit <id>
foundry kanban delete <id>
```

## Add Options

```bash
foundry kanban add "Title" \
  -p high \           # Priority: low, medium, high, critical
  -s todo \           # Status: backlog, todo, in_progress, review, done
  -l "bug,urgent" \   # Labels (comma-separated)
  -a "username" \     # Assignee
  -d "Description"    # Description
```

## Move Shortcuts

```bash
foundry kanban move abc123 b   # → backlog
foundry kanban move abc123 t   # → todo
foundry kanban move abc123 p   # → in_progress
foundry kanban move abc123 r   # → review
foundry kanban move abc123 d   # → done
```

## Aliases

```bash
foundry kb          # kanban
foundry issues      # kanban
foundry i           # kanban
```

## Issue IDs

Short 8-character hex IDs (e.g., `abc12345`).
