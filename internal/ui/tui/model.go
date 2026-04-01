package tui

import (
	"inventory-tui/internal/application/service"
	"inventory-tui/internal/domain/entity"

	"github.com/charmbracelet/bubbles/textinput"
)

// State define los diferentes estados o pantallas de la aplicación.
type State int

const (
	StateSessionList   State = iota // Lista de sesiones existentes.
	StateSessionCreate              // Formulario para crear una nueva sesión.
	StateScanning                   // Pantalla de escaneo de productos.
	StateHistory                    // Historial de escaneos en la sesión actual.
	StateLoyverse                   // Pantalla de totales y eventos de Loyverse.
	StateSyncLoyverse               // Pantalla de sincronización con Loyverse.
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
	Width                int
	Height               int
	TextInput            textinput.Model
	SessionInput         textinput.Model
	SyncModel            SyncModel
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

	return Model{
		Service:       svc,
		State:         StateSessionList,
		TextInput:     ti,
		SessionInput:  si,
		CatalogStatus: "Cargando catálogo...",
		GroupsStatus:  "Cargando grupos...",
		SyncModel:     NewSyncModel(svc),
	}
}
