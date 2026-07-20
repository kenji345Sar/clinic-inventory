package procurement

import (
	"errors"

	shareddomain "clinic-inventory/internal/domain/shared"
)

// OrderStatus は発注の状態。下書き→確定の2状態から始める。
type OrderStatus string

const (
	StatusDraft     OrderStatus = "draft"
	StatusConfirmed OrderStatus = "confirmed"
)

// OrderLine は発注明細。発注集約の内部に属する値オブジェクト。
// 「どのクリニック商品を何個」だけを持つ。
// 発注書に必要な卸商品コード・商品名は、PDF生成時にクリニック商品経由で引き当てる想定で、
// この段階では明細に持たせない。
type OrderLine struct {
	clinicProductID shareddomain.ID
	quantity        int
}

func (l OrderLine) ClinicProductID() shareddomain.ID { return l.clinicProductID }
func (l OrderLine) Quantity() int                    { return l.quantity }

// PurchaseOrder（発注）。発注コンテキストの集約ルート。
//
// 1発注 = 1卸（docs/requirements.md「11. 決定事項」2026-07-20）。
// 明細は全て同じ卸業者(distributorID)の商品であることを前提とするが、
// 「そのクリニック商品が実際にこの卸の商品か」という他集約にまたがる検証はユースケース層で行う。
// 集約自身は「数量は1以上」「確定には明細が1件以上必要」という自己完結する不変条件だけを守る。
type PurchaseOrder struct {
	id            shareddomain.ID
	facilityID    shareddomain.ID
	distributorID shareddomain.ID
	status        OrderStatus
	lines         []OrderLine
}

// NewPurchaseOrder は下書き状態の発注を作る。明細は AddLine で追加する。
func NewPurchaseOrder(facilityID, distributorID shareddomain.ID) (*PurchaseOrder, error) {
	if facilityID.IsZero() {
		return nil, errors.New("クリニックの指定は必須です")
	}
	if distributorID.IsZero() {
		return nil, errors.New("卸業者の指定は必須です")
	}
	return &PurchaseOrder{
		id:            shareddomain.NewID(),
		facilityID:    facilityID,
		distributorID: distributorID,
		status:        StatusDraft,
	}, nil
}

// AddLine は下書き中の発注に明細を追加する。
// 同じクリニック商品が既にあれば数量を加算し、重複明細を作らない。
func (o *PurchaseOrder) AddLine(clinicProductID shareddomain.ID, quantity int) error {
	if o.status != StatusDraft {
		return errors.New("確定済みの発注には明細を追加できません")
	}
	if clinicProductID.IsZero() {
		return errors.New("クリニック商品の指定は必須です")
	}
	if quantity <= 0 {
		return errors.New("数量は1以上で指定してください")
	}
	for i := range o.lines {
		if o.lines[i].clinicProductID == clinicProductID {
			o.lines[i].quantity += quantity
			return nil
		}
	}
	o.lines = append(o.lines, OrderLine{clinicProductID: clinicProductID, quantity: quantity})
	return nil
}

// Confirm は下書きを確定する。明細が1件以上必要。
func (o *PurchaseOrder) Confirm() error {
	if o.status == StatusConfirmed {
		return errors.New("既に確定済みです")
	}
	if len(o.lines) == 0 {
		return errors.New("明細が無い発注は確定できません")
	}
	o.status = StatusConfirmed
	return nil
}

func (o *PurchaseOrder) ID() shareddomain.ID            { return o.id }
func (o *PurchaseOrder) FacilityID() shareddomain.ID    { return o.facilityID }
func (o *PurchaseOrder) DistributorID() shareddomain.ID { return o.distributorID }
func (o *PurchaseOrder) Status() OrderStatus            { return o.status }

// Lines は明細のコピーを返す。呼び出し側が返り値を書き換えても集約内部は壊れない。
func (o *PurchaseOrder) Lines() []OrderLine {
	out := make([]OrderLine, len(o.lines))
	copy(out, o.lines)
	return out
}

// ReconstructPurchaseOrder は永続化データから発注を復元する。バリデーションは行わない。
func ReconstructPurchaseOrder(
	id, facilityID, distributorID shareddomain.ID,
	status OrderStatus,
	lines []OrderLine,
) *PurchaseOrder {
	return &PurchaseOrder{
		id:            id,
		facilityID:    facilityID,
		distributorID: distributorID,
		status:        status,
		lines:         lines,
	}
}

// ReconstructOrderLine は永続化データから明細を復元する。
func ReconstructOrderLine(clinicProductID shareddomain.ID, quantity int) OrderLine {
	return OrderLine{clinicProductID: clinicProductID, quantity: quantity}
}
