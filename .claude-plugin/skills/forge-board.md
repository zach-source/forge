---
name: forge-board
description: Sync with external project boards (Notion, GitHub Projects). Use when syncing tasks from project management tools.
triggers:
  - "sync notion"
  - "sync github projects"
  - "board sync"
  - "configure board"
  - "project management"
---

# Forge Board - External Project Management Sync

Bidirectional sync between external project management systems and forge.

## Supported Providers

### Notion (Default)
```bash
forge board                  # Interactive session
forge board --sync           # One-shot sync
forge board --watch          # Continuous sync
forge board --config         # Configure database
forge board --show           # Show config
```

### GitHub Projects
```bash
forge board --github         # Interactive session
forge board --github --sync  # One-shot sync
forge board --github --watch # Continuous sync
forge board --github --config
forge board --github --show
```

## Setup

### Notion Setup
1. Create integration at https://www.notion.so/my-integrations
2. Share database with integration
3. Set environment variable:
   ```bash
   export NOTION_API_TOKEN=secret_xxx
   ```
4. Configure database:
   ```bash
   forge board --config
   ```

### GitHub Projects Setup
1. Create Personal Access Token with `project` scope at https://github.com/settings/tokens
2. Set environment variable:
   ```bash
   export GITHUB_TOKEN=ghp_xxx
   ```
3. Configure project:
   ```bash
   forge board --github --config
   ```

## Sync Modes

### One-Shot Sync
```bash
forge board --sync
forge board --github --sync
```
Syncs once and exits. Good for CI/CD or manual sync.

### Watch Mode
```bash
forge board --watch
forge board --github --watch
```
Continuous sync loop, checking every 30 seconds.

### Interactive Mode
```bash
forge board
forge board --github
```
Starts Claude session with board MCP for interactive work.

## Configuration

Configuration stored at:
- Notion: `~/.forge/notion.yaml`
- GitHub: `~/.forge/github-projects.yaml`

View current config:
```bash
forge board --show
forge board --github --show
```

## MCP Servers

Board sessions include these MCP servers:
- `notion` or `github` - Provider-specific API access
- `graphiti` - Memory and context
- `context7` - Documentation lookup
