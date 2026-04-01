package loyverse

import (
	"fmt"
	"log/slog"
	"net/url"
	"sync"
	"time"
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

const maxWorkers = 5
const batchSize = 50

// UpdateStockBatch actualiza el stock de múltiples variantes usando workers concurrentes.
// Agrupa los niveles en batches de 50 items para minimizar requests HTTP.
// Retorna el número de éxitos, fallos, y los errores detallados.
func (c *Client) UpdateStockBatch(levels []InventoryLevel) (success, failed int, errors []SyncError) {
	if len(levels) == 0 {
		return 0, 0, nil
	}

	// Dividir en batches de 50 items
	var batches [][]InventoryLevel
	for i := 0; i < len(levels); i += batchSize {
		end := i + batchSize
		if end > len(levels) {
			end = len(levels)
		}
		batches = append(batches, levels[i:end])
	}

	jobs := make(chan []InventoryLevel, len(batches))
	results := make(chan batchResult, len(batches))

	// Lanzar 5 workers
	var wg sync.WaitGroup
	for w := 0; w < maxWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			batchWorker(c, jobs, results)
		}()
	}

	// Enviar batches como jobs
	for _, batch := range batches {
		jobs <- batch
	}
	close(jobs)

	// Esperar a que todos los workers terminen
	wg.Wait()
	close(results)

	// Contar resultados
	for _, r := range collectResults(results, batches) {
		if r.Err != "" {
			failed += r.Failed
			errors = append(errors, r.Errors...)
		} else {
			success += r.Success
		}
	}

	return success, failed, errors
}

type batchResult struct {
	Success int
	Failed  int
	Errors  []SyncError
	Err     string
}

// batchWorker procesa batches de actualizaciones de stock con retry logic.
func batchWorker(c *Client, jobs <-chan []InventoryLevel, results chan<- batchResult) {
	for batch := range jobs {
		result := updateBatchWithRetry(c, batch, 3)
		results <- result
	}
}

// updateBatchWithRetry intenta actualizar un batch con reintentos y backoff.
func updateBatchWithRetry(c *Client, batch []InventoryLevel, maxRetries int) batchResult {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Backoff exponencial: 1s, 2s, 4s
			time.Sleep(time.Duration(1<<uint(attempt-1)) * time.Second)
		}

		payload := map[string]interface{}{
			"inventory_levels": batch,
		}

		err := c.postJSON("/inventory", payload, nil)
		if err == nil {
			return batchResult{Success: len(batch)}
		}

		lastErr = err

		var apiErr *LoyverseAPIError
		if asAPIErr(err, &apiErr) && apiErr.StatusCode == 429 {
			time.Sleep(2 * time.Second)
			continue
		}

		// Si es 4xx (no 429), no reintentar — reportar cada item del batch como fallido
		if asAPIErr(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
			var errs []SyncError
			for _, level := range batch {
				errs = append(errs, SyncError{
					ProductName: level.VariantID,
					Error:       fmt.Sprintf("variant %s: %s", level.VariantID, apiErr.Message),
				})
			}
			return batchResult{Failed: len(batch), Errors: errs, Err: apiErr.Message}
		}
	}

	// Todos los reintentos fallaron
	var errs []SyncError
	for _, level := range batch {
		errs = append(errs, SyncError{
			ProductName: level.VariantID,
			Error:       fmt.Sprintf("variant %s: %v after %d retries", level.VariantID, lastErr, maxRetries),
		})
	}
	return batchResult{Failed: len(batch), Errors: errs, Err: lastErr.Error()}
}

func collectResults(results <-chan batchResult, batches [][]InventoryLevel) []batchResult {
	var all []batchResult
	for r := range results {
		all = append(all, r)
	}
	return all
}

// asAPIErr verifica si un error es de tipo LoyverseAPIError.
func asAPIErr(err error, target **LoyverseAPIError) bool {
	if e, ok := err.(*LoyverseAPIError); ok {
		*target = e
		return true
	}
	return false
}
