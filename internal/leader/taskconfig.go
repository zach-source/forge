// Package leader provides leader agent prompts and task configuration.
package leader

import (
	"encoding/json"
	"fmt"
	"strings"
)

// TaskConfig represents structured task configuration passed to workers.
type TaskConfig struct {
	// Worker identity
	Worker WorkerIdentity `json:"worker"`

	// Task details
	Task TaskDetails `json:"task"`

	// Workflow rules
	Workflow WorkflowRules `json:"workflow"`

	// Completion criteria
	Completion CompletionConfig `json:"completion"`
}

// WorkerIdentity identifies the worker.
type WorkerIdentity struct {
	Name   string `json:"name"`
	ID     string `json:"id"`
	Role   string `json:"role"`
	TaskID string `json:"task_id"`
}

// TaskDetails describes the task to be done.
type TaskDetails struct {
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Priority    string   `json:"priority"`
	Labels      []string `json:"labels,omitempty"`
}

// WorkflowRules defines what the worker should and shouldn't do.
type WorkflowRules struct {
	// What the worker should NOT do
	Prohibited []string `json:"prohibited"`

	// What the worker MUST do
	Required []string `json:"required"`

	// Working directory info
	Worktree string `json:"worktree"`
	Branch   string `json:"branch"`
}

// CompletionConfig defines how to signal completion.
type CompletionConfig struct {
	Promise string `json:"promise"`
	Format  string `json:"format"`
}

// ToJSON serializes the config to JSON.
func (tc *TaskConfig) ToJSON() (string, error) {
	data, err := json.MarshalIndent(tc, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ToPrompt converts the config to a prompt string.
func (tc *TaskConfig) ToPrompt() string {
	var sb strings.Builder

	sb.WriteString("## Task Configuration\n\n")
	sb.WriteString("```json\n")
	jsonStr, _ := tc.ToJSON()
	sb.WriteString(jsonStr)
	sb.WriteString("\n```\n\n")

	sb.WriteString("## Instructions\n\n")
	sb.WriteString(fmt.Sprintf("You are %s (ID: %s), a %s.\n\n", tc.Worker.Name, tc.Worker.ID, tc.Worker.Role))
	sb.WriteString(fmt.Sprintf("**Task**: %s\n\n", tc.Task.Title))

	if tc.Task.Description != "" {
		sb.WriteString(tc.Task.Description)
		sb.WriteString("\n\n")
	}

	if len(tc.Workflow.Prohibited) > 0 {
		sb.WriteString("**Do NOT**:\n")
		for _, rule := range tc.Workflow.Prohibited {
			sb.WriteString(fmt.Sprintf("- %s\n", rule))
		}
		sb.WriteString("\n")
	}

	if len(tc.Workflow.Required) > 0 {
		sb.WriteString("**You MUST**:\n")
		for _, rule := range tc.Workflow.Required {
			sb.WriteString(fmt.Sprintf("- %s\n", rule))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("**Completion**: Output `%s` in format: `%s`\n", tc.Completion.Promise, tc.Completion.Format))

	return sb.String()
}

// NewTaskConfig creates a task config from parameters.
func NewTaskConfig(workerName, workerID, role, taskID, title, description, priority, promise, worktree, branch string) *TaskConfig {
	return &TaskConfig{
		Worker: WorkerIdentity{
			Name:   workerName,
			ID:     workerID,
			Role:   role,
			TaskID: taskID,
		},
		Task: TaskDetails{
			Title:       title,
			Description: description,
			Priority:    priority,
		},
		Workflow: WorkflowRules{
			Prohibited: []string{
				"Move task to 'done' - supervisor handles transitions",
				"Run `foundry kanban move <id> done`",
				"Merge to main branch - merge leader handles that",
			},
			Required: []string{
				"Commit changes to your task branch",
				"Run tests to verify changes",
				"Output completion promise when done",
			},
			Worktree: worktree,
			Branch:   branch,
		},
		Completion: CompletionConfig{
			Promise: promise,
			Format:  "<promise>" + promise + "</promise>",
		},
	}
}

// SupervisorMessage represents a structured message from supervisor.
type SupervisorMessage struct {
	Type    string `json:"type"`    // "poke", "status", "warning", "error"
	Count   int    `json:"count"`   // For pokes, the poke number
	Message string `json:"message"` // Human-readable message
	Action  string `json:"action"`  // Suggested action
}

// ToJSON serializes the message.
func (sm *SupervisorMessage) ToJSON() string {
	data, _ := json.Marshal(sm)
	return string(data)
}

// BuildPokeMessage creates a structured poke message.
func BuildPokeMessage(pokeCount int) string {
	msg := SupervisorMessage{
		Type:  "poke",
		Count: pokeCount,
	}

	switch {
	case pokeCount >= 8:
		msg.Message = "Urgent: complete immediately"
		msg.Action = "output_promise"
	case pokeCount >= 5:
		msg.Message = "Time to finish"
		msg.Action = "wrap_up"
	case pokeCount >= 3:
		msg.Message = "Please wrap up soon"
		msg.Action = "finalize"
	default:
		msg.Message = "Status check"
		msg.Action = "continue"
	}

	return fmt.Sprintf("echo '%s'", msg.ToJSON())
}
