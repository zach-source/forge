---
name: foundry:worker
description: Parallel workers with NATO alphabet naming and supervisor integration. Use when running multiple Claude instances in parallel.
triggers:
  - "worker"
  - "parallel workers"
  - "create worker"
  - "worker alpha"
  - "start worker"
  - "supervisor"
---

# Foundry Workers - Parallel Claude Instances

Manage multiple Claude instances with persistent identity.

## Commands

```bash
foundry worker create              # Create worker (alpha, bravo...)
foundry worker create --alias dev  # With alias
foundry worker create --role reviewer  # Leader role
foundry worker list                # List all workers
foundry worker list --active       # Active only
foundry worker list --role worker  # Filter by role
foundry worker start <name> --task "<task>"
foundry worker stop <name>
foundry worker pause <name>        # Suspend (Ctrl+Z)
foundry worker resume <name>       # Resume (fg)
foundry worker status <name>       # Detailed status
foundry worker attach <name>       # Attach to tmux
foundry worker log <name> -n 50    # View output
foundry worker reset <name>        # Clear assignment
foundry worker delete <name>
foundry worker reassign <from> <to>
```

## Worker Naming

Workers use NATO alphabet: alpha, bravo, charlie, delta, echo...

```bash
foundry worker create          # → alpha
foundry worker create          # → bravo
foundry worker create --alias "api-dev"
```

## Roles

| Role | Purpose | Single-threaded |
|------|---------|-----------------|
| `worker` | Development tasks | No |
| `planner` | Planning, task breakdown | No |
| `reviewer` | Code review | No |
| `merge` | PR merge coordination | Yes (locked) |
| `deploy` | Deployment | Yes (locked) |

```bash
foundry worker create --role worker    # Default
foundry worker create --role planner   # Planning
foundry worker create --role reviewer  # Review
foundry worker create --role merge     # Single-threaded
foundry worker create --role deploy    # Single-threaded
```

## Supervisor Integration

Workers are automatically managed by the supervisor:

```bash
# Setup workers
foundry worker create                    # alpha (worker)
foundry worker create --role reviewer    # bravo (reviewer)
foundry worker create --role merge       # charlie (merge)

# Run supervisor - handles assignment automatically
foundry supervisor --leaders --interval 30s
```

The supervisor will:
1. Assign idle workers to todo tasks
2. Move completed tasks to review
3. Launch reviewer for tasks in review
4. Launch merge/deploy after all tasks done

## Manual Workflow

```bash
# Create workers
foundry worker create
foundry worker create

# Start parallel work
foundry worker start alpha --task "implement auth"
foundry worker start bravo --task "build UI"

# Monitor
foundry worker list --active
foundry worker attach alpha

# Finish
foundry worker stop alpha
foundry worker reset alpha
```

## Worker Lifecycle

```
create → idle → start → active → stop → stopped
                  ↓              ↑
                pause → paused → resume
```

## Storage

- Registry: `~/.forge/workers/registry.yaml`
- Locks: `~/.forge/workers/locks/*.lock`
