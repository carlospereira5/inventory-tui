package tui

import (
	"context"
	"fmt"
	"inventory-tui/internal/domain/entity"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"
)

// MsgTotalsLoaded informa que los totales de la sesión están listos.
type MsgTotalsLoaded struct {
	Totals []entity.SessionTotals
	Err    error
}

// MsgLoyverseEventsLoaded informa que los eventos de Loyverse están listos.
type MsgLoyverseEventsLoaded struct {
	Events []entity.LoyverseEvent
	Err    error
}

// MsgCatalogLoaded informa que el CSV de productos se ha procesado.
type MsgCatalogLoaded struct {
	Count int
	File  string
	Err   error
}

// MsgGroupsLoaded informa que el CSV de grupos personalizados se ha procesado.
type MsgGroupsLoaded struct {
	Count int
	File  string
	Err   error
}

// MsgSessionsLoaded informa que la lista de sesiones está lista.
type MsgSessionsLoaded struct {
	Sessions []entity.Session
	Err      error
}

// CmdLoadCatalog inicia la búsqueda e importación del catálogo CSV.
func (m Model) CmdLoadCatalog() tea.Cmd {
	return func() tea.Msg {
		slog.Debug("comando: CmdLoadCatalog ejecutándose")
		count, file, err := m.Service.LoadCatalog(context.Background())
		if err != nil {
			slog.Warn("comando: CmdLoadCatalog fallido", "err", err)
		} else {
			slog.Debug("comando: CmdLoadCatalog completado", "count", count, "file", file)
		}
		return MsgCatalogLoaded{Count: count, File: file, Err: err}
	}
}

// CmdLoadSessions recupera la lista actualizada de sesiones desde el servicio.
func (m Model) CmdLoadSessions() tea.Cmd {
	return func() tea.Msg {
		slog.Debug("comando: CmdLoadSessions ejecutándose")
		sessions, err := m.Service.GetSessions(context.Background())
		if err != nil {
			slog.Warn("comando: CmdLoadSessions fallido", "err", err)
		} else {
			slog.Debug("comando: CmdLoadSessions completado", "count", len(sessions))
		}
		return MsgSessionsLoaded{Sessions: sessions, Err: err}
	}
}

// CmdLoadGroups inicia la búsqueda e importación del CSV de grupos personalizados.
func (m Model) CmdLoadGroups() tea.Cmd {
	return func() tea.Msg {
		slog.Debug("comando: CmdLoadGroups ejecutándose")
		count, file, err := m.Service.LoadGroups(context.Background())
		if err != nil {
			slog.Warn("comando: CmdLoadGroups fallido", "err", err)
		} else {
			slog.Debug("comando: CmdLoadGroups completado", "count", count, "file", file)
		}
		return MsgGroupsLoaded{Count: count, File: file, Err: err}
	}
}

// CmdLoadTotals recupera los totales de la sesión activa.
func (m Model) CmdLoadTotals() tea.Cmd {
	return func() tea.Msg {
		if m.ActiveSession == nil {
			return MsgTotalsLoaded{Totals: nil, Err: nil}
		}
		slog.Debug("comando: CmdLoadTotals ejecutándose", "session_id", m.ActiveSession.ID)
		totals, err := m.Service.GetSessionTotals(context.Background(), m.ActiveSession.ID)
		if err != nil {
			slog.Warn("comando: CmdLoadTotals fallido", "session_id", m.ActiveSession.ID, "err", err)
		}
		return MsgTotalsLoaded{Totals: totals, Err: err}
	}
}

// CmdLoadLoyverseEvents recupera los eventos de Loyverse de la sesión activa.
func (m Model) CmdLoadLoyverseEvents() tea.Cmd {
	return func() tea.Msg {
		if m.ActiveSession == nil {
			return MsgLoyverseEventsLoaded{Events: nil, Err: nil}
		}
		slog.Debug("comando: CmdLoadLoyverseEvents ejecutándose", "session_id", m.ActiveSession.ID)
		events, err := m.Service.GetLoyverseEvents(context.Background(), m.ActiveSession.ID)
		if err != nil {
			slog.Warn("comando: CmdLoadLoyverseEvents fallido", "session_id", m.ActiveSession.ID, "err", err)
		}
		return MsgLoyverseEventsLoaded{Events: events, Err: err}
	}
}

// Init define los comandos iniciales que se ejecutan al arrancar la app.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.CmdLoadCatalog(),
		m.CmdLoadGroups(),
		m.CmdLoadSessions(),
	)
}

// formatStatus ayuda a presentar mensajes de error o éxito al usuario.
func (m *Model) formatStatus(err error, successMsg string) {
	if err != nil {
		m.StatusMsg = fmt.Sprintf("Error: %v", err)
	} else {
		m.StatusMsg = successMsg
	}
}
