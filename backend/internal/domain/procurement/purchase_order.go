package procurement

import (
	"errors"
	"time"

	shareddomain "clinic-inventory/internal/domain/shared"
)

// OrderStatus は発注の状態。下書き→確定の2状態から始める。
type OrderStatus string

const (
	StatusDraft     OrderStatus = "draft"
	StatusConfirmed OrderStatus = "confirmed"
)

// OrderLine は発注明細。発注集約の内部に属する値オブジェクト。
// 「どのクリニック商品を何個、いくらで」を持つ。
// 単価(unitPrice)は発注作成時のクリニック商品の単価をスナップショットした値。
// マスタ単価が後で変わっても、確定済み発注の金額は変わらないようにするため明細に固定で持つ。
type OrderLine struct {
	clinicProductID shareddomain.ID
	quantity        int
	unitPrice       int // 発注時点の単価（税抜・円）のスナップショット
}

func (l OrderLine) ClinicProductID() shareddomain.ID { return l.clinicProductID }
func (l OrderLine) Quantity() int                    { return l.quantity }
func (l OrderLine) UnitPrice() int                   { return l.unitPrice }

// Amount は明細金額（数量 × 単価、税抜・円）。
func (l OrderLine) Amount() int { return l.quantity * l.unitPrice }

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
	confirmedAt   *time.Time
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
// unitPrice は発注時点のクリニック商品単価をスナップショットした値。
func (o *PurchaseOrder) AddLine(clinicProductID shareddomain.ID, quantity, unitPrice int) error {
	if o.status != StatusDraft {
		return errors.New("確定済みの発注には明細を追加できません")
	}
	if clinicProductID.IsZero() {
		return errors.New("クリニック商品の指定は必須です")
	}
	if quantity <= 0 {
		return errors.New("数量は1以上で指定してください")
	}
	if unitPrice <= 0 {
		return errors.New("単価は1円以上で指定してください")
	}
	for i := range o.lines {
		if o.lines[i].clinicProductID == clinicProductID {
			o.lines[i].quantity += quantity
			return nil
		}
	}
	o.lines = append(o.lines, OrderLine{clinicProductID: clinicProductID, quantity: quantity, unitPrice: unitPrice})
	return nil
}

// Confirm は下書きを確定する。明細が1件以上必要。
// confirmedAt は発注日時として永続化され、発注CSVの「発注日」とも揃えるため
// 呼び出し元（ユースケース層）が生成した同一の time.Time を渡す想定。
func (o *PurchaseOrder) Confirm(confirmedAt time.Time) error {
	if o.status == StatusConfirmed {
		return errors.New("既に確定済みです")
	}
	if len(o.lines) == 0 {
		return errors.New("明細が無い発注は確定できません")
	}
	o.status = StatusConfirmed
	o.confirmedAt = &confirmedAt
	return nil
}

func (o *PurchaseOrder) ID() shareddomain.ID            { return o.id }
func (o *PurchaseOrder) FacilityID() shareddomain.ID    { return o.facilityID }
func (o *PurchaseOrder) DistributorID() shareddomain.ID { return o.distributorID }
func (o *PurchaseOrder) Status() OrderStatus            { return o.status }

// ConfirmedAt は確定日時。下書き中はnil。
func (o *PurchaseOrder) ConfirmedAt() *time.Time { return o.confirmedAt }

// Lines は明細のコピーを返す。呼び出し側が返り値を書き換えても集約内部は壊れない。
func (o *PurchaseOrder) Lines() []OrderLine {
	out := make([]OrderLine, len(o.lines))
	copy(out, o.lines)
	return out
}

// TotalAmount は発注の合計金額（明細金額の総和、税抜・円）。
func (o *PurchaseOrder) TotalAmount() int {
	total := 0
	for _, l := range o.lines {
		total += l.Amount()
	}
	return total
}

// ReconstructPurchaseOrder は永続化データから発注を復元する。バリデーションは行わない。
func ReconstructPurchaseOrder(
	id, facilityID, distributorID shareddomain.ID,
	status OrderStatus,
	lines []OrderLine,
	confirmedAt *time.Time,
) *PurchaseOrder {
	return &PurchaseOrder{
		id:            id,
		facilityID:    facilityID,
		distributorID: distributorID,
		status:        status,
		lines:         lines,
		confirmedAt:   confirmedAt,
	}
}

// ReconstructOrderLine は永続化データから明細を復元する。
func ReconstructOrderLine(clinicProductID shareddomain.ID, quantity, unitPrice int) OrderLine {
	return OrderLine{clinicProductID: clinicProductID, quantity: quantity, unitPrice: unitPrice}
}
