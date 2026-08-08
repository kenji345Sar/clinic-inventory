package procurement

import (
	"context"
	"errors"
	"fmt"

	distdomain "clinic-inventory/internal/domain/distributorcatalog"
	procdomain "clinic-inventory/internal/domain/procurement"
	proddomain "clinic-inventory/internal/domain/productcatalog"
	shareddomain "clinic-inventory/internal/domain/shared"
)

// SaveDraftPurchaseOrderUseCase はクリニックが特定の卸に対する発注をカート（下書き）に積む。
//
// 「1発注 = 1卸」というルールを実体レベルで担保するのがこのユースケースの主目的。
// 集約(PurchaseOrder)は数量や明細有無といった自己完結する不変条件だけを守り、
// 「その明細のクリニック商品が本当にこの卸の商品か」という他集約にまたがる検証はここで行う
// (RegisterClinicProductUseCase が卸商品の存在確認をユースケースで行うのと同じ切り分け)。
//
// 同じ卸への追加は、既存の下書きがあればそこに明細を合算する（AddLine が同一クリニック商品の
// 数量を自動加算する）。下書きの確定・卸へのCSV送付は ConfirmPurchaseOrderUseCase の責務。
type SaveDraftPurchaseOrderUseCase struct {
	purchaseOrderRepo      procdomain.PurchaseOrderRepository
	distributorRepo        distdomain.DistributorRepository
	distributorProductRepo distdomain.DistributorProductRepository
	clinicProductRepo      proddomain.ClinicProductRepository
}

func NewSaveDraftPurchaseOrderUseCase(
	purchaseOrderRepo procdomain.PurchaseOrderRepository,
	distributorRepo distdomain.DistributorRepository,
	distributorProductRepo distdomain.DistributorProductRepository,
	clinicProductRepo proddomain.ClinicProductRepository,
) *SaveDraftPurchaseOrderUseCase {
	return &SaveDraftPurchaseOrderUseCase{
		purchaseOrderRepo:      purchaseOrderRepo,
		distributorRepo:        distributorRepo,
		distributorProductRepo: distributorProductRepo,
		clinicProductRepo:      clinicProductRepo,
	}
}

type SaveDraftPurchaseOrderLineInput struct {
	ClinicProductID shareddomain.ID
	Quantity        int
}

type SaveDraftPurchaseOrderInput struct {
	FacilityID    shareddomain.ID
	DistributorID shareddomain.ID
	Lines         []SaveDraftPurchaseOrderLineInput
}

func (uc *SaveDraftPurchaseOrderUseCase) Execute(ctx context.Context, in SaveDraftPurchaseOrderInput) (*procdomain.PurchaseOrder, error) {
	// 発注先の卸業者が存在するか。
	if _, err := uc.distributorRepo.FindByID(ctx, in.DistributorID); err != nil {
		return nil, fmt.Errorf("発注先の卸業者が見つかりません: %w", err)
	}

	// 同じクリニック・卸の下書きが既にあればそれに合算する。無ければ新規の下書きを作る。
	order, err := uc.purchaseOrderRepo.FindDraftByFacilityAndDistributor(ctx, in.FacilityID, in.DistributorID)
	isNewDraft := false
	if err != nil {
		if !errors.Is(err, shareddomain.ErrNotFound) {
			return nil, err
		}
		order, err = procdomain.NewPurchaseOrder(in.FacilityID, in.DistributorID)
		if err != nil {
			return nil, err
		}
		isNewDraft = true
	}

	for _, line := range in.Lines {
		clinicProduct, err := uc.clinicProductRepo.FindByID(ctx, line.ClinicProductID)
		if err != nil {
			return nil, fmt.Errorf("発注対象のクリニック商品が見つかりません: %w", err)
		}

		// 他クリニックの商品を混ぜて発注できないようにする。
		if clinicProduct.FacilityID() != in.FacilityID {
			return nil, fmt.Errorf("クリニック商品 %s は発注先クリニックの商品ではありません: %w", clinicProduct.ProductCode(), shareddomain.ErrConflict)
		}

		// クリニック商品→卸商品→卸業者 とたどり、発注の卸業者と一致するか検証する。
		// これが「1発注 = 1卸」の実体レベルの担保。
		distributorProduct, err := uc.distributorProductRepo.FindByID(ctx, clinicProduct.DistributorProductID())
		if err != nil {
			return nil, fmt.Errorf("クリニック商品 %s の紐付け先卸商品が見つかりません: %w", clinicProduct.ProductCode(), err)
		}
		if distributorProduct.DistributorID() != in.DistributorID {
			return nil, fmt.Errorf("クリニック商品 %s は別の卸の商品のため、この発注には含められません（1発注=1卸）: %w", clinicProduct.ProductCode(), shareddomain.ErrConflict)
		}

		// 発注時点のクリニック商品単価を明細にスナップショットして固定する。
		if err := order.AddLine(line.ClinicProductID, line.Quantity, clinicProduct.UnitPrice()); err != nil {
			return nil, err
		}
	}

	if isNewDraft {
		if err := uc.purchaseOrderRepo.Create(ctx, order); err != nil {
			return nil, err
		}
	} else {
		if err := uc.purchaseOrderRepo.Update(ctx, order); err != nil {
			return nil, err
		}
	}
	return order, nil
}
