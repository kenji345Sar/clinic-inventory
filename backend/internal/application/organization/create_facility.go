package organization

import (
	"context"

	orgdomain "clinic-inventory/internal/domain/organization"
	shareddomain "clinic-inventory/internal/domain/shared"
)

type CreateFacilityUseCase struct {
	facilityRepo orgdomain.FacilityRepository
}

func NewCreateFacilityUseCase(facilityRepo orgdomain.FacilityRepository) *CreateFacilityUseCase {
	return &CreateFacilityUseCase{facilityRepo: facilityRepo}
}

type CreateFacilityInput struct {
	Name          string
	FacilityType  orgdomain.FacilityType
	CorporationID shareddomain.ID
}

func (uc *CreateFacilityUseCase) Execute(ctx context.Context, in CreateFacilityInput) (*orgdomain.Facility, error) {
	facility, err := orgdomain.NewFacility(in.Name, in.FacilityType, in.CorporationID)
	if err != nil {
		return nil, err
	}
	if err := uc.facilityRepo.Create(ctx, facility); err != nil {
		return nil, err
	}
	return facility, nil
}
