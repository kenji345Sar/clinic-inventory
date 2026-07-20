package distributorcatalog

import (
	"errors"

	shareddomain "clinic-inventory/internal/domain/shared"
)

// DistributorProduct（卸商品）。独立した集約ルート（Distributorの配下エンティティにはしない。
// 理由はdocs/architecture/domain-rules.mdの「卸連携コンテキスト」参照）。
//
// 卸商品コード（DistributorProductCode）は卸独自の商品コード体系。
// ベンダー（メーカー）名・ベンダーが割り当てている商品コード（VendorProductCode）は別途保持する。
type DistributorProduct struct {
	id                     shareddomain.ID
	distributorID          shareddomain.ID
	distributorProductCode string
	name                   string
	vendorName             string
	vendorProductCode      string // 任意。ベンダーが割り当てている商品コード
	janCode                string // 任意
	unitPrice              int    // 標準単価（税抜・円）。その卸の定価
	discontinued           bool
}

func NewDistributorProduct(distributorID shareddomain.ID, distributorProductCode, name, vendorName string, unitPrice int) (*DistributorProduct, error) {
	if distributorID.IsZero() {
		return nil, errors.New("卸業者の指定は必須です")
	}
	if distributorProductCode == "" {
		return nil, errors.New("卸商品コードは必須です")
	}
	if name == "" {
		return nil, errors.New("商品名は必須です")
	}
	if vendorName == "" {
		return nil, errors.New("ベンダー名は必須です")
	}
	if unitPrice <= 0 {
		return nil, errors.New("単価は1円以上で指定してください")
	}
	return &DistributorProduct{
		id:                     shareddomain.NewID(),
		distributorID:          distributorID,
		distributorProductCode: distributorProductCode,
		name:                   name,
		vendorName:             vendorName,
		unitPrice:              unitPrice,
	}, nil
}

func (p *DistributorProduct) ID() shareddomain.ID            { return p.id }
func (p *DistributorProduct) DistributorID() shareddomain.ID { return p.distributorID }
func (p *DistributorProduct) DistributorProductCode() string { return p.distributorProductCode }
func (p *DistributorProduct) Name() string                   { return p.name }
func (p *DistributorProduct) VendorName() string             { return p.vendorName }
func (p *DistributorProduct) VendorProductCode() string      { return p.vendorProductCode }
func (p *DistributorProduct) JANCode() string                { return p.janCode }
func (p *DistributorProduct) UnitPrice() int                 { return p.unitPrice }
func (p *DistributorProduct) Discontinued() bool             { return p.discontinued }

func (p *DistributorProduct) SetVendorProductCode(code string) {
	p.vendorProductCode = code
}

func (p *DistributorProduct) SetJANCode(code string) {
	p.janCode = code
}

// Discontinue は卸商品を廃盤にする。物理削除しないのは、クリニック商品からの参照が
// 残っている可能性があるため（参照整合性を壊さない）。
func (p *DistributorProduct) Discontinue() {
	p.discontinued = true
}

// ReconstructDistributorProduct は永続化データからDistributorProductを復元する。バリデーションは行わない。
func ReconstructDistributorProduct(
	id shareddomain.ID,
	distributorID shareddomain.ID,
	distributorProductCode, name, vendorName, vendorProductCode, janCode string,
	unitPrice int,
	discontinued bool,
) *DistributorProduct {
	return &DistributorProduct{
		id:                     id,
		distributorID:          distributorID,
		distributorProductCode: distributorProductCode,
		name:                   name,
		vendorName:             vendorName,
		vendorProductCode:      vendorProductCode,
		janCode:                janCode,
		unitPrice:              unitPrice,
		discontinued:           discontinued,
	}
}
