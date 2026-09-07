# リクエストがSQLになるまで — フロントからDBまでの1往復

「フロントの要求をGoがどう検知し、どのコードが動き、どうやってSQLが発行されるのか」を追うノート。
petty-cashの `docs/csharp/routing.md` と `usecase-domain-repository-flow.md` に相当する。

```
ブラウザ
  → Remix (:5173)  loader が fetch を実行
  → Go (:8080)     net/http が受けて handler → usecase/repository
  → gorm           Goの呼び出しをSQLに翻訳
  → PostgreSQL (:5432)
  ← 行データ → struct → ドメイン → JSON → 画面
```

実際の画面の操作を3つ選び、易しい順に1往復ずつ追う。

| 章 | 題材 | 何が新しく出てくるか |
|---|---|---|
| [0](#0-前提-リクエストごとにapiが起動するのではない) | 前提 | プロセスの寿命。全章の土台 |
| [1](#1-読み取りの1往復--クリニック一覧) | クリニック一覧（GET） | ルーティング、リポジトリ、gorm→SQL |
| [2](#2-書き込みの1往復--商品マスタ画面の登録ボタンからinsertまで) | 商品マスタの「登録」（POST） | フォーム、ユースケース、業務ルール、エラー表示 |
| [3](#3-書き込みの発展--カート方式の2往復とs3アップロード) | 発注のカートと確定（POST×2） | 1画面から複数API、副作用の順序、S3。本文は[別ノート](../purchase-order-flow.md)へ切り出し |
| [4](#4-逆引き早見表) | 逆引き早見表 | 「このボタンのコードはどこか」を引く用 |

---

## 0. 前提: 「リクエストごとにAPIが起動する」のではない

`go run ./cmd/api` した時点で `cmd/api/main.go` の `main()` が一度だけ実行され、
最後の `http.ListenAndServe(":8080", ...)` で**プロセスがTCPポート8080を開いて待ち続ける**。

開発時に打つのは `cd backend && ./run.sh` で、[`run.sh`](../../backend/run.sh)が`.env`を読み込んで
環境変数にしてから`go run ./cmd/api`を実行する（C#の`launchSettings.json` + `dotnet run`に相当）。
このターミナルは`Ctrl+C`で止めるまで開いたままになり、その状態が「待ち受けている」ということ。
本番では人が打つのではなく、`go build`したバイナリをsystemdやDockerが起動して常駐させる。

```go
// cmd/api/main.go（末尾）
if err := http.ListenAndServe(addr, httputil.CORS(mux)); err != nil {
```

- 「検知」の正体: OSカーネルが「8080宛のTCP接続はこのプロセスへ」と橋渡しする（ソケットのlisten/accept）
- リクエストが来るたびに起動するのではなく、**常駐プロセスに接続が渡される**
- handlerもユースケースもリポジトリも`main()`で1個ずつ作られ、全リクエストで使い回される
- petty-cash対応: ASP.NET Coreの`app.Run()`でKestrelが5001を待ち受けるのと同じ

---

## 1. 読み取りの1往復 — クリニック一覧

クリニック一覧画面を開いたとき（`GET /api/facilities`）の1往復を、最初から最後まで追う。
段階と、petty-cashでの相当物はこの対応表の通り。

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

### 1-1. フロント側: loaderがfetchする

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

### 1-2. Go側: ServeMuxが対応表を引いてhandler関数を呼ぶ

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

### 1-3. handler → repository（または usecase）→ gorm

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

### 1-4. gorm: Goの呼び出しがSQL文字列になる瞬間

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

### 1-5. 実際に発行されるSQLを目で見る方法

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

## 2. 書き込みの1往復 — 商品マスタ画面の「登録」ボタンからINSERTまで

1章はGET（読み取り）だった。ここでは**実際に画面にあるボタン**で書き込みの1往復を追う。
商品マスタ画面（`/facilities/:id/products`）で卸商品を選び、商品コードと発注点を入れて
「登録」ボタンを押したときの流れ。

読み取りとの違いは2つ。**ユースケースを経由すること**と、**業務ルールに引っかかったときの
エラーが画面の文言になること**。

### 2-1. 画面: フォームのsubmit

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

### 2-2. フロント→Go: actionがAPIを呼ぶ

```ts
// action() の中 → api.server.ts
await api.registerClinicProduct(params.facilityId, { productCode, ..., reorderPoint });
// = fetch POST http://localhost:8080/api/facilities/{facilityId}/products （JSONボディ）
```

### 2-3. Go: handlerが受けてユースケースへ

ServeMuxの対応表 `"POST /api/facilities/{facilityId}/products"` に一致し、
`internal/handler/productcatalog/handler.go` の `postProduct` が呼ばれる。
handlerがやるのは変換だけ: パスの`{facilityId}`をIDに(`ParseID`)、JSONボディをstructに(`DecodeJSON`)、
そしてユースケースの`Execute`を呼ぶ。**業務判断はしない**。

### 2-4. ユースケース: 業務の手順書（1行=1手順、SQL付きで読む）

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

### 2-5. 帰り道: 結果が画面の文言になるまで

- 成功: handlerが201でJSONを返す → actionが`{ok: true}`を返す → 画面に「登録しました。」
- 重複: 手順3の`ErrConflict`を`httputil.WriteError`が**409**に変換 →
  actionが`ApiError`を捕まえて`{ok: false, error: "商品コード C-1001 は...使われています"}` →
  画面に赤字でそのまま表示される

つまり画面の赤いエラーメッセージの出どころは、ユースケースの`fmt.Errorf(...)`の文字列。

---

## 3. 書き込みの発展 — カート方式の2往復とS3アップロード

この章は [docs/purchase-order-flow.md](../purchase-order-flow.md) に切り出した。

発注画面（`/facilities/:id/orders`）の「カートに追加」「発注する」「取消」を追い、
確定時に発注CSVがS3へ上がるまでを扱う。1〜2章で出てこない要素は次の3つ。

- 1画面から`intent`で3種類のAPIを撃ち分ける
- 外部サービス(S3)への副作用と、DB保存との**順序**（[§5](../purchase-order-flow.md#5-往復bのユースケース-確定--s3アップロード--保存の順序が肝)）
- インターフェースを挟んでAWS SDKまで降りる4段構造（[§6](../purchase-order-flow.md#6-uploadの中身-インターフェースからaws-sdkまで4段)）

---

## 4. 逆引き早見表

ここまでは「操作からコードへ」と順方向に追った。ここは逆に、既にあるコードや画面から
入口を引くための表。

### 4-1. ユースケースを起点に読む

どのユースケースでも、知りたいこと別に見る場所は同じ。

| 知りたいこと | 見る場所 |
|---|---|
| どのURL/ボタンから呼ばれるか | `internal/handler/<コンテキスト>/handler.go` の `Register()`（URL対応表） |
| 業務の手順（何をどの順で） | `internal/application/<コンテキスト>/` の `Execute` の中 |
| 業務ルール（何が許されないか） | `internal/domain/<コンテキスト>/` の `New...` 関数（`errors.New`の行） |
| どんなSQLになるか | `internal/infrastructure/<コンテキスト>/` のリポジトリ実装 |

逆引き（ボタン→コード）の探し方: フロントの`api.server.ts`でそのボタンが叩くURLパスを見て、
そのパスをhandlerの`Register()`から探すと入口の関数が見つかる。あとは`Execute`へ降りるだけ。

### 4-2. URLから画面ファイルを特定する

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
`Register()`がURLパターン→関数の対応表になっている（1-2参照）。
