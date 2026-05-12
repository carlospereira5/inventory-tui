package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleFilterCategoriesKeys gestiona el overlay de filtrado de categorías de webhook.
func (m Model) handleFilterCategoriesKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.State = StateScanning
		m.TextInput.Focus()
		return m, nil

	case tea.KeyUp:
		if m.FilterCatCursor > 0 {
			m.FilterCatCursor--
		}
		return m, nil

	case tea.KeyDown:
		if m.FilterCatCursor < len(m.FilterCategories)-1 {
			m.FilterCatCursor++
		}
		return m, nil

	case tea.KeyRunes:
		switch msg.String() {
		case "k":
			if m.FilterCatCursor > 0 {
				m.FilterCatCursor--
			}
		case "j":
			if m.FilterCatCursor < len(m.FilterCategories)-1 {
				m.FilterCatCursor++
			}
		case " ":
			return m.toggleCurrentCategory()
		}
		return m, nil

	case tea.KeySpace:
		return m.toggleCurrentCategory()
	}

	return m, nil
}

// toggleCurrentCategory alterna el estado activo de la categoría bajo el cursor.
func (m Model) toggleCurrentCategory() (tea.Model, tea.Cmd) {
	if len(m.FilterCategories) == 0 || m.FilterCatCursor >= len(m.FilterCategories) {
		return m, nil
	}
	cat := m.FilterCategories[m.FilterCatCursor]
	nowActive := !m.ActiveCategoryIDs[cat.ID]
	return m, m.CmdToggleCategory(cat.ID, nowActive)
}
