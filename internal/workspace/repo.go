package workspace

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CloneRepo clones a repository into the workspace.
func (w *Workspace) CloneRepo(remote, name, branch string) (*Repo, error) {
	if name == "" {
		// Extract name from remote URL
		name = extractRepoName(remote)
	}

	// Determine clone path
	clonePath := filepath.Join(w.ReposDir(), name)

	// Check if path already exists
	if _, err := os.Stat(clonePath); err == nil {
		return nil, fmt.Errorf("directory already exists: %s", clonePath)
	}

	// Build clone command
	args := []string{"clone"}
	if branch != "" {
		args = append(args, "-b", branch)
	}
	args = append(args, remote, clonePath)

	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("cloning repository: %w", err)
	}

	// Get the actual branch if not specified
	if branch == "" {
		branch = getCurrentBranch(clonePath)
	}

	repo := Repo{
		Name:     name,
		Path:     clonePath,
		Remote:   remote,
		Branch:   branch,
		IsLinked: false,
	}

	if err := w.AddRepo(repo); err != nil {
		return nil, err
	}

	return &repo, nil
}

// LinkRepo links an existing repository to the workspace.
func (w *Workspace) LinkRepo(path, name string) (*Repo, error) {
	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}

	// Verify it's a git repository
	gitDir := filepath.Join(absPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("not a git repository: %s", absPath)
	}

	if name == "" {
		name = filepath.Base(absPath)
	}

	// Get remote and branch info
	remote := getRemoteURL(absPath)
	branch := getCurrentBranch(absPath)

	repo := Repo{
		Name:     name,
		Path:     absPath,
		Remote:   remote,
		Branch:   branch,
		IsLinked: true,
	}

	if err := w.AddRepo(repo); err != nil {
		return nil, err
	}

	return &repo, nil
}

// extractRepoName extracts the repository name from a git remote URL.
func extractRepoName(remote string) string {
	// Handle SSH URLs: git@github.com:user/repo.git
	if strings.Contains(remote, ":") && !strings.Contains(remote, "://") {
		parts := strings.Split(remote, ":")
		if len(parts) == 2 {
			remote = parts[1]
		}
	}

	// Handle HTTPS URLs: https://github.com/user/repo.git
	remote = strings.TrimSuffix(remote, ".git")
	parts := strings.Split(remote, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return "repo"
}

// getCurrentBranch returns the current branch of a git repository.
func getCurrentBranch(repoPath string) string {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "main"
	}
	return strings.TrimSpace(string(output))
}

// getRemoteURL returns the origin remote URL of a git repository.
func getRemoteURL(repoPath string) string {
	cmd := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// Pull pulls the latest changes for a repository.
func (w *Workspace) Pull(name string) error {
	repo := w.GetRepo(name)
	if repo == nil {
		return fmt.Errorf("repository %q not found", name)
	}

	cmd := exec.Command("git", "-C", repo.Path, "pull")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// Status returns the git status of a repository.
func (w *Workspace) Status(name string) (string, error) {
	repo := w.GetRepo(name)
	if repo == nil {
		return "", fmt.Errorf("repository %q not found", name)
	}

	cmd := exec.Command("git", "-C", repo.Path, "status", "--short")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// FetchAll fetches all repositories in the workspace.
func (w *Workspace) FetchAll() error {
	for _, repo := range w.Repos {
		fmt.Printf("Fetching %s...\n", repo.Name)
		cmd := exec.Command("git", "-C", repo.Path, "fetch", "--all", "--prune")
		if err := cmd.Run(); err != nil {
			fmt.Printf("  Warning: failed to fetch %s: %v\n", repo.Name, err)
		}
	}
	return nil
}
