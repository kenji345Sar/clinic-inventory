# clinic-inventory（クリニック医薬品在庫管理システム）

医科・歯科・獣医クリニックの医薬品在庫管理。要件は [docs/requirements.md](docs/requirements.md)、ドメインモデルは [docs/architecture/domain-rules.md](docs/architecture/domain-rules.md) を参照。

Goの構文に慣れていない場合は [docs/go/go-for-csharp.md](docs/go/go-for-csharp.md)（C#経験者のためのGo読み方ノート。petty-cashとの対応表）から読むとよい。リクエストがDBまで届く仕組みは [docs/go/request-to-sql-flow.md](docs/go/request-to-sql-flow.md) を参照。

## 必要なもの

- Go 1.22
- Node.js 22（nvm推奨。`frontend/.nvmrc` あり）
- PostgreSQL 16（Homebrew: `brew services start postgresql@16`）

## 起動手順（毎回）

```bash
# ターミナル1: バックエンド（:8080）
cd backend && CGO_ENABLED=0 go run ./cmd/api

# ターミナル2: フロントエンド（:5173）
cd frontend && nvm use && npm run dev
```

- 初回のみ `createdb -h localhost -U "$(whoami)" clinic_inventory`（psql系コマンドは `export PATH="/usr/local/opt/postgresql@16/bin:$PATH"` で通す）
- テーブルはバックエンド起動時のAutoMigrateで自動作成
- このマシンではGoのビルド・テストに `CGO_ENABLED=0` が必須（詳細は [backend/README.md](backend/README.md)）

## プロジェクト構成

```
backend/   … Go + gorm。DDDのモジュラーモノリス（コンテキストごとに domain/application/infrastructure/handler）
frontend/  … React + React Router v7+（Remix後継、フレームワークモード）+ Tailwind
docs/      … 要件定義・ドメインモデル
```

## フロントエンドの技術選定メモ

「React + Remix」という当初方針に対し、Remix v2がメンテナンスモードで開発の主軸がReact Router v7（フレームワークモード）に移っているため、React Router v8を採用した。loader/actionの書き方はRemixとほぼ同一。GoバックエンドへのアクセスはRemixサーバー側（loader/action）からのみ行い、ブラウザから直接APIを呼ばない構成。

## 実装済み画面

| パス | 内容 |
|---|---|
| / | クリニック選択 |
| /facilities/:id/products | 商品マスタ。卸商品検索（卸選択→キーワード絞り込み）→クリニック商品登録、登録済み一覧 |
