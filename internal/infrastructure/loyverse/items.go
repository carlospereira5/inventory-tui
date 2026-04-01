package loyverse

import (
	"fmt"
	"log/slog"
	"net/url"
)

// ItemsResponse es la respuesta paginada de GET /items.
type ItemsResponse struct {
	Items  []LoyverseItem `json:"items"`
	Cursor *string        `json:"cursor"`
}

// GetAllItems obtiene todos los items del catálogo con paginación.
func (c *Client) GetAllItems() ([]LoyverseItem, error) {
	var allItems []LoyverseItem
	cursor := ""

	for {
		path := "/items?limit=250"
		if cursor != "" {
			path += "&cursor=" + url.QueryEscape(cursor)
		}

		var resp ItemsResponse
		if err := c.getJSON(path, &resp); err != nil {
			return nil, fmt.Errorf("fetching items: %w", err)
		}

		allItems = append(allItems, resp.Items...)

		if resp.Cursor == nil || *resp.Cursor == "" {
			break
		}
		cursor = *resp.Cursor
	}

	return allItems, nil
}

// BuildVariantMap construye un mapa de lookup desde los items obtenidos.
// Crea entradas por barcode y por name para maximizar matches.
// El StoreID NO viene en GET /items — se resuelve después con BuildStoreMap.
func BuildVariantMap(items []LoyverseItem) *VariantMap {
	vm := &VariantMap{
		ByBarcode: make(map[string]VariantInfo, len(items)*2),
		ByName:    make(map[string]VariantInfo, len(items)*2),
	}

	var totalVariants, mappedByBarcode int
	for _, item := range items {
		for _, variant := range item.Variants {
			totalVariants++
			info := VariantInfo{
				VariantID: variant.ID,
				// StoreID se resuelve después con BuildStoreMap
			}

			// Mapeo por barcode
			if variant.Barcode != "" {
				vm.ByBarcode[variant.Barcode] = info
				mappedByBarcode++
			}

			// Mapeo por nombre del item
			if item.Name != "" {
				vm.ByName[item.Name] = info
			}

			// Mapeo por nombre de variante
			if variant.Name != "" {
				vm.ByName[variant.Name] = info
			}
		}
	}

	slog.Info("BuildVariantMap",
		"items", len(items),
		"total_variants", totalVariants,
		"mapped_by_barcode", mappedByBarcode,
		"map_size", len(vm.ByBarcode),
	)

	return vm
}
