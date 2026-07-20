package procurement

import (
	"context"
	"errors"
	"fmt"

	procdomain "clinic-inventory/internal/domain/procurement"
	shareddomain "clinic-inventory/internal/domain/shared"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PurchaseOrderRepository struct {
	db *gorm.DB
}

func NewPurchaseOrderRepository(db *gorm.DB) *PurchaseOrderRepository {
	return &PurchaseOrderRepository{db: db}
}

// Create は発注(親)と明細(子)をトランザクション内でまとめて挿入する。
// 明細だけ入って親が無い・親だけ入って明細が無い、という中途半端な状態を作らない。
func (r *PurchaseOrderRepository) Create(ctx context.Context, order *procdomain.PurchaseOrder) error {
	orderModel := toPurchaseOrderModel(order)
	lineModels := toPurchaseOrderLineModels(order)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&orderModel).Error; err != nil {
			return err
		}
		if len(lineModels) > 0 {
			if err := tx.Create(&lineModels).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *PurchaseOrderRepository) FindByID(ctx context.Context, id shareddomain.ID) (*procdomain.PurchaseOrder, error) {
	var orderModel PurchaseOrderModel
	if err := r.db.WithContext(ctx).First(&orderModel, "id = ?", uuid.UUID(id)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("発注が見つかりません: %w", shareddomain.ErrNotFound)
		}
		return nil, err
	}
	lineModels, err := r.findLines(ctx, []uuid.UUID{orderModel.ID})
	if err != nil {
		return nil, err
	}
	return toDomainPurchaseOrder(orderModel, lineModels[orderModel.ID]), nil
}

func (r *PurchaseOrderRepository) FindByFacility(ctx context.Context, facilityID shareddomain.ID) ([]*procdomain.PurchaseOrder, error) {
	var orderModels []PurchaseOrderModel
	err := r.db.WithContext(ctx).
		Where("facility_id = ?", uuid.UUID(facilityID)).
		Order("id").
		Find(&orderModels).Error
	if err != nil {
		return nil, err
	}
	if len(orderModels) == 0 {
		return []*procdomain.PurchaseOrder{}, nil
	}

	orderIDs := make([]uuid.UUID, 0, len(orderModels))
	for _, m := range orderModels {
		orderIDs = append(orderIDs, m.ID)
	}
	linesByOrder, err := r.findLines(ctx, orderIDs)
	if err != nil {
		return nil, err
	}

	orders := make([]*procdomain.PurchaseOrder, 0, len(orderModels))
	for _, m := range orderModels {
		orders = append(orders, toDomainPurchaseOrder(m, linesByOrder[m.ID]))
	}
	return orders, nil
}

// findLines は複数発注の明細をまとめて引き、発注IDごとにグループ化して返す。
// 一覧表示で発注ごとにクエリを繰り返すN+1を避ける。
func (r *PurchaseOrderRepository) findLines(ctx context.Context, orderIDs []uuid.UUID) (map[uuid.UUID][]PurchaseOrderLineModel, error) {
	var lineModels []PurchaseOrderLineModel
	err := r.db.WithContext(ctx).
		Where("purchase_order_id IN ?", orderIDs).
		Order("id").
		Find(&lineModels).Error
	if err != nil {
		return nil, err
	}
	byOrder := make(map[uuid.UUID][]PurchaseOrderLineModel, len(orderIDs))
	for _, l := range lineModels {
		byOrder[l.PurchaseOrderID] = append(byOrder[l.PurchaseOrderID], l)
	}
	return byOrder, nil
}

func toPurchaseOrderModel(o *procdomain.PurchaseOrder) PurchaseOrderModel {
	return PurchaseOrderModel{
		ID:            uuid.UUID(o.ID()),
		FacilityID:    uuid.UUID(o.FacilityID()),
		DistributorID: uuid.UUID(o.DistributorID()),
		Status:        string(o.Status()),
	}
}

func toPurchaseOrderLineModels(o *procdomain.PurchaseOrder) []PurchaseOrderLineModel {
	lines := o.Lines()
	models := make([]PurchaseOrderLineModel, 0, len(lines))
	for _, l := range lines {
		models = append(models, PurchaseOrderLineModel{
			ID:              uuid.New(),
			PurchaseOrderID: uuid.UUID(o.ID()),
			ClinicProductID: uuid.UUID(l.ClinicProductID()),
			Quantity:        l.Quantity(),
			UnitPrice:       l.UnitPrice(),
		})
	}
	return models
}

func toDomainPurchaseOrder(orderModel PurchaseOrderModel, lineModels []PurchaseOrderLineModel) *procdomain.PurchaseOrder {
	lines := make([]procdomain.OrderLine, 0, len(lineModels))
	for _, l := range lineModels {
		lines = append(lines, procdomain.ReconstructOrderLine(shareddomain.ID(l.ClinicProductID), l.Quantity, l.UnitPrice))
	}
	return procdomain.ReconstructPurchaseOrder(
		shareddomain.ID(orderModel.ID),
		shareddomain.ID(orderModel.FacilityID),
		shareddomain.ID(orderModel.DistributorID),
		procdomain.OrderStatus(orderModel.Status),
		lines,
	)
}
