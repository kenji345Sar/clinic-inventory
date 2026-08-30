package distributorcatalog

import "github.com/google/uuid"

// DistributorModel はDistributorの永続化用モデル（gorm）。
type DistributorModel struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey"`
	// Code は卸コード。S3のフォルダ名(orders/{code}/, catalogs/{code}/)に使うため一意。
	Code string `gorm:"not null;uniqueIndex"`
	Name string `gorm:"not null"`
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
	// 標準単価（税抜・円）。NULLは「卸が単価を公表していない」を表す。
	// 0円との区別が必要なためポインタ（clinic-inventory-csv-functions/docs/distributor-catalog-import.md 2章）。
	UnitPrice    *int `gorm:""`
	Discontinued bool `gorm:"not null;default:false"`
}

func (DistributorProductModel) TableName() string { return "distributor_products" }

// DistributorProductFacilityPriceModel は医院別単価の永続化用モデル（gorm）。
// 同一性が(卸商品, クリニック)の組で決まるため、この2列を複合主キーにしている
// （＝同じ組が二重に登録されない）。
type DistributorProductFacilityPriceModel struct {
	DistributorProductID uuid.UUID `gorm:"type:uuid;primaryKey"`
	FacilityID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	UnitPrice            int       `gorm:"not null"`
}

func (DistributorProductFacilityPriceModel) TableName() string {
	return "distributor_product_facility_prices"
}
