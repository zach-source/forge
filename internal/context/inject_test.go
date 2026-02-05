package context

import (
	"strings"
	"testing"
)

func TestInjectLearningsEmpty(t *testing.T) {
	result := InjectLearnings(nil, DefaultInjectOptions())
	if result != "" {
		t.Errorf("Expected empty string for nil learnings, got %q", result)
	}

	result = InjectLearnings([]Learning{}, DefaultInjectOptions())
	if result != "" {
		t.Errorf("Expected empty string for empty learnings, got %q", result)
	}
}

func TestInjectLearningsBasic(t *testing.T) {
	learnings := []Learning{
		{
			Summary:  "Test learning one",
			Problem:  "First problem",
			Solution: "First solution",
			Files:    []string{"file1.go"},
		},
	}

	result := InjectLearnings(learnings, DefaultInjectOptions())

	if !strings.Contains(result, "## Relevant Learnings") {
		t.Error("Result should contain section header")
	}
	if !strings.Contains(result, "Test learning one") {
		t.Error("Result should contain summary")
	}
	if !strings.Contains(result, "First problem") {
		t.Error("Result should contain problem")
	}
	if !strings.Contains(result, "First solution") {
		t.Error("Result should contain solution")
	}
	if !strings.Contains(result, "file1.go") {
		t.Error("Result should contain files")
	}
}

func TestInjectLearningsLimit(t *testing.T) {
	learnings := []Learning{
		{Summary: "Learning 1"},
		{Summary: "Learning 2"},
		{Summary: "Learning 3"},
		{Summary: "Learning 4"},
		{Summary: "Learning 5"},
	}

	opts := InjectOptions{
		MaxLearnings: 2,
		Verbose:      true,
	}

	result := InjectLearnings(learnings, opts)

	if strings.Contains(result, "Learning 3") {
		t.Error("Result should not contain learning beyond limit")
	}
	if !strings.Contains(result, "Learning 1") || !strings.Contains(result, "Learning 2") {
		t.Error("Result should contain learnings within limit")
	}
}

func TestInjectLearningsVerboseOption(t *testing.T) {
	learnings := []Learning{
		{
			Summary:  "Test",
			Problem:  "Test problem",
			Solution: "Test solution",
		},
	}

	// Verbose on
	result := InjectLearnings(learnings, InjectOptions{MaxLearnings: 3, Verbose: true})
	if !strings.Contains(result, "**Problem**") {
		t.Error("Verbose should include problem")
	}

	// Verbose off
	result = InjectLearnings(learnings, InjectOptions{MaxLearnings: 3, Verbose: false})
	if strings.Contains(result, "**Problem**") {
		t.Error("Non-verbose should not include problem")
	}
}

func TestInjectLearningsFilesOption(t *testing.T) {
	learnings := []Learning{
		{
			Summary: "Test",
			Files:   []string{"file1.go", "file2.go"},
		},
	}

	// Files on
	result := InjectLearnings(learnings, InjectOptions{MaxLearnings: 3, IncludeFiles: true})
	if !strings.Contains(result, "file1.go") {
		t.Error("IncludeFiles should show files")
	}

	// Files off
	result = InjectLearnings(learnings, InjectOptions{MaxLearnings: 3, IncludeFiles: false})
	if strings.Contains(result, "file1.go") {
		t.Error("Without IncludeFiles should not show files")
	}
}

func TestFormatLearningForPrompt(t *testing.T) {
	learning := Learning{
		Summary:  "Test learning",
		Problem:  "Test problem",
		Solution: "Test solution",
		Files:    []string{"test.go"},
	}

	result := FormatLearningForPrompt(learning)

	if !strings.Contains(result, "**Test learning**") {
		t.Error("Should contain formatted summary")
	}
	if !strings.Contains(result, "Problem: Test problem") {
		t.Error("Should contain problem")
	}
	if !strings.Contains(result, "Solution: Test solution") {
		t.Error("Should contain solution")
	}
	if !strings.Contains(result, "Files: test.go") {
		t.Error("Should contain files")
	}
}

func TestBuildContextSection(t *testing.T) {
	tmpDir := t.TempDir()

	// Create store with test data
	store := NewStore()
	store.Add(Learning{
		Summary:  "Worker test fix",
		Problem:  "Tests failed",
		Solution: "Fixed mock setup",
		Keywords: []string{"worker", "test", "mock"},
	})
	if err := store.Save(tmpDir); err != nil {
		t.Fatalf("Failed to save store: %v", err)
	}

	// Test matching task
	result := BuildContextSection(tmpDir, "Worker test improvements", "Update worker tests")
	if result == "" {
		t.Error("Should return context for matching task")
	}
	if !strings.Contains(result, "Worker test fix") {
		t.Error("Should contain relevant learning")
	}

	// Test non-matching task
	result = BuildContextSection(tmpDir, "Documentation update", "Update README")
	if result != "" {
		t.Errorf("Should return empty for non-matching task, got %q", result)
	}
}

func TestBuildContextSectionNoStore(t *testing.T) {
	tmpDir := t.TempDir()

	// No store file exists
	result := BuildContextSection(tmpDir, "Any task", "Description")
	if result != "" {
		t.Error("Should return empty when no store exists")
	}
}

func TestDefaultInjectOptions(t *testing.T) {
	opts := DefaultInjectOptions()

	if opts.MaxLearnings != 3 {
		t.Errorf("Default MaxLearnings should be 3, got %d", opts.MaxLearnings)
	}
	if !opts.IncludeFiles {
		t.Error("Default IncludeFiles should be true")
	}
	if !opts.Verbose {
		t.Error("Default Verbose should be true")
	}
}
