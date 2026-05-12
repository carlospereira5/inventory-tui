package tui

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// Update es el corazón de la lógica de Bubble Tea, despacha mensajes a manejadores específicos.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case MsgCatalogLoaded: // Informa si el catálogo CSV se cargó correctamente.
		if msg.Err != nil {
			slog.Error("TUI: catálogo fallido", "err", msg.Err)
			m.CatalogStatus = fmt.Sprintf("Catálogo fallido: %v", msg.Err)
			m.CatalogIsError = true
		} else {
			slog.Info("TUI: catálogo cargado", "file", msg.File, "count", msg.Count)
			m.CatalogStatus = fmt.Sprintf("Catálogo: %s (%d p)", msg.File, msg.Count)
			m.CatalogIsError = false
		}
		return m, nil

	case MsgGroupsLoaded: // Informa si el CSV de grupos se cargó correctamente.
		if msg.Err != nil {
			// grupos.csv es legacy — si falla, no mostrar error (normal cuando se usa Loyverse).
			slog.Warn("TUI: grupos CSV no disponible (modo legacy)", "err", msg.Err)
			m.GroupsStatus = ""
			m.GroupsIsError = false
		} else {
			slog.Info("TUI: grupos cargados", "file", msg.File, "count", msg.Count)
			m.GroupsStatus = fmt.Sprintf("Grupos: %s (%d g)", msg.File, msg.Count)
			m.GroupsIsError = false
		}
		return m, nil

	case MsgSessionsLoaded: // Carga las sesiones en el menú principal.
		if msg.Err == nil {
			m.Sessions = msg.Sessions
			slog.Debug("TUI: sesiones cargadas", "count", len(msg.Sessions))
		}
		return m, nil

	case MsgTotalsLoaded: // Carga los totales de la sesión activa.
		if msg.Err == nil {
			m.Totals = msg.Totals
			slog.Debug("TUI: totales cargados", "count", len(msg.Totals))
		}
		return m, nil

	case MsgLoyverseEventsLoaded: // Carga los eventos de Loyverse de la sesión activa.
		if msg.Err == nil {
			m.LoyverseEvents = msg.Events
			slog.Debug("TUI: eventos Loyverse cargados", "count", len(msg.Events))
		}
		return m, nil

	case tea.WindowSizeMsg: // Ajusta el tamaño de la ventana y propaga al SyncModel.
		m.Width, m.Height = msg.Width, msg.Height
		var syncCmd tea.Cmd
		m.SyncModel, syncCmd = m.SyncModel.Update(msg)
		return m, syncCmd

	case tea.KeyMsg: // Atajos de teclado globales (ej. Ctrl+C o Esc).
		return m.handleKeyPress(msg)

	case MsgCategoriesLoaded:
		m.NewProductCategories = msg.Categories
		if msg.Err != nil {
			m.NewProductErr = fmt.Sprintf("Error cargando categorías: %v", msg.Err)
		}
		return m, nil

	case MsgFilterCategoriesLoaded:
		if msg.Err == nil {
			m.FilterCategories = msg.Categories
			m.ActiveCategoryIDs = msg.ActiveCategoryIDs
		}
		return m, nil

	case MsgCategoryToggled:
		if msg.Err == nil {
			m.ActiveCategoryIDs[msg.CategoryID] = msg.Active
			if !msg.Active {
				delete(m.ActiveCategoryIDs, msg.CategoryID)
			}
		}
		return m, nil

	case MsgProductCreated:
		if msg.Err != nil {
			m.NewProductErr = fmt.Sprintf("Error: %v", msg.Err)
			return m, nil
		}
		name := msg.Product.Name
		m = m.cancelNewProduct()
		m.StatusMsg = fmt.Sprintf("✓ %s creado en Loyverse", name)
		return m, nil

	case SyncCompletedMsg, SyncErrorMsg:
		var cmd tea.Cmd
		m.SyncModel, cmd = m.SyncModel.Update(msg)
		return m, cmd

	case spinner.TickMsg:
		// Rutear ticks del spinner al SyncModel solo cuando está sincronizando.
		if m.State == StateSyncLoyverse {
			var cmd tea.Cmd
			m.SyncModel, cmd = m.SyncModel.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	return m, nil
}

// handleKeyPress redirige la tecla presionada según el estado actual de la pantalla.
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}

	// '?' abre el help desde cualquier pantalla (excepto desde el propio help).
	if msg.String() == "?" && m.State != StateHelp {
		m.HelpPrevState = m.State
		m.State = StateHelp
		return m, nil
	}

	// Desde el help, cualquier tecla vuelve al estado anterior.
	if m.State == StateHelp {
		m.State = m.HelpPrevState
		return m, nil
	}

	switch m.State {
	case StateSessionList:
		return m.handleSessionListKeys(msg)
	case StateSessionCreate:
		return m.handleSessionCreateKeys(msg)
	case StateSessionRename:
		return m.handleSessionRenameKeys(msg)
	case StateScanning:
		return m.handleScanningKeys(msg)
	case StateQuickAdd:
		return m.handleQuickAddKeys(msg)
	case StateHistory:
		return m.handleHistoryKeys(msg)
	case StateLoyverse:
		return m.handleLoyverseKeys(msg)
	case StateSyncConfirm:
		return m.handleSyncConfirmKeys(msg)
	case StateSyncLoyverse:
		return m.handleSyncKeys(msg)
	case StateNewProduct:
		return m.handleNewProductKeys(msg)
	case StateFilterCategories:
		return m.handleFilterCategoriesKeys(msg)
	}

	return m, nil
}

// updateSessions es una utilidad para refrescar la lista de sesiones.
func (m *Model) updateSessions() {
	m.Sessions, _ = m.Service.GetSessions(context.Background())
}
