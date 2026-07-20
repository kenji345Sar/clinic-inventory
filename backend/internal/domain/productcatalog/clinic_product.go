package productcatalog

import (
	"errors"

	shareddomain "clinic-inventory/internal/domain/shared"
)

// ClinicProduct（クリニック商品）。商品マスタコンテキストの集約ルート。
//
// クリニックは卸商品（DistributorProduct）を元に商品を登録し、
// クリニック独自の商品コード（ProductCode）を割り当てる。
// 卸商品への紐付けは必須（docs/architecture/domain-rules.md「商品マスタコンテキスト」）。
//
// クリニック商品コードの同一クリニック内での一意性は、卸商品コードと同様に
// リポジトリ層の存在チェック + DBの (facility_id, product_code) ユニーク制約で担保する。
type ClinicProduct struct {
	id                   shareddomain.ID
	facilityID           shareddomain.ID
	productCode          string
	name                 string
	distributorProductID shareddomain.ID
	janCode              string // 任意。JANが無い商品は名称等で検索する運用
	reorderPoint         int
}

func NewClinicProduct(facilityID shareddomain.ID, productCode, name string, distributorProductID shareddomain.ID, reorderPoint int) (*ClinicProduct, error) {
	if facilityID.IsZero() {
		return nil, errors.New("クリニックの指定は必須です")
	}
	if productCode == "" {
		return nil, errors.New("商品コードは必須です")
	}
	if name == "" {
		return nil, errors.New("商品名は必須です")
	}
	if distributorProductID.IsZero() {
		return nil, errors.New("卸商品への紐付けは必須です")
	}
	if reorderPoint < 0 {
		return nil, errors.New("発注点は0以上で指定してください")
	}
	return &ClinicProduct{
		id:                   shareddomain.NewID(),
		facilityID:           facilityID,
		productCode:          productCode,
		name:                 name,
		distributorProductID: distributorProductID,
		reorderPoint:         reorderPoint,
	}, nil
}

func (p *ClinicProduct) ID() shareddomain.ID                   { return p.id }
func (p *ClinicProduct) FacilityID() shareddomain.ID           { return p.facilityID }
func (p *ClinicProduct) ProductCode() string                   { return p.productCode }
func (p *ClinicProduct) Name() string                          { return p.name }
func (p *ClinicProduct) DistributorProductID() shareddomain.ID { return p.distributorProductID }
func (p *ClinicProduct) JANCode() string                       { return p.janCode }
func (p *ClinicProduct) ReorderPoint() int                     { return p.reorderPoint }

func (p *ClinicProduct) SetJANCode(code string) {
	p.janCode = code
}

// ChangeReorderPoint は発注点を変更する。
func (p *ClinicProduct) ChangeReorderPoint(reorderPoint int) error {
	if reorderPoint < 0 {
		return errors.New("発注点は0以上で指定してください")
	}
	p.reorderPoint = reorderPoint
	return nil
}

// ReconstructClinicProduct は永続化データからClinicProductを復元する。バリデーションは行わない。
func ReconstructClinicProduct(
	id shareddomain.ID,
	facilityID shareddomain.ID,
	productCode, name string,
	distributorProductID shareddomain.ID,
	janCode string,
	reorderPoint int,
) *ClinicProduct {
	return &ClinicProduct{
		id:                   id,
		facilityID:           facilityID,
		productCode:          productCode,
		name:                 name,
		distributorProductID: distributorProductID,
		janCode:              janCode,
		reorderPoint:         reorderPoint,
	}
}
