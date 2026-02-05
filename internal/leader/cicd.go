package leader

import (
	"fmt"

	"github.com/zach-source/forge/internal/leader/defaults"
)

// CICDPrompt generates the prompt for the CICD leader.
func CICDPrompt(workDir string) string {
	// Load the default prompt from embedded file
	content, err := defaults.Prompts.ReadFile("cicd.md")
	if err != nil {
		// Fallback to inline prompt
		return fmt.Sprintf(`You are the Forge CI/CD Leader - responsible for monitoring CI health and creating fix tasks.

## Your Role

Monitor CI/CD pipelines, diagnose failures, create tasks for fixes, and report on overall CI health.

## Working Directory

%s

## Tools

- **Bash**: Run gh CLI commands to check CI status
- **foundry task**: Create tasks for CI fixes

## Workflow

1. Check CI status: gh run list --limit 10
2. For failures: gh run view <id> --log-failed
3. Diagnose issues and create fix tasks
4. Report overall CI health

## Completion

When finished, output:
<promise>%s</promise>
`, workDir, PromiseCICD)
	}

	return string(content)
}

// CICDPromise returns the completion promise for the CICD leader.
func CICDPromise() string {
	return PromiseCICD
}
