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
	// 手順1: 法人を組み立てる（名前の検証はドメイン側）。
	corporation, err := orgdomain.NewCorporation(name)
	if err != nil {
		return nil, err
	}
	// 手順2: DBに保存する。
	if err := uc.corporationRepo.Create(ctx, corporation); err != nil {
		return nil, err
	}
	return corporation, nil
}
