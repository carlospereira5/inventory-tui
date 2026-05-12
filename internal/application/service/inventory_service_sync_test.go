package service_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"inventory-tui/internal/infrastructure/loyverse"
)

// capturedLevel es el nivel de stock capturado desde el POST /inventory del mock.
type capturedLevel struct {
	VariantID  string  `json:"variant_id"`
	StoreID    string  `json:"store_id"`
	StockAfter float64 `json:"stock_after"`
}

// mockLoyverse crea un httptest.Server que simula los endpoints de la API de Loyverse.
// items, inventory y stores definen las respuestas de los GETs.
// El POST /inventory captura el body y NO envía nada a la API real.
func mockLoyverse(t *testing.T, items []loyverse.LoyverseItem, inv []loyverse.InventoryRecord, stores []loyverse.Store) (*httptest.Server, func() []capturedLevel) {
	t.Helper()

	var mu sync.Mutex
	var captured []capturedLevel

	mux := http.NewServeMux()

	mux.HandleFunc("GET /items", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"items": items})
	})
	mux.HandleFunc("GET /inventory", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"inventory_levels": inv})
	})
	mux.HandleFunc("GET /stores", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"stores": stores})
	})
	mux.HandleFunc("POST /inventory", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload struct {
			Levels []capturedLevel `json:"inventory_levels"`
		}
		json.Unmarshal(body, &payload)
		mu.Lock()
		captured = append(captured, payload.Levels...)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv, func() []capturedLevel {
		mu.Lock()
		defer mu.Unlock()
		out := make([]capturedLevel, len(captured))
		copy(out, captured)
		return out
	}
}

// buildTestCatalog devuelve items/inventory/stores consistentes para tests.
// barcode "7896789" → variant_id "var-001" → store_id "store-001", in_stock=10.
func buildTestCatalog(existingStock float64) ([]loyverse.LoyverseItem, []loyverse.InventoryRecord, []loyverse.Store) {
	items := []loyverse.LoyverseItem{{
		ID:   "item-001",
		Name: "Yerba 500g",
		Variants: []loyverse.LoyverseVariant{{
			ID:      "var-001",
			Barcode: "7896789",
		}},
	}}
	inv := []loyverse.InventoryRecord{{
		VariantID: "var-001",
		StoreID:   "store-001",
		Stock:     existingStock,
	}}
	stores := []loyverse.Store{{ID: "store-001", Name: "Tienda Principal"}}
	return items, inv, stores
}

func TestSync_replaceModesSendsExactScannedCount(t *testing.T) {
	svc, db := setupTestEnv(t)
	seedProduct(t, db, "7896789", "Yerba 500g")
	sid := createSession(t, svc, "test")

	ctx := context.Background()
	for range 5 {
		svc.ScanProduct(ctx, sid, "7896789")
	}

	items, inv, stores := buildTestCatalog(10) // Loyverse ya tiene 10, pero replace ignora eso
	srv, getLevels := mockLoyverse(t, items, inv, stores)

	client, err := loyverse.NewClientWithBaseURL("test-token", srv.URL)
	if err != nil {
		t.Fatalf("client: %v", err)
	}

	result, err := svc.SyncWithClient(ctx, client, []int{sid}, loyverse.SyncModeReplace)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if result.Failed > 0 {
		t.Fatalf("sync errors: %v", result.Errors)
	}

	levels := getLevels()
	if len(levels) != 1 {
		t.Fatalf("expected 1 level sent, got %d", len(levels))
	}
	if levels[0].StockAfter != 5 {
		t.Errorf("replace mode: got stock_after=%.0f, want 5", levels[0].StockAfter)
	}
}

func TestSync_addModeSumsPreviousWithScannedCount(t *testing.T) {
	svc, db := setupTestEnv(t)
	seedProduct(t, db, "7896789", "Yerba 500g")
	sid := createSession(t, svc, "test")

	ctx := context.Background()
	for range 5 {
		svc.ScanProduct(ctx, sid, "7896789")
	}

	items, inv, stores := buildTestCatalog(10) // Loyverse tiene 10 → esperamos 10+5=15
	srv, getLevels := mockLoyverse(t, items, inv, stores)

	client, _ := loyverse.NewClientWithBaseURL("test-token", srv.URL)
	result, err := svc.SyncWithClient(ctx, client, []int{sid}, loyverse.SyncModeAdd)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if result.Failed > 0 {
		t.Fatalf("sync errors: %v", result.Errors)
	}

	levels := getLevels()
	if len(levels) != 1 {
		t.Fatalf("expected 1 level sent, got %d", len(levels))
	}
	if levels[0].StockAfter != 15 {
		t.Errorf("add mode: got stock_after=%.0f, want 15", levels[0].StockAfter)
	}
}

// TestSync_emptySessionIDs_includesAllSessions documenta el comportamiento de footgun:
// pasar []int{} como sessionIDs resulta en un sync de TODAS las sesiones históricas
// porque GetStockSummary omite la cláusula WHERE cuando el slice está vacío.
// La TUI guarda esto en la UI, pero la capa de servicio no tiene defensa propia.
func TestSync_emptySessionIDs_includesAllSessions(t *testing.T) {
	svc, db := setupTestEnv(t)
	seedProduct(t, db, "7896789", "Yerba 500g")

	ctx := context.Background()
	sid1 := createSession(t, svc, "sesion-1")
	sid2 := createSession(t, svc, "sesion-2")

	for range 3 {
		svc.ScanProduct(ctx, sid1, "7896789")
	}
	for range 4 {
		svc.ScanProduct(ctx, sid2, "7896789")
	}

	items, inv, stores := buildTestCatalog(0)
	srv, getLevels := mockLoyverse(t, items, inv, stores)

	client, _ := loyverse.NewClientWithBaseURL("test-token", srv.URL)
	// sessionIDs vacío → incluye TODAS las sesiones (3 + 4 = 7)
	result, err := svc.SyncWithClient(ctx, client, []int{}, loyverse.SyncModeReplace)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if result.Failed > 0 {
		t.Fatalf("sync errors: %v", result.Errors)
	}

	levels := getLevels()
	if len(levels) != 1 {
		t.Fatalf("expected 1 level, got %d", len(levels))
	}
	// Comportamiento documentado: stock_after = suma de TODAS las sesiones
	if levels[0].StockAfter != 7 {
		t.Errorf("empty sessionIDs: got stock_after=%.0f, want 7 (all sessions summed)", levels[0].StockAfter)
	}
}

func TestSync_onlyIncludesSelectedSessions(t *testing.T) {
	svc, db := setupTestEnv(t)
	seedProduct(t, db, "7896789", "Yerba 500g")

	ctx := context.Background()
	sid1 := createSession(t, svc, "sesion-1")
	sid2 := createSession(t, svc, "sesion-2")

	for range 5 {
		svc.ScanProduct(ctx, sid1, "7896789")
	}
	for range 32 { // 37 total si no hay filtro — ese es el bug a detectar
		svc.ScanProduct(ctx, sid2, "7896789")
	}

	items, inv, stores := buildTestCatalog(0)
	srv, getLevels := mockLoyverse(t, items, inv, stores)

	client, _ := loyverse.NewClientWithBaseURL("test-token", srv.URL)
	// Sync SOLO con sid1 — no debe incluir los 32 de sid2
	result, err := svc.SyncWithClient(ctx, client, []int{sid1}, loyverse.SyncModeReplace)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if result.Failed > 0 {
		t.Fatalf("sync errors: %v", result.Errors)
	}

	levels := getLevels()
	if len(levels) != 1 {
		t.Fatalf("expected 1 level, got %d", len(levels))
	}
	if levels[0].StockAfter != 5 {
		t.Errorf("session filter: got stock_after=%.0f, want 5 (not 37)", levels[0].StockAfter)
	}
}
