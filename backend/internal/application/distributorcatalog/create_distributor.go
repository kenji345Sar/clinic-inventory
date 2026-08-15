package distributorcatalog

import (
	"context"
	"fmt"

	distdomain "clinic-inventory/internal/domain/distributorcatalog"
	shareddomain "clinic-inventory/internal/domain/shared"
)

type CreateDistributorUseCase struct {
	distributorRepo distdomain.DistributorRepository
}

func NewCreateDistributorUseCase(distributorRepo distdomain.DistributorRepository) *CreateDistributorUseCase {
	return &CreateDistributorUseCase{distributorRepo: distributorRepo}
}

type CreateDistributorInput struct {
	// Code は卸コード。S3上のフォルダ名(orders/{code}/, catalogs/{code}/)になる。
	Code string
	Name string
}

func (uc *CreateDistributorUseCase) Execute(ctx context.Context, in CreateDistributorInput) (*distdomain.Distributor, error) {
	// 手順1: 卸コードの重複を確認する。S3のフォルダ名になるため、重複すると
	// 別の卸のCSVが同じフォルダに混ざる。DB側のユニーク制約が最終防衛線。
	exists, err := uc.distributorRepo.ExistsByCode(ctx, in.Code)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("卸コード %s は既に使われています: %w", in.Code, shareddomain.ErrConflict)
	}

	// 手順2: 卸業者を組み立てる（コード・名前の検証はドメイン側）。
	distributor, err := distdomain.NewDistributor(in.Code, in.Name)
	if err != nil {
		return nil, err
	}

	// 手順3: DBに保存する。
	if err := uc.distributorRepo.Create(ctx, distributor); err != nil {
		return nil, err
	}
	return distributor, nil
}
