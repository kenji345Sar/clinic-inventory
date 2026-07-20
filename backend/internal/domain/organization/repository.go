package organization

import (
	"context"

	shareddomain "clinic-inventory/internal/domain/shared"
)

type CorporationRepository interface {
	Create(ctx context.Context, corporation *Corporation) error
	FindByID(ctx context.Context, id shareddomain.ID) (*Corporation, error)
}

type FacilityRepository interface {
	Create(ctx context.Context, facility *Facility) error
	FindByID(ctx context.Context, id shareddomain.ID) (*Facility, error)
	FindAll(ctx context.Context) ([]*Facility, error)
}
