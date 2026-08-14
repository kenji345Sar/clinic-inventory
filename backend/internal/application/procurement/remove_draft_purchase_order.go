package procurement

import (
	"context"
	"fmt"

	procdomain "clinic-inventory/internal/domain/procurement"
	shareddomain "clinic-inventory/internal/domain/shared"
)

// RemoveDraftPurchaseOrderUseCase はカート（下書き）に積んだ発注を確定前に取り消す。
// 確定済みの発注は不変（取消は逆仕訳で表現する方針）のため、下書きのみ削除できる。
type RemoveDraftPurchaseOrderUseCase struct {
	purchaseOrderRepo procdomain.PurchaseOrderRepository
}

func NewRemoveDraftPurchaseOrderUseCase(
	purchaseOrderRepo procdomain.PurchaseOrderRepository,
) *RemoveDraftPurchaseOrderUseCase {
	return &RemoveDraftPurchaseOrderUseCase{purchaseOrderRepo: purchaseOrderRepo}
}

type RemoveDraftPurchaseOrderInput struct {
	FacilityID shareddomain.ID
	OrderID    shareddomain.ID
}

func (uc *RemoveDraftPurchaseOrderUseCase) Execute(ctx context.Context, in RemoveDraftPurchaseOrderInput) error {
	// 手順1: 発注を取得し、URLのfacilityIdと発注の実際のfacilityIdが一致するか確認する
	// （他クリニックの発注を取り消せないようにする。ConfirmPurchaseOrderUseCaseと同じパターン）。
	order, err := uc.purchaseOrderRepo.FindByID(ctx, in.OrderID)
	if err != nil {
		return err
	}
	if order.FacilityID() != in.FacilityID {
		return fmt.Errorf("発注が見つかりません: %w", shareddomain.ErrNotFound)
	}

	// 手順2: 下書きであることを確認する（確定済みの発注は不変。取消は逆仕訳で表現する方針）。
	if order.Status() != procdomain.StatusDraft {
		return fmt.Errorf("確定済みの発注はカートから取り消せません: %w", shareddomain.ErrConflict)
	}

	// 手順3: 下書きを物理削除する。
	return uc.purchaseOrderRepo.Delete(ctx, in.OrderID)
}
