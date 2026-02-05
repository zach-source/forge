// Package worker provides worker identity and lifecycle management for forge.
// Workers are persistent agent identities that can be assigned tasks, paused,
// resumed, and maintain memory across sessions via Graphiti.
package worker

import (
	"fmt"
	"time"
)

// Worker represents a forge worker with persistent identity.
type Worker struct {
	// ID is the unique identifier (e.g., "w-a1b2c3d4")
	ID string `yaml:"id"`
	// Name is the human-readable name (NATO alphabet: alpha, bravo, etc.)
	Name string `yaml:"name"`
	// Alias is an optional user-defined name
	Alias string `yaml:"alias,omitempty"`
	// Role defines the worker's function
	Role Role `yaml:"role"`
	// Status is the current worker status
	Status Status `yaml:"status"`
	// CurrentTask is the ID of the assigned task
	CurrentTask string `yaml:"current_task,omitempty"`
	// Worktree is the path to the worker's git worktree
	Worktree string `yaml:"worktree,omitempty"`
	// SessionID is the tmux session ID when active
	SessionID string `yaml:"session_id,omitempty"`
	// CreatedAt is when the worker was created
	CreatedAt time.Time `yaml:"created_at"`
	// LastActive is when the worker was last active
	LastActive time.Time `yaml:"last_active"`
}

// Role defines the type of work a worker performs.
type Role string

const (
	// RoleWorker is a general-purpose worker for feature development
	RoleWorker Role = "worker"
	// RolePlanner is a planning leader
	RolePlanner Role = "planner"
	// RoleReviewer is a code review leader
	RoleReviewer Role = "reviewer"
	// RoleMerge is a merge leader (single-threaded)
	RoleMerge Role = "merge"
	// RoleDeploy is a deployment leader (single-threaded)
	RoleDeploy Role = "deploy"
	// RoleGroomer researches and details backlog items before moving to todo
	RoleGroomer Role = "groomer"
	// RoleMonitor watches infrastructure health (k0s, prometheus, loki, flux)
	RoleMonitor Role = "monitor"
	// RoleTester validates UI and API changes using browser automation
	RoleTester Role = "tester"
	// RolePM is the project manager that identifies high-value next tasks
	RolePM Role = "pm"
	// RoleCICD monitors CI/CD health and creates fix tasks
	RoleCICD Role = "cicd"
)

// String returns the string representation of a Role.
func (r Role) String() string {
	return string(r)
}

// IsSingleThreaded returns true if this role requires exclusive access.
func (r Role) IsSingleThreaded() bool {
	return r == RoleMerge || r == RoleDeploy
}

// ValidRoles returns all valid role values.
func ValidRoles() []Role {
	return []Role{RoleWorker, RolePlanner, RoleReviewer, RoleMerge, RoleDeploy, RoleGroomer, RoleMonitor, RoleTester, RolePM, RoleCICD}
}

// ParseRole parses a string into a Role.
func ParseRole(s string) (Role, error) {
	switch Role(s) {
	case RoleWorker, RolePlanner, RoleReviewer, RoleMerge, RoleDeploy, RoleGroomer, RoleMonitor, RoleTester, RolePM, RoleCICD:
		return Role(s), nil
	default:
		return "", fmt.Errorf("invalid role: %q", s)
	}
}

// Status represents the current state of a worker.
type Status string

const (
	// StatusIdle means the worker is available for assignment
	StatusIdle Status = "idle"
	// StatusActive means the worker is currently executing a task
	StatusActive Status = "active"
	// StatusPaused means the worker is suspended mid-task
	StatusPaused Status = "paused"
	// StatusStopped means the worker was stopped and needs reassignment
	StatusStopped Status = "stopped"
)

// String returns the string representation of a Status.
func (s Status) String() string {
	return string(s)
}

// IsAvailable returns true if the worker can accept a new task.
func (s Status) IsAvailable() bool {
	return s == StatusIdle || s == StatusStopped
}

// IsRunning returns true if the worker has an active session.
func (s Status) IsRunning() bool {
	return s == StatusActive || s == StatusPaused
}

// ValidStatuses returns all valid status values.
func ValidStatuses() []Status {
	return []Status{StatusIdle, StatusActive, StatusPaused, StatusStopped}
}

// ParseStatus parses a string into a Status.
func ParseStatus(s string) (Status, error) {
	switch Status(s) {
	case StatusIdle, StatusActive, StatusPaused, StatusStopped:
		return Status(s), nil
	default:
		return "", fmt.Errorf("invalid status: %q", s)
	}
}

// DisplayName returns the worker's display name (alias if set, otherwise name).
func (w *Worker) DisplayName() string {
	if w.Alias != "" {
		return w.Alias
	}
	return w.Name
}

// ShortID returns the first 8 characters of the worker ID (without "w-" prefix).
func (w *Worker) ShortID() string {
	if len(w.ID) >= 10 && w.ID[:2] == "w-" {
		// Skip "w-" prefix, take 8 chars
		return w.ID[2:10]
	}
	if len(w.ID) > 2 && w.ID[:2] == "w-" {
		return w.ID[2:]
	}
	return w.ID
}

// TmuxSessionName returns the expected tmux session name for this worker.
func (w *Worker) TmuxSessionName() string {
	return fmt.Sprintf("forge-%s-%s", w.Name, w.ShortID())
}

// GraphitiGroupID returns the Graphiti group ID for this worker's memories.
func (w *Worker) GraphitiGroupID() string {
	return fmt.Sprintf("forge-worker-%s", w.ShortID())
}

// StatusIcon returns an emoji icon for the worker status.
func (w *Worker) StatusIcon() string {
	switch w.Status {
	case StatusIdle:
		return "💤"
	case StatusActive:
		return "🔄"
	case StatusPaused:
		return "⏸️"
	case StatusStopped:
		return "🛑"
	default:
		return "❓"
	}
}

// RoleIcon returns an emoji icon for the worker role.
func (w *Worker) RoleIcon() string {
	switch w.Role {
	case RoleWorker:
		return "👷"
	case RolePlanner:
		return "📋"
	case RoleReviewer:
		return "🔍"
	case RoleMerge:
		return "🔀"
	case RoleDeploy:
		return "🚀"
	case RoleGroomer:
		return "🧹"
	case RoleMonitor:
		return "📡"
	case RoleTester:
		return "🧪"
	case RolePM:
		return "📊"
	default:
		return "❓"
	}
}

// Clone returns a deep copy of the worker.
func (w *Worker) Clone() *Worker {
	clone := *w
	return &clone
}

// Validate checks that the worker has valid required fields.
func (w *Worker) Validate() error {
	if w.ID == "" {
		return fmt.Errorf("worker ID is required")
	}
	if w.Name == "" {
		return fmt.Errorf("worker name is required")
	}
	if _, err := ParseRole(string(w.Role)); err != nil {
		return err
	}
	if _, err := ParseStatus(string(w.Status)); err != nil {
		return err
	}
	return nil
}
