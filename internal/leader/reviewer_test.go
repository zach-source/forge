package leader

import (
	"strings"
	"testing"

	"github.com/zach-source/forge/internal/github"
)

func TestDraftPRSection_Empty(t *testing.T) {
	result := DraftPRSection(nil)
	if result != "" {
		t.Errorf("DraftPRSection(nil) = %q, want empty string", result)
	}

	result = DraftPRSection([]github.PRState{})
	if result != "" {
		t.Errorf("DraftPRSection([]) = %q, want empty string", result)
	}
}

func TestDraftPRSection_WithPRs(t *testing.T) {
	prs := []github.PRState{
		{
			Number: 42,
			Title:  "WIP: Add feature",
			Branch: "task/abc",
			URL:    "https://github.com/owner/repo/pull/42",
			Draft:  true,
		},
		{
			Number: 99,
			Title:  "WIP: Fix bug",
			Branch: "task/xyz",
			URL:    "https://github.com/owner/repo/pull/99",
			Draft:  true,
		},
	}

	result := DraftPRSection(prs)

	expectedSubstrings := []string{
		"## Draft PRs (Early Feedback)",
		"guidance, not blocking feedback",
		"#42",
		"WIP: Add feature",
		"task/abc",
		"#99",
		"WIP: Fix bug",
		"task/xyz",
	}

	for _, expected := range expectedSubstrings {
		if !strings.Contains(result, expected) {
			t.Errorf("DraftPRSection() missing %q", expected)
		}
	}
}

func TestReviewerPromptContents(t *testing.T) {
	prompt := ReviewerPrompt("db-123", "/work", "main")

	expectedSubstrings := []string{
		"Forge Reviewer",
		"db-123",
		"/work",
		"main",
		"Review Checklist",
	}

	for _, expected := range expectedSubstrings {
		if !strings.Contains(prompt, expected) {
			t.Errorf("ReviewerPrompt() missing %q", expected)
		}
	}
}
