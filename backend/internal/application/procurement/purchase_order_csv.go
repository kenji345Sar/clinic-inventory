package procurement

import (
	"context"
	"time"

	procdomain "clinic-inventory/internal/domain/procurement"
)

// PurchaseOrderCsvLine は発注CSVの明細1行分。
// 卸商品コード・商品名は卸商品(DistributorProduct)から取る値のため、
// 集約(PurchaseOrder.OrderLine)には持たせずユースケース側で組み立てる。
type PurchaseOrderCsvLine struct {
	DistributorProductCode string
	ProductName            string
	Quantity               int
	UnitPrice              int
}

// Amount は明細金額（数量 × 単価、税抜・円）。
func (l PurchaseOrderCsvLine) Amount() int { return l.Quantity * l.UnitPrice }

// PurchaseOrderCsvUploader は確定した発注を卸連携用CSVに変換しS3へアップロードするポート。
// 卸ごとにCSVフォーマットが異なるため(docs/architecture/domain-rules.md「卸連携CSV基盤」)、
// 実装は卸別アダプタとして infrastructure 層に置く。
type PurchaseOrderCsvUploader interface {
	Upload(ctx context.Context, order *procdomain.PurchaseOrder, facilityName string, orderedAt time.Time, lines []PurchaseOrderCsvLine) error
}
