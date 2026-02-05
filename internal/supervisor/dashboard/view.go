package dashboard

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/zach-source/forge/internal/kanban"
	"github.com/zach-source/forge/internal/worker"
)

var (
	// Colors
	primaryColor   = lipgloss.Color("39")  // Cyan
	secondaryColor = lipgloss.Color("220") // Yellow
	successColor   = lipgloss.Color("40")  // Green
	errorColor     = lipgloss.Color("196") // Red
	mutedColor     = lipgloss.Color("240") // Gray

	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	tabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			MarginRight(1)

	activeTabStyle = tabStyle.
			Bold(true).
			Background(primaryColor).
			Foreground(lipgloss.Color("0"))

	inactiveTabStyle = tabStyle.
				Foreground(mutedColor)

	statusRunning = lipgloss.NewStyle().Foreground(successColor).Render("●")
	statusIdle    = lipgloss.NewStyle().Foreground(mutedColor).Render("○")
	statusStopped = lipgloss.NewStyle().Foreground(errorColor).Render("○")

	helpStyle = lipgloss.NewStyle().Foreground(mutedColor)
)

// View renders the dashboard.
func (m Model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("Foundry Supervisor Dashboard"))
	b.WriteString("\n")

	// Tabs
	b.WriteString(m.renderTabs())
	b.WriteString("\n\n")

	// Content based on active tab
	switch m.activeTab {
	case TabOverview:
		b.WriteString(m.renderOverview())
	case TabWorkers:
		b.WriteString(m.renderWorkers())
	case TabLeaders:
		b.WriteString(m.renderLeaders())
	case TabTasks:
		b.WriteString(m.renderTasks())
	case TabLogs:
		b.WriteString(m.renderLogs())
	}

	// Help
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("tab: switch tabs | ↑↓: navigate | r: refresh | q: quit"))

	return b.String()
}

// renderTabs renders the tab bar.
func (m Model) renderTabs() string {
	var tabs []string
	names := tabNames()

	for i, name := range names {
		if Tab(i) == m.activeTab {
			tabs = append(tabs, activeTabStyle.Render(fmt.Sprintf("%d %s", i+1, name)))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(fmt.Sprintf("%d %s", i+1, name)))
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}

// renderOverview renders the overview tab.
func (m Model) renderOverview() string {
	var b strings.Builder

	// Supervisor status
	b.WriteString("Supervisor\n")
	if m.supervisorState != nil {
		uptime := time.Since(m.supervisorState.StartedAt).Round(time.Second)
		b.WriteString(fmt.Sprintf("  Uptime: %s    Cycles: %d\n", uptime, m.supervisorState.CycleCount))
		b.WriteString(fmt.Sprintf("  Last cycle: %s\n", m.supervisorState.LastCycle.Format("15:04:05")))
	} else {
		b.WriteString("  Status: not running\n")
	}
	b.WriteString("\n")

	// Task Pipeline
	b.WriteString("Task Pipeline\n")
	if m.board != nil {
		counts := make(map[kanban.Status]int)
		for _, col := range m.board.Columns {
			counts[col.Status] = len(col.Issues)
		}
		b.WriteString(fmt.Sprintf("  Backlog:     %d\n", counts[kanban.StatusBacklog]))
		b.WriteString(fmt.Sprintf("  Todo:        %d\n", counts[kanban.StatusTodo]))
		b.WriteString(fmt.Sprintf("  In Progress: %d\n", counts[kanban.StatusInProgress]))
		b.WriteString(fmt.Sprintf("  Review:      %d\n", counts[kanban.StatusReview]))
		b.WriteString(fmt.Sprintf("  Merge:       %d\n", counts[kanban.StatusMerge]))
		b.WriteString(fmt.Sprintf("  Done:        %d\n", counts[kanban.StatusDone]))
	} else {
		b.WriteString("  (no board data)\n")
	}
	b.WriteString("\n")

	// Workers summary
	b.WriteString("Workers\n")
	if len(m.workers) > 0 {
		var active, idle, stopped int
		for _, w := range m.workers {
			switch w.Status {
			case worker.StatusActive:
				active++
			case worker.StatusIdle:
				idle++
			case worker.StatusStopped:
				stopped++
			}
		}
		b.WriteString(fmt.Sprintf("  Active:  %d\n", active))
		b.WriteString(fmt.Sprintf("  Idle:    %d\n", idle))
		b.WriteString(fmt.Sprintf("  Stopped: %d\n", stopped))
	} else {
		b.WriteString("  (no workers)\n")
	}
	b.WriteString("\n")

	// Leaders summary
	b.WriteString("Leaders\n")
	if m.supervisorState != nil && len(m.supervisorState.Leaders) > 0 {
		for role, ls := range m.supervisorState.Leaders {
			status := statusIdle
			if ls.Running {
				status = statusRunning
			}
			b.WriteString(fmt.Sprintf("  %s %-10s\n", status, role))
		}
	} else {
		b.WriteString("  (no leader data)\n")
	}
	b.WriteString("\n")

	// Infrastructure health
	b.WriteString("Infrastructure\n")
	if m.health != nil {
		statusIcon := statusRunning
		switch m.health.Status {
		case "degraded":
			statusIcon = lipgloss.NewStyle().Foreground(secondaryColor).Render("●")
		case "critical":
			statusIcon = lipgloss.NewStyle().Foreground(errorColor).Render("●")
		}
		b.WriteString(fmt.Sprintf("  Status: %s %s\n", statusIcon, m.health.Status))
		b.WriteString(fmt.Sprintf("  Nodes: %d/%d ready\n", m.health.Cluster.NodesReady, m.health.Cluster.NodesTotal))
		b.WriteString(fmt.Sprintf("  Alerts: %d crit, %d warn\n", m.health.Alerts.Critical, m.health.Alerts.Warning))
	} else {
		b.WriteString("  (no health data)\n")
	}

	return b.String()
}

// renderWorkers renders the workers tab.
func (m Model) renderWorkers() string {
	var b strings.Builder
	b.WriteString("Workers\n\n")

	if len(m.workers) == 0 {
		b.WriteString("  No workers registered.\n")
		b.WriteString("  Create one with: foundry worker create\n")
		return b.String()
	}

	// Header
	b.WriteString(fmt.Sprintf("  %-12s %-10s %-10s %-20s %s\n",
		"NAME", "ROLE", "STATUS", "TASK", "SESSION"))
	b.WriteString(fmt.Sprintf("  %s\n", strings.Repeat("-", 70)))

	for i, w := range m.workers {
		status := statusIdle
		statusText := "idle"
		switch w.Status {
		case worker.StatusActive:
			status = statusRunning
			statusText = "active"
		case worker.StatusStopped:
			status = statusStopped
			statusText = "stopped"
		case worker.StatusPaused:
			status = lipgloss.NewStyle().Foreground(secondaryColor).Render("●")
			statusText = "paused"
		}

		task := w.CurrentTask
		if task == "" {
			task = "-"
		} else if len(task) > 18 {
			task = task[:15] + "..."
		}

		session := w.SessionID
		if session == "" {
			session = "-"
		}

		prefix := "  "
		if i == m.selected && m.activeTab == TabWorkers {
			prefix = "> "
		}

		b.WriteString(fmt.Sprintf("%s%-12s %-10s %s %-8s %-20s %s\n",
			prefix, w.DisplayName(), w.Role, status, statusText, task, session))
	}

	return b.String()
}

// renderLeaders renders the leaders tab.
func (m Model) renderLeaders() string {
	var b strings.Builder
	b.WriteString("Leaders\n\n")

	if m.supervisorState == nil || len(m.supervisorState.Leaders) == 0 {
		b.WriteString("  No leader data available.\n")
		b.WriteString("  Start supervisor with: foundry supervisor --leaders\n")
		return b.String()
	}

	// Header
	b.WriteString(fmt.Sprintf("  %-12s %-10s %-12s %s\n",
		"ROLE", "STATUS", "UPTIME", "SESSION"))
	b.WriteString(fmt.Sprintf("  %s\n", strings.Repeat("-", 60)))

	i := 0
	for role, ls := range m.supervisorState.Leaders {
		status := statusIdle
		statusText := "idle"
		uptime := "-"

		if ls.Running {
			status = statusRunning
			statusText = "running"
			if !ls.StartedAt.IsZero() {
				uptime = time.Since(ls.StartedAt).Round(time.Second).String()
			}
		}

		session := ls.SessionID
		if session == "" {
			session = "-"
		}

		prefix := "  "
		if i == m.selected && m.activeTab == TabLeaders {
			prefix = "> "
		}

		b.WriteString(fmt.Sprintf("%s%-12s %s %-8s %-12s %s\n",
			prefix, role, status, statusText, uptime, session))
		i++
	}

	return b.String()
}

// renderTasks renders the tasks tab.
func (m Model) renderTasks() string {
	var b strings.Builder
	b.WriteString("Tasks\n\n")

	if m.board == nil {
		b.WriteString("  No board data available.\n")
		return b.String()
	}

	itemIdx := 0
	for _, col := range m.board.Columns {
		if len(col.Issues) == 0 {
			continue
		}

		// Column header
		b.WriteString(fmt.Sprintf("  %s (%d)\n", col.Status, len(col.Issues)))

		for _, issue := range col.Issues {
			prefix := "    "
			if itemIdx == m.selected && m.activeTab == TabTasks {
				prefix = "  > "
			}

			title := issue.Title
			if len(title) > 50 {
				title = title[:47] + "..."
			}

			priority := string(issue.Priority)
			switch issue.Priority {
			case kanban.PriorityCritical:
				priority = lipgloss.NewStyle().Foreground(errorColor).Render(priority)
			case kanban.PriorityHigh:
				priority = lipgloss.NewStyle().Foreground(secondaryColor).Render(priority)
			}

			id := issue.ID
			if len(id) > 8 {
				id = id[:8]
			}

			b.WriteString(fmt.Sprintf("%s[%s] %s: %s\n", prefix, priority, id, title))
			itemIdx++
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderLogs renders the logs tab.
func (m Model) renderLogs() string {
	var b strings.Builder
	b.WriteString("Logs\n\n")
	b.WriteString("  Log viewing not yet implemented.\n")
	b.WriteString("  Use: foundry log -f\n")
	return b.String()
}
