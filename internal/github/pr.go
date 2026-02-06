package github

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

// PRState represents the state of a GitHub pull request.
type PRState struct {
	Number int    `json:"number"`
	State  string `json:"state"`
	Draft  bool   `json:"isDraft"`
	Title  string `json:"title"`
	Branch string `json:"headRefName"`
	URL    string `json:"url"`
}

// GetPRForBranch returns the PR associated with the given branch, or nil if none exists.
func GetPRForBranch(branch string) (*PRState, error) {
	cmd := exec.Command("gh", "pr", "view", branch,
		"--json", "number,state,isDraft,title,headRefName,url")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("no PR for branch %s: %w", branch, err)
	}

	var pr PRState
	if err := json.Unmarshal(out, &pr); err != nil {
		return nil, fmt.Errorf("parsing PR response: %w", err)
	}

	return &pr, nil
}

// ListDraftPRs returns all open draft PRs in the current repository.
func ListDraftPRs() ([]PRState, error) {
	cmd := exec.Command("gh", "pr", "list",
		"--draft",
		"--json", "number,state,isDraft,title,headRefName,url")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing draft PRs: %w", err)
	}

	var prs []PRState
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil, fmt.Errorf("parsing PR list: %w", err)
	}

	return prs, nil
}
