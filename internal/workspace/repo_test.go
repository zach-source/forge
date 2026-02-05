package workspace

import "testing"

func TestExtractRepoName(t *testing.T) {
	tests := []struct {
		name     string
		remote   string
		expected string
	}{
		{
			name:     "https URL with .git suffix",
			remote:   "https://github.com/user/my-repo.git",
			expected: "my-repo",
		},
		{
			name:     "https URL without .git suffix",
			remote:   "https://github.com/user/my-repo",
			expected: "my-repo",
		},
		{
			name:     "SSH URL with .git suffix",
			remote:   "git@github.com:user/my-repo.git",
			expected: "my-repo",
		},
		{
			name:     "SSH URL without .git suffix",
			remote:   "git@github.com:user/my-repo",
			expected: "my-repo",
		},
		{
			name:     "https with organization path",
			remote:   "https://github.com/org/team/project.git",
			expected: "project",
		},
		{
			name:     "GitLab SSH URL",
			remote:   "git@gitlab.com:group/subgroup/repo.git",
			expected: "repo",
		},
		{
			name:     "simple name only",
			remote:   "repo-name",
			expected: "repo-name",
		},
		{
			name:     "empty string returns empty",
			remote:   "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRepoName(tt.remote)
			if result != tt.expected {
				t.Errorf("extractRepoName(%q) = %q, want %q", tt.remote, result, tt.expected)
			}
		})
	}
}
