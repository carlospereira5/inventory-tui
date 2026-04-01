package tui

import (
	"fmt"
	"inventory-tui/internal/ui/tui/styles"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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
			row := fmt.Sprintf("%s (%s)", ses.Name, ses.CreatedAt)
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

	help := styles.HelpStyle.Render("n: nueva • enter: entrar • e: exportar • d: borrar • q: salir")
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

	help := styles.HelpStyle.Render("enter: crear • esc: cancelar")
	return body, help
}

func (m Model) viewScanning() (string, string) {
	// Panel de Sesión
	sessionInfo := styles.Purple.Render(fmt.Sprintf("📍 %s", m.ActiveSession.Name))

	// Panel de Escaneo Principal
	scanBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1).
		Width(m.Width / 2).
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
		Width(m.Width / 3).
		Render(lastItem)

	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, scanBox, "  ", infoBox)

	status := ""
	if m.StatusMsg != "" {
		status = "\n" + styles.Gray.Render("💡 "+m.StatusMsg)
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		sessionInfo,
		"",
		mainContent,
		status,
	)

	help := styles.HelpStyle.Render("tab: ver totales • e: exportar • esc: menú")
	return body, help
}

func (m Model) viewHistory() (string, string) {
	title := styles.Purple.Render(fmt.Sprintf("📜 HISTORIAL: %s", m.ActiveSession.Name))

	contentWidth := m.Width - 10
	if contentWidth < 30 {
		contentWidth = 30
	}

	maxHeight := m.Height - 8
	if maxHeight < 5 {
		maxHeight = 5
	}

	var rows []string
	if len(m.History) == 0 {
		rows = append(rows, styles.Gray.Render("  (Sin movimientos)"))
	} else {
		for i, r := range m.History {
			cursor := "  "
			icon := "📦"
			if r.Quantity < 0 {
				icon = "🛒"
			}

			nameWidth := contentWidth / 3
			if nameWidth < 10 {
				nameWidth = 10
			}
			row := fmt.Sprintf("%s %-*s | %4d | %s", icon, nameWidth, r.Name, r.Quantity, r.Barcode)
			if m.Cursor == i {
				cursor = styles.Blue.Render("▸ ")
				rows = append(rows, cursor+styles.SelectedStyle.Render(row))
			} else {
				rows = append(rows, cursor+row)
			}
		}
	}

	// Aplicar scroll
	visibleRows := m.getVisibleRows(rows, m.Cursor, &m.HistoryScrollOffset, maxHeight)

	body := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		strings.Join(visibleRows, "\n"),
	)

	help := styles.HelpStyle.Render("tab: volver a escaneo • d: borrar • ↑↓ navegar • esc: menú")
	return body, help
}

func (m Model) viewLoyverse() (string, string) {
	title := styles.Purple.Render(fmt.Sprintf("📊 TOTALES Y LOYVERSE: %s", m.ActiveSession.Name))

	panelWidth := (m.Width - 16) / 2
	if panelWidth < 20 {
		panelWidth = 20
	}

	maxHeight := m.Height - 8
	if maxHeight < 5 {
		maxHeight = 5
	}

	// Panel izquierdo: Tabla de totales por producto
	var totalsRows []string
	totalsRows = append(totalsRows, styles.Green.Render("📦 PRODUCTO | CANT"))
	if len(m.Totals) == 0 {
		totalsRows = append(totalsRows, styles.Gray.Render("  (Sin productos contados)"))
	} else {
		nameWidth := panelWidth - 12
		if nameWidth < 8 {
			nameWidth = 8
		}
		for _, t := range m.Totals {
			row := fmt.Sprintf("  %-*s | %d", nameWidth, t.Name, t.Quantity)
			totalsRows = append(totalsRows, row)
		}
	}
	// Limitar altura del panel de totales
	if len(totalsRows) > maxHeight {
		totalsRows = totalsRows[:maxHeight]
	}
	totalsBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1).
		Width(panelWidth).
		Render(strings.Join(totalsRows, "\n"))

	// Panel derecho: Eventos de Loyverse con scroll
	var eventsRows []string
	eventsRows = append(eventsRows, styles.Green.Render("🛒 EVENTOS LOYVERSE"))
	if len(m.LoyverseEvents) == 0 {
		eventsRows = append(eventsRows, styles.Gray.Render("  (Sin eventos de Loyverse)"))
	} else {
		nameWidth := panelWidth / 3
		if nameWidth < 8 {
			nameWidth = 8
		}
		for i, e := range m.LoyverseEvents {
			icon := "🔻"
			if e.Quantity > 0 {
				icon = "🔺"
			}
			groupLabel := e.GroupName
			if groupLabel == "" {
				groupLabel = "sin grupo"
			}
			cursor := "  "
			if m.Cursor == i {
				cursor = styles.Blue.Render("▸ ")
			}
			row := fmt.Sprintf("%s%s %-*s | %3d | [%s]", cursor, icon, nameWidth, e.Name, e.Quantity, groupLabel)
			eventsRows = append(eventsRows, row)
		}
	}
	visibleEvents := m.getVisibleRows(eventsRows, m.Cursor, &m.LoyverseScrollOffset, maxHeight)
	eventsBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1).
		Width(panelWidth).
		Render(strings.Join(visibleEvents, "\n"))

	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, totalsBox, "  ", eventsBox)

	body := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		mainContent,
	)

	help := styles.HelpStyle.Render("tab: ver historial • d: borrar • ↑↓ navegar • esc: menú")
	return body, help
}
