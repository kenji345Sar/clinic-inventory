package shared

import "errors"

// HTTP層でステータスコードにマッピングするための共有センチネルエラー。
// リポジトリ実装は「見つからない」を ErrNotFound に、
// ユースケースは一意性違反等の競合を ErrConflict にラップして返す。
var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)
