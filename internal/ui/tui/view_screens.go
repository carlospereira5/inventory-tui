package tui

import (
	"fmt"
	"github.com/carlospereira5/inventory-tui/internal/ui/tui/styles"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// formatSessionDate convierte la fecha de SQLite al formato "DD/MM/YYYY HH:MM hrs".
// Maneja tanto el formato ISO 8601 (T y Z) como el de CURRENT_TIMESTAMP (espacio).
func formatSessionDate(s string) string {
	for _, layout := range []string{
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("02/01/2006 15:04 hrs")
		}
	}
	return s // fallback: devuelve el string original sin cambios
}

func (m Model) viewSessionList() (string, string) {
	title := styles.SelectedStyle.Render("📁 SESIONES DE INVENTARIO")
	msg := ""
	if m.StatusMsg != "" {
		msg = styles.Gray.Render("ℹ️ " + m.StatusMsg)
	}

	var rows []string
	if len(m.Sessions) == 0 {
		rows = append(rows, styles.Gray.Render("  (No hay sesiones. Pulsa 'N' para crear una)"))
	} else {
		for i, ses := range m.Sessions {
			cursor := "  "
			checkbox := "[ ]"
			if m.SelectedSessions[ses.ID] {
				checkbox = styles.Green.Render("[✓]")
			}
			row := fmt.Sprintf("%s %s (%s)", checkbox, ses.Name, formatSessionDate(ses.CreatedAt))
			if m.Cursor == i {
				cursor = styles.Blue.Render("▸ ")
				rows = append(rows, cursor+styles.SelectedStyle.Render(row))
			} else {
				rows = append(rows, cursor+row)
			}
		}
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		msg,
		"",
		strings.Join(rows, "\n"),
	)

	help := styles.HelpStyle.Render("n: nueva  enter: entrar  r: renombrar  e: exportar  d: borrar  espacio: seleccionar  s: sync  q: salir  ?: ayuda")
	if m.PendingDelete {
		help = styles.WarnStyle.Render("⚠  d: confirmar borrado de sesión   cualquier otra tecla: cancelar")
	}
	return body, help
}

func (m Model) viewSessionCreate() (string, string) {
	title := styles.SelectedStyle.Render("✨ NUEVA SESIÓN")
	prompt := "Escribe el nombre del almacén o sección:"

	body := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		prompt,
		"",
		m.SessionInput.View(),
	)

	help := styles.HelpStyle.Render("enter: crear  esc: cancelar")
	return body, help
}

func (m Model) viewSessionRename() (string, string) {
	var sessionName string
	if len(m.Sessions) > 0 {
		sessionName = m.Sessions[m.Cursor].Name
	}
	title := styles.SelectedStyle.Render("✏️  RENOMBRAR SESIÓN")
	prompt := fmt.Sprintf("Nuevo nombre para %q:", sessionName)

	body := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		prompt,
		"",
		m.SessionInput.View(),
	)

	help := styles.HelpStyle.Render("enter: guardar  esc: cancelar")
	return body, help
}

func (m Model) viewScanning() (string, string) {
	// Panel de Sesión
	sessionInfo := styles.Purple.Render(fmt.Sprintf("📍 %s", m.ActiveSession.Name))

	// Los dos paneles deben caber dentro de innerW.
	// Cada caja tiene Border(2) + Padding(2) = 4 chars de overhead; el gap = 2.
	// Budget total para los dos Width() = innerW - 4 - 2 - 4 = innerW - 10.
	budget := m.innerW() - 10
	if budget < 24 {
		budget = 24
	}
	scanW := budget * 6 / 10
	infoW := budget - scanW

	// Panel de Escaneo Principal
	scanBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1).
		Width(scanW).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			"🔍 ESCANEANDO...",
			"",
			m.TextInput.View(),
		))

	// Panel Lateral de Info
	var lastItem string
	if m.LastScanned != nil {
		lastItem = lipgloss.JoinVertical(lipgloss.Left,
			styles.Green.Render("ÚLTIMO REGISTRO:"),
			styles.SelectedStyle.Render(m.LastScanned.Name),
			styles.Gray.Render(fmt.Sprintf("Bar: %s", m.LastScanned.Barcode)),
			styles.Purple.Render(fmt.Sprintf("Contador: %d", m.ConsecutiveCount)),
		)
	} else {
		lastItem = styles.Gray.Render("Esperando escaneo...")
	}

	infoBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1).
		Width(infoW).
		Render(lastItem)

	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, scanBox, "  ", infoBox)

	status := ""
	if m.StatusMsg != "" {
		status = "\n" + styles.Gray.Render("💡 "+m.StatusMsg)
	}

	// Indicador de filtro de categorías activo.
	var categoryIndicator string
	if len(m.FilterCategories) > 0 {
		active := len(m.ActiveCategoryIDs)
		if active == 0 {
			categoryIndicator = "\n" + styles.Gray.Render("⚡ Webhook: sin filtro de categorías (c: configurar)")
		} else {
			categoryIndicator = "\n" + styles.Green.Render(fmt.Sprintf("⚡ Webhook: %d categoría(s) activa(s) (c: configurar)", active))
		}
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		sessionInfo,
		"",
		mainContent,
		status,
		categoryIndicator,
	)

	var help string
	if m.LastScanned != nil {
		help = styles.HelpStyle.Render("tab: ver totales  +: suma rápida  c: categorías webhook  e: exportar  esc: menú  ?: ayuda")
	} else {
		help = styles.HelpStyle.Render("tab: ver totales  c: categorías webhook  e: exportar  esc: menú  ?: ayuda")
	}
	return body, help
}

func (m Model) viewQuickAdd() (string, string) {
	title := styles.SelectedStyle.Render("⚡ SUMA RÁPIDA")

	var productInfo string
	if m.LastScanned != nil {
		productInfo = lipgloss.JoinVertical(lipgloss.Left,
			styles.Green.Render("Producto:"),
			styles.SelectedStyle.Render("  "+m.LastScanned.Name),
			styles.Gray.Render(fmt.Sprintf("  Barcode: %s", m.LastScanned.Barcode)),
			styles.Purple.Render(fmt.Sprintf("  Total actual: %d", m.LastScanned.Quantity)),
		)
	}

	hint := styles.Gray.Render("Cantidades comunes: 5 · 10 · 20 · 50 · 100")

	body := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		productInfo,
		"",
		"Cantidad a agregar:",
		"",
		m.QuickAddInput.View(),
		"",
		hint,
	)

	help := styles.HelpStyle.Render("enter: confirmar  esc: cancelar")
	return body, help
}

func (m Model) viewSyncConfirm() (string, string) {
	title := styles.TitleStyle.Render("Modo de Sincronización con Loyverse")

	sessionCount := len(m.SyncModel.SessionIDs)
	info := styles.Gray.Render(fmt.Sprintf("%d sesión(es) seleccionada(s) para sincronizar.", sessionCount))

	options := []struct {
		label string
		desc  string
	}{
		{
			label: "Reemplazar stock",
			desc:  "Sobreescribe el stock en Loyverse con los conteos del inventario local.",
		},
		{
			label: "Sumar al stock",
			desc:  "Suma los conteos locales al stock actual en Loyverse (útil tras ventas).",
		},
	}

	var rows []string
	for i, opt := range options {
		cursor := "  "
		label := opt.label
		desc := styles.Gray.Render("  " + opt.desc)
		if m.Cursor == i {
			cursor = styles.Blue.Render("▸ ")
			label = styles.SelectedStyle.Render(label)
		}
		rows = append(rows, cursor+label, desc)
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		info,
		"",
		strings.Join(rows, "\n"),
	)

	help := styles.HelpStyle.Render("↑↓: navegar  enter: confirmar  esc: volver")
	return body, help
}

func (m Model) viewHistory() (string, string) {
	title := styles.Purple.Render(fmt.Sprintf("📜 HISTORIAL: %s", m.ActiveSession.Name))

	contentWidth := m.innerW()

	// staticLines = título(1) + gap(1) + searchBar(1 inactiva / 2 activa) + gap(1)
	staticLines := 4
	if m.HistorySearchActive {
		staticLines = 5 // +1 por la línea de summary
	}
	contentHeight := m.listH(staticLines)

	filtered := m.filteredHistory()

	nameWidth := contentWidth / 3
	if nameWidth < 10 {
		nameWidth = 10
	}

	var rows []string
	if len(filtered) == 0 {
		if m.HistorySearchActive {
			rows = append(rows, styles.Gray.Render("  (Sin resultados para la búsqueda)"))
		} else {
			rows = append(rows, styles.Gray.Render("  (Sin movimientos)"))
		}
	} else {
		for i, r := range filtered {
			cursor := "  "
			icon := "+"
			qty := styles.Green.Render(fmt.Sprintf("%+d", r.Quantity))
			if r.Quantity < 0 {
				icon = "-"
				qty = styles.Red.Render(fmt.Sprintf("%+d", r.Quantity))
			}
			row := fmt.Sprintf("%s %-*s | %s | %s", icon, nameWidth, r.Name, qty, r.Barcode)
			if m.Cursor == i {
				cursor = styles.Blue.Render("▸ ")
				rows = append(rows, cursor+styles.SelectedStyle.Render(row))
			} else {
				rows = append(rows, cursor+row)
			}
		}
	}

	visibleRows := m.getVisibleRows(rows, m.Cursor, &m.HistoryScrollOffset, contentHeight)
	historyBox := lipgloss.NewStyle().
		Width(contentWidth).
		Height(contentHeight).
		Render(strings.Join(visibleRows, "\n"))

	// Construir la barra de búsqueda y el summary
	var searchBar string
	if m.HistorySearchActive {
		total := 0
		for _, r := range filtered {
			total += r.Quantity
		}
		summary := styles.Gray.Render(fmt.Sprintf("  %d registros · total: %d unidades", len(filtered), total))
		searchBar = lipgloss.JoinVertical(lipgloss.Left,
			styles.Blue.Render("/ ")+m.HistorySearch.View(),
			summary,
		)
	} else {
		searchBar = styles.Gray.Render("/: buscar")
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		searchBar,
		"",
		historyBox,
	)

	help := styles.HelpStyle.Render("tab: escaneo  ↑↓: navegar  /: buscar  d: borrar  esc: menú  ?: ayuda")
	if m.HistorySearchActive {
		help = styles.HelpStyle.Render("↑↓: navegar resultados  esc: cerrar búsqueda  tab: ir a escaneo")
	}
	if m.PendingDelete {
		help = styles.WarnStyle.Render("⚠  d: confirmar borrado   cualquier otra tecla: cancelar")
	}
	return body, help
}

func (m Model) viewLoyverse() (string, string) {
	title := styles.Purple.Render(fmt.Sprintf("📊 TOTALES Y LOYVERSE: %s", m.ActiveSession.Name))

	contentWidth := m.innerW()
	// Body Loyverse: título(1) + gap(1) + tabbar(1) + gap(1) = 4 líneas estáticas.
	// El resto es el área del tab activo (que internamente resta header y search).
	contentHeight := m.listH(4)

	// Barra de tabs
	const tabTotales = " TOTALES "
	const tabEventos = " EVENTOS "
	var tab0, tab1 string
	if m.LoyverseSubTab == 0 {
		tab0 = styles.TitleStyle.Render(tabTotales)
		tab1 = styles.Gray.Render(tabEventos)
	} else {
		tab0 = styles.Gray.Render(tabTotales)
		tab1 = styles.TitleStyle.Render(tabEventos)
	}
	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tab0, "  ", tab1)

	// Contenido del tab activo
	var tabContent string
	if m.LoyverseSubTab == 0 {
		tabContent = m.viewLoyverseTotals(contentWidth, contentHeight)
	} else {
		tabContent = m.viewLoyverseEvents(contentWidth, contentHeight)
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		tabBar,
		"",
		tabContent,
	)

	var help string
	if m.TotalsSearchActive {
		help = styles.HelpStyle.Render("↑↓: navegar resultados  esc: cerrar búsqueda  tab: ir a historial")
	} else {
		help = styles.HelpStyle.Render("tab: historial  ←→: cambiar panel  ↑↓: navegar  /: buscar  d: borrar evento  esc: menú  ?: ayuda")
	}
	if m.PendingDelete {
		help = styles.WarnStyle.Render("⚠  d: confirmar borrado de evento   cualquier otra tecla: cancelar")
	}
	return body, help
}

// viewLoyverseTotals renderiza el tab de totales con scroll completo y search bar.
func (m Model) viewLoyverseTotals(contentWidth, contentHeight int) string {
	nameWidth := contentWidth - 12
	if nameWidth < 10 {
		nameWidth = 10
	}

	filtered := m.filteredTotals()

	header := styles.Green.Render(fmt.Sprintf("  %-*s | CANT", nameWidth, "PRODUCTO"))
	var rows []string
	if len(filtered) == 0 {
		if m.TotalsSearchActive {
			rows = append(rows, styles.Gray.Render("  (Sin resultados para la búsqueda)"))
		} else {
			rows = append(rows, styles.Gray.Render("  (Sin productos contados)"))
		}
	} else {
		for i, t := range filtered {
			cursor := "  "
			qty := fmt.Sprintf("%4d", t.Quantity)
			if t.Quantity < 0 {
				qty = styles.Red.Render(fmt.Sprintf("%4d", t.Quantity))
			}
			row := fmt.Sprintf("%-*s | %s", nameWidth, t.Name, qty)
			if m.TotalsCursor == i {
				cursor = styles.Blue.Render("▸ ")
				rows = append(rows, cursor+styles.SelectedStyle.Render(row))
			} else {
				rows = append(rows, cursor+row)
			}
		}
	}

	// listHeight descuenta: header(1) + searchLine(1 inactiva / 2 activa)
	listHeight := contentHeight - 2 // header(1) + "/: buscar"(1)
	var searchLine string
	if m.TotalsSearchActive {
		listHeight = contentHeight - 3 // header(1) + input(1) + summary(1)
		total := 0
		for _, t := range filtered {
			total += t.Quantity
		}
		summary := styles.Gray.Render(fmt.Sprintf("  %d productos · total: %d unidades", len(filtered), total))
		searchLine = lipgloss.JoinVertical(lipgloss.Left,
			styles.Blue.Render("/ ")+m.TotalsSearch.View(),
			summary,
		)
	} else {
		searchLine = styles.Gray.Render("/: buscar")
	}
	if listHeight < 1 {
		listHeight = 1
	}

	visibleRows := m.getVisibleRows(rows, m.TotalsCursor, &m.TotalsScrollOffset, listHeight)

	box := lipgloss.NewStyle().
		Width(contentWidth).
		Height(contentHeight).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			header,
			searchLine,
			strings.Join(visibleRows, "\n"),
		))

	return box
}

// viewLoyverseEvents renderiza el tab de eventos Loyverse con scroll completo.
func (m Model) viewLoyverseEvents(contentWidth, contentHeight int) string {
	nameWidth := contentWidth/2 - 6
	if nameWidth < 10 {
		nameWidth = 10
	}

	header := styles.Green.Render(fmt.Sprintf("  %-*s | CANT | GRUPO", nameWidth, "PRODUCTO"))
	var rows []string
	if len(m.LoyverseEvents) == 0 {
		rows = append(rows, styles.Gray.Render("  (Sin eventos de Loyverse)"))
	} else {
		for i, e := range m.LoyverseEvents {
			icon := "▼"
			if e.Quantity > 0 {
				icon = "▲"
			}
			groupLabel := e.GroupName
			if groupLabel == "" {
				groupLabel = "—"
			}
			cursor := "  "
			row := fmt.Sprintf("%s %-*s | %4d | %s", icon, nameWidth, e.Name, e.Quantity, groupLabel)
			if m.Cursor == i {
				cursor = styles.Blue.Render("▸ ")
				rows = append(rows, cursor+styles.SelectedStyle.Render(row))
			} else {
				rows = append(rows, cursor+row)
			}
		}
	}

	visibleRows := m.getVisibleRows(rows, m.Cursor, &m.LoyverseScrollOffset, contentHeight-1)

	box := lipgloss.NewStyle().
		Width(contentWidth).
		Height(contentHeight).
		Render(lipgloss.JoinVertical(lipgloss.Left, header, strings.Join(visibleRows, "\n")))

	return box
}

func (m Model) viewSyncLoyverse() (string, string) {
	m.SyncModel.Width = m.Width
	body := m.SyncModel.View()
	return body, ""
}

func (m Model) viewHelp() (string, string) {
	title := styles.SelectedStyle.Render("AYUDA — ATAJOS DE TECLADO")

	sec := func(name string) string {
		return "\n" + styles.Purple.Render("── "+name)
	}
	row := func(k, desc string) string {
		return fmt.Sprintf("  %-20s %s", styles.Blue.Render(k), styles.Gray.Render(desc))
	}

	lines := []string{
		title,
		sec("LISTA DE SESIONES"),
		row("n", "nueva sesión"),
		row("r", "renombrar seleccionada"),
		row("enter", "entrar a sesión"),
		row("e", "exportar a CSV"),
		row("d d", "borrar sesión (doble d)"),
		row("espacio", "seleccionar para sync"),
		row("s", "sync sesiones seleccionadas"),
		row("q", "salir"),
		sec("ESCANEO"),
		row("enter", "registrar producto escaneado"),
		row("+", "suma rápida de cantidad"),
		row("tab", "ver totales y eventos Loyverse"),
		row("e", "exportar sesión a CSV"),
		row("esc", "volver al menú"),
		sec("HISTORIAL"),
		row("↑↓ / k/j", "navegar"),
		row("/", "activar búsqueda por nombre"),
		row("d d", "borrar escaneo (doble d)"),
		row("tab", "ir a pantalla de escaneo"),
		row("esc", "volver al menú (o cerrar búsqueda)"),
		sec("TOTALES / EVENTOS LOYVERSE"),
		row("←→ / h/l", "cambiar entre Totales y Eventos"),
		row("↑↓ / k/j", "navegar"),
		row("/", "buscar en tab Totales"),
		row("d d", "borrar evento Loyverse (doble d)"),
		row("tab", "ir a historial"),
		row("esc", "volver al menú (o cerrar búsqueda)"),
		sec("SYNC LOYVERSE"),
		row("enter", "iniciar sincronización"),
		row("esc", "volver a lista de sesiones"),
		sec("GLOBAL"),
		row("?", "abrir / cerrar esta ayuda"),
		row("ctrl+c", "salir forzado"),
	}

	body := strings.Join(lines, "\n")
	help := styles.HelpStyle.Render("cualquier tecla: cerrar")
	return body, help
}
