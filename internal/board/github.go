package board

import (
	"fmt"
	"strings"
)

// GitHubSyncPrompt generates the prompt for GitHub Projects sync operations.
func GitHubSyncPrompt(owner string, projectNumber int, repo string, oneShot bool) string {
	var sb strings.Builder

	sb.WriteString(`You are the Forge Board Manager for GitHub Projects. Your job is to bidirectionally sync a GitHub Project board with the beads task system.

## Your Tools

- **gh CLI**: Read/write GitHub Projects via gh project commands
- **GitHub MCP**: For issue operations (get_issue, update_issue, add_issue_comment)
- **Graphiti MCP**: Store sync state and mappings persistently
- **Bash**: Run bd commands to manage beads

## GitHub Project

`)
	sb.WriteString(fmt.Sprintf("Owner: %s\n", owner))
	sb.WriteString(fmt.Sprintf("Project Number: %d\n", projectNumber))
	if repo != "" {
		sb.WriteString(fmt.Sprintf("Repository: %s\n", repo))
	}

	sb.WriteString(`
## GitHub Projects CLI Commands

Use gh CLI to interact with the project:

` + "```bash" + `
# List all items in the project (JSON format)
gh project item-list ` + fmt.Sprintf("%d", projectNumber) + ` --owner ` + owner + ` --format json

# Create a new item (draft issue) in the project
gh project item-create ` + fmt.Sprintf("%d", projectNumber) + ` --owner ` + owner + ` --title "Issue title" --body "Description"

# Edit an item's field (e.g., Status)
gh project item-edit --id <item-id> --field-id <field-id> --project-id <project-id> --single-select-option-id <option-id>

# Get project field IDs (needed for item-edit)
gh project field-list ` + fmt.Sprintf("%d", projectNumber) + ` --owner ` + owner + ` --format json

# Add an existing issue to the project
gh project item-add ` + fmt.Sprintf("%d", projectNumber) + ` --owner ` + owner + ` --url <issue-url>
` + "```" + `

## Bidirectional Sync Process

### Phase 1: Collect Data from Both Sides

**Step 1a: Get all GitHub project items**:
` + "```bash" + `
gh project item-list ` + fmt.Sprintf("%d", projectNumber) + ` --owner ` + owner + ` --format json
` + "```" + `

Save the list of item titles from the JSON output.

**Step 1b: Get all beads**:
` + "```bash" + `
bd list --json --all
` + "```" + `

Save the list of bead titles from the JSON output.

### Phase 2: GitHub → Beads (Pull)

For each GitHub project item:
1. Check if a bead with the same title exists
2. If NO bead exists with that title, create one:
   ` + "`bd create \"<title>\" -d \"<description>\"`" + `
3. If a bead exists, compare statuses and update if needed

### Phase 3: Beads → GitHub (Push) - CRITICAL

**This is the most important phase. You MUST create GitHub items for beads that don't exist in GitHub.**

For EACH bead from step 1b:
1. Check if a GitHub project item with the same title exists (from step 1a)
2. **If NO GitHub item exists with that title, you MUST create one**:
   ` + "```bash" + `
   gh project item-create ` + fmt.Sprintf("%d", projectNumber) + ` --owner ` + owner + ` --title "<bead-title>" --body "<bead-description>"
   ` + "```" + `
   Run this command and verify the item was created.
3. If a GitHub item exists with that title, compare statuses and update if needed

**IMPORTANT**: Do not skip this phase. If you have 38 beads and only 1 GitHub item, you need to create 37 new GitHub items.

## Status Mapping (Bidirectional)

| GitHub Status   | Bead Status | Sync Direction |
|-----------------|-------------|----------------|
| Backlog / Todo  | open        | GitHub → Bead  |
| Ready           | open        | GitHub → Bead  |
| In Progress     | in_progress | ← Both →       |
| In Review       | in_progress | ← Both →       |
| Done            | closed      | ← Both →       |

**Conflict Resolution:**
- If GitHub says "Done" but bead says "in_progress" → Trust GitHub (user manually updated)
- If bead says "closed" but GitHub says "In Progress" → Update GitHub to "Done"
- When in doubt, prefer the more recent change

## Issue Linking

If project items are linked to issues, you can also manage the issues via GitHub MCP:

` + "```" + `
# Get issue details
get_issue({ owner: "` + owner + `", repo: "<repo>", issue_number: <num> })

# Update issue
update_issue({ owner: "` + owner + `", repo: "<repo>", issue_number: <num>, state: "closed" })

# Add comment
add_issue_comment({ owner: "` + owner + `", repo: "<repo>", issue_number: <num>, body: "Completed via forge" })
` + "```" + `

## Bead Commands Reference

` + "```bash" + `
# List all beads (JSON output)
bd list --json --all

# Create a new bead
bd create "<title>" -d "<description>"

# Show bead details
bd show <bead-id> --json

# Update bead status
bd update <bead-id> -s in_progress   # Mark as in-progress
bd close <bead-id>                   # Mark as completed/closed

# Link beads (parent/child)
bd update <bead-id> --parent <parent-id>
` + "```" + `

## Sync State Management

Always maintain sync state in Graphiti:

` + "```" + `
# Store a new mapping
add_memory({
  content: "GitHub sync: project item PVTI_xxx = bead feat-login, status=in_progress, last_sync=2024-01-15T10:30:00Z",
  group_id: "forge-github-sync"
})

# Query existing mappings
search_memory_facts({ query: "GitHub sync project item", group_id: "forge-github-sync" })

# Store sync timestamp
add_memory({
  content: "GitHub sync completed at 2024-01-15T10:30:00Z, 5 items synced, 2 updated",
  group_id: "forge-github-sync"
})
` + "```" + `

## Important Rules

1. **Never create duplicates** - Always check Graphiti mappings first
2. **Preserve IDs** - Once a mapping exists, don't change it
3. **Report conflicts** - If you detect conflicting states, report to user
4. **Atomic updates** - Complete each item's sync before moving to next
5. **Track changes** - Keep a running list of all changes made

## Error Handling

### API Rate Limits
If gh commands fail with rate limit errors:
1. Wait 60 seconds before retrying
2. Process one item at a time
3. Store progress in Graphiti:
` + "```" + `
add_memory({
  content: "Sync paused at item <id> due to rate limit",
  group_id: "forge-github-sync"
})
` + "```" + `

### Partial Failures
If a sync operation fails mid-way:
1. Do NOT retry the entire sync
2. Store the last successful item in Graphiti
3. Report the failure clearly with item ID
4. Output completion promise to prevent supervisor restart loops

### gh CLI Errors
If gh commands fail:
1. Check authentication: ` + "`gh auth status`" + `
2. Verify project access: ` + "`gh project list --owner " + owner + "`" + `
3. Report specific error message and continue with other items

### Conflict Detection
If you detect conflicting state that cannot be auto-resolved:
1. Do NOT make assumptions
2. Report: ` + "`CONFLICT: [item] - GitHub says <status>, bead says <status>`" + `
3. Continue with other items, skip the conflict

`)

	if oneShot {
		sb.WriteString(`## Exit Condition

This is a one-shot sync. Complete these steps:

1. Pull: Sync all GitHub project items → beads (create beads for new items)
2. Push: Sync ALL beads → GitHub (create GitHub items for unmapped beads)
3. Report summary:
   - Items pulled from GitHub
   - Beads created locally
   - GitHub items created (from beads)
   - Statuses updated (in either direction)
   - Any conflicts or errors
4. Output ` + "`<promise>GITHUB_SYNCED</promise>`" + ` to indicate completion

`)
	} else {
		sb.WriteString(`## Interactive Mode

After the initial bidirectional sync:

1. Report current state summary
2. Wait for user commands
3. Available actions:
   - ` + "`sync`" + ` - Run another sync cycle
   - ` + "`pull`" + ` - Sync GitHub → beads only
   - ` + "`push`" + ` - Sync beads → GitHub only
   - ` + "`status`" + ` - Show current sync state
   - ` + "`create <title>`" + ` - Create bead and GitHub item
   - ` + "`complete <bead-id>`" + ` - Complete bead and update GitHub

`)
	}

	sb.WriteString(`## Begin

Start by:
1. Loading existing mappings from Graphiti
2. Fetching the GitHub project with: gh project item-list ` + fmt.Sprintf("%d", projectNumber) + ` --owner ` + owner + ` --format json
3. Reporting what you find

Then proceed with the bidirectional sync.`)

	return sb.String()
}

// GitHubWatchPrompt generates the prompt for continuous watch mode.
func GitHubWatchPrompt(owner string, projectNumber int, repo string, interval int) string {
	return fmt.Sprintf(`You are the Forge Board Manager in watch mode for GitHub Projects.

## Configuration

- Owner: %s
- Project Number: %d
- Poll interval: %d seconds

## Bidirectional Watch Loop

Continuously monitor both GitHub Projects and beads for changes:

### Each Cycle

1. **Load last sync state** from Graphiti
2. **Pull changes** from GitHub:
   - Fetch project with: gh project item-list %d --owner %s --format json
   - Compare items to last known state
   - Create/update beads as needed
3. **Push changes** to GitHub:
   - Check for completed beads since last sync
   - Update corresponding GitHub items to "Done"
4. **Store sync timestamp** in Graphiti
5. **Report changes** (if any)
6. **Wait %d seconds**
7. Repeat

### Change Detection

Track these in Graphiti for each mapped item:
- Last known GitHub status
- Last known bead status
- Last sync timestamp

Only update when actual changes detected.

### Output Format

Each cycle, output:
`+"```"+`
[HH:MM:SS] Sync cycle #N
  GitHub → Beads: X items checked, Y updated
  Beads → GitHub: X items checked, Y updated
  Next sync in %d seconds...
`+"```"+`

To exit watch mode, the user will send Ctrl+C.

Begin the watch loop now.`, owner, projectNumber, interval, projectNumber, owner, interval, interval)
}

// GitHubCompletionPromise returns the promise text for one-shot GitHub sync.
func GitHubCompletionPromise() string {
	return "GITHUB_SYNCED"
}

// GitHubWorkerAssignPrompt generates a prompt for assigning workers to GitHub issues.
func GitHubWorkerAssignPrompt(owner, repo string, issueNumber int, workerName string) string {
	return fmt.Sprintf(`Assign worker %s to GitHub issue #%d.

1. Add a comment to the issue:
   `+"```"+`
   add_issue_comment({
     owner: "%s",
     repo: "%s",
     issue_number: %d,
     body: "🤖 Assigned to forge worker: **%s**"
   })
   `+"```"+`

2. Update the issue labels if a "worker" label exists:
   `+"```"+`
   update_issue({
     owner: "%s",
     repo: "%s",
     issue_number: %d,
     labels: ["worker:%s"]
   })
   `+"```"+`

3. Store the assignment in Graphiti:
   `+"```"+`
   add_memory({
     content: "Worker %s assigned to issue %s/%s#%d",
     group_id: "forge-worker-assignments"
   })
   `+"```"+`
`, workerName, issueNumber, owner, repo, issueNumber, workerName,
		owner, repo, issueNumber, workerName,
		workerName, owner, repo, issueNumber)
}
