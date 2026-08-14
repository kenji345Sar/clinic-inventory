package procurement

import (
	"context"
	"fmt"
	"time"

	distdomain "clinic-inventory/internal/domain/distributorcatalog"
	orgdomain "clinic-inventory/internal/domain/organization"
	procdomain "clinic-inventory/internal/domain/procurement"
	proddomain "clinic-inventory/internal/domain/productcatalog"
	shareddomain "clinic-inventory/internal/domain/shared"
)

// ConfirmPurchaseOrderUseCase はカート（下書き）に積んだ発注を確定し、卸へ発注CSVを送る。
//
// 「発注CSVが卸に届いて初めて確定と言える」というルールのため、CSVアップロードが失敗したら
// 確定状態を永続化しない（docs/go/request-to-sql-flow.md §10）。
type ConfirmPurchaseOrderUseCase struct {
	purchaseOrderRepo      procdomain.PurchaseOrderRepository
	distributorProductRepo distdomain.DistributorProductRepository
	clinicProductRepo      proddomain.ClinicProductRepository
	facilityRepo           orgdomain.FacilityRepository
	csvUploader            PurchaseOrderCsvUploader
}

func NewConfirmPurchaseOrderUseCase(
	purchaseOrderRepo procdomain.PurchaseOrderRepository,
	distributorProductRepo distdomain.DistributorProductRepository,
	clinicProductRepo proddomain.ClinicProductRepository,
	facilityRepo orgdomain.FacilityRepository,
	csvUploader PurchaseOrderCsvUploader,
) *ConfirmPurchaseOrderUseCase {
	return &ConfirmPurchaseOrderUseCase{
		purchaseOrderRepo:      purchaseOrderRepo,
		distributorProductRepo: distributorProductRepo,
		clinicProductRepo:      clinicProductRepo,
		facilityRepo:           facilityRepo,
		csvUploader:            csvUploader,
	}
}

type ConfirmPurchaseOrderInput struct {
	FacilityID shareddomain.ID
	OrderID    shareddomain.ID
}

func (uc *ConfirmPurchaseOrderUseCase) Execute(ctx context.Context, in ConfirmPurchaseOrderInput) (*procdomain.PurchaseOrder, error) {
	// 手順1: 発注を取得し、URLのfacilityIdと発注の実際のfacilityIdが一致するか確認する。
	// 他クリニックの発注IDを自分のfacilityId配下から直接叩いても操作できないようにする。
	order, err := uc.purchaseOrderRepo.FindByID(ctx, in.OrderID)
	if err != nil {
		return nil, err
	}
	if order.FacilityID() != in.FacilityID {
		return nil, fmt.Errorf("発注が見つかりません: %w", shareddomain.ErrNotFound)
	}

	// 手順2: 発注CSVの明細を組み立てる。卸に送る書類なので、クリニック側の呼び方ではなく
	// 解決済みのDistributorProduct側の値（卸商品コード・卸側の商品名）を使う。
	csvLines := make([]PurchaseOrderCsvLine, 0, len(order.Lines()))
	for _, line := range order.Lines() {
		clinicProduct, err := uc.clinicProductRepo.FindByID(ctx, line.ClinicProductID())
		if err != nil {
			return nil, fmt.Errorf("発注対象のクリニック商品が見つかりません: %w", err)
		}
		distributorProduct, err := uc.distributorProductRepo.FindByID(ctx, clinicProduct.DistributorProductID())
		if err != nil {
			return nil, fmt.Errorf("クリニック商品 %s の紐付け先卸商品が見つかりません: %w", clinicProduct.ProductCode(), err)
		}
		csvLines = append(csvLines, PurchaseOrderCsvLine{
			DistributorProductCode: distributorProduct.DistributorProductCode(),
			ProductName:            distributorProduct.Name(),
			Quantity:               line.Quantity(),
			UnitPrice:              line.UnitPrice(),
		})
	}

	// 手順3: 発注を確定状態にする（明細0件・確定済みならここでエラー）。
	// CSVの「発注日」とDBに永続化する確定日時を同じ値で揃えるため、時刻は1つだけ作って使い回す。
	confirmedAt := time.Now()
	if err := order.Confirm(confirmedAt); err != nil {
		return nil, err
	}

	// 手順4: 発注CSVを卸へアップロードする（CSVに発注元クリニック名を載せるため施設も取得）。
	// 発注CSVが卸に届いて初めて「発注確定」と言えるため、CSVアップロードが失敗したら
	// ここで打ち切り、確定状態は永続化しない（確定したのに卸に届いていない、という不整合を避ける）。
	facility, err := uc.facilityRepo.FindByID(ctx, in.FacilityID)
	if err != nil {
		return nil, fmt.Errorf("発注元クリニックが見つかりません: %w", err)
	}
	if err := uc.csvUploader.Upload(ctx, order, facility.Name(), confirmedAt, csvLines); err != nil {
		return nil, fmt.Errorf("発注CSVのアップロードに失敗しました: %w", err)
	}

	// 手順5: アップロード成功後にようやく確定状態をDBに保存する。
	if err := uc.purchaseOrderRepo.Update(ctx, order); err != nil {
		return nil, err
	}
	return order, nil
}
