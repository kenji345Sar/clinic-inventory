package distributorcatalog_test

import (
	"testing"

	distdomain "clinic-inventory/internal/domain/distributorcatalog"
)

func TestNewDistributor(t *testing.T) {
	t.Run("正常に作成できる", func(t *testing.T) {
		d, err := distdomain.NewDistributor("テスト卸業者")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if d.Name() != "テスト卸業者" {
			t.Errorf("Name() = %q, want %q", d.Name(), "テスト卸業者")
		}
	})

	t.Run("卸業者名が空だとエラー", func(t *testing.T) {
		_, err := distdomain.NewDistributor("")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
