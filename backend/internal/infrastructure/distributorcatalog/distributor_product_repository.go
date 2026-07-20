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

type DistributorProductRepository struct {
	db *gorm.DB
}

func NewDistributorProductRepository(db *gorm.DB) *DistributorProductRepository {
	return &DistributorProductRepository{db: db}
}

func (r *DistributorProductRepository) Create(ctx context.Context, product *distdomain.DistributorProduct) error {
	model := toDistributorProductModel(product)
	return r.db.WithContext(ctx).Create(&model).Error
}

func (r *DistributorProductRepository) FindByID(ctx context.Context, id shareddomain.ID) (*distdomain.DistributorProduct, error) {
	var model DistributorProductModel
	if err := r.db.WithContext(ctx).First(&model, "id = ?", uuid.UUID(id)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("卸商品が見つかりません: %w", shareddomain.ErrNotFound)
		}
		return nil, err
	}
	return toDomainDistributorProduct(model), nil
}

func (r *DistributorProductRepository) ExistsByDistributorAndCode(ctx context.Context, distributorID shareddomain.ID, code string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&DistributorProductModel{}).
		Where("distributor_id = ? AND distributor_product_code = ?", uuid.UUID(distributorID), code).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *DistributorProductRepository) FindByDistributor(ctx context.Context, distributorID shareddomain.ID) ([]*distdomain.DistributorProduct, error) {
	var models []DistributorProductModel
	err := r.db.WithContext(ctx).
		Where("distributor_id = ?", uuid.UUID(distributorID)).
		Order("distributor_product_code").
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	products := make([]*distdomain.DistributorProduct, 0, len(models))
	for _, model := range models {
		products = append(products, toDomainDistributorProduct(model))
	}
	return products, nil
}

func toDistributorProductModel(p *distdomain.DistributorProduct) DistributorProductModel {
	return DistributorProductModel{
		ID:                     uuid.UUID(p.ID()),
		DistributorID:          uuid.UUID(p.DistributorID()),
		DistributorProductCode: p.DistributorProductCode(),
		Name:                   p.Name(),
		VendorName:             p.VendorName(),
		VendorProductCode:      p.VendorProductCode(),
		JANCode:                p.JANCode(),
		Discontinued:           p.Discontinued(),
	}
}

func toDomainDistributorProduct(model DistributorProductModel) *distdomain.DistributorProduct {
	return distdomain.ReconstructDistributorProduct(
		shareddomain.ID(model.ID),
		shareddomain.ID(model.DistributorID),
		model.DistributorProductCode,
		model.Name,
		model.VendorName,
		model.VendorProductCode,
		model.JANCode,
		model.Discontinued,
	)
}
