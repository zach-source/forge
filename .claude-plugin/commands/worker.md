---
name: worker
description: Manage parallel forge workers
aliases: ["w", "workers"]
user_invocable: true
---

# /worker Command

Manage parallel Claude workers with NATO alphabet names.

## Usage

```
/worker              # List all workers
/worker create       # Create new worker
/worker <name>       # Show worker status
```

## Behavior

When invoked:

1. **No arguments**: List all workers
   ```bash
   forge worker list
   ```

2. **"create"**: Create a new worker
   ```bash
   forge worker create
   ```

3. **Worker name**: Show detailed status
   ```bash
   forge worker status "$ARGS"
   ```

## Examples

```
/worker
/worker create
/worker alpha
/worker start alpha --task "implement feature"
```

## Quick Actions

After listing workers:
- Start worker: `forge worker start <name> --task "<task>"`
- Stop worker: `forge worker stop <name>`
- Attach: `forge worker attach <name>`
- View logs: `forge worker log <name>`

## Worker Roles

- `worker` - General task execution (default)
- `planner` - Planning and coordination
- `reviewer` - Code review
- `merge` - PR management (single-threaded)
- `deploy` - Deployment (single-threaded)

Create with role:
```bash
forge worker create --role planner
```
