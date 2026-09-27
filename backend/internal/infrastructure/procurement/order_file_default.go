package procurement

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
)

var defaultCsvHeader = []string{"発注ID", "発注日", "クリニックID", "クリニック名", "卸商品コード", "商品名", "数量", "単価", "金額"}

// defaultEncode は対応表に無い卸へ送る形式。日本語ヘッダ9列、UTF-8、改行LF。
func defaultEncode(in orderFileInput) (orderFile, error) {
	buf := &bytes.Buffer{}
	w := csv.NewWriter(buf)

	if err := w.Write(defaultCsvHeader); err != nil {
		return orderFile{}, fmt.Errorf("発注CSVのヘッダ書き込みに失敗しました: %w", err)
	}

	orderDate := in.OrderedAt.Format("2006-01-02")
	for _, l := range in.Lines {
		row := []string{
			in.Order.ID().String(),
			orderDate,
			in.Order.FacilityID().String(),
			in.FacilityName,
			l.DistributorProductCode,
			l.ProductName,
			strconv.Itoa(l.Quantity),
			strconv.Itoa(l.UnitPrice),
			strconv.Itoa(l.Amount()),
		}
		if err := w.Write(row); err != nil {
			return orderFile{}, fmt.Errorf("発注CSVの明細書き込みに失敗しました: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return orderFile{}, fmt.Errorf("発注CSVの書き込みに失敗しました: %w", err)
	}
	return orderFile{Body: buf.Bytes(), Ext: ".csv", ContentType: "text/csv"}, nil
}
