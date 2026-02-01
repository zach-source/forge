---
name: foundry:forge
description: Start or manage Claude sessions
aliases: ["forge", "session"]
user_invocable: true
---

# /foundry:forge Command

Quick access to forge session runner.

## Usage

```
/forge                   # List sessions
/forge start <prompt>    # Start session
/forge attach            # Attach to session
/forge status            # Check status
```

## Behavior

1. **No arguments**: List sessions
   ```bash
   forge list
   ```

2. **"start" + prompt**: Start session
   ```bash
   forge start "$PROMPT"
   ```

3. **"attach"**: Attach to tmux
   ```bash
   forge attach
   ```

4. **"status"**: Show status
   ```bash
   forge status
   ```

## Examples

```
/forge
/forge start implement user auth
/forge attach
/forge status
```
