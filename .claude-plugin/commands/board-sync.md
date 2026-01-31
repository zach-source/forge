---
name: board-sync
description: Sync with Notion or GitHub Projects
aliases: ["sync", "board"]
user_invocable: true
---

# /board-sync Command

Sync tasks with external project management boards.

## Usage

```
/board-sync              # Sync with default provider (Notion)
/board-sync github       # Sync with GitHub Projects
/board-sync config       # Configure provider
```

## Behavior

When invoked:

1. **No arguments**: One-shot sync with default provider
   ```bash
   forge board --sync
   ```

2. **"github"**: Sync with GitHub Projects
   ```bash
   forge board --github --sync
   ```

3. **"config"**: Configure the provider
   ```bash
   forge board --config
   ```

4. **"github config"**: Configure GitHub Projects
   ```bash
   forge board --github --config
   ```

## Examples

```
/board-sync
/board-sync github
/board-sync config
/board-sync github config
```

## Environment Setup

### Notion
```bash
export NOTION_API_TOKEN=secret_xxx
```

### GitHub Projects
```bash
export GITHUB_TOKEN=ghp_xxx
```

## Modes

- `--sync`: One-shot sync, then exit
- `--watch`: Continuous sync loop
- (no flag): Interactive Claude session

## Provider Status

Check configuration:
```bash
forge board --show
forge board --github --show
```
