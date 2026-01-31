package github

import (
	"testing"
)

func TestParseProjectURL(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		wantOwner  string
		wantRepo   string
		wantNumber int
		wantErr    bool
	}{
		{
			name:       "user project",
			url:        "https://github.com/users/octocat/projects/1",
			wantOwner:  "octocat",
			wantRepo:   "",
			wantNumber: 1,
			wantErr:    false,
		},
		{
			name:       "org project",
			url:        "https://github.com/orgs/github/projects/42",
			wantOwner:  "github",
			wantRepo:   "",
			wantNumber: 42,
			wantErr:    false,
		},
		{
			name:       "repo project",
			url:        "https://github.com/owner/repo/projects/5",
			wantOwner:  "owner",
			wantRepo:   "repo",
			wantNumber: 5,
			wantErr:    false,
		},
		{
			name:       "with trailing slash",
			url:        "https://github.com/users/octocat/projects/1/",
			wantOwner:  "octocat",
			wantRepo:   "",
			wantNumber: 1,
			wantErr:    false,
		},
		{
			name:    "invalid url",
			url:     "https://example.com/projects/1",
			wantErr: true,
		},
		{
			name:    "invalid project number",
			url:     "https://github.com/users/octocat/projects/abc",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, num, err := ParseProjectURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseProjectURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if owner != tt.wantOwner {
					t.Errorf("ParseProjectURL() owner = %v, want %v", owner, tt.wantOwner)
				}
				if repo != tt.wantRepo {
					t.Errorf("ParseProjectURL() repo = %v, want %v", repo, tt.wantRepo)
				}
				if num != tt.wantNumber {
					t.Errorf("ParseProjectURL() number = %v, want %v", num, tt.wantNumber)
				}
			}
		})
	}
}

func TestConfigProjectURL(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   string
	}{
		{
			name:   "user project",
			config: Config{Owner: "octocat", ProjectNumber: 1},
			want:   "https://github.com/users/octocat/projects/1",
		},
		{
			name:   "repo project",
			config: Config{Owner: "owner", Repo: "repo", ProjectNumber: 5},
			want:   "https://github.com/owner/repo/projects/5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.ProjectURL()
			if got != tt.want {
				t.Errorf("Config.ProjectURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		owner   string
		number  int
		wantErr bool
	}{
		{"valid", "octocat", 1, false},
		{"empty owner", "", 1, true},
		{"zero number", "octocat", 0, true},
		{"negative number", "octocat", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.owner, tt.number)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
