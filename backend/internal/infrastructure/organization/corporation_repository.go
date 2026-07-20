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

type CorporationRepository struct {
	db *gorm.DB
}

func NewCorporationRepository(db *gorm.DB) *CorporationRepository {
	return &CorporationRepository{db: db}
}

func (r *CorporationRepository) Create(ctx context.Context, corporation *orgdomain.Corporation) error {
	model := CorporationModel{
		ID:   uuid.UUID(corporation.ID()),
		Name: corporation.Name(),
	}
	return r.db.WithContext(ctx).Create(&model).Error
}

func (r *CorporationRepository) FindByID(ctx context.Context, id shareddomain.ID) (*orgdomain.Corporation, error) {
	var model CorporationModel
	if err := r.db.WithContext(ctx).First(&model, "id = ?", uuid.UUID(id)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("法人が見つかりません: %w", shareddomain.ErrNotFound)
		}
		return nil, err
	}
	return orgdomain.ReconstructCorporation(shareddomain.ID(model.ID), model.Name), nil
}
