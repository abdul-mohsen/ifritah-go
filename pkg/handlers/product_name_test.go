package handlers

import "testing"

func TestAddProductNamePrefersName(t *testing.T) {
	product := AddProduct{Name: "Product name", PartName: "Legacy name"}

	if got := addProductName(product); got == nil || *got != "Product name" {
		t.Fatalf("name = %v, want Product name", got)
	}
}

func TestAddProductNameFallsBackToPartName(t *testing.T) {
	product := AddProduct{PartName: "Legacy name"}

	if got := addProductName(product); got == nil || *got != "Legacy name" {
		t.Fatalf("name = %v, want Legacy name", got)
	}
}

func TestAddProductNameReturnsNilWhenEmpty(t *testing.T) {
	if got := addProductName(AddProduct{}); got != nil {
		t.Fatalf("name = %q, want nil", *got)
	}
}
