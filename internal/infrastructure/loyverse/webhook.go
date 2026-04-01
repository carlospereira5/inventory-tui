package loyverse

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"inventory-tui/internal/domain/repository"

	tea "github.com/charmbracelet/bubbletea"
)

// LoyverseWebhook recibe y procesa webhooks de ventas de Loyverse.
type LoyverseWebhook struct {
	repo             repository.InventoryRepository
	program          atomic.Pointer[tea.Program]
	secret           string
	sessionStartedAt atomic.Int64 // Unix nanoseconds del inicio de sesión TUI; 0 = sin sesión TUI.
	logger           fileLogger

	// Sesiones activas: la TUI tiene prioridad sobre la web UI.
	tuiSessionID atomic.Int64 // ID de la sesión activa en la TUI; 0 = ninguna.
	webSessionID atomic.Int64 // ID de la sesión activa en la web UI; 0 = ninguna.

	// Callback para escribir directamente a DB (inyectado desde main).
	// Evita que el webhook dependa del ciclo de la TUI para persistir ventas.
	scanSaleMu sync.RWMutex
	scanSaleFn func(ctx context.Context, sessionID int, name string, delta int) error

	onSaleMu sync.RWMutex
	onSaleFn func() // notifica a la web UI vía SSE
}

// NewLoyverseWebhook crea una instancia del webhook.
func NewLoyverseWebhook(repo repository.InventoryRepository, secret string) *LoyverseWebhook {
	return &LoyverseWebhook{
		repo:   repo,
		secret: secret,
		logger: fileLogger{path: "loyverse.log"},
	}
}

// SetProgram asocia el programa Bubble Tea para poder enviarle mensajes desde el webhook.
func (h *LoyverseWebhook) SetProgram(p *tea.Program) {
	h.program.Store(p)
}

// SetTUIActiveSession registra la sesión activa en la TUI y marca el inicio
// de la sesión para el filtro temporal (descarta receipts anteriores a este momento).
// Llamar al activar una sesión. La TUI tiene prioridad sobre la web UI.
func (h *LoyverseWebhook) SetTUIActiveSession(id int) {
	h.tuiSessionID.Store(int64(id))
	h.SetSessionStartedAt(time.Now())
}

// SetActiveWebSession registra la sesión activa en la web UI.
// Llamar cuando el usuario abre una sesión en el browser; pasar 0 al salir.
func (h *LoyverseWebhook) SetActiveWebSession(id int) {
	h.webSessionID.Store(int64(id))
}

// SetScanSale inyecta el callback que escribe una venta a la DB.
// Desacopla el webhook del servicio de aplicación sin crear dependencias circulares.
func (h *LoyverseWebhook) SetScanSale(fn func(ctx context.Context, sessionID int, name string, delta int) error) {
	h.scanSaleMu.Lock()
	h.scanSaleFn = fn
	h.scanSaleMu.Unlock()
}

// SetOnSale registra un callback que se llama después de procesar cada venta.
// Usado para notificar a la web UI vía SSE sin acoplar los dos paquetes.
func (h *LoyverseWebhook) SetOnSale(fn func()) {
	h.onSaleMu.Lock()
	h.onSaleFn = fn
	h.onSaleMu.Unlock()
}

// SetSessionStartedAt marca el inicio de una sesión de inventario.
// Solo los receipts con receipt_date >= t serán procesados; los anteriores se descartan.
// Llamar al activar una sesión en la TUI.
func (h *LoyverseWebhook) SetSessionStartedAt(t time.Time) {
	h.sessionStartedAt.Store(t.UnixNano())
	h.logger.log("sesión iniciada — se ignorarán receipts anteriores a %s", t.UTC().Format(time.RFC3339))
}
