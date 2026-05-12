package service_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/carlospereira5/inventory-tui/internal/infrastructure/loyverse"

	"github.com/google/go-cmp/cmp"
)

// mockCatalogServer crea un httptest.Server que responde a GET /categories y POST /items.
// categories define la respuesta del GET. El POST responde con el item que se le pasa a setUp.
func mockCatalogServer(t *testing.T, categories []loyverse.Category, createdItem loyverse.LoyverseItem) (*httptest.Server, func() *loyverse.CreateItemRequest) {
	t.Helper()

	var captured *loyverse.CreateItemRequest

	mux := http.NewServeMux()

	mux.HandleFunc("GET /categories", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"categories": categories,
			"cursor":     nil,
		})
	})

	mux.HandleFunc("POST /items", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req loyverse.CreateItemRequest
		json.Unmarshal(body, &req)
		captured = &req
		json.NewEncoder(w).Encode(createdItem)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv, func() *loyverse.CreateItemRequest { return captured }
}

func TestGetCategoriesWithClient_returnsAllCategories(t *testing.T) {
	svc, _ := setupTestEnv(t)

	want := []loyverse.Category{
		{ID: "cat-1", Name: "Bebidas"},
		{ID: "cat-2", Name: "Snacks"},
	}

	srv, _ := mockCatalogServer(t, want, loyverse.LoyverseItem{})
	client, err := loyverse.NewClientWithBaseURL("test-token", srv.URL)
	if err != nil {
		t.Fatalf("client: %v", err)
	}

	got, err := svc.GetCategoriesWithClient(client)
	if err != nil {
		t.Fatalf("GetCategoriesWithClient() error = %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("GetCategoriesWithClient() mismatch (-want +got):\n%s", diff)
	}
}

func TestCreateProductWithClient_createsInLoyverseAndSeededLocally(t *testing.T) {
	svc, _ := setupTestEnv(t)

	returnedItem := loyverse.LoyverseItem{
		ID:   "item-001",
		Name: "Yerba 500g",
		Variants: []loyverse.LoyverseVariant{
			{ID: "var-001", Barcode: "1234567890123"},
		},
	}

	srv, getReq := mockCatalogServer(t, nil, returnedItem)
	client, err := loyverse.NewClientWithBaseURL("test-token", srv.URL)
	if err != nil {
		t.Fatalf("client: %v", err)
	}

	ctx := context.Background()
	p, err := svc.CreateProductWithClient(ctx, client, "1234567890123", "Yerba 500g", 9.99, "cat-1")
	if err != nil {
		t.Fatalf("CreateProductWithClient() error = %v", err)
	}

	// El producto devuelto debe tener el barcode y nombre correctos.
	if p.Barcode != "1234567890123" {
		t.Errorf("product barcode = %q, want %q", p.Barcode, "1234567890123")
	}
	if p.Name != "Yerba 500g" {
		t.Errorf("product name = %q, want %q", p.Name, "Yerba 500g")
	}

	// El payload enviado a Loyverse debe incluir barcode, nombre, precio y categoría.
	req := getReq()
	if req == nil {
		t.Fatal("no POST /items request captured")
	}
	wantReq := loyverse.CreateItemRequest{
		ItemName:   "Yerba 500g",
		CategoryID: "cat-1",
		TrackStock: true,
		Variants: []loyverse.CreateVariantRequest{{
			DefaultPricingType: "FIXED",
			Price:              9.99,
			Barcode:            "1234567890123",
		}},
	}
	if diff := cmp.Diff(wantReq, *req); diff != "" {
		t.Errorf("POST /items payload mismatch (-want +got):\n%s", diff)
	}

	// El producto debe haber quedado en el catálogo local (podemos escanearlo ahora).
	sid := createSession(t, svc, "test")
	product, _, err := svc.ScanProduct(ctx, sid, "1234567890123")
	if err != nil {
		t.Fatalf("ScanProduct after create: %v", err)
	}
	if product == nil {
		t.Fatal("product not found in local catalog after CreateProductWithClient")
	}
}

func TestCreateProductWithClient_usesLoyverseBarcodeWhenDiffers(t *testing.T) {
	svc, _ := setupTestEnv(t)

	// Loyverse devuelve un barcode normalizado diferente al enviado.
	returnedItem := loyverse.LoyverseItem{
		ID:   "item-001",
		Name: "Producto X",
		Variants: []loyverse.LoyverseVariant{
			{ID: "var-001", Barcode: "LOYVERSE_NORMALIZED_BC"},
		},
	}

	srv, _ := mockCatalogServer(t, nil, returnedItem)
	client, _ := loyverse.NewClientWithBaseURL("test-token", srv.URL)

	p, err := svc.CreateProductWithClient(context.Background(), client, "ORIGINAL_BC", "Producto X", 5.00, "")
	if err != nil {
		t.Fatalf("CreateProductWithClient() error = %v", err)
	}

	// Debe usar el barcode normalizado de Loyverse.
	if p.Barcode != "LOYVERSE_NORMALIZED_BC" {
		t.Errorf("barcode = %q, want %q (loyverse normalized)", p.Barcode, "LOYVERSE_NORMALIZED_BC")
	}
}

func TestCreateProductWithClient_loyverseAPIError_doesNotSeedLocally(t *testing.T) {
	svc, _ := setupTestEnv(t)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /items", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"internal server error"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client, _ := loyverse.NewClientWithBaseURL("test-token", srv.URL)

	_, err := svc.CreateProductWithClient(context.Background(), client, "9999999", "Fallo", 1.0, "")
	if err == nil {
		t.Fatal("expected error from Loyverse 500, got nil")
	}

	// El producto no debe estar en el catálogo local (Loyverse falló).
	sid := createSession(t, svc, "test")
	product, _, scanErr := svc.ScanProduct(context.Background(), sid, "9999999")
	if scanErr != nil {
		t.Fatalf("ScanProduct: %v", scanErr)
	}
	if product != nil {
		t.Error("product seeded locally despite Loyverse API error — should not happen")
	}
}
