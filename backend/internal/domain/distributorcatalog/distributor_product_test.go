package distributorcatalog_test

import (
	"testing"

	distdomain "clinic-inventory/internal/domain/distributorcatalog"
	shareddomain "clinic-inventory/internal/domain/shared"
)

func TestNewDistributorProduct(t *testing.T) {
	distributorID := shareddomain.NewID()

	t.Run("正常に作成できる", func(t *testing.T) {
		p, err := distdomain.NewDistributorProduct(distributorID, "D-0001", "テスト製品", "テストベンダー")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.DistributorProductCode() != "D-0001" {
			t.Errorf("DistributorProductCode() = %q, want %q", p.DistributorProductCode(), "D-0001")
		}
		if p.Discontinued() {
			t.Error("Discontinued() should be false by default")
		}
	})

	t.Run("卸業者IDが未指定だとエラー", func(t *testing.T) {
		_, err := distdomain.NewDistributorProduct(shareddomain.ID{}, "D-0001", "テスト製品", "テストベンダー")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("卸商品コードが空だとエラー", func(t *testing.T) {
		_, err := distdomain.NewDistributorProduct(distributorID, "", "テスト製品", "テストベンダー")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("商品名が空だとエラー", func(t *testing.T) {
		_, err := distdomain.NewDistributorProduct(distributorID, "D-0001", "", "テストベンダー")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("ベンダー名が空だとエラー", func(t *testing.T) {
		_, err := distdomain.NewDistributorProduct(distributorID, "D-0001", "テスト製品", "")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("Discontinueで廃盤にできる", func(t *testing.T) {
		p, err := distdomain.NewDistributorProduct(distributorID, "D-0001", "テスト製品", "テストベンダー")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		p.Discontinue()
		if !p.Discontinued() {
			t.Error("Discontinued() should be true after Discontinue()")
		}
	})
}
