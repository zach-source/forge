package monitor

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/zach-source/forge/internal/kanban"
	"github.com/zach-source/forge/internal/logs"
	"github.com/zach-source/forge/internal/session"
	"github.com/zach-source/forge/internal/theme"
	"github.com/zach-source/forge/internal/worker"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// renderView renders the main view.
func (m Model) renderView() string {
	var sb strings.Builder

	// Header with tabs
	sb.WriteString(m.renderHeader())
	sb.WriteString("\n")
	sb.WriteString(m.renderTabs())
	sb.WriteString("\n")

	// Tab content
	switch m.activeTab {
	case TabOverview:
		sb.WriteString(m.renderOverviewTab())
	case TabWorkers:
		sb.WriteString(m.renderWorkersTab())
	case TabSessions:
		sb.WriteString(m.renderSessionsTab())
	case TabLogs:
		sb.WriteString(m.renderLogsTab())
	}

	// Help
	sb.WriteString("\n")
	sb.WriteString(m.renderHelp())

	return sb.String()
}

// renderHeader renders the header bar.
func (m Model) renderHeader() string {
	title := theme.TitleStyle.Render("foundry monitor")

	// Summary counts
	sessions := m.sessions.List()
	activeSessionCount := 0
	for _, s := range sessions {
		if s.Status == session.StatusActive {
			activeSessionCount++
		}
	}

	activeWorkerCount := 0
	for _, w := range m.workers {
		if w.Status == worker.StatusActive {
			activeWorkerCount++
		}
	}

	summary := theme.MutedStyle.Render(fmt.Sprintf("%d workers, %d sessions", len(m.workers), len(sessions)))
	if activeWorkerCount > 0 || activeSessionCount > 0 {
		summary = theme.SuccessStyle.Render(fmt.Sprintf("%d workers, %d sessions active", activeWorkerCount, activeSessionCount))
	}

	// Right-align summary
	gap := m.width - lipgloss.Width(title) - lipgloss.Width(summary) - 4
	if gap < 0 {
		gap = 0
	}

	return fmt.Sprintf("  %s%s%s  ", title, strings.Repeat(" ", gap), summary)
}

// renderTabs renders the tab bar.
func (m Model) renderTabs() string {
	var tabs []string

	activeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Primary).
		Background(theme.Subtle).
		Padding(0, 2)

	inactiveStyle := lipgloss.NewStyle().
		Foreground(theme.Muted).
		Padding(0, 2)

	names := tabNames()
	for i, name := range names {
		label := fmt.Sprintf("%d %s", i+1, name)
		if Tab(i) == m.activeTab {
			tabs = append(tabs, activeStyle.Render(label))
		} else {
			tabs = append(tabs, inactiveStyle.Render(label))
		}
	}

	return "  " + strings.Join(tabs, " ")
}

// renderOverviewTab renders the Overview tab content.
func (m Model) renderOverviewTab() string {
	var sb strings.Builder

	// Kanban summary
	sb.WriteString("  📋 Kanban:\n")
	sb.WriteString(m.renderKanbanSummary())
	sb.WriteString("\n\n")

	// Worker summary
	sb.WriteString("  👷 Workers:\n")
	sb.WriteString(m.renderWorkerSummary())

	return sb.String()
}

// renderKanbanSummary renders the kanban status counts.
func (m Model) renderKanbanSummary() string {
	if m.kanban == nil {
		return theme.MutedStyle.Render("  No kanban data available")
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Subtle).
		Padding(0, 1).
		Width(m.width - 6)

	var rows []string
	for _, col := range m.kanban.Columns {
		icon := statusIcon(col.Status)
		count := len(col.Issues)
		statusName := cases.Title(language.English).String(strings.ReplaceAll(string(col.Status), "_", " "))

		style := theme.NormalStyle
		switch col.Status {
		case kanban.StatusInProgress:
			style = theme.SuccessStyle
		case kanban.StatusReview:
			style = theme.WarningStyle
		}

		rows = append(rows, style.Render(fmt.Sprintf("  %s %-15s %d", icon, statusName, count)))
	}

	return "  " + boxStyle.Render(strings.Join(rows, "\n"))
}

// statusIcon returns an icon for a kanban status.
func statusIcon(status kanban.Status) string {
	switch status {
	case kanban.StatusBacklog:
		return "📥"
	case kanban.StatusTodo:
		return "📝"
	case kanban.StatusInProgress:
		return "🔄"
	case kanban.StatusReview:
		return "🔍"
	case kanban.StatusDone:
		return "✅"
	default:
		return "❓"
	}
}

// renderWorkerSummary renders the worker status summary.
func (m Model) renderWorkerSummary() string {
	if len(m.workers) == 0 {
		return theme.MutedStyle.Render("  No workers registered")
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Subtle).
		Padding(0, 1).
		Width(m.width - 6)

	// Count by status
	statusCounts := make(map[worker.Status]int)
	roleCounts := make(map[worker.Role]int)
	for _, w := range m.workers {
		statusCounts[w.Status]++
		roleCounts[w.Role]++
	}

	var rows []string

	// Status counts
	rows = append(rows, theme.MutedStyle.Render("  Status:"))
	for _, status := range worker.ValidStatuses() {
		count := statusCounts[status]
		if count > 0 {
			icon := workerStatusIcon(status)
			rows = append(rows, fmt.Sprintf("    %s %-10s %d", icon, status, count))
		}
	}

	rows = append(rows, "")

	// Role counts
	rows = append(rows, theme.MutedStyle.Render("  Roles:"))
	for _, role := range worker.ValidRoles() {
		count := roleCounts[role]
		if count > 0 {
			icon := workerRoleIcon(role)
			rows = append(rows, fmt.Sprintf("    %s %-10s %d", icon, role, count))
		}
	}

	return "  " + boxStyle.Render(strings.Join(rows, "\n"))
}

// workerStatusIcon returns an icon for a worker status.
func workerStatusIcon(status worker.Status) string {
	switch status {
	case worker.StatusIdle:
		return "💤"
	case worker.StatusActive:
		return "🔄"
	case worker.StatusPaused:
		return "⏸️"
	case worker.StatusStopped:
		return "🛑"
	default:
		return "❓"
	}
}

// workerRoleIcon returns an icon for a worker role.
func workerRoleIcon(role worker.Role) string {
	switch role {
	case worker.RoleWorker:
		return "👷"
	case worker.RolePlanner:
		return "📋"
	case worker.RoleReviewer:
		return "🔍"
	case worker.RoleMerge:
		return "🔀"
	case worker.RoleDeploy:
		return "🚀"
	case worker.RoleGroomer:
		return "🧹"
	default:
		return "❓"
	}
}

// renderWorkersTab renders the Workers tab content.
func (m Model) renderWorkersTab() string {
	if len(m.workers) == 0 {
		return theme.MutedStyle.Render("  No workers registered.\n  Create one with: foundry worker create")
	}

	var sb strings.Builder
	sb.WriteString("  Workers:\n")

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Subtle).
		Padding(0, 1).
		Width(m.width - 6)

	var rows []string
	for i, w := range m.workers {
		row := m.renderWorkerRow(i, w)
		rows = append(rows, row)
	}

	content := strings.Join(rows, "\n")
	sb.WriteString("  ")
	sb.WriteString(boxStyle.Render(content))

	return sb.String()
}

// renderWorkerRow renders a single worker row.
func (m Model) renderWorkerRow(index int, w *worker.Worker) string {
	// Selection indicator
	indicator := "  "
	style := theme.NormalStyle
	if m.activeTab == TabWorkers && index == m.selected {
		indicator = "▸ "
		style = theme.SelectedStyle
	}

	// Icons
	statusIcon := w.StatusIcon()
	roleIcon := w.RoleIcon()

	// Task info
	task := "-"
	if w.CurrentTask != "" {
		task = truncate(w.CurrentTask, 15)
	}

	// Last active
	elapsed := formatDuration(time.Since(w.LastActive))

	// Format: ▸ alpha (👷) 💤 idle     task-abc123    5m ago
	return style.Render(fmt.Sprintf("%s%-8s %s %s %-8s %-15s %s",
		indicator,
		w.DisplayName(),
		roleIcon,
		statusIcon,
		w.Status,
		task,
		elapsed,
	))
}

// renderSessionsTab renders the Sessions tab content.
func (m Model) renderSessionsTab() string {
	var sb strings.Builder

	// Session list
	sb.WriteString(m.renderSessionList())
	sb.WriteString("\n")

	// Output preview
	sb.WriteString(m.renderOutput())

	return sb.String()
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
	if m.activeTab == TabSessions && index == m.selected {
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

// renderLogsTab renders the Logs tab content.
func (m Model) renderLogsTab() string {
	if len(m.logEntries) == 0 {
		return theme.MutedStyle.Render("  No log files found.")
	}

	var sb strings.Builder
	sb.WriteString("  Log Files:\n")

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Subtle).
		Padding(0, 1).
		Width(m.width - 6)

	var rows []string
	// Show up to 15 most recent logs
	maxLogs := 15
	if len(m.logEntries) < maxLogs {
		maxLogs = len(m.logEntries)
	}
	for i := 0; i < maxLogs; i++ {
		row := m.renderLogRow(i, m.logEntries[i])
		rows = append(rows, row)
	}

	content := strings.Join(rows, "\n")
	sb.WriteString("  ")
	sb.WriteString(boxStyle.Render(content))

	return sb.String()
}

// renderLogRow renders a single log file row.
func (m Model) renderLogRow(index int, entry logs.LogEntry) string {
	// Selection indicator
	indicator := "  "
	style := theme.NormalStyle
	if m.activeTab == TabLogs && index == m.selected {
		indicator = "▸ "
		style = theme.SelectedStyle
	}

	// Type icon
	icon := logTypeIcon(entry.Type)

	// Size
	size := formatSize(entry.Size)

	// Age
	elapsed := formatDuration(time.Since(entry.ModTime))

	// Format: ▸ 📄 session-abc123    512KB    5m ago
	return style.Render(fmt.Sprintf("%s%s %-25s %8s %s",
		indicator,
		icon,
		truncate(entry.Name, 25),
		size,
		elapsed,
	))
}

// logTypeIcon returns an icon for a log type.
func logTypeIcon(t logs.LogType) string {
	switch t {
	case logs.LogTypeSession:
		return "📄"
	case logs.LogTypeWorker:
		return "👷"
	case logs.LogTypeLeader:
		return "👑"
	default:
		return "📄"
	}
}

// renderHelp renders the help bar.
func (m Model) renderHelp() string {
	keys := []string{
		"[tab/1-4] switch tabs",
		"[↑/↓] select",
	}

	// Context-sensitive help
	switch m.activeTab {
	case TabSessions:
		keys = append(keys, "[a]ttach", "[c]ancel")
	case TabWorkers:
		keys = append(keys, "[a]ttach")
	}

	keys = append(keys, "[r]efresh", "[q]uit")

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

// formatDuration formats a duration as a human-readable string.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// formatSize formats a file size as a human-readable string.
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
