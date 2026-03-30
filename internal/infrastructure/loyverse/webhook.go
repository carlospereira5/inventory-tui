package loyverse

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"inventory-tui/internal/domain/repository"
	"io"
	"net/http"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
)

// LoyverseWebhook maneja las peticiones HTTP de Loyverse.
type LoyverseWebhook struct {
	repo    repository.InventoryRepository
	program atomic.Pointer[tea.Program] // Fix 3: acceso atómico para evitar data race entre goroutines.
	secret  string
}

// MsgLoyverseSale es el mensaje que se envía a la TUI cuando ocurre una venta.
type MsgLoyverseSale struct {
	Barcode string
	Delta   int
	Name    string
}

func NewLoyverseWebhook(repo repository.InventoryRepository, secret string) *LoyverseWebhook {
	return &LoyverseWebhook{
		repo:   repo,
		secret: secret,
	}
}

func (h *LoyverseWebhook) SetProgram(p *tea.Program) {
	h.program.Store(p)
}

func (h *LoyverseWebhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Fix 2: defer antes de ReadAll para garantizar el cierre incluso si ReadAll falla.
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if h.secret != "" {
		signature := r.Header.Get("X-Loyverse-Signature")
		if !h.verifySignature(body, signature) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	var payload struct {
		Receipts []struct {
			ReceiptType string `json:"receipt_type"`
			Status      string `json:"status"` // Fix 1: mapeamos Status para filtrar recibos CANCELLED.
			LineItems   []struct {
				SKU      string  `json:"sku"`
				Quantity float64 `json:"quantity"`
				ItemName string  `json:"item_name"`
			} `json:"line_items"`
		} `json:"receipts"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusOK) // Respondemos 200 para evitar reintentos de Loyverse en errores de parsing.
		return
	}

	prog := h.program.Load()

	for _, receipt := range payload.Receipts {
		// Fix 1: ignorar recibos que no estén completados (CANCELLED, OPEN, etc.).
		// Solo los recibos con Status "DONE" representan transacciones reales.
		if receipt.Status != "DONE" {
			continue
		}
		if receipt.ReceiptType != "SALE" && receipt.ReceiptType != "REFUND" {
			continue
		}

		deltaMultiplier := -1
		if receipt.ReceiptType == "REFUND" {
			deltaMultiplier = 1
		}

		for _, item := range receipt.LineItems {
			// Nota: el campo `sku` del webhook de Loyverse debe coincidir con el campo
			// `barcode` importado desde el CSV. Si son distintos, la sincronización
			// fallará silenciosamente — verificar que el CSV use el mismo identificador.
			if item.SKU == "" {
				continue
			}

			if prog != nil {
				prog.Send(MsgLoyverseSale{
					Barcode: item.SKU,
					Delta:   int(item.Quantity) * deltaMultiplier,
					Name:    item.ItemName,
				})
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
