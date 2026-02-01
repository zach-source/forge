---
name: foundry
description: Development orchestration platform
aliases: ["fd"]
user_invocable: true
---

# /foundry Command

Quick access to foundry orchestration.

## Usage

```
/foundry              # Show help
/foundry kanban       # View kanban board
/foundry worker       # List workers
/foundry board        # Board sync status
```

## Subcommands

| Command | Action |
|---------|--------|
| `/foundry kanban` | View kanban board |
| `/foundry kanban add X` | Add issue |
| `/foundry worker` | List workers |
| `/foundry worker create` | Create worker |
| `/foundry board --sync` | Sync with board |
| `/foundry planner` | Start planner |
| `/foundry init` | Init workspace |

## Examples

```
/foundry
/foundry kanban
/foundry kanban add Fix the login bug
/foundry worker create
/foundry board --github --sync
```
