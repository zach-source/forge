package kanban

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Column styles
	columnStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238"))

	columnTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("229")).
				Padding(0, 1).
				MarginBottom(1)

	// Issue card styles
	cardStyle = lipgloss.NewStyle().
			Padding(0, 1).
			MarginBottom(1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))

	cardTitleStyle = lipgloss.NewStyle().
			Bold(true)

	cardIDStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			Italic(true)

	// Priority colors
	priorityStyles = map[Priority]lipgloss.Style{
		PriorityLow:      lipgloss.NewStyle().Foreground(lipgloss.Color("242")),
		PriorityMedium:   lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		PriorityHigh:     lipgloss.NewStyle().Foreground(lipgloss.Color("202")),
		PriorityCritical: lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true),
	}

	// Status colors
	statusColors = map[Status]lipgloss.Color{
		StatusBacklog:    lipgloss.Color("243"),
		StatusTodo:       lipgloss.Color("75"),
		StatusInProgress: lipgloss.Color("214"),
		StatusReview:     lipgloss.Color("135"),
		StatusDone:       lipgloss.Color("42"),
	}

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("62")).
			Padding(0, 1)
)

// RenderCard renders a single issue card.
func RenderCard(issue *Issue, width int) string {
	var b strings.Builder

	// Title
	title := truncate(issue.Title, width-4)
	b.WriteString(cardTitleStyle.Render(title))
	b.WriteString("\n")

	// ID (short)
	shortID := issue.ID
	if len(shortID) > 16 {
		shortID = shortID[:16] + "..."
	}
	b.WriteString(cardIDStyle.Render(shortID))

	// Priority badge
	if issue.Priority != "" {
		prioStyle := priorityStyles[issue.Priority]
		b.WriteString(" ")
		b.WriteString(prioStyle.Render(string(issue.Priority)))
	}

	// Labels
	if len(issue.Labels) > 0 {
		b.WriteString("\n")
		for i, label := range issue.Labels {
			if i > 0 {
				b.WriteString(" ")
			}
			b.WriteString(labelStyle.Render(label))
		}
	}

	// Assignee
	if issue.Assignee != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("→ " + issue.Assignee))
	}

	card := cardStyle.Width(width).Render(b.String())
	return card
}

// RenderColumn renders a kanban column.
func RenderColumn(col Column, width int) string {
	// Title with count
	title := fmt.Sprintf("%s (%d)", statusTitle(col.Status), len(col.Issues))
	titleStyled := columnTitleStyle.
		Foreground(statusColors[col.Status]).
		Render(title)

	var content strings.Builder
	content.WriteString(titleStyled)
	content.WriteString("\n")

	if len(col.Issues) == 0 {
		content.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			Italic(true).
			Render("No issues"))
	} else {
		for _, issue := range col.Issues {
			content.WriteString(RenderCard(issue, width-4))
			content.WriteString("\n")
		}
	}

	return columnStyle.Width(width).Render(content.String())
}

// RenderBoard renders the full kanban board.
func RenderBoard(board *Board, termWidth int) string {
	if len(board.Columns) == 0 {
		return "Empty board"
	}

	// Calculate column width
	colWidth := (termWidth - 10) / len(board.Columns)
	if colWidth < 20 {
		colWidth = 20
	}
	if colWidth > 40 {
		colWidth = 40
	}

	// Render columns
	var cols []string
	for _, col := range board.Columns {
		cols = append(cols, RenderColumn(col, colWidth))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, cols...)
}

// RenderList renders issues as a simple list.
func RenderList(issues []*Issue) string {
	if len(issues) == 0 {
		return "No issues"
	}

	var b strings.Builder
	for _, issue := range issues {
		// Status indicator
		statusStyle := lipgloss.NewStyle().Foreground(statusColors[issue.Status])
		b.WriteString(statusStyle.Render("●"))
		b.WriteString(" ")

		// Priority
		prioStyle := priorityStyles[issue.Priority]
		b.WriteString(prioStyle.Render(fmt.Sprintf("[%s]", issue.Priority)))
		b.WriteString(" ")

		// Title
		b.WriteString(cardTitleStyle.Render(issue.Title))
		b.WriteString(" ")

		// ID
		b.WriteString(cardIDStyle.Render(fmt.Sprintf("(%s)", shortID(issue.ID))))

		b.WriteString("\n")
	}

	return b.String()
}

func statusTitle(s Status) string {
	switch s {
	case StatusBacklog:
		return "Backlog"
	case StatusTodo:
		return "To Do"
	case StatusInProgress:
		return "In Progress"
	case StatusReview:
		return "Review"
	case StatusDone:
		return "Done"
	default:
		return string(s)
	}
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
