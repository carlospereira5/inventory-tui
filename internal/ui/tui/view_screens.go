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

	var rows []string
	if len(m.History) == 0 {
		rows = append(rows, "  (Sin movimientos)")
	} else {
		for i, r := range m.History {
			cursor := "  "
			icon := "📦"
			if r.Quantity < 0 {
				icon = "🛒"
			} // Representa una venta de Loyverse

			row := fmt.Sprintf("%s %-20s | %d | %s", icon, r.Name, r.Quantity, r.Barcode)
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
		strings.Join(rows, "\n"),
	)

	help := styles.HelpStyle.Render("tab: volver a escaneo • d: borrar entrada • esc: menú")
	return body, help
}

func (m Model) viewLoyverse() (string, string) {
	title := styles.Purple.Render(fmt.Sprintf("📊 TOTALES Y LOYVERSE: %s", m.ActiveSession.Name))

	// Panel izquierdo: Tabla de totales por producto
	var totalsRows []string
	totalsRows = append(totalsRows, styles.Green.Render("📦 PRODUCTO              | CANTIDAD"))
	if len(m.Totals) == 0 {
		totalsRows = append(totalsRows, styles.Gray.Render("  (Sin productos contados)"))
	} else {
		for _, t := range m.Totals {
			row := fmt.Sprintf("  %-22s | %d", t.Name, t.Quantity)
			totalsRows = append(totalsRows, row)
		}
	}
	totalsBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1).
		Width(m.Width/2 - 2).
		Render(strings.Join(totalsRows, "\n"))

	// Panel derecho: Eventos de Loyverse en orden cronológico
	var eventsRows []string
	eventsRows = append(eventsRows, styles.Green.Render("🛒 EVENTOS LOYVERSE"))
	if len(m.LoyverseEvents) == 0 {
		eventsRows = append(eventsRows, styles.Gray.Render("  (Sin eventos de Loyverse)"))
	} else {
		for _, e := range m.LoyverseEvents {
			icon := "🔻"
			if e.Quantity > 0 {
				icon = "🔺"
			}
			row := fmt.Sprintf("%s %-18s | %d | %s", icon, e.Name, e.Quantity, e.CreatedAt)
			eventsRows = append(eventsRows, row)
		}
	}
	eventsBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1).
		Width(m.Width/2 - 2).
		Render(strings.Join(eventsRows, "\n"))

	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, totalsBox, "  ", eventsBox)

	body := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		mainContent,
	)

	help := styles.HelpStyle.Render("tab: ver historial • esc: menú")
	return body, help
}
