package distributorcatalog

import "github.com/google/uuid"

// DistributorModel はDistributorの永続化用モデル（gorm）。
type DistributorModel struct {
	ID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name string    `gorm:"not null"`
}

func (DistributorModel) TableName() string { return "distributors" }

// DistributorProductModel はDistributorProductの永続化用モデル（gorm）。
// (distributor_id, distributor_product_code) のユニーク制約が
// 「卸商品コードは同一卸業者内で一意」の最終防衛線（domain-rules.md「卸連携コンテキスト」）。
type DistributorProductModel struct {
	ID                     uuid.UUID `gorm:"type:uuid;primaryKey"`
	DistributorID          uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:uidx_distributor_product_code"`
	DistributorProductCode string    `gorm:"not null;uniqueIndex:uidx_distributor_product_code"`
	Name                   string    `gorm:"not null"`
	VendorName             string    `gorm:"not null"`
	VendorProductCode      string
	JANCode                string `gorm:"column:jan_code;index"`
	Discontinued           bool   `gorm:"not null;default:false"`
}

func (DistributorProductModel) TableName() string { return "distributor_products" }
