package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Update es el corazón de la lógica de Bubble Tea, despacha mensajes a manejadores específicos.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case MsgCatalogLoaded: // Informa si el catálogo CSV se cargó correctamente.
		if msg.Err != nil {
			m.CatalogStatus = fmt.Sprintf("Catálogo fallido: %v", msg.Err)
			m.CatalogIsError = true
		} else {
			m.CatalogStatus = fmt.Sprintf("Catálogo: %s (%d p)", msg.File, msg.Count)
			m.CatalogIsError = false
		}
		return m, nil

	case MsgGroupsLoaded: // Informa si el CSV de grupos se cargó correctamente.
		if msg.Err != nil {
			m.GroupsStatus = fmt.Sprintf("Grupos fallido: %v", msg.Err)
			m.GroupsIsError = true
		} else {
			m.GroupsStatus = fmt.Sprintf("Grupos: %s (%d g)", msg.File, msg.Count)
			m.GroupsIsError = false
		}
		return m, nil

	case MsgSessionsLoaded: // Carga las sesiones en el menú principal.
		if msg.Err == nil {
			m.Sessions = msg.Sessions
		}
		return m, nil

	case MsgTotalsLoaded: // Carga los totales de la sesión activa.
		if msg.Err == nil {
			m.Totals = msg.Totals
		}
		return m, nil

	case MsgLoyverseEventsLoaded: // Carga los eventos de Loyverse de la sesión activa.
		if msg.Err == nil {
			m.LoyverseEvents = msg.Events
		}
		return m, nil

	case tea.WindowSizeMsg: // Ajusta el tamaño de la ventana.
		m.Width, m.Height = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg: // Atajos de teclado globales (ej. Ctrl+C o Esc).
		return m.handleKeyPress(msg)
	}

	return m, nil
}

// handleKeyPress redirige la tecla presionada según el estado actual de la pantalla.
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}

	switch m.State {
	case StateSessionList:
		return m.handleSessionListKeys(msg)
	case StateSessionCreate:
		return m.handleSessionCreateKeys(msg)
	case StateScanning:
		return m.handleScanningKeys(msg)
	case StateHistory:
		return m.handleHistoryKeys(msg)
	case StateLoyverse:
		return m.handleLoyverseKeys(msg)
	}

	return m, nil
}

// updateSessions es una utilidad para refrescar la lista de sesiones.
func (m *Model) updateSessions() {
	m.Sessions, _ = m.Service.GetSessions(context.Background())
}
