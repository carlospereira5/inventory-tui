package tui

import (
	"fmt"
	"inventory-tui/internal/domain/entity"
	"inventory-tui/internal/ui/tui/styles"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renderiza la interfaz completa de la aplicación.
func (m Model) View() string {
	var body string
	var footer string

	// Cabecera: breadcrumb + estados de catálogo y grupos
	catalogStatus := m.CatalogStatus
	if m.CatalogIsError {
		catalogStatus = styles.ErrorStyle.Render(fmt.Sprintf("[%s]", catalogStatus))
	} else {
		catalogStatus = styles.SuccessStyle.Render(fmt.Sprintf("[%s]", catalogStatus))
	}

	var groupsStatus string
	if m.GroupsStatus != "" {
		groupsStatus = styles.SuccessStyle.Render(fmt.Sprintf("[%s]", m.GroupsStatus))
	}

	headerParts := []string{m.breadcrumb(), "   ", catalogStatus}
	if groupsStatus != "" {
		headerParts = append(headerParts, " ", groupsStatus)
	}

	header := styles.HeaderStyle.Render(
		lipgloss.JoinHorizontal(lipgloss.Top, headerParts...),
	)

	// Cuerpo y pie de página según el estado actual
	switch m.State {
	case StateSessionList:
		body, footer = m.viewSessionList()
	case StateSessionCreate:
		body, footer = m.viewSessionCreate()
	case StateSessionRename:
		body, footer = m.viewSessionRename()
	case StateScanning:
		body, footer = m.viewScanning()
	case StateQuickAdd:
		body, footer = m.viewQuickAdd()
	case StateHistory:
		body, footer = m.viewHistory()
	case StateLoyverse:
		body, footer = m.viewLoyverse()
	case StateSyncConfirm:
		body, footer = m.viewSyncConfirm()
	case StateSyncLoyverse:
		body, footer = m.viewSyncLoyverse()
	case StateNewProduct:
		body, footer = m.viewNewProduct()
	case StateFilterCategories:
		body, footer = m.viewFilterCategories()
	case StateHelp:
		body, footer = m.viewHelp()
	}

	if m.Err != nil {
		body += "\n" + styles.ErrorStyle.Render(fmt.Sprintf("Error: %v", m.Err))
	}

	// Borde del window con color semántico según el estado actual
	borderColor := styles.PurpleColor
	switch m.State {
	case StateScanning, StateQuickAdd:
		borderColor = styles.BlueColor
	case StateHistory, StateLoyverse:
		borderColor = styles.GreenColor
	}

	windowW := m.Width - 6
	if windowW < 10 {
		windowW = 10
	}
	windowH := m.Height - 3
	if windowH < 5 {
		windowH = 5
	}
	content := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	return styles.WindowStyle.Copy().
		BorderForeground(borderColor).
		Width(windowW).
		Height(windowH).
		Render(content)
}

// breadcrumb genera la cadena de navegación contextual del header.
func (m Model) breadcrumb() string {
	base := styles.TitleStyle.Render("Inventario TUI")
	sep := styles.BreadcrumbSep

	sessionName := ""
	if m.ActiveSession != nil {
		sessionName = m.ActiveSession.Name
	}

	switch m.State {
	case StateSessionCreate:
		return base + sep + styles.Gray.Render("Nueva sesión")
	case StateSessionRename:
		return base + sep + styles.Gray.Render("Renombrar")
	case StateScanning, StateQuickAdd:
		if sessionName != "" {
			return base + sep + styles.Gray.Render(sessionName)
		}
	case StateHistory:
		if sessionName != "" {
			return base + sep + styles.Gray.Render(sessionName) + sep + styles.Gray.Render("Historial")
		}
	case StateLoyverse:
		if sessionName != "" {
			tab := "Totales"
			if m.LoyverseSubTab == 1 {
				tab = "Eventos"
			}
			return base + sep + styles.Gray.Render(sessionName) + sep + styles.Gray.Render(tab)
		}
	case StateSyncConfirm:
		return base + sep + styles.Gray.Render("Modo de Sync")
	case StateSyncLoyverse:
		return base + sep + styles.Gray.Render("Sync Loyverse")
	case StateNewProduct:
		return base + sep + styles.Gray.Render("Nuevo producto")
	case StateFilterCategories:
		return base + sep + styles.Gray.Render("Filtro Categorías")
	case StateHelp:
		return base + sep + styles.Gray.Render("Ayuda")
	}
	return base
}

// filteredHistory retorna los registros del historial filtrados por el término de búsqueda activo.
// Si no hay búsqueda activa o el término es vacío, devuelve el slice completo.
func (m Model) filteredHistory() []entity.Record {
	if !m.HistorySearchActive || m.HistorySearch.Value() == "" {
		return m.History
	}
	term := strings.ToLower(m.HistorySearch.Value())
	var result []entity.Record
	for _, r := range m.History {
		if strings.Contains(strings.ToLower(r.Name), term) {
			result = append(result, r)
		}
	}
	return result
}

// filteredTotals retorna los totales filtrados por el término de búsqueda activo.
func (m Model) filteredTotals() []entity.SessionTotals {
	if !m.TotalsSearchActive || m.TotalsSearch.Value() == "" {
		return m.Totals
	}
	term := strings.ToLower(m.TotalsSearch.Value())
	var result []entity.SessionTotals
	for _, t := range m.Totals {
		if strings.Contains(strings.ToLower(t.Name), term) {
			result = append(result, t)
		}
	}
	return result
}

// innerW devuelve el ancho útil de contenido dentro del borde y padding de la ventana.
// WindowStyle: RoundedBorder(2) + Padding(1,2) lateral(4) = 6 overhead de ancho.
// El área usable dentro del padding = (m.Width-6) - 4 = m.Width-10.
func (m Model) innerW() int {
	w := m.Width - 10
	if w < 20 {
		return 20
	}
	return w
}

// listH calcula la altura disponible para una lista scrolleable.
// staticLines = líneas no-lista en el body: título, gaps, barra de búsqueda, etc.
// La constante 8 representa el overhead combinado de ventana + header + footer
// (empíricamente: listH(4) = m.Height-12 coincide con el comportamiento correcto a 24 filas).
func (m Model) listH(staticLines int) int {
	h := m.Height - 8 - staticLines
	if h < 1 {
		return 1
	}
	return h
}

// getVisibleRows devuelve un subconjunto de filas visibles basado en el scroll offset y altura máxima.
// cursor es la posición del cursor actual, scrollOffset es un puntero al offset de scroll que se actualiza.
func (m Model) getVisibleRows(rows []string, cursor int, scrollOffset *int, maxHeight int) []string {
	if len(rows) == 0 {
		return rows
	}

	// Asegurar que el cursor sea visible
	if cursor < *scrollOffset {
		*scrollOffset = cursor
	} else if cursor >= *scrollOffset+maxHeight {
		*scrollOffset = cursor - maxHeight + 1
	}

	// Limitar scroll offset
	if *scrollOffset < 0 {
		*scrollOffset = 0
	}
	if *scrollOffset >= len(rows) {
		*scrollOffset = len(rows) - 1
	}

	// Extraer filas visibles
	end := *scrollOffset + maxHeight
	if end > len(rows) {
		end = len(rows)
	}
	return rows[*scrollOffset:end]
}
