package worker

import (
	"strings"
	"testing"
)

func TestWorkerIdentityPrompt(t *testing.T) {
	w := &Worker{
		ID:       "w-a1b2c3d4",
		Name:     "alpha",
		Alias:    "api-dev",
		Role:     RoleWorker,
		Worktree: "/path/to/worktree",
	}

	prompt := WorkerIdentityPrompt(w, "task-123")

	// Check identity section
	if !strings.Contains(prompt, "**api-dev**") {
		t.Errorf("Prompt should contain display name (alias)")
	}
	if !strings.Contains(prompt, "a1b2c3d4") {
		t.Errorf("Prompt should contain short ID")
	}
	if !strings.Contains(prompt, "task-123") {
		t.Errorf("Prompt should contain task ID")
	}
	if !strings.Contains(prompt, "/path/to/worktree") {
		t.Errorf("Prompt should contain worktree")
	}

	// Check memory protocol
	if !strings.Contains(prompt, "forge-worker-a1b2c3d4") {
		t.Errorf("Prompt should contain Graphiti group ID")
	}
	if !strings.Contains(prompt, "worker:alpha") {
		t.Errorf("Prompt should contain worker tag")
	}

	// Check completion
	if !strings.Contains(prompt, "WORKER_ALPHA_COMPLETE") {
		t.Errorf("Prompt should contain completion promise")
	}
}

func TestWorkerPromise(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"alpha", "WORKER_ALPHA_COMPLETE"},
		{"bravo", "WORKER_BRAVO_COMPLETE"},
		{"alpha2", "WORKER_ALPHA2_COMPLETE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &Worker{Name: tt.name}
			got := WorkerPromise(w)
			if got != tt.want {
				t.Errorf("WorkerPromise() = %q, want %q", got, tt.want)
			}
		})
	}
}
