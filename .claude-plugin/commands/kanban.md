---
name: foundry:kanban
description: Quick access to foundry kanban
aliases: ["kanban", "kb", "issues"]
user_invocable: true
---

# /foundry:kanban Command

Quick access to foundry kanban issue tracker.

## Usage

```
/kanban              # Show board
/kanban add <title>  # Add issue
/kanban <id>         # Show issue
/kanban <id> <status> # Move issue
```

## Behavior

1. **No arguments**: Show board
   ```bash
   foundry kanban
   ```

2. **"add" + title**: Add issue
   ```bash
   foundry kanban add "$TITLE"
   ```

3. **ID only**: Show details
   ```bash
   foundry kanban show "$ID"
   ```

4. **ID + status**: Move issue
   ```bash
   foundry kanban move "$ID" "$STATUS"
   ```

## Status Shortcuts

- `b` = backlog
- `t` = todo
- `p` = in_progress
- `r` = review
- `d` = done

## Examples

```
/kanban
/kanban add Fix the login bug
/kanban abc123
/kanban abc123 d
```
