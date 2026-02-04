// Package github provides GitHub Projects configuration for forge.
package github

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config represents GitHub Projects configuration.
type Config struct {
	// Owner is the GitHub user or organization
	Owner string `yaml:"owner"`
	// Repo is the repository name (optional, for repo-level projects)
	Repo string `yaml:"repo,omitempty"`
	// ProjectNumber is the project number (visible in project URL)
	ProjectNumber int `yaml:"project_number"`
}

// FullConfig represents the full forge config file with GitHub section.
type FullConfig struct {
	GitHub Config `yaml:"github"`
}

// configPath returns the path to the forge config file.
func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".forge", "config.yaml")
}

// Load loads GitHub configuration from the config file.
func Load() (*Config, error) {
	path := configPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no config file found; run 'forge board --github --config' to set up")
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var full FullConfig
	if err := yaml.Unmarshal(data, &full); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if full.GitHub.Owner == "" {
		return nil, fmt.Errorf("github.owner not configured; run 'forge board --github --config'")
	}
	if full.GitHub.ProjectNumber == 0 {
		return nil, fmt.Errorf("github.project_number not configured; run 'forge board --github --config'")
	}

	return &full.GitHub, nil
}

// Save saves GitHub configuration to the config file.
func Save(cfg *Config) error {
	path := configPath()

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	// Load existing config or create new
	var full struct {
		Notion interface{} `yaml:"notion,omitempty"`
		GitHub Config      `yaml:"github"`
	}

	if data, err := os.ReadFile(path); err == nil {
		_ = yaml.Unmarshal(data, &full)
	}

	// Update GitHub config
	full.GitHub = *cfg

	// Write back
	data, err := yaml.Marshal(&full)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// HasToken returns true if GITHUB_TOKEN is set in the environment.
func HasToken() bool {
	return os.Getenv("GITHUB_TOKEN") != ""
}

// ValidateConfig validates the GitHub configuration.
func ValidateConfig(owner string, projectNumber int) error {
	if owner == "" {
		return fmt.Errorf("owner cannot be empty")
	}
	if projectNumber <= 0 {
		return fmt.Errorf("project number must be positive")
	}
	return nil
}

// ParseProjectURL extracts owner and project number from a GitHub Projects URL.
// Supports URLs like:
//   - https://github.com/users/USERNAME/projects/1
//   - https://github.com/orgs/ORGNAME/projects/1
//   - https://github.com/OWNER/REPO/projects/1
func ParseProjectURL(url string) (owner string, repo string, projectNumber int, err error) {
	// Remove trailing slash
	url = trimSuffix(url, "/")

	// Try to extract from URL patterns
	patterns := []struct {
		prefix  string
		hasOrg  bool
		hasUser bool
		hasRepo bool
	}{
		{"https://github.com/orgs/", true, false, false},
		{"https://github.com/users/", false, true, false},
		{"https://github.com/", false, false, true},
	}

	for _, p := range patterns {
		if hasPrefix(url, p.prefix) {
			rest := url[len(p.prefix):]
			parts := splitN(rest, "/", 4)

			if p.hasOrg || p.hasUser {
				// Format: orgs/OWNER/projects/N or users/OWNER/projects/N
				if len(parts) >= 3 && parts[1] == "projects" {
					owner = parts[0]
					projectNumber, err = strconv.Atoi(parts[2])
					if err != nil {
						return "", "", 0, fmt.Errorf("invalid project number: %s", parts[2])
					}
					return owner, "", projectNumber, nil
				}
			} else if p.hasRepo {
				// Format: OWNER/REPO/projects/N
				if len(parts) >= 4 && parts[2] == "projects" {
					owner = parts[0]
					repo = parts[1]
					projectNumber, err = strconv.Atoi(parts[3])
					if err != nil {
						return "", "", 0, fmt.Errorf("invalid project number: %s", parts[3])
					}
					return owner, repo, projectNumber, nil
				}
			}
		}
	}

	return "", "", 0, fmt.Errorf("could not parse GitHub project URL: %s", url)
}

// ProjectURL returns the URL for a GitHub project.
func (c *Config) ProjectURL() string {
	if c.Repo != "" {
		return fmt.Sprintf("https://github.com/%s/%s/projects/%d", c.Owner, c.Repo, c.ProjectNumber)
	}
	return fmt.Sprintf("https://github.com/users/%s/projects/%d", c.Owner, c.ProjectNumber)
}

// Helper functions to avoid importing strings package for simple ops
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func trimSuffix(s, suffix string) string {
	if len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix {
		return s[:len(s)-len(suffix)]
	}
	return s
}

func splitN(s, sep string, n int) []string {
	var result []string
	for i := 0; i < n-1; i++ {
		idx := indexOf(s, sep)
		if idx < 0 {
			break
		}
		result = append(result, s[:idx])
		s = s[idx+len(sep):]
	}
	result = append(result, s)
	return result
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
