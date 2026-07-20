package productcatalog

import "github.com/google/uuid"

// ClinicProductModel はClinicProductの永続化用モデル（gorm）。
// (facility_id, product_code) のユニーク制約が
// 「クリニック商品コードは同一クリニック内で一意」の最終防衛線。
type ClinicProductModel struct {
	ID                   uuid.UUID `gorm:"type:uuid;primaryKey"`
	FacilityID           uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:uidx_clinic_product_code"`
	ProductCode          string    `gorm:"not null;uniqueIndex:uidx_clinic_product_code"`
	Name                 string    `gorm:"not null"`
	DistributorProductID uuid.UUID `gorm:"type:uuid;not null;index"`
	JANCode              string    `gorm:"column:jan_code;index"`
	ReorderPoint         int       `gorm:"not null;default:0"`
}

func (ClinicProductModel) TableName() string { return "clinic_products" }
