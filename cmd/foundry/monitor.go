package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/zach-source/forge/internal/monitor"
)

func newMonitorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "Launch the monitor TUI",
		Long: `Launch an interactive TUI to monitor multiple forge agent sessions.

The monitor shows all active and recent sessions, their status, iteration count,
and recent output. You can attach to sessions, cancel them, or refresh the view.

Keybindings:
  ↑/↓, j/k  - Navigate sessions
  a         - Attach to selected session
  c         - Cancel selected session
  r         - Refresh
  q, Ctrl+C - Quit

Example:
  forge monitor
  forge mon`,
		Aliases: []string{"mon"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get current working directory for local session discovery
			wd, _ := os.Getwd()

			m := monitor.NewModel(wd)
			p := tea.NewProgram(m, tea.WithAltScreen())

			if _, err := p.Run(); err != nil {
				return fmt.Errorf("running monitor: %w", err)
			}

			return nil
		},
	}

	return cmd
}
