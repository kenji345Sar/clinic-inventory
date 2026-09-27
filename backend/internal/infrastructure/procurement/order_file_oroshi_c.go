package procurement

import (
	"bytes"
	"fmt"

	"golang.org/x/text/encoding/japanese"
)

// C社（oroshi-c）向けの固定長レコードの桁数（バイト数）。
const (
	oroshiCOrderNoWidth     = 36 // 発注No: 自社の発注ID(UUID)のまま。左詰め
	oroshiCLineNoWidth      = 4  // 行番号: 右詰め、前ゼロ
	oroshiCProductCodeWidth = 20 // 商品コード: 左詰め、後ろをスペースで埋める
	oroshiCQuantityWidth    = 6  // 数量: 右詰め、前ゼロ
)

// encodeOroshiC はC社（oroshi-c）向け。Shift_JISの固定長、1明細1レコード、改行CRLF、ヘッダ行なし。
// レコード長は 36+4+20+6 = 66バイト。
//
// 桁数はShift_JISに変換した後のバイト数で数える（全角文字は1文字2バイト）。
// 桁あふれは切り詰めずエラーにする。黙って切ると卸側で別の商品コードとして読まれるため。
func encodeOroshiC(in orderFileInput) (orderFile, error) {
	enc := japanese.ShiftJIS.NewEncoder()
	buf := &bytes.Buffer{}
	orderNo := in.Order.ID().String()

	for i, l := range in.Lines {
		code, err := enc.String(l.DistributorProductCode)
		if err != nil {
			return orderFile{}, fmt.Errorf("商品コード %q をShift_JISに変換できません: %w", l.DistributorProductCode, err)
		}
		lineNoStr := fmt.Sprintf("%0*d", oroshiCLineNoWidth, lineNo(i))
		qtyStr := fmt.Sprintf("%0*d", oroshiCQuantityWidth, l.Quantity)

		for _, c := range []struct {
			name  string
			value string
			width int
		}{
			{"発注No", orderNo, oroshiCOrderNoWidth},
			{"行番号", lineNoStr, oroshiCLineNoWidth},
			{"商品コード", code, oroshiCProductCodeWidth},
			{"数量", qtyStr, oroshiCQuantityWidth},
		} {
			if err := checkWidth(c.name, c.value, c.width); err != nil {
				return orderFile{}, fmt.Errorf("C社向け発注ファイル %d行目: %w", lineNo(i), err)
			}
		}

		fmt.Fprintf(buf, "%-*s%s%-*s%s\r\n",
			oroshiCOrderNoWidth, orderNo,
			lineNoStr,
			oroshiCProductCodeWidth, code,
			qtyStr,
		)
	}
	return orderFile{Body: buf.Bytes(), Ext: ".txt", ContentType: "text/plain; charset=Shift_JIS"}, nil
}
