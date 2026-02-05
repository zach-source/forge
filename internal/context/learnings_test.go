package context

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewStore(t *testing.T) {
	store := NewStore()
	if store == nil {
		t.Fatal("NewStore() returned nil")
	}
	if len(store.Learnings) != 0 {
		t.Errorf("NewStore() should have empty learnings, got %d", len(store.Learnings))
	}
}

func TestStoreAdd(t *testing.T) {
	store := NewStore()

	learning := Learning{
		Summary:  "Test learning",
		Problem:  "Test problem",
		Solution: "Test solution",
		Keywords: []string{"test", "learning"},
	}

	store.Add(learning)

	if len(store.Learnings) != 1 {
		t.Fatalf("Expected 1 learning, got %d", len(store.Learnings))
	}

	if store.Learnings[0].ID == "" {
		t.Error("Learning ID should be auto-generated")
	}
	if store.Learnings[0].CreatedAt.IsZero() {
		t.Error("Learning CreatedAt should be auto-set")
	}
}

func TestStoreAddMaxSize(t *testing.T) {
	store := NewStore()

	// Add more than MaxLearnings
	for i := 0; i < MaxLearnings+10; i++ {
		store.Add(Learning{
			Summary: "Test learning",
		})
	}

	if len(store.Learnings) != MaxLearnings {
		t.Errorf("Store should have max %d learnings, got %d", MaxLearnings, len(store.Learnings))
	}
}

func TestStoreAddNewestFirst(t *testing.T) {
	store := NewStore()

	store.Add(Learning{Summary: "First"})
	time.Sleep(10 * time.Millisecond)
	store.Add(Learning{Summary: "Second"})

	if store.Learnings[0].Summary != "Second" {
		t.Error("Newest learning should be first")
	}
	if store.Learnings[1].Summary != "First" {
		t.Error("Older learning should be second")
	}
}

func TestStoreGetByID(t *testing.T) {
	store := NewStore()
	store.Add(Learning{ID: "test-123", Summary: "Test"})

	found := store.GetByID("test-123")
	if found == nil {
		t.Fatal("GetByID should find existing learning")
	}
	if found.Summary != "Test" {
		t.Errorf("Expected summary 'Test', got '%s'", found.Summary)
	}

	notFound := store.GetByID("nonexistent")
	if notFound != nil {
		t.Error("GetByID should return nil for nonexistent ID")
	}
}

func TestStoreRemove(t *testing.T) {
	store := NewStore()
	store.Add(Learning{ID: "test-123", Summary: "Test"})

	removed := store.Remove("test-123")
	if !removed {
		t.Error("Remove should return true for existing learning")
	}
	if len(store.Learnings) != 0 {
		t.Error("Learning should be removed")
	}

	removed = store.Remove("nonexistent")
	if removed {
		t.Error("Remove should return false for nonexistent ID")
	}
}

func TestStoreSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()

	store := NewStore()
	store.Add(Learning{
		ID:       "test-123",
		Summary:  "Test learning",
		Problem:  "Test problem",
		Solution: "Test solution",
		Keywords: []string{"test", "save"},
		Files:    []string{"file.go"},
	})

	// Save
	err := store.Save(tmpDir)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	path := LearningsPath(tmpDir)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("Learnings file should exist after save")
	}

	// Load
	loaded, err := LoadStore(tmpDir)
	if err != nil {
		t.Fatalf("LoadStore failed: %v", err)
	}

	if len(loaded.Learnings) != 1 {
		t.Fatalf("Expected 1 learning, got %d", len(loaded.Learnings))
	}
	if loaded.Learnings[0].ID != "test-123" {
		t.Errorf("Expected ID 'test-123', got '%s'", loaded.Learnings[0].ID)
	}
	if loaded.Learnings[0].Summary != "Test learning" {
		t.Errorf("Expected summary 'Test learning', got '%s'", loaded.Learnings[0].Summary)
	}
}

func TestLoadStoreNonexistent(t *testing.T) {
	tmpDir := t.TempDir()

	// Loading from nonexistent path should return empty store
	store, err := LoadStore(tmpDir)
	if err != nil {
		t.Fatalf("LoadStore should not error for nonexistent file: %v", err)
	}
	if store == nil {
		t.Fatal("LoadStore should return empty store")
	}
	if len(store.Learnings) != 0 {
		t.Error("Store should be empty")
	}
}

func TestLearningsPath(t *testing.T) {
	path := LearningsPath("/workspace")
	expected := filepath.Join("/workspace", ".forge", "context", "learnings.json")
	if path != expected {
		t.Errorf("LearningsPath = %q, want %q", path, expected)
	}
}
