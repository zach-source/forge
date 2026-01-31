# Forge

Autonomous Claude execution orchestrator for software development workflows.

Forge spawns Claude in tmux sessions for attachable, persistent execution with MCP server integration. It coordinates the full development lifecycle through specialized leader agents.

## Installation

```bash
# Build
make build

# Install to ~/bin
make install

# Verify
forge --version
```

## Quick Start

```bash
# Initialize a workspace
forge init my-project
cd my-project

# Add repositories
forge repo add https://github.com/user/api.git
forge repo describe api --summary "REST API" --tech "Go,PostgreSQL"

# Configure Notion (optional)
export NOTION_API_TOKEN=secret_xxx
forge board --config

# Start planning
forge planner
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         FORGE CLI                                │
├─────────────────────────────────────────────────────────────────┤
│  Workspace     │  Worktree      │  Leaders         │  Core      │
│  ─────────     │  ────────      │  ───────         │  ────      │
│  init          │  work start    │  planner         │  start     │
│  repo add      │  work list     │  reviewer        │  attach    │
│  repo link     │  work attach   │  merge           │  status    │
│  repo describe │  work complete │  deploy          │  monitor   │
│  board         │  work abandon  │                  │  cancel    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      TMUX SESSION                                │
│  claude -p "<prompt>" --mcp-config <config> --allowedTools "*"  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                       MCP SERVERS                                │
│  Notion │ Graphiti │ Context7 │ Sequential-Thinking             │
└─────────────────────────────────────────────────────────────────┘
```

## Commands

### Workspace Management

```bash
forge init [dir]                    # Initialize workspace
forge repo add <url>                # Clone repository
forge repo link <path>              # Link existing repo
forge repo list                     # List repositories
forge repo describe <name>          # Set repo metadata
forge repo primary <name>           # Set primary repo
forge repo sync                     # Update CLAUDE.md
```

### Worktree Development

```bash
forge work start <name> --repo api  # Create feature worktree
forge work list                     # List active worktrees
forge work attach <name>            # Launch Claude in worktree
forge work complete <name>          # Mark ready for merge
forge work abandon <name>           # Remove worktree
```

### Board & Notion

```bash
forge board --config                # Configure Notion database
forge board --sync                  # One-shot sync
forge board                         # Interactive session
forge board --watch                 # Continuous sync
```

### Leaders

```bash
forge planner                       # Strategic planning
forge reviewer                      # Code review
forge merge                         # Merge coordination
forge deploy                        # Deployment & testing
```

### Autonomous Execution

```bash
forge start "<task>" -p DONE        # Start autonomous task
forge attach                        # Attach to session
forge status                        # View session status
forge monitor                       # TUI dashboard
forge cancel                        # Cancel session
forge list                          # List all sessions
forge log                           # View session log
```

## Development Workflow

### Worktree-Based Development

1. **Plan**: `forge planner` → Create epics/features in Notion
2. **Create worktree**: `forge work start "feature-name" --repo api`
3. **Develop**: Claude works in isolated worktree
4. **Complete**: `forge work complete "feature-name"`
5. **Review**: `forge reviewer` → Review from main
6. **Merge**: `forge merge` → Single-threaded merge to main
7. **Deploy**: `forge deploy` → Deploy & smoke test

### Leader Responsibilities

| Leader | Works From | Creates |
|--------|------------|---------|
| Planner | Notion | Epics, Features → Notion + Beads |
| Reviewer | main/develop | Issues for code findings |
| Merge Leader | Feature branches | Merges to main |
| Deploy Leader | main | Issues for failures |

## Workspace Structure

```
workspace/
├── CLAUDE.md                 # Auto-generated documentation
├── .forge/
│   ├── workspace.yaml        # Workspace configuration
│   ├── repos/                # Cloned repositories
│   ├── worktrees/            # Feature worktrees
│   │   ├── <repo>/
│   │   │   └── <feature>/    # Isolated working directory
│   │   └── worktrees.yaml    # Worktree tracking
│   ├── sessions/             # Session state files
│   └── logs/                 # Log files
└── (linked repos)
```

## Configuration

### Environment Variables

```bash
export NOTION_API_TOKEN=secret_xxx  # Notion integration token
```

### Notion Setup

1. Create integration at [notion.so/my-integrations](https://www.notion.so/my-integrations)
2. Share database with integration
3. Run `forge board --config` with database ID

See [docs/notion-board-setup.md](docs/notion-board-setup.md) for detailed setup.

## MCP Servers

Forge uses these MCP servers:

| Server | Purpose |
|--------|---------|
| Notion | Board sync, issue management |
| Graphiti | Persistent memory, decisions |
| Context7 | Library documentation |
| Sequential-Thinking | Complex reasoning |

## Development

```bash
# Build
make build

# Run tests
make test

# Format code
make fmt

# Lint
make lint
```

## License

MIT
