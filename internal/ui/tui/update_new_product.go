package tui

import (
	"fmt"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	newProductStepName     = 0
	newProductStepPrice    = 1
	newProductStepCategory = 2
	newProductStepConfirm  = 3
)

// handleNewProductKeys gestiona el formulario multi-step para agregar un producto nuevo a Loyverse.
func (m Model) handleNewProductKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		return m.cancelNewProduct(), nil
	case tea.KeyEnter:
		return m.advanceNewProduct()
	}

	// Delegar teclas al input activo del paso actual.
	var cmd tea.Cmd
	switch m.NewProductStep {
	case newProductStepName:
		m.NewProductNameInput, cmd = m.NewProductNameInput.Update(msg)
	case newProductStepPrice:
		m.NewProductPriceInput, cmd = m.NewProductPriceInput.Update(msg)
	case newProductStepCategory:
		switch msg.String() {
		case "up", "k":
			if m.NewProductCatCursor > 0 {
				m.NewProductCatCursor--
			}
		case "down", "j":
			if m.NewProductCatCursor < len(m.NewProductCategories)-1 {
				m.NewProductCatCursor++
			}
		}
	}
	return m, cmd
}

// advanceNewProduct valida el paso actual y avanza al siguiente (o ejecuta la creación).
func (m Model) advanceNewProduct() (tea.Model, tea.Cmd) {
	switch m.NewProductStep {
	case newProductStepName:
		if m.NewProductNameInput.Value() == "" {
			m.NewProductErr = "El nombre no puede estar vacío."
			return m, nil
		}
		m.NewProductErr = ""
		m.NewProductStep = newProductStepPrice
		m.NewProductNameInput.Blur()
		m.NewProductPriceInput.Focus()
		return m, nil

	case newProductStepPrice:
		val := m.NewProductPriceInput.Value()
		if _, err := strconv.ParseFloat(val, 64); err != nil || val == "" {
			m.NewProductErr = "Ingresá un precio válido (ej: 9.99)."
			return m, nil
		}
		m.NewProductErr = ""
		m.NewProductStep = newProductStepCategory
		m.NewProductPriceInput.Blur()
		// Categorías se cargan async; la view muestra "Cargando..." hasta MsgCategoriesLoaded.
		return m, m.CmdFetchCategories()

	case newProductStepCategory:
		if len(m.NewProductCategories) == 0 {
			m.NewProductErr = "Esperando categorías. Intentá de nuevo."
			return m, nil
		}
		m.NewProductErr = ""
		m.NewProductStep = newProductStepConfirm
		return m, nil

	case newProductStepConfirm:
		price, _ := strconv.ParseFloat(m.NewProductPriceInput.Value(), 64)
		categoryID := ""
		if m.NewProductCatCursor < len(m.NewProductCategories) {
			categoryID = m.NewProductCategories[m.NewProductCatCursor].ID
		}
		return m, m.CmdCreateProduct(m.NewProductBarcode, m.NewProductNameInput.Value(), price, categoryID)
	}
	return m, nil
}

// cancelNewProduct descarta el formulario y vuelve a StateScanning.
func (m Model) cancelNewProduct() Model {
	m.State = StateScanning
	m.StatusMsg = fmt.Sprintf("Descartado: %s", m.NewProductBarcode)
	m.NewProductBarcode = ""
	m.NewProductStep = 0
	m.NewProductErr = ""
	m.NewProductNameInput.SetValue("")
	m.NewProductPriceInput.SetValue("")
	m.NewProductCategories = nil
	m.NewProductCatCursor = 0
	m.TextInput.Focus()
	return m
}
