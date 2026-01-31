package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

// ConflictCheck checks for workspace conflicts.
type ConflictCheck struct {
	HasParentWorkspace bool
	ParentWorkspace    string
	HasChildWorkspace  bool
	ChildWorkspaces    []string
}

// CheckConflicts checks for existing workspaces that would conflict.
func CheckConflicts(dir string) (*ConflictCheck, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}

	check := &ConflictCheck{}

	// Check for parent workspaces (walk up)
	parentDir := filepath.Dir(absDir)
	for parentDir != absDir {
		configPath := filepath.Join(parentDir, ConfigDir, ConfigFile)
		if _, err := os.Stat(configPath); err == nil {
			check.HasParentWorkspace = true
			check.ParentWorkspace = parentDir
			break
		}
		absDir = parentDir
		parentDir = filepath.Dir(absDir)
	}

	// Check for child workspaces (walk down, max 3 levels)
	absDir, _ = filepath.Abs(dir)
	check.ChildWorkspaces = findChildWorkspaces(absDir, 3)
	check.HasChildWorkspace = len(check.ChildWorkspaces) > 0

	return check, nil
}

// findChildWorkspaces recursively finds workspace configs in subdirectories.
func findChildWorkspaces(dir string, maxDepth int) []string {
	if maxDepth <= 0 {
		return nil
	}

	var workspaces []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Skip hidden directories and common non-project dirs
		if name[0] == '.' || name == "node_modules" || name == "vendor" || name == "__pycache__" {
			continue
		}

		subDir := filepath.Join(dir, name)

		// Check if this is a workspace
		configPath := filepath.Join(subDir, ConfigDir, ConfigFile)
		if _, err := os.Stat(configPath); err == nil {
			workspaces = append(workspaces, subDir)
			continue // Don't recurse into existing workspaces
		}

		// Recurse
		childWorkspaces := findChildWorkspaces(subDir, maxDepth-1)
		workspaces = append(workspaces, childWorkspaces...)
	}

	return workspaces
}

// IsInsideWorkspace checks if a directory is inside a forge workspace.
func IsInsideWorkspace(dir string) bool {
	_, err := Find(dir)
	return err == nil
}

// ListGlobalWorkspaces finds all forge workspaces in common locations.
func ListGlobalWorkspaces() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	// Common workspace locations
	searchDirs := []string{
		filepath.Join(home, "workspaces"),
		filepath.Join(home, "repos"),
		filepath.Join(home, "projects"),
		filepath.Join(home, "code"),
		filepath.Join(home, "dev"),
	}

	var workspaces []string

	for _, searchDir := range searchDirs {
		if _, err := os.Stat(searchDir); os.IsNotExist(err) {
			continue
		}

		found := findChildWorkspaces(searchDir, 2)
		workspaces = append(workspaces, found...)
	}

	return workspaces, nil
}
