package kanban

import (
	"testing"
)

func TestStore_CreateAndGet(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

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
	if len(got.Labels) != 2 {
		t.Errorf("labels: got %v, want 2 items", got.Labels)
	}
}

func TestStore_Update(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	issue := &Issue{Title: "Original title"}
	if err := store.Create(issue); err != nil {
		t.Fatalf("Create: %v", err)
	}

	issue.Title = "Updated title"
	issue.Status = StatusInProgress
	if err := store.Update(issue); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := store.Get(issue.ID)
	if got.Title != "Updated title" {
		t.Errorf("title: got %q, want %q", got.Title, "Updated title")
	}
	if got.Status != StatusInProgress {
		t.Errorf("status: got %s, want %s", got.Status, StatusInProgress)
	}
}

func TestStore_Delete(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

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
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	// Create issues with different statuses
	issues := []*Issue{
		{Title: "Backlog 1", Status: StatusBacklog},
		{Title: "Backlog 2", Status: StatusBacklog},
		{Title: "In Progress", Status: StatusInProgress},
		{Title: "Done", Status: StatusDone},
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
	if len(all) != 4 {
		t.Errorf("List all: got %d, want 4", len(all))
	}

	// List by status
	backlog, err := store.List(StatusBacklog)
	if err != nil {
		t.Fatalf("List backlog: %v", err)
	}
	if len(backlog) != 2 {
		t.Errorf("List backlog: got %d, want 2", len(backlog))
	}

	// List multiple statuses
	active, err := store.List(StatusBacklog, StatusInProgress)
	if err != nil {
		t.Fatalf("List active: %v", err)
	}
	if len(active) != 3 {
		t.Errorf("List active: got %d, want 3", len(active))
	}
}

func TestStore_Move(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	issue := &Issue{Title: "Moving issue"}
	if err := store.Create(issue); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := store.Move(issue.ID, StatusInProgress); err != nil {
		t.Fatalf("Move: %v", err)
	}

	got, _ := store.Get(issue.ID)
	if got.Status != StatusInProgress {
		t.Errorf("status: got %s, want %s", got.Status, StatusInProgress)
	}
}

func TestStore_GetBoard(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	// Create issues in different columns
	issues := []*Issue{
		{Title: "Backlog 1", Status: StatusBacklog},
		{Title: "Todo 1", Status: StatusTodo},
		{Title: "In Progress 1", Status: StatusInProgress},
		{Title: "In Progress 2", Status: StatusInProgress},
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

	if len(board.Columns) != 5 {
		t.Errorf("columns: got %d, want 5", len(board.Columns))
	}

	// Check column order and contents
	expectedCounts := map[Status]int{
		StatusBacklog:    1,
		StatusTodo:       1,
		StatusInProgress: 2,
		StatusReview:     0,
		StatusDone:       0,
	}
	for _, col := range board.Columns {
		want := expectedCounts[col.Status]
		if len(col.Issues) != want {
			t.Errorf("column %s: got %d issues, want %d", col.Status, len(col.Issues), want)
		}
	}
}

func TestStore_ParentChild(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

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
