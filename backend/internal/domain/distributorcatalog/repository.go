package distributorcatalog

import (
	"context"

	shareddomain "clinic-inventory/internal/domain/shared"
)

type DistributorRepository interface {
	Create(ctx context.Context, distributor *Distributor) error
	// ExistsByCode は卸コードが既に使われているかを確認する。卸コードはS3のフォルダ名に
	// なるため、重複すると別の卸のCSVが混ざる。DBのユニーク制約と合わせて二重で防ぐ。
	ExistsByCode(ctx context.Context, code string) (bool, error)
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

// FacilityPriceRepository は医院別単価の参照を担う。
// 医院別単価の登録・更新は卸から届くCSVの取り込み（別リポジトリ clinic-inventory-csv-functions）が行うため、
// backend側は参照のみを持つ。
type FacilityPriceRepository interface {
	// FindByProductAndFacility は該当する医院別単価を返す。設定が無いのは異常ではなく
	// 通常のケース（標準単価を使う卸）のため、見つからない場合は (nil, nil) を返す。
	FindByProductAndFacility(ctx context.Context, distributorProductID, facilityID shareddomain.ID) (*FacilityPrice, error)
	// FindByFacility はそのクリニック向けに設定されている医院別単価をまとめて返す。
	// 卸商品一覧（1社数千件）の表示で1件ずつ引くとクエリが商品数分になるため、一括で取る。
	FindByFacility(ctx context.Context, facilityID shareddomain.ID) ([]*FacilityPrice, error)
	// FindByProduct は1商品に設定されている医院別単価を返す（卸ポータルの内訳表示用）。
	FindByProduct(ctx context.Context, distributorProductID shareddomain.ID) ([]*FacilityPrice, error)
	// CountByProducts は商品ごとの医院別単価の設定件数を返す。卸商品一覧で
	// 「医院別（N院）」と出すために使う。単価そのものは返さない（一覧では不要なため）。
	CountByProducts(ctx context.Context, distributorProductIDs []shareddomain.ID) (map[shareddomain.ID]int, error)
}
