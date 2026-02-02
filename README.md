# Forge & Foundry

Development automation tools for Claude-powered workflows.

## Overview

This repository contains two complementary CLI tools:

| Tool | Purpose | Scope |
|------|---------|-------|
| **Forge** | Claude session runner | Minimal - start/attach/monitor sessions |
| **Foundry** | Orchestration platform | Full - supervisor, workers, kanban, leaders |

```
┌─────────────────────────────────────────────────────────────────┐
│                        SUPERVISOR                                │
│     Orchestrates workers + leaders through kanban workflow       │
└───────────────────────────────┬─────────────────────────────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        ▼                       ▼                       ▼
┌───────────────┐      ┌───────────────┐      ┌───────────────┐
│    WORKERS    │      │    LEADERS    │      │    KANBAN     │
│ alpha, bravo  │      │ planner       │      │ View on beads │
│ charlie, ...  │      │ reviewer      │      │ issue tracker │
│ (NATO names)  │      │ merge, deploy │      │               │
└───────┬───────┘      └───────┬───────┘      └───────────────┘
        │                      │
        └──────────┬───────────┘
                   ▼
┌─────────────────────────────────────────────────────────────────┐
│                          FORGE                                   │
│             start │ attach │ status │ cancel │ list │ log       │
└───────────────────────────────┬─────────────────────────────────┘
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                       TMUX + CLAUDE                              │
└─────────────────────────────────────────────────────────────────┘
```

## Installation

```bash
# Build both tools
make build

# Install to ~/bin
make install

# Verify
forge --version
foundry --version
```

## Forge - Session Runner

Forge runs Claude sessions in tmux with completion detection.

```bash
# Start a session
forge start "implement user authentication"

# Attach to running session
forge attach

# Check status
forge status

# View output
forge log

# List all sessions
forge list

# Cancel a session
forge cancel
```

That's it. Forge is intentionally minimal.

## Foundry - Orchestration Platform

Foundry provides higher-level development automation.

### Local Kanban

View on beads issue tracker (`.beads/` via `bd` CLI):

```bash
foundry kanban                      # View board
foundry kanban add "Fix bug" -p high
foundry kanban move abc123 done     # or: move abc123 d
foundry kanban list
```

Status shortcuts: `b`acklog, `t`odo, `p`rogress, `r`eview, `d`one

### Supervisor

Automated orchestration that coordinates workers and leaders through the kanban workflow:

```bash
# Run supervisor with 30-second intervals
foundry supervisor --interval 30s

# Enable all leader agents (planner, reviewer, merge, deploy)
foundry supervisor --leaders

# Full autonomous workflow
foundry supervisor --interval 1m --leaders

# Task analysis and automatic requeue of stuck tasks
foundry supervisor --auto-requeue --stuck 30m

# Custom analysis interval
foundry supervisor --analyze-interval 10m --auto-requeue

# Health check: clean up orphaned tmux sessions on startup
foundry supervisor --cleanup-orphans

# Preview cleanup without taking action
foundry supervisor --cleanup-orphans --dry-run
```

Workflow: `todo` → `in_progress` (worker) → `review` (reviewer) → `done` → merge → deploy

Features:
- Assigns idle workers to todo tasks
- Pokes active workers periodically
- Analyzes stuck tasks and requeues them
- Launches leaders based on workflow state
- **Health checks**: Detects stale workers (session gone or Claude exited)
- **Orphan cleanup**: Cleans up untracked tmux sessions (forge-, mforge-, mf-)

### Parallel Workers

NATO-named workers with persistent identity:

```bash
# Create workers
foundry worker create               # Creates "alpha"
foundry worker create --alias dev   # Creates "bravo" with alias
foundry worker create --role planner # Create a leader worker

# Manage workers
foundry worker list                 # List all workers
foundry worker list --active        # Show only active
foundry worker start alpha --task "implement auth"
foundry worker stop alpha
foundry worker pause alpha          # Suspend session
foundry worker resume alpha         # Resume session

# Monitor
foundry worker status alpha         # Detailed status
foundry worker attach alpha         # Attach to tmux
foundry worker log alpha -n 50      # View output

# Reassign work
foundry worker reassign alpha bravo # Transfer task
```

Worker roles: `worker`, `planner`, `reviewer`, `merge`, `deploy`

### External Board Sync

Sync with Notion or GitHub Projects:

```bash
# Notion
export NOTION_API_TOKEN=secret_xxx
foundry board --config
foundry board --sync

# GitHub Projects
export GITHUB_TOKEN=ghp_xxx
foundry board --github --config
foundry board --github --sync
```

### Leader Agents

Specialized Claude agents for workflow stages:

```bash
foundry planner     # Strategic planning, task breakdown
foundry reviewer    # Code review, issue creation
foundry merge       # PR merge coordination (single-threaded)
foundry deploy      # Deployment and smoke testing
```

### Shutdown

Stop all running agents and sessions:

```bash
foundry shutdown              # Graceful shutdown of all agents
foundry shutdown --force      # Force kill all sessions
foundry stop-all              # Alias
foundry killall               # Alias
```

Stops: workers, forge sessions, leaders, board sync, orphaned tmux sessions.

### Workspace Management

Multi-repo workspace with git worktrees:

```bash
# Initialize workspace
foundry init my-project
cd my-project

# Add repositories
foundry repo add https://github.com/user/api.git
foundry repo add https://github.com/user/web.git

# Start feature development
foundry work start "user-auth" --repo api

# Complete feature
foundry work complete "user-auth"
```

## Development Workflow

### Automated (Supervisor)

```bash
# Add tasks to kanban
foundry kanban add "Set up React project" -p critical -s todo
foundry kanban add "Build user auth" -p high -s todo
foundry kanban add "Add tests" -p medium -s todo

# Create workers
foundry worker create                    # alpha (worker)
foundry worker create --role reviewer    # bravo (reviewer)
foundry worker create --role merge       # charlie (merge)

# Run supervisor - handles everything automatically
foundry supervisor --leaders --interval 30s
```

The supervisor will:
1. Assign idle workers to todo tasks
2. Move completed tasks to review
3. Launch reviewer to check completed work
4. Launch merge leader when all tasks done
5. Launch deploy leader after merge

### Manual

```
1. Plan      → foundry planner         → Creates tasks in kanban
2. Start     → foundry work start      → Creates isolated worktree
3. Develop   → foundry worker start    → Claude works autonomously
4. Review    → foundry reviewer        → Reviews completed work
5. Merge     → foundry merge           → Single-threaded merge
6. Deploy    → foundry deploy          → Deploy and verify
```

## Configuration

### Environment Variables

```bash
export NOTION_API_TOKEN=secret_xxx   # Notion integration
export GITHUB_TOKEN=ghp_xxx          # GitHub Projects
```

### File Locations

| Path | Purpose |
|------|---------|
| `~/.forge/sessions/*.state.md` | Session state files |
| `~/.forge/workers/registry.yaml` | Worker registry |
| `~/.forge/workers/locks/*.lock` | Resource locks (merge/deploy) |
| `.beads/` | Issue database (used by kanban) |
| `.foundry/workspace.yaml` | Workspace config |

## Development

```bash
make build          # Build both tools
make build-forge    # Build forge only
make build-foundry  # Build foundry only
make test           # Run tests
make fmt            # Format code
make lint           # Lint code
make install        # Install to ~/bin
```

## Architecture

```
cmd/
├── forge/              # Minimal session runner
│   ├── start.go
│   ├── attach.go
│   ├── status.go
│   ├── cancel.go
│   ├── list.go
│   └── log.go
│
└── foundry/            # Full orchestration
    ├── supervisor.go   # Orchestration loop
    ├── kanban.go       # Local issues
    ├── worker.go       # Parallel workers
    ├── board.go        # Board sync
    ├── monitor.go      # TUI dashboard
    ├── shutdown.go     # Stop all agents
    ├── planner.go      # Planning leader
    ├── reviewer.go     # Review leader
    ├── merge.go        # Merge leader
    ├── deploy.go       # Deploy leader
    ├── init.go         # Workspace init
    ├── repo.go         # Repo management
    └── work.go         # Worktree management

internal/
├── agent/              # Agent loop with tmux
├── worker/             # Worker system
│   ├── worker.go       # Worker type, roles, status
│   ├── registry.go     # YAML persistence, CRUD
│   ├── lifecycle.go    # Start, pause, resume, stop
│   ├── prompt.go       # Identity injection
│   ├── lock.go         # Resource locks (merge/deploy)
│   └── names.go        # NATO alphabet generation
├── kanban/             # Beads wrapper (bd CLI)
├── board/              # Board providers
├── leader/             # Leader prompts
├── workspace/          # Workspace ops
├── session/            # Session management
├── tmux/               # Tmux operations
└── ...
```

## License

MIT
