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
}

func NewRegisterClinicProductUseCase(
	clinicProductRepo proddomain.ClinicProductRepository,
	distributorProductRepo distdomain.DistributorProductRepository,
) *RegisterClinicProductUseCase {
	return &RegisterClinicProductUseCase{
		clinicProductRepo:      clinicProductRepo,
		distributorProductRepo: distributorProductRepo,
	}
}

type RegisterClinicProductInput struct {
	FacilityID           shareddomain.ID
	ProductCode          string
	Name                 string // 空なら卸商品の商品名を引き継ぐ
	DistributorProductID shareddomain.ID
	JANCode              string // 空なら卸商品のJANを引き継ぐ
	ReorderPoint         int
}

func (uc *RegisterClinicProductUseCase) Execute(ctx context.Context, in RegisterClinicProductInput) (*proddomain.ClinicProduct, error) {
	distributorProduct, err := uc.distributorProductRepo.FindByID(ctx, in.DistributorProductID)
	if err != nil {
		return nil, fmt.Errorf("紐付け先の卸商品が見つかりません: %w", err)
	}
	if distributorProduct.Discontinued() {
		return nil, fmt.Errorf("卸商品 %s は廃盤のため登録できません", distributorProduct.Name())
	}

	// 同一クリニック内での商品コードの一意性チェック。
	// 同時登録の競合はDB側のユニーク制約が最終防衛線となる。
	exists, err := uc.clinicProductRepo.ExistsByFacilityAndCode(ctx, in.FacilityID, in.ProductCode)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("商品コード %s はこのクリニックで既に使われています: %w", in.ProductCode, shareddomain.ErrConflict)
	}

	name := in.Name
	if name == "" {
		name = distributorProduct.Name()
	}

	product, err := proddomain.NewClinicProduct(in.FacilityID, in.ProductCode, name, in.DistributorProductID, in.ReorderPoint)
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

	if err := uc.clinicProductRepo.Create(ctx, product); err != nil {
		return nil, err
	}
	return product, nil
}
