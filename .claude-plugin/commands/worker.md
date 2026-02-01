---
name: worker
description: Manage foundry parallel workers
aliases: ["w", "workers"]
user_invocable: true
---

# /worker Command

Quick access to foundry worker management.

## Usage

```
/worker              # List workers
/worker create       # Create new worker
/worker <name>       # Show worker status
```

## Behavior

1. **No arguments**: List workers
   ```bash
   foundry worker list
   ```

2. **"create"**: Create worker
   ```bash
   foundry worker create
   ```

3. **Worker name**: Show status
   ```bash
   foundry worker status "$NAME"
   ```

## Examples

```
/worker
/worker create
/worker alpha
```

## Quick Actions

```bash
foundry worker start alpha --task "implement feature"
foundry worker stop alpha
foundry worker attach alpha
foundry worker log alpha
```
