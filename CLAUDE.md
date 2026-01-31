# Forge Development Guide

> Instructions for Claude when working on the forge codebase

## Project Overview

Forge is an autonomous Claude execution orchestrator written in Go. It manages Claude sessions in tmux, coordinates development workflows through specialized "leader" agents, and integrates with Notion for project management.

## Architecture

```
cmd/forge/           # CLI commands
├── main.go          # Entry point, command registration
├── start.go         # forge start - autonomous task execution
├── attach.go        # forge attach - attach to session
├── status.go        # forge status - session status
├── cancel.go        # forge cancel - cancel session
├── list.go          # forge list - list sessions
├── log.go           # forge log - view logs
├── monitor.go       # forge monitor - TUI dashboard
├── init.go          # forge init - workspace initialization
├── repo.go          # forge repo - repository management
├── work.go          # forge work - worktree management
├── board.go         # forge board - Notion sync
├── planner.go       # forge planner - planning leader
├── reviewer.go      # forge reviewer - review leader
├── merge.go         # forge merge - merge leader
└── deploy.go        # forge deploy - deployment leader

internal/
├── agent/           # Core agent loop
│   ├── agent.go     # Agent orchestration
│   └── config.go    # Agent configuration
├── board/           # Board sync prompts
│   └── prompt.go    # Notion sync prompt templates
├── detector/        # Completion detection
│   └── detector.go  # Promise matching
├── leader/          # Leader infrastructure
│   ├── leader.go    # Shared leader config
│   ├── planner.go   # Planner prompt
│   ├── reviewer.go  # Reviewer prompt
│   ├── merge.go     # Merge leader prompt
│   └── deploy.go    # Deploy leader prompt
├── mcp/             # MCP server configuration
│   ├── servers.go   # Server definitions
│   └── config.go    # Config generation
├── monitor/         # TUI components
│   ├── model.go     # Bubble Tea model
│   └── view.go      # View rendering
├── notion/          # Notion configuration
│   └── config.go    # Board config storage
├── ralph/           # Session state management
│   └── state.go     # State persistence
├── session/         # Session discovery
│   ├── session.go   # Session types
│   ├── manager.go   # Multi-session management
│   └── discover.go  # Session discovery
├── tmux/            # Tmux integration
│   └── session.go   # Tmux session management
├── workspace/       # Workspace management
│   ├── workspace.go # Core workspace types
│   ├── repo.go      # Repository operations
│   ├── worktree.go  # Git worktree management
│   ├── claudemd.go  # CLAUDE.md generation
│   └── detect.go    # Conflict detection
└── theme/           # UI theming
    └── theme.go     # Color schemes
```

## Key Patterns

### Command Structure

Each command follows this pattern:

```go
func newXxxCmd() *cobra.Command {
    var flags...

    cmd := &cobra.Command{
        Use:   "xxx",
        Short: "Brief description",
        Long:  `Detailed description`,
        RunE: func(cmd *cobra.Command, args []string) error {
            // Implementation
        },
    }

    cmd.Flags().StringVar(&flag, "name", "default", "description")
    return cmd
}
```

### Leader Pattern

Leaders share infrastructure in `internal/leader/leader.go`:

```go
cfg := leader.DefaultConfig(leader.RolePlanner)
prompt := leader.PlannerPrompt(databaseID, workDir)
return leader.Run(ctx, cfg, prompt, leader.PlannerPromise())
```

### Workspace Detection

Commands that need workspace context use:

```go
ws, err := workspace.Find(cwd)  // Walks up to find .forge/
```

### MCP Configuration

MCP servers are defined in `internal/mcp/servers.go`:

```go
"notion": {
    Type:    "stdio",
    Command: "npx",
    Args:    []string{"-y", "@notionhq/notion-mcp-server"},
    Env:     map[string]string{...},
},
```

## Development Commands

```bash
make build          # Build to ./bin/forge
make test           # Run all tests
make fmt            # Format code (gofumpt + goimports)
make lint           # Run golangci-lint
make install        # Install to ~/bin
```

## Testing

Run specific package tests:

```bash
go test -v ./internal/agent/...
go test -v ./internal/detector/...
```

## Adding New Features

### New Command

1. Create `cmd/forge/xxx.go` with `newXxxCmd()`
2. Register in `main.go`: `rootCmd.AddCommand(newXxxCmd())`
3. Add any needed internal packages

### New Leader

1. Add prompt in `internal/leader/xxx.go`
2. Add command in `cmd/forge/xxx.go`
3. Register in main.go

### New MCP Server

1. Add to `DefaultServers()` in `internal/mcp/servers.go`
2. Add to `AvailableServers()` list

## Code Style

- Use standard Go conventions
- Error messages: lowercase, no punctuation
- Comments: full sentences for exported items
- Prefer composition over inheritance
- Keep functions focused and small

## Key Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/lipgloss` - TUI styling
- `gopkg.in/yaml.v3` - YAML parsing

## State Files

- `~/.forge/sessions/*.state.md` - Global session state
- `.forge/workspace.yaml` - Workspace config
- `.forge/worktrees/worktrees.yaml` - Worktree tracking

## Common Tasks

### Add a flag to existing command

```go
cmd.Flags().BoolVar(&myFlag, "my-flag", false, "Description")
```

### Update workspace CLAUDE.md generation

Edit `internal/workspace/claudemd.go` - the `GenerateClaudeMD()` function.

### Modify leader prompts

Edit the appropriate file in `internal/leader/`:
- `planner.go` - Planning prompts
- `reviewer.go` - Review prompts
- `merge.go` - Merge prompts
- `deploy.go` - Deployment prompts

### Add worktree functionality

Edit `internal/workspace/worktree.go` for worktree operations.
