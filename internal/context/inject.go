package context

import (
	"fmt"
	"strings"
)

// InjectOptions configures how learnings are injected into prompts.
type InjectOptions struct {
	MaxLearnings int  // Maximum number of learnings to inject (default: 3)
	IncludeFiles bool // Include file paths in output
	Verbose      bool // Include full problem/solution details
}

// DefaultInjectOptions returns sensible defaults for injection.
func DefaultInjectOptions() InjectOptions {
	return InjectOptions{
		MaxLearnings: 3,
		IncludeFiles: true,
		Verbose:      true,
	}
}

// InjectLearnings generates a prompt section with relevant learnings.
func InjectLearnings(learnings []Learning, opts InjectOptions) string {
	if len(learnings) == 0 {
		return ""
	}

	if opts.MaxLearnings <= 0 {
		opts.MaxLearnings = 3
	}

	// Limit learnings
	if len(learnings) > opts.MaxLearnings {
		learnings = learnings[:opts.MaxLearnings]
	}

	var sb strings.Builder

	sb.WriteString("\n## Relevant Learnings\n\n")
	sb.WriteString("Before you start, consider these past learnings from similar work:\n\n")

	for i, l := range learnings {
		sb.WriteString(fmt.Sprintf("### %d. %s\n", i+1, l.Summary))

		if opts.Verbose {
			if l.Problem != "" {
				sb.WriteString(fmt.Sprintf("**Problem**: %s\n", l.Problem))
			}
			if l.Solution != "" {
				sb.WriteString(fmt.Sprintf("**Solution**: %s\n", l.Solution))
			}
		}

		if opts.IncludeFiles && len(l.Files) > 0 {
			sb.WriteString(fmt.Sprintf("**Files**: %s\n", strings.Join(l.Files, ", ")))
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatLearningForPrompt formats a single learning for inclusion in a prompt.
func FormatLearningForPrompt(l Learning) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("**%s**\n", l.Summary))
	if l.Problem != "" {
		sb.WriteString(fmt.Sprintf("- Problem: %s\n", l.Problem))
	}
	if l.Solution != "" {
		sb.WriteString(fmt.Sprintf("- Solution: %s\n", l.Solution))
	}
	if len(l.Files) > 0 {
		sb.WriteString(fmt.Sprintf("- Files: %s\n", strings.Join(l.Files, ", ")))
	}

	return sb.String()
}

// BuildContextSection builds a complete context section for a worker prompt.
func BuildContextSection(workspaceDir, taskTitle, taskDescription string) string {
	store, err := LoadStore(workspaceDir)
	if err != nil || store == nil {
		return ""
	}

	learnings := FindRelevantLearnings(store, taskTitle, taskDescription, DefaultInjectOptions().MaxLearnings)
	if len(learnings) == 0 {
		return ""
	}

	return InjectLearnings(learnings, DefaultInjectOptions())
}
