// Package kanban provides a simple issue tracker with kanban-style statuses.
package kanban

import (
	"time"

	"github.com/zach-source/forge/internal/complexity"
)

// Status represents the kanban column for an issue.
type Status string

const (
	StatusBacklog    Status = "backlog"
	StatusTodo       Status = "todo"
	StatusInProgress Status = "in_progress"
	StatusReview     Status = "review"
	StatusMerge      Status = "merge"
	StatusDone       Status = "done"
)

// ValidStatuses returns all valid status values.
func ValidStatuses() []Status {
	return []Status{StatusBacklog, StatusTodo, StatusInProgress, StatusReview, StatusMerge, StatusDone}
}

// Priority represents issue priority.
type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityMedium   Priority = "medium"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

// Issue represents a kanban issue/task.
type Issue struct {
	ID               string                `json:"id"`
	Title            string                `json:"title"`
	Description      string                `json:"description,omitempty"`
	Status           Status                `json:"status"`
	Priority         Priority              `json:"priority"`
	Complexity       complexity.Complexity `json:"complexity,omitempty"`
	ActualComplexity complexity.Complexity `json:"actual_complexity,omitempty"`
	Labels           []string              `json:"labels,omitempty"`
	Assignee         string                `json:"assignee,omitempty"`
	ParentID         string                `json:"parent_id,omitempty"` // For subtasks
	CreatedAt        time.Time             `json:"created_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
}

// IsValid checks if the issue has required fields.
func (i *Issue) IsValid() bool {
	return i.ID != "" && i.Title != "" && i.Status != ""
}

// Column represents a kanban column with issues.
type Column struct {
	Status Status
	Issues []*Issue
}

// Board represents a kanban board view.
type Board struct {
	Columns []Column
}
