package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeBranchName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple lowercase",
			input:    "feature",
			expected: "feature",
		},
		{
			name:     "converts uppercase to lowercase",
			input:    "MyFeature",
			expected: "myfeature",
		},
		{
			name:     "replaces spaces with dashes",
			input:    "my new feature",
			expected: "my-new-feature",
		},
		{
			name:     "replaces underscores with dashes",
			input:    "my_feature_name",
			expected: "my-feature-name",
		},
		{
			name:     "removes special characters",
			input:    "feature@#$%name",
			expected: "featurename",
		},
		{
			name:     "collapses multiple dashes",
			input:    "feature--name---test",
			expected: "feature-name-test",
		},
		{
			name:     "trims leading and trailing dashes",
			input:    "-feature-name-",
			expected: "feature-name",
		},
		{
			name:     "preserves numbers",
			input:    "feature123test",
			expected: "feature123test",
		},
		{
			name:     "complex case",
			input:    "My  New_Feature  #42",
			expected: "my-new-feature-42",
		},
		{
			name:     "only special chars",
			input:    "@#$%^&*()",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeBranchName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeBranchName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestWorktreeConfigLoadSave(t *testing.T) {
	t.Run("load returns empty config when file does not exist", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		config, err := ws.loadWorktreeConfig()
		if err != nil {
			t.Fatalf("loadWorktreeConfig() error = %v", err)
		}

		if len(config.Worktrees) != 0 {
			t.Errorf("len(Worktrees) = %d, want 0", len(config.Worktrees))
		}
	})

	t.Run("save and load roundtrip", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		// Create config with worktree
		config := &WorktreeConfig{
			Worktrees: []Worktree{
				{
					Name:       "feature-a",
					RepoName:   "test-repo",
					Branch:     "feature/feature-a",
					BaseBranch: "main",
					Path:       "/path/to/worktree",
					Status:     "active",
				},
			},
		}

		if err := ws.saveWorktreeConfig(config); err != nil {
			t.Fatalf("saveWorktreeConfig() error = %v", err)
		}

		// Load back
		loaded, err := ws.loadWorktreeConfig()
		if err != nil {
			t.Fatalf("loadWorktreeConfig() error = %v", err)
		}

		if len(loaded.Worktrees) != 1 {
			t.Fatalf("len(Worktrees) = %d, want 1", len(loaded.Worktrees))
		}

		wt := loaded.Worktrees[0]
		if wt.Name != "feature-a" {
			t.Errorf("Name = %q, want %q", wt.Name, "feature-a")
		}
		if wt.RepoName != "test-repo" {
			t.Errorf("RepoName = %q, want %q", wt.RepoName, "test-repo")
		}
		if wt.Branch != "feature/feature-a" {
			t.Errorf("Branch = %q, want %q", wt.Branch, "feature/feature-a")
		}
		if wt.Status != "active" {
			t.Errorf("Status = %q, want %q", wt.Status, "active")
		}
	})

	t.Run("load handles invalid yaml", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		// Write invalid yaml
		configPath := ws.worktreeConfigPath()
		if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
			t.Fatalf("mkdir error = %v", err)
		}
		if err := os.WriteFile(configPath, []byte("invalid: yaml: ["), 0o644); err != nil {
			t.Fatalf("write error = %v", err)
		}

		_, err = ws.loadWorktreeConfig()
		if err == nil {
			t.Error("loadWorktreeConfig() should fail for invalid yaml")
		}
	})
}

func TestSaveWorktree(t *testing.T) {
	t.Run("adds worktree to config", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		wt := &Worktree{
			Name:     "new-feature",
			RepoName: "test-repo",
			Branch:   "feature/new-feature",
			Status:   "active",
		}

		if err := ws.saveWorktree(wt); err != nil {
			t.Fatalf("saveWorktree() error = %v", err)
		}

		// Verify it was saved
		config, _ := ws.loadWorktreeConfig()
		if len(config.Worktrees) != 1 {
			t.Fatalf("len(Worktrees) = %d, want 1", len(config.Worktrees))
		}
		if config.Worktrees[0].Name != "new-feature" {
			t.Errorf("Name = %q, want %q", config.Worktrees[0].Name, "new-feature")
		}
	})

	t.Run("rejects duplicate worktree name", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		wt1 := &Worktree{Name: "same-name", RepoName: "repo1"}
		if err := ws.saveWorktree(wt1); err != nil {
			t.Fatalf("first saveWorktree() error = %v", err)
		}

		wt2 := &Worktree{Name: "same-name", RepoName: "repo2"}
		err = ws.saveWorktree(wt2)
		if err == nil {
			t.Error("saveWorktree() should reject duplicate name")
		}
	})
}

func TestRemoveWorktreeFromConfig(t *testing.T) {
	t.Run("removes worktree from config", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		// Add worktree
		ws.saveWorktree(&Worktree{Name: "to-remove"})

		// Remove it
		if err := ws.removeWorktreeFromConfig("to-remove"); err != nil {
			t.Fatalf("removeWorktreeFromConfig() error = %v", err)
		}

		config, _ := ws.loadWorktreeConfig()
		if len(config.Worktrees) != 0 {
			t.Errorf("len(Worktrees) = %d, want 0", len(config.Worktrees))
		}
	})

	t.Run("does not error if worktree does not exist", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		// Should not error
		err = ws.removeWorktreeFromConfig("non-existent")
		if err != nil {
			t.Errorf("removeWorktreeFromConfig() error = %v", err)
		}
	})
}

func TestListWorktrees(t *testing.T) {
	t.Run("returns all worktrees", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		// Add worktrees
		ws.saveWorktree(&Worktree{Name: "wt-a", Status: "active"})
		ws.saveWorktree(&Worktree{Name: "wt-b", Status: "merged"})
		ws.saveWorktree(&Worktree{Name: "wt-c", Status: "active"})

		worktrees, err := ws.ListWorktrees()
		if err != nil {
			t.Fatalf("ListWorktrees() error = %v", err)
		}

		if len(worktrees) != 3 {
			t.Errorf("len(worktrees) = %d, want 3", len(worktrees))
		}
	})

	t.Run("returns empty slice when no worktrees", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		worktrees, err := ws.ListWorktrees()
		if err != nil {
			t.Fatalf("ListWorktrees() error = %v", err)
		}

		if len(worktrees) != 0 {
			t.Errorf("len(worktrees) = %d, want 0", len(worktrees))
		}
	})
}

func TestGetWorktree(t *testing.T) {
	t.Run("returns existing worktree", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		ws.saveWorktree(&Worktree{
			Name:     "my-worktree",
			RepoName: "test-repo",
			Branch:   "feature/test",
		})

		wt, err := ws.GetWorktree("my-worktree")
		if err != nil {
			t.Fatalf("GetWorktree() error = %v", err)
		}

		if wt.Name != "my-worktree" {
			t.Errorf("Name = %q, want %q", wt.Name, "my-worktree")
		}
		if wt.RepoName != "test-repo" {
			t.Errorf("RepoName = %q, want %q", wt.RepoName, "test-repo")
		}
	})

	t.Run("returns error for non-existent worktree", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		_, err = ws.GetWorktree("non-existent")
		if err == nil {
			t.Error("GetWorktree() should fail for non-existent worktree")
		}
	})
}

func TestActiveWorktrees(t *testing.T) {
	t.Run("returns only active worktrees", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		ws.saveWorktree(&Worktree{Name: "active-1", Status: "active"})
		ws.saveWorktree(&Worktree{Name: "merged-1", Status: "merged"})
		ws.saveWorktree(&Worktree{Name: "active-2", Status: "active"})
		ws.saveWorktree(&Worktree{Name: "abandoned", Status: "abandoned"})

		active, err := ws.ActiveWorktrees()
		if err != nil {
			t.Fatalf("ActiveWorktrees() error = %v", err)
		}

		if len(active) != 2 {
			t.Errorf("len(active) = %d, want 2", len(active))
		}

		// Verify they're the right ones
		names := map[string]bool{}
		for _, wt := range active {
			names[wt.Name] = true
		}
		if !names["active-1"] || !names["active-2"] {
			t.Errorf("active worktrees = %v, want active-1 and active-2", names)
		}
	})
}

func TestUpdateWorktreeStatus(t *testing.T) {
	t.Run("updates status", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		ws.saveWorktree(&Worktree{Name: "my-wt", Status: "active"})

		if err := ws.UpdateWorktreeStatus("my-wt", "merged"); err != nil {
			t.Fatalf("UpdateWorktreeStatus() error = %v", err)
		}

		wt, _ := ws.GetWorktree("my-wt")
		if wt.Status != "merged" {
			t.Errorf("Status = %q, want %q", wt.Status, "merged")
		}
	})

	t.Run("returns error for non-existent worktree", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		err = ws.UpdateWorktreeStatus("non-existent", "merged")
		if err == nil {
			t.Error("UpdateWorktreeStatus() should fail for non-existent worktree")
		}
	})
}

func TestLinkWorktreeToNotion(t *testing.T) {
	t.Run("links worktree to notion ID", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		ws.saveWorktree(&Worktree{Name: "my-wt"})

		if err := ws.LinkWorktreeToNotion("my-wt", "notion-123"); err != nil {
			t.Fatalf("LinkWorktreeToNotion() error = %v", err)
		}

		wt, _ := ws.GetWorktree("my-wt")
		if wt.NotionID != "notion-123" {
			t.Errorf("NotionID = %q, want %q", wt.NotionID, "notion-123")
		}
	})

	t.Run("returns error for non-existent worktree", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		err = ws.LinkWorktreeToNotion("non-existent", "notion-123")
		if err == nil {
			t.Error("LinkWorktreeToNotion() should fail for non-existent worktree")
		}
	})
}

func TestLinkWorktreeToBead(t *testing.T) {
	t.Run("links worktree to bead ID", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		ws.saveWorktree(&Worktree{Name: "my-wt"})

		if err := ws.LinkWorktreeToBead("my-wt", "bead-456"); err != nil {
			t.Fatalf("LinkWorktreeToBead() error = %v", err)
		}

		wt, _ := ws.GetWorktree("my-wt")
		if wt.BeadID != "bead-456" {
			t.Errorf("BeadID = %q, want %q", wt.BeadID, "bead-456")
		}
	})

	t.Run("returns error for non-existent worktree", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		err = ws.LinkWorktreeToBead("non-existent", "bead-456")
		if err == nil {
			t.Error("LinkWorktreeToBead() should fail for non-existent worktree")
		}
	})
}

func TestWorktreeConfigPath(t *testing.T) {
	dir := t.TempDir()

	ws, err := Init(dir, "test-ws", "")
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	expected := filepath.Join(dir, ConfigDir, "worktrees", "worktrees.yaml")
	if ws.worktreeConfigPath() != expected {
		t.Errorf("worktreeConfigPath() = %q, want %q", ws.worktreeConfigPath(), expected)
	}
}
