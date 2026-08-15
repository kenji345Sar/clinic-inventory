# ドメインモデル — 集約とルール（v0）

[docs/requirements.md](../requirements.md)の6章で洗い出した境界づけられたコンテキストごとに、集約ルート・配下エンティティ・業務ルール（不変条件）を書き出す。
petty-cashと同じく、業務ルールは極力ドメイン層（`domain/`）に集中させ、UseCase・Infrastructureはそれを呼ぶだけにする方針。

---

## 組織（Organization）コンテキスト

**集約ルート: Facility（クリニック）**

| ルール | 意図 |
|---|---|
| クリニック名は必須 | 最低限のバリデーション |
| 種別（医科/歯科/獣医）を持つ | 将来種別ごとの扱い分けに備える |
| 必ずいずれかのCorporation（法人）に属する | 単体クリニックでも「一人法人」的にCorporationを1つ作り、モデルをシンプルに保つ |
| Groupへの所属は任意 | [requirements.md 3章](../requirements.md#3-組織構造)の未確定事項。MVPでは使わないが、後でCorporationとFacilityの間に挟めるようフィールドだけ用意する |

**参照集約: Corporation（法人）**
- 法人名を持つ。配下Facility一覧は持たず、Facility側がCorporationIDを参照する疎結合構成（petty-cashのSafeと同様、参照される側に一覧を持たせない）
- 法人管理者は自CorporationIDに属する全Facilityを操作可能（[requirements.md 8章](../requirements.md#8-権限モデル仮案)の権限モデルと対応）

---

## 卸連携（DistributorCatalog）コンテキスト

**集約ルート: Distributor（卸業者）**

| ルール | 意図 |
|---|---|
| 卸業者名は必須 | 最低限のバリデーション |

**集約ルート: DistributorProduct（卸商品）** — Distributorの配下エンティティにはしない

卸ごとに約5,000点あり、Distributor集約がDistributorProductを丸ごと抱えると1件登録するだけで5,000件をロードすることになり実運用で破綻する。そのためDistributorProductは独立した集約とし、DistributorIDは参照（値）として持つだけにする。

| ルール | 意図 |
|---|---|
| 卸商品コードは同一卸業者内で一意 | 卸側の商品コード体系が卸ごとに閉じているため（会社ごとに約5,000点）。一意性はDistributorProduct集約内では判定せず、リポジトリ層で登録前に存在チェックし、DB側も`(distributor_id, distributor_product_code)`のユニーク制約で担保する |
| 卸商品はベンダー（メーカー）名を持つ | 要件（卸商品マスタの必須項目） |
| 卸商品を廃盤にする場合は物理削除せず廃盤フラグを立てる | クリニック商品からの参照（紐付け）が残っている可能性があるため、参照整合性を壊さない |
| 卸からの商品マスタ・価格表CSVの取り込みは`(distributor_id, distributor_product_code)`でupsertする | 卸商品コードは卸内で一意という既存ルールと整合させ、在庫有無・単価を更新する |
| 卸商品の標準単価は「非公表」を取りうる（NULL可） | 単価を公表せず商品マスタだけ送ってくる卸があるため。0円と非公表を区別する（[distributor-catalog-import.md 3章](distributor-catalog-import.md#3-卸ごとに違うデータの持ち方をどう受けるか)） |
| 医院ごとに単価を決めている卸の単価は、卸商品ではなく医院別単価（FacilityPrice）が持つ | 単価は卸とクリニックの契約に紐づく情報で、商品マスタとは別のタイミング・別のCSVで届くため、独立して更新できるようにする |

---

## 商品マスタ（ProductCatalog）コンテキスト

**集約ルート: ClinicProduct（クリニック商品）**

**なぜ卸商品マスタ（DistributorProduct）と分かれているか**

「卸は卸の商品コードがあり、クリニックではクリニックで扱う商品コードを割り当てる」という当初要件の通り、持ち主が違うデータのため分けている。卸商品マスタは卸のカタログ（卸1社約5,000点、内容を決めるのは卸）、クリニック商品マスタはそこから選んで登録した「うちで使う商品」（クリニックの持ち物）。分けない場合、次の3点で破綻する。

1. クリニック固有の設定（発注点など）の置き場所がない — 同じ商品でもクリニックごとに発注点は違う
2. 複数クリニックで卸カタログを共有できない — 本システムは複数クリニック前提で、1つの卸カタログを契約中の全クリニックが参照する
3. 廃盤と在庫のズレに耐えられない — 卸が廃盤にしてもクリニックには在庫が残る。だから卸商品は物理削除せず廃盤フラグとし、クリニック商品からの参照を生かす

| ルール | 意図 |
|---|---|
| クリニック独自の商品コードは同一クリニック内で一意 | クリニックごとに商品コード体系を持つという要件 |
| 卸商品（DistributorProduct）への紐付けが必須 | 「卸商品を元にクリニック商品を登録する」という業務フロー（[requirements.md 5章](../requirements.md#5-業務フロー)手順3）を担保。卸商品が未登録だとクリニック商品は作れない |
| JANコードを保持できる（任意） | バーコード消費のため。JANが無い商品も許容し、名称等の検索キーワードを別途保持する（名寄せ方式は[requirements.md 9章](../requirements.md#9-未確定事項次に詰める)で未確定） |
| 発注点（ReorderPoint）は0以上 | 発注アラート/自動発注の判定に使う |

---

## 発注（Procurement）コンテキスト

**集約ルート: PurchaseOrder（発注）**

配下エンティティ: **PurchaseOrderLine（発注明細）**

| ルール | 意図 |
|---|---|
| 発注明細は1件以上必須 | 空の発注を防ぐ |
| 発注はクリニック×卸業者の単位で作成する | [requirements.md 11章](../requirements.md#11-決定事項decision-log)の契約単位の決定と対応 |
| 発注確定後は明細を変更できない | 確定後の改ざん防止。取消は逆仕訳（赤伝と同じ考え方）で表現する |
| 卸からの受注確定CSVを取り込むと、PurchaseOrderLineに確定数量・欠品数量が記録される | 卸側の在庫状況（欠品）を発注側に反映するため。発注数量そのものは書き換えない別フィールドとし、「明細を変更できない」不変条件とは矛盾しない |
| 確定時に発注書（PDF）を生成する | [requirements.md 10章](../requirements.md#10-発注書フォーマット決定)のフォーマットに従う |
| EDI対応卸への確定発注は、卸別アダプタ経由でデータを送信する | 卸ごとにフォーマットが異なるため、共通の発注データモデル→卸別アダプタで変換する方針（[requirements.md 5章](../requirements.md#5-業務フロー)手順5-b） |
| 在庫が発注点を下回っている状態を判定できる（`IsBelowReorderPoint()`） | 発注点アラート・自動発注のトリガーとして在庫コンテキストから参照する |

---

## 在庫（Inventory）コンテキスト

**集約ルート: Stock（在庫）** — クリニック商品ごとに1つ

配下エンティティ: **Lot（ロット）**

| ルール | 意図 |
|---|---|
| 在庫数量はLotの残数量の合計としてのみ算出可能、直接編集不可 | 入荷・消費以外での改ざんを防ぐ |
| 入荷時はロット番号・使用期限が必須 | ロット/期限管理必須という要件（[requirements.md 8章](../requirements.md#8-権限モデル仮案)より前の決定事項） |
| 消費時は使用期限が近いロットから優先的に減算する（FEFO: First-Expired-First-Out） | 期限切れロスを防ぐ、医薬品で標準的な運用 |
| 消費数量が全ロット残数の合計を超える消費は不可 | マイナス在庫を防ぐ |
| `IsBelowReorderPoint()` で発注点割れを判定できる | アラート/自動発注のトリガーに使う（発注コンテキストから参照） |

---

## 卸連携CSV基盤（DistributorCsvIngestion）コンテキスト

このコンテキストは独自の集約を持たず、S3を介したクリニック・卸業者間のCSVファイル連携をトリガーに、DistributorCatalogコンテキストとProcurementコンテキストの集約を更新するプロセスを担う。**卸→クリニック方向の取り込み処理の実装は別リポジトリ`clinic-inventory-csv-functions`にあり**、このリポジトリはテーブル定義と反映先の集約を持つ([distributor-catalog-import.md](distributor-catalog-import.md))。S3バケット・IAMの実際の設定・運用手順は[s3-storage.md](s3-storage.md)を参照。

CSVは3種類あり、アップロード主体と反映先が異なる。

| CSV種別 | アップロード主体 | 反映先 | 内容 |
|---|---|---|---|
| 発注CSV | クリニック | Procurement（PurchaseOrder新規作成） | 発注時の品目・数量 |
| 商品マスタ・価格表CSV | 卸業者 | DistributorCatalog（DistributorProduct更新） | 在庫有無・単価の更新 |
| 受注確定CSV | 卸業者 | Procurement（PurchaseOrderLine更新） | 欠品情報を含む確定数量 |

| ルール | 意図 |
|---|---|
| CSVアップロードはS3イベント駆動で自動取り込みする（人手を介さない） | 運用負荷を減らし、取り込み漏れを防ぐ |
| 卸ごとにCSVフォーマットが異なるため、卸別アダプタで共通データモデルに変換してから集約を更新する | 発注コンテキストのEDIアダプタ方針（[発注コンテキスト](#発注procurementコンテキスト)の既存ルール）と同じ考え方をCSV取り込みにも適用する。初期実装では卸1社分のフォーマットを代表として定義し、他社は順次アダプタを追加する |
| CSVの形式不正・コード不一致など取り込みに失敗した場合は「要確認」状態として記録し、自動リトライはしない | 原因不明のまま自動反映し続けて業務データを壊すことを防ぐ。生データはS3に残し、対応は人手に委ねる（詳細な復旧フローは未確定） |

---

## 卸ポータル（DistributorPortal）コンテキスト

このコンテキストは独自の集約を持たず、発注・在庫コンテキストのデータを**契約（クリニック×卸業者）に基づいてフィルタした読み取り専用ビュー**として提供する。

| ルール | 意図 |
|---|---|
| 卸業者担当者は、自社と契約しているクリニックのデータのみ閲覧できる | [requirements.md 11章](../requirements.md#11-決定事項decision-log)の契約単位（クリニック×卸業者）の決定と対応 |
| 卸業者担当者は発注・在庫データを閲覧のみでき、更新はできない | [requirements.md 8章](../requirements.md#8-権限モデル仮案)の権限モデル |

---

## コンテキスト間の依存関係（概観）

```
Organization ← (Facility参照) ← Procurement / Inventory / ProductCatalog
DistributorCatalog ← (DistributorProduct参照) ← ProductCatalog
ProductCatalog ← (ClinicProduct参照) ← Procurement / Inventory
Procurement ←→ Inventory （発注点割れの判定を在庫側が提供し、発注側がトリガーとして使う）
DistributorPortal → Procurement / Inventory の読み取り専用ビュー
DistributorCsvIngestion → DistributorCatalog（商品マスタCSV反映） / Procurement（発注CSV・受注確定CSV反映）
```
