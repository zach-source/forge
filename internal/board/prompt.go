// Package board provides board sync prompts for forge.
package board

import (
	"fmt"
	"strings"
)

// SyncPrompt generates the prompt for board sync operations.
func SyncPrompt(databaseID string, oneShot bool) string {
	var sb strings.Builder

	sb.WriteString(`You are the Forge Board Manager. Your job is to bidirectionally sync a Notion board with the beads task system.

## Your Tools

- **Notion MCP**: Read/write Notion pages and databases
- **Graphiti MCP**: Store sync state and mappings persistently
- **Bash**: Run bd commands to manage beads

## Notion Database

Database ID: `)
	sb.WriteString(databaseID)
	sb.WriteString(`

## Bidirectional Sync Process

### Phase 1: Load Existing Mappings

First, check Graphiti for existing sync state:
` + "```" + `
search_memory_facts({ query: "notion bead mapping" })
` + "```" + `

This returns any previously stored Notion Page ID ↔ Bead ID mappings.

### Phase 2: Notion → Beads (Pull)

1. **Query the Notion database** for all items
2. **For each Notion item**:
   - Check Graphiti for existing mapping
   - If no bead exists, create one:
     ` + "`bd create \"<title>\" -d \"<description>\"`" + `
   - Store the mapping in Graphiti:
     ` + "```" + `
     add_memory({
       content: "Notion page <page-id> maps to bead <bead-id>",
       group_id: "forge-board-sync"
     })
     ` + "```" + `
   - If bead exists, update status if Notion status changed

### Phase 3: Beads → Notion (Push)

1. **List all beads**: ` + "`bd list`" + `
2. **For each completed bead**:
   - Look up Notion page ID from Graphiti mapping
   - Update Notion page status to "Done" using Notion MCP:
     ` + "```" + `
     Use the Notion API to update the page properties:
     - Set Status property to "Done"
     ` + "```" + `
3. **For beads marked active**:
   - Update Notion status to "In Progress"

## Status Mapping (Bidirectional)

| Notion Status   | Bead Status | Sync Direction |
|-----------------|-------------|----------------|
| Backlog         | pending     | Notion → Bead  |
| Ready           | pending     | Notion → Bead  |
| In Progress     | active      | ← Both →       |
| Done            | completed   | ← Both →       |

**Conflict Resolution:**
- If Notion says "Done" but bead says "active" → Trust Notion (user manually updated)
- If bead says "completed" but Notion says "In Progress" → Update Notion to "Done"
- When in doubt, prefer the more recent change

## Hierarchy Handling

Notion Type → Bead Structure:
- **Epic** → Parent bead (no parent itself)
- **Feature** → Child of Epic bead
- **Task** → Child of Feature bead

Use the Parent relation in Notion to establish hierarchy:
` + "```bash" + `
# Link child to parent
bd link <parent-bead-id> <child-bead-id>
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
  content: "Board sync: Notion page abc123 = bead feat-login, status=active, last_sync=2024-01-15T10:30:00Z",
  group_id: "forge-board-sync"
})

# Query existing mappings
search_memory_facts({ query: "Board sync Notion page", group_id: "forge-board-sync" })

# Store sync timestamp
add_memory({
  content: "Board sync completed at 2024-01-15T10:30:00Z, 5 items synced, 2 updated",
  group_id: "forge-board-sync"
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
If you receive rate limit errors (429 status):
1. Wait 60 seconds before retrying
2. Reduce batch size (sync one item at a time)
3. Store progress in Graphiti so you can resume:
` + "```" + `
add_memory({
  content: "Sync paused at item <id> due to rate limit",
  group_id: "forge-board-sync"
})
` + "```" + `

### Partial Failures
If a sync operation fails mid-way:
1. Do NOT retry the entire sync
2. Store the last successful item in Graphiti
3. Report the failure clearly with item ID
4. Output completion promise to prevent supervisor restart loops

### Conflict Detection
If you detect conflicting state that cannot be auto-resolved:
1. Do NOT make assumptions
2. Create a conflict report:
` + "```" + `
add_memory({
  content: "CONFLICT: <item-id> - Notion says <status>, bead says <status>",
  group_id: "forge-board-sync"
})
` + "```" + `
3. Report to user: ` + "`CONFLICT: [item-id] - requires manual resolution`" + `

### Network Errors
If Notion API is unavailable:
1. Store partial progress in Graphiti
2. Report the error clearly
3. Output completion promise with error summary

`)

	if oneShot {
		sb.WriteString(`## Exit Condition

This is a one-shot sync. Complete these steps:

1. Pull: Sync all Notion items → beads
2. Push: Sync all completed beads → Notion
3. Report summary:
   - Items pulled from Notion
   - Beads created
   - Statuses updated (in either direction)
   - Any conflicts or errors
4. Output ` + "`<promise>SYNCED</promise>`" + ` to indicate completion

`)
	} else {
		sb.WriteString(`## Interactive Mode

After the initial bidirectional sync:

1. Report current state summary
2. Wait for user commands
3. Available actions:
   - ` + "`sync`" + ` - Run another sync cycle
   - ` + "`pull`" + ` - Sync Notion → beads only
   - ` + "`push`" + ` - Sync beads → Notion only
   - ` + "`status`" + ` - Show current sync state
   - ` + "`create <title>`" + ` - Create bead and Notion item
   - ` + "`complete <bead-id>`" + ` - Complete bead and update Notion

`)
	}

	sb.WriteString(`## Begin

Start by:
1. Loading existing mappings from Graphiti
2. Reading the Notion database
3. Reporting what you find

Then proceed with the bidirectional sync.`)

	return sb.String()
}

// WatchPrompt generates the prompt for continuous watch mode.
func WatchPrompt(databaseID string, interval int) string {
	return fmt.Sprintf(`You are the Forge Board Manager in watch mode.

## Configuration

- Database ID: %s
- Poll interval: %d seconds

## Bidirectional Watch Loop

Continuously monitor both Notion and beads for changes:

### Each Cycle

1. **Load last sync state** from Graphiti
2. **Pull changes** from Notion:
   - Query database for items modified since last sync
   - Create/update beads as needed
3. **Push changes** to Notion:
   - Check for completed beads since last sync
   - Update corresponding Notion items to "Done"
4. **Store sync timestamp** in Graphiti
5. **Report changes** (if any)
6. **Wait %d seconds**
7. Repeat

### Change Detection

Track these in Graphiti for each mapped item:
- Last known Notion status
- Last known bead status
- Last sync timestamp

Only update when actual changes detected.

### Output Format

Each cycle, output:
`+"```"+`
[HH:MM:SS] Sync cycle #N
  Notion → Beads: X items checked, Y updated
  Beads → Notion: X items checked, Y updated
  Next sync in %d seconds...
`+"```"+`

To exit watch mode, the user will send Ctrl+C.

Begin the watch loop now.`, databaseID, interval, interval, interval)
}

// CompletionPromise returns the promise text for one-shot sync.
func CompletionPromise() string {
	return "SYNCED"
}
