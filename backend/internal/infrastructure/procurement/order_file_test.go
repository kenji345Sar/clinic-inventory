package procurement

import (
	"bytes"
	"strings"
	"testing"
	"time"

	procapp "clinic-inventory/internal/application/procurement"
	procdomain "clinic-inventory/internal/domain/procurement"
	shareddomain "clinic-inventory/internal/domain/shared"

	"github.com/google/uuid"
	"golang.org/x/text/encoding/japanese"
)

func testOrderInput(t *testing.T, lines ...procapp.PurchaseOrderCsvLine) orderFileInput {
	t.Helper()
	id := shareddomain.ID(uuid.MustParse("11111111-2222-3333-4444-555555555555"))
	order := procdomain.ReconstructPurchaseOrder(id, shareddomain.NewID(), shareddomain.NewID(), procdomain.StatusConfirmed, nil, nil)
	return orderFileInput{
		Order:        order,
		FacilityName: "テスト医院",
		OrderedAt:    time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC),
		Lines:        lines,
	}
}

func TestResolveEncoder_表に無い卸は既定の9列CSV(t *testing.T) {
	in := testOrderInput(t, procapp.PurchaseOrderCsvLine{DistributorProductCode: "P1", ProductName: "商品1", Quantity: 2, UnitPrice: 100})
	f, err := resolveEncoder("sample-pharma")(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(f.Body), "発注ID,発注日,") || f.Ext != ".csv" {
		t.Errorf("既定の形式になっていない: ext=%s body=%q", f.Ext, f.Body)
	}
}

func TestEncodeOroshiB(t *testing.T) {
	in := testOrderInput(t,
		procapp.PurchaseOrderCsvLine{DistributorProductCode: "B-001", Quantity: 3},
		procapp.PurchaseOrderCsvLine{DistributorProductCode: "B-002", Quantity: 10},
	)
	f, err := resolveEncoder("oroshi-b")(in)
	if err != nil {
		t.Fatal(err)
	}
	want := "11111111-2222-3333-4444-555555555555,1,B-001,3\r" +
		"11111111-2222-3333-4444-555555555555,2,B-002,10\r"
	if string(f.Body) != want {
		t.Errorf("got  %q\nwant %q", f.Body, want)
	}
	if bytes.Contains(f.Body, []byte("\n")) {
		t.Error("改行がCRだけになっていない（LFが含まれている）")
	}
}

func TestEncodeOroshiC(t *testing.T) {
	in := testOrderInput(t,
		procapp.PurchaseOrderCsvLine{DistributorProductCode: "C-001", Quantity: 3},
		procapp.PurchaseOrderCsvLine{DistributorProductCode: "ｱｲｳ全角", Quantity: 120},
	)
	f, err := resolveEncoder("oroshi-c")(in)
	if err != nil {
		t.Fatal(err)
	}
	records := bytes.Split(bytes.TrimSuffix(f.Body, []byte("\r\n")), []byte("\r\n"))
	if len(records) != 2 {
		t.Fatalf("レコード数 = %d, want 2", len(records))
	}
	for i, r := range records {
		if len(r) != 66 {
			t.Errorf("%d行目のレコード長 = %dバイト, want 66", i+1, len(r))
		}
	}
	if got := string(records[0]); got != "11111111-2222-3333-4444-555555555555"+"0001"+"C-001               "+"000003" {
		t.Errorf("1行目 = %q", got)
	}
	// 2行目: 半角ｶﾅ3文字(3バイト)+全角2文字(4バイト)=7バイトとして桁埋めされ、Shift_JISで読み戻せる
	decoded, err := japanese.ShiftJIS.NewDecoder().Bytes(records[1])
	if err != nil {
		t.Fatal(err)
	}
	if want := "11111111-2222-3333-4444-555555555555" + "0002" + "ｱｲｳ全角" + strings.Repeat(" ", 13) + "000120"; string(decoded) != want {
		t.Errorf("2行目 = %q, want %q", decoded, want)
	}
}

func TestEncodeOroshiC_桁あふれはエラー(t *testing.T) {
	in := testOrderInput(t, procapp.PurchaseOrderCsvLine{DistributorProductCode: strings.Repeat("X", 21), Quantity: 1})
	if _, err := resolveEncoder("oroshi-c")(in); err == nil {
		t.Error("商品コード21バイトでエラーにならない")
	}
	in = testOrderInput(t, procapp.PurchaseOrderCsvLine{DistributorProductCode: "C-001", Quantity: 1000000})
	if _, err := resolveEncoder("oroshi-c")(in); err == nil {
		t.Error("数量7桁でエラーにならない")
	}
}
