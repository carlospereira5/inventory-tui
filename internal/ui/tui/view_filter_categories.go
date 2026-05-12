package tui

import (
	"fmt"
	"strings"

	"inventory-tui/internal/ui/tui/styles"
)

// viewFilterCategories renderiza el overlay de selección de categorías activas para webhook.
func (m Model) viewFilterCategories() (string, string) {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Filtro de Webhook por Categoría"))
	b.WriteString("\n")
	b.WriteString(styles.Gray.Render("Solo los productos de categorías activas generarán eventos al recibir ventas."))
	b.WriteString("\n\n")

	if len(m.FilterCategories) == 0 {
		b.WriteString(styles.Gray.Render("Cargando categorías de Loyverse..."))
	} else {
		activeCount := len(m.ActiveCategoryIDs)
		summary := fmt.Sprintf("%d de %d categorías activas", activeCount, len(m.FilterCategories))
		b.WriteString(styles.Blue.Render(summary))
		b.WriteString("\n\n")

		for i, cat := range m.FilterCategories {
			var checkbox string
			if m.ActiveCategoryIDs[cat.ID] {
				checkbox = styles.Green.Render("[✓]")
			} else {
				checkbox = styles.Gray.Render("[ ]")
			}

			line := fmt.Sprintf("%s %s", checkbox, cat.Name)
			if i == m.FilterCatCursor {
				b.WriteString(styles.Blue.Render("▸ ") + styles.SelectedStyle.Render(line))
			} else {
				b.WriteString("  " + line)
			}
			b.WriteString("\n")
		}
	}

	footer := styles.HelpStyle.Render("espacio: activar/desactivar  ↑↓/jk: navegar  esc: volver")
	return b.String(), footer
}
