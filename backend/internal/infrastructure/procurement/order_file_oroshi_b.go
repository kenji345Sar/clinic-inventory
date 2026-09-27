package procurement

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
)

// encodeOroshiB はB社（oroshi-b）向け。UTF-8のCSV、改行CR、ヘッダ行なし。
// 列: 発注No（自社の発注IDのまま）, 行番号, 商品コード, 数量
func encodeOroshiB(in orderFileInput) (orderFile, error) {
	buf := &bytes.Buffer{}
	for i, l := range in.Lines {
		row := []string{
			in.Order.ID().String(),
			strconv.Itoa(lineNo(i)),
			l.DistributorProductCode,
			strconv.Itoa(l.Quantity),
		}
		if err := writeCsvRecordCR(buf, row); err != nil {
			return orderFile{}, fmt.Errorf("B社向け発注CSVの書き込みに失敗しました: %w", err)
		}
	}
	return orderFile{Body: buf.Bytes(), Ext: ".csv", ContentType: "text/csv; charset=UTF-8"}, nil
}

// writeCsvRecordCR は1レコードをCSVとして書き、改行をCRにする。
// encoding/csv はLFかCRLFしか出せないため、1行ずつ書いて末尾のLFをCRに置き換える。
func writeCsvRecordCR(buf *bytes.Buffer, record []string) error {
	line := &bytes.Buffer{}
	w := csv.NewWriter(line)
	if err := w.Write(record); err != nil {
		return err
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	buf.Write(bytes.TrimSuffix(line.Bytes(), []byte("\n")))
	buf.WriteByte('\r')
	return nil
}
