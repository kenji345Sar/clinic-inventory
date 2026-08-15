package distributorcatalog_test

import (
	"testing"

	distdomain "clinic-inventory/internal/domain/distributorcatalog"
)

func TestNewDistributor(t *testing.T) {
	t.Run("正常に作成できる", func(t *testing.T) {
		d, err := distdomain.NewDistributor("oroshi-b", "テスト卸業者")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if d.Code() != "oroshi-b" {
			t.Errorf("Code() = %q, want %q", d.Code(), "oroshi-b")
		}
		if d.Name() != "テスト卸業者" {
			t.Errorf("Name() = %q, want %q", d.Name(), "テスト卸業者")
		}
	})

	t.Run("卸業者名が空だとエラー", func(t *testing.T) {
		_, err := distdomain.NewDistributor("oroshi-b", "")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("卸コードが空だとエラー", func(t *testing.T) {
		_, err := distdomain.NewDistributor("", "テスト卸業者")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	// 卸コードはS3のフォルダ名にそのまま使うため、記号や日本語・大文字を許すと
	// URLエンコードや大文字小文字の扱いで事故る。
	t.Run("卸コードに使えない文字はエラー", func(t *testing.T) {
		for _, code := range []string{"卸B", "Oroshi-B", "oroshi b", "oroshi/b", "-oroshi"} {
			if _, err := distdomain.NewDistributor(code, "テスト卸業者"); err == nil {
				t.Errorf("code %q は拒否されるべき", code)
			}
		}
	})
}
