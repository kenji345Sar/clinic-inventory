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

## 10. 書き込みの1往復（発展編）— 発注確定ボタンでS3アップロードを含む例

7章は「登録」ボタンでINSERT1回のシンプルな例だった。ここでは発注画面
（`/facilities/:id/orders`）の「この卸に発注」ボタンを押したときの、
**外部サービス(S3)への副作用を含む**書き込みの1往復を追う。S3周りの詳細は
[`s3-storage.md`](../architecture/s3-storage.md)を参照。

### 10-1. 画面: 卸ごとのフォームをsubmit

```tsx
// frontend/app/routes/facility-orders.tsx
<Form method="post">
  <input type="hidden" name="distributorId" value={group.distributorId} />
  <input name={`qty_${p.id}`} type="number" ... />
  <button type="submit">この卸に発注</button>
</Form>
```

「1発注 = 1卸」なので、卸業者ごとに別々のフォームが並んでいる（画面は卸の数だけ`<Form>`を描画）。

### 10-2. フロント: actionがqty_*を集めてAPIを呼ぶ

```ts
// action() の中（facility-orders.tsx）
// qty_<clinicProductId> というキーを集めて、数量1以上の行だけ明細にする
const order = await api.createPurchaseOrder(accessToken, params.facilityId, {
  distributorId,
  lines, // [{ clinicProductId, quantity }, ...]
});
// = fetch POST http://localhost:8080/api/facilities/{facilityId}/orders
```

### 10-3. Go: handler → ユースケース

`ServeMux`の対応表 `"POST /api/facilities/{facilityId}/orders"` に一致し、
`internal/handler/procurement/handler.go`の`postOrder`が呼ばれる。
handlerはここでも変換だけ（認可チェック→ID/JSONパース→`Execute`呼び出し）。

### 10-4. ユースケース: 検証 → S3アップロード → 保存、の順序が肝

```go
// internal/application/procurement/create_purchase_order.go の Execute
// 手順1: 発注先の卸業者が実在するか
uc.distributorRepo.FindByID(ctx, in.DistributorID)

// 手順2: 明細ごとに クリニック商品→卸商品→卸業者 を辿り、
//        発注先の卸と一致するか検証（「1発注=1卸」の実体レベルの担保）
distributorProduct, _ := uc.distributorProductRepo.FindByID(ctx, clinicProduct.DistributorProductID())
if distributorProduct.DistributorID() != in.DistributorID { return nil, ...ErrConflict }

// 手順3: 発注時点の単価をスナップショットして明細に固定
order.AddLine(line.ClinicProductID, line.Quantity, clinicProduct.UnitPrice())

// 手順4: 明細0件ならエラー。1ステップ作成方針のため作成と同時に確定する
order.Confirm()

// 手順5: 確定したのでCSVをS3にアップロード（ここが今回の副作用）
uc.csvUploader.Upload(ctx, order, facility.Name(), time.Now(), csvLines)
//   失敗したら return error でここで打ち切り → 次のDB保存に進まない

// 手順6: アップロード成功後にようやくDB保存
uc.purchaseOrderRepo.Create(ctx, order)
//   → INSERT INTO purchase_orders / purchase_order_lines ...
```

**手順5→6の順序が業務ルール**: 「発注CSVが卸に届いて初めて確定と言える」ため、
S3アップロードが失敗したらDBには保存しない（確定したのに卸に届いていない、という
不整合を避けるため）。7章の例（検証→ドメイン生成→保存のみ）と違い、外部サービス呼び出しが
DB保存より**前**に来る点が今回のポイント。

### 10-5. 帰り道

- 成功: 201 → actionが`{ok: true, orderId}`を返す → 画面に「発注を確定しました。」+ 発注履歴に新しい行
- 失敗（卸不一致・S3アップロード失敗など）: `fmt.Errorf(...)`のメッセージが
  `httputil.WriteError`経由でエラーレスポンスになり、actionが捕まえて画面に赤字表示
