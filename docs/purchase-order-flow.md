# 発注フロー — カート追加から確定・S3アップロードまで

発注画面（`/facilities/:id/orders`）でボタンを押してから、卸に渡す発注CSVがS3に置かれるまでを追う。
この画面は**1回の操作では発注が完了しない**。「カートに追加」で下書きを作り、「発注する」で確定する
2段階なので、往復も2回に分かれ、**外部サービス(S3)への副作用は2回目にだけ**起きる。

1リクエストがhandler → ユースケース → SQLに届く基本形は
[go/request-to-sql-flow.md](go/request-to-sql-flow.md) の1〜2章にある。S3のバケット構成や
認証情報の詳細は別リポジトリ`clinic-inventory-csv-functions`の`docs/s3-storage.md`を参照。

このドキュメントは**今の実装**を追ったもの。確定をDBに保存してから別の処理で卸ごとの形式に変換して送る形へ移す設計は
[purchase-order-export.md](purchase-order-export.md)にある(未実装)。

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
| ③ | [api.server.ts:155](../frontend/app/lib/api.server.ts#L155) | `POST /api/facilities/{facilityId}/orders/{orderId}/confirm` を発行 |
| ④ | [main.go:124](../backend/cmd/api/main.go#L124) | `root.Handle("/api/", httputil.RequireAuth(protected))` で認証を通過 |
| ⑤ | [handler.go:39](../backend/internal/handler/procurement/handler.go#L39) | `Register`の登録パターンに一致し、`confirmOrderHandler`が呼ばれる |
| ⑥ | [handler.go:138](../backend/internal/handler/procurement/handler.go#L138) | 認可チェックと、URLから`facilityId`/`orderId`を取り出してID型に変換（変換だけ） |
| ⑦ | [handler.go:153](../backend/internal/handler/procurement/handler.go#L153) | ⑥のIDを`ConfirmPurchaseOrderInput`に詰めて`h.confirmOrder.Execute(...)`を呼ぶ → [confirm_purchase_order.go:51](../backend/internal/application/procurement/confirm_purchase_order.go#L51) |

⑤の「登録パターン」は、ボタンを押したときではなく**アプリ起動時に1回**登録されている
（[main.go:109](../backend/cmd/api/main.go#L109)が`Register`を呼び、[handler.go:39](../backend/internal/handler/procurement/handler.go#L39)の`mux.HandleFunc`でURLと関数を対応づける）。
ボタンを押すたびに起きるのは、届いたURLをその対応表で引いて`confirmOrderHandler`を動かすところから。

⑦で詰めた箱が、ユースケース側では引数`in`として届く。

```go
// handler.go:153 — 詰める側
h.confirmOrder.Execute(r.Context(), procapp.ConfirmPurchaseOrderInput{
	FacilityID: facilityID, // URLの {facilityId}
	OrderID:    orderID,    // URLの {orderId}
})

// confirm_purchase_order.go:51 — 受け取る側
func (uc *ConfirmPurchaseOrderUseCase) Execute(ctx context.Context, in ConfirmPurchaseOrderInput) ...
```

だから`Execute`の中の`in.OrderID`（54行目）・`in.FacilityID`（58行目）は、どちらも「URLに入っていたID」のこと。
値の出どころはフロントが送ったURLだが、`ConfirmPurchaseOrderInput`に詰めるのはhandlerで、フロントはこの型を知らない。

読むときに迷いやすいのが④で、**認証だけは`handler.go`に一切書かれていない**。`RequireAuth`は
`main.go`でmuxごとラップするミドルウェアなので、`Register`にも各ハンドラ関数にも出てこない。
「handler.goを読んでも認証が見当たらない」ときは`main.go`を見る。

もう1つが⑦で、`h.confirmOrder`というフィールドに実体が入るのは`main.go`である点。
呼ぶ場所（handler）と用意する場所（main.go）が離れているのはDIコンテナが無いためで、
配線の追い方は[`go-for-csharp.md`の8章](go/go-for-csharp.md)にまとめてある。

### Laravelで言うと

| # | このプロジェクト | 役割 | Laravel |
|---|---|---|---|
| 起動時 | [main.go:109](../backend/cmd/api/main.go#L109) `.Register(protected)` | 起動時にルートを読み込む | `bootstrap/app.php`（古い版は`RouteServiceProvider`）が`routes/api.php`を読み込む。自分では書かない |
| 起動時 | main.goでリポジトリを作ってユースケースに渡す | 配線 | サービスコンテナが自動で注入する。インターフェースを使う場合だけ`AppServiceProvider`に`bind(PurchaseOrderRepositoryInterface::class, EloquentPurchaseOrderRepository::class)`を書く |
| ①〜③ | [facility-orders.tsx:57-64](../frontend/app/routes/facility-orders.tsx#L57-L64) `intent`で分岐してAPIを呼ぶ | フロントの中継 | Bladeのフォームが直接Laravelのルートに送るので、この層は無い。`intent`の分岐はControllerの中か、ルートを分けて表す |
| ④ | [main.go:124](../backend/cmd/api/main.go#L124) `RequireAuth(...)` | ログイン確認 | `->middleware('auth:sanctum')` |
| ⑤ | [handler.go:39](../backend/internal/handler/procurement/handler.go#L39) `mux.HandleFunc("POST /api/.../confirm", ...)` | URLと関数の対応 | `routes/api.php` の `Route::post('/facilities/{facility}/orders/{order}/confirm', [PurchaseOrderController::class, 'confirm']);` |
| ⑥ | [handler.go:138](../backend/internal/handler/procurement/handler.go#L138) `confirmOrderHandler` | 受け口 | Controllerの`confirm()`メソッド |
| ⑥ | [handler.go:139](../backend/internal/handler/procurement/handler.go#L139) `AuthorizeFacility` | このクリニックを操作してよいか | Policy / Gate（`$this->authorize(...)`） |
| ⑥ | [handler.go:143-151](../backend/internal/handler/procurement/handler.go#L143-L151) `PathValue` → `ParseID` | URLからIDを取り出す | メソッド引数 `confirm(string $facility, string $order)` に自動で入る |
| ⑦ | `ConfirmPurchaseOrderInput{...}` | ユースケースへの箱 | 自前のDTOクラス（Laravel標準の仕組みは無い） |
| ⑦ | `ConfirmPurchaseOrderUseCase.Execute` | 手順を並べる | 自前のService / Actionクラス（`app/Actions/ConfirmPurchaseOrder.php`など） |

Laravelのルートモデルバインディングに`scopeBindings()`を付けると、
[confirm_purchase_order.go:54](../backend/internal/application/procurement/confirm_purchase_order.go#L54)の`FindByID`と
[58行目](../backend/internal/application/procurement/confirm_purchase_order.go#L58)の「その発注がそのクリニックのものか」の比較を、Laravelが自動で行う。

```php
Route::post('/facilities/{facility}/orders/{order}/confirm', ...)->scopeBindings();

public function confirm(Facility $facility, PurchaseOrder $order) { ... }  // 違うクリニックの発注なら404
```

このとき渡ってくる`$order`はEloquentモデルで、`$order->status = 'confirmed'; $order->save();`のように
どこからでも状態を書き換えて保存できる。`PurchaseOrder`の`Confirm()`のように、ルールを通らないと状態を変えられない作りにはなっていない。

ユースケースはフレームワークの機能ではなく自分で書くクラスなので（このプロジェクトの`ConfirmPurchaseOrderUseCase`も同じ）、
Laravelでも書ける。Laravelでの書き方は2通りある。

| | A. Laravel標準の書き方 | B. このプロジェクトと同じ形 |
|---|---|---|
| Controllerが受け取るもの | Eloquentモデル `PurchaseOrder $order`（ルートモデルバインディング） | IDの文字列 `string $order` |
| 発注をDBから取ってくる人 | Laravel（自動） | ユースケースの中のリポジトリ（[54行目](../backend/internal/application/procurement/confirm_purchase_order.go#L54)と同じ） |
| 「このクリニックの発注か」のチェック | `scopeBindings()`（自動） | ユースケースの中で比較（[58行目](../backend/internal/application/procurement/confirm_purchase_order.go#L58)と同じ） |
| 「明細0件は確定できない」ルールの置き場所 | Eloquentモデルにメソッドを足す、またはControllerに書く | ドメインのクラス（[purchase_order.go:95](../backend/internal/domain/procurement/purchase_order.go#L95)の`Confirm()`と同じ） |
| 書く量 | 少ない | 多い |

Bの形にする場合は、ルートモデルバインディングの自動取得は使わず、IDだけ受け取ってユースケースの中でリポジトリから取得する。

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
//        S3のフォルダを卸コードで分けるので、発注先の卸も引いてくる
distributor, _ := uc.distributorRepo.FindByID(ctx, order.DistributorID())
uc.csvUploader.Upload(ctx, order, distributor.Code(), facility.Name(), confirmedAt, csvLines)
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

### ユースケースと集約ルートの分担

上のコードには、似た名前のものが3つ出てくる。

| 名前 | 何か | 置き場所 |
|---|---|---|
| 発注ID（`in.OrderID`） | 発注1件を指す番号。画面のボタンが持っていた`orderId`がURLに乗って届き、handlerが詰める（[handler.go:155](../backend/internal/handler/procurement/handler.go#L155)） | — |
| 集約ルート（`order`、型は`*PurchaseOrder`） | 発注1件そのもの。状態・明細を持ち、「明細0件は確定できない」などのルールも自分で持つ。発注の数だけ存在する | [domain/procurement/purchase_order.go:41](../backend/internal/domain/procurement/purchase_order.go#L41) |
| ユースケース（`ConfirmPurchaseOrderUseCase`） | 「取ってくる → ルールを呼ぶ → 送る → 保存する」の手順を並べるもの。発注のデータは持たず、リポジトリやアップローダーを持つ | [application/procurement/confirm_purchase_order.go](../backend/internal/application/procurement/confirm_purchase_order.go) |

`PurchaseOrder`は種類としてはエンティティ（IDを持つドメインの型）で、「集約ルート」はその役割の名前。
集約は「発注 + その明細`OrderLine`たち」という、一緒に変えて一緒に保存するまとまりを指し、
そのまとまりに外から触るときの入口が集約ルートの`PurchaseOrder`になる。

`Execute`の中で集約ルートに触るのは次の行だけ。

| 行 | コード | 何をしているか | 実際に動くソース |
|---|---|---|---|
| [54](../backend/internal/application/procurement/confirm_purchase_order.go#L54) | `purchaseOrderRepo.FindByID(ctx, in.OrderID)` | 発注IDを渡して、集約ルートを受け取る。リポジトリがDBの発注と明細を読んで`PurchaseOrder`に組み立てて返す | [purchase_order_repository.go:82](../backend/internal/infrastructure/procurement/purchase_order_repository.go#L82)（インターフェース経由） |
| [58](../backend/internal/application/procurement/confirm_purchase_order.go#L58) | `order.FacilityID()` | 読む: URLの施設と一致するか | [purchase_order.go:108](../backend/internal/domain/procurement/purchase_order.go#L108) |
| [65](../backend/internal/application/procurement/confirm_purchase_order.go#L65) | `order.Lines()` | 読む: CSVの明細を作るため | [purchase_order.go:116](../backend/internal/domain/procurement/purchase_order.go#L116) |
| [85](../backend/internal/application/procurement/confirm_purchase_order.go#L85) | `order.Confirm(confirmedAt)` | **状態を変える唯一の呼び出し**。ルールはここで判定される | [purchase_order.go:95](../backend/internal/domain/procurement/purchase_order.go#L95) |
| [97](../backend/internal/application/procurement/confirm_purchase_order.go#L97) | `order.DistributorID()` | 読む: 発注先の卸を引くため | [purchase_order.go:109](../backend/internal/domain/procurement/purchase_order.go#L109) |
| [101](../backend/internal/application/procurement/confirm_purchase_order.go#L101) | `csvUploader.Upload(ctx, order, ...)` | 丸ごと渡す: 卸向けファイルに変換して送る | [purchase_order_csv_uploader.go:25](../backend/internal/infrastructure/procurement/purchase_order_csv_uploader.go#L25)（インターフェース経由。先は§6） |
| [106](../backend/internal/application/procurement/confirm_purchase_order.go#L106) | `purchaseOrderRepo.Update(ctx, order)` | 丸ごと渡す: 確定した状態を保存する | [purchase_order_repository.go:43](../backend/internal/infrastructure/procurement/purchase_order_repository.go#L43)（インターフェース経由） |

`order.〜()`の4つは`PurchaseOrder`のメソッドなので、エディタで定義へジャンプすればそのまま
[domain/procurement/purchase_order.go](../backend/internal/domain/procurement/purchase_order.go)に着く。
「インターフェース経由」の3つは、ジャンプするとインターフェースの宣言
（[repository.go:18](../backend/internal/domain/procurement/repository.go#L18)、[purchase_order_csv.go:26](../backend/internal/application/procurement/purchase_order_csv.go#L26)）で止まる。
中身がどの構造体かは[main.go:92](../backend/cmd/api/main.go#L92)（リポジトリ）と[main.go:66](../backend/cmd/api/main.go#L66)（アップローダー）で作られ、
[main.go:101](../backend/cmd/api/main.go#L101)でユースケースに渡されているので、そこを見ると上の表の「実際に動くソース」に辿り着ける。

ユースケースは`status`を直接書き換えず、必ず`order.Confirm()`を呼ぶ。`PurchaseOrder`のフィールドは小文字で
`domain/procurement`パッケージの外から書き換えられないので、ユースケースから確定させる手段は`Confirm()`しかない。
もしユースケースが直接`confirmed`にできると、明細0件の発注や確定済みの発注の二重確定を止めるチェックを
ユースケースごとに書く必要があり、1つでも書き忘れると空の発注CSVが卸に届く。

## 6. `Upload`の中身: インターフェースからAWS SDKまで4段

§5の`uc.csvUploader.Upload(...)`はこのノートで唯一の外部サービス呼び出しだが、その1行の先は
4つのファイルに分かれている。層をまたぐごとに**知っていることが減っていく**のがポイント。

| 段 | ファイル | 何を知っているか |
|---|---|---|
| ① 呼び出し | [confirm_purchase_order.go:101](../backend/internal/application/procurement/confirm_purchase_order.go#L101) | 発注のことだけ。S3もCSVも知らない |
| ② インターフェース | [purchase_order_csv.go:26](../backend/internal/application/procurement/purchase_order_csv.go#L26) | 「送る」という契約だけ |
| ③ CSV変換とキー決定 | [purchase_order_csv_uploader.go:25](../backend/internal/infrastructure/procurement/purchase_order_csv_uploader.go#L25) | CSVの列順とS3のキー構造 |
| ④ SDK呼び出し | [s3_uploader.go:24](../backend/internal/infrastructure/storage/s3_uploader.go#L24) | バケット名とAWS SDK。発注を知らない |

### ① ユースケースが持つのはインターフェース型

```go
// confirm_purchase_order.go:25
csvUploader PurchaseOrderCsvUploader  // ← interface。S3実装のことは知らない
```

契約は1メソッドだけ。

```go
// purchase_order_csv.go:26
type PurchaseOrderCsvUploader interface {
	Upload(ctx context.Context, order *procdomain.PurchaseOrder, distributorCode, facilityName string,
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

## 発注書フォーマット

要件として決まっている発注書の形式。**未実装**で、現在の発注確定は上の5〜6章の通り発注CSVをS3に置くところまで。

- 出力形式: PDF
- ヘッダ: 発注番号（クリニックごとの連番）、発注日、クリニック名・住所・電話番号、担当者名、卸業者名、クリニックの卸取引先コード
- 明細: 卸商品コード、商品名、規格/包装単位、数量、単価・金額（金額データは必須として保持する。発注書PDF上での表示/非表示のみ運用に応じて切り替え可能とする）
- 備考欄
- 用途: (a) 卸購買サイトへの手入力の下敷き、(b) EDI非対応の卸への発注控えとして使用
