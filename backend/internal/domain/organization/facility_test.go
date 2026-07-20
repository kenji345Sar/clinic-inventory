package organization_test

import (
	"testing"

	orgdomain "clinic-inventory/internal/domain/organization"
	shareddomain "clinic-inventory/internal/domain/shared"
)

func TestNewFacility(t *testing.T) {
	corporationID := shareddomain.NewID()

	t.Run("正常に作成できる", func(t *testing.T) {
		f, err := orgdomain.NewFacility("テストクリニック", orgdomain.FacilityTypeMedical, corporationID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.Name() != "テストクリニック" {
			t.Errorf("Name() = %q, want %q", f.Name(), "テストクリニック")
		}
		if f.CorporationID() != corporationID {
			t.Errorf("CorporationID() = %v, want %v", f.CorporationID(), corporationID)
		}
		if f.GroupID() != nil {
			t.Error("GroupID() should be nil by default")
		}
	})

	t.Run("名前が空だとエラー", func(t *testing.T) {
		_, err := orgdomain.NewFacility("", orgdomain.FacilityTypeMedical, corporationID)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("法人IDが未指定だとエラー", func(t *testing.T) {
		_, err := orgdomain.NewFacility("テストクリニック", orgdomain.FacilityTypeMedical, shareddomain.ID{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("不正な種別だとエラー", func(t *testing.T) {
		_, err := orgdomain.NewFacility("テストクリニック", orgdomain.FacilityType("invalid"), corporationID)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("AssignToGroupでグループに所属させられる", func(t *testing.T) {
		f, err := orgdomain.NewFacility("テストクリニック", orgdomain.FacilityTypeDental, corporationID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		groupID := shareddomain.NewID()
		f.AssignToGroup(groupID)
		if f.GroupID() == nil || *f.GroupID() != groupID {
			t.Errorf("GroupID() = %v, want %v", f.GroupID(), groupID)
		}
	})
}
