package organization

import (
	"context"
	"fmt"

	orgdomain "clinic-inventory/internal/domain/organization"
	shareddomain "clinic-inventory/internal/domain/shared"
)

type CreateFacilityUseCase struct {
	facilityRepo    orgdomain.FacilityRepository
	corporationRepo orgdomain.CorporationRepository
}

func NewCreateFacilityUseCase(facilityRepo orgdomain.FacilityRepository, corporationRepo orgdomain.CorporationRepository) *CreateFacilityUseCase {
	return &CreateFacilityUseCase{facilityRepo: facilityRepo, corporationRepo: corporationRepo}
}

type CreateFacilityInput struct {
	Name          string
	FacilityType  orgdomain.FacilityType
	CorporationID shareddomain.ID
}

func (uc *CreateFacilityUseCase) Execute(ctx context.Context, in CreateFacilityInput) (*orgdomain.Facility, error) {
	// クリニックは必ずいずれかの法人に属する(単体クリニックも「一人法人」として法人を持つ。
	// docs/architecture/domain-rules.md「組織コンテキスト」参照)。その法人IDが実在するかを
	// ここで検証する(CreatePurchaseOrderUseCaseが卸業者の実在確認をするのと同じパターン)。
	if _, err := uc.corporationRepo.FindByID(ctx, in.CorporationID); err != nil {
		return nil, fmt.Errorf("指定された法人が見つかりません: %w", err)
	}

	facility, err := orgdomain.NewFacility(in.Name, in.FacilityType, in.CorporationID)
	if err != nil {
		return nil, err
	}
	if err := uc.facilityRepo.Create(ctx, facility); err != nil {
		return nil, err
	}
	return facility, nil
}
