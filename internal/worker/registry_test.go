package worker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistry(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "forge-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	regPath := filepath.Join(tmpDir, "registry.yaml")

	// Load empty registry
	reg, err := LoadRegistryFrom(regPath)
	if err != nil {
		t.Fatalf("LoadRegistryFrom() error = %v", err)
	}

	if reg.Count() != 0 {
		t.Errorf("Empty registry count = %d, want 0", reg.Count())
	}

	// Create workers
	w1, err := reg.Create(RoleWorker, "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if w1.Name != "alpha" {
		t.Errorf("First worker name = %q, want %q", w1.Name, "alpha")
	}

	w2, err := reg.Create(RoleWorker, "api-dev")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if w2.Name != "bravo" {
		t.Errorf("Second worker name = %q, want %q", w2.Name, "bravo")
	}
	if w2.Alias != "api-dev" {
		t.Errorf("Second worker alias = %q, want %q", w2.Alias, "api-dev")
	}

	// Test Get
	got := reg.Get("alpha")
	if got == nil || got.ID != w1.ID {
		t.Errorf("Get(alpha) failed")
	}

	got = reg.Get(w1.ID)
	if got == nil || got.Name != "alpha" {
		t.Errorf("Get(ID) failed")
	}

	got = reg.Get("api-dev")
	if got == nil || got.ID != w2.ID {
		t.Errorf("Get(alias) failed")
	}

	// Test List
	workers := reg.List()
	if len(workers) != 2 {
		t.Errorf("List() length = %d, want 2", len(workers))
	}

	// Test Update
	err = reg.Update("alpha", func(w *Worker) {
		w.Status = StatusActive
		w.CurrentTask = "test-task"
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got = reg.Get("alpha")
	if got.Status != StatusActive {
		t.Errorf("After Update, Status = %q, want %q", got.Status, StatusActive)
	}
	if got.CurrentTask != "test-task" {
		t.Errorf("After Update, CurrentTask = %q, want %q", got.CurrentTask, "test-task")
	}

	// Test List with filter
	active := reg.List(StatusActive)
	if len(active) != 1 {
		t.Errorf("List(StatusActive) length = %d, want 1", len(active))
	}

	idle := reg.List(StatusIdle)
	if len(idle) != 1 {
		t.Errorf("List(StatusIdle) length = %d, want 1", len(idle))
	}

	// Test FindAvailable
	avail := reg.FindAvailable(RoleWorker)
	if avail == nil || avail.Name != "bravo" {
		t.Errorf("FindAvailable() should find bravo")
	}

	// Test Delete (should fail for active)
	err = reg.Delete("alpha")
	if err == nil {
		t.Errorf("Delete() should fail for active worker")
	}

	// Reset and delete
	reg.Update("alpha", func(w *Worker) {
		w.Status = StatusIdle
	})
	err = reg.Delete("alpha")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if reg.Count() != 1 {
		t.Errorf("After delete, count = %d, want 1", reg.Count())
	}

	// Test persistence
	reg2, err := LoadRegistryFrom(regPath)
	if err != nil {
		t.Fatalf("LoadRegistryFrom() after save error = %v", err)
	}

	if reg2.Count() != 1 {
		t.Errorf("Reloaded registry count = %d, want 1", reg2.Count())
	}

	got = reg2.Get("bravo")
	if got == nil || got.Alias != "api-dev" {
		t.Errorf("Reloaded registry missing bravo")
	}
}

func TestRegistryFindByWorktree(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "forge-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	reg, _ := LoadRegistryFrom(filepath.Join(tmpDir, "registry.yaml"))

	w, _ := reg.Create(RoleWorker, "")
	reg.Update(w.ID, func(w *Worker) {
		w.Worktree = "/path/to/worktree"
	})

	found := reg.FindByWorktree("/path/to/worktree")
	if found == nil || found.ID != w.ID {
		t.Errorf("FindByWorktree() failed")
	}

	notFound := reg.FindByWorktree("/other/path")
	if notFound != nil {
		t.Errorf("FindByWorktree() should return nil for unknown path")
	}
}
