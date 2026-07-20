package procurement

import "github.com/google/uuid"

// PurchaseOrderModel は発注(親)の永続化用モデル(gorm)。
// 明細は PurchaseOrderLineModel に別テーブルで持つ(1発注 : N明細)。
type PurchaseOrderModel struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	FacilityID    uuid.UUID `gorm:"type:uuid;not null;index"`
	DistributorID uuid.UUID `gorm:"type:uuid;not null;index"`
	Status        string    `gorm:"not null"`
}

func (PurchaseOrderModel) TableName() string { return "purchase_orders" }

// PurchaseOrderLineModel は発注明細(子)の永続化用モデル(gorm)。
// PurchaseOrderID で親に紐付く。明細自体の一意性は集約の外では判定しない
// (同一クリニック商品の重複は集約側の AddLine が数量加算で吸収する)。
type PurchaseOrderLineModel struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey"`
	PurchaseOrderID uuid.UUID `gorm:"type:uuid;not null;index"`
	ClinicProductID uuid.UUID `gorm:"type:uuid;not null;index"`
	Quantity        int       `gorm:"not null"`
}

func (PurchaseOrderLineModel) TableName() string { return "purchase_order_lines" }
