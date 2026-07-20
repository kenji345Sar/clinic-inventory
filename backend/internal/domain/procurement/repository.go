package procurement

import (
	"context"

	shareddomain "clinic-inventory/internal/domain/shared"
)

type PurchaseOrderRepository interface {
	// Create は発注(親)と明細(子)をまとめて永続化する。
	// 明細の原子性はリポジトリ実装側でトランザクションにより担保する。
	Create(ctx context.Context, order *PurchaseOrder) error
	FindByID(ctx context.Context, id shareddomain.ID) (*PurchaseOrder, error)
	FindByFacility(ctx context.Context, facilityID shareddomain.ID) ([]*PurchaseOrder, error)
}
