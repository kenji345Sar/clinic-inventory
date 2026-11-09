package procurement

import (
	"context"
	"fmt"

	distdomain "clinic-inventory/internal/domain/distributorcatalog"
	orgdomain "clinic-inventory/internal/domain/organization"
	procdomain "clinic-inventory/internal/domain/procurement"
	shareddomain "clinic-inventory/internal/domain/shared"
)

// SendOrderConfirmedNotificationUseCase は確定済みの発注について、医院と卸にお知らせを送る。
// 通知ワーカー(cmd/notifyworker)がSQSから発注IDを受け取って呼ぶ。
//
// ConfirmPurchaseOrderUseCase と違い、失敗したらエラーを返す。ワーカーはエラーならメッセージを消さず、
// SQSが時間をおいて渡し直す（＝自動再試行）。
type SendOrderConfirmedNotificationUseCase struct {
	purchaseOrderRepo procdomain.PurchaseOrderRepository
	distributorRepo   distdomain.DistributorRepository
	facilityRepo      orgdomain.FacilityRepository
	notifier          PurchaseOrderNotifier
}

func NewSendOrderConfirmedNotificationUseCase(
	purchaseOrderRepo procdomain.PurchaseOrderRepository,
	distributorRepo distdomain.DistributorRepository,
	facilityRepo orgdomain.FacilityRepository,
	notifier PurchaseOrderNotifier,
) *SendOrderConfirmedNotificationUseCase {
	return &SendOrderConfirmedNotificationUseCase{
		purchaseOrderRepo: purchaseOrderRepo,
		distributorRepo:   distributorRepo,
		facilityRepo:      facilityRepo,
		notifier:          notifier,
	}
}

func (uc *SendOrderConfirmedNotificationUseCase) Execute(ctx context.Context, orderID shareddomain.ID) error {
	// 手順1: 発注を取得し、確定済みであることを確認する。
	// 下書きのお知らせを送ってしまうと、まだ卸に届いていない発注を「確定しました」と伝えることになる。
	order, err := uc.purchaseOrderRepo.FindByID(ctx, orderID)
	if err != nil {
		return err
	}
	if order.Status() != procdomain.StatusConfirmed {
		return fmt.Errorf("発注 %s は確定していません(status=%s)", orderID.String(), order.Status())
	}

	// 手順2: 宛先と本文に使う卸・医院を取得する。
	distributor, err := uc.distributorRepo.FindByID(ctx, order.DistributorID())
	if err != nil {
		return fmt.Errorf("発注先の卸業者が見つかりません: %w", err)
	}
	facility, err := uc.facilityRepo.FindByID(ctx, order.FacilityID())
	if err != nil {
		return fmt.Errorf("発注元クリニックが見つかりません: %w", err)
	}

	// 手順3: 送る。
	return uc.notifier.NotifyConfirmed(ctx, order, distributor, facility)
}
