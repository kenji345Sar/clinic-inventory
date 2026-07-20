package organization_test

import (
	"testing"

	orgdomain "clinic-inventory/internal/domain/organization"
)

func TestNewCorporation(t *testing.T) {
	t.Run("正常に作成できる", func(t *testing.T) {
		c, err := orgdomain.NewCorporation("テスト法人")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.Name() != "テスト法人" {
			t.Errorf("Name() = %q, want %q", c.Name(), "テスト法人")
		}
		if c.ID().IsZero() {
			t.Error("ID() should not be zero")
		}
	})

	t.Run("法人名が空だとエラー", func(t *testing.T) {
		_, err := orgdomain.NewCorporation("")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
