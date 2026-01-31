---
name: forge-worker
description: Parallel worker management with NATO alphabet naming. Use when running multiple Claude instances in parallel or managing autonomous agents.
triggers:
  - "create worker"
  - "parallel workers"
  - "start worker"
  - "worker alpha"
  - "list workers"
  - "stop worker"
---

# Forge Workers - Parallel Claude Instances

Manage multiple Claude instances running in parallel with persistent identities.

## Concepts

- **Workers**: Named Claude instances (alpha, bravo, charlie...)
- **Registry**: Persistent storage at `~/.forge/workers/registry.yaml`
- **Roles**: worker, planner, reviewer, merge, deploy
- **Status**: idle, active, paused, stopped

## Commands

### Create Worker
```bash
forge worker create                  # Auto-name (alpha, bravo...)
forge worker create --alias "api"    # With custom alias
forge worker create --role planner   # Specific role
```

### List Workers
```bash
forge worker list                    # All workers
forge worker list --active           # Only active
forge worker list --idle             # Available workers
```

### Start Worker
```bash
forge worker start alpha --task "implement auth"
forge worker start alpha --task auth --worktree ./worktrees/api
```

### Lifecycle Management
```bash
forge worker pause alpha     # Suspend worker
forge worker resume alpha    # Resume paused worker
forge worker stop alpha      # Stop worker completely
forge worker reset alpha     # Reset stopped worker to idle
```

### Monitoring
```bash
forge worker status alpha    # Detailed status
forge worker attach alpha    # Attach to tmux session
forge worker log alpha       # View output
```

### Task Management
```bash
forge worker reassign alpha bravo    # Move task between workers
forge worker delete alpha            # Remove worker
```

## Workflow Example

```bash
# Create parallel workers
forge worker create --alias "api-dev"
forge worker create --alias "frontend-dev"

# Start on tasks
forge worker start alpha --task "implement auth" --worktree api/
forge worker start bravo --task "build ui" --worktree frontend/

# Monitor progress
forge worker list --active
forge worker status alpha

# Finish and cleanup
forge worker stop alpha
forge worker reset alpha
```

## Graphiti Integration

Workers store context in Graphiti with worker-tagged group IDs:
- Group: `forge-worker-{id}`
- Tags: `worker:{name}`

Query worker context:
```
search_nodes({ query: "worker:alpha context" })
search_memory_facts({ query: "worker:alpha decisions" })
```

## Resource Locking

Single-threaded roles (merge, deploy) use file-based locks to prevent conflicts:
- Locks stored at `~/.forge/workers/locks/`
- Automatically acquired/released during operations
