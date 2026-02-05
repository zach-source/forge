package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInit(t *testing.T) {
	t.Run("creates workspace with expected structure", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-workspace", "A test workspace")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		// Verify workspace fields
		if ws.Name != "test-workspace" {
			t.Errorf("Name = %q, want %q", ws.Name, "test-workspace")
		}
		if ws.Description != "A test workspace" {
			t.Errorf("Description = %q, want %q", ws.Description, "A test workspace")
		}
		if ws.Path != dir {
			t.Errorf("Path = %q, want %q", ws.Path, dir)
		}
		if ws.CreatedAt.IsZero() {
			t.Error("CreatedAt should not be zero")
		}
		if ws.Repos == nil {
			t.Error("Repos should not be nil")
		}

		// Verify directory structure
		expectedDirs := []string{
			ConfigDir,
			filepath.Join(ConfigDir, "sessions"),
			filepath.Join(ConfigDir, "logs"),
			filepath.Join(ConfigDir, "repos"),
			filepath.Join(ConfigDir, "worktrees"),
		}

		for _, subdir := range expectedDirs {
			path := filepath.Join(dir, subdir)
			info, err := os.Stat(path)
			if err != nil {
				t.Errorf("directory %s should exist: %v", subdir, err)
			} else if !info.IsDir() {
				t.Errorf("%s should be a directory", subdir)
			}
		}

		// Verify config file exists
		configPath := filepath.Join(dir, ConfigDir, ConfigFile)
		if _, err := os.Stat(configPath); err != nil {
			t.Errorf("config file should exist: %v", err)
		}

		// Verify .gitignore was created
		gitignorePath := filepath.Join(dir, ConfigDir, ".gitignore")
		if _, err := os.Stat(gitignorePath); err != nil {
			t.Errorf(".gitignore should exist: %v", err)
		}
	})

	t.Run("fails if already initialized", func(t *testing.T) {
		dir := t.TempDir()

		// First init should succeed
		_, err := Init(dir, "first", "")
		if err != nil {
			t.Fatalf("first Init() error = %v", err)
		}

		// Second init should fail
		_, err = Init(dir, "second", "")
		if err == nil {
			t.Error("second Init() should fail")
		}
	})

	t.Run("creates parent directories if needed", func(t *testing.T) {
		tempBase := t.TempDir()
		dir := filepath.Join(tempBase, "deep", "nested", "workspace")

		ws, err := Init(dir, "nested-workspace", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		if ws.Path != dir {
			t.Errorf("Path = %q, want %q", ws.Path, dir)
		}
	})
}

func TestLoad(t *testing.T) {
	t.Run("loads existing workspace", func(t *testing.T) {
		dir := t.TempDir()

		// Create workspace first
		original, err := Init(dir, "test-ws", "test description")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		// Load it back
		loaded, err := Load(dir)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if loaded.Name != original.Name {
			t.Errorf("Name = %q, want %q", loaded.Name, original.Name)
		}
		if loaded.Description != original.Description {
			t.Errorf("Description = %q, want %q", loaded.Description, original.Description)
		}
		if loaded.Path != dir {
			t.Errorf("Path = %q, want %q", loaded.Path, dir)
		}
	})

	t.Run("returns error for non-workspace directory", func(t *testing.T) {
		dir := t.TempDir()

		_, err := Load(dir)
		if err == nil {
			t.Error("Load() should fail for non-workspace")
		}
	})

	t.Run("returns error for invalid yaml", func(t *testing.T) {
		dir := t.TempDir()

		// Create invalid config file
		configDir := filepath.Join(dir, ConfigDir)
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			t.Fatalf("mkdir error = %v", err)
		}

		configPath := filepath.Join(configDir, ConfigFile)
		if err := os.WriteFile(configPath, []byte("invalid: yaml: content: ["), 0o644); err != nil {
			t.Fatalf("write error = %v", err)
		}

		_, err := Load(dir)
		if err == nil {
			t.Error("Load() should fail for invalid yaml")
		}
	})
}

func TestFind(t *testing.T) {
	t.Run("finds workspace in current directory", func(t *testing.T) {
		dir := t.TempDir()

		// Create workspace
		_, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		// Find from same directory
		ws, err := Find(dir)
		if err != nil {
			t.Fatalf("Find() error = %v", err)
		}

		if ws.Name != "test-ws" {
			t.Errorf("Name = %q, want %q", ws.Name, "test-ws")
		}
	})

	t.Run("finds workspace in parent directory", func(t *testing.T) {
		dir := t.TempDir()

		// Create workspace
		_, err := Init(dir, "parent-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		// Create nested directories
		nestedDir := filepath.Join(dir, "level1", "level2", "level3")
		if err := os.MkdirAll(nestedDir, 0o755); err != nil {
			t.Fatalf("mkdir error = %v", err)
		}

		// Find from nested directory
		ws, err := Find(nestedDir)
		if err != nil {
			t.Fatalf("Find() error = %v", err)
		}

		if ws.Name != "parent-ws" {
			t.Errorf("Name = %q, want %q", ws.Name, "parent-ws")
		}
		if ws.Path != dir {
			t.Errorf("Path = %q, want %q", ws.Path, dir)
		}
	})

	t.Run("returns error when no workspace found", func(t *testing.T) {
		dir := t.TempDir()

		// Don't initialize - just try to find
		_, err := Find(dir)
		if err == nil {
			t.Error("Find() should fail when no workspace exists")
		}
	})
}

func TestSave(t *testing.T) {
	t.Run("persists changes to disk", func(t *testing.T) {
		dir := t.TempDir()

		// Create workspace
		ws, err := Init(dir, "original", "original description")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		// Modify and save
		ws.Name = "modified"
		ws.Description = "modified description"
		if err := ws.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		// Load and verify
		loaded, err := Load(dir)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if loaded.Name != "modified" {
			t.Errorf("Name = %q, want %q", loaded.Name, "modified")
		}
		if loaded.Description != "modified description" {
			t.Errorf("Description = %q, want %q", loaded.Description, "modified description")
		}
	})
}

func TestAddRepo(t *testing.T) {
	t.Run("adds repository to workspace", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		repo := Repo{
			Name:   "test-repo",
			Path:   "/path/to/repo",
			Remote: "https://github.com/user/repo.git",
		}

		if err := ws.AddRepo(repo); err != nil {
			t.Fatalf("AddRepo() error = %v", err)
		}

		if len(ws.Repos) != 1 {
			t.Fatalf("len(Repos) = %d, want 1", len(ws.Repos))
		}

		if ws.Repos[0].Name != "test-repo" {
			t.Errorf("Name = %q, want %q", ws.Repos[0].Name, "test-repo")
		}
		if ws.Repos[0].AddedAt.IsZero() {
			t.Error("AddedAt should be set")
		}
	})

	t.Run("rejects duplicate name", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		repo1 := Repo{Name: "my-repo", Path: "/path/one"}
		if err := ws.AddRepo(repo1); err != nil {
			t.Fatalf("first AddRepo() error = %v", err)
		}

		repo2 := Repo{Name: "my-repo", Path: "/path/two"}
		err = ws.AddRepo(repo2)
		if err == nil {
			t.Error("AddRepo() should reject duplicate name")
		}
	})

	t.Run("rejects duplicate path", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		repo1 := Repo{Name: "repo-one", Path: "/same/path"}
		if err := ws.AddRepo(repo1); err != nil {
			t.Fatalf("first AddRepo() error = %v", err)
		}

		repo2 := Repo{Name: "repo-two", Path: "/same/path"}
		err = ws.AddRepo(repo2)
		if err == nil {
			t.Error("AddRepo() should reject duplicate path")
		}
	})

	t.Run("persists to disk", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		repo := Repo{Name: "persisted-repo", Path: "/some/path"}
		if err := ws.AddRepo(repo); err != nil {
			t.Fatalf("AddRepo() error = %v", err)
		}

		// Load from disk
		loaded, err := Load(dir)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if len(loaded.Repos) != 1 {
			t.Fatalf("len(Repos) = %d, want 1", len(loaded.Repos))
		}
		if loaded.Repos[0].Name != "persisted-repo" {
			t.Errorf("Name = %q, want %q", loaded.Repos[0].Name, "persisted-repo")
		}
	})
}

func TestRemoveRepo(t *testing.T) {
	t.Run("removes existing repository", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		// Add repo
		if err := ws.AddRepo(Repo{Name: "to-remove", Path: "/path"}); err != nil {
			t.Fatalf("AddRepo() error = %v", err)
		}

		// Remove it
		if err := ws.RemoveRepo("to-remove"); err != nil {
			t.Fatalf("RemoveRepo() error = %v", err)
		}

		if len(ws.Repos) != 0 {
			t.Errorf("len(Repos) = %d, want 0", len(ws.Repos))
		}
	})

	t.Run("returns error for non-existent repository", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		err = ws.RemoveRepo("non-existent")
		if err == nil {
			t.Error("RemoveRepo() should fail for non-existent repo")
		}
	})

	t.Run("removes correct repo from middle", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		// Add three repos
		ws.AddRepo(Repo{Name: "repo-a", Path: "/a"})
		ws.AddRepo(Repo{Name: "repo-b", Path: "/b"})
		ws.AddRepo(Repo{Name: "repo-c", Path: "/c"})

		// Remove middle one
		if err := ws.RemoveRepo("repo-b"); err != nil {
			t.Fatalf("RemoveRepo() error = %v", err)
		}

		if len(ws.Repos) != 2 {
			t.Fatalf("len(Repos) = %d, want 2", len(ws.Repos))
		}
		if ws.Repos[0].Name != "repo-a" || ws.Repos[1].Name != "repo-c" {
			t.Errorf("remaining repos = %v, want [repo-a, repo-c]", ws.Repos)
		}
	})
}

func TestGetRepo(t *testing.T) {
	t.Run("returns existing repository", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		ws.AddRepo(Repo{Name: "my-repo", Path: "/path", Remote: "https://example.com"})

		repo := ws.GetRepo("my-repo")
		if repo == nil {
			t.Fatal("GetRepo() returned nil")
		}
		if repo.Name != "my-repo" {
			t.Errorf("Name = %q, want %q", repo.Name, "my-repo")
		}
		if repo.Remote != "https://example.com" {
			t.Errorf("Remote = %q, want %q", repo.Remote, "https://example.com")
		}
	})

	t.Run("returns nil for non-existent repository", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		repo := ws.GetRepo("non-existent")
		if repo != nil {
			t.Error("GetRepo() should return nil for non-existent repo")
		}
	})
}

func TestSetPrimary(t *testing.T) {
	t.Run("sets repository as primary", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		ws.AddRepo(Repo{Name: "repo-a", Path: "/a"})
		ws.AddRepo(Repo{Name: "repo-b", Path: "/b"})

		if err := ws.SetPrimary("repo-b"); err != nil {
			t.Fatalf("SetPrimary() error = %v", err)
		}

		if !ws.Repos[1].IsPrimary {
			t.Error("repo-b should be primary")
		}
		if ws.Repos[0].IsPrimary {
			t.Error("repo-a should not be primary")
		}
	})

	t.Run("clears previous primary", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		ws.AddRepo(Repo{Name: "repo-a", Path: "/a"})
		ws.AddRepo(Repo{Name: "repo-b", Path: "/b"})

		// Set first as primary
		ws.SetPrimary("repo-a")

		// Set second as primary
		ws.SetPrimary("repo-b")

		if ws.Repos[0].IsPrimary {
			t.Error("repo-a should no longer be primary")
		}
		if !ws.Repos[1].IsPrimary {
			t.Error("repo-b should be primary")
		}
	})

	t.Run("returns error for non-existent repository", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		err = ws.SetPrimary("non-existent")
		if err == nil {
			t.Error("SetPrimary() should fail for non-existent repo")
		}
	})
}

func TestPrimaryRepo(t *testing.T) {
	t.Run("returns primary repository", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		ws.AddRepo(Repo{Name: "repo-a", Path: "/a"})
		ws.AddRepo(Repo{Name: "repo-b", Path: "/b"})
		ws.SetPrimary("repo-b")

		primary := ws.PrimaryRepo()
		if primary == nil {
			t.Fatal("PrimaryRepo() returned nil")
		}
		if primary.Name != "repo-b" {
			t.Errorf("Name = %q, want %q", primary.Name, "repo-b")
		}
	})

	t.Run("returns first repo when none marked primary", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		ws.AddRepo(Repo{Name: "first-repo", Path: "/first"})
		ws.AddRepo(Repo{Name: "second-repo", Path: "/second"})

		primary := ws.PrimaryRepo()
		if primary == nil {
			t.Fatal("PrimaryRepo() returned nil")
		}
		if primary.Name != "first-repo" {
			t.Errorf("Name = %q, want %q", primary.Name, "first-repo")
		}
	})

	t.Run("returns nil when no repos", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		primary := ws.PrimaryRepo()
		if primary != nil {
			t.Error("PrimaryRepo() should return nil when no repos")
		}
	})
}

func TestUpdateRepo(t *testing.T) {
	t.Run("updates repository metadata", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		ws.AddRepo(Repo{Name: "my-repo", Path: "/path"})

		err = ws.UpdateRepo("my-repo", func(r *Repo) {
			r.Summary = "A test repository"
			r.Technologies = []string{"Go", "Docker"}
		})
		if err != nil {
			t.Fatalf("UpdateRepo() error = %v", err)
		}

		repo := ws.GetRepo("my-repo")
		if repo.Summary != "A test repository" {
			t.Errorf("Summary = %q, want %q", repo.Summary, "A test repository")
		}
		if len(repo.Technologies) != 2 {
			t.Errorf("len(Technologies) = %d, want 2", len(repo.Technologies))
		}
	})

	t.Run("returns error for non-existent repository", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		err = ws.UpdateRepo("non-existent", func(r *Repo) {
			r.Summary = "test"
		})
		if err == nil {
			t.Error("UpdateRepo() should fail for non-existent repo")
		}
	})

	t.Run("persists changes to disk", func(t *testing.T) {
		dir := t.TempDir()

		ws, err := Init(dir, "test-ws", "")
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}

		ws.AddRepo(Repo{Name: "my-repo", Path: "/path"})
		ws.UpdateRepo("my-repo", func(r *Repo) {
			r.DevBranch = "develop"
		})

		// Load from disk
		loaded, err := Load(dir)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		repo := loaded.GetRepo("my-repo")
		if repo.DevBranch != "develop" {
			t.Errorf("DevBranch = %q, want %q", repo.DevBranch, "develop")
		}
	})
}

func TestDirectoryHelpers(t *testing.T) {
	dir := t.TempDir()

	ws, err := Init(dir, "test-ws", "")
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	t.Run("ReposDir", func(t *testing.T) {
		expected := filepath.Join(dir, ConfigDir, "repos")
		if ws.ReposDir() != expected {
			t.Errorf("ReposDir() = %q, want %q", ws.ReposDir(), expected)
		}
	})

	t.Run("SessionsDir", func(t *testing.T) {
		expected := filepath.Join(dir, ConfigDir, "sessions")
		if ws.SessionsDir() != expected {
			t.Errorf("SessionsDir() = %q, want %q", ws.SessionsDir(), expected)
		}
	})

	t.Run("LogsDir", func(t *testing.T) {
		expected := filepath.Join(dir, ConfigDir, "logs")
		if ws.LogsDir() != expected {
			t.Errorf("LogsDir() = %q, want %q", ws.LogsDir(), expected)
		}
	})

	t.Run("WorktreesDir", func(t *testing.T) {
		expected := filepath.Join(dir, ConfigDir, "worktrees")
		if ws.WorktreesDir() != expected {
			t.Errorf("WorktreesDir() = %q, want %q", ws.WorktreesDir(), expected)
		}
	})
}
