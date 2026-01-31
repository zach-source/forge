package worker

import (
	"fmt"
	"strings"
)

// WorkerIdentityPrompt generates the identity section to prepend to worker prompts.
func WorkerIdentityPrompt(w *Worker, taskID string) string {
	var sb strings.Builder

	sb.WriteString("## Your Identity\n\n")
	sb.WriteString(fmt.Sprintf("You are **%s** (ID: %s), a Forge %s.\n\n",
		w.DisplayName(), w.ShortID(), formatRole(w.Role)))

	if taskID != "" {
		sb.WriteString(fmt.Sprintf("**Current Task**: %s\n", taskID))
	}
	if w.Worktree != "" {
		sb.WriteString(fmt.Sprintf("**Worktree**: %s\n", w.Worktree))
	}

	sb.WriteString("\n## Memory Protocol\n\n")
	sb.WriteString(fmt.Sprintf("Store memories in Graphiti with your worker identity:\n"))
	sb.WriteString(fmt.Sprintf("- Group ID: `%s`\n", w.GraphitiGroupID()))
	sb.WriteString(fmt.Sprintf("- Include `worker:%s` tag in all memories\n\n", w.Name))

	sb.WriteString("Retrieve your context:\n")
	sb.WriteString(fmt.Sprintf("- `search_nodes({ query: \"worker:%s context\" })`\n", w.Name))
	sb.WriteString(fmt.Sprintf("- `search_memory_facts({ query: \"worker:%s decisions\" })`\n", w.Name))

	sb.WriteString("\n## Completion\n\n")
	sb.WriteString(fmt.Sprintf("When you have completed your task, output:\n"))
	sb.WriteString(fmt.Sprintf("```\n<promise>%s</promise>\n```\n", WorkerPromise(w)))

	return sb.String()
}

// formatRole formats a role for display in prompts.
func formatRole(role Role) string {
	switch role {
	case RoleWorker:
		return "Worker"
	case RolePlanner:
		return "Planning Leader"
	case RoleReviewer:
		return "Review Leader"
	case RoleMerge:
		return "Merge Leader"
	case RoleDeploy:
		return "Deployment Leader"
	default:
		return string(role)
	}
}

// TaskPrompt generates a task assignment prompt for a worker.
func TaskPrompt(w *Worker, task, description string) string {
	var sb strings.Builder

	identity := WorkerIdentityPrompt(w, task)
	sb.WriteString(identity)

	sb.WriteString("\n---\n\n")
	sb.WriteString("## Task\n\n")
	sb.WriteString(description)
	sb.WriteString("\n")

	return sb.String()
}

// ContinuationPrompt generates a prompt for continuing interrupted work.
func ContinuationPrompt(w *Worker, lastState string) string {
	var sb strings.Builder

	sb.WriteString("## Resuming Work\n\n")
	sb.WriteString(fmt.Sprintf("You are **%s** (ID: %s), resuming interrupted work.\n\n",
		w.DisplayName(), w.ShortID()))

	sb.WriteString("### Previous State\n\n")
	sb.WriteString(lastState)
	sb.WriteString("\n\n")

	sb.WriteString("### Instructions\n\n")
	sb.WriteString("1. Query your Graphiti memory for context:\n")
	sb.WriteString(fmt.Sprintf("   - `search_nodes({ query: \"worker:%s context\" })`\n", w.Name))
	sb.WriteString(fmt.Sprintf("   - `search_memory_facts({ query: \"worker:%s last session\" })`\n\n", w.Name))
	sb.WriteString("2. Review git status and recent changes\n")
	sb.WriteString("3. Continue from where you left off\n\n")

	sb.WriteString("## Completion\n\n")
	sb.WriteString(fmt.Sprintf("When complete, output: `<promise>%s</promise>`\n", WorkerPromise(w)))

	return sb.String()
}

// SessionCheckpoint generates a prompt for storing session state before ending.
func SessionCheckpoint(w *Worker) string {
	var sb strings.Builder

	sb.WriteString("## Session Checkpoint\n\n")
	sb.WriteString("Before this session ends, save your state to Graphiti:\n\n")
	sb.WriteString("1. Store current progress:\n")
	sb.WriteString("```\n")
	sb.WriteString(fmt.Sprintf(`add_memory({
  group_id: "%s",
  messages: [{
    role: "assistant",
    content: "Session checkpoint for worker:%s - [describe current state, what was completed, what remains]"
  }]
})`, w.GraphitiGroupID(), w.Name))
	sb.WriteString("\n```\n\n")

	sb.WriteString("2. Commit any pending changes with a descriptive message\n")
	sb.WriteString("3. Document key decisions and blockers encountered\n")

	return sb.String()
}

// CoordinationPrompt generates instructions for coordinating with other workers.
func CoordinationPrompt(workerNames []string) string {
	if len(workerNames) == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("## Coordination\n\n")
	sb.WriteString("Other workers are active in this workspace:\n")
	for _, name := range workerNames {
		sb.WriteString(fmt.Sprintf("- %s\n", name))
	}
	sb.WriteString("\n")

	sb.WriteString("**Coordination guidelines:**\n")
	sb.WriteString("- Avoid modifying the same files concurrently\n")
	sb.WriteString("- Use Graphiti to communicate status and blockers\n")
	sb.WriteString("- Tag coordination messages with `coordination` and worker names\n")
	sb.WriteString("- Check for conflicts before starting on shared components\n\n")

	return sb.String()
}
