// Package leader provides prompt loading and management for leader agents.
package leader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zach-source/forge/internal/leader/defaults"
)

// PromptLoader handles loading leader prompts from files or defaults.
type PromptLoader struct {
	workDir string
}

// NewPromptLoader creates a new prompt loader for the given workspace.
func NewPromptLoader(workDir string) *PromptLoader {
	return &PromptLoader{workDir: workDir}
}

// promptDir returns the path to the prompts directory.
func (p *PromptLoader) promptDir() string {
	return filepath.Join(p.workDir, ".forge", "prompts")
}

// PromptPath returns the path to a prompt file for a role.
func (p *PromptLoader) PromptPath(role string) string {
	return filepath.Join(p.promptDir(), role+".md")
}

// Load loads a prompt for the given role from file or returns empty string.
// The caller should fall back to built-in prompts if empty.
func (p *PromptLoader) Load(role string) (string, error) {
	path := p.PromptPath(role)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // No custom prompt, use default
		}
		return "", fmt.Errorf("reading prompt file: %w", err)
	}
	return string(data), nil
}

// LoadWithVars loads a prompt and replaces template variables.
// Variables are in the format {{.VarName}} and are replaced with values from vars map.
func (p *PromptLoader) LoadWithVars(role string, vars map[string]string) (string, error) {
	content, err := p.Load(role)
	if err != nil {
		return "", err
	}
	if content == "" {
		return "", nil
	}

	// Replace template variables
	for key, value := range vars {
		placeholder := fmt.Sprintf("{{.%s}}", key)
		content = strings.ReplaceAll(content, placeholder, value)
	}

	return content, nil
}

// Exists checks if a custom prompt file exists for the given role.
func (p *PromptLoader) Exists(role string) bool {
	path := p.PromptPath(role)
	_, err := os.Stat(path)
	return err == nil
}

// EnsureDir creates the prompts directory if it doesn't exist.
func (p *PromptLoader) EnsureDir() error {
	return os.MkdirAll(p.promptDir(), 0755)
}

// Save saves a prompt to a file for the given role.
func (p *PromptLoader) Save(role, content string) error {
	if err := p.EnsureDir(); err != nil {
		return fmt.Errorf("creating prompts directory: %w", err)
	}

	path := p.PromptPath(role)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing prompt file: %w", err)
	}

	return nil
}

// ListCustomPrompts returns a list of roles that have custom prompt files.
func (p *PromptLoader) ListCustomPrompts() ([]string, error) {
	dir := p.promptDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var roles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".md") {
			roles = append(roles, strings.TrimSuffix(name, ".md"))
		}
	}

	return roles, nil
}

// InitDefaults initializes the prompts directory with default prompt files.
// If force is true, existing files will be overwritten.
// Returns the list of roles that were initialized.
func (p *PromptLoader) InitDefaults(force bool) ([]string, error) {
	if err := p.EnsureDir(); err != nil {
		return nil, fmt.Errorf("creating prompts directory: %w", err)
	}

	var initialized []string
	for _, role := range defaults.AllRoles() {
		// Check if already exists
		if !force && p.Exists(role) {
			continue
		}

		// Read default prompt
		content, err := defaults.Prompts.ReadFile(role + ".md")
		if err != nil {
			return initialized, fmt.Errorf("reading default prompt for %s: %w", role, err)
		}

		// Write to workspace
		if err := p.Save(role, string(content)); err != nil {
			return initialized, fmt.Errorf("saving prompt for %s: %w", role, err)
		}

		initialized = append(initialized, role)
	}

	return initialized, nil
}

// InitDefaultsWithContext initializes prompts with project-specific context.
// The projectContext string replaces {{.ProjectContext}} in templates.
func (p *PromptLoader) InitDefaultsWithContext(projectContext string, force bool) ([]string, error) {
	if err := p.EnsureDir(); err != nil {
		return nil, fmt.Errorf("creating prompts directory: %w", err)
	}

	var initialized []string
	for _, role := range defaults.AllRoles() {
		// Check if already exists
		if !force && p.Exists(role) {
			continue
		}

		// Read default prompt
		content, err := defaults.Prompts.ReadFile(role + ".md")
		if err != nil {
			return initialized, fmt.Errorf("reading default prompt for %s: %w", role, err)
		}

		// Replace project context placeholder
		promptContent := strings.ReplaceAll(string(content), "{{.ProjectContext}}", projectContext)

		// Write to workspace
		if err := p.Save(role, promptContent); err != nil {
			return initialized, fmt.Errorf("saving prompt for %s: %w", role, err)
		}

		initialized = append(initialized, role)
	}

	return initialized, nil
}

// GetDefaultPrompt returns the embedded default prompt for a role.
func GetDefaultPrompt(role string) (string, error) {
	content, err := defaults.Prompts.ReadFile(role + ".md")
	if err != nil {
		return "", fmt.Errorf("reading default prompt for %s: %w", role, err)
	}
	return string(content), nil
}

// AllRoles returns the list of all available leader roles.
func AllRoles() []string {
	return defaults.AllRoles()
}

// DefaultPromptVars returns common variables for prompt templates.
func DefaultPromptVars(workDir string) map[string]string {
	return map[string]string{
		"WorkDir":         workDir,
		"PromiseFormat":   "When finished, output EXACTLY this text (including the XML tags):\n```\n<promise>{{.Promise}}</promise>\n```",
		"OutputFormat":    OutputFormat,
		"HandoffProtocol": HandoffProtocol,
		"ErrorHandling":   ErrorHandlingGuidance,
	}
}

// GraphitiContextSection returns the standard Graphiti context loading section.
const GraphitiContextSection = `
## Loading Context from Memory

At the start of your session, load relevant context from Graphiti:

` + "```" + `
# Search for project context
search_nodes({ query: "project goals roadmap" })

# Search for recent work and handoffs
search_memory_facts({ query: "forge-handoff" })

# Search for known issues or blockers
search_memory_facts({ query: "blocker issue error" })
` + "```" + `

Use this context to inform your decisions and avoid repeating past mistakes.
`

// GraphitiSaveSection returns the standard Graphiti save section.
const GraphitiSaveSection = `
## Saving to Memory

Before completing, save important learnings to Graphiti:

` + "```" + `
add_memory({
  group_id: "forge-{{.Role}}",
  content: ` + "`" + `
    ROLE: {{.Role}}
    SESSION: [timestamp]
    LEARNINGS: [key insights from this session]
    BLOCKERS: [any issues encountered]
    RECOMMENDATIONS: [suggestions for future sessions]
  ` + "`" + `
})
` + "```" + `
`
