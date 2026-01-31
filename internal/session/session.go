// Package session provides multi-session management for forge agents.
package session

import (
	"time"

	"github.com/zach-source/forge/internal/ralph"
)

// Status represents the current status of a session.
type Status string

const (
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
	StatusCancelled Status = "cancelled"
	StatusError     Status = "error"
	StatusPaused    Status = "paused"
)

// Session represents a forge agent session.
type Session struct {
	ID        string       // Unique session identifier (e.g., "forge-api")
	State     *ralph.State // Current loop state
	Tmux      string       // Tmux session name
	WorkDir   string       // Project directory
	LogFile   string       // Output log path
	Status    Status       // Current status
	StartedAt time.Time    // When session started
	Error     error        // Last error if status is error
}

// ElapsedTime returns how long the session has been running.
func (s *Session) ElapsedTime() time.Duration {
	if s.StartedAt.IsZero() {
		return 0
	}
	return time.Since(s.StartedAt)
}

// ElapsedString returns a human-readable elapsed time.
func (s *Session) ElapsedString() string {
	elapsed := s.ElapsedTime()

	if elapsed < time.Minute {
		return "just now"
	}
	if elapsed < time.Hour {
		mins := int(elapsed.Minutes())
		if mins == 1 {
			return "1m ago"
		}
		return formatDuration(mins, "m") + " ago"
	}
	if elapsed < 24*time.Hour {
		hours := int(elapsed.Hours())
		if hours == 1 {
			return "1h ago"
		}
		return formatDuration(hours, "h") + " ago"
	}

	days := int(elapsed.Hours() / 24)
	if days == 1 {
		return "1d ago"
	}
	return formatDuration(days, "d") + " ago"
}

func formatDuration(n int, unit string) string {
	return string(rune('0'+n%10)) + unit
}

// StatusIcon returns an emoji icon for the session status.
func (s *Session) StatusIcon() string {
	switch s.Status {
	case StatusActive:
		return "🔄"
	case StatusCompleted:
		return "✅"
	case StatusCancelled:
		return "❌"
	case StatusError:
		return "⚠️"
	case StatusPaused:
		return "⏸️"
	default:
		return "❓"
	}
}

// IterationString returns a formatted iteration string like "7/50".
func (s *Session) IterationString() string {
	if s.State == nil {
		return "-/-"
	}
	if s.State.MaxIterations == 0 {
		return formatInt(s.State.Iteration) + "/∞"
	}
	return formatInt(s.State.Iteration) + "/" + formatInt(s.State.MaxIterations)
}

func formatInt(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	// Simple conversion for small numbers
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}
