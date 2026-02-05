package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckConflicts(t *testing.T) {
	t.Run("detects parent workspace", func(t *testing.T) {
		dir := t.TempDir()

		// Create parent workspace
		if _, err := Init(dir, "parent-ws", ""); err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		// Check conflicts from a subdirectory
		subDir := filepath.Join(dir, "subdir")
		if err := os.MkdirAll(subDir, 0o755); err != nil {
			t.Fatalf("mkdir error = %v", err)
		}

		check, err := CheckConflicts(subDir)
		if err != nil {
			t.Fatalf("CheckConflicts() error = %v", err)
		}

		if !check.HasParentWorkspace {
			t.Error("should detect parent workspace")
		}
		if check.ParentWorkspace != dir {
			t.Errorf("ParentWorkspace = %q, want %q", check.ParentWorkspace, dir)
		}
	})

	t.Run("detects child workspaces", func(t *testing.T) {
		dir := t.TempDir()

		// Create child workspaces
		child1 := filepath.Join(dir, "child1")
		child2 := filepath.Join(dir, "child2")

		if _, err := Init(child1, "child1-ws", ""); err != nil {
			t.Fatalf("Init() error = %v", err)
		}
		if _, err := Init(child2, "child2-ws", ""); err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		check, err := CheckConflicts(dir)
		if err != nil {
			t.Fatalf("CheckConflicts() error = %v", err)
		}

		if !check.HasChildWorkspace {
			t.Error("should detect child workspaces")
		}
		if len(check.ChildWorkspaces) != 2 {
			t.Errorf("len(ChildWorkspaces) = %d, want 2", len(check.ChildWorkspaces))
		}
	})

	t.Run("no conflicts in empty directory", func(t *testing.T) {
		dir := t.TempDir()

		check, err := CheckConflicts(dir)
		if err != nil {
			t.Fatalf("CheckConflicts() error = %v", err)
		}

		if check.HasParentWorkspace {
			t.Error("should not detect parent workspace")
		}
		if check.HasChildWorkspace {
			t.Error("should not detect child workspace")
		}
	})

	t.Run("respects max depth for child search", func(t *testing.T) {
		dir := t.TempDir()

		// Create workspace at depth 4 (beyond default max of 3)
		deepPath := filepath.Join(dir, "l1", "l2", "l3", "l4")
		if err := os.MkdirAll(deepPath, 0o755); err != nil {
			t.Fatalf("mkdir error = %v", err)
		}
		if _, err := Init(deepPath, "deep-ws", ""); err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		check, err := CheckConflicts(dir)
		if err != nil {
			t.Fatalf("CheckConflicts() error = %v", err)
		}

		// Should not find workspace at depth 4
		if check.HasChildWorkspace {
			t.Error("should not find workspace beyond max depth")
		}
	})
}

func TestFindChildWorkspaces(t *testing.T) {
	t.Run("finds workspaces in subdirectories", func(t *testing.T) {
		dir := t.TempDir()

		// Create workspaces
		ws1 := filepath.Join(dir, "project1")
		ws2 := filepath.Join(dir, "project2")

		Init(ws1, "ws1", "")
		Init(ws2, "ws2", "")

		found := findChildWorkspaces(dir, 2)
		if len(found) != 2 {
			t.Errorf("len(found) = %d, want 2", len(found))
		}
	})

	t.Run("skips hidden directories", func(t *testing.T) {
		dir := t.TempDir()

		// Create workspace in hidden directory
		hidden := filepath.Join(dir, ".hidden")
		if err := os.MkdirAll(hidden, 0o755); err != nil {
			t.Fatalf("mkdir error = %v", err)
		}
		Init(hidden, "hidden-ws", "")

		// Create visible workspace
		visible := filepath.Join(dir, "visible")
		Init(visible, "visible-ws", "")

		found := findChildWorkspaces(dir, 2)
		if len(found) != 1 {
			t.Errorf("len(found) = %d, want 1", len(found))
		}
		if len(found) > 0 && found[0] != visible {
			t.Errorf("found = %v, want %q", found, visible)
		}
	})

	t.Run("skips node_modules", func(t *testing.T) {
		dir := t.TempDir()

		// Create workspace in node_modules
		nodeModules := filepath.Join(dir, "node_modules", "some-package")
		if err := os.MkdirAll(nodeModules, 0o755); err != nil {
			t.Fatalf("mkdir error = %v", err)
		}
		Init(nodeModules, "node-ws", "")

		found := findChildWorkspaces(dir, 3)
		if len(found) != 0 {
			t.Errorf("should skip node_modules, found = %v", found)
		}
	})

	t.Run("skips vendor directory", func(t *testing.T) {
		dir := t.TempDir()

		// Create workspace in vendor
		vendor := filepath.Join(dir, "vendor", "some-dep")
		if err := os.MkdirAll(vendor, 0o755); err != nil {
			t.Fatalf("mkdir error = %v", err)
		}
		Init(vendor, "vendor-ws", "")

		found := findChildWorkspaces(dir, 3)
		if len(found) != 0 {
			t.Errorf("should skip vendor, found = %v", found)
		}
	})

	t.Run("skips __pycache__ directory", func(t *testing.T) {
		dir := t.TempDir()

		// Create workspace in __pycache__
		pycache := filepath.Join(dir, "__pycache__")
		if err := os.MkdirAll(pycache, 0o755); err != nil {
			t.Fatalf("mkdir error = %v", err)
		}
		Init(pycache, "pycache-ws", "")

		found := findChildWorkspaces(dir, 2)
		if len(found) != 0 {
			t.Errorf("should skip __pycache__, found = %v", found)
		}
	})

	t.Run("does not recurse into found workspaces", func(t *testing.T) {
		dir := t.TempDir()

		// Create parent workspace with nested workspace
		parent := filepath.Join(dir, "parent")
		nested := filepath.Join(parent, "nested")

		Init(parent, "parent-ws", "")
		Init(nested, "nested-ws", "")

		found := findChildWorkspaces(dir, 3)

		// Should only find parent, not recurse into it
		if len(found) != 1 {
			t.Errorf("len(found) = %d, want 1", len(found))
		}
		if len(found) > 0 && found[0] != parent {
			t.Errorf("found = %v, want only %q", found, parent)
		}
	})

	t.Run("returns nil for max depth 0", func(t *testing.T) {
		dir := t.TempDir()
		Init(filepath.Join(dir, "ws"), "ws", "")

		found := findChildWorkspaces(dir, 0)
		if found != nil {
			t.Errorf("found = %v, want nil", found)
		}
	})

	t.Run("handles unreadable directory", func(t *testing.T) {
		dir := t.TempDir()

		// Create unreadable directory
		unreadable := filepath.Join(dir, "unreadable")
		if err := os.MkdirAll(unreadable, 0o000); err != nil {
			t.Fatalf("mkdir error = %v", err)
		}
		defer os.Chmod(unreadable, 0o755) // Cleanup

		found := findChildWorkspaces(dir, 2)
		// Should not error, just skip
		if found == nil {
			found = []string{}
		}
		_ = found // No error expected
	})
}

func TestIsInsideWorkspace(t *testing.T) {
	t.Run("returns true when inside workspace", func(t *testing.T) {
		dir := t.TempDir()

		Init(dir, "ws", "")

		subDir := filepath.Join(dir, "sub", "path")
		os.MkdirAll(subDir, 0o755)

		if !IsInsideWorkspace(subDir) {
			t.Error("should be inside workspace")
		}
	})

	t.Run("returns false when not inside workspace", func(t *testing.T) {
		dir := t.TempDir()

		if IsInsideWorkspace(dir) {
			t.Error("should not be inside workspace")
		}
	})

	t.Run("returns true at workspace root", func(t *testing.T) {
		dir := t.TempDir()

		Init(dir, "ws", "")

		if !IsInsideWorkspace(dir) {
			t.Error("workspace root should be inside workspace")
		}
	})
}

func TestListGlobalWorkspaces(t *testing.T) {
	// Note: This test is limited because it depends on the user's home directory
	// We can only verify it doesn't error when directories don't exist

	t.Run("does not error when common directories do not exist", func(t *testing.T) {
		workspaces, err := ListGlobalWorkspaces()
		if err != nil {
			t.Errorf("ListGlobalWorkspaces() error = %v", err)
		}

		// Should return a slice (possibly empty)
		if workspaces == nil {
			// nil is acceptable, but we'll make it an empty slice for consistency
			workspaces = []string{}
		}
		_ = workspaces
	})
}
