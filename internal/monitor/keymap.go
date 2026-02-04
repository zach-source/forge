// Package monitor provides the TUI for monitoring forge sessions.
package monitor

import (
	"github.com/charmbracelet/bubbles/key"
)

// KeyMap defines the keybindings for the monitor.
type KeyMap struct {
	// Navigation
	Up   key.Binding
	Down key.Binding

	// Tab navigation
	TabNext key.Binding
	TabPrev key.Binding
	Tab1    key.Binding
	Tab2    key.Binding
	Tab3    key.Binding
	Tab4    key.Binding

	// Actions
	Attach  key.Binding
	Cancel  key.Binding
	Refresh key.Binding
	Help    key.Binding
	Quit    key.Binding
}

// DefaultKeyMap returns the default keybindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		TabNext: key.NewBinding(
			key.WithKeys("tab", "l"),
			key.WithHelp("tab/l", "next tab"),
		),
		TabPrev: key.NewBinding(
			key.WithKeys("shift+tab", "h"),
			key.WithHelp("shift+tab/h", "prev tab"),
		),
		Tab1: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "overview"),
		),
		Tab2: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "workers"),
		),
		Tab3: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "sessions"),
		),
		Tab4: key.NewBinding(
			key.WithKeys("4"),
			key.WithHelp("4", "logs"),
		),
		Attach: key.NewBinding(
			key.WithKeys("a", "enter"),
			key.WithHelp("a", "attach"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "cancel"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

// ShortHelp returns the short help text.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.TabNext, k.Up, k.Down, k.Attach, k.Refresh, k.Quit}
}

// FullHelp returns the full help text.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.TabNext, k.TabPrev},
		{k.Up, k.Down},
		{k.Attach, k.Cancel},
		{k.Refresh, k.Quit},
	}
}
