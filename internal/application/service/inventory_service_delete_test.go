package service_test

import (
	"context"
	"testing"
)

func TestDeleteScan_reducesTotalByOne(t *testing.T) {
	svc, db := setupTestEnv(t)
	seedProduct(t, db, "7896789", "Yerba 500g")
	sid := createSession(t, svc, "test")
	ctx := context.Background()

	for range 3 {
		svc.ScanProduct(ctx, sid, "7896789")
	}

	history, err := svc.GetHistory(ctx, sid)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(history))
	}

	if err := svc.DeleteScan(ctx, history[0].ID); err != nil {
		t.Fatalf("DeleteScan: %v", err)
	}

	totals, _ := svc.GetSessionTotals(ctx, sid)
	if totals[0].Quantity != 2 {
		t.Errorf("after delete: got quantity %d, want 2", totals[0].Quantity)
	}
}

func TestDeleteScan_quickAddDeltaFullyRemoved(t *testing.T) {
	// Un registro de QuickAdd tiene delta=N, no delta=1.
	// Borrar ese registro debe reducir el total en N, no en 1.
	svc, db := setupTestEnv(t)
	seedProduct(t, db, "7896789", "Yerba 500g")
	sid := createSession(t, svc, "test")
	ctx := context.Background()

	svc.ScanProduct(ctx, sid, "7896789") // delta=1, total=1
	svc.AddQuickScan(ctx, sid, "7896789", 4) // delta=4, total=5

	history, err := svc.GetHistory(ctx, sid)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	// history[0] es el más reciente (ORDER BY created_at DESC): el QuickAdd
	var quickAddID int
	for _, rec := range history {
		if rec.Quantity == 4 {
			quickAddID = rec.ID
			break
		}
	}
	if quickAddID == 0 {
		t.Fatal("no encontré el registro de QuickAdd (delta=4) en el historial")
	}

	if err := svc.DeleteScan(ctx, quickAddID); err != nil {
		t.Fatalf("DeleteScan: %v", err)
	}

	totals, _ := svc.GetSessionTotals(ctx, sid)
	if totals[0].Quantity != 1 {
		t.Errorf("after deleting QuickAdd(4): got quantity %d, want 1", totals[0].Quantity)
	}
}

func TestDeleteSession_cascadeRemovesScans(t *testing.T) {
	svc, db := setupTestEnv(t)
	seedProduct(t, db, "7896789", "Yerba 500g")
	sid := createSession(t, svc, "to-delete")
	ctx := context.Background()

	for range 5 {
		svc.ScanProduct(ctx, sid, "7896789")
	}

	if err := svc.DeleteSession(ctx, sid); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	// La sesión eliminada no debe aparecer en la lista
	sessions, _ := svc.GetSessions(ctx)
	for _, s := range sessions {
		if s.ID == sid {
			t.Errorf("sesión %d todavía existe después de DeleteSession", sid)
		}
	}

	// Los scans no deben quedar huérfanos en la DB
	var count int
	db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM inventory_scans WHERE session_id = ?`, sid,
	).Scan(&count)
	if count != 0 {
		t.Errorf("inventory_scans: quedan %d registros huérfanos tras DeleteSession", count)
	}
}
