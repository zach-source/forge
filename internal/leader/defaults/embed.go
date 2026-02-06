// Package defaults provides embedded default leader prompts.
package defaults

import "embed"

//go:embed *.md
var Prompts embed.FS

// AllRoles returns the list of all leader roles with default prompts.
func AllRoles() []string {
	return []string{
		"planner",
		"reviewer",
		"groomer",
		"merge",
		"deploy",
		"monitor",
		"tester",
		"pm",
		"cicd",
		"team-lead",
	}
}
