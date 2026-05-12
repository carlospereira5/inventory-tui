package tui

import (
	"github.com/carlospereira5/inventory-tui/internal/application/service"
	"github.com/carlospereira5/inventory-tui/internal/domain/entity"
	"github.com/carlospereira5/inventory-tui/internal/infrastructure/loyverse"

	"github.com/charmbracelet/bubbles/textinput"
)

// State define los diferentes estados o pantallas de la aplicación.
type State int

const (
	StateSessionList      State = iota // Lista de sesiones existentes.
	StateSessionCreate                 // Formulario para crear una nueva sesión.
	StateSessionRename                 // Formulario para renombrar una sesión existente.
	StateScanning                      // Pantalla de escaneo de productos.
	StateQuickAdd                      // Overlay para agregar cantidad rápida al último producto escaneado.
	StateHistory                       // Historial de escaneos en la sesión actual.
	StateLoyverse                      // Pantalla de totales y eventos de Loyverse.
	StateSyncConfirm                   // Pantalla de confirmación de modo de sync (reemplazar vs sumar).
	StateSyncLoyverse                  // Pantalla de sincronización con Loyverse.
	StateNewProduct                    // Overlay para agregar producto desconocido a Loyverse.
	StateFilterCategories              // Overlay para activar/desactivar categorías de filtrado de webhook.
	StateHelp                          // Overlay de ayuda con todos los atajos de teclado.
)

// Model representa el estado global de la interfaz de usuario.
type Model struct {
	Service              *service.InventoryService
	State                State
	ActiveSession        *entity.Session
	LastScanned          *entity.Record
	ConsecutiveCount     int
	StatusMsg            string
	CatalogStatus        string
	CatalogIsError       bool
	GroupsStatus         string
	GroupsIsError        bool
	Err                  error
	Sessions             []entity.Session
	History              []entity.Record
	Totals               []entity.SessionTotals
	LoyverseEvents       []entity.LoyverseEvent
	Cursor               int
	HistoryScrollOffset  int
	LoyverseScrollOffset int
	TotalsCursor         int          // cursor independiente para el tab de Totales
	TotalsScrollOffset   int          // scroll independiente para el tab de Totales
	LoyverseSubTab       int          // 0 = Totales, 1 = Eventos
	SelectedSessions     map[int]bool // session.ID → seleccionada para sync
	PendingDelete        bool         // true cuando se espera confirmación de borrado (segundo 'd')
	HelpPrevState        State        // estado desde el que se abrió el help, para restaurarlo al cerrar
	Width                int
	Height               int
	TextInput            textinput.Model
	SessionInput         textinput.Model
	QuickAddInput        textinput.Model
	HistorySearch        textinput.Model
	HistorySearchActive  bool // true mientras el usuario escribe en la search bar del historial
	TotalsSearch         textinput.Model
	TotalsSearchActive   bool // true mientras el usuario escribe en la search bar de totales
	SyncModel            SyncModel

	// StateNewProduct: formulario multi-step para agregar producto desconocido a Loyverse.
	NewProductBarcode    string
	NewProductStep       int // 0=nombre, 1=precio, 2=categoría, 3=confirmar
	NewProductNameInput  textinput.Model
	NewProductPriceInput textinput.Model
	NewProductCategories []loyverse.Category
	NewProductCatCursor  int
	NewProductErr        string

	// StateFilterCategories: overlay para activar/desactivar categorías de filtrado de webhook.
	FilterCategories  []loyverse.Category // todas las categorías disponibles
	ActiveCategoryIDs map[string]bool     // category_id → activa (espejo de la DB)
	FilterCatCursor   int                 // cursor en la lista de categorías
}

// NewModel inicializa el modelo con sus valores por defecto y sub-componentes.
func NewModel(svc *service.InventoryService) Model {
	ti := textinput.New()
	ti.Placeholder = "Escanea el código de barras..."
	ti.Focus()
	ti.Width = 30

	si := textinput.New()
	si.Placeholder = "Nombre de la sesión (ej. Almacén 1)..."
	si.Width = 40

	qi := textinput.New()
	qi.Placeholder = "ej: 5, 10, 20..."
	qi.CharLimit = 6
	qi.Width = 20

	hs := textinput.New()
	hs.Placeholder = "Buscar producto..."
	hs.Width = 30

	ts := textinput.New()
	ts.Placeholder = "Buscar producto..."
	ts.Width = 30

	ni := textinput.New()
	ni.Placeholder = "Nombre del producto..."
	ni.Width = 40

	pi := textinput.New()
	pi.Placeholder = "ej: 9.99"
	pi.CharLimit = 10
	pi.Width = 20

	return Model{
		Service:              svc,
		State:                StateSessionList,
		TextInput:            ti,
		SessionInput:         si,
		QuickAddInput:        qi,
		HistorySearch:        hs,
		TotalsSearch:         ts,
		CatalogStatus:        "Cargando catálogo...",
		GroupsStatus:         "Cargando grupos...",
		SyncModel:            NewSyncModel(svc),
		SelectedSessions:     make(map[int]bool),
		NewProductNameInput:  ni,
		NewProductPriceInput: pi,
		ActiveCategoryIDs:    make(map[string]bool),
	}
}
