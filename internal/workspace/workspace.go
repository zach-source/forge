// Package workspace manages forge workspace configuration and repositories.
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Workspace represents a forge workspace configuration.
type Workspace struct {
	Name        string    `yaml:"name"`
	Description string    `yaml:"description,omitempty"`
	CreatedAt   time.Time `yaml:"created_at"`
	Repos       []Repo    `yaml:"repos,omitempty"`

	// Runtime fields (not persisted)
	Path string `yaml:"-"`
}

// Repo represents a repository in the workspace.
type Repo struct {
	Name      string    `yaml:"name"`
	Path      string    `yaml:"path"`
	Remote    string    `yaml:"remote,omitempty"`
	Branch    string    `yaml:"branch,omitempty"`
	AddedAt   time.Time `yaml:"added_at"`
	IsLinked  bool      `yaml:"is_linked"` // true if linked, false if cloned
	IsPrimary bool      `yaml:"is_primary,omitempty"`

	// Metadata about the repository
	Summary      string   `yaml:"summary,omitempty"`      // Brief description of the repo
	Features     []string `yaml:"features,omitempty"`     // Key features/capabilities
	Technologies []string `yaml:"technologies,omitempty"` // Languages, frameworks, tools
	DevBranch    string   `yaml:"dev_branch,omitempty"`   // Development branch (default: main)
}

const (
	// ConfigDir is the directory name for forge configuration
	ConfigDir = ".forge"
	// ConfigFile is the workspace configuration file name
	ConfigFile = "workspace.yaml"
)

// Init initializes a new forge workspace in the given directory.
func Init(dir, name, description string) (*Workspace, error) {
	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating workspace directory: %w", err)
	}

	// Check if already initialized
	configPath := filepath.Join(dir, ConfigDir, ConfigFile)
	if _, err := os.Stat(configPath); err == nil {
		return nil, fmt.Errorf("workspace already initialized at %s", dir)
	}

	// Create .forge directory
	forgeDir := filepath.Join(dir, ConfigDir)
	if err := os.MkdirAll(forgeDir, 0755); err != nil {
		return nil, fmt.Errorf("creating .forge directory: %w", err)
	}

	// Create workspace config
	ws := &Workspace{
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
		Repos:       []Repo{},
		Path:        dir,
	}

	if err := ws.Save(); err != nil {
		return nil, err
	}

	// Create subdirectories
	subdirs := []string{"sessions", "logs", "repos", "worktrees"}
	for _, sub := range subdirs {
		subPath := filepath.Join(forgeDir, sub)
		if err := os.MkdirAll(subPath, 0755); err != nil {
			return nil, fmt.Errorf("creating %s directory: %w", sub, err)
		}
	}

	// Create .gitignore for .forge directory
	gitignore := `# Forge workspace files
sessions/
logs/
worktrees/
*.log
*.tmp
`
	gitignorePath := filepath.Join(forgeDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(gitignore), 0644); err != nil {
		// Non-fatal
		fmt.Printf("Warning: failed to create .gitignore: %v\n", err)
	}

	return ws, nil
}

// Load loads workspace configuration from the given directory.
func Load(dir string) (*Workspace, error) {
	configPath := filepath.Join(dir, ConfigDir, ConfigFile)

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("not a forge workspace (no %s found)", configPath)
		}
		return nil, fmt.Errorf("reading workspace config: %w", err)
	}

	var ws Workspace
	if err := yaml.Unmarshal(data, &ws); err != nil {
		return nil, fmt.Errorf("parsing workspace config: %w", err)
	}

	ws.Path = dir
	return &ws, nil
}

// Find searches for a workspace starting from the given directory and walking up.
func Find(startDir string) (*Workspace, error) {
	dir := startDir
	for {
		configPath := filepath.Join(dir, ConfigDir, ConfigFile)
		if _, err := os.Stat(configPath); err == nil {
			return Load(dir)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root
			break
		}
		dir = parent
	}

	return nil, fmt.Errorf("no forge workspace found (searched from %s to root)", startDir)
}

// Save persists the workspace configuration to disk.
func (w *Workspace) Save() error {
	configPath := filepath.Join(w.Path, ConfigDir, ConfigFile)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(w)
	if err != nil {
		return fmt.Errorf("marshaling workspace config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("writing workspace config: %w", err)
	}

	return nil
}

// AddRepo adds a repository to the workspace.
func (w *Workspace) AddRepo(repo Repo) error {
	// Check for duplicates
	for _, r := range w.Repos {
		if r.Name == repo.Name {
			return fmt.Errorf("repository %q already exists in workspace", repo.Name)
		}
		if r.Path == repo.Path {
			return fmt.Errorf("repository at %q already exists as %q", repo.Path, r.Name)
		}
	}

	repo.AddedAt = time.Now()
	w.Repos = append(w.Repos, repo)

	return w.Save()
}

// RemoveRepo removes a repository from the workspace by name.
func (w *Workspace) RemoveRepo(name string) error {
	idx := -1
	for i, r := range w.Repos {
		if r.Name == name {
			idx = i
			break
		}
	}

	if idx == -1 {
		return fmt.Errorf("repository %q not found in workspace", name)
	}

	w.Repos = append(w.Repos[:idx], w.Repos[idx+1:]...)
	return w.Save()
}

// GetRepo returns a repository by name.
func (w *Workspace) GetRepo(name string) *Repo {
	for i := range w.Repos {
		if w.Repos[i].Name == name {
			return &w.Repos[i]
		}
	}
	return nil
}

// SetPrimary sets a repository as the primary repo.
func (w *Workspace) SetPrimary(name string) error {
	found := false
	for i := range w.Repos {
		if w.Repos[i].Name == name {
			w.Repos[i].IsPrimary = true
			found = true
		} else {
			w.Repos[i].IsPrimary = false
		}
	}

	if !found {
		return fmt.Errorf("repository %q not found in workspace", name)
	}

	return w.Save()
}

// PrimaryRepo returns the primary repository, or nil if none set.
func (w *Workspace) PrimaryRepo() *Repo {
	for i := range w.Repos {
		if w.Repos[i].IsPrimary {
			return &w.Repos[i]
		}
	}
	// If no primary, return first repo
	if len(w.Repos) > 0 {
		return &w.Repos[0]
	}
	return nil
}

// ReposDir returns the path to the repos directory.
func (w *Workspace) ReposDir() string {
	return filepath.Join(w.Path, ConfigDir, "repos")
}

// SessionsDir returns the path to the sessions directory.
func (w *Workspace) SessionsDir() string {
	return filepath.Join(w.Path, ConfigDir, "sessions")
}

// LogsDir returns the path to the logs directory.
func (w *Workspace) LogsDir() string {
	return filepath.Join(w.Path, ConfigDir, "logs")
}

// WorktreesDir returns the path to the worktrees directory.
func (w *Workspace) WorktreesDir() string {
	return filepath.Join(w.Path, ConfigDir, "worktrees")
}

// UpdateRepo updates a repository's metadata.
func (w *Workspace) UpdateRepo(name string, updater func(*Repo)) error {
	for i := range w.Repos {
		if w.Repos[i].Name == name {
			updater(&w.Repos[i])
			return w.Save()
		}
	}
	return fmt.Errorf("repository %q not found", name)
}
