package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/carlospereira5/inventory-tui/internal/domain/entity"
	"github.com/carlospereira5/inventory-tui/internal/domain/repository"
	"github.com/carlospereira5/inventory-tui/internal/infrastructure/loyverse"
	"github.com/carlospereira5/inventory-tui/internal/infrastructure/storage"
)

// InventoryService coordina los procesos de negocio relacionados con el inventario.
type InventoryService struct {
	db                *sql.DB
	products          repository.ProductRepository
	sessions          repository.SessionRepository
	inventory         repository.InventoryRepository
	loyverseEvents    repository.LoyverseEventRepository
	groups            repository.CustomGroupRepository
	activeCategories  repository.ActiveCategoryRepository
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
	ac repository.ActiveCategoryRepository,
	csv *storage.CSVStorage,
) *InventoryService {
	return &InventoryService{db: db, products: p, sessions: s, inventory: i, loyverseEvents: l, groups: g, activeCategories: ac, csvStorage: csv}
}

// LoadCatalog importa el catálogo de productos.
// Estrategia: si LOYVERSE_TOKEN está disponible, carga directo desde la API de Loyverse.
// Si no hay token (o falla la conexión), cae a CSV como modo legacy.
func (s *InventoryService) LoadCatalog(ctx context.Context) (int, string, error) {
	token := os.Getenv("LOYVERSE_TOKEN")
	if token != "" {
		client, err := loyverse.NewClient(token)
		if err == nil {
			count, err := s.LoadCatalogFromLoyverse(ctx, client)
			if err != nil {
				slog.Warn("catálogo Loyverse fallido, usando CSV", "err", err)
			} else {
				return count, "Loyverse API", nil
			}
		}
	}

	// Fallback: modo legacy CSV
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

// RenameSession cambia el nombre de una sesión existente.
func (s *InventoryService) RenameSession(ctx context.Context, id int, name string) error {
	return s.sessions.UpdateName(ctx, id, name)
}

// ScanLoyverseSale descuenta del stock de la sesión cuando Loyverse reporta una venta.
// Filtrado por categorías activas (primario) o grupos personalizados (legacy/CSV).
//
// Lógica de filtrado:
//  1. Si el producto tiene category_id → usa active_categories (Loyverse nativo).
//     Solo registra el evento si la categoría está activa.
//  2. Si el producto no tiene category_id (vino de CSV) → legacy: verifica custom_groups.
func (s *InventoryService) ScanLoyverseSale(ctx context.Context, sessionID int, name string, delta int) error {
	p, err := s.products.FindByName(ctx, name)
	if err != nil {
		return err
	}
	if p == nil {
		return nil // producto no encontrado en catálogo — ignorar silenciosamente
	}

	if p.CategoryID != "" {
		// Camino primario: filtrado por categorías Loyverse.
		active, err := s.activeCategories.IsActive(ctx, p.CategoryID)
		if err != nil {
			return fmt.Errorf("verificando categoría activa: %w", err)
		}
		if !active {
			slog.Debug("webhook: categoría no activa, ignorando venta", "name", name, "category_id", p.CategoryID)
			return nil
		}
	} else {
		// Camino legacy: filtrado por grupos personalizados (CSV).
		groups, err := s.groups.GetGroupsForProduct(ctx, p.ID)
		if err != nil {
			return fmt.Errorf("error al verificar grupos del producto: %w", err)
		}
		if len(groups) == 0 {
			return nil
		}
	}

	source := "LOYVERSE_SALE"
	if delta > 0 {
		source = "LOYVERSE_REFUND"
	}
	return s.loyverseEvents.AddEvent(ctx, sessionID, p.Barcode, delta, source)
}

// AddQuickScan agrega una cantidad arbitraria de un producto ya identificado.
// A diferencia de ScanProduct, no busca por barcode en el catálogo — el producto
// ya fue resuelto previamente. Devuelve el record actualizado con el total acumulado.
func (s *InventoryService) AddQuickScan(ctx context.Context, sessionID int, barcode string, quantity int) (*entity.Record, error) {
	if err := s.inventory.AddScan(ctx, sessionID, barcode, quantity, "SCAN"); err != nil {
		return nil, err
	}
	return s.inventory.GetRecord(ctx, sessionID, barcode)
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
// Crea el cliente desde LOYVERSE_TOKEN y delega a SyncWithClient.
func (s *InventoryService) SyncWithLoyverse(ctx context.Context, sessionIDs []int, mode loyverse.SyncMode) (*loyverse.SyncResult, error) {
	token := os.Getenv("LOYVERSE_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("LOYVERSE_TOKEN env var not set")
	}
	client, err := loyverse.NewClient(token)
	if err != nil {
		return nil, fmt.Errorf("creating Loyverse client: %w", err)
	}
	return s.SyncWithClient(ctx, client, sessionIDs, mode)
}

// SyncWithClient ejecuta el sync completo con un cliente Loyverse ya construido.
// Expuesto para tests: permite apuntar a un httptest.Server en lugar de la API real.
//
// Optimización: las fases de fetch independientes (catálogo, inventario, stores,
// stock local) se ejecutan en paralelo con errgroup para reducir el tiempo total.
func (s *InventoryService) SyncWithClient(ctx context.Context, client *loyverse.Client, sessionIDs []int, mode loyverse.SyncMode) (*loyverse.SyncResult, error) {

	// Fase paralela: fetches independientes con errgroup
	var items []loyverse.LoyverseItem
	var inventoryRecords []loyverse.InventoryRecord
	var stores []loyverse.Store
	var stockSummary map[string]float64

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		start := time.Now()
		var err error
		items, err = client.GetAllItems()
		if err != nil {
			return fmt.Errorf("fetching catalog: %w", err)
		}
		slog.Info("sync: catalog fetched", "items", len(items), "duration", time.Since(start).Round(time.Millisecond))
		return nil
	})

	g.Go(func() error {
		start := time.Now()
		var err error
		inventoryRecords, err = client.GetInventory()
		if err != nil {
			return fmt.Errorf("fetching inventory: %w", err)
		}
		slog.Info("sync: inventory fetched", "count", len(inventoryRecords), "duration", time.Since(start).Round(time.Millisecond))
		return nil
	})

	g.Go(func() error {
		start := time.Now()
		var err error
		stores, err = client.GetStores()
		if err != nil {
			return fmt.Errorf("fetching stores: %w", err)
		}
		slog.Info("sync: stores fetched", "count", len(stores), "duration", time.Since(start).Round(time.Millisecond))
		return nil
	})

	g.Go(func() error {
		start := time.Now()
		var err error
		stockSummary, err = s.inventory.GetStockSummary(gCtx, sessionIDs)
		if err != nil {
			return fmt.Errorf("calculating local stock: %w", err)
		}
		slog.Info("sync: local stock calculated", "products", len(stockSummary), "duration", time.Since(start).Round(time.Millisecond))
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("sync: %w", err)
	}

	// Fase secuencial: mapeo y construcción de niveles
	// NOTA: usar ctx original (no gCtx), porque errgroup cancela gCtx al completar Wait().
	start := time.Now()
	variantMap := loyverse.BuildVariantMap(items)
	storeMap := loyverse.BuildStoreMap(inventoryRecords)

	// En modo suma, construimos el mapa de stock actual para sumar a los conteos locales.
	var currentStockMap map[string]float64
	if mode == loyverse.SyncModeAdd {
		currentStockMap = loyverse.BuildCurrentStockMap(inventoryRecords)
		slog.Info("sync: modo suma activado", "productos_con_stock_previo", len(currentStockMap))
	}

	var defaultStoreID string
	if len(stores) > 0 {
		defaultStoreID = stores[0].ID
	}

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

		storeID, ok := storeMap[info.VariantID]
		if !ok {
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

		stockAfter := quantity
		if mode == loyverse.SyncModeAdd {
			key := info.VariantID + "|" + storeID
			stockAfter = currentStockMap[key] + quantity
		}

		levels = append(levels, loyverse.InventoryLevel{
			VariantID:  info.VariantID,
			StoreID:    storeID,
			StockAfter: stockAfter,
		})
	}
	slog.Info("sync: mapping completed", "mode", mode, "levels", len(levels), "unmapped", len(unmapped), "duration", time.Since(start).Round(time.Millisecond))

	// Fase final: batch update (ya tiene workers internos)
	start = time.Now()
	slog.Info("sync: executing batch update", "levels_count", len(levels))
	success, failed, batchErrors := client.UpdateStockBatch(ctx, levels)
	slog.Info("sync: batch update completed", "duration", time.Since(start).Round(time.Millisecond))

	result := &loyverse.SyncResult{
		Total:   len(stockSummary),
		Success: success,
		Failed:  failed + len(syncErrors),
		Mode:    mode,
		Errors:  append(syncErrors, batchErrors...),
	}

	if len(unmapped) > 0 {
		slog.Warn("sync: unmapped products", "count", len(unmapped), "barcodes", unmapped)
	}

	slog.Info("sync: completed",
		"total", result.Total,
		"success", result.Success,
		"failed", result.Failed,
	)
	return result, nil
}
