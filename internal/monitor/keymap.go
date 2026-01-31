// Package monitor provides the TUI for monitoring forge sessions.
package monitor

import (
	"github.com/charmbracelet/bubbles/key"
)

// KeyMap defines the keybindings for the monitor.
type KeyMap struct {
	Up      key.Binding
	Down    key.Binding
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
	return []key.Binding{k.Up, k.Down, k.Attach, k.Cancel, k.Refresh, k.Quit}
}

// FullHelp returns the full help text.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.Attach, k.Cancel},
		{k.Refresh, k.Quit},
	}
}
