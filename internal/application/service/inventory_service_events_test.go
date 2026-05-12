package service_test

import (
	"context"
	"database/sql"
	"testing"
)

// seedGroup inserta un grupo y lo asocia con un producto (por product_id).
func seedGroup(t *testing.T, db *sql.DB, groupName string, productID int) {
	t.Helper()
	var groupID int64
	row := db.QueryRowContext(context.Background(),
		`INSERT INTO custom_groups (group_name) VALUES (?) RETURNING id`, groupName)
	if err := row.Scan(&groupID); err != nil {
		t.Fatalf("seed group %q: %v", groupName, err)
	}
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO custom_group_products (group_id, product_id) VALUES (?, ?)`, groupID, productID)
	if err != nil {
		t.Fatalf("seed group_product: %v", err)
	}
}

func productIDForBarcode(t *testing.T, db *sql.DB, barcode string) int {
	t.Helper()
	var id int
	if err := db.QueryRowContext(context.Background(),
		`SELECT id FROM products WHERE barcode = ?`, barcode).Scan(&id); err != nil {
		t.Fatalf("productIDForBarcode(%q): %v", barcode, err)
	}
	return id
}

func TestScanLoyverseSale_productInGroup_recordsEvent(t *testing.T) {
	svc, db := setupTestEnv(t)
	seedProduct(t, db, "7896789", "Yerba 500g")
	pid := productIDForBarcode(t, db, "7896789")
	seedGroup(t, db, "grupo-ventas", pid)
	sid := createSession(t, svc, "test")
	ctx := context.Background()

	if err := svc.ScanLoyverseSale(ctx, sid, "Yerba 500g", -3); err != nil {
		t.Fatalf("ScanLoyverseSale: %v", err)
	}

	events, err := svc.GetLoyverseEvents(ctx, sid)
	if err != nil {
		t.Fatalf("GetLoyverseEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Quantity != -3 {
		t.Errorf("event quantity = %d, want -3", events[0].Quantity)
	}
	if events[0].Source != "LOYVERSE_SALE" {
		t.Errorf("event source = %q, want LOYVERSE_SALE", events[0].Source)
	}
}

func TestScanLoyverseSale_productNotInGroup_noEvent(t *testing.T) {
	// Producto sin grupo → evento silenciosamente ignorado.
	svc, db := setupTestEnv(t)
	seedProduct(t, db, "7896789", "Yerba 500g")
	sid := createSession(t, svc, "test")
	ctx := context.Background()

	if err := svc.ScanLoyverseSale(ctx, sid, "Yerba 500g", -2); err != nil {
		t.Fatalf("ScanLoyverseSale: %v", err)
	}

	events, _ := svc.GetLoyverseEvents(ctx, sid)
	if len(events) != 0 {
		t.Errorf("expected no events for ungrouped product, got %d", len(events))
	}
}

func TestScanLoyverseSale_refundAddsToSessionTotal(t *testing.T) {
	// Una devolución (delta > 0) debe sumar al total en GetSessionTotals.
	svc, db := setupTestEnv(t)
	seedProduct(t, db, "7896789", "Yerba 500g")
	pid := productIDForBarcode(t, db, "7896789")
	seedGroup(t, db, "grupo-ventas", pid)
	sid := createSession(t, svc, "test")
	ctx := context.Background()

	for range 5 {
		svc.ScanProduct(ctx, sid, "7896789") // +5 en inventory_scans
	}
	svc.ScanLoyverseSale(ctx, sid, "Yerba 500g", -3) // -3 venta → total 2
	svc.ScanLoyverseSale(ctx, sid, "Yerba 500g", 1)  // +1 refund → total 3

	totals, err := svc.GetSessionTotals(ctx, sid)
	if err != nil {
		t.Fatalf("GetSessionTotals: %v", err)
	}
	if len(totals) != 1 {
		t.Fatalf("expected 1 product total, got %d", len(totals))
	}
	if totals[0].Quantity != 3 {
		t.Errorf("GetSessionTotals: got %d, want 3 (5 scanned - 3 sale + 1 refund)", totals[0].Quantity)
	}
}
