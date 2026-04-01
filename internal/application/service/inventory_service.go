package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	"inventory-tui/internal/domain/entity"
	"inventory-tui/internal/domain/repository"
	"inventory-tui/internal/infrastructure/loyverse"
	"inventory-tui/internal/infrastructure/storage"
)

// InventoryService coordina los procesos de negocio relacionados con el inventario.
type InventoryService struct {
	db                *sql.DB
	products          repository.ProductRepository
	sessions          repository.SessionRepository
	inventory         repository.InventoryRepository
	loyverseEvents    repository.LoyverseEventRepository
	groups            repository.CustomGroupRepository
	csvStorage        *storage.CSVStorage
	onSessionActivate func(sessionID int)
}

// NewInventoryService crea una instancia del servicio de inventario.
func NewInventoryService(
	db *sql.DB,
	p repository.ProductRepository,
	s repository.SessionRepository,
	i repository.InventoryRepository,
	l repository.LoyverseEventRepository,
	g repository.CustomGroupRepository,
	csv *storage.CSVStorage,
) *InventoryService {
	return &InventoryService{db: db, products: p, sessions: s, inventory: i, loyverseEvents: l, groups: g, csvStorage: csv}
}

// LoadCatalog busca e importa el archivo CSV en la base de datos de productos.
func (s *InventoryService) LoadCatalog(ctx context.Context) (int, string, error) {
	file, err := s.csvStorage.FindCSVInRoot()
	if err != nil {
		return 0, "", err
	}
	count, err := s.csvStorage.ImportProducts(ctx, s.db, file)
	return count, file, err
}

// LoadGroups busca e importa el archivo CSV de grupos personalizados.
func (s *InventoryService) LoadGroups(ctx context.Context) (int, string, error) {
	file, err := s.csvStorage.FindGroupsCSV()
	if err != nil {
		return 0, "", err
	}
	count, err := s.csvStorage.ImportGroups(ctx, file)
	return count, file, err
}

// GetSessions devuelve todas las sesiones de inventario existentes.
func (s *InventoryService) GetSessions(ctx context.Context) ([]entity.Session, error) {
	return s.sessions.GetAll(ctx)
}

// CreateSession crea una nueva sesión y la devuelve con su ID generado.
func (s *InventoryService) CreateSession(ctx context.Context, name string) (int, error) {
	return s.sessions.Create(ctx, name)
}

// ScanProduct busca el producto, añade un nuevo escaneo y devuelve el total actualizado del registro.
func (s *InventoryService) ScanProduct(ctx context.Context, sessionID int, barcode string) (*entity.Product, *entity.Record, error) {
	p, err := s.products.FindByBarcode(ctx, barcode)
	if err != nil || p == nil {
		return p, nil, err
	}
	// Añadimos el escaneo individual con un delta de 1.
	if err := s.inventory.AddScan(ctx, sessionID, barcode, 1, "SCAN"); err != nil {
		return p, nil, err
	}
	// GetRecord devolverá el total acumulado del producto para la sesión.
	rec, err := s.inventory.GetRecord(ctx, sessionID, barcode)
	return p, rec, err
}

// GetHistory obtiene el historial completo de TODOS los escaneos realizados en la sesión.
func (s *InventoryService) GetHistory(ctx context.Context, sessionID int) ([]entity.Record, error) {
	return s.inventory.GetSessionHistory(ctx, sessionID)
}

// ExportSession descarga los datos de la sesión a un archivo CSV.
func (s *InventoryService) ExportSession(ctx context.Context, id int, name string) (string, error) {
	records, err := s.inventory.GetSessionHistory(ctx, id)
	if err != nil {
		return "", err
	}
	return s.csvStorage.ExportSession(name, records)
}

// DeleteSession elimina la sesión por completo.
func (s *InventoryService) DeleteSession(ctx context.Context, id int) error {
	return s.sessions.Delete(ctx, id)
}

// ScanLoyverseSale descuenta del stock de la sesión cuando Loyverse reporta una venta.
// name debe coincidir exactamente con el nombre del producto en el catálogo CSV.
// Solo actúa si el producto pertenece a un grupo personalizado o si fue escaneado manualmente.
func (s *InventoryService) ScanLoyverseSale(ctx context.Context, sessionID int, name string, delta int) error {
	// Bug 3 fix: el webhook expone item_name, no barcode. Resolvemos el producto por nombre.
	p, err := s.products.FindByName(ctx, name)
	if err != nil {
		return err
	}
	if p == nil {
		return nil // producto no encontrado en catálogo — mismatch de nombres, ignorar silenciosamente
	}

	// Verificar si el producto pertenece a algún grupo personalizado
	groups, err := s.groups.GetGroupsForProduct(ctx, p.ID)
	if err != nil {
		return fmt.Errorf("error al verificar grupos del producto: %w", err)
	}

	// Si el producto no pertenece a ningún grupo, no aplicar descuento
	if len(groups) == 0 {
		return nil
	}

	source := "LOYVERSE_SALE"
	if delta > 0 {
		source = "LOYVERSE_REFUND"
	}
	return s.loyverseEvents.AddEvent(ctx, sessionID, p.Barcode, delta, source)
}

// DeleteScan elimina un evento de escaneo individual por su ID.
func (s *InventoryService) DeleteScan(ctx context.Context, id int) error {
	return s.inventory.DeleteScan(ctx, id)
}

// GetSessionTotals devuelve el total acumulado por producto en una sesión.
func (s *InventoryService) GetSessionTotals(ctx context.Context, sessionID int) ([]entity.SessionTotals, error) {
	return s.inventory.GetSessionTotals(ctx, sessionID)
}

// GetLoyverseEvents devuelve solo los eventos de Loyverse (ventas y refunds) de una sesión.
func (s *InventoryService) GetLoyverseEvents(ctx context.Context, sessionID int) ([]entity.LoyverseEvent, error) {
	return s.loyverseEvents.GetEvents(ctx, sessionID)
}

// DeleteLoyverseEvent elimina un evento de Loyverse (venta o refund) por su ID.
func (s *InventoryService) DeleteLoyverseEvent(ctx context.Context, eventID int) error {
	return s.loyverseEvents.DeleteEvent(ctx, eventID)
}

// SetOnSessionActivate registra un callback que se llama cuando se activa una sesión.
func (s *InventoryService) SetOnSessionActivate(fn func(sessionID int)) {
	s.onSessionActivate = fn
}

// ActivateSession notifica que una sesión se activó (para el webhook).
func (s *InventoryService) ActivateSession(sessionID int) {
	if s.onSessionActivate != nil {
		s.onSessionActivate(sessionID)
	}
}

// SyncWithLoyverse sincroniza el inventario local con la API de Loyverse.
// Obtiene el catálogo, mapea productos locales y envía el stock calculado.
// El StoreID se obtiene desde GET /inventory; para variants sin stock previo,
// se usa la primera tienda de GET /stores como fallback.
func (s *InventoryService) SyncWithLoyverse(ctx context.Context) (*loyverse.SyncResult, error) {
	token := os.Getenv("LOYVERSE_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("LOYVERSE_TOKEN env var not set")
	}

	client, err := loyverse.NewClient(token)
	if err != nil {
		return nil, fmt.Errorf("creating Loyverse client: %w", err)
	}

	// 1. Fetch catálogo
	slog.Info("sync: fetching catalog")
	items, err := client.GetAllItems()
	if err != nil {
		return nil, fmt.Errorf("fetching catalog: %w", err)
	}
	slog.Info("sync: catalog fetched", "items", len(items))

	// Debug: log detallado del mapeo de variantes
	variantMap := loyverse.BuildVariantMap(items)
	slog.Info("sync: variant map built",
		"by_barcode_count", len(variantMap.ByBarcode),
		"by_name_count", len(variantMap.ByName),
	)

	// 2. Fetch inventario actual para obtener variant_id → store_id
	slog.Info("sync: fetching inventory levels")
	inventoryRecords, err := client.GetInventory()
	if err != nil {
		return nil, fmt.Errorf("fetching inventory: %w", err)
	}
	storeMap := loyverse.BuildStoreMap(inventoryRecords)
	slog.Info("sync: inventory records fetched", "count", len(inventoryRecords), "store_map_entries", len(storeMap))

	// 3. Fetch stores para fallback (variants sin stock previo no aparecen en GET /inventory)
	slog.Info("sync: fetching stores")
	stores, err := client.GetStores()
	if err != nil {
		return nil, fmt.Errorf("fetching stores: %w", err)
	}
	var defaultStoreID string
	if len(stores) > 0 {
		defaultStoreID = stores[0].ID
		slog.Info("sync: default store", "id", defaultStoreID, "name", stores[0].Name)
	}

	// 4. Calcular stock local
	stockSummary, err := s.inventory.GetStockSummary(ctx)
	if err != nil {
		return nil, fmt.Errorf("calculating local stock: %w", err)
	}
	slog.Info("sync: local stock calculated", "products", len(stockSummary))

	// 5. Mapear productos locales → InventoryLevels
	var levels []loyverse.InventoryLevel
	var unmapped []string
	var syncErrors []loyverse.SyncError

	for barcode, quantity := range stockSummary {
		info, found := variantMap.ByBarcode[barcode]
		if !found {
			slog.Warn("sync: barcode no mapeado", "barcode", barcode)
			unmapped = append(unmapped, barcode)
			continue
		}

		slog.Debug("sync: barcode mapeado",
			"barcode", barcode,
			"variant_id", info.VariantID,
			"quantity", quantity,
		)

		// Intentar obtener store_id desde inventario
		storeID, ok := storeMap[info.VariantID]
		if !ok {
			// Fallback: usar primera tienda disponible
			if defaultStoreID == "" {
				syncErrors = append(syncErrors, loyverse.SyncError{
					ProductName: barcode,
					Barcode:     barcode,
					Error:       fmt.Sprintf("no store_id for variant %s (not in inventory, no stores available)", info.VariantID),
				})
				continue
			}
			slog.Warn("sync: variant not in inventory, using default store",
				"barcode", barcode,
				"variant_id", info.VariantID,
				"default_store_id", defaultStoreID,
			)
			storeID = defaultStoreID
		}

		slog.Debug("sync: level preparado",
			"barcode", barcode,
			"variant_id", info.VariantID,
			"store_id", storeID,
			"stock_after", quantity,
		)

		levels = append(levels, loyverse.InventoryLevel{
			VariantID:  info.VariantID,
			StoreID:    storeID,
			StockAfter: quantity,
		})
	}

	slog.Info("sync: levels prepared", "total", len(levels), "unmapped", len(unmapped), "errors", len(syncErrors))

	// 6. Ejecutar batch update
	slog.Info("sync: executing batch update", "levels_count", len(levels))
	success, failed, batchErrors := client.UpdateStockBatch(levels)

	result := &loyverse.SyncResult{
		Total:   len(stockSummary),
		Success: success,
		Failed:  failed + len(syncErrors),
		Errors:  append(syncErrors, batchErrors...),
	}

	if len(unmapped) > 0 {
		slog.Warn("sync: unmapped products", "count", len(unmapped), "barcodes", unmapped)
	}

	slog.Info("sync: completed",
		"total", result.Total,
		"success", result.Success,
		"failed", result.Failed,
		"batch_errors", len(batchErrors),
	)
	return result, nil
}
