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
| `public void Bar()` | `func (f *Foo) Bar()` | `(f *Foo)`がレシーバ＝`this` |
| `int?`（null許容） | `*int`（ポインタ） | nilになれる型で「値が無い」を表す |
| `interface IFooRepository` | `type FooRepository interface` | ほぼ同じ。Goは`I`プレフィックスを付けない |
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

`*`（ポインタ）付きレシーバは「本体を書き換えられる」。C#のclassは参照型なので常にこの挙動であり、ドメインのエンティティは基本`*`付きと覚えてよい。

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
	createOrder *procapp.CreatePurchaseOrderUseCase   // 1. UseCaseをフィールドとして持つ
}

func New(createOrder *procapp.CreatePurchaseOrderUseCase, ...) *Handler {
	return &Handler{createOrder: createOrder}          // （受け取ってフィールドに入れる）
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/facilities/{facilityId}/orders", h.postOrder)  // ← C#の[HttpPost]に相当
}

func (h *Handler) postOrder(w http.ResponseWriter, r *http.Request) {
	order, err := h.createOrder.Execute(r.Context(), ...)  // 2. フィールドのUseCaseを呼ぶ
}
```

呼び方はほぼ同じ。C#は`depositBagUseCase.ExecuteAsync(...)`、Goは`h.createOrder.Execute(...)`。
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
createPurchaseOrder := procapp.NewCreatePurchaseOrderUseCase(purchaseOrderRepo, ...)  // UseCaseを作る
prochandler.New(createPurchaseOrder, purchaseOrderRepo).Register(protected)           // handlerに渡す
```

C#は「登録さえすれば、あとはフレームワークが実行時に勝手に渡してくれる」。
Goは「作って渡すコードを`main.go`に自分で書く」。この受け渡し先が[handler.go](../../backend/internal/handler/procurement/handler.go)の`New`の引数になり、`Handler`のフィールドに入る。

なので**「このUseCaseはどこで作られて渡されているのか」を知りたくなったら`cmd/api/main.go`を見る**。
C#と違ってDIコンテナが隠していないので、`main.go`に全部書いてある。

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
// backend/internal/application/procurement/create_purchase_order.go:22
purchaseOrderRepo  procdomain.PurchaseOrderRepository  // ← domain層のインターフェース。gorm実装のことは知らない
```

ユースケースがインターフェースだけを見て、実際のgorm実装は`main.go`で差し込む（8章）。
「ユースケースはDBの具体実装を知らない」というpetty-cashと同じ構造になっている。
