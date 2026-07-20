package productcatalog_test

import (
	"testing"

	proddomain "clinic-inventory/internal/domain/productcatalog"
	shareddomain "clinic-inventory/internal/domain/shared"
)

func TestNewClinicProduct(t *testing.T) {
	facilityID := shareddomain.NewID()
	distributorProductID := shareddomain.NewID()

	t.Run("正常に作成できる", func(t *testing.T) {
		p, err := proddomain.NewClinicProduct(facilityID, "C-0001", "抗生剤100mg", distributorProductID, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.ProductCode() != "C-0001" {
			t.Errorf("ProductCode() = %q, want %q", p.ProductCode(), "C-0001")
		}
		if p.ReorderPoint() != 10 {
			t.Errorf("ReorderPoint() = %d, want 10", p.ReorderPoint())
		}
	})

	t.Run("発注点0で作成できる", func(t *testing.T) {
		if _, err := proddomain.NewClinicProduct(facilityID, "C-0001", "抗生剤100mg", distributorProductID, 0); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("クリニックIDが未指定だとエラー", func(t *testing.T) {
		if _, err := proddomain.NewClinicProduct(shareddomain.ID{}, "C-0001", "抗生剤100mg", distributorProductID, 10); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("商品コードが空だとエラー", func(t *testing.T) {
		if _, err := proddomain.NewClinicProduct(facilityID, "", "抗生剤100mg", distributorProductID, 10); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("商品名が空だとエラー", func(t *testing.T) {
		if _, err := proddomain.NewClinicProduct(facilityID, "C-0001", "", distributorProductID, 10); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("卸商品への紐付けが無いとエラー", func(t *testing.T) {
		if _, err := proddomain.NewClinicProduct(facilityID, "C-0001", "抗生剤100mg", shareddomain.ID{}, 10); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("発注点が負だとエラー", func(t *testing.T) {
		if _, err := proddomain.NewClinicProduct(facilityID, "C-0001", "抗生剤100mg", distributorProductID, -1); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("ChangeReorderPointで発注点を変更できる", func(t *testing.T) {
		p, err := proddomain.NewClinicProduct(facilityID, "C-0001", "抗生剤100mg", distributorProductID, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := p.ChangeReorderPoint(20); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.ReorderPoint() != 20 {
			t.Errorf("ReorderPoint() = %d, want 20", p.ReorderPoint())
		}
	})

	t.Run("ChangeReorderPointに負を渡すとエラー", func(t *testing.T) {
		p, err := proddomain.NewClinicProduct(facilityID, "C-0001", "抗生剤100mg", distributorProductID, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := p.ChangeReorderPoint(-1); err == nil {
			t.Fatal("expected error, got nil")
		}
		if p.ReorderPoint() != 10 {
			t.Errorf("ReorderPoint() should remain 10, got %d", p.ReorderPoint())
		}
	})
}
