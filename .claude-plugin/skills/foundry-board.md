---
name: foundry:board
description: Sync with external project boards (Notion, GitHub Projects). Use when syncing tasks from project management tools.
triggers:
  - "board sync"
  - "notion sync"
  - "github projects"
  - "sync tasks"
---

# Foundry Board - External Project Sync

Sync with Notion or GitHub Projects.

## Providers

| Provider | Flag | Token |
|----------|------|-------|
| Notion | (default) | `NOTION_API_TOKEN` |
| GitHub Projects | `--github` | `GITHUB_TOKEN` |

## Commands

```bash
# Notion
foundry board --config       # Configure
foundry board --show         # Show config
foundry board --sync         # One-shot sync
foundry board --watch        # Continuous sync
foundry board                # Interactive

# GitHub Projects
foundry board --github --config
foundry board --github --show
foundry board --github --sync
foundry board --github --watch
foundry board --github
```

## Setup

### Notion

```bash
# 1. Create integration at notion.so/my-integrations
# 2. Share database with integration
# 3. Set token
export NOTION_API_TOKEN=secret_xxx

# 4. Configure
foundry board --config
```

### GitHub Projects

```bash
# 1. Create PAT with 'project' scope at github.com/settings/tokens
# 2. Set token
export GITHUB_TOKEN=ghp_xxx

# 3. Configure
foundry board --github --config
```

## Modes

| Mode | Description |
|------|-------------|
| `--sync` | One-shot sync, then exit |
| `--watch` | Continuous loop (30s interval) |
| (none) | Interactive Claude session |
