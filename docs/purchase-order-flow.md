# 発注フロー — カート追加から確定・S3アップロードまで

発注画面（`/facilities/:id/orders`）でボタンを押してから、卸に渡す発注CSVがS3に置かれるまでを追う。
この画面は**1回の操作では発注が完了しない**。「カートに追加」で下書きを作り、「発注する」で確定する
2段階なので、往復も2回に分かれ、**外部サービス(S3)への副作用は2回目にだけ**起きる。

1リクエストがhandler → ユースケース → SQLに届く基本形は
[go/request-to-sql-flow.md](go/request-to-sql-flow.md) の1〜2章にある。S3のバケット構成や
認証情報の詳細は別リポジトリ`clinic-inventory-csv-functions`の`docs/s3-storage.md`を参照。

| 画面の操作 | HTTPメソッド + パス | handler | ユースケース | DBに起きること |
|---|---|---|---|---|
| カートに追加 | `POST /api/facilities/{facilityId}/orders` | `postOrder` | `SaveDraftPurchaseOrderUseCase` | 下書きをINSERT、既にあればUPDATE |
| 発注する | `POST /api/facilities/{facilityId}/orders/{orderId}/confirm` | `confirmOrderHandler` | `ConfirmPurchaseOrderUseCase` | S3送信の**後**にUPDATE（`status='confirmed'`） |
| 取消 | `DELETE /api/facilities/{facilityId}/orders/{orderId}` | `deleteOrder` | `RemoveDraftPurchaseOrderUseCase` | 下書きをDELETE |

「1発注 = 1卸」なので、カートは卸業者ごとに別々の下書き（= 別々の`purchase_orders`行）になる。
画面の「カート」欄に卸ごとのカードが並び、それぞれに「発注する」ボタンが付くのはそのため。

## 1. 画面: 1つの画面から3種類のAPIを撃ち分ける

```tsx
// frontend/app/routes/facility-orders.tsx
// 上段: 卸ごとの数量入力フォーム → カートに積む
<Form method="post">
  <input type="hidden" name="intent" value="addToCart" />
  <input type="hidden" name="distributorId" value={group.distributorId} />
  <input name={`qty_${p.id}`} type="number" ... />
  <button type="submit">カートに追加</button>
</Form>

// 下段: カート内の下書き1件ごとの確定・取消（intentはbutton自身のname/valueで送る）
<Form method="post">
  <input type="hidden" name="orderId" value={order.id} />
  <button type="submit" name="intent" value="confirm">発注する</button>
</Form>
```

React Routerは1つの画面（route）に`action`を1つしか持てないので、どのボタンが押されたかを
`intent`という隠しフィールドで見分ける。C#なら「1つのControllerに複数のPOSTアクションを生やす」
ところを、1つの`action`関数の中で分岐している、と読み替えると分かりやすい。

## 2. フロント: actionがintentで分岐して別々のAPIを呼ぶ

```ts
// action() の中（facility-orders.tsx）
const intent = String(form.get("intent") ?? "addToCart");

if (intent === "confirm" || intent === "remove") {
  if (intent === "confirm") {
    await api.confirmPurchaseOrder(accessToken, params.facilityId, orderId);
    // = fetch POST .../orders/{orderId}/confirm
  } else {
    await api.deletePurchaseOrder(accessToken, params.facilityId, orderId);
    // = fetch DELETE .../orders/{orderId}
  }
  return { ok: true, intent, orderId };
}

// ここから下が「カートに追加」。
// qty_<clinicProductId> というキーを集めて、数量1以上の行だけ明細にする
const order = await api.saveDraftPurchaseOrder(accessToken, params.facilityId, {
  distributorId,
  lines, // [{ clinicProductId, quantity }, ...]
});
// = fetch POST http://localhost:8080/api/facilities/{facilityId}/orders
```

## 3. 往復A「カートに追加」: 検証して下書きに積む

`ServeMux`の対応表 `"POST /api/facilities/{facilityId}/orders"` に一致し、
`internal/handler/procurement/handler.go`の`postOrder`が呼ばれる。
handlerはここでも変換だけ（認可チェック→ID/JSONパース→`Execute`呼び出し）。

```go
// internal/application/procurement/save_draft_purchase_order.go の Execute
// 手順1: 発注先の卸業者が実在するか
uc.distributorRepo.FindByID(ctx, in.DistributorID)

// 手順2: 同じクリニック・卸の下書きが既にあるか探す。あれば積み増し、無ければ新規作成
order, err := uc.purchaseOrderRepo.FindDraftByFacilityAndDistributor(ctx, in.FacilityID, in.DistributorID)
//   → SELECT * FROM purchase_orders WHERE facility_id=? AND distributor_id=? AND status='draft'

// 手順3: 明細ごとに クリニック商品→卸商品→卸業者 を辿り、
//        発注先の卸と一致するか検証（「1発注=1卸」の実体レベルの担保）
distributorProduct, _ := uc.distributorProductRepo.FindByID(ctx, clinicProduct.DistributorProductID())
if distributorProduct.DistributorID() != in.DistributorID { return nil, ...ErrConflict }

// 手順4: 発注時点の単価をスナップショットして明細に固定
//        同じクリニック商品を再度カートに入れると AddLine が数量を加算する
order.AddLine(line.ClinicProductID, line.Quantity, clinicProduct.UnitPrice())

// 手順5: 新規の下書きならINSERT、既存の下書きへの積み増しならUPDATE
uc.purchaseOrderRepo.Create(ctx, order)  // INSERT INTO purchase_orders / purchase_order_lines ...
uc.purchaseOrderRepo.Update(ctx, order)
```

ここにはCSVもS3も出てこない。**カートに積むだけでは卸に何も送らない**からで、
下書きは発注履歴にも現れない（画面では「カート」欄に「下書き」バッジ付きで並ぶ）。

## 4. 往復B「発注する」: ボタンからユースケースに届くまで

`confirm_purchase_order.go`の`Execute`にたどり着くまでに7段ある。どのファイルの何行目で
次に渡されるかを追うと、こうなる（往復Aの「カートに追加」もURLと`intent`が違うだけで同じ形）。

| # | 場所 | 何が起きるか |
|---|---|---|
| ① | [facility-orders.tsx:278](../frontend/app/routes/facility-orders.tsx#L278) | カート内カードの`<button name="intent" value="confirm">発注する</button>`をsubmit |
| ② | [facility-orders.tsx:63](../frontend/app/routes/facility-orders.tsx#L63) | `action`が`intent`で分岐し、`api.confirmPurchaseOrder(...)`を呼ぶ |
| ③ | [api.server.ts:146](../frontend/app/lib/api.server.ts#L146) | `POST /api/facilities/{facilityId}/orders/{orderId}/confirm` を発行 |
| ④ | [main.go:117](../backend/cmd/api/main.go#L117) | `root.Handle("/api/", httputil.RequireAuth(protected))` で認証を通過 |
| ⑤ | [handler.go:39](../backend/internal/handler/procurement/handler.go#L39) | `Register`の登録パターンに一致し、`confirmOrderHandler`が呼ばれる |
| ⑥ | [handler.go:138](../backend/internal/handler/procurement/handler.go#L138) | 認可チェックと`facilityId`/`orderId`のパース（変換だけ） |
| ⑦ | [handler.go:153](../backend/internal/handler/procurement/handler.go#L153) | `h.confirmOrder.Execute(...)` → [confirm_purchase_order.go:48](../backend/internal/application/procurement/confirm_purchase_order.go#L48) |

読むときに迷いやすいのが④で、**認証だけは`handler.go`に一切書かれていない**。`RequireAuth`は
`main.go`でmuxごとラップするミドルウェアなので、`Register`にも各ハンドラ関数にも出てこない。
「handler.goを読んでも認証が見当たらない」ときは`main.go`を見る。

もう1つが⑦で、`h.confirmOrder`というフィールドに実体が入るのは`main.go`である点。
呼ぶ場所（handler）と用意する場所（main.go）が離れているのはDIコンテナが無いためで、
配線の追い方は[`go-for-csharp.md`の8章](go/go-for-csharp.md)にまとめてある。

## 5. 往復Bのユースケース: 確定 → S3アップロード → 保存、の順序が肝

```go
// internal/application/procurement/confirm_purchase_order.go の Execute
// 手順1: 発注を取得し、URLのfacilityIdと一致するか確認
//        （他クリニックの発注IDを自分のURL配下から叩いても操作できないようにする）
order, _ := uc.purchaseOrderRepo.FindByID(ctx, in.OrderID)
if order.FacilityID() != in.FacilityID { return nil, ...ErrNotFound }

// 手順2: CSVの明細を組み立てる。卸に送る書類なので、クリニック側の呼び方ではなく
//        卸商品コード・卸側の商品名を使う
distributorProduct, _ := uc.distributorProductRepo.FindByID(ctx, clinicProduct.DistributorProductID())

// 手順3: CSVの「発注日」とDBのconfirmed_atを揃えるため、時刻は1つだけ作って使い回す
confirmedAt := time.Now()
order.Confirm(confirmedAt)  // 明細0件ならここでエラー

// 手順4: 確定したのでCSVをS3にアップロード（ここが今回の副作用）
uc.csvUploader.Upload(ctx, order, facility.Name(), confirmedAt, csvLines)
//   失敗したら return error でここで打ち切り → 次のDB保存に進まない

// 手順5: アップロード成功後にようやくDB保存
uc.purchaseOrderRepo.Update(ctx, order)
//   → UPDATE purchase_orders SET status='confirmed', confirmed_at=... WHERE id=...
```

**手順4→5の順序が業務ルール**: 「発注CSVが卸に届いて初めて確定と言える」ため、
S3アップロードが失敗したらDBには保存しない（確定したのに卸に届いていない、という
不整合を避けるため）。検証→ドメイン生成→保存で完結する通常のユースケースと違い、
外部サービス呼び出しがDB保存より**前**に来る点がこのフローの肝。

なお`order.Confirm()`はメモリ上の集約を確定状態に変えるだけで、この時点ではまだDBに何も書いていない。
だから手順4で失敗しても「確定が漏れる」のではなく、単にUPDATEが実行されずカートに残るだけで済む。

## 6. `Upload`の中身: インターフェースからAWS SDKまで4段

§5の`uc.csvUploader.Upload(...)`はこのノートで唯一の外部サービス呼び出しだが、その1行の先は
4つのファイルに分かれている。層をまたぐごとに**知っていることが減っていく**のがポイント。

| 段 | ファイル | 何を知っているか |
|---|---|---|
| ① 呼び出し | [confirm_purchase_order.go:92](../backend/internal/application/procurement/confirm_purchase_order.go#L92) | 発注のことだけ。S3もCSVも知らない |
| ② インターフェース | [purchase_order_csv.go:26](../backend/internal/application/procurement/purchase_order_csv.go#L26) | 「送る」という契約だけ |
| ③ CSV変換とキー決定 | [purchase_order_csv_uploader.go:31](../backend/internal/infrastructure/procurement/purchase_order_csv_uploader.go#L31) | CSVの列順とS3のキー構造 |
| ④ SDK呼び出し | [s3_uploader.go:24](../backend/internal/infrastructure/storage/s3_uploader.go#L24) | バケット名とAWS SDK。発注を知らない |

### ① ユースケースが持つのはインターフェース型

```go
// confirm_purchase_order.go:24
csvUploader PurchaseOrderCsvUploader  // ← interface。S3実装のことは知らない
```

契約は1メソッドだけ。

```go
// purchase_order_csv.go:26
type PurchaseOrderCsvUploader interface {
	Upload(ctx context.Context, order *procdomain.PurchaseOrder, facilityName string,
		orderedAt time.Time, lines []PurchaseOrderCsvLine) error
}
```

`facilityName`と`lines`が`order`と別に渡されるのは、**集約が持っていない値だから**。
`PurchaseOrder`はクリニック商品IDしか持たず、CSVに必要な卸商品コード・商品名・クリニック名は
他集約から解決しないと出てこないので、§5の手順2でユースケースが解決してから渡している。

### ③ CSVを組み立て、キーを決める（infrastructure層）

ファイルは作らず、メモリ上のバッファに`encoding/csv`で書く。

```go
buf := &bytes.Buffer{}
w := csv.NewWriter(buf)
w.Write(csvHeader)  // 発注ID, 発注日, クリニックID, クリニック名, 卸商品コード, 商品名, 数量, 単価, 金額
...
w.Flush()
if err := w.Error(); err != nil { ... }  // ← Flushの「後に」見るのが要点
return buf.Bytes(), nil
```

`csv.Writer`は内部でバッファリングするため、書き込みエラーが`Write()`の戻り値ではなく
`Flush()`の後にしか現れないことがある。`Write`だけ見ていると取りこぼす。

S3のキーもここで決める。命名の意図（卸ごとのテナント分離）は
別リポジトリ`clinic-inventory-csv-functions`の`docs/s3-storage.md`3章を参照。

```go
key := fmt.Sprintf("orders/%s/%s/%s.csv",
	order.DistributorID().String(), order.FacilityID().String(), order.ID().String())
```

### ④ AWS SDKを呼ぶ

```go
// s3_uploader.go:24
_, err := u.client.PutObject(ctx, &s3.PutObjectInput{
	Bucket:      &u.bucket,
	Key:         &key,
	Body:        bytes.NewReader(body),
	ContentType: &contentType,
})
```

C#の`IAmazonS3.PutObjectAsync`に相当する。Goで戸惑いやすい点が2つある。

- **引数が全部ポインタ**（`&u.bucket`, `&key`）。AWS SDKは「未指定」と「空文字を指定」を
  区別する必要があるため、`nil`かどうかで表現している。C#の`string?`に近い。
- **`Body`が`io.Reader`**。`bytes.NewReader(body)`でバイト列を包んでいる。巨大ファイルなら
  ストリームをそのまま渡せる形だが、発注CSVは小さいのでメモリ上で十分。

この`S3Uploader`が発注用の場所ではなく`internal/infrastructure/storage/`（汎用の置き場所）に
あるのは、S3が発注専用ではなく今後CSV取り込みなど他コンテキストからも使われるため。

### 認証情報はコードに出てこない

`S3Uploader`にアクセスキーの類は一切ない。認証は`main.go`で解決される。

```go
// main.go:60
awsCfg, err := config.LoadDefaultConfig(context.Background())
s3Client := s3.NewFromConfig(awsCfg)
```

`LoadDefaultConfig`が環境変数 → `~/.aws/credentials` → IAMロールの順に自動で探す
（C#のAWSSDKのデフォルト認証チェーンと同じ考え方）。実際に設定している値と、なぜ
アクセスキー方式にしているかは別リポジトリ`clinic-inventory-csv-functions`の`docs/s3-storage.md`2章を参照。

### この4段構造の実利

ユースケースが持つのは②のインターフェースなので、**ユースケースのテストにS3が要らない**。
`Upload`がエラーを返すだけのフェイクを渡せば、§5の「アップロード失敗時にDBが更新されない」を
AWSに一切触らずに検証できる。

## 7. 「取消」: 下書きだけが消せる

`DELETE .../orders/{orderId}` → `deleteOrder` → `RemoveDraftPurchaseOrderUseCase`。
確定済みの発注は不変（取消は逆仕訳で表現する方針）のため、`status`が`draft`でなければ
`ErrConflict`で弾く。カートの「取消」ボタンだけがこのエンドポイントを叩く。

## 8. 帰り道

- カートに追加 成功: 200 → actionが`{ok: true, intent: "addToCart"}`を返す
  → フォームに「カートに追加しました。」+ 下の「カート」欄に卸ごとのカードが増える
- 発注する 成功: 200 → 下書きが確定に変わるので、カート欄から消えて発注履歴に発注日時つきで並ぶ
- 取消 成功: 204（レスポンスボディなし）→ カート欄からカードが消える
- 失敗（卸不一致・S3アップロード失敗・確定済みの取消など）: `fmt.Errorf(...)`のメッセージが
  `httputil.WriteError`経由でエラーレスポンスになり、actionが捕まえて
  該当するフォーム／カートのカードに赤字表示（`intent`と`orderId`で表示先を絞っている）
