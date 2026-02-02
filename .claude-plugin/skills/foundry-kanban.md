---
name: foundry:kanban
description: Kanban view on beads issue tracker. Use when tracking tasks, managing issues, or viewing project board.
triggers:
  - "kanban"
  - "add issue"
  - "track task"
  - "issue board"
  - "move issue"
---

# Foundry Kanban - View on Beads

Kanban-style frontend on top of the beads issue tracker (bd CLI).
Issues are stored in `.beads/` and can be managed with either `foundry kanban` or `bd` directly.

## Commands

```bash
foundry kanban              # View board
foundry kanban list         # List issues
foundry kanban add "Title"  # Add issue
foundry kanban show <id>    # Show details
foundry kanban move <id> <status>
foundry kanban edit <id>
foundry kanban delete <id>

# Or use bd directly:
bd list                     # List all issues
bd create "title"           # Create issue
bd update <id> -s in_progress
bd close <id>               # Mark done
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

## Status Mapping (Kanban ↔ Beads)

| Kanban Status | Beads Status |
|---------------|--------------|
| backlog       | open         |
| todo          | open         |
| in_progress   | in_progress  |
| review        | in_progress  |
| done          | closed       |

## Aliases

```bash
foundry kb          # kanban
foundry issues      # kanban
foundry i           # kanban
```
