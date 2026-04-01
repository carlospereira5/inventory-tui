package tui

import (
	"context"
	"fmt"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"
)

// handleScanningKeys procesa el escaneo de códigos de barras y navegación.
func (m Model) handleScanningKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.Type {
	case tea.KeyEsc:
		slog.Debug("navegación: esc → StateSessionList")
		m.State = StateSessionList
		m.updateSessions()
		return m, nil
	case tea.KeyTab:
		slog.Debug("navegación: tab → StateLoyverse")
		m.State = StateLoyverse
		m.Totals, _ = m.Service.GetSessionTotals(context.Background(), m.ActiveSession.ID)
		m.LoyverseEvents, _ = m.Service.GetLoyverseEvents(context.Background(), m.ActiveSession.ID)
		m.Cursor = 0
		return m, nil
	case tea.KeyEnter:
		barcode := m.TextInput.Value()
		if barcode == "" {
			return m, nil
		}

		// Limpieza de duplicados por escáner.
		if len(barcode) > 0 && len(barcode)%2 == 0 {
			half := len(barcode) / 2
			if barcode[:half] == barcode[half:] {
				barcode = barcode[:half]
			}
		}

		slog.Debug("escaneo", "barcode", barcode, "session_id", m.ActiveSession.ID)
		p, rec, err := m.Service.ScanProduct(context.Background(), m.ActiveSession.ID, barcode)
		if err != nil {
			slog.Error("escaneo fallido", "barcode", barcode, "err", err)
			m.Err = err
			return m, nil
		}
		if p == nil {
			slog.Warn("producto no encontrado", "barcode", barcode)
			m.StatusMsg = fmt.Sprintf("No encontrado: %s", barcode)
			m.LastScanned = nil
			m.ConsecutiveCount = 0
		} else {
			// Actualizar contador consecutivo
			if m.LastScanned != nil && m.LastScanned.Barcode == p.Barcode {
				m.ConsecutiveCount++
			} else {
				m.ConsecutiveCount = 1
			}
			m.LastScanned = rec
			m.StatusMsg = fmt.Sprintf("¡Escaneado: %s!", p.Name)
			slog.Info("escaneo exitoso", "barcode", barcode, "name", p.Name, "consecutive", m.ConsecutiveCount)
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
		slog.Debug("navegación: tab → StateScanning")
		m.State = StateScanning
		m.TextInput.Focus()
	case "esc":
		slog.Debug("navegación: esc → StateSessionList")
		m.State = StateSessionList
		m.updateSessions()
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
		}
	case "down", "j":
		if m.Cursor < len(m.History)-1 {
			m.Cursor++
		}
	case "d", "backspace":
		if len(m.History) > 0 && m.Cursor < len(m.History) {
			scanID := m.History[m.Cursor].ID
			slog.Info("historial: eliminando escaneo", "scan_id", scanID)
			if err := m.Service.DeleteScan(context.Background(), scanID); err != nil {
				slog.Error("historial: error eliminando escaneo", "scan_id", scanID, "err", err)
				m.Err = err
			}
			var err error
			m.History, err = m.Service.GetHistory(context.Background(), m.ActiveSession.ID)
			if err != nil {
				slog.Error("historial: error recargando historial", "err", err)
				m.Err = err
			}
			if m.Cursor >= len(m.History) && m.Cursor > 0 {
				m.Cursor--
			}
		}
	}
	return m, nil
}

// handleLoyverseKeys gestiona la navegación en la pantalla de totales y eventos de Loyverse.
func (m Model) handleLoyverseKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		slog.Debug("navegación: tab → StateHistory")
		m.State = StateHistory
		var err error
		m.History, err = m.Service.GetHistory(context.Background(), m.ActiveSession.ID)
		if err != nil {
			slog.Error("loyverse: error cargando historial", "err", err)
			m.Err = err
		}
		m.Cursor = 0
	case "esc":
		slog.Debug("navegación: esc → StateSessionList")
		m.State = StateSessionList
		m.updateSessions()
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
		}
	case "down", "j":
		maxCursor := len(m.LoyverseEvents) - 1
		if m.Cursor < maxCursor {
			m.Cursor++
		}
	case "d", "backspace":
		if len(m.LoyverseEvents) > 0 && m.Cursor < len(m.LoyverseEvents) {
			eventID := m.LoyverseEvents[m.Cursor].ID
			slog.Info("loyverse: eliminando evento", "event_id", eventID)
			if err := m.Service.DeleteLoyverseEvent(context.Background(), eventID); err != nil {
				slog.Error("loyverse: error eliminando evento", "event_id", eventID, "err", err)
				m.Err = err
			}
			var err error
			m.LoyverseEvents, err = m.Service.GetLoyverseEvents(context.Background(), m.ActiveSession.ID)
			if err != nil {
				slog.Error("loyverse: error recargando eventos", "err", err)
				m.Err = err
			}
			if m.Cursor >= len(m.LoyverseEvents) && m.Cursor > 0 {
				m.Cursor--
			}
		}
	case "s":
		slog.Debug("navegación: s → StateSyncLoyverse")
		m.State = StateSyncLoyverse
		m.SyncModel = NewSyncModel(m.Service)
		m.SyncModel.Width = m.Width
		return m, nil
	}
	return m, nil
}

// handleSyncKeys gestiona la navegación en la pantalla de sincronización.
func (m Model) handleSyncKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "enter":
		if m.SyncModel.State == SyncIdle || m.SyncModel.State == SyncError {
			m.SyncModel.State = SyncSyncing
			m.SyncModel.Help = "Sincronizando..."
			return m, m.SyncModel.CmdSync()
		}
	case "esc":
		m.State = StateLoyverse
		m.Totals, _ = m.Service.GetSessionTotals(context.Background(), m.ActiveSession.ID)
		m.LoyverseEvents, _ = m.Service.GetLoyverseEvents(context.Background(), m.ActiveSession.ID)
		m.Cursor = 0
		return m, nil
	default:
		m.SyncModel, cmd = m.SyncModel.Update(msg)
		return m, cmd
	}
	return m, nil
}
