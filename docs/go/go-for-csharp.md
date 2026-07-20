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
