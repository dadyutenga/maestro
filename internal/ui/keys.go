package ui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all application keybindings.
type KeyMap struct {
	Quit            key.Binding
	NextPanel       key.Binding
	PrevPanel       key.Binding
	NewWorkspace    key.Binding
	DeleteWorkspace key.Binding
	SpawnAgent      key.Binding
	KillAgent       key.Binding
	Review          key.Binding
	Broadcast       key.Binding
	Clear           key.Binding
	Send            key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		NextPanel: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next panel"),
		),
		PrevPanel: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev panel"),
		),
		NewWorkspace: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new workspace"),
		),
		DeleteWorkspace: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete workspace"),
		),
		SpawnAgent: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "spawn agent"),
		),
		KillAgent: key.NewBinding(
			key.WithKeys("k"),
			key.WithHelp("k", "kill agent"),
		),
		Review: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "review"),
		),
		Broadcast: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "broadcast"),
		),
		Clear: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "clear"),
		),
		Send: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "send"),
		),
	}
}
