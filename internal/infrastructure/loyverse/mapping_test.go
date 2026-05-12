package loyverse

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestBuildVariantMap_byBarcode(t *testing.T) {
	items := []LoyverseItem{
		{ID: "item-1", Name: "Yerba", Variants: []LoyverseVariant{
			{ID: "var-1", Barcode: "111"},
			{ID: "var-2", Barcode: "222"},
		}},
		{ID: "item-2", Name: "Mate", Variants: []LoyverseVariant{
			{ID: "var-3", Barcode: "333"},
			{ID: "var-4", Barcode: ""}, // sin barcode — no debe entrar al mapa
		}},
	}

	vm := BuildVariantMap(items)

	wantByBarcode := map[string]VariantInfo{
		"111": {VariantID: "var-1"},
		"222": {VariantID: "var-2"},
		"333": {VariantID: "var-3"},
	}
	if diff := cmp.Diff(wantByBarcode, vm.ByBarcode); diff != "" {
		t.Errorf("BuildVariantMap ByBarcode mismatch (-want +got):\n%s", diff)
	}

	if _, ok := vm.ByBarcode[""]; ok {
		t.Error("ByBarcode contains empty-string key — variants sin barcode no deben mapearse")
	}
}

func TestBuildStoreMap_mapsVariantToStore(t *testing.T) {
	records := []InventoryRecord{
		{VariantID: "var-1", StoreID: "store-A", Stock: 10},
		{VariantID: "var-2", StoreID: "store-A", Stock: 5},
		{VariantID: "var-3", StoreID: "store-B", Stock: 0},
	}

	got := BuildStoreMap(records)
	want := map[string]string{
		"var-1": "store-A",
		"var-2": "store-A",
		"var-3": "store-B",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("BuildStoreMap mismatch (-want +got):\n%s", diff)
	}
}

func TestBuildCurrentStockMap_compositeKey(t *testing.T) {
	records := []InventoryRecord{
		{VariantID: "var-1", StoreID: "store-A", Stock: 7},
		{VariantID: "var-1", StoreID: "store-B", Stock: 3},
		{VariantID: "var-2", StoreID: "store-A", Stock: 0},
	}

	got := BuildCurrentStockMap(records)
	want := map[string]float64{
		"var-1|store-A": 7,
		"var-1|store-B": 3,
		"var-2|store-A": 0,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("BuildCurrentStockMap mismatch (-want +got):\n%s", diff)
	}
}

func TestBuildVariantMap_emptyItems(t *testing.T) {
	vm := BuildVariantMap(nil)
	if len(vm.ByBarcode) != 0 {
		t.Errorf("expected empty ByBarcode for nil input, got %d entries", len(vm.ByBarcode))
	}
}
