package productcatalog

import (
	"context"
	"fmt"

	distdomain "clinic-inventory/internal/domain/distributorcatalog"
	proddomain "clinic-inventory/internal/domain/productcatalog"
	shareddomain "clinic-inventory/internal/domain/shared"
)

// RegisterClinicProductUseCase は卸商品を元にクリニック商品を登録する。
// 「卸商品への紐付けが必須」というルールを実体レベルでも担保するため、
// 卸連携コンテキストのリポジトリで紐付け先の存在を確認する。
type RegisterClinicProductUseCase struct {
	clinicProductRepo      proddomain.ClinicProductRepository
	distributorProductRepo distdomain.DistributorProductRepository
	facilityPriceRepo      distdomain.FacilityPriceRepository
}

func NewRegisterClinicProductUseCase(
	clinicProductRepo proddomain.ClinicProductRepository,
	distributorProductRepo distdomain.DistributorProductRepository,
	facilityPriceRepo distdomain.FacilityPriceRepository,
) *RegisterClinicProductUseCase {
	return &RegisterClinicProductUseCase{
		clinicProductRepo:      clinicProductRepo,
		distributorProductRepo: distributorProductRepo,
		facilityPriceRepo:      facilityPriceRepo,
	}
}

type RegisterClinicProductInput struct {
	FacilityID           shareddomain.ID
	ProductCode          string
	Name                 string // 空なら卸商品の商品名を引き継ぐ
	DistributorProductID shareddomain.ID
	JANCode              string // 空なら卸商品のJANを引き継ぐ
	UnitPrice            int    // 0なら卸商品の標準単価を引き継ぐ。指定があれば医院別単価として上書き
	ReorderPoint         int
}

func (uc *RegisterClinicProductUseCase) Execute(ctx context.Context, in RegisterClinicProductInput) (*proddomain.ClinicProduct, error) {
	// 手順1: 紐付け先の卸商品が実在し、廃盤でないことを確認する。
	distributorProduct, err := uc.distributorProductRepo.FindByID(ctx, in.DistributorProductID)
	if err != nil {
		return nil, fmt.Errorf("紐付け先の卸商品が見つかりません: %w", err)
	}
	if distributorProduct.Discontinued() {
		return nil, fmt.Errorf("卸商品 %s は廃盤のため登録できません", distributorProduct.Name())
	}

	// 手順2: 同一クリニック内での商品コードの一意性チェック。
	// 同時登録の競合はDB側のユニーク制約が最終防衛線となる。
	exists, err := uc.clinicProductRepo.ExistsByFacilityAndCode(ctx, in.FacilityID, in.ProductCode)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("商品コード %s はこのクリニックで既に使われています: %w", in.ProductCode, shareddomain.ErrConflict)
	}

	// 手順3: 未指定の項目（商品名・単価・JAN）は卸商品の値を継承してクリニック商品を組み立てる。
	name := in.Name
	if name == "" {
		name = distributorProduct.Name()
	}

	// 単価は指定があればその値。未指定(0)なら「卸が決めたこのクリニック向けの単価
	// → 卸の標準単価」の順で継承する（clinic-inventory-csv-functions/docs/distributor-catalog-import.md 2章）。
	// どれも無い（単価が分からない卸）場合は0のまま登録する。単価は後日、卸から届く
	// 受注結果の単価で更新する運用のため、ここで登録を止めない。
	unitPrice := in.UnitPrice
	if unitPrice == 0 {
		facilityPrice, err := uc.facilityPriceRepo.FindByProductAndFacility(ctx, in.DistributorProductID, in.FacilityID)
		if err != nil {
			return nil, err
		}
		switch {
		case facilityPrice != nil:
			unitPrice = facilityPrice.UnitPrice()
		case distributorProduct.HasUnitPrice():
			unitPrice = *distributorProduct.UnitPrice()
		}
	}

	product, err := proddomain.NewClinicProduct(in.FacilityID, in.ProductCode, name, in.DistributorProductID, unitPrice, in.ReorderPoint)
	if err != nil {
		return nil, err
	}

	janCode := in.JANCode
	if janCode == "" {
		janCode = distributorProduct.JANCode()
	}
	if janCode != "" {
		product.SetJANCode(janCode)
	}

	// 手順4: DBに保存する。
	if err := uc.clinicProductRepo.Create(ctx, product); err != nil {
		return nil, err
	}
	return product, nil
}
