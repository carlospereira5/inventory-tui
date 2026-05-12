package tui

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
)

// deduplicateBarcode detecta barcodes que el escáner envió duplicados antes del Enter.
// Si el string tiene longitud par y su primera mitad es igual a la segunda, devuelve
// solo la primera mitad. En cualquier otro caso, devuelve el string sin cambios.
func deduplicateBarcode(s string) string {
	if len(s) == 0 || len(s)%2 != 0 {
		return s
	}
	half := len(s) / 2
	if s[:half] == s[half:] {
		return s[:half]
	}
	return s
}

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
		m.TotalsCursor = 0
		m.TotalsScrollOffset = 0
		m.LoyverseScrollOffset = 0
		m.LoyverseSubTab = 0
		m.TotalsSearchActive = false
		m.TotalsSearch.SetValue("")
		return m, nil
	case tea.KeyRunes:
		// Interceptar '+' para suma rápida solo si el input está vacío y hay un producto activo.
		if msg.String() == "+" && m.LastScanned != nil && m.TextInput.Value() == "" {
			slog.Debug("navegación: + → StateQuickAdd", "barcode", m.LastScanned.Barcode)
			m.State = StateQuickAdd
			m.QuickAddInput.SetValue("")
			m.QuickAddInput.Focus()
			return m, nil
		}
		// Interceptar 'c' para filtro de categorías (solo si input está vacío).
		if msg.String() == "c" && m.TextInput.Value() == "" {
			slog.Debug("navegación: c → StateFilterCategories")
			m.State = StateFilterCategories
			m.FilterCatCursor = 0
			m.TextInput.Blur()
			return m, m.CmdLoadFilterCategories()
		}
	case tea.KeyEnter:
		barcode := m.TextInput.Value()
		if barcode == "" {
			return m, nil
		}

		// Limpieza de duplicados por escáner.
		barcode = deduplicateBarcode(barcode)

		slog.Debug("escaneo", "barcode", barcode, "session_id", m.ActiveSession.ID)
		p, rec, err := m.Service.ScanProduct(context.Background(), m.ActiveSession.ID, barcode)
		if err != nil {
			slog.Error("escaneo fallido", "barcode", barcode, "err", err)
			m.Err = err
			return m, nil
		}
		if p == nil {
			slog.Warn("producto no encontrado, abriendo alta", "barcode", barcode)
			m.NewProductBarcode = barcode
			m.NewProductStep = 0
			m.NewProductErr = ""
			m.NewProductNameInput.SetValue("")
			m.NewProductPriceInput.SetValue("")
			m.NewProductCategories = nil
			m.NewProductCatCursor = 0
			m.NewProductNameInput.Focus()
			m.TextInput.Blur()
			m.State = StateNewProduct
			m.TextInput.SetValue("")
			return m, nil
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
	if m.PendingDelete && msg.String() != "d" && msg.String() != "backspace" {
		m.PendingDelete = false
	}

	// Con la búsqueda activa, la mayoría de teclas van al input.
	if m.HistorySearchActive {
		switch msg.Type {
		case tea.KeyEsc:
			m.HistorySearchActive = false
			m.HistorySearch.SetValue("")
			m.Cursor = 0
			return m, nil
		case tea.KeyTab:
			m.HistorySearchActive = false
			m.HistorySearch.SetValue("")
			m.State = StateScanning
			m.TextInput.Focus()
			return m, nil
		case tea.KeyUp:
			if m.Cursor > 0 {
				m.Cursor--
			}
			return m, nil
		case tea.KeyDown:
			filtered := m.filteredHistory()
			if m.Cursor < len(filtered)-1 {
				m.Cursor++
			}
			return m, nil
		default:
			var cmd tea.Cmd
			m.HistorySearch, cmd = m.HistorySearch.Update(msg)
			m.Cursor = 0 // resetear cursor al cambiar el término
			return m, cmd
		}
	}

	switch msg.String() {
	case "tab":
		slog.Debug("navegación: tab → StateScanning")
		m.State = StateScanning
		m.TextInput.Focus()
	case "esc":
		slog.Debug("navegación: esc → StateSessionList")
		m.State = StateSessionList
		m.updateSessions()
	case "/":
		m.HistorySearchActive = true
		m.HistorySearch.SetValue("")
		m.HistorySearch.Focus()
		m.Cursor = 0
		return m, nil
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
		}
	case "down", "j":
		filtered := m.filteredHistory()
		if m.Cursor < len(filtered)-1 {
			m.Cursor++
		}
	case "d", "backspace":
		filtered := m.filteredHistory()
		if len(filtered) > 0 && m.Cursor < len(filtered) {
			if !m.PendingDelete {
				m.PendingDelete = true
				return m, nil
			}
			m.PendingDelete = false
			scanID := filtered[m.Cursor].ID
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
			if m.Cursor >= len(m.filteredHistory()) && m.Cursor > 0 {
				m.Cursor--
			}
		}
	}
	return m, nil
}

// handleLoyverseKeys gestiona la navegación en la pantalla de totales y eventos de Loyverse.
func (m Model) handleLoyverseKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.PendingDelete && msg.String() != "d" && msg.String() != "backspace" {
		m.PendingDelete = false
	}

	// Con búsqueda de totales activa, la mayoría de teclas van al input.
	if m.TotalsSearchActive {
		switch msg.Type {
		case tea.KeyEsc:
			m.TotalsSearchActive = false
			m.TotalsSearch.SetValue("")
			m.TotalsCursor = 0
			return m, nil
		case tea.KeyTab:
			m.TotalsSearchActive = false
			m.TotalsSearch.SetValue("")
			m.State = StateHistory
			var err error
			m.History, err = m.Service.GetHistory(context.Background(), m.ActiveSession.ID)
			if err != nil {
				slog.Error("loyverse: error cargando historial", "err", err)
				m.Err = err
			}
			m.Cursor = 0
			return m, nil
		case tea.KeyUp:
			if m.TotalsCursor > 0 {
				m.TotalsCursor--
			}
			return m, nil
		case tea.KeyDown:
			filtered := m.filteredTotals()
			if m.TotalsCursor < len(filtered)-1 {
				m.TotalsCursor++
			}
			return m, nil
		default:
			var cmd tea.Cmd
			m.TotalsSearch, cmd = m.TotalsSearch.Update(msg)
			m.TotalsCursor = 0
			return m, cmd
		}
	}

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
		m.HistorySearchActive = false
		m.HistorySearch.SetValue("")
	case "esc":
		slog.Debug("navegación: esc → StateSessionList")
		m.State = StateSessionList
		m.updateSessions()

	// Activar búsqueda con '/': solo en el tab activo
	case "/":
		if m.LoyverseSubTab == 0 {
			m.TotalsSearchActive = true
			m.TotalsSearch.SetValue("")
			m.TotalsSearch.Focus()
			m.TotalsCursor = 0
		}
		return m, nil

	// Cambio de tab: izquierda/derecha o h/l (cancela búsqueda al cambiar)
	case "left", "h":
		if m.LoyverseSubTab > 0 {
			m.LoyverseSubTab--
			m.TotalsSearchActive = false
			m.TotalsSearch.SetValue("")
			m.TotalsCursor = 0
		}
	case "right", "l":
		if m.LoyverseSubTab < 1 {
			m.LoyverseSubTab++
			m.TotalsSearchActive = false
			m.TotalsSearch.SetValue("")
			m.TotalsCursor = 0
		}

	// Navegación vertical: opera sobre el tab activo
	case "up", "k":
		if m.LoyverseSubTab == 0 {
			if m.TotalsCursor > 0 {
				m.TotalsCursor--
			}
		} else {
			if m.Cursor > 0 {
				m.Cursor--
			}
		}
	case "down", "j":
		if m.LoyverseSubTab == 0 {
			filtered := m.filteredTotals()
			if m.TotalsCursor < len(filtered)-1 {
				m.TotalsCursor++
			}
		} else {
			if m.Cursor < len(m.LoyverseEvents)-1 {
				m.Cursor++
			}
		}

	// Borrado: solo disponible en el tab de Eventos
	case "d", "backspace":
		if m.LoyverseSubTab == 1 && len(m.LoyverseEvents) > 0 && m.Cursor < len(m.LoyverseEvents) {
			if !m.PendingDelete {
				m.PendingDelete = true
				return m, nil
			}
			m.PendingDelete = false
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
	}
	return m, nil
}

// handleQuickAddKeys gestiona el overlay de suma rápida de cantidad al último producto escaneado.
func (m Model) handleQuickAddKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.Type {
	case tea.KeyEsc:
		slog.Debug("quick add: cancelado")
		m.State = StateScanning
		m.TextInput.Focus()
		return m, nil
	case tea.KeyEnter:
		val := m.QuickAddInput.Value()
		if val == "" {
			m.State = StateScanning
			m.TextInput.Focus()
			return m, nil
		}
		quantity, err := strconv.Atoi(val)
		if err != nil || quantity <= 0 {
			m.StatusMsg = "Ingresá un número entero positivo."
			return m, nil
		}
		rec, err := m.Service.AddQuickScan(context.Background(), m.ActiveSession.ID, m.LastScanned.Barcode, quantity)
		if err != nil {
			slog.Error("quick add: error agregando cantidad", "barcode", m.LastScanned.Barcode, "quantity", quantity, "err", err)
			m.Err = err
		} else {
			m.LastScanned = rec
			m.ConsecutiveCount += quantity
			m.StatusMsg = fmt.Sprintf("+%d unidades de %s", quantity, rec.Name)
			slog.Info("quick add: exitoso", "barcode", rec.Barcode, "name", rec.Name, "quantity", quantity, "total", rec.Quantity)
		}
		m.State = StateScanning
		m.TextInput.Focus()
		return m, nil
	}
	m.QuickAddInput, cmd = m.QuickAddInput.Update(msg)
	return m, cmd
}

// handleSyncKeys gestiona la navegación en la pantalla de sincronización.
func (m Model) handleSyncKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "enter":
		if m.SyncModel.State == SyncIdle || m.SyncModel.State == SyncError {
			m.SyncModel.State = SyncSyncing
			m.SyncModel.Help = "Sincronizando..."
			return m, tea.Batch(m.SyncModel.CmdSync(), m.SyncModel.Spinner.Tick)
		}
	case "esc":
		slog.Debug("navegación: sync esc → StateSessionList")
		m.State = StateSessionList
		m.updateSessions()
		m.Cursor = 0
		return m, nil
	default:
		m.SyncModel, cmd = m.SyncModel.Update(msg)
		return m, cmd
	}
	return m, nil
}
