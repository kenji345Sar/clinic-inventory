package procurement

import (
	"fmt"
	"time"

	procapp "clinic-inventory/internal/application/procurement"
	procdomain "clinic-inventory/internal/domain/procurement"
)

// orderFileInput は発注ファイル1本を書き出すのに要る値。卸ごとの書き方はここから必要な列だけを使う。
type orderFileInput struct {
	Order        *procdomain.PurchaseOrder
	FacilityName string
	OrderedAt    time.Time
	Lines        []procapp.PurchaseOrderCsvLine
}

// orderFile は書き出した発注ファイル。拡張子とContent-Typeも卸ごとに違うため本文と一緒に返す。
type orderFile struct {
	Body        []byte
	Ext         string // S3キーの拡張子（".csv" など）
	ContentType string
}

// encodeFunc は「発注 → 卸に送るファイル」の変換1つ分。卸ごとに1つ書く（order_file_<卸コード>.go）。
type encodeFunc func(in orderFileInput) (orderFile, error)

// orderFileEncoders は卸コードと卸別の書き方の対応表。
//
// 取り込み側(clinic-inventory-csv-functions)の ParserRegistry の逆向き。
// **卸ごとに違うのはファイルの中身だけ**で、S3キーの組み立て・アップロードは全卸で共通。
// 新しい卸に対応するときは、order_file_<卸コード>.go を1つ書いてこの表に1行足す。
// 表に無い卸には defaultEncode（日本語ヘッダ9列のCSV）で送る。
var orderFileEncoders = map[string]encodeFunc{
	"oroshi-b": encodeOroshiB, // UTF-8のCSV、改行CR
	"oroshi-c": encodeOroshiC, // Shift_JISの固定長
}

func resolveEncoder(distributorCode string) encodeFunc {
	if enc, ok := orderFileEncoders[distributorCode]; ok {
		return enc
	}
	return defaultEncode
}

// lineNo は明細の行番号（1始まり）を返す。
// 行番号はDBに保存しておらず、発注ファイルを書く時点の明細の並び順で振る。
func lineNo(i int) int { return i + 1 }

func checkWidth(name, value string, width int) error {
	if len(value) > width {
		return fmt.Errorf("%sが固定長の桁数(%d)を超えています: %q", name, width, value)
	}
	return nil
}
