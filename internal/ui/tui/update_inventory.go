package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// handleScanningKeys procesa el escaneo de códigos de barras y navegación al historial.
func (m Model) handleScanningKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.Type {
	case tea.KeyEsc:
		m.State = StateSessionList
		m.updateSessions()
	case tea.KeyTab:
		m.State = StateHistory
		m.History, _ = m.Service.GetHistory(context.Background(), m.ActiveSession.ID)
		m.Cursor = 0
	case tea.KeyEnter:
		barcode := m.TextInput.Value()
		if barcode == "" { return m, nil }
		
		// Limpieza de duplicados por escáner.
		if len(barcode) > 0 && len(barcode)%2 == 0 {
			half := len(barcode) / 2
			if barcode[:half] == barcode[half:] { barcode = barcode[:half] }
		}

		p, rec, err := m.Service.ScanProduct(context.Background(), m.ActiveSession.ID, barcode)
		if err != nil { m.Err = err; return m, nil }
		if p == nil {
			m.StatusMsg = fmt.Sprintf("No encontrado: %s", barcode)
			m.LastScanned = nil
		} else {
			m.LastScanned = rec
			m.StatusMsg = fmt.Sprintf("¡Escaneado: %s!", p.Name)
		}
		m.TextInput.SetValue("")
	}
	m.TextInput, cmd = m.TextInput.Update(msg)
	return m, cmd
}

// handleHistoryKeys gestiona la navegación y borrado en el historial de escaneos.
func (m Model) handleHistoryKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		m.State = StateScanning
		m.TextInput.Focus()
	case "up", "k":
		if m.Cursor > 0 { m.Cursor-- }
	case "down", "j":
		if m.Cursor < len(m.History)-1 { m.Cursor++ }
	case "d", "backspace":
		if len(m.History) > 0 && m.Cursor < len(m.History) {
			_ = m.Service.DeleteScan(context.Background(), m.History[m.Cursor].ID)
			m.History, _ = m.Service.GetHistory(context.Background(), m.ActiveSession.ID)
			if m.Cursor >= len(m.History) && m.Cursor > 0 { m.Cursor-- }
		}
	}
	return m, nil
}
