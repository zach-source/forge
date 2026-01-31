package workspace

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Worktree represents an active git worktree for feature development.
type Worktree struct {
	Name       string    `yaml:"name"`
	RepoName   string    `yaml:"repo_name"`
	Branch     string    `yaml:"branch"`
	BaseBranch string    `yaml:"base_branch"`
	Path       string    `yaml:"path"`
	CreatedAt  time.Time `yaml:"created_at"`
	BeadID     string    `yaml:"bead_id,omitempty"`
	NotionID   string    `yaml:"notion_id,omitempty"`
	Status     string    `yaml:"status"` // active, merged, abandoned
}

// WorktreeConfig stores worktree state for the workspace.
type WorktreeConfig struct {
	Worktrees []Worktree `yaml:"worktrees"`
}

const worktreeConfigFile = "worktrees.yaml"

// CreateWorktree creates a new git worktree for feature development.
func (w *Workspace) CreateWorktree(repoName, featureName, baseBranch string) (*Worktree, error) {
	repo := w.GetRepo(repoName)
	if repo == nil {
		return nil, fmt.Errorf("repository %q not found", repoName)
	}

	// Default base branch
	if baseBranch == "" {
		baseBranch = repo.DevBranch
		if baseBranch == "" {
			baseBranch = "main"
		}
	}

	// Generate branch name
	branchName := fmt.Sprintf("feature/%s", sanitizeBranchName(featureName))

	// Worktree path
	worktreePath := filepath.Join(w.WorktreesDir(), repoName, sanitizeBranchName(featureName))

	// Ensure worktrees directory exists
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0755); err != nil {
		return nil, fmt.Errorf("creating worktrees directory: %w", err)
	}

	// First, fetch to ensure we have latest
	fetchCmd := exec.Command("git", "-C", repo.Path, "fetch", "origin", baseBranch)
	if err := fetchCmd.Run(); err != nil {
		// Non-fatal, continue
		fmt.Printf("Warning: failed to fetch: %v\n", err)
	}

	// Create the worktree with a new branch based on origin/<baseBranch>
	args := []string{
		"-C", repo.Path,
		"worktree", "add",
		"-b", branchName,
		worktreePath,
		fmt.Sprintf("origin/%s", baseBranch),
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("creating worktree: %w\n%s", err, string(output))
	}

	wt := &Worktree{
		Name:       featureName,
		RepoName:   repoName,
		Branch:     branchName,
		BaseBranch: baseBranch,
		Path:       worktreePath,
		CreatedAt:  time.Now(),
		Status:     "active",
	}

	// Save to config
	if err := w.saveWorktree(wt); err != nil {
		return nil, err
	}

	return wt, nil
}

// ListWorktrees returns all worktrees in the workspace.
func (w *Workspace) ListWorktrees() ([]Worktree, error) {
	config, err := w.loadWorktreeConfig()
	if err != nil {
		return nil, err
	}
	return config.Worktrees, nil
}

// GetWorktree returns a worktree by name.
func (w *Workspace) GetWorktree(name string) (*Worktree, error) {
	config, err := w.loadWorktreeConfig()
	if err != nil {
		return nil, err
	}

	for i := range config.Worktrees {
		if config.Worktrees[i].Name == name {
			return &config.Worktrees[i], nil
		}
	}

	return nil, fmt.Errorf("worktree %q not found", name)
}

// RemoveWorktree removes a worktree.
func (w *Workspace) RemoveWorktree(name string, deleteBranch bool) error {
	wt, err := w.GetWorktree(name)
	if err != nil {
		return err
	}

	repo := w.GetRepo(wt.RepoName)
	if repo == nil {
		return fmt.Errorf("repository %q not found", wt.RepoName)
	}

	// Remove the git worktree
	cmd := exec.Command("git", "-C", repo.Path, "worktree", "remove", wt.Path, "--force")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("removing worktree: %w\n%s", err, string(output))
	}

	// Optionally delete the branch
	if deleteBranch {
		cmd := exec.Command("git", "-C", repo.Path, "branch", "-D", wt.Branch)
		if err := cmd.Run(); err != nil {
			fmt.Printf("Warning: failed to delete branch %s: %v\n", wt.Branch, err)
		}
	}

	// Update config
	return w.removeWorktreeFromConfig(name)
}

// UpdateWorktreeStatus updates the status of a worktree.
func (w *Workspace) UpdateWorktreeStatus(name, status string) error {
	config, err := w.loadWorktreeConfig()
	if err != nil {
		return err
	}

	for i := range config.Worktrees {
		if config.Worktrees[i].Name == name {
			config.Worktrees[i].Status = status
			return w.saveWorktreeConfig(config)
		}
	}

	return fmt.Errorf("worktree %q not found", name)
}

// LinkWorktreeToNotion links a worktree to a Notion page.
func (w *Workspace) LinkWorktreeToNotion(name, notionID string) error {
	config, err := w.loadWorktreeConfig()
	if err != nil {
		return err
	}

	for i := range config.Worktrees {
		if config.Worktrees[i].Name == name {
			config.Worktrees[i].NotionID = notionID
			return w.saveWorktreeConfig(config)
		}
	}

	return fmt.Errorf("worktree %q not found", name)
}

// LinkWorktreeToBead links a worktree to a bead.
func (w *Workspace) LinkWorktreeToBead(name, beadID string) error {
	config, err := w.loadWorktreeConfig()
	if err != nil {
		return err
	}

	for i := range config.Worktrees {
		if config.Worktrees[i].Name == name {
			config.Worktrees[i].BeadID = beadID
			return w.saveWorktreeConfig(config)
		}
	}

	return fmt.Errorf("worktree %q not found", name)
}

// ActiveWorktrees returns all active worktrees.
func (w *Workspace) ActiveWorktrees() ([]Worktree, error) {
	all, err := w.ListWorktrees()
	if err != nil {
		return nil, err
	}

	var active []Worktree
	for _, wt := range all {
		if wt.Status == "active" {
			active = append(active, wt)
		}
	}
	return active, nil
}

// worktree config helpers

func (w *Workspace) worktreeConfigPath() string {
	return filepath.Join(w.WorktreesDir(), worktreeConfigFile)
}

func (w *Workspace) loadWorktreeConfig() (*WorktreeConfig, error) {
	path := w.worktreeConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &WorktreeConfig{}, nil
		}
		return nil, fmt.Errorf("reading worktree config: %w", err)
	}

	var config WorktreeConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing worktree config: %w", err)
	}

	return &config, nil
}

func (w *Workspace) saveWorktreeConfig(config *WorktreeConfig) error {
	path := w.worktreeConfigPath()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating worktrees directory: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshaling worktree config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing worktree config: %w", err)
	}

	return nil
}

func (w *Workspace) saveWorktree(wt *Worktree) error {
	config, err := w.loadWorktreeConfig()
	if err != nil {
		return err
	}

	// Check for duplicates
	for _, existing := range config.Worktrees {
		if existing.Name == wt.Name {
			return fmt.Errorf("worktree %q already exists", wt.Name)
		}
	}

	config.Worktrees = append(config.Worktrees, *wt)
	return w.saveWorktreeConfig(config)
}

func (w *Workspace) removeWorktreeFromConfig(name string) error {
	config, err := w.loadWorktreeConfig()
	if err != nil {
		return err
	}

	idx := -1
	for i, wt := range config.Worktrees {
		if wt.Name == name {
			idx = i
			break
		}
	}

	if idx == -1 {
		return nil // Already gone
	}

	config.Worktrees = append(config.Worktrees[:idx], config.Worktrees[idx+1:]...)
	return w.saveWorktreeConfig(config)
}

// sanitizeBranchName converts a feature name to a valid git branch name.
func sanitizeBranchName(name string) string {
	// Replace spaces and special chars with dashes
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")

	// Remove invalid characters
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}

	// Collapse multiple dashes
	s := result.String()
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}

	return strings.Trim(s, "-")
}
