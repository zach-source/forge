---
name: foundry:leaders
description: Launch foundry leader agents (planner, reviewer, merge, deploy)
aliases: ["leaders", "leader"]
user_invocable: true
---

# /foundry:leaders Command

Quick access to foundry leader agents for specialized tasks.

## Usage

```
/leaders                 # Show available leaders
/leaders planner         # Start planning leader
/leaders reviewer        # Start review leader
/leaders merge           # Start merge leader
/leaders deploy          # Start deploy leader
```

## Behavior

1. **No arguments**: Show available leaders
   ```bash
   echo "Available: planner, reviewer, merge, deploy"
   ```

2. **"planner"**: Start planning leader
   ```bash
   foundry planner
   ```

3. **"reviewer"**: Start review leader
   ```bash
   foundry reviewer
   ```

4. **"merge"**: Start merge leader (single-threaded, locked)
   ```bash
   foundry merge
   ```

5. **"deploy"**: Start deploy leader (single-threaded, locked)
   ```bash
   foundry deploy
   ```

## Leader Roles

| Leader | Purpose | Single-threaded |
|--------|---------|-----------------|
| `planner` | Planning and architecture | No |
| `reviewer` | Code review | No |
| `merge` | Merge coordination | Yes (locked) |
| `deploy` | Deployment management | Yes (locked) |

## Examples

```bash
# Manual workflow
foundry planner     # Plan tasks
foundry reviewer    # Review code
foundry merge       # Merge PRs
foundry deploy      # Deploy and verify

# Or use supervisor for automation
foundry supervisor --leaders
```
