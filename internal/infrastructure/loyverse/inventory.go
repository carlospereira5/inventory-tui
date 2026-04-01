package loyverse

import (
	"fmt"
	"log/slog"
	"net/url"
)

// InventoryResponse es la respuesta paginada de GET /inventory.
type InventoryResponse struct {
	Inventory []InventoryRecord `json:"inventory_levels"`
	Cursor    *string           `json:"cursor"`
}

// Store representa una tienda en Loyverse.
type Store struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// StoresResponse es la respuesta paginada de GET /stores.
type StoresResponse struct {
	Stores []Store `json:"stores"`
	Cursor *string `json:"cursor"`
}

// GetStores obtiene todas las tiendas de Loyverse con paginación.
func (c *Client) GetStores() ([]Store, error) {
	var allStores []Store
	cursor := ""

	for {
		path := "/stores?limit=250"
		if cursor != "" {
			path += "&cursor=" + url.QueryEscape(cursor)
		}

		var resp StoresResponse
		if err := c.getJSON(path, &resp); err != nil {
			return nil, fmt.Errorf("fetching stores: %w", err)
		}

		allStores = append(allStores, resp.Stores...)

		if resp.Cursor == nil || *resp.Cursor == "" {
			break
		}
		cursor = *resp.Cursor
	}

	slog.Info("stores fetched", "count", len(allStores))
	return allStores, nil
}

// GetInventory obtiene todos los registros de inventario con paginación.
func (c *Client) GetInventory() ([]InventoryRecord, error) {
	var allRecords []InventoryRecord
	cursor := ""

	for {
		path := "/inventory?limit=250"
		if cursor != "" {
			path += "&cursor=" + url.QueryEscape(cursor)
		}

		var resp InventoryResponse
		if err := c.getJSON(path, &resp); err != nil {
			return nil, fmt.Errorf("fetching inventory: %w", err)
		}

		allRecords = append(allRecords, resp.Inventory...)

		if resp.Cursor == nil || *resp.Cursor == "" {
			break
		}
		cursor = *resp.Cursor
	}

	slog.Info("inventory records fetched", "count", len(allRecords))
	return allRecords, nil
}

// BuildStoreMap construye un mapa variant_id → store_id desde los registros de inventario.
func BuildStoreMap(records []InventoryRecord) map[string]string {
	storeMap := make(map[string]string, len(records))
	for _, r := range records {
		storeMap[r.VariantID] = r.StoreID
	}
	return storeMap
}
