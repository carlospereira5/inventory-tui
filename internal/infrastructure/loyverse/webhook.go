package loyverse

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"inventory-tui/internal/domain/repository"

	tea "github.com/charmbracelet/bubbletea"
)

// fileLogger escribe a loyverse.log para no interferir con la TUI (que secuestra stdout).
// Usar con: tail -f loyverse.log mientras corre la aplicación.
type fileLogger struct{ path string }

func (l fileLogger) log(format string, args ...any) {
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	prefix := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(f, "[%s] "+format+"\n", append([]any{prefix}, args...)...)
}

// MsgLoyverseSale es el mensaje enviado a la TUI cuando Loyverse reporta una venta.
// Bug 3 fix: eliminado el campo Barcode — el webhook no expone barcode en line_items.
// La resolución barcode ← name ocurre en InventoryService.ScanLoyverseSale.
type MsgLoyverseSale struct {
	Name  string
	Delta int
}

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

// SetTUIActiveSession registra la sesión activa en la TUI.
// Llamar al activar una sesión. La TUI tiene prioridad sobre la web UI.
func (h *LoyverseWebhook) SetTUIActiveSession(id int) {
	h.tuiSessionID.Store(int64(id))
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

// ServeHTTP implementa http.Handler para el endpoint del webhook.
func (h *LoyverseWebhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	h.logger.log("payload recibido (%d bytes): %s", len(body), string(body))

	if h.secret != "" {
		signature := r.Header.Get("X-Loyverse-Signature")
		if !h.verifySignature(body, signature) {
			h.logger.log("firma inválida — petición rechazada")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	var payload struct {
		Receipts []struct {
			ReceiptType string `json:"receipt_type"`
			// Bug 1 fix: el schema real de Loyverse no tiene campo "status".
			// cancelled_at vacío = completado; con fecha ISO8601 = cancelado.
			CancelledAt string `json:"cancelled_at"`
			// ReceiptDate es la fecha real de la venta (UTC). Usada para filtrar por inicio de sesión.
			ReceiptDate string `json:"receipt_date"`
			LineItems   []struct {
				// Bug 2 fix: barcode no existe en line_items del webhook.
				// sku existe pero no coincide con el barcode EAN del catálogo CSV.
				// Usamos item_name que sí coincide con el nombre importado desde CSV.
				ItemName string  `json:"item_name"`
				Quantity float64 `json:"quantity"`
			} `json:"line_items"`
		} `json:"receipts"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		h.logger.log("error parseando JSON: %v", err)
		// 200 para evitar que Loyverse reintente indefinidamente por errores de formato.
		w.WriteHeader(http.StatusOK)
		return
	}

	prog := h.program.Load()

	// Determinar sesión destino: TUI tiene prioridad sobre web UI.
	tuiSession := int(h.tuiSessionID.Load())
	webSession := int(h.webSessionID.Load())
	targetSession := tuiSession
	if targetSession == 0 {
		targetSession = webSession
	}
	if targetSession == 0 {
		h.logger.log("descartando payload — no hay sesión activa (ni TUI ni web)")
		w.WriteHeader(http.StatusOK)
		return
	}
	h.logger.log("sesión destino: %d (tui=%d, web=%d)", targetSession, tuiSession, webSession)

	// Cargar callbacks una vez fuera del loop.
	h.scanSaleMu.RLock()
	scanFn := h.scanSaleFn
	h.scanSaleMu.RUnlock()

	h.onSaleMu.RLock()
	onSale := h.onSaleFn
	h.onSaleMu.RUnlock()

	// Filtro temporal: solo aplica cuando la TUI activó una sesión explícitamente.
	// Descarta ventas que ocurrieron antes de que comenzara el conteo.
	startedAtNano := h.sessionStartedAt.Load()
	var sessionStart *time.Time
	if startedAtNano != 0 {
		t := time.Unix(0, startedAtNano)
		sessionStart = &t
		h.logger.log("filtro temporal activo desde %s", t.UTC().Format(time.RFC3339))
	}

	for _, receipt := range payload.Receipts {
		if receipt.CancelledAt != "" {
			h.logger.log("ignorando receipt cancelado (cancelled_at=%s)", receipt.CancelledAt)
			continue
		}
		if receipt.ReceiptType != "SALE" && receipt.ReceiptType != "REFUND" {
			h.logger.log("ignorando tipo de receipt: %q", receipt.ReceiptType)
			continue
		}

		if sessionStart != nil {
			receiptTime, parseErr := time.Parse(time.RFC3339, receipt.ReceiptDate)
			if parseErr != nil {
				receiptTime, parseErr = time.Parse(time.RFC3339Nano, receipt.ReceiptDate)
			}
			if parseErr != nil {
				h.logger.log("no se pudo parsear receipt_date=%q — descartando: %v", receipt.ReceiptDate, parseErr)
				continue
			}
			if receiptTime.Before(*sessionStart) {
				h.logger.log("ignorando receipt anterior a la sesión: receipt_date=%s, inicio=%s",
					receipt.ReceiptDate, sessionStart.UTC().Format(time.RFC3339))
				continue
			}
		}

		deltaMultiplier := -1
		if receipt.ReceiptType == "REFUND" {
			deltaMultiplier = 1
		}

		for _, item := range receipt.LineItems {
			if item.ItemName == "" {
				continue
			}
			delta := int(item.Quantity) * deltaMultiplier
			h.logger.log("escribiendo venta: item=%q, delta=%d, session=%d", item.ItemName, delta, targetSession)

			// Escritura directa a DB — ya no depende del ciclo de la TUI.
			if scanFn != nil {
				if err := scanFn(context.Background(), targetSession, item.ItemName, delta); err != nil {
					h.logger.log("error escribiendo venta: %v", err)
				}
			}

			// Notificar a la TUI solo para refrescar la pantalla (no para escribir).
			if prog != nil {
				prog.Send(MsgLoyverseSale{Name: item.ItemName, Delta: delta})
			}

			// Notificar a la web UI vía SSE.
			if onSale != nil {
				onSale()
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (h *LoyverseWebhook) verifySignature(body []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(body)
	expectedMAC := mac.Sum(nil)
	expectedSignature := base64.StdEncoding.EncodeToString(expectedMAC)
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}
