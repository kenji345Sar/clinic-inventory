# C#経験者のためのGo読み方ノート

petty-cash（C#）とclinic-inventory（Go）は同じDDDの層構成で書かれている。
このノートは「petty-cashのコードは読めるが、Goの構文に慣れない」状態から、
clinic-inventoryのドメイン層を読めるようになるための対応表。

実際のコードで対応を確認するときは、この2ファイルを並べて開くとよい。

- C#: `petty-cash/backend/PettyCash.Domain/PettyCash/Ledger/PettyCashTransaction.cs`
- Go: `clinic-inventory/backend/internal/domain/distributorcatalog/distributor_product.go`

---

## 対応表（早見）

| C# | Go | 補足 |
|---|---|---|
| `namespace PettyCash.Domain.Ledger` | `package distributorcatalog` | Goはディレクトリ＝パッケージ |
| `using PettyCash.Domain.Shared;` | `import shareddomain "clinic-inventory/internal/domain/shared"` | `shareddomain`は別名（エイリアス） |
| `public class Foo` | `type Foo struct {...}` ＋ 関数群 | 後述。classは無い |
| `public int Id { get; private set; }` | 小文字フィールド＋大文字getterメソッド | 後述。アクセス修飾子も無い |
| コンストラクタ / `static Create()` | `NewFoo(...)` 関数 | 命名規約。`New`+型名 |
| `throw new ArgumentException(...)` | `return nil, errors.New(...)` | 例外は無い。エラーは戻り値 |
| `public void Bar()` | `func (f *Foo) Bar()` | `(f *Foo)`がレシーバ＝`this`。拡張メソッドの`this Foo f`と同じ位置 |
| `int?`（null許容） | `*int`（ポインタ） | nilになれる型で「値が無い」を表す |
| `interface IFooRepository` | `type FooRepository interface` | ほぼ同じ。Goは`I`プレフィックスを付けない |
| `record CreateFooInput(...)` | `type CreateFooInput struct` | DTOも同じ`struct`。役割の見分けは後述 |
| `enum FacilityType` | `type FacilityType string` ＋ `const` | `type`はstruct以外にも付く。後述 |
| `List<T>` | `[]T`（スライス） | `T[]`（固定長配列）は`[5]T`で別物。後述 |
| `/// <summary>...</summary>` | `// コメント`（型・関数の直前） | 業務ルールはここに書く方針も同じ |
| `internal`アクセス修飾子 | `internal/` ディレクトリ | 後述。ディレクトリ名で公開範囲を制御する |
| `builder.Services.AddScoped<T>()` | `cmd/api/main.go`に手書きの配線 | 後述。DIコンテナが無い |

---

## 1. classが無い → struct＋関数で同じことをする

C#のclassは「データ＋メソッド＋アクセス制御」のセット。Goはこれを分けて書く。

```csharp
// C#（petty-cash流）
public class DistributorProduct
{
    public Guid Id { get; private set; }
    public string Name { get; private set; }

    public void Discontinue() { ... }
}
```

```go
// Go（clinic-inventory流）— 上と同じ意味
type DistributorProduct struct { // ← データ部分
	id   shareddomain.ID
	name string
}

func (p *DistributorProduct) ID() shareddomain.ID { return p.id }   // ← getter
func (p *DistributorProduct) Name() string        { return p.name } // ← getter

func (p *DistributorProduct) Discontinue() { ... } // ← メソッド
```

### `type`で作れるものは`struct`だけではない

`type`は「新しい型を1つ作る」宣言で、後ろに来るものは4種類ある（backend/internal での出現数）。

| 出現数 | Go | C# |
|---|---|---|
| 62 | `type Foo struct { ... }` | `class` / `record` |
| 8 | `type Foo interface { ... }` | `interface` |
| 2 | `type FacilityType string` ＋ `const`群 | `enum` |
| 1 | `type ID uuid.UUID` | ラッパー型（`readonly record struct`） |

csv-functions側には関数型もある。

```go
type ParseFunc func(body []byte) ([]ingapp.CatalogRow, error)   // C#の delegate / Func<>
```

`type ID uuid.UUID`は**別名ではなく別の型**になる。中身が`uuid.UUID`でも代入には変換が要るので、
`FacilityID`と`OrderID`を取り違えるとコンパイルエラーになる。C#の`record struct`で
プリミティブを包むのと同じ目的。

### `struct`は3つの役割を兼ねる — 見分けはフィールドの大文字/小文字

C#は`record`（DTO）と`class`（サービス・エンティティ）で見た目が分かれるが、Goはすべて`type X struct`。
役割はフィールドの頭文字と接尾辞から読む。

| | DTO | サービス | エンティティ |
|---|---|---|---|
| 実例 | `CreateDistributorInput` | `ConfirmPurchaseOrderUseCase` | `DistributorProduct` |
| フィールド | **大文字**（`Code`, `Name`） | **小文字**（`purchaseOrderRepo`） | **小文字**（`id`, `name`） |
| フィールドの型 | 素の値（`string`, `int`） | リポジトリ・インターフェース | 素の値＋値オブジェクト |
| 外からの扱い | 直接詰める | 触らせない | getter で読むだけ |
| 生成のタイミング | リクエストごと | `main.go`で1個 | 業務データとして都度 |
| メソッド | 無し | `Execute` | `Discontinue()`など業務操作 |
| 接尾辞 | `Input` / `Output` / `Row` | `UseCase` / `Repository` / `Handler` | 集約名そのもの |
| C# | `record` | `class`（primary constructor） | `class` |

---

## 2. アクセス修飾子が無い → 大文字/小文字で決まる

Goには`public`/`private`キーワードが無い。**識別子の頭文字**で決まる。

- 小文字始まり（`id`, `name`, `discontinued`）… パッケージ外から見えない ＝ `private`
- 大文字始まり（`ID()`, `Name()`, `Discontinue()`）… パッケージ外から見える ＝ `public`

clinic-inventoryのドメインは「フィールドは全部小文字（private）、公開したいものだけ大文字のgetterを生やす」で、C#の`{ get; private set; }`と同じ「外から読めるが勝手に書き換えられない」を実現している。

**ドメインの不変条件を守る仕組みがこれ**。フィールドがprivateなので、外部（ユースケースやインフラ層）は`NewDistributorProduct()`や`Discontinue()`など、バリデーション付きの入口を通らないと状態を作れない・変えられない。

---

## 3. コンストラクタが無い → `New型名()` ファクトリ関数

```csharp
// C#: petty-cashのChangeBag.CreateDeposit()と同じパターン
public static DistributorProduct Create(string code, ...)
{
    if (string.IsNullOrEmpty(code)) throw new ArgumentException("卸商品コードは必須です");
    return new DistributorProduct { ... };
}
```

```go
// Go: 戻り値が (*DistributorProduct, error) の2つになるのがC#との最大の違い
func NewDistributorProduct(distributorID shareddomain.ID, code, name, vendorName string) (*DistributorProduct, error) {
	if code == "" {
		return nil, errors.New("卸商品コードは必須です") // ← throwに相当
	}
	return &DistributorProduct{...}, nil // ← 正常時。エラー側はnil
}
```

`&DistributorProduct{...}`の`&`は「structを作ってそのポインタを返す」。C#の`new`に相当すると読んでよい。

`New`は言語機能ではなく命名規約で、中身はただの関数。だから戻り値を自由に決められる。

| | Go | C#のコンストラクタ |
|---|---|---|
| 失敗しうる（業務ルール検証あり） | `func NewDistributorProduct(...) (*DistributorProduct, error)` | 戻り値を持てないので`throw`するしかない |
| 失敗しない（配線するだけ） | `func NewDistributorProductRepository(db *gorm.DB) *DistributorProductRepository` | 同じ |

名前の付け方も2通りある。

| 形 | 実例 | 使うとき |
|---|---|---|
| `New` ＋ 型名 | `procapp.NewConfirmPurchaseOrderUseCase(...)` | 同じパッケージに複数の型がある |
| `New` だけ | `prochandler.New(...)` | パッケージ名で何を作るか自明。`handler.NewHandler`は冗長になるため |

---

## 4. 例外が無い → エラーは戻り値で伝播する

C#は`throw`すると呼び出し元を突き抜けてcatchまで飛ぶ。Goは飛ばない。
関数が`(結果, error)`を返し、呼び出し側が毎回チェックして上に返す。

```go
// ユースケース層（application/）の典型パターン
product, err := distdomain.NewDistributorProduct(...)
if err != nil {
	return nil, err // ← ドメインのエラーをそのまま上（handler層）へ返す
}
```

この`if err != nil { return nil, err }`はGoの定型句で、C#で例外が自動でやっている「エラーの伝播」を手書きしている。clinic-inventoryでは最終的にhandler層の`httputil.WriteError()`がエラーをHTTPステータス（400/404/409）に変換する。petty-cashのMiddlewareでの例外→HTTPレスポンス変換と同じ位置づけ。

---

## 5. レシーバ = 明示的な`this`

```go
func (p *DistributorProduct) Discontinue() {
	p.discontinued = true
}
```

`(p *DistributorProduct)`が「このメソッドはDistributorProductに付く」という宣言で、`p`はC#の`this`。`p.discontinued = true`は`this.discontinued = true`と同じ。

### C#の拡張メソッドと同じ形

C#でも、classの外にメソッドを書くときは同じ位置に型を書いている。

```csharp
// C#（拡張メソッド）
public static class ProductExtensions
{
    public static void Discontinue(this DistributorProduct p) { ... }
    //                             ^^^^^^^^^^^^^^^^^^^^^^^^^
}
```

```go
// Go — 上と同じ位置に同じものを書く
func (p *DistributorProduct) Discontinue() { ... }
//   ^^^^^^^^^^^^^^^^^^^^^^^
```

C#の`this DistributorProduct p`が、Goでは関数名の前の丸括弧に出ているだけ。Goのメソッドは全部この形で書く。

### 読む順番

`func (p *DistributorProduct) ID() shareddomain.ID`

左から読むと詰まるので、真ん中→右→左の順で読む。

| 読む順 | Goの部分 | C#で言うと |
|---|---|---|
| ① | `(p *DistributorProduct)` | `class DistributorProduct { ... }` の内側。`p`が`this` |
| ② | `ID()` | メソッド名と引数リスト |
| ③ | `shareddomain.ID` | 戻り値の型。C#はメソッド名の左、Goは右 |
| ④ | `func` | 対応するものは無い（関数宣言の目印） |

`(...)`が2つ並んでいたら、1つ目がレシーバ（所属する型）、2つ目が引数。

### `*`（ポインタ）レシーバ

`*`付きレシーバは「本体を書き換えられる」。

| C# | Go | 補足 |
|---|---|---|
| `class`（参照型・常に本体を渡す） | `func (p *DistributorProduct) ...` | 書き換えが呼び出し元に反映される |
| `struct`（値型・コピーが渡る） | `func (p DistributorProduct) ...` | 書き換えてもコピーが変わるだけ |

C#は`class`と宣言した時点で参照型が確定するので、呼び出し側では意識しなくてよかった。Goのstructは既定でコピーが渡るため、メソッドごとに`*`を書いて指定する。ドメインのエンティティは基本`*`付きと覚えてよい。

---

## 6. `internal/` ディレクトリ = C#の`internal`修飾子

`internal`はGoで予約された特別なディレクトリ名。**`internal/`配下のパッケージは、その親（このプロジェクトなら`clinic-inventory`モジュール）の中からしかimportできない**ことをコンパイラが強制する。外部プロジェクトが`import "clinic-inventory/internal/domain/..."`と書くとコンパイルエラーになる。

C#の`internal`アクセス修飾子（同一アセンブリ内のみ参照可）と同じ発想で、名前もそこから来ている。C#はクラスごとにキーワードで公開範囲を指定するが、Goは「`internal/`ディレクトリに置くかどうか」で指定する。

clinic-inventoryでは、ドメイン・ユースケース・DB実装・ハンドラすべてが「このアプリの内部実装」なので`internal/`配下に置き、エントリポイント（`cmd/`）だけ外にある。

---

## 7. 読むときの地図（petty-cashとの対応）

| 知りたいこと | petty-cash | clinic-inventory |
|---|---|---|
| 業務ルール | PettyCash.Domain の各クラス | `internal/domain/` の各struct＋`New...`関数 |
| ユースケース | PettyCash.Application/UseCases | `internal/application/` |
| DB実装 | PettyCash.Infrastructure | `internal/infrastructure/` |
| HTTP受け口 | PettyCash.Api/Controllers | `internal/handler/` |

どちらも「業務で『やってはいけないこと』はドメイン層で弾く」方針。
C#では`throw`の行を探す、Goでは`return nil, errors.New(...)`の行を探す——見つかる場所が業務ルールの在り処。

---

## 8. Controller / Handler が UseCase をどう呼ぶか

### まず名前の話：なぜGoは「Controller」でなく「Handler」なのか

ただの名前で、役割はC#のControllerと同じ（HTTPを受けて、UseCaseを呼んで、レスポンスを返す）。
`Controller`という名前にしても動く。土台のライブラリが使っている単語に合わせているだけ。

| | フレームワークが使う言葉 | だからこう名付ける |
|---|---|---|
| C# (ASP.NET Core) | `ControllerBase`, `[ApiController]` | `BagsController` |
| Go (net/http) | `http.Handler`, `HandleFunc` | `Handler` |

Goは標準ライブラリ`net/http`が、HTTPを受ける型を`Handler`、登録メソッドを`HandleFunc`と呼んでいるので、
それに合わせて`Handler`と名付ける習慣がある。C#が`ControllerBase`を継承する前提で「Controller」が標準語なのと同じ。

### 本題：やることは同じ3ステップ

C#のController、Goのhandlerも、やることは同じ。「UseCaseを受け取る → 自分の中に持つ → メソッドで呼ぶ」。

### C#（petty-cash）

```csharp
// petty-cash/backend/PettyCash.Api/Controllers/BagsController.cs
public class BagsController(
    DepositBagUseCase depositBagUseCase) : ControllerBase   // 1. コンストラクタでUseCaseを受け取る
{
    [HttpPost("deposit")]
    public async Task<ActionResult<ChangeBagDto>> Deposit(DepositRequestDto dto)
    {
        var bag = await depositBagUseCase.ExecuteAsync(dto);  // 2. 受け取ったUseCaseを呼ぶ
        return CreatedAtAction(...);
    }
}
```

### Go（clinic-inventory）

```go
// backend/internal/handler/procurement/handler.go
type Handler struct {
	saveDraftOrder *procapp.SaveDraftPurchaseOrderUseCase  // 1. UseCaseをフィールドとして持つ
}

func New(saveDraftOrder *procapp.SaveDraftPurchaseOrderUseCase, ...) *Handler {
	return &Handler{saveDraftOrder: saveDraftOrder}         // （受け取ってフィールドに入れる）
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/facilities/{facilityId}/orders", h.postOrder)  // ← C#の[HttpPost]に相当
}

func (h *Handler) postOrder(w http.ResponseWriter, r *http.Request) {
	order, err := h.saveDraftOrder.Execute(r.Context(), ...)  // 2. フィールドのUseCaseを呼ぶ
}
```

呼び方はほぼ同じ。C#は`depositBagUseCase.ExecuteAsync(...)`、Goは`h.saveDraftOrder.Execute(...)`。
どちらも「保持しているUseCaseの`Execute`を呼ぶ」だけ。

### 「このメソッドはPOST /xxxを処理する」の書き方の違い

C#はメソッドの真上に`[HttpPost("deposit")]`という**属性**を付けて、その場でURLと結びつける。
Goには属性が無いので、`Register()`の中に`mux.HandleFunc("POST ...", h.postOrder)`と**別の場所に一覧で書く**。

| | C# (petty-cash) | Go (clinic-inventory) |
|---|---|---|
| メソッドとURLの結びつけ | メソッド直上の属性 `[HttpPost("deposit")]` | `Register()`内の `mux.HandleFunc("POST /...", h.postOrder)` |
| どこに書くか | 各メソッドの真上 | `Register()`に全ルートをまとめて記述 |

なのでGoで「このURLはどの関数が処理するのか」を知りたいときは、メソッドの上ではなく
`handler.go`の`Register()`を見る。ここがC#でいうルート属性の一覧に相当する。

### 違うのは1点だけ：そのUseCaseを誰が用意するか

| | C# (petty-cash) | Go (clinic-inventory) |
|---|---|---|
| UseCaseのインスタンス | DIコンテナが自動で作って渡す | `main.go`で自分で作って渡す |
| 書くコード | `Program.cs`に`builder.Services.AddScoped<DepositBagUseCase>();`の登録1行だけ | `main.go`に生成と受け渡しを手書き |

```go
// backend/cmd/api/main.go — C#のDIコンテナがやっていた「生成して渡す」を手書きしている
saveDraftPurchaseOrder := procapp.NewSaveDraftPurchaseOrderUseCase(purchaseOrderRepo, ...)  // UseCaseを作る
prochandler.New(saveDraftPurchaseOrder, confirmPurchaseOrder, ...).Register(protected)      // handlerに渡す
```

C#は「登録さえすれば、あとはフレームワークが実行時に勝手に渡してくれる」。
Goは「作って渡すコードを`main.go`に自分で書く」。この受け渡し先が[handler.go](../../backend/internal/handler/procurement/handler.go)の`New`の引数になり、`Handler`のフィールドに入る。

なので**「このUseCaseはどこで作られて渡されているのか」を知りたくなったら`cmd/api/main.go`を見る**。
C#と違ってDIコンテナが隠していないので、`main.go`に全部書いてある。

### DIコンテナが無いと、具体的にどの3箇所が変わるか

UseCaseがhandlerに届くまでには①生成②受け渡し③保持の3段がある。C#はこのうち①②が
コンテナの中で実行時に起きるためコードに現れない。Goは①②が`main.go`に並んで書かれる。

| 段階 | C# (petty-cash) | Go (clinic-inventory) |
|---|---|---|
| ① 生成する | コンテナが実行時に生成（書くのは`Program.cs`の`AddScoped<T>()`1行だけ） | `main.go`に`procapp.New...UseCase(...)`と手書き |
| ② handlerに渡す | コンストラクタ引数に型を書けば自動で注入 | `main.go`で`prochandler.New(...)`の引数に手渡し |
| ③ handlerが保持する | コンストラクタ引数（primary constructor） | 構造体のフィールド |

`ConfirmPurchaseOrderUseCase`の実物で並べるとこうなる。

```go
// ① backend/cmd/api/main.go:101 — 依存する5つのリポジトリとS3アップローダを渡して生成
confirmPurchaseOrder := procapp.NewConfirmPurchaseOrderUseCase(
	purchaseOrderRepo, distributorRepo, distributorProductRepo, clinicProductRepo, facilityRepo, orderCsvUploader)

// ② backend/cmd/api/main.go:109 — handlerのNewに渡す
prochandler.New(saveDraftPurchaseOrder, confirmPurchaseOrder, removeDraftPurchaseOrder, purchaseOrderRepo).Register(protected)

// ③ backend/internal/handler/procurement/handler.go:16 — フィールドとして保持
confirmOrder *procapp.ConfirmPurchaseOrderUseCase
```

### 配線を間違えたときの現れ方の違い

| | C# (petty-cash) | Go (clinic-inventory) |
|---|---|---|
| 登録し忘れ | ビルドは通り、実行時に`InvalidOperationException: Unable to resolve service` | `New(...)`の引数が足りずコンパイルエラー |
| UseCaseの依存を1つ増やしたとき | コンストラクタに1行足すだけ（登録済みなら`Program.cs`は変更不要） | `New...UseCase(...)`の呼び出し側（`main.go`）も直す必要がある |

C#は「登録さえ済んでいれば追加の依存はコンテナが解決するが、登録漏れは動かすまで分からない」。
Goは「依存を増やすたび`main.go`を直す手間がかかるが、直し忘れはコンパイルが通らない」。

### 配線の探し方

Goには「コンテナの中」に相当する場所が無いので、UseCaseの**型名でgrepすると配線の全体が出る**。
出てくるのは必ず「定義」「①生成（`main.go`）」「③フィールド（`handler.go`）」の3種類。

```
grep -rn "ConfirmPurchaseOrderUseCase" backend/
```

C#で`GetVendorDashboardUseCase`をF12で追うとコンストラクタ引数までしか辿れず、
「誰がこれを生成しているのか」はコンテナの中なのでコードに無い。Goはその1箇所が
必ず`main.go`にあるので、grepの結果がそのまま答えになる。

---

## 9. リポジトリの「抽象と実装の分け方」— C#とGoの対応

petty-cashで`IPurchaseOrderRepository`（インターフェース）とその実装クラスを分けていたのと同じことを、
clinic-inventoryでもやっている。書き方の対応はこれだけ。

| | petty-cash (C#) | clinic-inventory (Go) |
|---|---|---|
| インターフェースの宣言 | `interface IPurchaseOrderRepository` | `type PurchaseOrderRepository interface` |
| インターフェースの置き場所 | `PettyCash.Domain/<集約>/`（例: `Domain/Vendor/BagManagement/`） | `internal/domain/<集約>/`（例: `internal/domain/procurement/`） |
| 実装の宣言 | `class PurchaseOrderRepository : IPurchaseOrderRepository` | `type PurchaseOrderRepository struct`（`implements`宣言なし） |
| 実装の置き場所 | `PettyCash.Infrastructure/Repositories/` | `internal/infrastructure/<集約>/` |
| ユースケースが受け取る型 | `IPurchaseOrderRepository` | `PurchaseOrderRepository` |

命名の違いも1つある。C#はインターフェースに`I`を付け、実装クラスは`I`なしで別名にする
（`IPurchaseOrderRepository` と `PurchaseOrderRepository`）。Goは`I`を付けない慣習なので、
インターフェースも実装も同じ`PurchaseOrderRepository`という名前になる（パッケージが`procdomain`と`procinfra`で
違うので衝突しない）。

置き場所は**どちらも「Domain層を集約ごとのフォルダに分けて、その中にインターフェースを置く」**で同じ。
Goのフォルダ名が具体的に見えるのは、Goが「1フォルダ = 1パッケージ」という言語仕様で、集約を表すには
必ずその名前のフォルダを作る必要があるため（フォルダ＝集約の区切りが強制される）。C#は`namespace`とフォルダが
独立していてフォルダを分けなくても名前空間で区切れるが、petty-cashは慣習として同じく集約ごとにフォルダを切っている。

**インターフェース（domain層）**
```go
// backend/internal/domain/procurement/repository.go:9
type PurchaseOrderRepository interface {
	Create(ctx context.Context, order *PurchaseOrder) error
	FindByID(ctx context.Context, id shareddomain.ID) (*PurchaseOrder, error)
	...
}
```

**実装（infra層）**
```go
// backend/internal/infrastructure/procurement/purchase_order_repository.go:15
type PurchaseOrderRepository struct { ... }
func (r *PurchaseOrderRepository) Create(...) error { ... }
```

C#と1つだけ違うのは、`: IPurchaseOrderRepository`のような「実装します」という宣言が無いこと。
Goは**同じメソッドを揃えていれば自動的にインターフェースを満たした扱いになる**ので、どこにも「実装宣言」が書かれない。

**ユースケースはインターフェースだけを持つ（C#と同じ）**
```go
// backend/internal/application/procurement/save_draft_purchase_order.go:24
purchaseOrderRepo  procdomain.PurchaseOrderRepository  // ← domain層のインターフェース。gorm実装のことは知らない
```

ユースケースがインターフェースだけを見て、実際のgorm実装は`main.go`で差し込む（8章）。
「ユースケースはDBの具体実装を知らない」というpetty-cashと同じ構造になっている。

---

## 10. `List<T>` が無い → スライス `[]T`

```go
// confirm_purchase_order.go:64
csvLines := make([]PurchaseOrderCsvLine, 0, len(order.Lines()))   // 長さ0・容量は明細数
csvLines = append(csvLines, PurchaseOrderCsvLine{ ... })          // 足すたびに伸びる
```

```csharp
var csvLines = new List<PurchaseOrderCsvLine>(order.Lines.Count);
csvLines.Add(new PurchaseOrderCsvLine { ... });
```

| C# | Go | |
|---|---|---|
| `List<T>` | `[]T`（スライス） | 伸びる。実務ではほぼこれ |
| `T[]` | `[5]T`（配列） | 長さが型に含まれる。長さが違えば別の型 |
| `new List<T>(capacity)` | `make([]T, 0, capacity)` | 第2引数が長さ、第3引数が容量 |
| `list.Add(x)` | `list = append(list, x)` | **代入し直しが必要** |

`append`が代入を伴うのは、伸ばすときに別のメモリへ移ることがあり、戻り値を受け取らないと
反映されないため。`list.Add(x)`の感覚で`append(list, x)`と書き捨てると要素が増えない。
