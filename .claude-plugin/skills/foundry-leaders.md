---
name: foundry:leaders
description: Specialized leader agents for planning, review, merge, and deploy. Use for expert-level tasks requiring domain-specific prompts.
triggers:
  - "leader"
  - "planner"
  - "reviewer"
  - "merge leader"
  - "deploy leader"
  - "planning agent"
  - "review agent"
---

# Foundry Leaders - Specialized Agents

Leaders are specialized forge sessions with domain-specific prompts.

## Commands

```bash
foundry planner    # Planning and architecture
foundry reviewer   # Code review
foundry merge      # Merge coordination (single-threaded)
foundry deploy     # Deployment management (single-threaded)
```

## Leader Roles

| Leader | Purpose | Single-threaded |
|--------|---------|-----------------|
| `planner` | Task breakdown, architecture decisions | No |
| `reviewer` | Code review, quality checks | No |
| `merge` | PR merge, conflict resolution | Yes (locked) |
| `deploy` | Deployment, verification | Yes (locked) |

## Single-Threaded Leaders

Merge and deploy leaders use file-based locks (`~/.forge/workers/locks/`) to ensure only one instance runs at a time. This prevents:
- Conflicting merges
- Duplicate deployments
- Race conditions

## Manual vs Automated

### Manual Workflow

```bash
# 1. Plan work
foundry planner

# 2. Execute tasks (with workers)
foundry worker start alpha --task "implement feature"

# 3. Review
foundry reviewer

# 4. Merge
foundry merge

# 5. Deploy
foundry deploy
```

### Automated (Supervisor)

```bash
# Supervisor handles leader coordination automatically
foundry supervisor --leaders --interval 30s
```

When `--leaders` is enabled, the supervisor:

| Condition | Action |
|-----------|--------|
| Tasks in review | Launch reviewer |
| Backlog needs prioritization | Launch planner |
| All tasks done | Launch merge leader |
| Merge complete | Launch deploy leader |

## Creating Leader Workers

```bash
# Create workers with leader roles
foundry worker create --role planner
foundry worker create --role reviewer
foundry worker create --role merge
foundry worker create --role deploy

# List by role
foundry worker list --role reviewer
```

## Integration with Kanban

Leaders respond to kanban state:

```
backlog (planner) → todo → in_progress → review (reviewer) → done → merge → deploy
```
