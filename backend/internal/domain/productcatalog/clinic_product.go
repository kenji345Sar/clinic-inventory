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
	// 仕入単価（税抜・円）。卸商品の標準単価または医院別単価を初期値にする。
	// 単価が分からない卸もあるため0を許容し、その場合は後から受注結果の単価で更新する運用
	// （docs/architecture/distributor-catalog-import.md 3章）。
	unitPrice    int
	reorderPoint int
}

func NewClinicProduct(facilityID shareddomain.ID, productCode, name string, distributorProductID shareddomain.ID, unitPrice, reorderPoint int) (*ClinicProduct, error) {
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
	if unitPrice < 0 {
		return nil, errors.New("単価は0以上で指定してください")
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
		unitPrice:            unitPrice,
		reorderPoint:         reorderPoint,
	}, nil
}

func (p *ClinicProduct) ID() shareddomain.ID                   { return p.id }
func (p *ClinicProduct) FacilityID() shareddomain.ID           { return p.facilityID }
func (p *ClinicProduct) ProductCode() string                   { return p.productCode }
func (p *ClinicProduct) Name() string                          { return p.name }
func (p *ClinicProduct) DistributorProductID() shareddomain.ID { return p.distributorProductID }
func (p *ClinicProduct) JANCode() string                       { return p.janCode }
func (p *ClinicProduct) UnitPrice() int                        { return p.unitPrice }
func (p *ClinicProduct) ReorderPoint() int                     { return p.reorderPoint }

func (p *ClinicProduct) SetJANCode(code string) {
	p.janCode = code
}

// ChangeUnitPrice は仕入単価を変更する（医院別単価の設定・更新）。
// 単価未確定の商品を0のまま登録できるため、0への変更も許容する。
func (p *ClinicProduct) ChangeUnitPrice(unitPrice int) error {
	if unitPrice < 0 {
		return errors.New("単価は0以上で指定してください")
	}
	p.unitPrice = unitPrice
	return nil
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
	unitPrice int,
	reorderPoint int,
) *ClinicProduct {
	return &ClinicProduct{
		id:                   id,
		facilityID:           facilityID,
		productCode:          productCode,
		name:                 name,
		distributorProductID: distributorProductID,
		janCode:              janCode,
		unitPrice:            unitPrice,
		reorderPoint:         reorderPoint,
	}
}
