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
	distributor, err := distdomain.NewDistributor(name)
	if err != nil {
		return nil, err
	}
	if err := uc.distributorRepo.Create(ctx, distributor); err != nil {
		return nil, err
	}
	return distributor, nil
}
