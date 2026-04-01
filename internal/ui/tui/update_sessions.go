package tui

import (
	"context"
	"fmt"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"
)

// handleSessionListKeys gestiona la navegación en el menú principal de sesiones.
func (m Model) handleSessionListKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
		}
	case "down", "j":
		if m.Cursor < len(m.Sessions)-1 {
			m.Cursor++
		}
	case "enter":
		if len(m.Sessions) > 0 {
			m.ActiveSession = &m.Sessions[m.Cursor]
			m.Service.ActivateSession(m.ActiveSession.ID)
			slog.Info("TUI: cambio de estado", "from", "StateSessionList", "to", "StateScanning", "session_id", m.ActiveSession.ID, "session_name", m.ActiveSession.Name)
			m.State = StateScanning
			m.TextInput.Focus()
			m.StatusMsg = ""
		}
	case "n":
		slog.Info("TUI: cambio de estado", "from", "StateSessionList", "to", "StateSessionCreate")
		m.State = StateSessionCreate
		m.SessionInput.Focus()
		m.SessionInput.SetValue("")
	case "e":
		if len(m.Sessions) > 0 {
			s := m.Sessions[m.Cursor]
			slog.Info("TUI: exportando sesión", "session_id", s.ID, "session_name", s.Name)
			file, err := m.Service.ExportSession(context.Background(), s.ID, s.Name)
			m.formatStatus(err, fmt.Sprintf("Exportado: %s", file))
		}
	case "d":
		if len(m.Sessions) > 0 {
			s := m.Sessions[m.Cursor]
			slog.Info("TUI: eliminando sesión", "session_id", s.ID, "session_name", s.Name)
			_ = m.Service.DeleteSession(context.Background(), m.Sessions[m.Cursor].ID)
			m.updateSessions()
			if m.Cursor >= len(m.Sessions) && m.Cursor > 0 {
				m.Cursor--
			}
		}
	case "q":
		slog.Info("TUI: usuario solicitó salir")
		return m, tea.Quit
	case "s":
		slog.Info("TUI: cambio de estado", "from", "StateSessionList", "to", "StateSyncLoyverse")
		m.State = StateSyncLoyverse
		m.SyncModel = NewSyncModel(m.Service)
		m.SyncModel.Width = m.Width
		return m, nil
	}
	return m, nil
}

// handleSessionCreateKeys gestiona el formulario de creación de una sesión.
func (m Model) handleSessionCreateKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.Type {
	case tea.KeyEsc:
		slog.Info("TUI: cambio de estado", "from", "StateSessionCreate", "to", "StateSessionList")
		m.State = StateSessionList
	case tea.KeyEnter:
		name := m.SessionInput.Value()
		if name != "" {
			id, err := m.Service.CreateSession(context.Background(), name)
			if err == nil {
				slog.Info("TUI: sesión creada", "id", id, "name", name)
				m.updateSessions()
				for i, s := range m.Sessions {
					if s.ID == id {
						m.ActiveSession = &m.Sessions[i]
						break
					}
				}
				m.Service.ActivateSession(id)
				slog.Info("TUI: cambio de estado", "from", "StateSessionCreate", "to", "StateScanning", "session_id", id)
				m.State = StateScanning
				m.TextInput.Focus()
			}
		}
	}
	m.SessionInput, cmd = m.SessionInput.Update(msg)
	return m, cmd
}
