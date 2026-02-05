package kanban

import (
	"os/exec"
	"strings"
)

// TaskBranch holds branch info linked to kanban.
type TaskBranch struct {
	Branch  string
	TaskID  string
	Title   string
	Status  Status
	HasDiff bool // Whether branch differs from main
}

// ExtractBranchSuffix extracts the branch suffix from a task ID.
// Task IDs are like "blocks-forge-0vk" and branches are "task/0vk".
// Returns the part after the last hyphen.
func ExtractBranchSuffix(taskID string) string {
	lastHyphen := strings.LastIndex(taskID, "-")
	if lastHyphen >= 0 && lastHyphen < len(taskID)-1 {
		return taskID[lastHyphen+1:]
	}
	// If no hyphen, use the whole ID (might be a short ID)
	if len(taskID) <= 8 {
		return taskID
	}
	return taskID[:8]
}

// FindTaskBranch finds a git branch matching the task suffix.
func FindTaskBranch(suffix string) (string, error) {
	// Try exact match first
	branch := "task/" + suffix
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", branch)
	if err := cmd.Run(); err == nil {
		return branch, nil
	}

	// Try to find branch containing suffix
	out, err := exec.Command("git", "branch", "--list", "task/*"+suffix+"*").Output()
	if err != nil {
		return "", err
	}

	branches := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, b := range branches {
		b = strings.TrimSpace(b)
		b = strings.TrimPrefix(b, "* ")
		if b != "" {
			return b, nil
		}
	}

	return "", nil
}

// ListTaskBranches returns all task/* branches with their statuses.
func ListTaskBranches(store *Store) ([]TaskBranch, error) {
	// Get all task branches
	out, err := exec.Command("git", "branch", "--list", "task/*").Output()
	if err != nil {
		return nil, err
	}

	branches := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(branches) == 0 || (len(branches) == 1 && branches[0] == "") {
		return nil, nil
	}

	// Get all issues for lookup
	allIssues, err := store.List()
	if err != nil {
		return nil, err
	}

	// Build suffix -> issue map
	issueMap := make(map[string]*Issue)
	for _, issue := range allIssues {
		suffix := ExtractBranchSuffix(issue.ID)
		issueMap[suffix] = issue
	}

	var result []TaskBranch
	for _, branch := range branches {
		branch = strings.TrimSpace(branch)
		branch = strings.TrimPrefix(branch, "* ") // Remove current branch marker

		if branch == "" {
			continue
		}

		// Extract suffix from branch name (task/abc -> abc)
		suffix := strings.TrimPrefix(branch, "task/")

		// Find matching issue
		issue := issueMap[suffix]

		// Check if branch has unmerged commits
		hasDiff := false
		diffCmd := exec.Command("git", "rev-list", "--count", "main.."+branch)
		if diffOut, err := diffCmd.Output(); err == nil {
			count := strings.TrimSpace(string(diffOut))
			hasDiff = count != "0"
		}

		tb := TaskBranch{
			Branch:  branch,
			HasDiff: hasDiff,
		}

		if issue != nil {
			tb.TaskID = issue.ID
			tb.Title = issue.Title
			tb.Status = issue.Status
		}

		result = append(result, tb)
	}

	return result, nil
}

// BranchHasDiff checks if a branch has commits not in main.
func BranchHasDiff(branch string) bool {
	cmd := exec.Command("git", "rev-list", "--count", "main.."+branch)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	count := strings.TrimSpace(string(out))
	return count != "0"
}

// FindTask finds a task by exact or partial ID match.
func FindTask(store *Store, partialID string) (*Issue, error) {
	// Try exact match first
	issue, err := store.Get(partialID)
	if err != nil {
		return nil, err
	}
	if issue != nil {
		return issue, nil
	}

	// Try to find by suffix match
	allIssues, err := store.List()
	if err != nil {
		return nil, err
	}

	for _, issue := range allIssues {
		// Check if ID ends with the partial
		if strings.HasSuffix(issue.ID, partialID) {
			return issue, nil
		}
		// Check if partial matches the suffix after last hyphen
		suffix := ExtractBranchSuffix(issue.ID)
		if suffix == partialID {
			return issue, nil
		}
	}

	return nil, nil
}
