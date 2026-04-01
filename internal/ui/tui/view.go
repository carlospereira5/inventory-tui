package tui

import (
	"fmt"
	"inventory-tui/internal/ui/tui/styles"

	"github.com/charmbracelet/lipgloss"
)

// View renderiza la interfaz completa de la aplicación.
func (m Model) View() string {
	var body string
	var footer string

	// Renderizar Cabecera
	catalogStatus := m.CatalogStatus
	if m.CatalogIsError {
		catalogStatus = styles.ErrorStyle.Render(fmt.Sprintf("[%s]", catalogStatus))
	} else {
		catalogStatus = styles.SuccessStyle.Render(fmt.Sprintf("[%s]", catalogStatus))
	}

	groupsStatus := m.GroupsStatus
	if m.GroupsIsError {
		groupsStatus = styles.ErrorStyle.Render(fmt.Sprintf("[%s]", groupsStatus))
	} else {
		groupsStatus = styles.SuccessStyle.Render(fmt.Sprintf("[%s]", groupsStatus))
	}

	header := styles.HeaderStyle.Render(
		lipgloss.JoinHorizontal(lipgloss.Top,
			styles.TitleStyle.Render("Inventario TUI"), " ", catalogStatus, " ", groupsStatus,
		),
	)

	// Renderizar Cuerpo y Pie de página según el estado
	switch m.State {
	case StateSessionList:
		body, footer = m.viewSessionList()
	case StateSessionCreate:
		body, footer = m.viewSessionCreate()
	case StateScanning:
		body, footer = m.viewScanning()
	case StateHistory:
		body, footer = m.viewHistory()
	case StateLoyverse:
		body, footer = m.viewLoyverse()
	}

	if m.Err != nil {
		body += "\n" + styles.ErrorStyle.Render(fmt.Sprintf("Error: %v", m.Err))
	}

	// Componer la vista final dentro de un contenedor estilizado
	content := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	return styles.WindowStyle.Copy().
		Width(m.Width - 6).
		Height(m.Height - 3).
		Render(content)
}
