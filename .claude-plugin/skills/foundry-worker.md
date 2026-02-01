---
name: foundry-worker
description: Parallel workers with NATO alphabet naming. Use when running multiple Claude instances in parallel.
triggers:
  - "worker"
  - "parallel workers"
  - "create worker"
  - "worker alpha"
  - "start worker"
---

# Foundry Workers - Parallel Claude Instances

Manage multiple Claude instances with persistent identity.

## Commands

```bash
foundry worker create              # Create worker (alpha, bravo...)
foundry worker list                # List all workers
foundry worker start <name> --task "<task>"
foundry worker stop <name>
foundry worker pause <name>
foundry worker resume <name>
foundry worker status <name>
foundry worker attach <name>
foundry worker log <name>
foundry worker reset <name>
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

```bash
foundry worker create --role worker    # Default
foundry worker create --role planner   # Planning
foundry worker create --role reviewer  # Review
foundry worker create --role merge     # Single-threaded
foundry worker create --role deploy    # Single-threaded
```

## Workflow

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

## Storage

Registry: `~/.forge/workers/registry.yaml`
