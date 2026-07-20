package organization

import (
	"context"
	"errors"
	"fmt"

	orgdomain "clinic-inventory/internal/domain/organization"
	shareddomain "clinic-inventory/internal/domain/shared"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type FacilityRepository struct {
	db *gorm.DB
}

func NewFacilityRepository(db *gorm.DB) *FacilityRepository {
	return &FacilityRepository{db: db}
}

func (r *FacilityRepository) Create(ctx context.Context, facility *orgdomain.Facility) error {
	model := toFacilityModel(facility)
	return r.db.WithContext(ctx).Create(&model).Error
}

func (r *FacilityRepository) FindByID(ctx context.Context, id shareddomain.ID) (*orgdomain.Facility, error) {
	var model FacilityModel
	if err := r.db.WithContext(ctx).First(&model, "id = ?", uuid.UUID(id)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("クリニックが見つかりません: %w", shareddomain.ErrNotFound)
		}
		return nil, err
	}
	return toDomainFacility(model), nil
}

func (r *FacilityRepository) FindAll(ctx context.Context) ([]*orgdomain.Facility, error) {
	var models []FacilityModel
	if err := r.db.WithContext(ctx).Find(&models).Error; err != nil {
		return nil, err
	}
	facilities := make([]*orgdomain.Facility, 0, len(models))
	for _, model := range models {
		facilities = append(facilities, toDomainFacility(model))
	}
	return facilities, nil
}

func toFacilityModel(f *orgdomain.Facility) FacilityModel {
	var groupID *uuid.UUID
	if f.GroupID() != nil {
		id := uuid.UUID(*f.GroupID())
		groupID = &id
	}
	return FacilityModel{
		ID:            uuid.UUID(f.ID()),
		Name:          f.Name(),
		FacilityType:  string(f.Type()),
		CorporationID: uuid.UUID(f.CorporationID()),
		GroupID:       groupID,
	}
}

func toDomainFacility(model FacilityModel) *orgdomain.Facility {
	var groupID *shareddomain.ID
	if model.GroupID != nil {
		id := shareddomain.ID(*model.GroupID)
		groupID = &id
	}
	return orgdomain.ReconstructFacility(
		shareddomain.ID(model.ID),
		model.Name,
		orgdomain.FacilityType(model.FacilityType),
		shareddomain.ID(model.CorporationID),
		groupID,
	)
}
