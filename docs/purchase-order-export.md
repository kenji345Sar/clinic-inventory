# 発注ファイル送付の設計 — 確定した発注を卸ごとの形式で送る

クリニックが確定した発注を、卸ごとの形式のファイルにしてS3へ置くまでの設計。

**未実装。** 現在は「発注する」のリクエストの中でCSVを作ってS3へ上げ、そのあと確定をDBに保存している(2章)。
このドキュメントは、目指す形(1章・3章以降)と、そこへ移す作業の順番(8章)を決めるためのもの。決まっていない点は「未決」と明記する。

- [purchase-order-flow.md](purchase-order-flow.md) … 現在の「カートに追加 → 発注する → S3」の流れをコードで追ったもの
- `clinic-inventory-csv-functions` の `docs/s3-storage.md` … S3バケット・IAMの設定とキーの規約
- `clinic-inventory-csv-functions` の `docs/order-acknowledgement-import.md` … 送った発注に対して卸から返ってくる受注確定の取り込み。発注No・行番号で突き合わせるため、この設計と噛み合わせる必要がある

---

## 1. 全体像

```
[「発注する」のリクエスト]
  ConfirmPurchaseOrderUseCase ─→ DB: 発注を確定(status=confirmed)、送付状態=未送付
                                                     │
[送付処理(同じ backend の別処理)]                      │
  未送付の発注をDBから読む ←─────────────────────────┘
    → 中間ファイル(全卸共通の形式)を作り、S3に控えとして置く
    → 卸コードで書き方を選び、卸別ファイルを作る      ← 卸ごとの差はここだけ
    → S3の orders/{卸コード}/… に置く
    → DB: 送付状態=送付済み(失敗したら 送付失敗 と理由)
```

| 段 | いつ動くか | やること | 卸の形式を知っているか |
|---|---|---|---|
| 発注確定 | クリニックが「発注する」を押したとき | 確定と「未送付」をDBに保存するだけ。S3には触らない | 知らない |
| 送付処理 | 確定のあと、別に動く(起動方法は未決) | 未送付の発注を読み、中間ファイル → 卸別ファイル → S3 → 送付状態を更新 | 書き方の関数だけが知っている |

どちらも clinic-inventory の backend で動く。

---

## 2. 現状 — 今のまま卸が増えるとどうなるか

### 2-1. 今の発注確定の順序

[confirm_purchase_order.go](../backend/internal/application/procurement/confirm_purchase_order.go) の `Execute` は、1回のリクエストの中で次の順に動く。

1. DBから下書きの発注を読む(カートに追加した時点でDBに入っている)
2. 明細ごとに卸商品をDBから引き、CSVの行を組み立てる
3. 発注を確定状態にする(メモリ上だけ)
4. CSVを作ってS3にアップロードする
5. アップロードが成功したら、確定をDBに保存する

### 2-2. 今の形の問題

| 項目 | 現状 | 起きること |
|---|---|---|
| 形式 | 全卸共通。日本語ヘッダ9列、UTF-8、改行LF | 卸が複数社になると、フォルダは卸ごとに分かれるが中身はどの卸にも同じCSVが届く。別の列構成・文字コード・固定長を求める卸は読めない |
| S3とDBの順序 | S3に上げてからDBに保存 | S3は成功したのにDB保存が失敗すると、卸にCSVが届いているのに発注は下書きのまま残る。クリニックがもう一度「発注する」を押すと、**同じ発注が卸に2回届く** |
| S3の失敗 | 発注確定そのものが失敗する | 通信エラーや形式の誤りといった、クリニックには直せない原因で発注できなくなる |
| 行番号 | 無い。明細はUUIDの主キーで持つだけで並び順も保存していない | 卸から「○行目は欠品」と返ってきても、どの明細か突き合わせられない |
| 卸向けの発注No | 無い。発注ID(UUID)をそのまま使う | 独自の番号を求める卸(A社)に対応できない |
| 卸側の取得 | 卸にS3のアクセス権を渡していない | 実際には卸はファイルを取得できない |

---

## 3. 方針と理由

### 3-1. 発注確定ではDBに保存するだけにし、送付は別の処理で行う

- **二重発注の穴が塞がる。** 先に確定をDBに保存し、「未送付」の発注を後から送る。送付に失敗しても同じ発注を送り直すだけで、発注が増えることはない
- **S3の不調でクリニックの発注が止まらない。** 確定は通し、送付の失敗は発注ごとに記録して直す

代わりに「発注済みだが卸にはまだ届いていない」時間ができる。そのため送付状態を発注に持たせ、画面で見られるようにする(4章)。

### 3-2. 送付処理は clinic-inventory の backend で行う

送る元のデータ(発注・明細・卸商品)も、送付状態を記録する列も backend のテーブルにあり、それを読み書きするコード(リポジトリ)も backend にある。
別リポジトリ(`clinic-inventory-csv-functions`)で送付を行うと、発注を読むコードをそちらにも書くことになり、業務ルールを2か所に持つことになる。

### 3-3. 中間ファイルを挟む

発注データから卸ごとの形式を直接書かず、いったん全卸共通の中間ファイルにしてから書き直す。

- **卸ごとの差を書き方の関数1か所に閉じられる。** DBを読む・S3に置く・状態を更新する処理は全卸共通になり、卸を足すときは書き方の関数を1つ書くだけで済む。取り込み側(`clinic-inventory-csv-functions`)が「卸ごとの形式 → 中間表現 → 共通の後段」としているのと逆向きの同じ形
- **何を送ったかが残る。** 送付の後で卸商品名などのマスタが変わっても、中間ファイルを見れば送った時点の内容が分かる

---

## 4. 送付状態

発注(`purchase_orders`)に送付状態を持たせる。対象は確定済みの発注だけで、下書きは持たない。

| 列 | 意味 |
|---|---|
| `export_status` | `pending`(未送付)/ `exported`(送付済み)/ `failed`(送付失敗) |
| `exported_at` | 送付済みになった日時 |
| `export_error` | 送付失敗の理由。失敗していなければNULL |

```
確定 ─→ pending ─→ exported
            │
            └──→ failed ─(原因を直して pending に戻す)─→ pending
```

- 確定の時点で `pending` にする
- 送付処理が成功したら `exported`、失敗したら `failed` と理由を保存する
- 送り直すときは `pending` に戻す。卸別ファイルは同じS3キーに上書きされるだけなので、何度送り直してもファイルは増えない
- この変更より前に確定した発注は、今の仕組みで既にS3に上がっているので `exported` として扱う

クリニックの発注履歴には、発注日時に加えて送付状態を出す。`failed` の発注は目立つように表示する。

---

## 5. 中間ファイルの仕様

### 5-1. 置き場所

```
order-exports/{卸コード}/{facilityId}/{orderId}.json
```

卸に見せる `orders/` とは別のプレフィックスにする。卸にS3の読み取り権限を渡したとき、共通形式の控えまで卸に見せないため。

### 5-2. 形式

UTF-8のJSON、1発注1ファイル。

```json
{
  "schemaVersion": 1,
  "orderId": "11111111-2222-3333-4444-555555555555",
  "distributorOrderNo": null,
  "orderedAt": "2026-09-13T10:00:00+09:00",
  "facility": {
    "id": "38057321-88d6-4a6c-8820-0da36fdd3766",
    "name": "テスト医院",
    "distributorFacilityCode": null
  },
  "distributor": {
    "id": "…",
    "code": "oroshi-b"
  },
  "lines": [
    { "lineNo": 1, "distributorProductCode": "B-001", "productName": "商品1", "quantity": 3, "unitPrice": 100, "amount": 300 },
    { "lineNo": 2, "distributorProductCode": "B-002", "productName": "商品2", "quantity": 10, "unitPrice": 50, "amount": 500 }
  ]
}
```

| 項目 | 意味 | 今のデータにあるか |
|---|---|---|
| `schemaVersion` | 中間ファイルの形式の版。項目を足したとき、古いファイルと読み分けるため | — |
| `orderId` | 自社の発注ID(UUID) | ある |
| `distributorOrderNo` | 卸に指定された形式の発注No。A社のように独自の番号を求める卸だけで使う | **無い**(未決) |
| `orderedAt` | 発注確定日時 | ある(`purchase_orders.confirmed_at`) |
| `facility.distributorFacilityCode` | 卸側の医院コード | **無い**(医院コードの対応表が未実装) |
| `lines[].lineNo` | 行番号(1始まり) | **無い**(5-3) |
| その他 | 卸商品コード・商品名・数量・単価・金額 | ある |

中間ファイルは**卸に送る可能性のある値を全部持つ**。どの値を使うかは卸別の書き方が決める。
卸が中間ファイルに無い値を求めてきたら、中間ファイルに項目を足し、`schemaVersion` を上げる。

### 5-3. 行番号

行番号は発注明細に保存する(`purchase_order_lines.line_no`)。ファイルを書く時点で振るだけにはしない。

振るだけだと、卸から返ってくる受注確定の「発注No ○○ の2行目」がどの明細か突き合わせられない。
番号はカートに追加した順に振り、確定後は変えない。

---

## 6. 卸別ファイルの仕様

置き場所は3社とも `orders/{卸コード}/{facilityId}/{orderId}{拡張子}`。

| | A社 `oroshi-a` | B社 `oroshi-b` | C社 `oroshi-c` |
|---|---|---|---|
| 形式 | CSV | CSV | 固定長 |
| 文字コード | UTF-8 | UTF-8 | Shift_JIS |
| 改行 | CR(要確認) | CR(要確認) | CRLF(仮) |
| ヘッダ行 | 無し(仮) | 無し(仮) | 無し(仮) |
| 発注No | **卸の番号体系の20桁**(未決) | 自社の発注ID(UUID)のまま | 自社の発注ID(UUID)のまま |
| 列 | 発注No, 行番号, 商品コード, 数量 | 発注No, 行番号, 商品コード, 数量 | 発注No, 行番号, 商品コード, 数量 |
| 拡張子 | `.csv` | `.csv` | `.txt`(仮) |

### 6-1. C社の固定長レコード(仮)

| 項目 | 桁(バイト) | 詰め方 |
|---|---|---|
| 発注No | 36 | 左詰め |
| 行番号 | 4 | 右詰め、前ゼロ |
| 商品コード | 20 | 左詰め、後ろをスペース |
| 数量 | 6 | 右詰め、前ゼロ |

1レコード66バイト。桁はShift_JISに変換した後のバイト数で数える(全角は1文字2バイト)。
**桁あふれは切り詰めずに送付失敗にする。** 黙って切ると、卸側で別の商品コードとして読まれるため。

### 6-2. 書き方が決まっていない卸

書き方の対応表に無い卸の発注は、**送付失敗にする**。
今の9列CSVのような既定の形式で送ると、卸が読めないファイルが「送付済み」に見えてしまうため。

---

## 7. ソースの形

### 7-1. パッケージの分け方

送付は「発注を作る(カート・確定・取消)」とは別の仕事なので、**`orderexport` という別のパッケージに分ける**。
`application/procurement/` に足すと、発注の操作と送付の部品が1つのフォルダに混ざり、どれが発注の操作なのか見分けにくくなるため。

| パッケージ | 中身 | ファイル数 |
|---|---|---|
| `application/procurement/` | 発注の操作(カートに追加・確定・取消) | 今の4 → **3**(CSV用の型を消す) |
| `application/orderexport/` | 送付のユースケースと、中間ファイルの型 | 新規 **2** |
| `infrastructure/orderexport/format/` | 卸ごとのファイルの形式(アダプタ) | 新規(卸の数+対応表) |
| `infrastructure/orderexport/sender/` | S3への送信 | 新規 1 |

送付状態そのもの(未送付・送付済み・失敗)は発注の状態なので、ドメインは `domain/procurement/` の発注に持たせる。

```
backend/
  cmd/api/main.go                             … 配線。送付処理の起動もここ(変更)
  internal/
    domain/procurement/
      purchase_order.go                       … 送付状態と、送付済み/失敗にするメソッド。明細の行番号(変更)
      repository.go                           … 未送付の発注を探すメソッドを追加(変更)

    application/procurement/                  … 発注の操作だけ
      save_draft_purchase_order.go            … カートに追加(変更なし)
      confirm_purchase_order.go               … 確定。S3を外し、保存だけにする(変更)
      remove_draft_purchase_order.go          … 取消(変更なし)
      purchase_order_csv.go                   … 削除

    application/orderexport/                  … 【新規】送付
      export_purchase_order.go                …   送付のユースケース。一連の流れはここだけ読めば分かる
      document.go                             …   中間ファイルの型と、ユースケースが外に頼む操作(インターフェース2つ)

    infrastructure/procurement/
      model.go                                … 送付状態・行番号の列を追加(変更)
      purchase_order_repository.go            … 未送付の発注を探すクエリを追加(変更)
      purchase_order_csv_uploader.go          … 削除

    infrastructure/orderexport/               … 【新規】送付の外側
      format/                                 …   卸ごとのファイルの形式(アダプタ)。中間ファイル → 卸別ファイル
        encoder.go                            …     卸コード → 書き方の関数 の対応表
        oroshi_a.go                           …     ★A社: UTF-8 CSV、改行CR、20桁の発注No
        oroshi_b.go                           …     ★B社: UTF-8 CSV、改行CR
        oroshi_c.go                           …     ★C社: Shift_JIS 固定長
        csv.go                                …     A社・B社で共通のCSV書き出し(改行CR)
        encoder_test.go                       …     卸ごとの出力をバイト単位で確かめる
      sender/                                 …   S3への送信。形式は知らない
        sender.go                             …     S3のキーを組み立て、storage.S3Uploader で置く

    infrastructure/storage/
      s3_uploader.go                          … 既存。キーと中身を受け取ってS3に置くだけの汎用部品(変更なし)
```

infrastructure は2つのフォルダで役割が分かれる。

| フォルダ | やること | 知っていること | 知らないこと |
|---|---|---|---|
| `format/` | 中間ファイルを卸ごとの形式のバイト列にする | 卸ごとの列・文字コード・改行・固定長 | S3、キーの規約 |
| `sender/` | できたファイルと中間ファイルをS3のどのキーに置くか決めて置く | S3のキーの規約(`orders/`・`order-exports/`) | 卸ごとの形式 |

**卸ごとの形式を知っているのは `format/` だけ**で、ユースケース・ドメイン・DB・`sender/` はどの卸の形式も知らない。
新しい卸に対応するときは、`format/` に `oroshi_<卸>.go` を1つ足し、`encoder.go` の対応表に1行足す。`sender/` は触らない。
逆に、S3のキーの規約を変えるときや、送り先をS3以外(SFTPなど)にするときは `sender/` だけを変え、`format/` は触らない。

### 7-2. 送付のユースケース — 一連の流れ

`application/orderexport/export_purchase_order.go`。このファイル1つに、発注1件を送るユースケースと、未送付をまとめて送るユースケースを置く。
発注1件を送る `Execute` は、上から順に読めば送付の手順がそのまま分かるように、手順ごとに1ブロックにする。

```go
package orderexport

// ExportPurchaseOrderUseCase は確定済み・未送付の発注1件を、卸ごとの形式のファイルにしてS3へ置く。
type ExportPurchaseOrderUseCase struct {
	purchaseOrderRepo      procdomain.PurchaseOrderRepository
	distributorRepo        distdomain.DistributorRepository
	distributorProductRepo distdomain.DistributorProductRepository
	clinicProductRepo      proddomain.ClinicProductRepository
	facilityRepo           orgdomain.FacilityRepository
	encoder                Encoder // 中間ファイル → 卸別ファイル(infrastructure/orderexport/format/)
	sender                 Sender  // S3に置く(infrastructure/orderexport/sender/)
}

func (uc *ExportPurchaseOrderUseCase) Execute(ctx context.Context, orderID shareddomain.ID) error {
	// 手順1: 発注を読む。未送付でなければ何もしない(二重に送らない)
	order, err := uc.purchaseOrderRepo.FindByID(ctx, orderID)
	if err != nil {
		return err
	}
	if order.ExportStatus() != procdomain.ExportPending {
		return nil
	}

	// 手順2: 中間ファイルに載せる値を集める
	facility, err := uc.facilityRepo.FindByID(ctx, order.FacilityID())
	// …(エラー処理は省略)
	distributor, err := uc.distributorRepo.FindByID(ctx, order.DistributorID())
	lines := make([]Line, 0, len(order.Lines()))
	for _, line := range order.Lines() {
		clinicProduct, err := uc.clinicProductRepo.FindByID(ctx, line.ClinicProductID())
		distributorProduct, err := uc.distributorProductRepo.FindByID(ctx, clinicProduct.DistributorProductID())
		lines = append(lines, Line{
			LineNo:                 line.LineNo(),
			DistributorProductCode: distributorProduct.DistributorProductCode(),
			ProductName:            distributorProduct.Name(),
			Quantity:               line.Quantity(),
			UnitPrice:              line.UnitPrice(),
		})
	}
	doc := NewDocument(order, facility, distributor, lines)

	// 手順3: 中間ファイルをS3に控えとして置く
	if err := uc.sender.PutDocument(ctx, doc); err != nil {
		return uc.fail(ctx, order, err)
	}

	// 手順4: 卸ごとの形式のファイルを作る(どの形式になるかは卸コードで決まる)
	file, err := uc.encoder.Encode(doc)
	if err != nil {
		return uc.fail(ctx, order, err)
	}

	// 手順5: 卸別ファイルをS3に置く
	if err := uc.sender.PutDistributorFile(ctx, doc, file); err != nil {
		return uc.fail(ctx, order, err)
	}

	// 手順6: 送付済みにして保存する
	order.MarkExported(time.Now())
	return uc.purchaseOrderRepo.Update(ctx, order)
}

// fail は送付失敗と理由を発注に保存する。発注の確定は取り消さない。
func (uc *ExportPurchaseOrderUseCase) fail(ctx context.Context, order *procdomain.PurchaseOrder, cause error) error {
	order.MarkExportFailed(cause.Error())
	if err := uc.purchaseOrderRepo.Update(ctx, order); err != nil {
		return err
	}
	return cause
}
```

手順2は、今の `application/procurement/confirm_purchase_order.go` の手順2(CSVの明細を組み立てる)をそのまま移したもの。

同じファイルの下に、未送付の発注を探して1件ずつ上の `Execute` に渡すユースケースを置く。1件失敗しても残りは続ける。

```go
// ExportPendingPurchaseOrdersUseCase は未送付の発注をすべて送る。
func (uc *ExportPendingPurchaseOrdersUseCase) Execute(ctx context.Context) error {
	orders, err := uc.purchaseOrderRepo.FindPendingExports(ctx)
	if err != nil {
		return err
	}
	for _, order := range orders {
		if err := uc.exportOne.Execute(ctx, order.ID()); err != nil {
			log.Printf("発注 %s の送付に失敗しました: %v", order.ID(), err) // 理由は発注にも保存済み
		}
	}
	return nil
}
```

これを誰がいつ呼ぶか(確定の直後か、一定間隔か)は未決(9章)。どちらにしても `cmd/api/main.go` から呼ぶ。

### 7-3. 中間ファイルの型と、外に頼む操作

`application/orderexport/document.go`。中間ファイル(5-2のJSON)に対応する型と、ユースケースが infrastructure に頼む操作を置く。
ユースケース(application)は infrastructure を直接呼べないため、今の `PurchaseOrderCsvUploader` と同じくインターフェースにする。

```go
// Document は中間ファイル1本。JSONにするとそのまま5-2の形になる。
type Document struct {
	SchemaVersion      int       `json:"schemaVersion"`
	OrderID            string    `json:"orderId"`
	DistributorOrderNo *string   `json:"distributorOrderNo"`
	OrderedAt          time.Time `json:"orderedAt"`
	Facility           Facility  `json:"facility"`
	Distributor        Distributor `json:"distributor"`
	Lines              []Line    `json:"lines"`
}

// Encoder は中間ファイルを、卸ごとの形式のファイルに書き直す。
type Encoder interface {
	Encode(doc Document) (DistributorFile, error)
}

// Sender は送付に使うファイルをS3に置く。キーの組み立てもここが持つ。
type Sender interface {
	PutDocument(ctx context.Context, doc Document) error                           // order-exports/{卸コード}/{facilityId}/{orderId}.json
	PutDistributorFile(ctx context.Context, doc Document, f DistributorFile) error // orders/{卸コード}/{facilityId}/{orderId}{拡張子}
}

// DistributorFile は卸ごとの形式で書き出したファイル。拡張子と Content-Type も卸で違う。
type DistributorFile struct {
	Body        []byte
	Ext         string
	ContentType string
}
```

### 7-4. 発注確定のユースケース — 何が消えるか

`application/procurement/confirm_purchase_order.go` は、S3・CSV・卸・クリニック・卸商品に一切触らなくなる。

```go
// 変更後
func (uc *ConfirmPurchaseOrderUseCase) Execute(ctx context.Context, in ConfirmPurchaseOrderInput) (*procdomain.PurchaseOrder, error) {
	// 手順1: 発注を取得し、URLのfacilityIdと一致するか確認する(今と同じ)
	order, err := uc.purchaseOrderRepo.FindByID(ctx, in.OrderID)
	if err != nil {
		return nil, err
	}
	if order.FacilityID() != in.FacilityID {
		return nil, fmt.Errorf("発注が見つかりません: %w", shareddomain.ErrNotFound)
	}

	// 手順2: 確定する。送付状態は Confirm の中で「未送付」になる
	if err := order.Confirm(time.Now()); err != nil {
		return nil, err
	}

	// 手順3: 保存する
	if err := uc.purchaseOrderRepo.Update(ctx, order); err != nil {
		return nil, err
	}
	return order, nil
}
```

今の手順2(CSVの明細を組み立てる)は7-2へ移り、手順4(S3へアップロード)は7-2の手順3〜5に置き換わる。
ユースケースが受け取るリポジトリも `purchaseOrderRepo` の1つだけになる。

### 7-5. ドメイン — 発注に足すもの

`domain/procurement/purchase_order.go`。送付状態を変えるのはこのメソッドだけにする。

```go
type ExportStatus string

const (
	ExportPending  ExportStatus = "pending"  // 未送付
	ExportExported ExportStatus = "exported" // 送付済み
	ExportFailed   ExportStatus = "failed"   // 送付失敗
)

// Confirm は今の処理に加えて、送付状態を「未送付」にする。
func (o *PurchaseOrder) Confirm(confirmedAt time.Time) error { … o.exportStatus = ExportPending … }

func (o *PurchaseOrder) MarkExported(at time.Time) { … }      // 未送付 → 送付済み
func (o *PurchaseOrder) MarkExportFailed(reason string) { … } // 未送付 → 送付失敗(理由を持つ)
func (o *PurchaseOrder) RetryExport() error { … }              // 送付失敗 → 未送付(送り直し)
```

明細(`OrderLine`)には行番号 `lineNo` を足す。`AddLine` で新しい明細を足すときに、今ある明細数+1を振る(同じ商品の数量加算では振り直さない)。

### 7-6. 卸ごとの形式 — `infrastructure/orderexport/format/`

`format/encoder.go` が対応表を持ち、7-3の `Encoder` を実装する。

```go
package format

// 卸コード → 書き方の関数。新しい卸はここに1行足す。
var encoders = map[string]func(expapp.Document) (expapp.DistributorFile, error){
	"oroshi-a": encodeOroshiA,
	"oroshi-b": encodeOroshiB,
	"oroshi-c": encodeOroshiC,
}

func (Encoder) Encode(doc expapp.Document) (expapp.DistributorFile, error) {
	encode, ok := encoders[doc.Distributor.Code]
	if !ok {
		return expapp.DistributorFile{}, fmt.Errorf("卸コード %s の発注ファイルの書き方がありません。oroshi_<卸>.go を作って encoders に登録してください", doc.Distributor.Code)
	}
	return encode(doc)
}
```

卸ごとのファイルは、中間ファイルを受け取り、その卸の列だけを書く関数1つ。例えばB社:

```go
// oroshi_b.go — B社: UTF-8 CSV、改行CR、ヘッダなし。列: 発注No(自社の発注ID), 行番号, 商品コード, 数量
func encodeOroshiB(doc expapp.Document) (expapp.DistributorFile, error) {
	buf := &bytes.Buffer{}
	for _, l := range doc.Lines {
		row := []string{doc.OrderID, strconv.Itoa(l.LineNo), l.DistributorProductCode, strconv.Itoa(l.Quantity)}
		if err := writeCsvRecordCR(buf, row); err != nil {
			return expapp.DistributorFile{}, err
		}
	}
	return expapp.DistributorFile{Body: buf.Bytes(), Ext: ".csv", ContentType: "text/csv; charset=UTF-8"}, nil
}
```

C社(`oroshi_c.go`)は同じ形で、各項目を6-1の桁に詰めてShift_JISに変換する。A社(`oroshi_a.go`)は発注Noに `doc.DistributorOrderNo` を使う。

S3への送信は `sender/sender.go` に分かれていて、ここには出てこない。

```go
// sender/sender.go — キーを組み立てて置くだけ。どの卸の形式かは知らない
func (s *Sender) PutDistributorFile(ctx context.Context, doc expapp.Document, f expapp.DistributorFile) error {
	key := fmt.Sprintf("orders/%s/%s/%s%s", doc.Distributor.Code, doc.Facility.ID, doc.OrderID, f.Ext)
	return s.uploader.Put(ctx, key, f.Body, f.ContentType)
}
```

今 backend に試作してある `infrastructure/procurement/order_file_*.go` は、入力を中間ファイルの型に変え、`infrastructure/orderexport/format/` に移す。

### 7-7. 間接参照の場所と辿り方

コードを上から読むだけでは呼び先が分からない箇所は次の3つ。

| 箇所 | 呼び先 | 辿り方 |
|---|---|---|
| 7-2の `uc.encoder.Encode` | `infrastructure/orderexport/format/encoder.go` の `Encode` | `cmd/api/main.go` で `NewExportPurchaseOrderUseCase` に何を渡しているかを見る |
| `Encode` の中の `encoders[…]` | `infrastructure/orderexport/format/oroshi_<卸>.go` の関数 | `encoder.go` の対応表に書かれた関数名へ飛ぶ |
| 7-2の `uc.sender.Put…` | `infrastructure/orderexport/sender/sender.go` | `cmd/api/main.go` の配線を見る |

1つめと3つめは、今の `PurchaseOrderCsvUploader` と同じ仕組み(application に置いたインターフェースを infrastructure が実装し、main.go で渡す)。
2つめの対応表は、卸ごとに別の書き方が確実に増えるため(今の時点で3社)に置く。

---

## 8. 作業の順番

| 手順 | やること | 確かめること |
|---|---|---|
| 1 | 発注明細に行番号を保存する(`line_no`、カート追加順に採番) | カートに追加した順に1,2,3と保存され、確定後に変わらない |
| 2 | 発注に送付状態の列を足す(4章)。既に確定済みの発注は `exported` にする | 既存の確定済み発注がすべて `exported` になっている |
| 3 | 送付処理を作る。未送付の発注を読み、中間ファイル → B社・C社の卸別ファイル → S3 → 送付状態を更新 | B社はCR区切りのCSV、C社は66バイト×明細数のShift_JISになる。桁あふれや書き方の無い卸は `failed` と理由が残る |
| 4 | 発注確定のユースケースからS3アップロードを外し、確定と `pending` の保存だけにする。あわせて送付処理の起動を組み込む | 「発注する」でS3に何も起きず、送付処理が動いたあとに `orders/` にファイルが出る |
| 5 | 発注履歴に送付状態を出す | `failed` の発注が画面で分かる |
| 6 | A社の書き方を入れる(発注Noの決定後) | 20桁の発注Noで出る |

- **手順3と4は同時に出す。** 4だけ先に入れると、確定した発注がどこにも送られなくなる
- 手順1〜3は未決事項に左右されずに進められる
- backend で卸別の書き方を試した未コミットのコード(`backend/internal/infrastructure/procurement/order_file_*.go`)は、手順3で中間ファイルを入力に受け取る形へ手直しし、`infrastructure/orderexport/format/` に移す
- 手順4を入れたら、[purchase-order-flow.md](purchase-order-flow.md) の「発注CSVが卸に届いて初めて確定」という説明と、`clinic-inventory-csv-functions` の `docs/s3-storage.md` のキー規約(`order-exports/` の追加)を書き換える

---

## 9. 未決

| 項目 | 内容 |
|---|---|
| 送付処理の起動方法 | 確定の直後に同じプロセスの中ですぐ動かすか、一定の間隔でDBの未送付を見に行くか、その両方か。すぐ動かすだけだと、backend が落ちたときに `pending` のまま取り残された発注を拾えない |
| 失敗時の自動再試行 | 通信エラーのような一時的な失敗を自動で送り直すか、人が `pending` に戻すまで待つか |
| A社の発注No(20桁) | こちらがA社の決まりに沿って番号を振るのか、A社が払い出した番号を使うのか。前者なら20桁の中身の決まりと、振った番号の保存先が要る |
| 改行コードCR | A社・B社の改行がCRのみか、CRLFの意味か |
| C社の桁数・改行・拡張子 | 6-1の桁数、CRLF、`.txt` はすべて仮 |
| ヘッダ行の有無 | 3社とも無しで仮置き |
| 卸の取得方法 | 卸に `orders/{卸コード}/` の読み取り権限を渡すか、SFTP・メールなど別の手段で送るか。現在は卸にアクセス権が無い |
| 卸側の医院コード | 中間ファイルの `distributorFacilityCode` の出どころ。医院コードの対応表が未実装 |
