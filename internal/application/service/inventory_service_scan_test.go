package service_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"inventory-tui/internal/application/service"
	"inventory-tui/internal/infrastructure/database"
	"inventory-tui/internal/infrastructure/storage"
)

// setupTestEnv wires up a fully in-memory service + DB and seeds one product.
// Returns the service, the raw DB (for direct seeding), and the session ID.
//
// Uses a temp file instead of :memory: because GetGroupsForProduct opens nested
// queries (outer rows + inner productRows simultaneously), requiring a second
// connection. With :memory: each connection is an independent empty database.
func setupTestEnv(t *testing.T) (*service.InventoryService, *sql.DB) {
	t.Helper()

	sdb, err := database.NewSQLiteDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db init: %v", err)
	}
	t.Cleanup(func() { sdb.Conn.Close() })

	db := sdb.Conn
	products := database.NewSQLiteProductRepository(db)
	sessions := database.NewSQLiteSessionRepository(db)
	inv := database.NewSQLiteInventoryRepository(db)
	events := database.NewSQLiteLoyverseEventRepository(db)
	groups := database.NewSQLiteCustomGroupRepository(db)
	activeCats := database.NewSQLiteActiveCategoryRepository(db)
	csv := storage.NewCSVStorage(products, groups)

	svc := service.NewInventoryService(db, products, sessions, inv, events, groups, activeCats, csv)
	return svc, db
}

func seedProduct(t *testing.T, db *sql.DB, barcode, name string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO products (barcode, name) VALUES (?, ?)`, barcode, name)
	if err != nil {
		t.Fatalf("seed product %q: %v", barcode, err)
	}
}

func createSession(t *testing.T, svc *service.InventoryService, name string) int {
	t.Helper()
	id, err := svc.CreateSession(context.Background(), name)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return id
}

func TestScanProduct_recordsOneUnit(t *testing.T) {
	svc, db := setupTestEnv(t)
	seedProduct(t, db, "7896789", "Yerba 500g")
	sid := createSession(t, svc, "test")

	_, rec, err := svc.ScanProduct(context.Background(), sid, "7896789")
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if rec.Quantity != 1 {
		t.Errorf("got quantity %d, want 1", rec.Quantity)
	}
}

func TestScanProduct_repeatedScansAccumulate(t *testing.T) {
	svc, db := setupTestEnv(t)
	seedProduct(t, db, "7896789", "Yerba 500g")
	sid := createSession(t, svc, "test")

	ctx := context.Background()
	for range 5 {
		if _, _, err := svc.ScanProduct(ctx, sid, "7896789"); err != nil {
			t.Fatalf("scan: %v", err)
		}
	}

	totals, err := svc.GetSessionTotals(ctx, sid)
	if err != nil {
		t.Fatalf("totals: %v", err)
	}
	if len(totals) != 1 {
		t.Fatalf("expected 1 product, got %d", len(totals))
	}
	if totals[0].Quantity != 5 {
		t.Errorf("got quantity %d, want 5", totals[0].Quantity)
	}
}

func TestAddQuickScan_addsToExistingCount(t *testing.T) {
	svc, db := setupTestEnv(t)
	seedProduct(t, db, "7896789", "Yerba 500g")
	sid := createSession(t, svc, "test")

	ctx := context.Background()
	svc.ScanProduct(ctx, sid, "7896789") // +1

	rec, err := svc.AddQuickScan(ctx, sid, "7896789", 4) // +4
	if err != nil {
		t.Fatalf("quick scan: %v", err)
	}
	if rec.Quantity != 5 {
		t.Errorf("got quantity %d, want 5", rec.Quantity)
	}
}

func TestScanProduct_sessionIsolation(t *testing.T) {
	svc, db := setupTestEnv(t)
	seedProduct(t, db, "7896789", "Yerba 500g")

	ctx := context.Background()
	sid1 := createSession(t, svc, "session-1")
	sid2 := createSession(t, svc, "session-2")

	for range 3 {
		svc.ScanProduct(ctx, sid1, "7896789")
	}
	for range 7 {
		svc.ScanProduct(ctx, sid2, "7896789")
	}

	totals1, _ := svc.GetSessionTotals(ctx, sid1)
	totals2, _ := svc.GetSessionTotals(ctx, sid2)

	if totals1[0].Quantity != 3 {
		t.Errorf("session 1: got %d, want 3", totals1[0].Quantity)
	}
	if totals2[0].Quantity != 7 {
		t.Errorf("session 2: got %d, want 7", totals2[0].Quantity)
	}
}
