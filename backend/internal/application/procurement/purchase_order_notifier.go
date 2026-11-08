package procurement

import (
	"context"

	distdomain "clinic-inventory/internal/domain/distributorcatalog"
	orgdomain "clinic-inventory/internal/domain/organization"
	procdomain "clinic-inventory/internal/domain/procurement"
)

// PurchaseOrderNotifier は発注が確定したことを医院と卸に知らせるポート。
// 送信手段(SES等)に依存しないよう、実装は infrastructure 層に置く。
type PurchaseOrderNotifier interface {
	// NotifyConfirmed は医院・卸それぞれの通知先メールアドレスへお知らせを送る。
	// 通知先が空の相手には送らない。
	NotifyConfirmed(ctx context.Context, order *procdomain.PurchaseOrder, distributor *distdomain.Distributor, facility *orgdomain.Facility) error
}
