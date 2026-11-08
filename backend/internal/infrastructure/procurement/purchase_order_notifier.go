package procurement

import (
	"context"
	"errors"
	"fmt"
	"time"

	distdomain "clinic-inventory/internal/domain/distributorcatalog"
	orgdomain "clinic-inventory/internal/domain/organization"
	procdomain "clinic-inventory/internal/domain/procurement"
	"clinic-inventory/internal/infrastructure/mail"
)

// jst は本文の確定日時を日本時間で出すためのタイムゾーン。
// LoadLocationはOSのタイムゾーンデータが無い環境で失敗するため、固定オフセットで持つ。
var jst = time.FixedZone("JST", 9*60*60)

// PurchaseOrderNotifier は発注確定のお知らせをメールで医院と卸に送る。
// CSVは添付しない（卸への発注ファイルの届け方は未決。docs/purchase-order-export.md 9章）。
type PurchaseOrderNotifier struct {
	sender *mail.SESSender
}

func NewPurchaseOrderNotifier(sender *mail.SESSender) *PurchaseOrderNotifier {
	return &PurchaseOrderNotifier{sender: sender}
}

func (n *PurchaseOrderNotifier) NotifyConfirmed(ctx context.Context, order *procdomain.PurchaseOrder, distributor *distdomain.Distributor, facility *orgdomain.Facility) error {
	subject := fmt.Sprintf("【発注確定】%s → %s", facility.Name(), distributor.Name())
	body := confirmedMailBody(order, distributor, facility)

	// 片方の送信に失敗しても、もう片方には送る。失敗はまとめて返す。
	var errs []error
	for _, to := range []string{facility.NotificationEmail(), distributor.NotificationEmail()} {
		if to == "" {
			continue
		}
		if err := n.sender.Send(ctx, to, subject, body); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func confirmedMailBody(order *procdomain.PurchaseOrder, distributor *distdomain.Distributor, facility *orgdomain.Facility) string {
	confirmedAt := ""
	if order.ConfirmedAt() != nil {
		confirmedAt = order.ConfirmedAt().In(jst).Format("2006-01-02 15:04")
	}
	return fmt.Sprintf(`以下の発注が確定しました。

発注ID: %s
発注元: %s
発注先: %s
確定日時: %s
明細件数: %d件
合計金額(税抜): %d円
`, order.ID().String(), facility.Name(), distributor.Name(), confirmedAt, len(order.Lines()), order.TotalAmount())
}
