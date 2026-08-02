package procurement

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	procapp "clinic-inventory/internal/application/procurement"
	procdomain "clinic-inventory/internal/domain/procurement"
	"clinic-inventory/internal/infrastructure/storage"
)

// PurchaseOrderCsvUploader は確定した発注を発注CSVに変換しS3へアップロードする。
//
// 卸ごとにCSVフォーマットが異なるため(docs/architecture/domain-rules.md「卸連携CSV基盤」)、
// これは代表として卸1社分のフォーマットを実装したもの。他の卸に対応する際は、
// この実装を卸別アダプタとして差し替える(発注コンテキストのEDIアダプタ方針と同じ考え方)。
type PurchaseOrderCsvUploader struct {
	uploader *storage.S3Uploader
}

func NewPurchaseOrderCsvUploader(uploader *storage.S3Uploader) *PurchaseOrderCsvUploader {
	return &PurchaseOrderCsvUploader{uploader: uploader}
}

var csvHeader = []string{"発注ID", "発注日", "クリニックID", "クリニック名", "卸商品コード", "商品名", "数量", "単価", "金額"}

func (u *PurchaseOrderCsvUploader) Upload(ctx context.Context, order *procdomain.PurchaseOrder, facilityName string, orderedAt time.Time, lines []procapp.PurchaseOrderCsvLine) error {
	body, err := encodePurchaseOrderCsv(order, facilityName, orderedAt, lines)
	if err != nil {
		return err
	}
	// 卸ごとにフォルダを分け、卸側からはIAM等で自社フォルダのみアクセスさせる想定。
	key := fmt.Sprintf("orders/%s/%s/%s.csv", order.DistributorID().String(), order.FacilityID().String(), order.ID().String())
	return u.uploader.Put(ctx, key, body, "text/csv")
}

func encodePurchaseOrderCsv(order *procdomain.PurchaseOrder, facilityName string, orderedAt time.Time, lines []procapp.PurchaseOrderCsvLine) ([]byte, error) {
	buf := &bytes.Buffer{}
	w := csv.NewWriter(buf)

	if err := w.Write(csvHeader); err != nil {
		return nil, fmt.Errorf("発注CSVのヘッダ書き込みに失敗しました: %w", err)
	}

	orderDate := orderedAt.Format("2006-01-02")
	for _, l := range lines {
		row := []string{
			order.ID().String(),
			orderDate,
			order.FacilityID().String(),
			facilityName,
			l.DistributorProductCode,
			l.ProductName,
			strconv.Itoa(l.Quantity),
			strconv.Itoa(l.UnitPrice),
			strconv.Itoa(l.Amount()),
		}
		if err := w.Write(row); err != nil {
			return nil, fmt.Errorf("発注CSVの明細書き込みに失敗しました: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("発注CSVの書き込みに失敗しました: %w", err)
	}
	return buf.Bytes(), nil
}
