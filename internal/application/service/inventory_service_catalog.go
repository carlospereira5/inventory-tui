package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/carlospereira5/inventory-tui/internal/domain/entity"
	"github.com/carlospereira5/inventory-tui/internal/infrastructure/loyverse"
)

// LoadCatalogFromLoyverse importa todos los items de Loyverse al catálogo local.
// Usa una única transacción SQL para máximo rendimiento (mismo patrón que ImportProducts).
// Uso en tests: pasar un cliente apuntando a httptest.Server.
func (s *InventoryService) LoadCatalogFromLoyverse(ctx context.Context, client *loyverse.Client) (int, error) {
	items, err := client.GetAllItems()
	if err != nil {
		return 0, fmt.Errorf("obteniendo items de Loyverse: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("iniciando transacción: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO products (barcode, name, category_id) VALUES (?, ?, ?)
		ON CONFLICT(barcode) DO UPDATE SET name = excluded.name, category_id = excluded.category_id;
	`)
	if err != nil {
		return 0, fmt.Errorf("preparando statement: %w", err)
	}
	defer stmt.Close()

	count := 0
	for _, item := range items {
		for _, variant := range item.Variants {
			if variant.Barcode == "" {
				continue // variantes sin barcode no son escaneables
			}
			if _, err := stmt.ExecContext(ctx, variant.Barcode, item.Name, item.CategoryID); err != nil {
				return 0, fmt.Errorf("insertando %s (%s): %w", variant.Barcode, item.Name, err)
			}
			count++
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("confirmando transacción: %w", err)
	}

	slog.Info("catálogo importado desde Loyverse", "items", len(items), "variantes", count)
	return count, nil
}

// GetActiveCategories devuelve el mapa category_id → active de la DB.
func (s *InventoryService) GetActiveCategories(ctx context.Context) (map[string]bool, error) {
	return s.activeCategories.GetAll(ctx)
}

// SetCategoryActive activa o desactiva una categoría para el filtrado del webhook.
func (s *InventoryService) SetCategoryActive(ctx context.Context, categoryID string, active bool) error {
	return s.activeCategories.SetActive(ctx, categoryID, active)
}

// GetCategoriesWithClient obtiene categorías de Loyverse usando el cliente inyectado.
// Uso en tests: pasar un cliente apuntando a httptest.Server.
func (s *InventoryService) GetCategoriesWithClient(client *loyverse.Client) ([]loyverse.Category, error) {
	return client.GetCategories()
}

// GetCategories obtiene categorías de Loyverse creando el cliente desde LOYVERSE_TOKEN.
func (s *InventoryService) GetCategories() ([]loyverse.Category, error) {
	token := os.Getenv("LOYVERSE_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("LOYVERSE_TOKEN env var not set")
	}
	client, err := loyverse.NewClient(token)
	if err != nil {
		return nil, err
	}
	return s.GetCategoriesWithClient(client)
}

// CreateProductWithClient crea un producto en Loyverse y lo agrega al catálogo local.
// Retorna la entidad Product con el barcode y nombre del producto creado.
// Uso en tests: pasar un cliente apuntando a httptest.Server.
func (s *InventoryService) CreateProductWithClient(ctx context.Context, client *loyverse.Client, barcode, name string, price float64, categoryID string) (*entity.Product, error) {
	req := loyverse.CreateItemRequest{
		ItemName:   name,
		CategoryID: categoryID,
		TrackStock: true,
		Variants: []loyverse.CreateVariantRequest{{
			DefaultPricingType: "FIXED",
			Price:              price,
			Barcode:            barcode,
		}},
	}

	item, err := client.CreateItem(req)
	if err != nil {
		return nil, fmt.Errorf("creando item en Loyverse: %w", err)
	}

	// Usar el barcode tal como lo devuelve Loyverse (puede diferir si lo normalizó).
	finalBarcode := barcode
	if len(item.Variants) > 0 && item.Variants[0].Barcode != "" {
		finalBarcode = item.Variants[0].Barcode
	}

	p := &entity.Product{Barcode: finalBarcode, Name: name}
	if err := s.products.Upsert(ctx, p); err != nil {
		return nil, fmt.Errorf("agregando producto al catálogo local: %w", err)
	}

	return p, nil
}

// CreateProduct crea un producto creando el cliente desde LOYVERSE_TOKEN.
func (s *InventoryService) CreateProduct(ctx context.Context, barcode, name string, price float64, categoryID string) (*entity.Product, error) {
	token := os.Getenv("LOYVERSE_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("LOYVERSE_TOKEN env var not set")
	}
	client, err := loyverse.NewClient(token)
	if err != nil {
		return nil, err
	}
	return s.CreateProductWithClient(ctx, client, barcode, name, price, categoryID)
}
