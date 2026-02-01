# Forge & Foundry Development Guide

> Instructions for Claude when working on the forge/foundry codebase

## Project Overview

This repository contains two CLI tools:

**Forge** - Minimal Claude session runner
- Starts Claude in tmux for autonomous execution
- Detects completion promises
- Session lifecycle management (start, attach, status, cancel, list, log)

**Foundry** - Development orchestration platform
- Builds on top of forge for higher-level workflows
- Parallel workers with persistent identity
- External board sync (Notion, GitHub Projects)
- Local kanban issue tracking
- Workspace and worktree management
- Leader agents (planner, reviewer, merge, deploy)

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
    ├── kanban.go        # Local issue tracker
    ├── worker.go        # Parallel workers
    ├── board.go         # Notion/GitHub sync
    ├── monitor.go       # TUI dashboard
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
# Local tools
foundry kanban                  # Issue tracker
foundry kanban add "Fix bug"

# Workers (orchestrate multiple forge sessions)
foundry worker create
foundry worker start alpha --task "feature"

# External sync
foundry board --sync            # Notion
foundry board --github --sync   # GitHub Projects

# Leaders (launch forge with specific prompts)
foundry planner
foundry reviewer
foundry merge
foundry deploy

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
worker := registry.Create(worker.RoleWorker, "")
// worker.Name = "alpha" (NATO alphabet)
// worker.ID = "w-abc12345"
```

## State Files

- `~/.forge/sessions/*.state.md` - Forge session state
- `~/.forge/workers/registry.yaml` - Worker registry
- `.foundry/kanban.db` - Local issue database
- `.foundry/workspace.yaml` - Workspace config

## Code Style

- Use standard Go conventions
- Error messages: lowercase, no punctuation
- Keep forge minimal - don't add non-session features
- Add orchestration features to foundry
