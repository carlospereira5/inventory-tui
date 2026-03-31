package loyverse

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// processPayload handles the business logic of processing a Loyverse webhook payload.
func (h *LoyverseWebhook) processPayload(payload WebhookPayload) {
	prog := h.program.Load()

	// Determinar sesión destino: TUI tiene prioridad sobre Web UI.
	tuiSession := int(h.tuiSessionID.Load())
	webSession := int(h.webSessionID.Load())
	targetSession := tuiSession
	if targetSession == 0 {
		targetSession = webSession
	}
	if targetSession == 0 {
		h.logger.log("descartando payload — no hay sesión activa (ni TUI ni web)")
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
		if !h.isValidReceipt(receipt, sessionStart) {
			continue
		}

		h.processReceipt(receipt, targetSession, scanFn, prog, onSale)
	}
}

// isValidReceipt verifica si un receipt debe procesarse según estado, tipo y fecha.
func (h *LoyverseWebhook) isValidReceipt(receipt Receipt, sessionStart *time.Time) bool {
	if receipt.CancelledAt != "" {
		h.logger.log("ignorando receipt cancelado (cancelled_at=%s)", receipt.CancelledAt)
		return false
	}
	if receipt.ReceiptType != "SALE" && receipt.ReceiptType != "REFUND" {
		h.logger.log("ignorando tipo de receipt: %q", receipt.ReceiptType)
		return false
	}

	if sessionStart != nil {
		receiptTime, err := parseReceiptDate(receipt.ReceiptDate)
		if err != nil {
			h.logger.log("no se pudo parsear receipt_date=%q — descartando: %v", receipt.ReceiptDate, err)
			return false
		}
		if receiptTime.Before(*sessionStart) {
			h.logger.log("ignorando receipt anterior a la sesión: receipt_date=%s, inicio=%s",
				receipt.ReceiptDate, sessionStart.UTC().Format(time.RFC3339))
			return false
		}
	}

	return true
}

// processReceipt itera sobre los line items de un receipt válido y ejecuta las acciones.
func (h *LoyverseWebhook) processReceipt(receipt Receipt, targetSession int, scanFn func(ctx context.Context, sessionID int, name string, delta int) error, prog *tea.Program, onSale func()) {
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

		// Escritura directa a DB.
		if scanFn != nil {
			if err := scanFn(context.Background(), targetSession, item.ItemName, delta); err != nil {
				h.logger.log("error escribiendo venta: %v", err)
			}
		}

		// Notificar a la TUI para refrescar pantalla.
		if prog != nil {
			prog.Send(MsgLoyverseSale{Name: item.ItemName, Delta: delta})
		}

		// Notificar a la Web UI vía SSE.
		if onSale != nil {
			onSale()
		}
	}
}

// parseReceiptDate intenta parsear la fecha en formato RFC3339 o RFC3339Nano.
func parseReceiptDate(dateStr string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, dateStr)
	}
	return t, err
}
