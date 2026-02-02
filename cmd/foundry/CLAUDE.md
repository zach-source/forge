# Foundry CLI

> Development orchestration platform built on top of forge

## Overview

Foundry provides higher-level development workflows that use forge sessions internally. While forge handles raw Claude session management, foundry orchestrates multi-agent development.

## Commands

### Supervisor (Automated Orchestration)

```bash
foundry supervisor                    # Default 2 minute interval
foundry supervisor --interval 30s     # Faster polling
foundry supervisor --leaders          # Enable all leader agents
foundry supervisor --no-auto-assign   # Only monitor, don't assign
foundry supervisor -d /path           # Custom working directory

# Task analysis and requeue
foundry supervisor --auto-requeue     # Auto-requeue stuck tasks
foundry supervisor --analyze-interval 10m  # Task analysis interval
foundry supervisor --stuck 30m        # Stuck task threshold
```

Workflow: `todo` → `in_progress` → `review` → `done` → merge → deploy

Features:
- Assigns idle workers to todo tasks
- Pokes active workers periodically
- Analyzes stuck tasks and requeues them (--auto-requeue)
- Launches leaders based on workflow state

### Kanban (View on Beads)

Kanban is a frontend view on top of the beads issue tracker (bd CLI).
Issues are stored in `.beads/` and managed by the `bd` command.

```bash
foundry kanban              # View kanban board (aliases: kb, issues, i)
foundry kanban add "title"  # Add issue with flags: -p priority, -s status, -l labels
foundry kanban move <id> t  # Move issue (shortcuts: b=backlog, t=todo, p=progress, r=review, d=done)
foundry kanban show <id>    # Show issue details
foundry kanban edit <id>    # Edit issue
foundry kanban delete <id>  # Delete issue (requires --force)

# Or use bd directly:
bd list                     # List all issues
bd create "title"           # Create issue
bd update <id> -s in_progress  # Update status
bd close <id>               # Close issue
```

### Worker Management

```bash
foundry worker create               # Create worker (auto-named: alpha, bravo, charlie...)
foundry worker create --alias name  # Create with alias
foundry worker create --role planner # Create leader worker
foundry worker list                 # List workers (--active, --idle, --role)
foundry worker start <name>         # Start worker with --task, --prompt, --worktree
foundry worker stop <name>          # Stop worker
foundry worker pause <name>         # Pause worker (Ctrl+Z)
foundry worker resume <name>        # Resume worker (fg)
foundry worker attach <name>        # Attach to tmux session
foundry worker log <name>           # View output (-n lines)
foundry worker status <name>        # Detailed status
foundry worker reset <name>         # Reset to idle
foundry worker delete <name>        # Delete worker (--force for active)
foundry worker reassign <from> <to> # Transfer task between workers
```

### Leaders (launch forge sessions with specialized prompts)

```bash
foundry planner    # Planning leader
foundry reviewer   # Review leader
foundry merge      # Merge leader (single-threaded, locked)
foundry deploy     # Deploy leader (single-threaded, locked)
```

### Shutdown (Stop All Agents)

```bash
foundry shutdown              # Graceful shutdown of all agents
foundry shutdown --force      # Force kill all sessions
foundry stop-all              # Alias
foundry killall               # Alias
```

Stops: foundry workers, forge sessions, leaders, board sync, orphaned tmux sessions.

### External Board Sync

```bash
foundry board --sync            # Sync with Notion
foundry board --github --sync   # Sync with GitHub Projects
```

### Workspace Management

```bash
foundry init                    # Initialize workspace
foundry repo add <url>          # Add repository
foundry work start "feature"    # Start feature worktree
```

## State Files

| File | Purpose |
|------|---------|
| `.beads/` | Beads issue database (used by kanban) |
| `.foundry/workspace.yaml` | Workspace configuration |
| `~/.forge/workers/registry.yaml` | Worker registry |
| `~/.forge/workers/locks/*.lock` | Resource locks (merge/deploy) |

## Adding Commands

1. Create `cmd/foundry/<command>.go`
2. Define `new<Command>Cmd() *cobra.Command`
3. Add to `rootCmd.AddCommand()` in `main.go`

### Pattern: Cobra Command

```go
func newFooCmd() *cobra.Command {
    var flag string

    cmd := &cobra.Command{
        Use:   "foo <arg>",
        Short: "Brief description",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            // Implementation
            return nil
        },
    }

    cmd.Flags().StringVarP(&flag, "flag", "f", "default", "Description")
    return cmd
}
```

### Pattern: Using Forge

When foundry needs Claude sessions, call forge:

```go
cmd := exec.Command("forge", "start", prompt)
```

### Pattern: Kanban Store

```go
store, err := getKanbanStore()  // Wraps bd CLI, uses .beads/
if err != nil {
    return err
}
defer store.Close()

issue := &kanban.Issue{Title: "...", Priority: kanban.PriorityMedium}
store.Create(issue)
```

### Pattern: Worker Registry

```go
reg, err := worker.LoadRegistry()
if err != nil {
    return fmt.Errorf("loading registry: %w", err)
}

w := reg.Get(workerID)  // By name, alias, or ID
worker.Start(ctx, reg, w.ID, opts)
```

### Pattern: Supervisor Loop

```go
// supervisor.go - main orchestration loop
func runCycle(cfg supervisorConfig, state *supervisorState) {
    // 1. Check for completed workers → move tasks to review
    checkCompletedWorkers(store, reg, state)

    // 2. Check leader sessions
    if cfg.withLeaders {
        checkLeaderSessions(reg, state)
    }

    // 3. Poke active workers
    pokeActiveWorkers(reg, state, cfg.maxPokes)

    // 4. Assign idle workers to todo tasks
    if cfg.autoAssign {
        assignTasks(store, reg, state, cfg.workDir)
    }

    // 5. Run leader workflow if enabled
    if cfg.withLeaders {
        runLeaderWorkflow(store, reg, state, cfg.workDir)
    }

    // 6. Analyze and requeue stuck tasks (if enabled)
    if cfg.autoRequeue && time.Since(state.lastAnalysis) > cfg.analyzeInterval {
        analyzeAndRequeueTasks(store, reg, state, cfg)
    }
}

// Task analysis uses Claude to:
// - Check if stuck tasks should be requeued
// - Suggest new tasks based on patterns
// - Identify abandoned work
```

## Worker Roles

| Role | Purpose | Single-threaded |
|------|---------|-----------------|
| `worker` | General development tasks | No |
| `planner` | Planning and architecture | No |
| `reviewer` | Code review | No |
| `merge` | Merge coordination | Yes (locked) |
| `deploy` | Deployment management | Yes (locked) |

Single-threaded roles use file-based locks in `~/.forge/workers/locks/`.

## Worker Lifecycle

```
create → idle → start → active → stop → stopped
                  ↓              ↑
                pause → paused → resume
```

## Internal Packages

| Package | Purpose |
|---------|---------|
| `internal/kanban` | Issue storage and rendering |
| `internal/worker` | Worker registry and lifecycle |
| `internal/leader` | Leader role prompts |
| `internal/board` | Board provider interface |
| `internal/workspace` | Workspace operations |
| `internal/monitor` | TUI dashboard |

## Code Style

- Use `getFoundryDir()` for `.foundry/` directory access
- Error messages: lowercase, no punctuation
- Commands use cobra conventions
- Table output via `text/tabwriter`
- Terminal width detection via `golang.org/x/term`
