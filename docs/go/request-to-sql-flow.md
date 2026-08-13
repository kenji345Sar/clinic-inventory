# リクエストがSQLになるまで — フロントからDBまでの1往復

「フロントの要求をGoがどう検知し、どのコードが動き、どうやってSQLが発行されるのか」を追うノート。
petty-cashの `docs/csharp/routing.md` と `usecase-domain-repository-flow.md` に相当する。

例として「クリニック一覧を表示する」（`GET /api/facilities`）の1往復を最初から最後まで追う。

```
ブラウザ
  → Remix (:5173)  loader が fetch を実行
  → Go (:8080)     net/http が受けて handler → usecase/repository
  → gorm           Goの呼び出しをSQLに翻訳
  → PostgreSQL (:5432)
  ← 行データ → struct → ドメイン → JSON → 画面
```

---

## 0. 前提: 「リクエストごとにAPIが起動する」のではない

`go run ./cmd/api` した時点で `cmd/api/main.go` の `main()` が一度だけ実行され、
最後の `http.ListenAndServe(":8080", ...)` で**プロセスがTCPポート8080を開いて待ち続ける**。

```go
// cmd/api/main.go（末尾）
if err := http.ListenAndServe(addr, httputil.CORS(mux)); err != nil {
```

- 「検知」の正体: OSカーネルが「8080宛のTCP接続はこのプロセスへ」と橋渡しする（ソケットのlisten/accept）
- リクエストが来るたびに起動するのではなく、**常駐プロセスに接続が渡される**
- petty-cash対応: ASP.NET Coreの`app.Run()`でKestrelが5001を待ち受けるのと同じ

---

## 1. フロント側: loaderがfetchする

Remixのloader（サーバーサイド）がGoのAPIを呼ぶ。ブラウザから直接ではない。

```ts
// frontend/app/lib/api.server.ts
const res = await fetch(`${API_BASE}${path}`, ...)  // API_BASE = http://localhost:8080
```

このfetchがTCP接続を張り、HTTPリクエストをテキストで送る。中身はただの文字列:

```
GET /api/facilities HTTP/1.1
Host: localhost:8080
Content-Type: application/json
```

---

## 2. Go側: ServeMuxが対応表を引いてhandler関数を呼ぶ

起動時にmain.goで**URLパターン→関数の対応表**を登録してある。

```go
// internal/handler/organization/handler.go
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/corporations", h.postCorporation)
	mux.HandleFunc("POST /api/facilities", h.postFacility)
	mux.HandleFunc("GET /api/facilities", h.listFacilities)   // ← 今回はこれに一致
}
```

`net/http`がHTTPテキストを解析し、ServeMuxが「メソッド＋パス」で対応表を引き、
一致した`listFacilities`を呼ぶ。**1リクエストにつき1つのgoroutine**（軽量スレッド）が
割り当てられるので、同時アクセスは自動で並行処理される。

petty-cash対応: KestrelがControllerのルート属性（`[HttpGet]`等）を見てアクションメソッドを呼ぶのと同じ。goroutineはASP.NETのスレッドプールに相当。

---

## 3. handler → repository（または usecase）→ gorm

```go
// internal/handler/organization/handler.go
func (h *Handler) listFacilities(w http.ResponseWriter, r *http.Request) {
	facilities, err := h.facilityRepo.FindAll(r.Context())   // ← リポジトリを呼ぶ
	...
	httputil.WriteJSON(w, http.StatusOK, res)                // ← 最後にJSONを書き戻す
}
```

読み取りはリポジトリ直、書き込み（POST）はユースケース経由:

```
GET  /api/facilities  → handler → FacilityRepository.FindAll → gorm
POST /api/facilities  → handler → CreateFacilityUseCase → NewFacility()（ドメインのバリデーション）
                                → FacilityRepository.Create → gorm
```

---

## 4. gorm: Goの呼び出しがSQL文字列になる瞬間

```go
// internal/infrastructure/organization/facility_repository.go
func (r *FacilityRepository) FindAll(ctx context.Context) ([]*orgdomain.Facility, error) {
	var models []FacilityModel
	if err := r.db.WithContext(ctx).Find(&models).Error; err != nil {
```

この`Find(&models)`でgormがやること:

1. `[]FacilityModel`という型から対象テーブルを特定（`TableName()`が`"facilities"`を返す）
2. SQL文字列を組み立てる → `SELECT * FROM "facilities"`
3. pgxドライバがPostgreSQL(:5432)への**別のTCP接続**でSQLを送信
4. 返ってきた行を1件ずつ`FacilityModel`のフィールドに詰める（列名⇔フィールドはgormタグで対応）

petty-cash対応: EF Coreで`context.Safes.ToListAsync()`がSQLに翻訳されるのと同じ。`*gorm.DB`がDbContext、`FacilityModel`がEFのエンティティクラスに相当する。

その後リポジトリが`toDomainFacility(model)`で**永続化モデル→ドメインエンティティ**に変換して返す。gormのモデルとドメインを分けているのは、DB都合の型（gormタグ等）をドメインに持ち込まないため。

---

## 5. 実際に発行されるSQLを目で見る方法

gormのログレベルを上げると、翻訳結果のSQLがターミナルに出る。

```go
// internal/infrastructure/database/database.go を一時的に変更
import "gorm.io/gorm/logger"

func Connect(dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // ← 全SQLをログに出す
	})
}
```

この状態でAPIサーバーを起動し、別ターミナルから叩くと:

```bash
curl -s http://localhost:8080/api/facilities
# サーバー側ターミナルに出るログ:
#   SELECT * FROM "facilities"
```

確認が終わったら戻すこと（本番相当の設定でSQL全ログは出さない）。

---

## 6. 1往復の全体対応表

| 段階 | このプロジェクト | petty-cash (C#) |
|---|---|---|
| 待ち受け | `http.ListenAndServe(":8080")` | Kestrel（`app.Run()`、:5001） |
| ルーティング | `ServeMux` のパターン登録 | Controllerのルート属性 |
| 受け口 | `internal/handler/` の関数 | Controllerのアクションメソッド |
| ユースケース | `internal/application/` | PettyCash.Application/UseCases |
| ドメイン | `internal/domain/` | PettyCash.Domain |
| ORM | gorm（`*gorm.DB`） | EF Core（DbContext） |
| DBドライバ | pgx | Npgsql |
| 並行処理 | goroutine（1リクエスト1本） | スレッドプール |

---

## 7. 書き込みの1往復 — 商品マスタ画面の「登録」ボタンからINSERTまで

1〜5章はGET（読み取り）だった。ここでは**実際に画面にあるボタン**で書き込みの1往復を追う。
商品マスタ画面（`/facilities/:id/products`）で卸商品を選び、商品コードと発注点を入れて
「登録」ボタンを押したときの流れ。

### 7-1. 画面: フォームのsubmit

```tsx
// frontend/app/routes/facility-products.tsx
<Form method="post" ...>
  <input type="hidden" name="distributorProductId" value={selected.id} />
  <input name="productCode" required ... />
  <input name="reorderPoint" type="number" ... />
  <button type="submit">登録</button>
</Form>
```

「登録」を押すと、React RouterがこのルートへPOSTし、同じファイルの`action()`が
**サーバーサイドで**実行される。`form.get("productCode")`でフォーム項目を取り出す。

### 7-2. フロント→Go: actionがAPIを呼ぶ

```ts
// action() の中 → api.server.ts
await api.registerClinicProduct(params.facilityId, { productCode, ..., reorderPoint });
// = fetch POST http://localhost:8080/api/facilities/{facilityId}/products （JSONボディ）
```

### 7-3. Go: handlerが受けてユースケースへ

ServeMuxの対応表 `"POST /api/facilities/{facilityId}/products"` に一致し、
`internal/handler/productcatalog/handler.go` の `postProduct` が呼ばれる。
handlerがやるのは変換だけ: パスの`{facilityId}`をIDに(`ParseID`)、JSONボディをstructに(`DecodeJSON`)、
そしてユースケースの`Execute`を呼ぶ。**業務判断はしない**。

### 7-4. ユースケース: 業務の手順書（1行=1手順、SQL付きで読む）

```go
// internal/application/productcatalog/register_clinic_product.go の Execute
// 手順1: 紐付け先の卸商品が実在するか
distributorProduct, err := uc.distributorProductRepo.FindByID(ctx, in.DistributorProductID)
//   → SELECT * FROM distributor_products WHERE id = $1

// 手順2: 廃盤なら拒否（業務ルール）
if distributorProduct.Discontinued() { return nil, ... }

// 手順3: 商品コードの重複チェック
exists, err := uc.clinicProductRepo.ExistsByFacilityAndCode(ctx, in.FacilityID, in.ProductCode)
//   → SELECT count(*) FROM clinic_products WHERE facility_id = $1 AND product_code = $2
if exists { return nil, ...ErrConflict }   // ← 409になる

// 手順4: 名前/JANが空なら卸商品から引き継ぐ
name := in.Name; if name == "" { name = distributorProduct.Name() }

// 手順5: ドメインに生成を依頼（必須チェック・発注点0以上はこの中）
product, err := proddomain.NewClinicProduct(in.FacilityID, in.ProductCode, name, ...)

// 手順6: 保存
err := uc.clinicProductRepo.Create(ctx, product)
//   → INSERT INTO clinic_products (id, facility_id, product_code, ...) VALUES (...)
```

### 7-5. 帰り道: 結果が画面の文言になるまで

- 成功: handlerが201でJSONを返す → actionが`{ok: true}`を返す → 画面に「登録しました。」
- 重複: 手順3の`ErrConflict`を`httputil.WriteError`が**409**に変換 →
  actionが`ApiError`を捕まえて`{ok: false, error: "商品コード C-1001 は...使われています"}` →
  画面に赤字でそのまま表示される

つまり画面の赤いエラーメッセージの出どころは、ユースケースの`fmt.Errorf(...)`の文字列。

---

## 8. ユースケースを起点に読む方法

どのユースケースでも、知りたいこと別に見る場所は同じ。

| 知りたいこと | 見る場所 |
|---|---|
| どのURL/ボタンから呼ばれるか | `internal/handler/<コンテキスト>/handler.go` の `Register()`（URL対応表） |
| 業務の手順（何をどの順で） | `internal/application/<コンテキスト>/` の `Execute` の中 |
| 業務ルール（何が許されないか） | `internal/domain/<コンテキスト>/` の `New...` 関数（`errors.New`の行） |
| どんなSQLになるか | `internal/infrastructure/<コンテキスト>/` のリポジトリ実装 |

逆引き（ボタン→コード）の探し方: フロントの`api.server.ts`でそのボタンが叩くURLパスを見て、
そのパスをhandlerの`Register()`から探すと入口の関数が見つかる。あとは`Execute`へ降りるだけ。

---

## 9. URLから画面ファイルを特定する方法（逆引きの入口）

ブラウザのアドレスバーのURLから、対応する画面ファイルをどう探すか。
このプロジェクトはReact Router v7の「フレームワークモード」で、ルーティングは
ファイル名の規約ではなく`routes.ts`に**明示的な対応表**として書いてある。

```ts
// frontend/app/routes.ts
export default [
  index("routes/home.tsx"),
  route("facilities/:facilityId/products", "routes/facility-products.tsx"),
  route("facilities/:facilityId/orders", "routes/facility-orders.tsx"),   // ← /facilities/{id}/orders はここ
  ...
] satisfies RouteConfig;
```

例: `http://localhost:5173/facilities/38057321-.../orders` というURLなら、
`facilities/:facilityId/orders`パターンに一致し`routes/facility-orders.tsx`が該当ファイルだとわかる。
このファイル1つに`loader`（画面表示時のデータ取得）・`action`（フォーム送信時の処理）・
コンポーネント本体がまとまっているのがReact Router v7の特徴（petty-cash対応: Controller+Viewが1ファイルに寄った形）。

バックエンド側の逆引きも同じ考え方で、`internal/handler/<コンテキスト>/handler.go`の
`Register()`がURLパターン→関数の対応表になっている（3章参照）。

---

## 10. 書き込みの往復（発展編）— カート方式の2往復とS3アップロード

7章は「登録」ボタン1つでINSERT1回のシンプルな例だった。ここでは発注画面
（`/facilities/:id/orders`）を追う。この画面は**1回の操作では発注が完了しない**。
「カートに追加」で下書きを作り、「発注する」で確定する2段階なので、往復も2回に分かれ、
**外部サービス(S3)への副作用は2回目にだけ**起きる。S3周りの詳細は
[`s3-storage.md`](../architecture/s3-storage.md)を参照。

| 画面の操作 | HTTPメソッド + パス | handler | ユースケース | DBに起きること |
|---|---|---|---|---|
| カートに追加 | `POST /api/facilities/{facilityId}/orders` | `postOrder` | `SaveDraftPurchaseOrderUseCase` | 下書きをINSERT、既にあればUPDATE |
| 発注する | `POST /api/facilities/{facilityId}/orders/{orderId}/confirm` | `confirmOrderHandler` | `ConfirmPurchaseOrderUseCase` | S3送信の**後**にUPDATE（`status='confirmed'`） |
| 取消 | `DELETE /api/facilities/{facilityId}/orders/{orderId}` | `deleteOrder` | `RemoveDraftPurchaseOrderUseCase` | 下書きをDELETE |

「1発注 = 1卸」なので、カートは卸業者ごとに別々の下書き（= 別々の`purchase_orders`行）になる。
画面の「カート」欄に卸ごとのカードが並び、それぞれに「発注する」ボタンが付くのはそのため。

### 10-1. 画面: 1つの画面から3種類のAPIを撃ち分ける

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

### 10-2. フロント: actionがintentで分岐して別々のAPIを呼ぶ

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

### 10-3. 往復A「カートに追加」: 検証して下書きに積む

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

### 10-4. 往復B「発注する」: ボタンからユースケースに届くまで

`confirm_purchase_order.go`の`Execute`にたどり着くまでに7段ある。どのファイルの何行目で
次に渡されるかを追うと、こうなる（往復Aの「カートに追加」もURLと`intent`が違うだけで同じ形）。

| # | 場所 | 何が起きるか |
|---|---|---|
| ① | [facility-orders.tsx:278](../../frontend/app/routes/facility-orders.tsx#L278) | カート内カードの`<button name="intent" value="confirm">発注する</button>`をsubmit |
| ② | [facility-orders.tsx:63](../../frontend/app/routes/facility-orders.tsx#L63) | `action`が`intent`で分岐し、`api.confirmPurchaseOrder(...)`を呼ぶ |
| ③ | [api.server.ts:146](../../frontend/app/lib/api.server.ts#L146) | `POST /api/facilities/{facilityId}/orders/{orderId}/confirm` を発行 |
| ④ | [main.go:117](../../backend/cmd/api/main.go#L117) | `root.Handle("/api/", httputil.RequireAuth(protected))` で認証を通過 |
| ⑤ | [handler.go:39](../../backend/internal/handler/procurement/handler.go#L39) | `Register`の登録パターンに一致し、`confirmOrderHandler`が呼ばれる |
| ⑥ | [handler.go:138](../../backend/internal/handler/procurement/handler.go#L138) | 認可チェックと`facilityId`/`orderId`のパース（変換だけ） |
| ⑦ | [handler.go:153](../../backend/internal/handler/procurement/handler.go#L153) | `h.confirmOrder.Execute(...)` → [confirm_purchase_order.go:48](../../backend/internal/application/procurement/confirm_purchase_order.go#L48) |

読むときに迷いやすいのが④で、**認証だけは`handler.go`に一切書かれていない**。`RequireAuth`は
`main.go`でmuxごとラップされているため、`Register`にも各ハンドラ関数にも出てこない
（8章のミドルウェアの話）。「handler.goを読んでも認証が見当たらない」ときは`main.go`を見る。

もう1つが⑦で、`h.confirmOrder`というフィールドに実体が入るのは`main.go`である点。
呼ぶ場所（handler）と用意する場所（main.go）が離れているのはDIコンテナが無いためで、
配線の追い方は[`go-for-csharp.md`の8章](go-for-csharp.md)にまとめてある。

### 10-5. 往復Bのユースケース: 確定 → S3アップロード → 保存、の順序が肝

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
不整合を避けるため）。7章の例（検証→ドメイン生成→保存のみ）と違い、外部サービス呼び出しが
DB保存より**前**に来る点が今回のポイント。

なお`order.Confirm()`はメモリ上の集約を確定状態に変えるだけで、この時点ではまだDBに何も書いていない。
だから手順4で失敗しても「確定が漏れる」のではなく、単にUPDATEが実行されずカートに残るだけで済む。

### 10-6. `Upload`の中身: インターフェースからAWS SDKまで4段

10-5の`uc.csvUploader.Upload(...)`はこの章で唯一の外部サービス呼び出しだが、その1行の先は
4つのファイルに分かれている。層をまたぐごとに**知っていることが減っていく**のがポイント。

| 段 | ファイル | 何を知っているか |
|---|---|---|
| ① 呼び出し | [confirm_purchase_order.go:92](../../backend/internal/application/procurement/confirm_purchase_order.go#L92) | 発注のことだけ。S3もCSVも知らない |
| ② インターフェース | [purchase_order_csv.go:26](../../backend/internal/application/procurement/purchase_order_csv.go#L26) | 「送る」という契約だけ |
| ③ CSV変換とキー決定 | [purchase_order_csv_uploader.go:31](../../backend/internal/infrastructure/procurement/purchase_order_csv_uploader.go#L31) | CSVの列順とS3のキー構造 |
| ④ SDK呼び出し | [s3_uploader.go:24](../../backend/internal/infrastructure/storage/s3_uploader.go#L24) | バケット名とAWS SDK。発注を知らない |

#### ① ユースケースが持つのはインターフェース型

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
他集約から解決しないと出てこないので、10-5の手順2でユースケースが解決してから渡している。

#### ③ CSVを組み立て、キーを決める（infrastructure層）

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
[`s3-storage.md`の3章](../architecture/s3-storage.md)を参照。

```go
key := fmt.Sprintf("orders/%s/%s/%s.csv",
	order.DistributorID().String(), order.FacilityID().String(), order.ID().String())
```

#### ④ AWS SDKを呼ぶ

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

#### 認証情報はコードに出てこない

`S3Uploader`にアクセスキーの類は一切ない。認証は`main.go`で解決される。

```go
// main.go:60
awsCfg, err := config.LoadDefaultConfig(context.Background())
s3Client := s3.NewFromConfig(awsCfg)
```

`LoadDefaultConfig`が環境変数 → `~/.aws/credentials` → IAMロールの順に自動で探す
（C#のAWSSDKのデフォルト認証チェーンと同じ考え方）。実際に設定している値と、なぜ
アクセスキー方式にしているかは[`s3-storage.md`の2章](../architecture/s3-storage.md)を参照。

#### この4段構造の実利

ユースケースが持つのは②のインターフェースなので、**ユースケースのテストにS3が要らない**。
`Upload`がエラーを返すだけのフェイクを渡せば、10-5の「アップロード失敗時にDBが更新されない」を
AWSに一切触らずに検証できる。

### 10-7. 「取消」: 下書きだけが消せる

`DELETE .../orders/{orderId}` → `deleteOrder` → `RemoveDraftPurchaseOrderUseCase`。
確定済みの発注は不変（取消は逆仕訳で表現する方針）のため、`status`が`draft`でなければ
`ErrConflict`で弾く。カートの「取消」ボタンだけがこのエンドポイントを叩く。

### 10-8. 帰り道

- カートに追加 成功: 200 → actionが`{ok: true, intent: "addToCart"}`を返す
  → フォームに「カートに追加しました。」+ 下の「カート」欄に卸ごとのカードが増える
- 発注する 成功: 200 → 下書きが確定に変わるので、カート欄から消えて発注履歴に発注日時つきで並ぶ
- 取消 成功: 204（レスポンスボディなし）→ カート欄からカードが消える
- 失敗（卸不一致・S3アップロード失敗・確定済みの取消など）: `fmt.Errorf(...)`のメッセージが
  `httputil.WriteError`経由でエラーレスポンスになり、actionが捕まえて
  該当するフォーム／カートのカードに赤字表示（`intent`と`orderId`で表示先を絞っている）
