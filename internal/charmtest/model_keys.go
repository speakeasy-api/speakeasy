package charmtest

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Sends control keys into the model.
func (m Model) SendKeys(keys ...tea.KeyType) {
	for _, key := range keys {
		msg := tea.KeyMsg{
			Type: key,
		}

		m.TestModel.Send(msg)
	}
}
