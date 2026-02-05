package kanban

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Store provides access to beads (bd) for kanban operations.
// This is a wrapper around the bd CLI - all data is stored in .beads/
type Store struct {
	workDir string
}

// NewStore creates a new store that wraps bd in the given directory.
// The directory should contain a .beads/ folder (run `bd init` if not).
func NewStore(workDir string) (*Store, error) {
	return &Store{workDir: workDir}, nil
}

// Close is a no-op for the bd wrapper (no resources to release).
func (s *Store) Close() error {
	return nil
}

// bdIssue represents the JSON structure returned by bd list --json
type bdIssue struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	Status          string    `json:"status"`
	Priority        int       `json:"priority"`
	IssueType       string    `json:"issue_type"`
	Owner           string    `json:"owner"`
	Labels          []string  `json:"labels"`
	ParentID        string    `json:"parent_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	DependencyCount int       `json:"dependency_count"`
	DependentCount  int       `json:"dependent_count"`
}

// Create inserts a new issue via bd create.
func (s *Store) Create(issue *Issue) error {
	args := []string{"create", issue.Title}

	if issue.Description != "" {
		args = append(args, "-d", issue.Description)
	}

	// Map kanban priority to bd priority (0-4, where 0 is highest)
	bdPriority := kanbanToBdPriority(issue.Priority)
	args = append(args, "-p", fmt.Sprintf("%d", bdPriority))

	if issue.Assignee != "" {
		args = append(args, "-a", issue.Assignee)
	}

	// Combine user labels with kanban status label
	allLabels := make([]string, 0, len(issue.Labels)+1)
	allLabels = append(allLabels, issue.Labels...)

	// Add kanban: label to preserve status
	if issue.Status == "" {
		issue.Status = StatusBacklog
	}
	if kanbanLabel := kanbanLabelForStatus(issue.Status); kanbanLabel != "" {
		allLabels = append(allLabels, kanbanLabel)
	}

	if len(allLabels) > 0 {
		args = append(args, "-l", strings.Join(allLabels, ","))
	}

	if issue.ParentID != "" {
		args = append(args, "--parent", issue.ParentID)
	}

	// bd outputs the created issue ID
	args = append(args, "--silent")

	cmd := exec.Command("bd", args...)
	cmd.Dir = s.workDir
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("bd create failed: %s", string(exitErr.Stderr))
		}
		return fmt.Errorf("bd create failed: %w", err)
	}

	issue.ID = strings.TrimSpace(string(out))
	issue.CreatedAt = time.Now()
	issue.UpdatedAt = time.Now()

	return nil
}

// Get retrieves an issue by ID via bd show --json.
func (s *Store) Get(id string) (*Issue, error) {
	cmd := exec.Command("bd", "show", id, "--json")
	cmd.Dir = s.workDir
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			// bd returns non-zero with empty stderr when issue doesn't exist
			if strings.Contains(stderr, "not found") || stderr == "" {
				return nil, nil
			}
			return nil, fmt.Errorf("bd show failed: %s", stderr)
		}
		return nil, fmt.Errorf("bd show failed: %w", err)
	}

	// Handle empty output (deleted or not found)
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" || trimmed == "[]" {
		return nil, nil
	}

	// bd show --json returns an array even for single issues
	var bdIssues []bdIssue
	if err := json.Unmarshal(out, &bdIssues); err != nil {
		return nil, fmt.Errorf("parsing bd output: %w", err)
	}

	if len(bdIssues) == 0 {
		return nil, nil
	}

	return bdToKanbanIssue(&bdIssues[0]), nil
}

// Update updates an existing issue via bd update.
func (s *Store) Update(issue *Issue) error {
	args := []string{"update", issue.ID}

	if issue.Title != "" {
		args = append(args, "--title", issue.Title)
	}

	if issue.Description != "" {
		args = append(args, "-d", issue.Description)
	}

	if issue.Priority != "" {
		bdPriority := kanbanToBdPriority(issue.Priority)
		args = append(args, "-p", fmt.Sprintf("%d", bdPriority))
	}

	if issue.Assignee != "" {
		args = append(args, "-a", issue.Assignee)
	}

	if len(issue.Labels) > 0 {
		args = append(args, "--set-labels", strings.Join(issue.Labels, ","))
	}

	cmd := exec.Command("bd", args...)
	cmd.Dir = s.workDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("bd update failed: %s", string(out))
	}

	issue.UpdatedAt = time.Now()
	return nil
}

// Delete removes an issue by ID via bd delete.
func (s *Store) Delete(id string) error {
	cmd := exec.Command("bd", "delete", id, "--force")
	cmd.Dir = s.workDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("bd delete failed: %s", string(out))
	}
	return nil
}

// List retrieves all issues via bd list --json, optionally filtered by status.
func (s *Store) List(status ...Status) ([]*Issue, error) {
	args := []string{"list", "--json", "--all", "--limit", "0"}

	// If filtering by specific statuses, we filter after fetching
	// bd only supports single status filter, so we do OR logic in Go

	cmd := exec.Command("bd", args...)
	cmd.Dir = s.workDir
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			// Empty database is not an error
			if strings.Contains(stderr, "no beads") {
				return nil, nil
			}
			return nil, fmt.Errorf("bd list failed: %s", stderr)
		}
		return nil, fmt.Errorf("bd list failed: %w", err)
	}

	var bdIssues []bdIssue
	if err := json.Unmarshal(out, &bdIssues); err != nil {
		return nil, fmt.Errorf("parsing bd output: %w", err)
	}

	// Convert and optionally filter
	statusSet := make(map[Status]bool)
	for _, s := range status {
		statusSet[s] = true
	}

	var issues []*Issue
	for _, bdi := range bdIssues {
		issue := bdToKanbanIssue(&bdi)
		if len(status) == 0 || statusSet[issue.Status] {
			issues = append(issues, issue)
		}
	}

	return issues, nil
}

// Move changes an issue's status via bd update --status or bd close.
// Also updates the kanban: label to preserve the 5-column status.
func (s *Store) Move(id string, status Status) error {
	bdStatus := kanbanToBdStatus(status)

	// For closed status, just close the bead (no label needed)
	if bdStatus == "closed" {
		cmd := exec.Command("bd", "close", id)
		cmd.Dir = s.workDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("bd close failed: %s", string(out))
		}
		// Remove any kanban: labels when closing
		_ = s.removeKanbanLabels(id)
		return nil
	}

	// Update bd status
	cmd := exec.Command("bd", "update", id, "-s", bdStatus)
	cmd.Dir = s.workDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("bd update failed: %s", string(out))
	}

	// Update kanban label to preserve the finer-grained status
	return s.updateKanbanLabel(id, status)
}

// updateKanbanLabel sets the kanban: label for an issue.
func (s *Store) updateKanbanLabel(id string, status Status) error {
	// Get current issue to preserve other labels
	issue, err := s.Get(id)
	if err != nil || issue == nil {
		return err
	}

	// Build new label list: keep non-kanban labels, add new kanban label
	var newLabels []string
	for _, label := range issue.Labels {
		if !strings.HasPrefix(label, "kanban:") {
			newLabels = append(newLabels, label)
		}
	}

	// Add the new kanban label
	if kanbanLabel := kanbanLabelForStatus(status); kanbanLabel != "" {
		newLabels = append(newLabels, kanbanLabel)
	}

	// Update labels (use --set-labels to replace all labels)
	if len(newLabels) > 0 {
		cmd := exec.Command("bd", "update", id, "--set-labels", strings.Join(newLabels, ","))
		cmd.Dir = s.workDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("bd update labels failed: %s", string(out))
		}
	} else {
		// Clear all labels if none remain
		cmd := exec.Command("bd", "update", id, "--set-labels", "")
		cmd.Dir = s.workDir
		_, _ = cmd.CombinedOutput() // Ignore error - some bd versions may not support empty labels
	}

	return nil
}

// removeKanbanLabels removes all kanban: prefixed labels from an issue.
func (s *Store) removeKanbanLabels(id string) error {
	issue, err := s.Get(id)
	if err != nil || issue == nil {
		return err
	}

	// Keep only non-kanban labels
	var newLabels []string
	hasKanbanLabel := false
	for _, label := range issue.Labels {
		if strings.HasPrefix(label, "kanban:") {
			hasKanbanLabel = true
		} else {
			newLabels = append(newLabels, label)
		}
	}

	// Only update if there was a kanban label to remove
	if hasKanbanLabel {
		labelArg := strings.Join(newLabels, ",")
		if labelArg == "" {
			labelArg = " " // Use space to clear labels (bd may require non-empty)
		}
		cmd := exec.Command("bd", "update", id, "--set-labels", labelArg)
		cmd.Dir = s.workDir
		_, _ = cmd.CombinedOutput() // Best effort
	}

	return nil
}

// GetBoard returns issues organized by status columns.
func (s *Store) GetBoard() (*Board, error) {
	issues, err := s.List()
	if err != nil {
		return nil, err
	}

	// Group by status
	byStatus := make(map[Status][]*Issue)
	for _, issue := range issues {
		byStatus[issue.Status] = append(byStatus[issue.Status], issue)
	}

	// Build columns in order
	board := &Board{}
	for _, status := range ValidStatuses() {
		board.Columns = append(board.Columns, Column{
			Status: status,
			Issues: byStatus[status],
		})
	}
	return board, nil
}

// GetChildren retrieves child issues of a parent via bd children.
func (s *Store) GetChildren(parentID string) ([]*Issue, error) {
	cmd := exec.Command("bd", "children", parentID, "--json")
	cmd.Dir = s.workDir
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "no children") || strings.Contains(stderr, "not found") {
				return nil, nil
			}
			return nil, fmt.Errorf("bd children failed: %s", stderr)
		}
		return nil, fmt.Errorf("bd children failed: %w", err)
	}

	var bdIssues []bdIssue
	if err := json.Unmarshal(out, &bdIssues); err != nil {
		return nil, fmt.Errorf("parsing bd output: %w", err)
	}

	var issues []*Issue
	for _, bdi := range bdIssues {
		issues = append(issues, bdToKanbanIssue(&bdi))
	}

	return issues, nil
}

// bdToKanbanIssue converts a bd issue to a kanban Issue.
func bdToKanbanIssue(bdi *bdIssue) *Issue {
	// Determine kanban status from bd status + labels
	status := bdToKanbanStatusWithLabels(bdi.Status, bdi.Labels)

	// Filter out kanban: prefixed labels from the visible labels
	var visibleLabels []string
	for _, label := range bdi.Labels {
		if !strings.HasPrefix(label, "kanban:") {
			visibleLabels = append(visibleLabels, label)
		}
	}

	return &Issue{
		ID:          bdi.ID,
		Title:       bdi.Title,
		Description: bdi.Description,
		Status:      status,
		Priority:    bdToKanbanPriority(bdi.Priority),
		Labels:      visibleLabels,
		Assignee:    bdi.Owner,
		ParentID:    bdi.ParentID,
		CreatedAt:   bdi.CreatedAt,
		UpdatedAt:   bdi.UpdatedAt,
	}
}

// Status mapping: kanban <-> bd
//
// Beads only has: open, in_progress, closed
// Kanban has: backlog, todo, in_progress, review, merge, done
//
// We use kanban:* labels to preserve the finer-grained kanban status.
//
// | Kanban Status  | Bd Status    | Label          |
// |----------------|--------------|----------------|
// | backlog        | open         | kanban:backlog |
// | todo           | open         | kanban:todo    |
// | in_progress    | in_progress  | kanban:wip     |
// | review         | in_progress  | kanban:review  |
// | merge          | in_progress  | kanban:merge   |
// | done           | closed       | (none needed)  |

// bdToKanbanStatusWithLabels determines kanban status from bd status + labels.
// This preserves the 5-column kanban view on top of beads' 3-status system.
func bdToKanbanStatusWithLabels(bdStatus string, labels []string) Status {
	// Closed always means done, regardless of any stale kanban labels
	if bdStatus == "closed" {
		return StatusDone
	}

	// Check for kanban: label to determine finer-grained status
	for _, label := range labels {
		switch label {
		case "kanban:backlog":
			return StatusBacklog
		case "kanban:todo":
			return StatusTodo
		case "kanban:wip":
			return StatusInProgress
		case "kanban:review":
			return StatusReview
		case "kanban:merge":
			return StatusMerge
		}
	}

	// Fall back to bd status mapping
	switch bdStatus {
	case "open":
		return StatusBacklog // Default open → backlog (use kanban:todo label for todo)
	case "in_progress":
		return StatusInProgress
	case "blocked":
		return StatusBacklog
	case "deferred":
		return StatusBacklog
	default:
		return StatusBacklog
	}
}

// bdToKanbanStatus is the simple mapping without labels (kept for compatibility).
func bdToKanbanStatus(bdStatus string) Status {
	return bdToKanbanStatusWithLabels(bdStatus, nil)
}

// kanbanLabelForStatus returns the kanban: label for a status, or empty if not needed.
func kanbanLabelForStatus(status Status) string {
	switch status {
	case StatusBacklog:
		return "kanban:backlog"
	case StatusTodo:
		return "kanban:todo"
	case StatusInProgress:
		return "kanban:wip"
	case StatusReview:
		return "kanban:review"
	case StatusMerge:
		return "kanban:merge"
	case StatusDone:
		return "" // closed status is unambiguous
	default:
		return ""
	}
}

func kanbanToBdStatus(status Status) string {
	switch status {
	case StatusBacklog, StatusTodo:
		return "open"
	case StatusInProgress, StatusReview, StatusMerge:
		return "in_progress"
	case StatusDone:
		return "closed"
	default:
		return "open"
	}
}

// Priority mapping: kanban <-> bd
//
// | Kanban Priority | Bd Priority |
// |-----------------|-------------|
// | critical        | 0           |
// | high            | 1           |
// | medium          | 2           |
// | low             | 3-4         |

func bdToKanbanPriority(bdPriority int) Priority {
	switch bdPriority {
	case 0:
		return PriorityCritical
	case 1:
		return PriorityHigh
	case 2:
		return PriorityMedium
	default:
		return PriorityLow
	}
}

func kanbanToBdPriority(priority Priority) int {
	switch priority {
	case PriorityCritical:
		return 0
	case PriorityHigh:
		return 1
	case PriorityMedium:
		return 2
	case PriorityLow:
		return 3
	default:
		return 2
	}
}
