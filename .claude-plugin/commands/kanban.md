---
name: kanban
description: Quick access to local kanban issue tracker
aliases: ["kb", "issues"]
user_invocable: true
---

# /kanban Command

Quick access to the forge kanban issue tracker.

## Usage

```
/kanban              # Show board
/kanban add <title>  # Add issue
/kanban <id>         # Show issue details
```

## Behavior

When invoked:

1. **No arguments**: Display the kanban board view
   ```bash
   forge kanban
   ```

2. **With "add" + title**: Create a new issue
   ```bash
   forge kanban add "$ARGS"
   ```

3. **With issue ID**: Show issue details
   ```bash
   forge kanban show "$ARGS"
   ```

## Examples

```
/kanban
/kanban add Fix authentication bug
/kanban abc123
```

## Quick Actions

After viewing the board, suggest common actions:
- Move an issue: `forge kanban move <id> <status>`
- Add priority: `forge kanban add "title" -p high`
- Filter: `forge kanban list -s in_progress`
