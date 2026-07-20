package procurement

import (
	"context"
	"fmt"

	distdomain "clinic-inventory/internal/domain/distributorcatalog"
	procdomain "clinic-inventory/internal/domain/procurement"
	proddomain "clinic-inventory/internal/domain/productcatalog"
	shareddomain "clinic-inventory/internal/domain/shared"
)

// CreatePurchaseOrderUseCase はクリニックが特定の卸に対して発注を作成・確定する。
//
// 「1発注 = 1卸」というルールを実体レベルで担保するのがこのユースケースの主目的。
// 集約(PurchaseOrder)は数量や明細有無といった自己完結する不変条件だけを守り、
// 「その明細のクリニック商品が本当にこの卸の商品か」という他集約にまたがる検証はここで行う
// (RegisterClinicProductUseCase が卸商品の存在確認をユースケースで行うのと同じ切り分け)。
type CreatePurchaseOrderUseCase struct {
	purchaseOrderRepo      procdomain.PurchaseOrderRepository
	distributorRepo        distdomain.DistributorRepository
	distributorProductRepo distdomain.DistributorProductRepository
	clinicProductRepo      proddomain.ClinicProductRepository
}

func NewCreatePurchaseOrderUseCase(
	purchaseOrderRepo procdomain.PurchaseOrderRepository,
	distributorRepo distdomain.DistributorRepository,
	distributorProductRepo distdomain.DistributorProductRepository,
	clinicProductRepo proddomain.ClinicProductRepository,
) *CreatePurchaseOrderUseCase {
	return &CreatePurchaseOrderUseCase{
		purchaseOrderRepo:      purchaseOrderRepo,
		distributorRepo:        distributorRepo,
		distributorProductRepo: distributorProductRepo,
		clinicProductRepo:      clinicProductRepo,
	}
}

type CreatePurchaseOrderLineInput struct {
	ClinicProductID shareddomain.ID
	Quantity        int
}

type CreatePurchaseOrderInput struct {
	FacilityID    shareddomain.ID
	DistributorID shareddomain.ID
	Lines         []CreatePurchaseOrderLineInput
}

func (uc *CreatePurchaseOrderUseCase) Execute(ctx context.Context, in CreatePurchaseOrderInput) (*procdomain.PurchaseOrder, error) {
	// 発注先の卸業者が存在するか。
	if _, err := uc.distributorRepo.FindByID(ctx, in.DistributorID); err != nil {
		return nil, fmt.Errorf("発注先の卸業者が見つかりません: %w", err)
	}

	order, err := procdomain.NewPurchaseOrder(in.FacilityID, in.DistributorID)
	if err != nil {
		return nil, err
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

		if err := order.AddLine(line.ClinicProductID, line.Quantity); err != nil {
			return nil, err
		}
	}

	// 明細0件ならここでエラーになる。1ステップ作成方針のため作成と同時に確定する。
	if err := order.Confirm(); err != nil {
		return nil, err
	}

	if err := uc.purchaseOrderRepo.Create(ctx, order); err != nil {
		return nil, err
	}
	return order, nil
}
