package worker

import (
	"strings"
	"testing"

	"github.com/zach-source/forge/internal/context"
)

func TestWorkerIdentityPrompt(t *testing.T) {
	w := &Worker{
		ID:       "w-a1b2c3d4",
		Name:     "alpha",
		Alias:    "api-dev",
		Role:     RoleWorker,
		Worktree: "/path/to/worktree",
	}

	prompt := WorkerIdentityPrompt(w, "task-123", "")

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

func TestWorkingGuidelines_IncrementalDevelopment(t *testing.T) {
	w := &Worker{
		Name: "alpha",
		Role: RoleWorker,
	}

	guidelines := WorkingGuidelines(w)

	expectedSubstrings := []string{
		"### Incremental Development",
		"draft PR",
		"gh pr create --draft",
		"gh pr ready",
		"Regular commits",
		"### Quality Standards",
	}

	for _, expected := range expectedSubstrings {
		if !strings.Contains(guidelines, expected) {
			t.Errorf("WorkingGuidelines() missing %q", expected)
		}
	}
}

func TestWorkingGuidelines_ContainsAllSections(t *testing.T) {
	w := &Worker{
		Name: "bravo",
		Role: RoleWorker,
	}

	guidelines := WorkingGuidelines(w)

	sections := []string{
		"Available Tools",
		"Incremental Development",
		"Quality Standards",
		"If Blocked",
		"Before Completing",
	}

	for _, section := range sections {
		if !strings.Contains(guidelines, section) {
			t.Errorf("WorkingGuidelines() missing section %q", section)
		}
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

func TestTaskPromptWithLearnings(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a learnings store with test data
	store := context.NewStore()
	store.Add(context.Learning{
		Summary:  "Worker registry deadlock fix",
		Problem:  "Concurrent access caused deadlock",
		Solution: "Use RWMutex instead of Mutex",
		Keywords: []string{"worker", "registry", "deadlock"},
		Files:    []string{"internal/worker/registry.go"},
	})
	if err := store.Save(tmpDir); err != nil {
		t.Fatalf("Failed to save test learnings: %v", err)
	}

	w := &Worker{
		ID:   "w-test123",
		Name: "alpha",
		Role: RoleWorker,
	}

	opts := TaskPromptOptions{
		WorkspaceDir: tmpDir,
		TaskID:       "task-456",
		Title:        "Fix worker registry bug",
		Description:  "There's a bug in the worker registry causing issues",
	}

	prompt := TaskPromptWithLearnings(w, opts)

	// Should contain identity
	if !strings.Contains(prompt, "**alpha**") {
		t.Error("Prompt should contain worker name")
	}

	// Should contain task
	if !strings.Contains(prompt, "task-456") {
		t.Error("Prompt should contain task ID")
	}

	// Should contain learnings section
	if !strings.Contains(prompt, "Relevant Learnings") {
		t.Error("Prompt should contain relevant learnings section")
	}

	// Should contain the relevant learning
	if !strings.Contains(prompt, "Worker registry deadlock fix") {
		t.Error("Prompt should contain the relevant learning about registry")
	}

	// Should contain guidelines
	if !strings.Contains(prompt, "Working Guidelines") {
		t.Error("Prompt should contain working guidelines")
	}
}

func TestTaskPromptWithLearningsNoMatch(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a learnings store with unrelated data
	store := context.NewStore()
	store.Add(context.Learning{
		Summary:  "Tmux session cleanup",
		Problem:  "Sessions not cleaned up",
		Solution: "Add defer cleanup",
		Keywords: []string{"tmux", "session", "cleanup"},
	})
	if err := store.Save(tmpDir); err != nil {
		t.Fatalf("Failed to save test learnings: %v", err)
	}

	w := &Worker{
		ID:   "w-test123",
		Name: "bravo",
		Role: RoleWorker,
	}

	opts := TaskPromptOptions{
		WorkspaceDir: tmpDir,
		TaskID:       "task-789",
		Title:        "Update pricing page",
		Description:  "Change subscription rates",
	}

	prompt := TaskPromptWithLearnings(w, opts)

	// Should NOT contain learnings section (no match)
	if strings.Contains(prompt, "Relevant Learnings") {
		t.Error("Prompt should not contain learnings section when no matches")
	}

	// Should still contain identity and guidelines
	if !strings.Contains(prompt, "**bravo**") {
		t.Error("Prompt should contain worker name")
	}
	if !strings.Contains(prompt, "Working Guidelines") {
		t.Error("Prompt should contain working guidelines")
	}
}

func TestTaskPromptWithLearningsNoWorkspace(t *testing.T) {
	w := &Worker{
		ID:   "w-test123",
		Name: "charlie",
		Role: RoleWorker,
	}

	opts := TaskPromptOptions{
		WorkspaceDir: "", // No workspace
		TaskID:       "task-000",
		Title:        "Some task",
		Description:  "Some description",
	}

	prompt := TaskPromptWithLearnings(w, opts)

	// Should NOT contain learnings section (no workspace)
	if strings.Contains(prompt, "Relevant Learnings") {
		t.Error("Prompt should not contain learnings section when no workspace")
	}

	// Should still contain basic prompt
	if !strings.Contains(prompt, "**charlie**") {
		t.Error("Prompt should contain worker name")
	}
}
