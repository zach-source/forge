// Package theme provides styling for the forge TUI.
package theme

import (
	"github.com/charmbracelet/lipgloss"
)

// Colors
var (
	Primary   = lipgloss.Color("#7C3AED") // Purple
	Secondary = lipgloss.Color("#10B981") // Green
	Accent    = lipgloss.Color("#F59E0B") // Amber
	Error     = lipgloss.Color("#EF4444") // Red
	Muted     = lipgloss.Color("#6B7280") // Gray
	Text      = lipgloss.Color("#F3F4F6") // Light gray
	Subtle    = lipgloss.Color("#374151") // Dark gray
)

// Styles
var (
	// Title style
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary)

	// Header style for the top bar
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Background(Subtle).
			Foreground(Text).
			Padding(0, 1)

	// Selected item style
	SelectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary)

	// Normal item style
	NormalStyle = lipgloss.NewStyle().
			Foreground(Text)

	// Muted text style
	MutedStyle = lipgloss.NewStyle().
			Foreground(Muted)

	// Success style
	SuccessStyle = lipgloss.NewStyle().
			Foreground(Secondary)

	// Error style
	ErrorStyle = lipgloss.NewStyle().
			Foreground(Error)

	// Warning style
	WarningStyle = lipgloss.NewStyle().
			Foreground(Accent)

	// Box style for panels
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Subtle).
			Padding(0, 1)

	// Help style for keybindings
	HelpStyle = lipgloss.NewStyle().
			Foreground(Muted)

	// StatusBar style
	StatusBarStyle = lipgloss.NewStyle().
			Background(Subtle).
			Foreground(Text).
			Padding(0, 1)
)

// StatusColor returns the appropriate color for a status.
func StatusColor(status string) lipgloss.Color {
	switch status {
	case "active":
		return Secondary
	case "completed":
		return Primary
	case "cancelled", "error":
		return Error
	case "paused":
		return Accent
	default:
		return Muted
	}
}
