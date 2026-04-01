package loyverse

import "fmt"

// MsgLoyverseSale es el mensaje enviado a la TUI cuando Loyverse reporta una venta.
// La resolución barcode ← name ocurre en InventoryService.ScanLoyverseSale.
type MsgLoyverseSale struct {
	Name  string
	Delta int
}

// WebhookPayload representa la estructura JSON que envía Loyverse en el webhook.
type WebhookPayload struct {
	Receipts []Receipt `json:"receipts"`
}

// Receipt representa un ticket individual (venta o devolución) de Loyverse.
type Receipt struct {
	ReceiptType string     `json:"receipt_type"`
	CancelledAt string     `json:"cancelled_at"`
	ReceiptDate string     `json:"receipt_date"`
	LineItems   []LineItem `json:"line_items"`
}

// LineItem representa un producto dentro de un ticket de Loyverse.
type LineItem struct {
	ItemName string  `json:"item_name"`
	Quantity float64 `json:"quantity"`
}

// LoyverseItem representa un producto del catálogo de Loyverse.
// NOTA: La API de Loyverse NO incluye "stores" en la respuesta de GET /items.
// El store_id se obtiene desde GET /inventory.
type LoyverseItem struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Variants []LoyverseVariant `json:"variants"`
}

// LoyverseVariant representa una variante de un item en Loyverse.
// NOTA: El campo "id" en la respuesta de GET /items se llama "variant_id" en los variants.
type LoyverseVariant struct {
	ID      string `json:"variant_id"`
	Name    string `json:"name"`
	Sku     string `json:"sku"`
	Barcode string `json:"barcode"`
}

// InventoryLevel representa un nivel de stock a actualizar en Loyverse.
type InventoryLevel struct {
	VariantID  string  `json:"variant_id"`
	StoreID    string  `json:"store_id"`
	StockAfter float64 `json:"stock_after"`
}

// InventoryRecord representa un registro de inventario obtenido de Loyverse.
type InventoryRecord struct {
	VariantID string  `json:"variant_id"`
	StoreID   string  `json:"store_id"`
	Stock     float64 `json:"in_stock"`
}

// VariantInfo contiene información de mapeo para una variante.
type VariantInfo struct {
	VariantID string
	StoreID   string
}

// VariantMap es un mapa en memoria para lookup rápido de variantes.
type VariantMap struct {
	ByBarcode map[string]VariantInfo
	ByName    map[string]VariantInfo
}

// SyncResult contiene el resultado de una sincronización.
type SyncResult struct {
	Total   int
	Success int
	Failed  int
	Errors  []SyncError
}

// SyncError representa un error durante la sincronización.
type SyncError struct {
	ProductName string
	Barcode     string
	Error       string
}

// LoyverseAPIError representa un error de la API de Loyverse.
type LoyverseAPIError struct {
	StatusCode int
	Message    string
}

func (e *LoyverseAPIError) Error() string {
	return fmt.Sprintf("Loyverse API error %d: %s", e.StatusCode, e.Message)
}
