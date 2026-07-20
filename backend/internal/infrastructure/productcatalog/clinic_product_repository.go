package productcatalog

import (
	"context"
	"errors"
	"fmt"

	proddomain "clinic-inventory/internal/domain/productcatalog"
	shareddomain "clinic-inventory/internal/domain/shared"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ClinicProductRepository struct {
	db *gorm.DB
}

func NewClinicProductRepository(db *gorm.DB) *ClinicProductRepository {
	return &ClinicProductRepository{db: db}
}

func (r *ClinicProductRepository) Create(ctx context.Context, product *proddomain.ClinicProduct) error {
	model := toClinicProductModel(product)
	return r.db.WithContext(ctx).Create(&model).Error
}

func (r *ClinicProductRepository) FindByID(ctx context.Context, id shareddomain.ID) (*proddomain.ClinicProduct, error) {
	var model ClinicProductModel
	if err := r.db.WithContext(ctx).First(&model, "id = ?", uuid.UUID(id)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("クリニック商品が見つかりません: %w", shareddomain.ErrNotFound)
		}
		return nil, err
	}
	return toDomainClinicProduct(model), nil
}

func (r *ClinicProductRepository) ExistsByFacilityAndCode(ctx context.Context, facilityID shareddomain.ID, productCode string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&ClinicProductModel{}).
		Where("facility_id = ? AND product_code = ?", uuid.UUID(facilityID), productCode).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *ClinicProductRepository) FindByFacility(ctx context.Context, facilityID shareddomain.ID) ([]*proddomain.ClinicProduct, error) {
	var models []ClinicProductModel
	err := r.db.WithContext(ctx).
		Where("facility_id = ?", uuid.UUID(facilityID)).
		Order("product_code").
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	products := make([]*proddomain.ClinicProduct, 0, len(models))
	for _, model := range models {
		products = append(products, toDomainClinicProduct(model))
	}
	return products, nil
}

func (r *ClinicProductRepository) FindByFacilityAndJAN(ctx context.Context, facilityID shareddomain.ID, janCode string) (*proddomain.ClinicProduct, error) {
	var model ClinicProductModel
	err := r.db.WithContext(ctx).
		Where("facility_id = ? AND jan_code = ?", uuid.UUID(facilityID), janCode).
		First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("JANコード %s に該当する商品が見つかりません: %w", janCode, shareddomain.ErrNotFound)
		}
		return nil, err
	}
	return toDomainClinicProduct(model), nil
}

func toClinicProductModel(p *proddomain.ClinicProduct) ClinicProductModel {
	return ClinicProductModel{
		ID:                   uuid.UUID(p.ID()),
		FacilityID:           uuid.UUID(p.FacilityID()),
		ProductCode:          p.ProductCode(),
		Name:                 p.Name(),
		DistributorProductID: uuid.UUID(p.DistributorProductID()),
		JANCode:              p.JANCode(),
		UnitPrice:            p.UnitPrice(),
		ReorderPoint:         p.ReorderPoint(),
	}
}

func toDomainClinicProduct(model ClinicProductModel) *proddomain.ClinicProduct {
	return proddomain.ReconstructClinicProduct(
		shareddomain.ID(model.ID),
		shareddomain.ID(model.FacilityID),
		model.ProductCode,
		model.Name,
		shareddomain.ID(model.DistributorProductID),
		model.JANCode,
		model.UnitPrice,
		model.ReorderPoint,
	)
}
