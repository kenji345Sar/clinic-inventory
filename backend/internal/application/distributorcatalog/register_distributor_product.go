package distributorcatalog

import (
	"context"
	"fmt"

	distdomain "clinic-inventory/internal/domain/distributorcatalog"
	shareddomain "clinic-inventory/internal/domain/shared"
)

type RegisterDistributorProductUseCase struct {
	productRepo distdomain.DistributorProductRepository
}

func NewRegisterDistributorProductUseCase(productRepo distdomain.DistributorProductRepository) *RegisterDistributorProductUseCase {
	return &RegisterDistributorProductUseCase{productRepo: productRepo}
}

type RegisterDistributorProductInput struct {
	DistributorID          shareddomain.ID
	DistributorProductCode string
	Name                   string
	VendorName             string
	VendorProductCode      string // 任意
	JANCode                string // 任意
	UnitPrice              int    // 標準単価（税抜・円・必須）
}

func (uc *RegisterDistributorProductUseCase) Execute(ctx context.Context, in RegisterDistributorProductInput) (*distdomain.DistributorProduct, error) {
	// 同一卸業者内での卸商品コードの一意性チェック。
	// 同時登録の競合はDB側のユニーク制約が最終防衛線となる（domain-rules.md「卸連携コンテキスト」）。
	exists, err := uc.productRepo.ExistsByDistributorAndCode(ctx, in.DistributorID, in.DistributorProductCode)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("卸商品コード %s はこの卸業者で既に使われています: %w", in.DistributorProductCode, shareddomain.ErrConflict)
	}

	product, err := distdomain.NewDistributorProduct(in.DistributorID, in.DistributorProductCode, in.Name, in.VendorName, in.UnitPrice)
	if err != nil {
		return nil, err
	}
	if in.VendorProductCode != "" {
		product.SetVendorProductCode(in.VendorProductCode)
	}
	if in.JANCode != "" {
		product.SetJANCode(in.JANCode)
	}

	if err := uc.productRepo.Create(ctx, product); err != nil {
		return nil, err
	}
	return product, nil
}
