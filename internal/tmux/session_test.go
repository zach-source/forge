package tmux

import (
	"testing"
)

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "'simple'"},
		{"with space", "'with space'"},
		{"with'quote", "'with'\"'\"'quote'"},
		{"", "''"},
	}

	for _, tt := range tests {
		got := shellQuote(tt.input)
		if got != tt.expected {
			t.Errorf("shellQuote(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestIsShellPrompt(t *testing.T) {
	tests := []struct {
		line     string
		expected bool
	}{
		{"user@host:~/dir$ ", true},
		{"$ ", true},
		{"# ", true},
		{"> ", true},
		{"% ", true},
		{"$", true},
		{"running command...", false},
		{"", false},
		{"   ", false},
	}

	for _, tt := range tests {
		got := isShellPrompt(tt.line)
		if got != tt.expected {
			t.Errorf("isShellPrompt(%q) = %v, want %v", tt.line, got, tt.expected)
		}
	}
}

func TestIsTmuxInstalled(t *testing.T) {
	// This is an environment-dependent test
	// Just verify it doesn't panic
	_ = IsTmuxInstalled()
}

func TestSessionExists(t *testing.T) {
	s := &Session{Name: "nonexistent-forge-test-session-12345"}
	if s.Exists() {
		t.Error("Expected nonexistent session to return false")
	}
}
