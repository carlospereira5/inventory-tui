package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/carlospereira5/inventory-tui/internal/infrastructure/loyverse"
)

// mockItemsServer crea un httptest.Server que sirve GET /items con los items dados.
func mockItemsServer(t *testing.T, items []loyverse.LoyverseItem) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/items" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{"items": items, "cursor": nil})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestLoadCatalogFromLoyverse_upsertsAllVariants(t *testing.T) {
	svc, _ := setupTestEnv(t)

	items := []loyverse.LoyverseItem{
		{ID: "item-1", Name: "Yerba 500g", Variants: []loyverse.LoyverseVariant{
			{ID: "var-1", Barcode: "1111111111111"},
		}},
		{ID: "item-2", Name: "Mate", Variants: []loyverse.LoyverseVariant{
			{ID: "var-2", Barcode: "2222222222222"},
			{ID: "var-3", Barcode: "3333333333333"},
		}},
	}

	srv := mockItemsServer(t, items)
	client, err := loyverse.NewClientWithBaseURL("test-token", srv.URL)
	if err != nil {
		t.Fatalf("client: %v", err)
	}

	count, err := svc.LoadCatalogFromLoyverse(context.Background(), client)
	if err != nil {
		t.Fatalf("LoadCatalogFromLoyverse() error = %v", err)
	}
	// 3 variantes con barcode → 3 productos
	if count != 3 {
		t.Errorf("got count %d, want 3", count)
	}

	// Verificar que los productos son escaneables
	ctx := context.Background()
	sid := createSession(t, svc, "test")
	for _, tc := range []struct{ barcode, name string }{
		{"1111111111111", "Yerba 500g"},
		{"2222222222222", "Mate"},
		{"3333333333333", "Mate"},
	} {
		p, _, err := svc.ScanProduct(ctx, sid, tc.barcode)
		if err != nil {
			t.Fatalf("ScanProduct(%s): %v", tc.barcode, err)
		}
		if p == nil {
			t.Errorf("barcode %s not found after LoadCatalogFromLoyverse", tc.barcode)
			continue
		}
		if p.Name != tc.name {
			t.Errorf("barcode %s: got name %q, want %q", tc.barcode, p.Name, tc.name)
		}
	}
}

func TestLoadCatalogFromLoyverse_skipsVariantsWithoutBarcode(t *testing.T) {
	svc, _ := setupTestEnv(t)

	items := []loyverse.LoyverseItem{
		{ID: "item-1", Name: "Sin barcode", Variants: []loyverse.LoyverseVariant{
			{ID: "var-1", Barcode: ""},              // sin barcode — debe ignorarse
			{ID: "var-2", Barcode: "9999999999999"}, // con barcode — debe importarse
		}},
	}

	srv := mockItemsServer(t, items)
	client, _ := loyverse.NewClientWithBaseURL("test-token", srv.URL)

	count, err := svc.LoadCatalogFromLoyverse(context.Background(), client)
	if err != nil {
		t.Fatalf("LoadCatalogFromLoyverse() error = %v", err)
	}
	if count != 1 {
		t.Errorf("got count %d, want 1 (empty barcode should be skipped)", count)
	}
}

func TestLoadCatalogFromLoyverse_updatesExistingProduct(t *testing.T) {
	svc, db := setupTestEnv(t)
	// Sembramos el producto con nombre viejo
	seedProduct(t, db, "1111111111111", "Nombre Viejo")

	items := []loyverse.LoyverseItem{
		{ID: "item-1", Name: "Nombre Nuevo", Variants: []loyverse.LoyverseVariant{
			{ID: "var-1", Barcode: "1111111111111"},
		}},
	}

	srv := mockItemsServer(t, items)
	client, _ := loyverse.NewClientWithBaseURL("test-token", srv.URL)

	if _, err := svc.LoadCatalogFromLoyverse(context.Background(), client); err != nil {
		t.Fatalf("LoadCatalogFromLoyverse() error = %v", err)
	}

	// El nombre debe haberse actualizado
	sid := createSession(t, svc, "test")
	p, _, err := svc.ScanProduct(context.Background(), sid, "1111111111111")
	if err != nil {
		t.Fatalf("ScanProduct: %v", err)
	}
	if p == nil {
		t.Fatal("product not found")
	}
	if p.Name != "Nombre Nuevo" {
		t.Errorf("name = %q, want %q (should have been updated)", p.Name, "Nombre Nuevo")
	}
}

func TestLoadCatalogFromLoyverse_loyverseAPIError(t *testing.T) {
	svc, _ := setupTestEnv(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"invalid token"}`))
	}))
	t.Cleanup(srv.Close)

	client, _ := loyverse.NewClientWithBaseURL("test-token", srv.URL)

	_, err := svc.LoadCatalogFromLoyverse(context.Background(), client)
	if err == nil {
		t.Fatal("expected error for 401 response, got nil")
	}
}
