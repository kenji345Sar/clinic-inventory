package distributorcatalog

import (
	"context"

	shareddomain "clinic-inventory/internal/domain/shared"
)

type DistributorRepository interface {
	Create(ctx context.Context, distributor *Distributor) error
	FindByID(ctx context.Context, id shareddomain.ID) (*Distributor, error)
	FindAll(ctx context.Context) ([]*Distributor, error)
}

type DistributorProductRepository interface {
	Create(ctx context.Context, product *DistributorProduct) error
	FindByID(ctx context.Context, id shareddomain.ID) (*DistributorProduct, error)
	// ExistsByDistributorAndCode は同一卸業者内で卸商品コードが既に使われているかを確認する。
	// 一意性はDistributorProduct集約内では判定せず、ここ（リポジトリ層）とDBのユニーク制約で担保する
	// (docs/architecture/domain-rules.md「卸連携コンテキスト」参照)。
	ExistsByDistributorAndCode(ctx context.Context, distributorID shareddomain.ID, code string) (bool, error)
	FindByDistributor(ctx context.Context, distributorID shareddomain.ID) ([]*DistributorProduct, error)
}
