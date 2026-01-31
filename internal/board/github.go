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

- **GitHub MCP**: Read/write GitHub Projects, issues, and pull requests
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
## GitHub Projects MCP Tools

Use these tools to interact with the project:

` + "```" + `
# List projects for the owner
projects_list({ owner: "` + owner + `" })

# Get project details including all items
projects_get({ owner: "` + owner + `", project_number: ` + fmt.Sprintf("%d", projectNumber) + ` })

# Update a project item (move between columns, update fields)
projects_write({
  owner: "` + owner + `",
  project_number: ` + fmt.Sprintf("%d", projectNumber) + `,
  item_id: "<item-id>",
  field: "Status",
  value: "Done"
})
` + "```" + `

## Bidirectional Sync Process

### Phase 1: Load Existing Mappings

First, check Graphiti for existing sync state:
` + "```" + `
search_memory_facts({ query: "github project bead mapping" })
` + "```" + `

This returns any previously stored GitHub Item ID ↔ Bead ID mappings.

### Phase 2: GitHub → Beads (Pull)

1. **Get all project items**:
   ` + "```" + `
   projects_get({ owner: "` + owner + `", project_number: ` + fmt.Sprintf("%d", projectNumber) + ` })
   ` + "```" + `

2. **For each project item**:
   - Check Graphiti for existing mapping
   - If no bead exists, create one:
     ` + "`bd create \"<title>\" -d \"<description>\"`" + `
   - Store the mapping in Graphiti:
     ` + "```" + `
     add_memory({
       content: "GitHub project item <item-id> maps to bead <bead-id>",
       group_id: "forge-github-sync"
     })
     ` + "```" + `
   - If bead exists, update status if GitHub status changed

### Phase 3: Beads → GitHub (Push)

1. **List all beads**: ` + "`bd list`" + `
2. **For each completed bead**:
   - Look up GitHub item ID from Graphiti mapping
   - Update GitHub project item status to "Done":
     ` + "```" + `
     projects_write({
       owner: "` + owner + `",
       project_number: ` + fmt.Sprintf("%d", projectNumber) + `,
       item_id: "<item-id>",
       field: "Status",
       value: "Done"
     })
     ` + "```" + `
3. **For beads marked active**:
   - Update GitHub status to "In Progress"

## Status Mapping (Bidirectional)

| GitHub Status   | Bead Status | Sync Direction |
|-----------------|-------------|----------------|
| Backlog / Todo  | pending     | GitHub → Bead  |
| Ready           | pending     | GitHub → Bead  |
| In Progress     | active      | ← Both →       |
| In Review       | active      | ← Both →       |
| Done            | completed   | ← Both →       |

**Conflict Resolution:**
- If GitHub says "Done" but bead says "active" → Trust GitHub (user manually updated)
- If bead says "completed" but GitHub says "In Progress" → Update GitHub to "Done"
- When in doubt, prefer the more recent change

## Issue Linking

If project items are linked to issues, you can also manage the issues:

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
# List all beads
bd list

# Create a new bead
bd create "<title>" -d "<description>"

# Show bead details
bd show <bead-id>

# Update bead status
bd start <bead-id>      # Mark as active/in-progress
bd complete <bead-id>   # Mark as completed

# Link beads (parent/child)
bd link <parent-id> <child-id>
` + "```" + `

## Sync State Management

Always maintain sync state in Graphiti:

` + "```" + `
# Store a new mapping
add_memory({
  content: "GitHub sync: project item PVTI_xxx = bead feat-login, status=active, last_sync=2024-01-15T10:30:00Z",
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

`)

	if oneShot {
		sb.WriteString(`## Exit Condition

This is a one-shot sync. Complete these steps:

1. Pull: Sync all GitHub project items → beads
2. Push: Sync all completed beads → GitHub
3. Report summary:
   - Items pulled from GitHub
   - Beads created
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
2. Fetching the GitHub project with projects_get
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
   - Fetch project with projects_get
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

Begin the watch loop now.`, owner, projectNumber, interval, interval, interval)
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
