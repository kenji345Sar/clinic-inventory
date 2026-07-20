package organization

import (
	"context"

	orgdomain "clinic-inventory/internal/domain/organization"
)

type CreateCorporationUseCase struct {
	corporationRepo orgdomain.CorporationRepository
}

func NewCreateCorporationUseCase(corporationRepo orgdomain.CorporationRepository) *CreateCorporationUseCase {
	return &CreateCorporationUseCase{corporationRepo: corporationRepo}
}

func (uc *CreateCorporationUseCase) Execute(ctx context.Context, name string) (*orgdomain.Corporation, error) {
	corporation, err := orgdomain.NewCorporation(name)
	if err != nil {
		return nil, err
	}
	if err := uc.corporationRepo.Create(ctx, corporation); err != nil {
		return nil, err
	}
	return corporation, nil
}
