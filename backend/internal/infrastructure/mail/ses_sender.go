package mail

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

// SESSender はAmazon SESでテキストメールを1通送るだけの薄いラッパー。
// storage.S3Uploader と同じく、特定コンテキストに属さないinfrastructure直下に置く。
type SESSender struct {
	client *sesv2.Client
	from   string
}

// NewSESSender は送信元アドレスfromで送るSESSenderを作る。
// fromはSESで検証済みのアドレス(またはドメイン)でないと送信時に拒否される。
func NewSESSender(client *sesv2.Client, from string) *SESSender {
	return &SESSender{client: client, from: from}
}

// Send はtoへ件名subject・本文bodyのテキストメールを送る。
func (s *SESSender) Send(ctx context.Context, to, subject, body string) error {
	if s.from == "" {
		return errors.New("送信元メールアドレス(SES_FROM_ADDRESS)が未設定です")
	}
	charset := "UTF-8"
	_, err := s.client.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: &s.from,
		Destination:      &types.Destination{ToAddresses: []string{to}},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{Data: &subject, Charset: &charset},
				Body:    &types.Body{Text: &types.Content{Data: &body, Charset: &charset}},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("メール送信に失敗しました(to=%s): %w", to, err)
	}
	return nil
}
