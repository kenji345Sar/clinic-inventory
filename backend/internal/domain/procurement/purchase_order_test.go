package procurement_test

import (
	"testing"

	procdomain "clinic-inventory/internal/domain/procurement"
	shareddomain "clinic-inventory/internal/domain/shared"
)

func TestNewPurchaseOrder(t *testing.T) {
	facilityID := shareddomain.NewID()
	distributorID := shareddomain.NewID()

	t.Run("正常に作成でき、初期状態はdraft", func(t *testing.T) {
		o, err := procdomain.NewPurchaseOrder(facilityID, distributorID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if o.Status() != procdomain.StatusDraft {
			t.Errorf("Status() = %q, want %q", o.Status(), procdomain.StatusDraft)
		}
		if len(o.Lines()) != 0 {
			t.Errorf("新規発注の明細は0件のはず, got %d", len(o.Lines()))
		}
	})

	t.Run("クリニックIDが未指定だとエラー", func(t *testing.T) {
		if _, err := procdomain.NewPurchaseOrder(shareddomain.ID{}, distributorID); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("卸業者IDが未指定だとエラー", func(t *testing.T) {
		if _, err := procdomain.NewPurchaseOrder(facilityID, shareddomain.ID{}); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestPurchaseOrder_AddLine(t *testing.T) {
	facilityID := shareddomain.NewID()
	distributorID := shareddomain.NewID()

	newOrder := func(t *testing.T) *procdomain.PurchaseOrder {
		t.Helper()
		o, err := procdomain.NewPurchaseOrder(facilityID, distributorID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return o
	}

	t.Run("明細を追加できる", func(t *testing.T) {
		o := newOrder(t)
		productID := shareddomain.NewID()
		if err := o.AddLine(productID, 5); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lines := o.Lines()
		if len(lines) != 1 {
			t.Fatalf("明細は1件のはず, got %d", len(lines))
		}
		if lines[0].Quantity() != 5 {
			t.Errorf("Quantity() = %d, want 5", lines[0].Quantity())
		}
	})

	t.Run("同一商品を追加すると数量が加算され明細は増えない", func(t *testing.T) {
		o := newOrder(t)
		productID := shareddomain.NewID()
		if err := o.AddLine(productID, 5); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := o.AddLine(productID, 3); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lines := o.Lines()
		if len(lines) != 1 {
			t.Fatalf("明細は1件のままのはず, got %d", len(lines))
		}
		if lines[0].Quantity() != 8 {
			t.Errorf("Quantity() = %d, want 8", lines[0].Quantity())
		}
	})

	t.Run("数量0以下はエラー", func(t *testing.T) {
		o := newOrder(t)
		if err := o.AddLine(shareddomain.NewID(), 0); err == nil {
			t.Fatal("expected error, got nil")
		}
		if err := o.AddLine(shareddomain.NewID(), -1); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("クリニック商品IDが未指定だとエラー", func(t *testing.T) {
		o := newOrder(t)
		if err := o.AddLine(shareddomain.ID{}, 5); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("確定後は明細を追加できない", func(t *testing.T) {
		o := newOrder(t)
		if err := o.AddLine(shareddomain.NewID(), 5); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := o.Confirm(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := o.AddLine(shareddomain.NewID(), 1); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestPurchaseOrder_Confirm(t *testing.T) {
	facilityID := shareddomain.NewID()
	distributorID := shareddomain.NewID()

	t.Run("明細があれば確定でき、状態がconfirmedになる", func(t *testing.T) {
		o, err := procdomain.NewPurchaseOrder(facilityID, distributorID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := o.AddLine(shareddomain.NewID(), 5); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := o.Confirm(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if o.Status() != procdomain.StatusConfirmed {
			t.Errorf("Status() = %q, want %q", o.Status(), procdomain.StatusConfirmed)
		}
	})

	t.Run("明細が0件だと確定できない", func(t *testing.T) {
		o, err := procdomain.NewPurchaseOrder(facilityID, distributorID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := o.Confirm(); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("二重確定はエラー", func(t *testing.T) {
		o, err := procdomain.NewPurchaseOrder(facilityID, distributorID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := o.AddLine(shareddomain.NewID(), 5); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := o.Confirm(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := o.Confirm(); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
