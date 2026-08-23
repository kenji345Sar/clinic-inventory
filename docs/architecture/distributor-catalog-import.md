# 卸業者の商品マスタCSV取り込み(商品マスタ側から見た設計)

複数の卸業者から送付される商品マスタCSVをS3経由で受け取り、`distributor_products`に反映する仕組みについて、
**backend側が持つ責務**をまとめる。

> **取り込み処理そのものは別リポジトリ [clinic-inventory-csv-functions](../../../clinic-inventory-csv-functions) にある。**
> パイプラインの設計(S3キーの読み取り・卸ごとのフォーマット差の吸収・冪等性・起動方法)は
> そちらの `docs/design.md` を参照。このドキュメントは「反映される側」の設計を扱う。

- 位置づけ: [domain-rules.md「卸連携CSV基盤」](domain-rules.md#卸連携csv基盤distributorcsvingestionコンテキスト)の3種類のCSVのうち「商品マスタ・価格表CSV」
- S3バケット・IAMの実設定は[s3-storage.md](s3-storage.md)
- 別種のCSV: 受注確定CSV(卸の引き当て結果・納入単価)の受け皿は未実装。論点は[order-acknowledgement-import.md](order-acknowledgement-import.md)

最終更新: 2026-08-14

---

## 1. なぜリポジトリを分けたか

- **デプロイ単位が違う**。backendは常時起動するAPIサーバー、取り込みは定刻に起動するバッチ(将来はGCPのCloud Functions + Cloud Scheduler)。
- **止まったときの影響が違う**。取り込みが失敗してもAPIは動き続ける必要がある。
- 実際、社内の別プロジェクトでも同じ形(`external-csv-transaction-functions`)を採っている。

分けたうえで、**テーブルの作成・変更(マイグレーション)はbackendだけが行う**。取り込み側は
マイグレーションを持たず、既にあるテーブルへ読み書きするだけにしている。スキーマの所有者を
1つに保つため。

| | backend(このリポジトリ) | clinic-inventory-csv-functions |
|---|---|---|
| テーブルの作成・変更 | **行う**(`cmd/api/main.go`のAutoMigrate) | 行わない |
| 画面・APIからの参照/更新 | 行う | 行わない |
| S3からのCSV取り込みと反映 | 行わない | **行う** |

---

## 2. S3キーの規約

```
orders/{卸コード}/{facilityId}/{orderId}.csv   ← 発注CSV(backendが書く)
catalogs/{卸コード}/{任意のファイル名}.csv      ← 商品マスタCSV(卸が置き、取り込み側が読む)
```

卸業者ごとにフォルダを分けているため、どの卸のCSVかは中身ではなく置かれた場所で決まる。フォルダ名は`distributors.code`(卸コード)で、卸業者に案内する際に人が読める識別子にするためUUIDは使わない。
詳細は[s3-storage.md 3章](s3-storage.md#3-s3キーの命名規則)。

---

## 3. 卸ごとに違うデータの持ち方をどう受けるか

**卸業者ごとに、送ってくる項目も粒度もバラバラ**である、というのがこのコンテキストの前提。
1社の形に合わせてモデルを作ると他社で破綻するため、「無い」ことを許容できる受け皿にしてある。

| 項目 | 卸による違い | 受け方 |
|---|---|---|
| 単価 | 商品ごと / 医院ごと / そもそも提供しない | 後述（3つとも表現できるようにする） |
| JANコード | 持っている卸と持っていない卸がある | 任意項目。無ければ空のまま取り込む（バーコード消費はJANのある商品でのみ効く） |
| ベンダー（メーカー）名 | 列が無い卸がある | 卸商品の必須項目のため、取り込み側の設定（`defaultVendorName`）で補う |
| ベンダー商品コード | 持っている卸と持っていない卸がある | 任意項目。無ければ空 |
| 廃番 | 廃番列を持つ卸と、単にCSVから消す卸がある | 列がある場合のみ反映。CSVから消えた商品は勝手に廃番にしない |

以降は、いちばん影響が大きい単価について詳しく書く。

### 単価の3パターン

| 卸のパターン | 保存先 |
|---|---|
| 商品ごとの単価のみ公開 | `distributor_products.unit_price` |
| 医院ごとに単価を決めている | `distributor_product_facility_prices`(医院別単価)。商品側の`unit_price`はNULLのまま |
| 単価を公表していない | どちらにも入れない(`unit_price`はNULL) |

パターンごとに実際の行がどう作られるか(CSVの例と対応表)は、取り込み側リポジトリの
`docs/design.md`「3-1. パターン別に、テーブルがどうなるか」を参照。

- `unit_price`は**NULL許容**。「0円」と「非公表」を区別するため、ドメイン側も`*int`で持つ
  ([distributor_product.go](../../backend/internal/domain/distributorcatalog/distributor_product.go))。
- 医院別単価は`DistributorProduct`の配下エンティティにせず独立させている。単価は卸とクリニックの契約に
  紐づく情報で、商品マスタとは別のタイミング・別のCSVで届くことがあるため
  ([facility_price.go](../../backend/internal/domain/distributorcatalog/facility_price.go))。
- クリニック商品(`clinic_products.unit_price`)の仕入単価は、**クリニックでの入力値 → 自院向けの医院別単価 →
  卸の標準単価**の順で決める。どれも無い場合は**0のまま登録する**(登録を止めない)。単価が分からない卸があり、
  その場合は後日、卸から届く受注結果の単価で更新する運用にしているため。
  実装は[register_clinic_product.go](../../backend/internal/application/productcatalog/register_clinic_product.go)。
- 画面では「非公表」という表記は使わず、単価が無い場合は0円として表示する。卸から見れば非公表でも、
  クリニックから見れば「まだ分からない金額」であり、後から確定する扱いのため。
- 医院ごとに単価を決めている卸の商品は、クリニック側の卸商品一覧・登録画面で**自院向けの単価**が出る
  (`GET /api/distributors/{id}/products?facilityId=...` が `facilityUnitPrice` を返す)。
- 卸ポータルの商品マスタでは、標準単価が無く医院別単価がある商品を「医院別（N院）」と表示し、
  選ぶと医院名付きの内訳が出る。¥0と出すと「0円で卸している」と読めてしまうため
  (`GET /api/portal/distributors/{id}/products` の `facilityPriceCount` と、
  `.../products/{productId}/facility-prices`)。一覧に全商品分の単価を載せると
  商品数×医院数になるため、一覧は件数だけ・内訳は選択時に取得する。

---

## 4. backend側が持つもの

| 役割 | 場所 |
|---|---|
| 反映先テーブルの定義 | [infrastructure/distributorcatalog/model.go](../../backend/internal/infrastructure/distributorcatalog/model.go) |
| 取り込み用テーブル(履歴・ステージング)の定義 | [infrastructure/distributorcsvingestion/model.go](../../backend/internal/infrastructure/distributorcsvingestion/model.go) — 定義のみ。読み書きは取り込み側 |
| テーブル作成 | [cmd/api/main.go](../../backend/cmd/api/main.go)のAutoMigrate |
| 卸商品・医院別単価の参照 | 各リポジトリ。医院別単価は参照のみ(登録・更新は取り込み側) |

**取り込み側リポジトリと同じ集約(`DistributorProduct` / `FacilityPrice`)が両方に存在する。**
業務ルール(必須項目・単価の扱い・廃盤)を変更する場合は、両方を揃える必要がある。
これはリポジトリを分けたことによるトレードオフとして受け入れている。

---

## 5. 取り込み結果をどう確認するか

取り込み1回分の記録は`distributor_catalog_ingestion_runs`、正規化されたCSVの行は
`distributor_catalog_staging_rows`に残る(定義は[database-schema.md](database-schema.md))。

- `status = applied` … 反映済み
- `status = needs_review` … 要確認。反映されていない。`message`に理由、ステージング行に原文とエラー内容が残る
- `status = staged` … 中間表現までは作られたが反映前

要確認になった取り込みは自動リトライしない(domain-rules.md「卸連携CSV基盤」)。原因を直したうえで、
CSVを置き直す(ETagが変わるので再取り込みされる)。

---

## 6. 未決

| 項目 | 現状 |
|---|---|
| IAM権限 | 付与済み(2026-08-14)。ポリシーは[s3-storage.md 3-1章](s3-storage.md) |
| 卸側の医院コード | 現状は「医院コード = クリニックID(UUID)」。実際の卸のコード体系との対応表が必要 |
| 卸側の書き込み手段 | 卸に`catalogs/{卸コード}/`へのPut権限を渡すか、こちらが受領してアップロードするか。当面は後者 |
| CSVから消えた商品の扱い | 廃盤にしない(放置)。全件洗い替えの卸に対して自動廃盤にするかは未決 |
| 廃盤になった商品の発注 | **取り込みによって顕在化した未解決事項**。取り込みが`discontinued`を自動更新するため「登録後に廃盤になった商品」が発生するが、発注(カート追加・確定)では廃盤を検証していないため発注できてしまう。止める場所と強さは[requirements.md 9章](../requirements.md#9-未確定事項次に詰める)に記載 |
