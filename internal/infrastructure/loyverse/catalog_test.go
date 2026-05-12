package loyverse

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// newTestClient crea un Client apuntando al httptest.Server dado.
func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	c, err := NewClientWithBaseURL("test-token", srv.URL)
	if err != nil {
		t.Fatalf("NewClientWithBaseURL: %v", err)
	}
	return c
}

func TestGetCategories_returnsSinglePage(t *testing.T) {
	want := []Category{
		{ID: "cat-1", Name: "Bebidas", Color: "#FF0000"},
		{ID: "cat-2", Name: "Snacks", Color: "#00FF00"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/categories" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		json.NewEncoder(w).Encode(CategoriesResponse{Categories: want, Cursor: nil})
	}))
	t.Cleanup(srv.Close)

	got, err := newTestClient(t, srv).GetCategories()
	if err != nil {
		t.Fatalf("GetCategories() error = %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("GetCategories() mismatch (-want +got):\n%s", diff)
	}
}

func TestGetCategories_paginatesThroughCursor(t *testing.T) {
	page1 := []Category{{ID: "cat-1", Name: "Bebidas"}}
	page2 := []Category{{ID: "cat-2", Name: "Snacks"}}
	cursor1 := "cursor-abc"

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.URL.Query().Get("cursor") == "" {
			json.NewEncoder(w).Encode(CategoriesResponse{Categories: page1, Cursor: &cursor1})
		} else {
			json.NewEncoder(w).Encode(CategoriesResponse{Categories: page2, Cursor: nil})
		}
	}))
	t.Cleanup(srv.Close)

	got, err := newTestClient(t, srv).GetCategories()
	if err != nil {
		t.Fatalf("GetCategories() error = %v", err)
	}

	want := append(page1, page2...)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("GetCategories() paginated mismatch (-want +got):\n%s", diff)
	}
	if callCount != 2 {
		t.Errorf("expected 2 HTTP calls, got %d", callCount)
	}
}

func TestGetCategories_emptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(CategoriesResponse{Categories: nil, Cursor: nil})
	}))
	t.Cleanup(srv.Close)

	got, err := newTestClient(t, srv).GetCategories()
	if err != nil {
		t.Fatalf("GetCategories() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d categories", len(got))
	}
}

func TestCreateItem_sendsCorrectPayloadAndReturnsItem(t *testing.T) {
	wantReq := CreateItemRequest{
		ItemName:   "Mate 1kg",
		CategoryID: "cat-1",
		TrackStock: true,
		Variants: []CreateVariantRequest{{
			DefaultPricingType: "FIXED",
			Price:              9.99,
			Barcode:            "1234567890123",
		}},
	}

	returnedItem := LoyverseItem{
		ID:   "item-new",
		Name: "Mate 1kg",
		Variants: []LoyverseVariant{
			{ID: "var-new", Barcode: "1234567890123"},
		},
	}

	var receivedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/items" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		receivedBody, _ = io.ReadAll(r.Body)
		json.NewEncoder(w).Encode(returnedItem)
	}))
	t.Cleanup(srv.Close)

	got, err := newTestClient(t, srv).CreateItem(wantReq)
	if err != nil {
		t.Fatalf("CreateItem() error = %v", err)
	}

	// Verificar que el item devuelto es el correcto.
	if diff := cmp.Diff(returnedItem, *got); diff != "" {
		t.Errorf("CreateItem() returned item mismatch (-want +got):\n%s", diff)
	}

	// Verificar que el payload enviado es exactamente el esperado.
	var sentReq CreateItemRequest
	if err := json.Unmarshal(receivedBody, &sentReq); err != nil {
		t.Fatalf("unmarshal sent body: %v", err)
	}
	if diff := cmp.Diff(wantReq, sentReq); diff != "" {
		t.Errorf("CreateItem() sent payload mismatch (-want +got):\n%s", diff)
	}
}

func TestCreateItem_propagatesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"invalid token"}`))
	}))
	t.Cleanup(srv.Close)

	_, err := newTestClient(t, srv).CreateItem(CreateItemRequest{ItemName: "X"})
	if err == nil {
		t.Fatal("expected error for 401 response, got nil")
	}
}
