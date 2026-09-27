package procurement

import (
	"context"
	"fmt"
	"time"

	procapp "clinic-inventory/internal/application/procurement"
	procdomain "clinic-inventory/internal/domain/procurement"
	"clinic-inventory/internal/infrastructure/storage"
)

// PurchaseOrderCsvUploader は確定した発注を卸に送るファイルに変換しS3へアップロードする。
//
// 卸ごとにファイルの形式（列・文字コード・改行・固定長など）が異なるため、
// 書き方は卸コードで order_file.go の対応表から選ぶ。表に無い卸には既定の9列CSVで送る。
type PurchaseOrderCsvUploader struct {
	uploader *storage.S3Uploader
}

func NewPurchaseOrderCsvUploader(uploader *storage.S3Uploader) *PurchaseOrderCsvUploader {
	return &PurchaseOrderCsvUploader{uploader: uploader}
}

func (u *PurchaseOrderCsvUploader) Upload(ctx context.Context, order *procdomain.PurchaseOrder, distributorCode, facilityName string, orderedAt time.Time, lines []procapp.PurchaseOrderCsvLine) error {
	file, err := resolveEncoder(distributorCode)(orderFileInput{
		Order:        order,
		FacilityName: facilityName,
		OrderedAt:    orderedAt,
		Lines:        lines,
	})
	if err != nil {
		return err
	}
	// 卸ごとにフォルダを分け、卸側からはIAM等で自社フォルダのみアクセスさせる想定。
	// フォルダ名は卸業者に案内する場面で人が読めるよう、UUIDではなく卸コードを使う
	// (clinic-inventory-csv-functions/docs/s3-storage.md 3章)。
	key := fmt.Sprintf("orders/%s/%s/%s%s", distributorCode, order.FacilityID().String(), order.ID().String(), file.Ext)
	return u.uploader.Put(ctx, key, file.Body, file.ContentType)
}
