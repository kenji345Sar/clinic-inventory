#!/usr/bin/env bash
# 通知ワーカー(Go cmd/notifyworker)をローカル起動する。
# run.sh と同じく、同じディレクトリの .env を読み込んで環境変数にしてから go run する。
# 使い方: cd backend && ./run-notifyworker.sh
set -euo pipefail

cd "$(dirname "$0")"

if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  . ./.env
  set +a
else
  echo "警告: .env がありません。.env.example をコピーして作成してください。" >&2
fi

# このマシンではGoのビルドに CGO_ENABLED=0 が必須(詳細は README)。
exec env CGO_ENABLED=0 go run ./cmd/notifyworker
