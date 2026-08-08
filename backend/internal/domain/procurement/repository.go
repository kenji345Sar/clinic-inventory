package procurement

import (
	"context"

	shareddomain "clinic-inventory/internal/domain/shared"
)

type PurchaseOrderRepository interface {
	// Create は発注(親)と明細(子)をまとめて永続化する。
	// 明細の原子性はリポジトリ実装側でトランザクションにより担保する。
	Create(ctx context.Context, order *PurchaseOrder) error
	// Update は発注の状態と明細を丸ごと差し替える。
	// カートへの明細追加（下書き更新）と、確定（下書き→確定への状態更新）の両方で使う。
	Update(ctx context.Context, order *PurchaseOrder) error
	// Delete は下書きの発注をカートから取り消す（明細ごと削除）。
	Delete(ctx context.Context, id shareddomain.ID) error
	FindByID(ctx context.Context, id shareddomain.ID) (*PurchaseOrder, error)
	FindByFacility(ctx context.Context, facilityID shareddomain.ID) ([]*PurchaseOrder, error)
	// FindByDistributor は卸業者宛の確定済み発注を全て返す（卸ポータルの受注一覧で使う）。
	// 下書き（カートの中身）は卸側にはまだ見せない。
	FindByDistributor(ctx context.Context, distributorID shareddomain.ID) ([]*PurchaseOrder, error)
	// FindDraftByFacilityAndDistributor はカート追加時に「既存の下書きに合算するか、新規作成するか」を
	// 判定するために使う。見つからなければ shareddomain.ErrNotFound を返す。
	FindDraftByFacilityAndDistributor(ctx context.Context, facilityID, distributorID shareddomain.ID) (*PurchaseOrder, error)
}
