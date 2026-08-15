package distributorcatalog

import (
	"context"
	"errors"
	"fmt"

	distdomain "clinic-inventory/internal/domain/distributorcatalog"
	shareddomain "clinic-inventory/internal/domain/shared"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DistributorRepository struct {
	db *gorm.DB
}

func NewDistributorRepository(db *gorm.DB) *DistributorRepository {
	return &DistributorRepository{db: db}
}

func (r *DistributorRepository) Create(ctx context.Context, distributor *distdomain.Distributor) error {
	model := DistributorModel{
		ID:   uuid.UUID(distributor.ID()),
		Code: distributor.Code(),
		Name: distributor.Name(),
	}
	return r.db.WithContext(ctx).Create(&model).Error
}

func (r *DistributorRepository) FindByID(ctx context.Context, id shareddomain.ID) (*distdomain.Distributor, error) {
	var model DistributorModel
	if err := r.db.WithContext(ctx).First(&model, "id = ?", uuid.UUID(id)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("卸業者が見つかりません: %w", shareddomain.ErrNotFound)
		}
		return nil, err
	}
	return distdomain.ReconstructDistributor(shareddomain.ID(model.ID), model.Code, model.Name), nil
}

func (r *DistributorRepository) ExistsByCode(ctx context.Context, code string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&DistributorModel{}).
		Where("code = ?", code).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *DistributorRepository) FindAll(ctx context.Context) ([]*distdomain.Distributor, error) {
	var models []DistributorModel
	if err := r.db.WithContext(ctx).Find(&models).Error; err != nil {
		return nil, err
	}
	distributors := make([]*distdomain.Distributor, 0, len(models))
	for _, model := range models {
		distributors = append(distributors, distdomain.ReconstructDistributor(shareddomain.ID(model.ID), model.Code, model.Name))
	}
	return distributors, nil
}
