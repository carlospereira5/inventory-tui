package loyverse

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
