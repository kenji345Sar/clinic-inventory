package productcatalog

import (
	"context"

	shareddomain "clinic-inventory/internal/domain/shared"
)

type ClinicProductRepository interface {
	Create(ctx context.Context, product *ClinicProduct) error
	FindByID(ctx context.Context, id shareddomain.ID) (*ClinicProduct, error)
	// ExistsByFacilityAndCode は同一クリニック内で商品コードが既に使われているかを確認する。
	// 一意性は集約内では判定せず、ここ（リポジトリ層）とDBのユニーク制約で担保する。
	ExistsByFacilityAndCode(ctx context.Context, facilityID shareddomain.ID, productCode string) (bool, error)
	FindByFacility(ctx context.Context, facilityID shareddomain.ID) ([]*ClinicProduct, error)
	// FindByFacilityAndJAN はバーコード消費の入口。JANでクリニック商品を引き当てる。
	FindByFacilityAndJAN(ctx context.Context, facilityID shareddomain.ID, janCode string) (*ClinicProduct, error)
}
