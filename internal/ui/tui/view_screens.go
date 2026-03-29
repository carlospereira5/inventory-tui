package tui

import (
	"fmt"
	"inventory-tui/internal/ui/tui/styles"
)

func (m Model) viewSessionList() (string, string) {
	body := styles.SelectedStyle.Render("MENÚ DE SESIONES") + "\n\n"
	if m.StatusMsg != "" { body += styles.Gray.Render(m.StatusMsg) + "\n\n" }
	body += "Selecciona o crea una sesión de conteo:\n\n"
	
	if len(m.Sessions) == 0 {
		body += " No hay sesiones. Presiona 'N' para crear una.\n\n"
	} else {
		for i, ses := range m.Sessions {
			cursor := " "
			if m.Cursor == i {
				cursor = ">"
				body += styles.SelectedStyle.Render(fmt.Sprintf("%s %s (%s)", cursor, ses.Name, ses.CreatedAt)) + "\n"
			} else {
				body += fmt.Sprintf("%s %s (%s)", cursor, ses.Name, ses.CreatedAt) + "\n"
			}
		}
	}
	return body, styles.HelpStyle.Render("N: Nueva • Enter: Entrar • E: Exportar • D: Borrar • Q: Salir")
}

func (m Model) viewSessionCreate() (string, string) {
	body := styles.SelectedStyle.Render("NUEVA SESIÓN") + "\n\n"
	body += "Introduce el nombre para la nueva sesión:\n\n"
	body += m.SessionInput.View() + "\n\n"
	return body, styles.HelpStyle.Render("Enter: Crear • Esc: Cancelar")
}

func (m Model) viewScanning() (string, string) {
	body := styles.Purple.Render(fmt.Sprintf("SESIÓN ACTIVA: %s", m.ActiveSession.Name)) + "\n\n"
	body += m.TextInput.View() + "\n\n"
	if m.LastScanned != nil {
		body += "Último producto escaneado:\n"
		body += styles.Purple.Render(m.LastScanned.Name) + "\n"
		body += fmt.Sprintf("Código: %s | Cantidad: %d\n\n", m.LastScanned.Barcode, m.LastScanned.Quantity)
	}
	if m.StatusMsg != "" { body += styles.Gray.Render(m.StatusMsg) + "\n\n" }
	return body, styles.HelpStyle.Render("Tab: Historial • E: Exportar • Esc: Menú")
}

func (m Model) viewHistory() (string, string) {
	body := styles.Purple.Render(fmt.Sprintf("HISTORIAL DE SESIÓN: %s", m.ActiveSession.Name)) + "\n\n"
	if len(m.History) == 0 {
		body += " No hay escaneos en esta sesión.\n\n"
	} else {
		for i, r := range m.History {
			cursor := " "
			if m.Cursor == i {
				cursor = ">"
				body += styles.SelectedStyle.Render(fmt.Sprintf("%s %s: %d (Bar: %s)", cursor, r.Name, r.Quantity, r.Barcode)) + "\n"
			} else {
				body += fmt.Sprintf("%s %s: %d (Bar: %s)", cursor, r.Name, r.Quantity, r.Barcode) + "\n"
			}
		}
	}
	return body, styles.HelpStyle.Render("Tab: Escaneo • D: Borrar • Esc: Menú")
}
