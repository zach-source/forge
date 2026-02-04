package leader

// Standard promise constants for consistent completion detection.
// All agents should use these constants for their completion promises.
const (
	// Leader promises
	PromisePlanner  = "PLANNER_COMPLETE"
	PromiseReviewer = "REVIEWER_COMPLETE"
	PromiseMerge    = "MERGE_COMPLETE"
	PromiseDeploy   = "DEPLOY_COMPLETE"

	// Sync promises
	PromiseSync       = "SYNC_COMPLETE"
	PromiseGitHubSync = "GITHUB_SYNCED"
	PromiseNotionSync = "NOTION_SYNCED"

	// Analyzer promises
	PromiseAnalyzer = "ANALYZER_COMPLETE"

	// Groomer promises
	PromiseGroomer = "GROOMER_COMPLETE"
)

// WorkerPromiseFormat is the format string for worker promises.
// Use fmt.Sprintf(WorkerPromiseFormat, taskID) to generate.
const WorkerPromiseFormat = "WORKER_%s_COMPLETE"

// OutputFormat provides the standard output format section for prompts.
const OutputFormat = `
## Output Format

### Status Reports

When reporting progress, use this structure:

` + "```" + `
### Status Report
- **Phase**: [Current workflow phase]
- **Progress**: [X/Y items processed]
- **Blockers**: [List any blockers or issues]
- **Next Action**: [What you'll do next]
` + "```" + `

### Item Creation

When creating items (issues, tasks, beads), output:
` + "```" + `
CREATED: [ID] - [Title]
` + "```" + `

### Completion

When finished, output your completion promise:
` + "```" + `
<promise>YOUR_PROMISE_HERE</promise>

## Summary
- Created: X items
- Updated: Y items
- Issues: Z (if any)
` + "```" + `
`

// HandoffProtocol provides instructions for passing context between agents.
const HandoffProtocol = `
## Handoff Protocol

Before completing, store a handoff summary for the next agent:

` + "```" + `
add_memory({
  group_id: "forge-handoff",
  content: ` + "`" + `
    FROM: [your-role]
    TO: [next-role]
    TASK: [task-id]
    SUMMARY: [what you accomplished]
    FILES_CHANGED: [list of files]
    CONCERNS: [any caveats for next agent]
    COMMIT: [commit hash if applicable]
  ` + "`" + `
})
` + "```" + `

Then output your completion promise.
`

// ErrorHandlingGuidance provides standard error handling instructions.
const ErrorHandlingGuidance = `
## Error Handling

### API Rate Limits
If you receive rate limit errors (429):
1. Wait 60 seconds before retrying
2. Reduce batch size (process one item at a time)
3. Store progress in Graphiti so you can resume

### Partial Failures
If an operation fails mid-way:
1. Do NOT retry the entire operation
2. Store last successful item in Graphiti
3. Report the failure clearly with item ID
4. Output promise anyway to prevent restart loops

### When Blocked
If you cannot proceed:
1. Document what you tried and why it failed
2. Store blocker in Graphiti with clear tags
3. Create an issue describing the blocker
4. Output your completion promise
`

// SequentialThinkingTriggers provides guidance on when to use chain-of-thought.
const SequentialThinkingTriggers = `
## When to Use Sequential Thinking

Use the Sequential Thinking MCP for complex decisions:

**DO use sequential thinking when:**
- Merge conflict spans multiple files
- Rollback decision with unclear impact
- Conflicting requirements between systems
- Complex dependency analysis
- Trade-off decisions with multiple factors

**Example invocation:**
` + "```" + `
sequential_thinking({
  question: "Should I merge branch X given conflicts in A, B, C?",
  context: "File A: auth logic, File B: tests, File C: docs"
})
` + "```" + `

**Do NOT use sequential thinking for:**
- Single-file changes
- Status updates
- Simple mapping operations
- Straightforward CRUD
`
