package loyverse_test

// Tests de concurrencia y correctitud para UpdateStockBatch.
//
// Race condition status: NO hay race conditions en el código actual.
// Razones: workers solo leen job.levels (nunca escriben), results es un canal
// bufferado de exactamente len(batches), el conteo final corre single-threaded
// después de wg.Wait(), y http.Client es goroutine-safe por spec de Go.
//
// Ejecutar con detector de races: go test -race ./internal/infrastructure/loyverse/...

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/carlospereira5/inventory-tui/internal/infrastructure/loyverse"
)

// batchServer es un servidor de test configurable para POST /inventory.
type batchServer struct {
	statusCode int32 // HTTP status a devolver (0 = 200)
	calls      int32 // atomic — total de requests recibidos
	items      int32 // atomic — total de items recibidos sumando todos los batches
}

func (s *batchServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt32(&s.calls, 1)

	code := int(atomic.LoadInt32(&s.statusCode))
	if code >= 400 {
		w.WriteHeader(code)
		fmt.Fprintf(w, `{"message":"test error %d"}`, code)
		return
	}

	body, _ := io.ReadAll(r.Body)
	var payload struct {
		Levels []json.RawMessage `json:"inventory_levels"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		atomic.AddInt32(&s.items, int32(len(payload.Levels)))
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("{}"))
}

func makeLevels(n int) []loyverse.InventoryLevel {
	levels := make([]loyverse.InventoryLevel, n)
	for i := range levels {
		levels[i] = loyverse.InventoryLevel{
			VariantID:  fmt.Sprintf("var-%04d", i),
			StoreID:    "store-001",
			StockAfter: float64(i + 1),
		}
	}
	return levels
}

func newBatchClient(t *testing.T, srv *batchServer) (*loyverse.Client, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	c, err := loyverse.NewClientWithBaseURL("test-token", ts.URL)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	return c, ts
}

func TestUpdateStockBatch_emptyInput_noHTTPCalls(t *testing.T) {
	srv := &batchServer{}
	c, _ := newBatchClient(t, srv)

	success, failed, errs := c.UpdateStockBatch(context.Background(), nil)

	if success != 0 || failed != 0 || len(errs) != 0 {
		t.Errorf("UpdateStockBatch(nil) = (%d, %d, %v), want (0, 0, nil)", success, failed, errs)
	}
	if atomic.LoadInt32(&srv.calls) != 0 {
		t.Errorf("expected 0 HTTP calls for empty input, got %d", srv.calls)
	}
}

func TestUpdateStockBatch_singleBatch_allSucceed(t *testing.T) {
	srv := &batchServer{}
	c, _ := newBatchClient(t, srv)
	levels := makeLevels(10)

	success, failed, _ := c.UpdateStockBatch(context.Background(), levels)

	if success != 10 {
		t.Errorf("success = %d, want 10", success)
	}
	if failed != 0 {
		t.Errorf("failed = %d, want 0", failed)
	}
	if atomic.LoadInt32(&srv.calls) != 1 {
		t.Errorf("expected 1 HTTP call for 10 items, got %d", srv.calls)
	}
}

// TestUpdateStockBatch_multiBatch_noItemsLost es el test crítico de concurrencia.
// 300 items fuerzan 2 batches procesados por workers concurrentes.
// Verifica que success+failed == total (ningún item se pierde ni se cuenta doble).
func TestUpdateStockBatch_multiBatch_noItemsLost(t *testing.T) {
	srv := &batchServer{}
	c, _ := newBatchClient(t, srv)
	const total = 300
	levels := makeLevels(total)

	success, failed, _ := c.UpdateStockBatch(context.Background(), levels)

	if success+failed != total {
		t.Errorf("success(%d) + failed(%d) = %d, want %d — items perdidos o duplicados",
			success, failed, success+failed, total)
	}
	if success != total {
		t.Errorf("success = %d, want %d", success, total)
	}
	if atomic.LoadInt32(&srv.calls) != 2 {
		t.Errorf("expected 2 HTTP calls for %d items (batchSize=250), got %d", total, srv.calls)
	}
}

// TestUpdateStockBatch_4xx_failsImmediately verifica que un error 4xx (no 429)
// no reintenta — el batch falla de inmediato y todos sus items se cuentan como fallidos.
func TestUpdateStockBatch_4xx_failsImmediately(t *testing.T) {
	srv := &batchServer{statusCode: 400}
	c, _ := newBatchClient(t, srv)
	const n = 5
	levels := makeLevels(n)

	success, failed, errs := c.UpdateStockBatch(context.Background(), levels)

	if failed != n {
		t.Errorf("failed = %d, want %d", failed, n)
	}
	if success != 0 {
		t.Errorf("success = %d, want 0", success)
	}
	if len(errs) != n {
		t.Errorf("len(errors) = %d, want %d (uno por item del batch)", len(errs), n)
	}
	// 4xx no debe reintentar: exactamente 1 llamada HTTP
	if atomic.LoadInt32(&srv.calls) != 1 {
		t.Errorf("expected 1 HTTP call (no retry on 4xx), got %d", srv.calls)
	}
}

// TestUpdateStockBatch_allFailed_countIsConsistent verifica que cuando todos
// los batches fallan, success+failed sigue siendo igual al input total.
func TestUpdateStockBatch_allFailed_countIsConsistent(t *testing.T) {
	srv := &batchServer{statusCode: 400}
	c, _ := newBatchClient(t, srv)
	const total = 300
	levels := makeLevels(total)

	success, failed, _ := c.UpdateStockBatch(context.Background(), levels)

	if success+failed != total {
		t.Errorf("success(%d) + failed(%d) = %d, want %d",
			success, failed, success+failed, total)
	}
	if success != 0 {
		t.Errorf("success = %d, want 0", success)
	}
}

// TestUpdateStockBatch_contextCancelledBeforeStart verifica que si el context
// ya está cancelado antes de procesar jobs, todos los items se reportan como fallidos.
func TestUpdateStockBatch_contextCancelledBeforeStart(t *testing.T) {
	srv := &batchServer{}
	c, _ := newBatchClient(t, srv)
	const n = 10
	levels := makeLevels(n)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelar antes de llamar

	success, failed, errs := c.UpdateStockBatch(ctx, levels)

	if success != 0 {
		t.Errorf("success = %d, want 0 (context already cancelled)", success)
	}
	if failed != n {
		t.Errorf("failed = %d, want %d", failed, n)
	}
	if len(errs) == 0 {
		t.Error("expected at least one error entry for cancelled context")
	}
}
