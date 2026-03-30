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
			styles.Purple.Render(fmt.Sprintf("Cant. Sesión: %d", m.LastScanned.Quantity)),
		)
	} else {
		lastItem = styles.Gray.Render("Esperando escaneo...")
	}

	infoBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1).
		Width(m.Width/3).
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

	help := styles.HelpStyle.Render("tab: ver historial • e: exportar • esc: menú")
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
			if r.Quantity < 0 { icon = "🛒" } // Representa una venta de Loyverse
			
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
