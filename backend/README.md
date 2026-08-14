# clinic-inventory backend

Go + gorm。設計方針は[../docs/requirements.md](../docs/requirements.md)と[../docs/architecture/domain-rules.md](../docs/architecture/domain-rules.md)を参照。

## ビルド・テスト

このマシンでは `go test` / `go build` をcgo有効（デフォルト）で実行すると、テストバイナリが `dyld: missing LC_UUID load command` で異常終了する（Go 1.22の外部リンカとこのマシンのXcodeコマンドラインツールの組み合わせに起因するローカル環境の問題で、コードの不具合ではない）。**`CGO_ENABLED=0` を付けて実行すること**。

```bash
CGO_ENABLED=0 go build ./...
CGO_ENABLED=0 go test ./...
```

## DB接続

ローカルではHomebrewの`postgresql@16`を使用（`brew services list`で起動確認済み）。`psql`等のクライアントコマンドはPATHに入っていないため、都度以下を通す。

```bash
export PATH="/usr/local/opt/postgresql@16/bin:$PATH"
```

DB作成（初回のみ）:

```bash
createdb -h localhost -U "$(whoami)" clinic_inventory
```

接続文字列はデフォルトで `host=localhost user=apple dbname=clinic_inventory port=5432 sslmode=disable`（`cmd/api/main.go`の`dsn()`）。環境変数`DATABASE_DSN`で上書き可能。

テーブルは起動時の`AutoMigrate`で自動作成される。

## 起動

```bash
# APIサーバー（デフォルト :8080。PORTで変更可）
CGO_ENABLED=0 go run ./cmd/api

# 縦スライスの動作確認スクリプト（サーバーを立てずユースケースを直接実行）
CGO_ENABLED=0 go run ./cmd/demo
```

`listen tcp :8080: bind: address already in use` で起動に失敗する場合、同じポートで前回のプロセスが起動したままになっている（`./run.sh`をバックグラウンドで動かした後に停止し忘れた等）。該当プロセスを止めてから起動し直す。

```bash
lsof -ti:8080 -sTCP:LISTEN | xargs kill
./run.sh
```

（フロントの`:5173`/`:5174`が同様の状態になった場合もポート番号を読み替えて同じ手順でよい）

## APIエンドポイント

| メソッド/パス | 内容 |
|---|---|
| GET /api/health | ヘルスチェック |
| POST /api/corporations | 法人作成 `{name}` |
| POST /api/facilities | クリニック作成 `{name, facilityType(medical/dental/vet), corporationId}` |
| GET /api/facilities | クリニック一覧 |
| POST /api/distributors | 卸業者作成 `{name}` |
| GET /api/distributors | 卸業者一覧 |
| POST /api/distributors/{id}/products | 卸商品登録 `{distributorProductCode, name, vendorName, vendorProductCode?, janCode?}` |
| GET /api/distributors/{id}/products | 卸商品一覧 |
| POST /api/facilities/{id}/products | クリニック商品登録 `{productCode, distributorProductId, name?, janCode?, reorderPoint}`。name/JAN省略時は卸商品から引き継ぐ |
| GET /api/facilities/{id}/products | クリニック商品一覧。`?jan=XXX`でJAN引き当て（バーコード消費の入口） |

エラーは `{"error": "..."}` 形式。バリデーション→400、重複（商品コード等）→409、未存在→404。

## ディレクトリ構成

petty-cashと同じ**レイヤー最上位**の構成。最上位がレイヤー（domain / application / infrastructure / handler）、その下がコンテキスト（[docs/architecture/domain-rules.md](../docs/architecture/domain-rules.md)のコンテキスト一覧に対応）。

| petty-cash (C#) | clinic-inventory (Go) |
|---|---|
| PettyCash.Domain | internal/domain/ |
| PettyCash.Application | internal/application/ |
| PettyCash.Infrastructure | internal/infrastructure/ |
| PettyCash.Api (Controllers) | internal/handler/ + cmd/api/ |

HTTP層のパッケージ名は、`interface`がGoの予約語のため`handler`とした。書き込みはユースケース経由、一覧などの単純な読み取りはハンドラからリポジトリを直接使う。

### レイヤー間の依存の向き（依存性の逆転）

リポジトリのインターフェースは`domain/`に置き、gormを使った実装は`infrastructure/`に置く
（例: [domain/procurement/repository.go](internal/domain/procurement/repository.go) と
[infrastructure/procurement/purchase_order_repository.go](internal/infrastructure/procurement/purchase_order_repository.go)）。

domain/applicationはインターフェースだけを見ていて、gormもDBも知らない。逆にinfrastructureが
domainの決めたインターフェースに合わせる。この「上位レイヤーがインターフェースを所有し、
下位レイヤーがそれに従う」向きが依存性の逆転で、DB実装を差し替えてもdomain/applicationは変更不要になる。
実際にどの実装を差し込むかは`cmd/api/main.go`で決めている（C#のDIコンテナ登録に相当。
書き方の対応は[docs/go/go-for-csharp.md 8-9章](../docs/go/go-for-csharp.md)）。

```
internal/
  domain/                        … 業務ルールの在り処（主役）
    shared/                      … 共通の値オブジェクト（ID）、センチネルエラー（ErrNotFound/ErrConflict）
    organization/                … Facility, Corporation + リポジトリIF
    distributorcatalog/          … Distributor, DistributorProduct + リポジトリIF
    productcatalog/              … ClinicProduct + リポジトリIF
  application/                   … ユースケース（ドメインを組み合わせる指揮者）
    organization/                … CreateCorporationUseCase, CreateFacilityUseCase
    distributorcatalog/          … CreateDistributorUseCase, RegisterDistributorProductUseCase
    productcatalog/              … RegisterClinicProductUseCase
                                   （卸商品の実在・廃盤チェック、名前/JANの引き継ぎもここ）
  infrastructure/                … DBへの読み書き。業務ルールは知らない
    database/                    … DB接続（gorm.Open）
    organization/                … gormモデル・リポジトリ実装
    distributorcatalog/          … 同上。卸商品コードの一意性は
                                   (distributor_id, distributor_product_code) のユニーク制約が最終防衛線
    productcatalog/              … 同上。商品コードの一意性は
                                   (facility_id, product_code) のユニーク制約が最終防衛線
  handler/                       … HTTPの受け口。業務ルールは知らない
    httputil/                    … JSONレスポンス、エラー→HTTPステータス変換、CORS
    organization/ distributorcatalog/ productcatalog/
cmd/
  api/                           … HTTPサーバー（:8080）。DB接続・マイグレーション・DI配線
  demo/                          … 縦スライスの動作確認スクリプト（ユースケース直接実行）
```

発注（procurement）・在庫（inventory）コンテキストは未実装。追加時は各レイヤーの下にコンテキストのディレクトリを足す。
