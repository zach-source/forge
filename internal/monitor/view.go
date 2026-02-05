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

// safeWidth returns a minimum safe width for rendering boxes.
func (m Model) safeWidth() int {
	if m.width < 46 {
		return 40
	}
	return m.width - 6
}

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
		Width(m.safeWidth())

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
		Width(m.safeWidth())

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
		Width(m.safeWidth())

	var rows []string
	for i, w := range m.workers {
		row := m.renderWorkerRow(i, w)
		rows = append(rows, row)
	}

	content := strings.Join(rows, "\n")
	sb.WriteString("  ")
	sb.WriteString(boxStyle.Render(content))

	// Add health detail section for selected worker
	sb.WriteString("\n")
	sb.WriteString(m.renderWorkerHealthDetail())

	return sb.String()
}

// renderWorkerRow renders a single worker row with health indicators.
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

	// Health indicator
	healthIcon := "💚" // healthy by default
	if h, ok := m.workerHealth[w.ID]; ok {
		if h.IsStuck {
			healthIcon = "⚠️" // stuck warning
			style = theme.WarningStyle
		} else if w.Status == worker.StatusActive && !h.SessionActive {
			healthIcon = "💔" // session dead
			style = theme.ErrorStyle
		} else if h.PromiseStatus == PromisePending {
			healthIcon = "⏳" // waiting for promise
		}
	}

	// Task info
	task := "-"
	if w.CurrentTask != "" {
		task = truncate(w.CurrentTask, 12)
	}

	// Uptime/activity
	uptimeStr := "-"
	if h, ok := m.workerHealth[w.ID]; ok && w.Status == worker.StatusActive {
		if h.Uptime > 0 {
			uptimeStr = formatDuration(h.Uptime)
		}
	} else if !w.LastActive.IsZero() {
		uptimeStr = formatDuration(time.Since(w.LastActive))
	}

	// Format: ▸ alpha  👷 💤 idle    💚 task-abc123   5m
	return style.Render(fmt.Sprintf("%s%-8s %s %s %-8s %s %-12s %6s",
		indicator,
		w.DisplayName(),
		roleIcon,
		statusIcon,
		w.Status,
		healthIcon,
		task,
		uptimeStr,
	))
}

// renderWorkerHealthDetail renders detailed health info for the selected worker.
func (m Model) renderWorkerHealthDetail() string {
	if len(m.workers) == 0 || m.selected >= len(m.workers) {
		return ""
	}

	w := m.workers[m.selected]
	h, ok := m.workerHealth[w.ID]
	if !ok {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  🏥 Health [%s]:\n", w.DisplayName()))

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Subtle).
		Padding(0, 1).
		Width(m.safeWidth())

	var rows []string

	// Session status
	sessionStatus := "inactive"
	sessionStyle := theme.MutedStyle
	if h.SessionActive {
		sessionStatus = "active"
		sessionStyle = theme.SuccessStyle
	}
	rows = append(rows, fmt.Sprintf("  Session:    %s", sessionStyle.Render(sessionStatus)))

	// Uptime
	uptimeStr := "-"
	if h.Uptime > 0 {
		uptimeStr = formatDurationLong(h.Uptime)
	}
	rows = append(rows, fmt.Sprintf("  Uptime:     %s", uptimeStr))

	// Last activity
	activityStr := "-"
	if !h.LastActivity.IsZero() {
		activityStr = fmt.Sprintf("%s ago", formatDurationLong(time.Since(h.LastActivity)))
	}
	rows = append(rows, fmt.Sprintf("  Activity:   %s", activityStr))

	// Iteration progress
	if h.Iteration > 0 || h.MaxIterations > 0 {
		iterStr := fmt.Sprintf("%d", h.Iteration)
		if h.MaxIterations > 0 {
			iterStr = fmt.Sprintf("%d/%d", h.Iteration, h.MaxIterations)
		} else {
			iterStr = fmt.Sprintf("%d/∞", h.Iteration)
		}
		rows = append(rows, fmt.Sprintf("  Iteration:  %s", iterStr))
	}

	// Promise status
	promiseStr := "-"
	promiseStyle := theme.MutedStyle
	switch h.PromiseStatus {
	case PromiseNone:
		promiseStr = "not configured"
	case PromisePending:
		promiseStr = fmt.Sprintf("pending %q", truncate(h.Promise, 20))
		promiseStyle = theme.WarningStyle
	case PromiseDetected:
		promiseStr = "detected ✓"
		promiseStyle = theme.SuccessStyle
	}
	rows = append(rows, fmt.Sprintf("  Promise:    %s", promiseStyle.Render(promiseStr)))

	// Stuck warning
	if h.IsStuck {
		stuckStr := fmt.Sprintf("⚠️  STUCK for %s (no activity)", formatDurationLong(h.StuckDuration))
		rows = append(rows, theme.WarningStyle.Render("  "+stuckStr))
	}

	sb.WriteString("  ")
	sb.WriteString(boxStyle.Render(strings.Join(rows, "\n")))

	return sb.String()
}

// formatDurationLong formats a duration with more detail.
func formatDurationLong(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		if secs > 0 {
			return fmt.Sprintf("%dm %ds", mins, secs)
		}
		return fmt.Sprintf("%dm", mins)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		mins := int(d.Minutes()) % 60
		if mins > 0 {
			return fmt.Sprintf("%dh %dm", hours, mins)
		}
		return fmt.Sprintf("%dh", hours)
	}
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	if hours > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	return fmt.Sprintf("%dd", days)
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
		Width(m.safeWidth())

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

	// Calculate available height for output pane
	// Layout: header(1) + tabs(1) + sessions_label(1) + sessions_box(n+2) + output_label(1) + help(1) + margins(2)
	sessionBoxHeight := len(sessions) + 2 // sessions + border
	usedHeight := 1 + 1 + 1 + sessionBoxHeight + 1 + 1 + 2
	availableHeight := m.height - usedHeight
	if availableHeight < 4 {
		availableHeight = 4
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Subtle).
		Padding(0, 1).
		Width(m.safeWidth()).
		Height(availableHeight)

	// Content height is box height minus border (2 lines)
	contentLines := availableHeight - 2
	if contentLines < 1 {
		contentLines = 1
	}

	var content string
	if len(m.output) == 0 {
		content = theme.MutedStyle.Render("No output available")
	} else {
		// Take last N lines to fill available space
		start := 0
		if len(m.output) > contentLines {
			start = len(m.output) - contentLines
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
		Width(m.safeWidth())

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
