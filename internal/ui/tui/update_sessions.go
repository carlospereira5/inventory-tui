package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// handleSessionListKeys gestiona la navegación en el menú principal de sesiones.
func (m Model) handleSessionListKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.Cursor > 0 { m.Cursor-- }
	case "down", "j":
		if m.Cursor < len(m.Sessions)-1 { m.Cursor++ }
	case "enter":
		if len(m.Sessions) > 0 {
			m.ActiveSession = &m.Sessions[m.Cursor]
			m.State = StateScanning
			m.TextInput.Focus()
			m.StatusMsg = ""
		}
	case "n":
		m.State = StateSessionCreate
		m.SessionInput.Focus()
		m.SessionInput.SetValue("")
	case "e":
		if len(m.Sessions) > 0 {
			s := m.Sessions[m.Cursor]
			file, err := m.Service.ExportSession(context.Background(), s.ID, s.Name)
			m.formatStatus(err, fmt.Sprintf("Exportado: %s", file))
		}
	case "d":
		if len(m.Sessions) > 0 {
			_ = m.Service.DeleteSession(context.Background(), m.Sessions[m.Cursor].ID)
			m.updateSessions()
			if m.Cursor >= len(m.Sessions) && m.Cursor > 0 { m.Cursor-- }
		}
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

// handleSessionCreateKeys gestiona el formulario de creación de una sesión.
func (m Model) handleSessionCreateKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.Type {
	case tea.KeyEsc:
		m.State = StateSessionList
	case tea.KeyEnter:
		name := m.SessionInput.Value()
		if name != "" {
			id, err := m.Service.CreateSession(context.Background(), name)
			if err == nil {
				m.updateSessions()
				for i, s := range m.Sessions {
					if s.ID == id { m.ActiveSession = &m.Sessions[i]; break }
				}
				m.State = StateScanning
				m.TextInput.Focus()
			}
		}
	}
	m.SessionInput, cmd = m.SessionInput.Update(msg)
	return m, cmd
}
