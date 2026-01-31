package monitor

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/zach-source/forge/internal/session"
	"github.com/zach-source/forge/internal/theme"
)

// renderView renders the main view.
func (m Model) renderView() string {
	var sb strings.Builder

	// Header
	sb.WriteString(m.renderHeader())
	sb.WriteString("\n")

	// Session list
	sb.WriteString(m.renderSessionList())
	sb.WriteString("\n")

	// Output preview
	sb.WriteString(m.renderOutput())
	sb.WriteString("\n")

	// Help
	sb.WriteString(m.renderHelp())

	return sb.String()
}

// renderHeader renders the header bar.
func (m Model) renderHeader() string {
	sessions := m.sessions.List()
	active := 0
	for _, s := range sessions {
		if s.Status == session.StatusActive {
			active++
		}
	}

	title := theme.TitleStyle.Render("forge monitor")
	count := theme.MutedStyle.Render(fmt.Sprintf("%d sessions", len(sessions)))
	if active > 0 {
		count = theme.SuccessStyle.Render(fmt.Sprintf("%d active", active))
	}

	// Right-align the count
	gap := m.width - lipgloss.Width(title) - lipgloss.Width(count) - 4
	if gap < 0 {
		gap = 0
	}

	return fmt.Sprintf("  %s%s%s  ", title, strings.Repeat(" ", gap), count)
}

// renderSessionList renders the session list.
func (m Model) renderSessionList() string {
	sessions := m.sessions.List()

	if len(sessions) == 0 {
		return theme.MutedStyle.Render("  No forge sessions found.\n  Start one with: forge start \"prompt\" --promise \"DONE\"")
	}

	var sb strings.Builder
	sb.WriteString("  Sessions:\n")

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Subtle).
		Padding(0, 1).
		Width(m.width - 6)

	var rows []string
	for i, s := range sessions {
		row := m.renderSessionRow(i, s)
		rows = append(rows, row)
	}

	content := strings.Join(rows, "\n")
	sb.WriteString("  ")
	sb.WriteString(boxStyle.Render(content))

	return sb.String()
}

// renderSessionRow renders a single session row.
func (m Model) renderSessionRow(index int, s *session.Session) string {
	// Selection indicator
	indicator := "  "
	style := theme.NormalStyle
	if index == m.selected {
		indicator = "▸ "
		style = theme.SelectedStyle
	}

	// Status icon
	icon := s.StatusIcon()

	// Iteration
	iter := s.IterationString()

	// Promise
	promise := "-"
	if s.State != nil {
		promise = fmt.Sprintf("%q", s.State.CompletionPromise)
		if len(promise) > 20 {
			promise = promise[:17] + "...\""
		}
	}

	// Elapsed
	elapsed := s.ElapsedString()

	// Format: ▸ forge-api     🔄 iter 7/50   "API DONE"     15m ago
	return style.Render(fmt.Sprintf("%s%-15s %s iter %-8s %-20s %s",
		indicator,
		truncate(s.ID, 15),
		icon,
		iter,
		promise,
		elapsed,
	))
}

// renderOutput renders the output preview pane.
func (m Model) renderOutput() string {
	sessions := m.sessions.List()
	if len(sessions) == 0 {
		return ""
	}

	var sessionName string
	if m.selected < len(sessions) {
		sessionName = sessions[m.selected].ID
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  📋 Output [%s]:\n", sessionName))

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Subtle).
		Padding(0, 1).
		Width(m.width - 6).
		Height(8)

	var content string
	if len(m.output) == 0 {
		content = theme.MutedStyle.Render("No output available")
	} else {
		// Take last 8 lines
		start := 0
		if len(m.output) > 8 {
			start = len(m.output) - 8
		}
		lines := m.output[start:]
		content = strings.Join(lines, "\n")
	}

	sb.WriteString("  ")
	sb.WriteString(boxStyle.Render(content))

	return sb.String()
}

// renderHelp renders the help bar.
func (m Model) renderHelp() string {
	keys := []string{
		"[↑/↓] select",
		"[a]ttach",
		"[c]ancel",
		"[r]efresh",
		"[q]uit",
	}

	help := theme.HelpStyle.Render(strings.Join(keys, "  "))
	return fmt.Sprintf("  %s", help)
}

// truncate truncates a string to a maximum length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
