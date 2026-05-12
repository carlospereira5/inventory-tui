package tui

import (
	"fmt"
	"strings"

	"inventory-tui/internal/ui/tui/styles"
)

// viewNewProduct renderiza el overlay de alta de producto desconocido a Loyverse.
func (m Model) viewNewProduct() (string, string) {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Producto desconocido"))
	b.WriteString("\n")
	b.WriteString(styles.Gray.Render(fmt.Sprintf("Código: %s", m.NewProductBarcode)))
	b.WriteString("\n\n")

	switch m.NewProductStep {
	case newProductStepName:
		b.WriteString(styles.Blue.Render("Paso 1/3 — Nombre del producto"))
		b.WriteString("\n")
		b.WriteString(m.NewProductNameInput.View())

	case newProductStepPrice:
		b.WriteString(styles.Gray.Render("Nombre: ") + m.NewProductNameInput.Value())
		b.WriteString("\n\n")
		b.WriteString(styles.Blue.Render("Paso 2/3 — Precio de venta"))
		b.WriteString("\n")
		b.WriteString(m.NewProductPriceInput.View())

	case newProductStepCategory:
		b.WriteString(styles.Gray.Render("Nombre: ") + m.NewProductNameInput.Value())
		b.WriteString("\n")
		b.WriteString(styles.Gray.Render("Precio: $") + m.NewProductPriceInput.Value())
		b.WriteString("\n\n")
		b.WriteString(styles.Blue.Render("Paso 3/3 — Categoría"))
		b.WriteString("\n")
		if len(m.NewProductCategories) == 0 {
			b.WriteString(styles.Gray.Render("Cargando categorías..."))
		} else {
			for i, cat := range m.NewProductCategories {
				if i == m.NewProductCatCursor {
					b.WriteString(styles.Blue.Render("▸ ") + styles.SelectedStyle.Render(cat.Name))
				} else {
					b.WriteString("  " + cat.Name)
				}
				b.WriteString("\n")
			}
		}

	case newProductStepConfirm:
		catName := "(sin categoría)"
		if m.NewProductCatCursor < len(m.NewProductCategories) {
			catName = m.NewProductCategories[m.NewProductCatCursor].Name
		}
		b.WriteString(styles.Blue.Render("Confirmar creación"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("  Código:    %s\n", m.NewProductBarcode))
		b.WriteString(fmt.Sprintf("  Nombre:    %s\n", m.NewProductNameInput.Value()))
		b.WriteString(fmt.Sprintf("  Precio:    $%s\n", m.NewProductPriceInput.Value()))
		b.WriteString(fmt.Sprintf("  Categoría: %s\n", catName))
		b.WriteString("\n")
		b.WriteString(styles.Gray.Render("Presioná Enter para crear en Loyverse."))
	}

	if m.NewProductErr != "" {
		b.WriteString("\n")
		b.WriteString(styles.ErrorStyle.Render(m.NewProductErr))
	}

	footer := styles.HelpStyle.Render("enter: Continuar | esc: Cancelar")
	return b.String(), footer
}
