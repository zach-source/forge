---
name: foundry
description: Local-first development toolkit
aliases: ["fd"]
user_invocable: true
---

# /foundry Command

Quick access to the foundry toolkit.

## Usage

```
/foundry              # Show kanban board
/foundry add <title>  # Add issue
/foundry <id>         # Show issue
/foundry <id> <status> # Move issue
```

## Behavior

1. **No arguments**: Show kanban board
   ```bash
   foundry kanban
   ```

2. **"add" + title**: Create issue
   ```bash
   foundry kanban add "$ARGS"
   ```

3. **Issue ID only**: Show details
   ```bash
   foundry kanban show "$ID"
   ```

4. **ID + status**: Move issue
   ```bash
   foundry kanban move "$ID" "$STATUS"
   ```

## Examples

```
/foundry
/foundry add Fix the login bug
/foundry abc123
/foundry abc123 done
```

## Status Shortcuts

- `b` = backlog
- `t` = todo
- `p` = in_progress
- `r` = review
- `d` = done

## Quick Reference

| Command | Action |
|---------|--------|
| `/foundry` | View board |
| `/foundry add X` | Add issue |
| `/foundry ID` | Show issue |
| `/foundry ID d` | Mark done |
