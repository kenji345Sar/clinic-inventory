package distributorcatalog

import (
	"context"

	distdomain "clinic-inventory/internal/domain/distributorcatalog"
)

type CreateDistributorUseCase struct {
	distributorRepo distdomain.DistributorRepository
}

func NewCreateDistributorUseCase(distributorRepo distdomain.DistributorRepository) *CreateDistributorUseCase {
	return &CreateDistributorUseCase{distributorRepo: distributorRepo}
}

func (uc *CreateDistributorUseCase) Execute(ctx context.Context, name string) (*distdomain.Distributor, error) {
	// 手順1: 卸業者を組み立てる（名前の検証はドメイン側）。
	distributor, err := distdomain.NewDistributor(name)
	if err != nil {
		return nil, err
	}
	// 手順2: DBに保存する。
	if err := uc.distributorRepo.Create(ctx, distributor); err != nil {
		return nil, err
	}
	return distributor, nil
}
