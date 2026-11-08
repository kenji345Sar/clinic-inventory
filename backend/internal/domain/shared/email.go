package shared

import (
	"errors"
	"net/mail"
)

// ValidateEmail はメールアドレスの書式を確かめる。
// 「山田 <a@example.com>」のような表示名付きは通さない。DBにそのまま保存して送信先に使うため、
// アドレス部分だけを受け付けないと、送るときに表示名ごと宛先に入ってしまう。
func ValidateEmail(s string) error {
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s {
		return errors.New("メールアドレスの形式が正しくありません")
	}
	return nil
}
