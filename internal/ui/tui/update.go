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
			m.CSVStatus = fmt.Sprintf("Catálogo fallido: %v", msg.Err)
			m.CSVIsError = true
		} else {
			m.CSVStatus = fmt.Sprintf("Catálogo listo: %s (%d p)", msg.File, msg.Count)
			m.CSVIsError = false
		}
		return m, nil

	case MsgSessionsLoaded: // Carga las sesiones en el menú principal.
		if msg.Err == nil {
			m.Sessions = msg.Sessions
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
	}

	return m, nil
}

// updateSessions es una utilidad para refrescar la lista de sesiones.
func (m *Model) updateSessions() {
	m.Sessions, _ = m.Service.GetSessions(context.Background())
}
