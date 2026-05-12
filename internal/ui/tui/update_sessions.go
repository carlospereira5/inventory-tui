package tui

import (
	"context"
	"fmt"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"
)

// handleSessionListKeys gestiona la navegación en el menú principal de sesiones.
func (m Model) handleSessionListKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Cancelar pending delete si la tecla no es 'd'.
	if m.PendingDelete && msg.String() != "d" {
		m.PendingDelete = false
	}

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
	case "r":
		if len(m.Sessions) > 0 {
			s := m.Sessions[m.Cursor]
			slog.Info("TUI: cambio de estado", "from", "StateSessionList", "to", "StateSessionRename", "session_id", s.ID)
			m.State = StateSessionRename
			m.SessionInput.Focus()
			m.SessionInput.SetValue(s.Name)
		}
	case "e":
		if len(m.Sessions) > 0 {
			s := m.Sessions[m.Cursor]
			slog.Info("TUI: exportando sesión", "session_id", s.ID, "session_name", s.Name)
			file, err := m.Service.ExportSession(context.Background(), s.ID, s.Name)
			m.formatStatus(err, fmt.Sprintf("Exportado: %s", file))
		}
	case "d":
		if len(m.Sessions) > 0 {
			if !m.PendingDelete {
				m.PendingDelete = true
				return m, nil
			}
			// Segunda 'd': ejecutar borrado.
			m.PendingDelete = false
			s := m.Sessions[m.Cursor]
			slog.Info("TUI: eliminando sesión", "session_id", s.ID, "session_name", s.Name)
			_ = m.Service.DeleteSession(context.Background(), s.ID)
			// Limpiar selección de la sesión borrada.
			delete(m.SelectedSessions, s.ID)
			m.updateSessions()
			if m.Cursor >= len(m.Sessions) && m.Cursor > 0 {
				m.Cursor--
			}
		}
	case "q":
		slog.Info("TUI: usuario solicitó salir")
		return m, tea.Quit
	case " ":
		// Alternar selección de la sesión en el cursor para el sync.
		if len(m.Sessions) > 0 {
			sessionID := m.Sessions[m.Cursor].ID
			m.SelectedSessions[sessionID] = !m.SelectedSessions[sessionID]
		}
	case "s":
		// Recolectar IDs de las sesiones seleccionadas.
		var ids []int
		for _, ses := range m.Sessions {
			if m.SelectedSessions[ses.ID] {
				ids = append(ids, ses.ID)
			}
		}
		if len(ids) == 0 {
			m.StatusMsg = "Seleccioná al menos una sesión con Espacio antes de sincronizar."
			return m, nil
		}
		slog.Info("TUI: sync de sesiones seleccionadas", "count", len(ids), "ids", ids)
		m.SyncModel = NewSyncModel(m.Service)
		m.SyncModel.SessionIDs = ids
		m.SyncModel.Width = m.Width
		m.Cursor = 0
		m.State = StateSyncConfirm
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

// handleSyncConfirmKeys gestiona la selección del modo de sync (Reemplazar o Sumar).
func (m Model) handleSyncConfirmKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
		}
	case "down", "j":
		if m.Cursor < 1 {
			m.Cursor++
		}
	case "enter":
		if m.Cursor == 0 {
			m.SyncModel.Mode = 0 // SyncModeReplace
		} else {
			m.SyncModel.Mode = 1 // SyncModeAdd
		}
		slog.Info("TUI: modo de sync seleccionado", "mode", m.SyncModel.Mode)
		m.State = StateSyncLoyverse
		return m, nil
	case "esc":
		slog.Info("TUI: cambio de estado", "from", "StateSyncConfirm", "to", "StateSessionList")
		m.State = StateSessionList
		m.Cursor = 0
	}
	return m, nil
}

// handleSessionRenameKeys gestiona el formulario de renombrado de una sesión existente.
func (m Model) handleSessionRenameKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.Type {
	case tea.KeyEsc:
		slog.Info("TUI: cambio de estado", "from", "StateSessionRename", "to", "StateSessionList")
		m.State = StateSessionList
	case tea.KeyEnter:
		name := m.SessionInput.Value()
		if name != "" && len(m.Sessions) > 0 {
			s := m.Sessions[m.Cursor]
			if err := m.Service.RenameSession(context.Background(), s.ID, name); err == nil {
				slog.Info("TUI: sesión renombrada", "id", s.ID, "new_name", name)
				m.updateSessions()
			} else {
				slog.Error("TUI: error renombrando sesión", "id", s.ID, "err", err)
				m.formatStatus(err, "")
			}
			m.State = StateSessionList
		}
	}
	m.SessionInput, cmd = m.SessionInput.Update(msg)
	return m, cmd
}
