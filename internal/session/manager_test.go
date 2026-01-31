package session

import (
	"testing"
	"time"
)

func TestGenerateSessionID(t *testing.T) {
	tests := []struct {
		prompt   string
		contains string
	}{
		{"Build a REST API", "build-a"},
		{"Fix the bug", "fix-the"},
		{"", "forge-"},
	}

	for _, tt := range tests {
		id := GenerateSessionID(tt.prompt)
		if tt.contains != "" && len(id) == 0 {
			t.Errorf("GenerateSessionID(%q) returned empty", tt.prompt)
		}
		// IDs should be unique
		id2 := GenerateSessionID(tt.prompt + "different")
		if id == id2 {
			t.Errorf("Different prompts generated same ID: %s", id)
		}
	}
}

func TestSessionElapsedString(t *testing.T) {
	tests := []struct {
		ago  time.Duration
		want string
	}{
		{30 * time.Second, "just now"},
		{5 * time.Minute, "5m ago"},
		{2 * time.Hour, "2h ago"},
		{48 * time.Hour, "2d ago"},
	}

	for _, tt := range tests {
		s := &Session{
			StartedAt: time.Now().Add(-tt.ago),
		}
		got := s.ElapsedString()
		if got != tt.want {
			t.Errorf("ElapsedString() for %v ago = %q, want %q", tt.ago, got, tt.want)
		}
	}
}

func TestSessionStatusIcon(t *testing.T) {
	tests := []struct {
		status Status
		icon   string
	}{
		{StatusActive, "🔄"},
		{StatusCompleted, "✅"},
		{StatusCancelled, "❌"},
		{StatusError, "⚠️"},
		{StatusPaused, "⏸️"},
	}

	for _, tt := range tests {
		s := &Session{Status: tt.status}
		got := s.StatusIcon()
		if got != tt.icon {
			t.Errorf("StatusIcon() for %v = %q, want %q", tt.status, got, tt.icon)
		}
	}
}

func TestManagerBasics(t *testing.T) {
	m := NewManager()

	// Add a session
	s := &Session{
		ID:     "test-1",
		Status: StatusActive,
	}
	m.Add(s)

	// Get it back
	got := m.Get("test-1")
	if got == nil {
		t.Fatal("Expected to get session back")
	}
	if got.ID != "test-1" {
		t.Errorf("ID = %q, want %q", got.ID, "test-1")
	}

	// List should have it
	list := m.List()
	if len(list) != 1 {
		t.Errorf("List() len = %d, want 1", len(list))
	}

	// Active count
	if m.ActiveCount() != 1 {
		t.Errorf("ActiveCount() = %d, want 1", m.ActiveCount())
	}

	// Remove it
	m.Remove("test-1")
	if m.Get("test-1") != nil {
		t.Error("Expected session to be removed")
	}
}
