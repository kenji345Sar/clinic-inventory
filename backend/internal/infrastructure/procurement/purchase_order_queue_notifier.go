package procurement

import (
	"context"
	"encoding/json"

	distdomain "clinic-inventory/internal/domain/distributorcatalog"
	orgdomain "clinic-inventory/internal/domain/organization"
	procdomain "clinic-inventory/internal/domain/procurement"
	"clinic-inventory/internal/infrastructure/queue"
)

// OrderConfirmedMessage は発注確定のお知らせ用にSQSへ積むメッセージ。
// 本文に使う値は入れず発注IDだけにする。積んだ時点の値を入れると、送るのが遅れたときに
// DBと違う内容のメールになり、DBとメッセージの二重管理になる。本文は受け取った側がDBから組み立てる。
type OrderConfirmedMessage struct {
	OrderID string `json:"orderId"`
}

// PurchaseOrderQueueNotifier は PurchaseOrderNotifier のSQS版。メールは送らず、SQSに積むだけ。
// 実際の送信は通知ワーカー(cmd/notifyworker)がSQSから取り出して PurchaseOrderNotifier(SES版)で行う。
type PurchaseOrderQueueNotifier struct {
	queue *queue.SQSQueue
}

func NewPurchaseOrderQueueNotifier(q *queue.SQSQueue) *PurchaseOrderQueueNotifier {
	return &PurchaseOrderQueueNotifier{queue: q}
}

func (n *PurchaseOrderQueueNotifier) NotifyConfirmed(ctx context.Context, order *procdomain.PurchaseOrder, _ *distdomain.Distributor, _ *orgdomain.Facility) error {
	body, err := json.Marshal(OrderConfirmedMessage{OrderID: order.ID().String()})
	if err != nil {
		return err
	}
	return n.queue.Send(ctx, string(body))
}
