package tui

import (
	"context"
	"fmt"
	"inventory-tui/internal/domain/entity"

	tea "github.com/charmbracelet/bubbletea"
)

// MsgCatalogLoaded informa que el CSV de productos se ha procesado.
type MsgCatalogLoaded struct {
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
		count, file, err := m.Service.LoadCatalog(context.Background())
		return MsgCatalogLoaded{Count: count, File: file, Err: err}
	}
}

// CmdLoadSessions recupera la lista actualizada de sesiones desde el servicio.
func (m Model) CmdLoadSessions() tea.Cmd {
	return func() tea.Msg {
		sessions, err := m.Service.GetSessions(context.Background())
		return MsgSessionsLoaded{Sessions: sessions, Err: err}
	}
}

// Init define los comandos iniciales que se ejecutan al arrancar la app.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.CmdLoadCatalog(),
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
