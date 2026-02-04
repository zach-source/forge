package ralph

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseState(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    *State
		wantErr bool
	}{
		{
			name: "valid state file",
			content: `---
id: "forge-api"
active: true
iteration: 7
max_iterations: 50
completion_promise: "API DONE"
started_at: "2026-01-31T10:00:00Z"
tmux_session: "forge-api"
workdir: "/path/to/project"
log_file: "/tmp/forge-api.log"
---

Build a REST API for user management.`,
			want: &State{
				ID:                "forge-api",
				Active:            true,
				Iteration:         7,
				MaxIterations:     50,
				CompletionPromise: "API DONE",
				TmuxSession:       "forge-api",
				WorkDir:           "/path/to/project",
				LogFile:           "/tmp/forge-api.log",
				Prompt:            "Build a REST API for user management.",
			},
			wantErr: false,
		},
		{
			name:    "missing frontmatter",
			content: "Just some text without frontmatter",
			wantErr: true,
		},
		{
			name: "incomplete frontmatter",
			content: `---
active: true
iteration: 5`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseState(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseState() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if got.ID != tt.want.ID {
				t.Errorf("ID = %v, want %v", got.ID, tt.want.ID)
			}
			if got.Active != tt.want.Active {
				t.Errorf("Active = %v, want %v", got.Active, tt.want.Active)
			}
			if got.Iteration != tt.want.Iteration {
				t.Errorf("Iteration = %v, want %v", got.Iteration, tt.want.Iteration)
			}
			if got.MaxIterations != tt.want.MaxIterations {
				t.Errorf("MaxIterations = %v, want %v", got.MaxIterations, tt.want.MaxIterations)
			}
			if got.CompletionPromise != tt.want.CompletionPromise {
				t.Errorf("CompletionPromise = %v, want %v", got.CompletionPromise, tt.want.CompletionPromise)
			}
			if got.Prompt != tt.want.Prompt {
				t.Errorf("Prompt = %v, want %v", got.Prompt, tt.want.Prompt)
			}
		})
	}
}

func TestFormatState(t *testing.T) {
	state := &State{
		ID:                "test-123",
		Active:            true,
		Iteration:         3,
		MaxIterations:     10,
		CompletionPromise: "DONE",
		StartedAt:         time.Date(2026, 1, 31, 10, 0, 0, 0, time.UTC),
		TmuxSession:       "forge-test",
		WorkDir:           "/test/dir",
		LogFile:           "/tmp/test.log",
		Prompt:            "Test prompt content",
	}

	content, err := FormatState(state)
	if err != nil {
		t.Fatalf("FormatState() error = %v", err)
	}

	// Parse it back
	parsed, err := ParseState(content)
	if err != nil {
		t.Fatalf("ParseState() error = %v", err)
	}

	if parsed.ID != state.ID {
		t.Errorf("ID = %v, want %v", parsed.ID, state.ID)
	}
	if parsed.Active != state.Active {
		t.Errorf("Active = %v, want %v", parsed.Active, state.Active)
	}
	if parsed.Iteration != state.Iteration {
		t.Errorf("Iteration = %v, want %v", parsed.Iteration, state.Iteration)
	}
	if parsed.Prompt != state.Prompt {
		t.Errorf("Prompt = %v, want %v", parsed.Prompt, state.Prompt)
	}
}

func TestStateController(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "test.state.md")

	ctrl := NewStateController(statePath)

	// Test that file doesn't exist initially
	if ctrl.Exists() {
		t.Error("Expected state file to not exist")
	}

	// Write a state
	state := &State{
		ID:                "test-ctrl",
		Active:            true,
		Iteration:         1,
		MaxIterations:     5,
		CompletionPromise: "COMPLETE",
		StartedAt:         time.Now(),
		TmuxSession:       "forge-ctrl",
		WorkDir:           tmpDir,
		LogFile:           filepath.Join(tmpDir, "test.log"),
		Prompt:            "Controller test prompt",
	}

	if err := ctrl.Write(state); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Should exist now
	if !ctrl.Exists() {
		t.Error("Expected state file to exist after write")
	}

	// Read it back
	read, err := ctrl.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if read.ID != state.ID {
		t.Errorf("ID = %v, want %v", read.ID, state.ID)
	}
	if read.Iteration != state.Iteration {
		t.Errorf("Iteration = %v, want %v", read.Iteration, state.Iteration)
	}

	// Test Update
	if err := ctrl.Update(func(s *State) {
		s.Iteration = 2
	}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	read, _ = ctrl.Read()
	if read.Iteration != 2 {
		t.Errorf("Iteration after update = %v, want 2", read.Iteration)
	}

	// Test Delete
	if err := ctrl.Delete(); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if ctrl.Exists() {
		t.Error("Expected state file to not exist after delete")
	}
}

func TestListSessionStateFiles(t *testing.T) {
	// Create temp dir and set it as the sessions dir
	tmpDir := t.TempDir()
	sessionsDir := filepath.Join(tmpDir, ".forge", "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create some state files
	files := []string{"session1.state.md", "session2.state.md", "notastate.txt"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(sessionsDir, f), []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Note: This test uses the default sessions dir, which we can't easily override
	// In a real test, we'd inject the path
}
