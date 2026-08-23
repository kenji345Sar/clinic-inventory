# clinic-inventory（クリニック医薬品在庫管理システム）

医科・歯科・獣医クリニックの医薬品在庫管理。要件は [docs/requirements.md](docs/requirements.md)、ドメインモデルは [docs/architecture/domain-rules.md](docs/architecture/domain-rules.md) を参照。

**今どこまで進んでいて、何が課題で、次に何をやるかは [docs/status_log.md](docs/status_log.md) を参照。**

Goの構文に慣れていない場合は [docs/go/go-for-csharp.md](docs/go/go-for-csharp.md)（C#経験者のためのGo読み方ノート。petty-cashとの対応表）から読むとよい。リクエストがDBまで届く仕組みは [docs/go/request-to-sql-flow.md](docs/go/request-to-sql-flow.md) を参照。

## 必要なもの

- Go 1.22
- Node.js 22（nvm推奨。`frontend/.nvmrc` あり）
- PostgreSQL 16（Homebrew: `brew services start postgresql@16`）

## 初回セットアップ

```bash
# DB作成（psql系コマンドは export PATH="/usr/local/opt/postgresql@16/bin:$PATH" で通す）
createdb -h localhost -U "$(whoami)" clinic_inventory

# 環境変数ファイルを用意（実値は .env に記入。どちらも git 管理外）
cp backend/.env.example backend/.env    # AUTH0_DOMAIN / AUTH0_AUDIENCE を記入
cp frontend/.env.example frontend/.env  # Auth0 の各値・SESSION_SECRET を記入
```

- バックエンドとフロントの `AUTH_DISABLED` は揃える（本番Auth0検証なら両方 `false`、開発バイパスなら両方 `true`）
- テーブルはバックエンド起動時のAutoMigrateで自動作成

## 起動手順（毎回）

```bash
# ターミナル1: バックエンド（:8080）。.env を読み込んで起動する
cd backend && ./run.sh

# ターミナル2: クリニック向けフロントエンド（:5173）
cd frontend && nvm use && npm run dev

# ターミナル3: 卸ポータル（:5174）
cd distributor-portal && nvm use 22 && npm run dev
```

- `run.sh` は `backend/.env` を読み込み、`CGO_ENABLED=0 go run ./cmd/api` を実行する
- このマシンではGoのビルド・テストに `CGO_ENABLED=0` が必須（詳細は [backend/README.md](backend/README.md)）
- 卸ポータルは未認証（卸業者にはまだAuth0アカウントが無いため）。`AUTH_DISABLED` の設定に関わらず `/api/portal/...` はトークン無しでアクセスできる

## プロジェクト構成

```
backend/             … Go + gorm。DDDのモジュラーモノリス（コンテキストごとに domain/application/infrastructure/handler）
frontend/            … クリニック向けサイト。React + React Router v7+（Remix後継、フレームワークモード）+ Tailwind
distributor-portal/  … 卸業者向けポータル。frontendと同じ技術構成の別アプリ（未認証、:5174）
docs/                … 要件定義・ドメインモデル
```

## フロントエンドの技術選定メモ

「React + Remix」という当初方針に対し、Remix v2がメンテナンスモードで開発の主軸がReact Router v7（フレームワークモード）に移っているため、React Router v8を採用した。loader/actionの書き方はRemixとほぼ同一。GoバックエンドへのアクセスはRemixサーバー側（loader/action）からのみ行い、ブラウザから直接APIを呼ばない構成。

## 実装済み画面

### クリニック向けサイト（frontend）

| パス | 内容 |
|---|---|
| / | クリニック選択 |
| /facilities/:id/products | 商品マスタ。卸商品検索（卸選択→キーワード絞り込み）→クリニック商品登録、登録済み一覧 |
| /facilities/:id/orders | 発注。卸業者ごとに数量入力→発注確定、発注履歴の閲覧 |

### 卸ポータル（distributor-portal）

| パス | 内容 |
|---|---|
| / | 卸業者選択 |
| /distributors/:id/orders | 受注一覧。クリニックが確定した発注をクリニック名・卸商品コード付きで確認できる |
| /distributors/:id/products | 自社商品マスタ。一覧・登録 |
