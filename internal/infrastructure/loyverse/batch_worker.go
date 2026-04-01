package loyverse

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const maxWorkers = 10
const batchSize = 250

// UpdateStockBatch actualiza el stock de múltiples variantes usando workers concurrentes.
// Agrupa los niveles en batches de 250 items para minimizar requests HTTP.
// Acepta context para permitir cancelación desde la TUI.
// Retorna el número de éxitos, fallos, y los errores detallados.
func (c *Client) UpdateStockBatch(ctx context.Context, levels []InventoryLevel) (success, failed int, errors []SyncError) {
	if len(levels) == 0 {
		return 0, 0, nil
	}

	totalStart := time.Now()

	// Dividir en batches de 250 items
	var batches [][]InventoryLevel
	for i := 0; i < len(levels); i += batchSize {
		end := i + batchSize
		if end > len(levels) {
			end = len(levels)
		}
		batches = append(batches, levels[i:end])
	}

	totalBatches := len(batches)
	slog.Info("sync: batch update started", "items", len(levels), "batches", totalBatches)

	jobs := make(chan batchJob, len(batches))
	results := make(chan batchResult, len(batches))

	// Lanzar 10 workers
	var wg sync.WaitGroup
	for w := 0; w < maxWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			batchWorker(ctx, c, jobs, results)
		}()
	}

	// Enviar batches como jobs
	for i, batch := range batches {
		jobs <- batchJob{index: i + 1, levels: batch}
	}
	close(jobs)

	// Esperar a que todos los workers terminen
	wg.Wait()
	close(results)

	// Contar resultados
	for _, r := range collectResults(results) {
		if r.Err != "" {
			failed += r.Failed
			errors = append(errors, r.Errors...)
		} else {
			success += r.Success
		}
	}

	slog.Info("sync: batch update completed",
		"success", success,
		"failed", failed,
		"duration", time.Since(totalStart).Round(time.Millisecond))

	return success, failed, errors
}

type batchJob struct {
	index  int
	levels []InventoryLevel
}

type batchResult struct {
	Success int
	Failed  int
	Errors  []SyncError
	Err     string
}

// batchWorker procesa batches de actualizaciones de stock con retry logic.
func batchWorker(ctx context.Context, c *Client, jobs <-chan batchJob, results chan<- batchResult) {
	for job := range jobs {
		select {
		case <-ctx.Done():
			results <- batchResult{
				Failed: len(job.levels),
				Err:    "context cancelled",
				Errors: []SyncError{{
					ProductName: job.levels[0].VariantID,
					Error:       "context cancelled",
				}},
			}
			continue
		default:
		}

		batchStart := time.Now()
		result := updateBatchWithRetry(c, job.levels, 3)
		slog.Info("sync: batch update progress",
			"batch", job.index,
			"items", len(job.levels),
			"duration", time.Since(batchStart).Round(time.Millisecond))
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

func collectResults(results <-chan batchResult) []batchResult {
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
