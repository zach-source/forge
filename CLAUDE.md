# Forge & Foundry Development Guide

> Instructions for Claude when working on the forge/foundry codebase

## Project Overview

This repository contains two CLI tools:

**Forge** - Minimal Claude session runner
- Starts Claude in tmux for autonomous execution
- Detects completion promises
- Session lifecycle management (start, attach, status, cancel, list, log)

**Foundry** - Development orchestration platform
- Supervisor for automated workflow orchestration
- Parallel workers with persistent identity (NATO alphabet naming)
- External board sync (Notion, GitHub Projects)
- Local kanban issue tracking (view on beads)
- Workspace and worktree management
- Leader agents (planner, reviewer, merge, deploy)
- Resource locking for single-threaded operations

## Architecture

```
cmd/
├── forge/               # Minimal Claude session runner
│   ├── main.go          # Entry point
│   ├── start.go         # Start Claude session
│   ├── attach.go        # Attach to tmux
│   ├── status.go        # Session status
│   ├── cancel.go        # Cancel session
│   ├── list.go          # List sessions
│   └── log.go           # View output
│
└── foundry/             # Orchestration platform
    ├── main.go          # Entry point
    ├── supervisor.go    # Orchestration loop
    ├── kanban.go        # Local issue tracker
    ├── worker.go        # Parallel workers
    ├── board.go         # Notion/GitHub sync
    ├── monitor.go       # TUI dashboard
    ├── shutdown.go      # Stop all agents
    ├── planner.go       # Planning leader
    ├── reviewer.go      # Review leader
    ├── merge.go         # Merge leader
    ├── deploy.go        # Deploy leader
    ├── init.go          # Workspace setup
    ├── repo.go          # Repository management
    └── work.go          # Worktree management

internal/
├── agent/           # Core agent loop (used by forge)
├── session/         # Session management (used by forge)
├── tmux/            # Tmux operations (used by forge)
├── detector/        # Completion detection (used by forge)
├── ralph/           # State files (used by forge)
├── kanban/          # Local issue tracker (used by foundry)
├── worker/          # Worker system (used by foundry)
│   ├── worker.go    # Worker type, roles, status
│   ├── registry.go  # YAML persistence, CRUD
│   ├── lifecycle.go # Start, pause, resume, stop
│   ├── prompt.go    # Identity injection
│   ├── lock.go      # Resource locks (merge/deploy)
│   └── names.go     # NATO alphabet generation
├── leader/          # Leader prompts (used by foundry)
├── board/           # Board providers (used by foundry)
├── workspace/       # Workspace ops (used by foundry)
├── mcp/             # MCP config (used by foundry)
├── monitor/         # TUI (used by foundry)
├── github/          # GitHub config (used by foundry)
├── notion/          # Notion config (used by foundry)
└── theme/           # UI theming (shared)
```

## Key Concepts

### Forge (Session Runner)

Forge is intentionally minimal - it only manages Claude sessions:

```bash
forge start "implement feature X"   # Start session
forge attach                        # Attach to tmux
forge status                        # Check status
forge cancel                        # Cancel
forge list                          # List sessions
forge log                           # View output
```

### Foundry (Orchestration)

Foundry provides higher-level features that use forge internally:

```bash
# Supervisor - automated orchestration
foundry supervisor                    # Default 2m interval, 4 workers
foundry supervisor --interval 30s     # Faster polling
foundry supervisor --max-workers 4    # Max concurrent workers (default 4)
foundry supervisor --leaders          # Enable all leader agents

# Local kanban
foundry kanban                        # View board
foundry kanban add "Fix bug" -p high -s todo

# Workers (parallel Claude sessions)
foundry worker create                 # Creates "alpha"
foundry worker create --role reviewer # Create leader worker
foundry worker start alpha --task "feature"
foundry worker list --active
foundry worker stop alpha

# External sync
foundry board --sync                  # Notion
foundry board --github --sync         # GitHub Projects

# Leaders (launch forge with specific prompts)
foundry planner
foundry reviewer
foundry merge
foundry deploy

# Shutdown all agents
foundry shutdown              # Graceful shutdown
foundry shutdown --force      # Force kill all sessions

# Workspace
foundry init
foundry repo add https://github.com/user/repo
foundry work start "feature"
```

## Development Commands

```bash
make build              # Build both forge and foundry
make build-forge        # Build forge only
make build-foundry      # Build foundry only
make test               # Run all tests
make install            # Install both to ~/bin
```

## Iteration Workflow

When iterating on forge/foundry code, use this command to build and install:

```bash
go install ./cmd/forge/ ./cmd/foundry/ && rm -f ~/bin/forge ~/bin/foundry && cp ~/go/bin/forge ~/go/bin/foundry ~/bin/
```

This ensures:
1. Both binaries are compiled with latest changes
2. Removes old binaries first (avoids stale file handle issues)
3. Copies to `~/bin/` which is first in PATH
4. No stale local binaries shadow the installed versions

**Verify installation:**
```bash
which foundry        # Should show ~/bin/foundry
foundry --version    # Should show "foundry version dev"
```

**Remove local binaries** (if any exist in working directory):
```bash
rm -f ./forge ./foundry
```

## Adding Features

### To Forge (session management only)

Add to `cmd/forge/` if it's about running/managing Claude sessions.

### To Foundry (everything else)

Add to `cmd/foundry/` for:
- Local tools (kanban, etc.)
- Orchestration (workers, leaders)
- External integrations (board sync)
- Workspace management

## Key Patterns

### Foundry calling Forge

Foundry commands that need Claude sessions should call forge:

```go
// In foundry command
cmd := exec.Command("forge", "start", prompt)
```

### Board Provider Pattern

```go
provider := board.GetProvider(board.ProviderGitHub)
provider.Configure()
prompt := provider.SyncPrompt(true)
```

### Worker Identity

```go
reg, _ := worker.LoadRegistry()
w, _ := reg.Create(worker.RoleWorker, "")
// w.Name = "alpha" (NATO alphabet)
// w.ID = "w-abc12345"

// Start worker
opts := worker.StartOptions{
    TaskID:   "task-123",
    Worktree: "/path/to/worktree",
    Prompt:   "Implement feature X",
}
worker.Start(ctx, reg, w.ID, opts)
```

### Worker Lifecycle

```go
worker.Start(ctx, reg, workerID, opts)  // Start session
worker.Pause(reg, workerID)              // Suspend (Ctrl+Z)
worker.Resume(reg, workerID)             // Resume (fg)
worker.Stop(reg, workerID)               // Kill session
worker.Reset(reg, workerID)              // Clear task/worktree
worker.Reassign(reg, fromID, toID)       // Transfer work
```

### Worker Roles

| Role | Purpose | Single-threaded |
|------|---------|-----------------|
| `worker` | Development tasks | No |
| `planner` | Planning, task breakdown | No |
| `reviewer` | Code review | No |
| `groomer` | Backlog research and detailing | No |
| `monitor` | Infrastructure health monitoring | No |
| `tester` | UI/API testing with browser automation | No |
| `merge` | PR merge coordination | Yes (locked) |
| `deploy` | Deployment | Yes (locked) |

### Leader Prompts

Each leader role can have a customizable prompt file in `.forge/prompts/`:

```
.forge/prompts/
├── planner.md      # Planning and prioritization instructions
├── reviewer.md     # Code review checklist and process
├── groomer.md      # Backlog grooming guidelines
├── merge.md        # Merge coordination process
├── deploy.md       # Deployment procedures
├── monitor.md      # Infrastructure monitoring checks
└── tester.md       # UI/API testing procedures
```

**How it works:**
- When a leader starts, it loads the custom prompt from `.forge/prompts/<role>.md`
- If no custom file exists, the built-in default prompt is used
- Dynamic board state (tasks in review, backlog items, etc.) is appended automatically
- Prompts can include Graphiti memory queries, CLI commands, and completion criteria

**Creating custom prompts:**
```bash
# Prompts are initialized when running foundry init or manually created
mkdir -p .forge/prompts
# Edit the prompt for a specific role
vim .forge/prompts/reviewer.md
```

Custom prompts allow workspace-specific context (project conventions, tech stack, review checklists) to be injected into leader sessions.

### Supervisor Pattern

```go
// Supervisor runs a loop that:
// 0. Health check → detects stale workers (session gone or Claude exited)
// 1. Checks for completed workers → moves tasks to review
// 2. Pokes active workers periodically
// 3. Assigns idle workers to todo tasks (up to --max-workers in parallel)
// 4. Analyzes stuck tasks → requeues if --auto-requeue enabled
// 5. Launches leaders when appropriate:
//    - Reviewer when tasks in review (runs alongside workers)
//    - Planner when backlog needs prioritization
//    - Merge when all tasks done
//    - Deploy after merge complete

// Config flags:
// --interval 30s       Cycle interval (default 2m)
// --max-workers 4      Max concurrent development workers (default 4)
// --analyze-interval   Task analysis interval (default 15m)
// --stuck 30m          Threshold for stuck tasks (default 30m)
// --auto-requeue       Automatically requeue stuck tasks
// --leaders            Enable all leader agents
// --no-auto-assign     Only monitor, don't assign tasks
// --cleanup-orphans    Clean up orphaned tmux sessions on startup
// --dry-run            Preview cleanup without taking action

// Task Isolation:
// - Each task gets worktree: .forge/worktrees/<task-id>/
// - Each task gets branch: task/<task-id>
// - Workers work in isolation, no file conflicts
// - Worktrees cleaned up when tasks complete
```

## State Files

- `~/.forge/sessions/*.state.md` - Forge session state
- `~/.forge/workers/registry.yaml` - Worker registry
- `.forge/worktrees/` - Task-specific worktrees (supervisor creates these)
- `.forge/prompts/` - Customizable leader prompt files (editable markdown)
- `~/.forge/workers/locks/*.lock` - Resource locks (merge/deploy)
- `.beads/` - Issue database (used by kanban view)
- `.foundry/workspace.yaml` - Workspace config

## Code Style

- Use standard Go conventions
- Error messages: lowercase, no punctuation
- Keep forge minimal - don't add non-session features
- Add orchestration features to foundry
