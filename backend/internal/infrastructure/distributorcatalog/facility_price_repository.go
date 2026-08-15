package distributorcatalog

import (
	"context"

	distdomain "clinic-inventory/internal/domain/distributorcatalog"
	shareddomain "clinic-inventory/internal/domain/shared"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type FacilityPriceRepository struct {
	db *gorm.DB
}

func NewFacilityPriceRepository(db *gorm.DB) *FacilityPriceRepository {
	return &FacilityPriceRepository{db: db}
}

func (r *FacilityPriceRepository) FindByProductAndFacility(ctx context.Context, distributorProductID, facilityID shareddomain.ID) (*distdomain.FacilityPrice, error) {
	// 医院別単価が無いのは通常のケース（標準単価を使う卸）なので、
	// 「見つからない」をエラーにしないLimit(1).Findで引く。
	var models []DistributorProductFacilityPriceModel
	err := r.db.WithContext(ctx).
		Where("distributor_product_id = ? AND facility_id = ?", uuid.UUID(distributorProductID), uuid.UUID(facilityID)).
		Limit(1).
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return nil, nil
	}
	return toDomainFacilityPrice(models[0]), nil
}

func (r *FacilityPriceRepository) FindByFacility(ctx context.Context, facilityID shareddomain.ID) ([]*distdomain.FacilityPrice, error) {
	var models []DistributorProductFacilityPriceModel
	if err := r.db.WithContext(ctx).
		Where("facility_id = ?", uuid.UUID(facilityID)).
		Find(&models).Error; err != nil {
		return nil, err
	}
	prices := make([]*distdomain.FacilityPrice, 0, len(models))
	for _, model := range models {
		prices = append(prices, toDomainFacilityPrice(model))
	}
	return prices, nil
}

func (r *FacilityPriceRepository) FindByProduct(ctx context.Context, distributorProductID shareddomain.ID) ([]*distdomain.FacilityPrice, error) {
	var models []DistributorProductFacilityPriceModel
	if err := r.db.WithContext(ctx).
		Where("distributor_product_id = ?", uuid.UUID(distributorProductID)).
		Find(&models).Error; err != nil {
		return nil, err
	}
	prices := make([]*distdomain.FacilityPrice, 0, len(models))
	for _, model := range models {
		prices = append(prices, toDomainFacilityPrice(model))
	}
	return prices, nil
}

func (r *FacilityPriceRepository) CountByProducts(ctx context.Context, distributorProductIDs []shareddomain.ID) (map[shareddomain.ID]int, error) {
	counts := map[shareddomain.ID]int{}
	if len(distributorProductIDs) == 0 {
		return counts, nil
	}
	ids := make([]uuid.UUID, 0, len(distributorProductIDs))
	for _, id := range distributorProductIDs {
		ids = append(ids, uuid.UUID(id))
	}

	// 商品ごとの件数はDB側でGROUP BYして数える（行そのものは取らない）。
	var rows []struct {
		DistributorProductID uuid.UUID
		Count                int
	}
	if err := r.db.WithContext(ctx).
		Model(&DistributorProductFacilityPriceModel{}).
		Select("distributor_product_id, COUNT(*) AS count").
		Where("distributor_product_id IN ?", ids).
		Group("distributor_product_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		counts[shareddomain.ID(row.DistributorProductID)] = row.Count
	}
	return counts, nil
}

func toDomainFacilityPrice(model DistributorProductFacilityPriceModel) *distdomain.FacilityPrice {
	return distdomain.ReconstructFacilityPrice(
		shareddomain.ID(model.DistributorProductID),
		shareddomain.ID(model.FacilityID),
		model.UnitPrice,
	)
}
