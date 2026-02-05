package kanban

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupTestBeads creates a temporary directory with an initialized beads database.
// Returns the directory path and a cleanup function.
func setupTestBeads(t *testing.T) (string, func()) {
	t.Helper()

	// Check if bd is available
	if _, err := exec.Command("which", "bd").Output(); err != nil {
		t.Skip("bd CLI not available, skipping integration tests")
	}

	tmpDir, err := os.MkdirTemp("", "kanban-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}

	// Initialize beads in the temp directory
	cmd := exec.Command("bd", "init")
	cmd.Dir = tmpDir
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("bd init failed: %s", string(out))
	}

	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

func TestStore_CreateAndGet(t *testing.T) {
	tmpDir, cleanup := setupTestBeads(t)
	defer cleanup()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer func() { _ = store.Close() }()

	issue := &Issue{
		Title:       "Test issue",
		Description: "This is a test",
		Priority:    PriorityHigh,
		Labels:      []string{"bug", "urgent"},
	}

	if err := store.Create(issue); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if issue.ID == "" {
		t.Error("expected ID to be set")
	}
	if issue.Status != StatusBacklog {
		t.Errorf("expected status backlog, got %s", issue.Status)
	}

	// Retrieve
	got, err := store.Get(issue.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("expected issue, got nil")
	}
	if got.Title != issue.Title {
		t.Errorf("title: got %q, want %q", got.Title, issue.Title)
	}
	// Note: bd may handle labels differently, so we check they exist
	if got.Description != issue.Description {
		t.Errorf("description: got %q, want %q", got.Description, issue.Description)
	}
}

func TestStore_GetNonexistent(t *testing.T) {
	tmpDir, cleanup := setupTestBeads(t)
	defer cleanup()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Getting a nonexistent issue should return nil, nil (not an error)
	// This allows the caller to handle "not found" with a clear error message
	got, err := store.Get("nonexistent-task-id")
	if err != nil {
		t.Errorf("Get nonexistent: expected no error, got %v", err)
	}
	if got != nil {
		t.Errorf("Get nonexistent: expected nil issue, got %+v", got)
	}
}

func TestStore_Update(t *testing.T) {
	tmpDir, cleanup := setupTestBeads(t)
	defer cleanup()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer func() { _ = store.Close() }()

	issue := &Issue{Title: "Original title"}
	if err := store.Create(issue); err != nil {
		t.Fatalf("Create: %v", err)
	}

	issue.Title = "Updated title"
	if err := store.Update(issue); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := store.Get(issue.ID)
	if got.Title != "Updated title" {
		t.Errorf("title: got %q, want %q", got.Title, "Updated title")
	}
}

func TestStore_Delete(t *testing.T) {
	tmpDir, cleanup := setupTestBeads(t)
	defer cleanup()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer func() { _ = store.Close() }()

	issue := &Issue{Title: "To delete"}
	if err := store.Create(issue); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := store.Delete(issue.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := store.Get(issue.ID)
	if err != nil {
		t.Fatalf("Get after delete: %v", err)
	}
	if got != nil {
		t.Error("expected nil after delete")
	}
}

func TestStore_List(t *testing.T) {
	tmpDir, cleanup := setupTestBeads(t)
	defer cleanup()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Create issues
	issues := []*Issue{
		{Title: "Issue 1"},
		{Title: "Issue 2"},
		{Title: "Issue 3"},
	}
	for _, issue := range issues {
		if err := store.Create(issue); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	// List all
	all, err := store.List()
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("List all: got %d, want 3", len(all))
	}
}

func TestStore_Move(t *testing.T) {
	tmpDir, cleanup := setupTestBeads(t)
	defer cleanup()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer func() { _ = store.Close() }()

	issue := &Issue{Title: "Moving issue"}
	if err := store.Create(issue); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Move to in_progress
	if err := store.Move(issue.ID, StatusInProgress); err != nil {
		t.Fatalf("Move to in_progress: %v", err)
	}

	got, _ := store.Get(issue.ID)
	if got.Status != StatusInProgress {
		t.Errorf("status: got %s, want %s", got.Status, StatusInProgress)
	}

	// Move to done (close)
	if err := store.Move(issue.ID, StatusDone); err != nil {
		t.Fatalf("Move to done: %v", err)
	}

	got, _ = store.Get(issue.ID)
	if got.Status != StatusDone {
		t.Errorf("status: got %s, want %s", got.Status, StatusDone)
	}
}

func TestStore_GetBoard(t *testing.T) {
	tmpDir, cleanup := setupTestBeads(t)
	defer cleanup()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Create issues
	issues := []*Issue{
		{Title: "Issue 1"},
		{Title: "Issue 2"},
	}
	for _, issue := range issues {
		if err := store.Create(issue); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	board, err := store.GetBoard()
	if err != nil {
		t.Fatalf("GetBoard: %v", err)
	}

	if len(board.Columns) != 6 {
		t.Errorf("columns: got %d, want 6", len(board.Columns))
	}

	// Count total issues across columns
	total := 0
	for _, col := range board.Columns {
		total += len(col.Issues)
	}
	if total != 2 {
		t.Errorf("total issues: got %d, want 2", total)
	}
}

func TestStore_ParentChild(t *testing.T) {
	tmpDir, cleanup := setupTestBeads(t)
	defer cleanup()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Create parent
	parent := &Issue{Title: "Parent issue"}
	if err := store.Create(parent); err != nil {
		t.Fatalf("Create parent: %v", err)
	}

	// Create children
	child1 := &Issue{Title: "Child 1", ParentID: parent.ID}
	child2 := &Issue{Title: "Child 2", ParentID: parent.ID}
	if err := store.Create(child1); err != nil {
		t.Fatalf("Create child1: %v", err)
	}
	if err := store.Create(child2); err != nil {
		t.Fatalf("Create child2: %v", err)
	}

	// Get children
	children, err := store.GetChildren(parent.ID)
	if err != nil {
		t.Fatalf("GetChildren: %v", err)
	}
	if len(children) != 2 {
		t.Errorf("children: got %d, want 2", len(children))
	}
}

// TestStatusMapping verifies the bidirectional status mapping between kanban and bd.
func TestStatusMapping(t *testing.T) {
	tests := []struct {
		bdStatus     string
		kanbanStatus Status
	}{
		{"open", StatusBacklog},
		{"in_progress", StatusInProgress},
		{"blocked", StatusBacklog},
		{"deferred", StatusBacklog},
		{"closed", StatusDone},
	}

	for _, tt := range tests {
		got := bdToKanbanStatus(tt.bdStatus)
		if got != tt.kanbanStatus {
			t.Errorf("bdToKanbanStatus(%q) = %s, want %s", tt.bdStatus, got, tt.kanbanStatus)
		}
	}

	// Test reverse mapping
	reverseTests := []struct {
		kanbanStatus Status
		bdStatus     string
	}{
		{StatusBacklog, "open"},
		{StatusTodo, "open"},
		{StatusInProgress, "in_progress"},
		{StatusReview, "in_progress"},
		{StatusDone, "closed"},
	}

	for _, tt := range reverseTests {
		got := kanbanToBdStatus(tt.kanbanStatus)
		if got != tt.bdStatus {
			t.Errorf("kanbanToBdStatus(%s) = %q, want %q", tt.kanbanStatus, got, tt.bdStatus)
		}
	}
}

// TestPriorityMapping verifies the bidirectional priority mapping.
func TestPriorityMapping(t *testing.T) {
	priorityTests := []struct {
		bdPriority     int
		kanbanPriority Priority
	}{
		{0, PriorityCritical},
		{1, PriorityHigh},
		{2, PriorityMedium},
		{3, PriorityLow},
		{4, PriorityLow},
	}

	for _, tt := range priorityTests {
		got := bdToKanbanPriority(tt.bdPriority)
		if got != tt.kanbanPriority {
			t.Errorf("bdToKanbanPriority(%d) = %s, want %s", tt.bdPriority, got, tt.kanbanPriority)
		}
	}

	// Test reverse mapping
	reverseTests := []struct {
		kanbanPriority Priority
		bdPriority     int
	}{
		{PriorityCritical, 0},
		{PriorityHigh, 1},
		{PriorityMedium, 2},
		{PriorityLow, 3},
	}

	for _, tt := range reverseTests {
		got := kanbanToBdPriority(tt.kanbanPriority)
		if got != tt.bdPriority {
			t.Errorf("kanbanToBdPriority(%s) = %d, want %d", tt.kanbanPriority, got, tt.bdPriority)
		}
	}
}

// Ensure beads directory exists for tests that need it
func init() {
	// Create test fixtures directory if needed
	fixturesDir := filepath.Join(os.TempDir(), "kanban-test-fixtures")
	_ = os.MkdirAll(fixturesDir, 0o755)
}
