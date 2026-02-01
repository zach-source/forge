# Forge & Foundry

Development automation tools for Claude-powered workflows.

## Overview

This repository contains two complementary CLI tools:

| Tool | Purpose | Scope |
|------|---------|-------|
| **Forge** | Claude session runner | Minimal - start/attach/monitor sessions |
| **Foundry** | Orchestration platform | Full - workers, boards, kanban, workspace |

```
┌────────────────────────────────────────────────────────────────┐
│                         FOUNDRY                                 │
│  Workers │ Kanban │ Board Sync │ Leaders │ Workspace           │
└────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────────┐
│                          FORGE                                  │
│            start │ attach │ status │ cancel │ list │ log       │
└────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────────┐
│                      TMUX + CLAUDE                              │
└────────────────────────────────────────────────────────────────┘
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

SQLite-based issue tracker:

```bash
foundry kanban                      # View board
foundry kanban add "Fix bug" -p high
foundry kanban move abc123 done     # or: move abc123 d
foundry kanban list
```

Status shortcuts: `b`acklog, `t`odo, `p`rogress, `r`eview, `d`one

### Parallel Workers

NATO-named workers with persistent identity:

```bash
foundry worker create               # Creates "alpha"
foundry worker create               # Creates "bravo"
foundry worker start alpha --task "implement auth"
foundry worker list
foundry worker attach alpha
foundry worker stop alpha
```

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

```
1. Plan      → foundry planner         → Creates tasks in Notion/kanban
2. Start     → foundry work start      → Creates isolated worktree
3. Develop   → foundry worker start    → Claude works autonomously
4. Review    → foundry reviewer        → Reviews from main branch
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
| `~/.forge/sessions/` | Session state files |
| `~/.forge/workers/` | Worker registry |
| `.foundry/kanban.db` | Local issue database |
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
├── forge/           # Minimal session runner
│   ├── start.go
│   ├── attach.go
│   ├── status.go
│   ├── cancel.go
│   ├── list.go
│   └── log.go
│
└── foundry/         # Full orchestration
    ├── kanban.go    # Local issues
    ├── worker.go    # Parallel workers
    ├── board.go     # Board sync
    ├── monitor.go   # TUI dashboard
    ├── planner.go   # Planning leader
    ├── reviewer.go  # Review leader
    ├── merge.go     # Merge leader
    ├── deploy.go    # Deploy leader
    ├── init.go      # Workspace init
    ├── repo.go      # Repo management
    └── work.go      # Worktree management

internal/
├── agent/           # Agent loop
├── kanban/          # SQLite issue tracker
├── worker/          # Worker system
├── board/           # Board providers
├── leader/          # Leader prompts
├── workspace/       # Workspace ops
├── session/         # Session management
├── tmux/            # Tmux operations
└── ...
```

## License

MIT
