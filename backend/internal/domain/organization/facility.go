package organization

import (
	"errors"

	shareddomain "clinic-inventory/internal/domain/shared"
)

// FacilityType はクリニックの種別。
type FacilityType string

const (
	FacilityTypeMedical FacilityType = "medical" // 医科
	FacilityTypeDental  FacilityType = "dental"  // 歯科
	FacilityTypeVet     FacilityType = "vet"     // 獣医
)

func (t FacilityType) valid() bool {
	switch t {
	case FacilityTypeMedical, FacilityTypeDental, FacilityTypeVet:
		return true
	default:
		return false
	}
}

// Facility（クリニック）。組織コンテキストの集約ルート。
// 必ずいずれかのCorporationに属する。Groupへの所属は任意
// (requirements.md 3章の未確定事項のため、フィールドだけ用意している)。
type Facility struct {
	id            shareddomain.ID
	name          string
	facilityType  FacilityType
	corporationID shareddomain.ID
	groupID       *shareddomain.ID
}

func NewFacility(name string, facilityType FacilityType, corporationID shareddomain.ID) (*Facility, error) {
	if name == "" {
		return nil, errors.New("クリニック名は必須です")
	}
	if corporationID.IsZero() {
		return nil, errors.New("クリニックはいずれかの法人に属する必要があります")
	}
	if !facilityType.valid() {
		return nil, errors.New("不正なクリニック種別です")
	}
	return &Facility{
		id:            shareddomain.NewID(),
		name:          name,
		facilityType:  facilityType,
		corporationID: corporationID,
	}, nil
}

func (f *Facility) ID() shareddomain.ID            { return f.id }
func (f *Facility) Name() string                   { return f.name }
func (f *Facility) Type() FacilityType             { return f.facilityType }
func (f *Facility) CorporationID() shareddomain.ID { return f.corporationID }
func (f *Facility) GroupID() *shareddomain.ID      { return f.groupID }

// AssignToGroup はクリニックをグループに所属させる。グループ運用が必要になった場合に使う。
func (f *Facility) AssignToGroup(groupID shareddomain.ID) {
	f.groupID = &groupID
}

// ReconstructFacility は永続化データからFacilityを復元する。バリデーションは行わない
// （すでに永続化された時点で妥当な状態であることが前提のため）。
func ReconstructFacility(id shareddomain.ID, name string, facilityType FacilityType, corporationID shareddomain.ID, groupID *shareddomain.ID) *Facility {
	return &Facility{
		id:            id,
		name:          name,
		facilityType:  facilityType,
		corporationID: corporationID,
		groupID:       groupID,
	}
}
