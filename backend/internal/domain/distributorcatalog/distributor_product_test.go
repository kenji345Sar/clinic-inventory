package distributorcatalog_test

import (
	"testing"

	distdomain "clinic-inventory/internal/domain/distributorcatalog"
	shareddomain "clinic-inventory/internal/domain/shared"
)

// yen は単価をポインタで渡すためのテスト用ヘルパー。
// 標準単価は「非公表(nil)」を表現するためポインタになっている。
func yen(v int) *int { return &v }

func TestNewDistributorProduct(t *testing.T) {
	distributorID := shareddomain.NewID()

	t.Run("正常に作成できる", func(t *testing.T) {
		p, err := distdomain.NewDistributorProduct(distributorID, "D-0001", "テスト製品", "テストベンダー", yen(1200))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.DistributorProductCode() != "D-0001" {
			t.Errorf("DistributorProductCode() = %q, want %q", p.DistributorProductCode(), "D-0001")
		}
		if p.UnitPrice() == nil || *p.UnitPrice() != 1200 {
			t.Errorf("UnitPrice() = %v, want 1200", p.UnitPrice())
		}
		if p.Discontinued() {
			t.Error("Discontinued() should be false by default")
		}
	})

	t.Run("卸業者IDが未指定だとエラー", func(t *testing.T) {
		_, err := distdomain.NewDistributorProduct(shareddomain.ID{}, "D-0001", "テスト製品", "テストベンダー", yen(1200))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("卸商品コードが空だとエラー", func(t *testing.T) {
		_, err := distdomain.NewDistributorProduct(distributorID, "", "テスト製品", "テストベンダー", yen(1200))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("商品名が空だとエラー", func(t *testing.T) {
		_, err := distdomain.NewDistributorProduct(distributorID, "D-0001", "", "テストベンダー", yen(1200))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("ベンダー名が空だとエラー", func(t *testing.T) {
		_, err := distdomain.NewDistributorProduct(distributorID, "D-0001", "テスト製品", "", yen(1200))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("単価が0以下だとエラー", func(t *testing.T) {
		if _, err := distdomain.NewDistributorProduct(distributorID, "D-0001", "テスト製品", "テストベンダー", yen(0)); err == nil {
			t.Fatal("expected error, got nil")
		}
		if _, err := distdomain.NewDistributorProduct(distributorID, "D-0001", "テスト製品", "テストベンダー", yen(-1)); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("単価nilは非公表として作成できる", func(t *testing.T) {
		p, err := distdomain.NewDistributorProduct(distributorID, "D-0001", "テスト製品", "テストベンダー", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.HasUnitPrice() {
			t.Error("HasUnitPrice() should be false when unit price is not disclosed")
		}
	})

	t.Run("Discontinueで廃盤にできる", func(t *testing.T) {
		p, err := distdomain.NewDistributorProduct(distributorID, "D-0001", "テスト製品", "テストベンダー", yen(1200))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		p.Discontinue()
		if !p.Discontinued() {
			t.Error("Discontinued() should be true after Discontinue()")
		}
	})
}
