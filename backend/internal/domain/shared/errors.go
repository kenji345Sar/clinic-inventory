package shared

import "errors"

// HTTP層でステータスコードにマッピングするための共有センチネルエラー。
// リポジトリ実装は「見つからない」を ErrNotFound に、
// ユースケースは一意性違反等の競合を ErrConflict にラップして返す。
// ErrForbidden は認可(ロール×所属クリニック)で権限が無いと判定された場合に使う。
var (
	ErrNotFound  = errors.New("not found")
	ErrConflict  = errors.New("conflict")
	ErrForbidden = errors.New("forbidden")
)
