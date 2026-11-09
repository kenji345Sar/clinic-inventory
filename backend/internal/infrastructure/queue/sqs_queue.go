package queue

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// SQSQueue はSQSの1つのキューへの送信・受信・削除だけを担う薄いラッパー。
// storage.S3Uploader・mail.SESSender と同じく、特定コンテキストに属さないinfrastructure直下に置く。
type SQSQueue struct {
	client   *sqs.Client
	queueURL string
}

func NewSQSQueue(client *sqs.Client, queueURL string) *SQSQueue {
	return &SQSQueue{client: client, queueURL: queueURL}
}

// Message は受信した1件。ReceiptHandle は削除するときに渡す受け取り証。
type Message struct {
	Body          string
	ReceiptHandle string
}

// Send はbodyを1件キューに積む。
func (q *SQSQueue) Send(ctx context.Context, body string) error {
	_, err := q.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    &q.queueURL,
		MessageBody: &body,
	})
	if err != nil {
		return fmt.Errorf("SQSへの送信に失敗しました: %w", err)
	}
	return nil
}

// Receive は最大10件を受け取る。キューが空なら最大20秒待ってから空で返す（ロングポーリング）。
// 待たずにすぐ返す設定だと、空のキューを1秒に何回も問い合わせることになり、リクエスト数が無駄に増える。
//
// 受け取ったメッセージはキューから消えず、一定時間（可視性タイムアウト。キュー側の設定）他の受信者から見えなくなるだけ。
// その間に Delete しなければ再び受け取れる状態に戻る。これが「失敗したら自動でやり直す」の仕組み。
func (q *SQSQueue) Receive(ctx context.Context) ([]Message, error) {
	out, err := q.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            &q.queueURL,
		MaxNumberOfMessages: 10,
		WaitTimeSeconds:     20,
	})
	if err != nil {
		return nil, fmt.Errorf("SQSからの受信に失敗しました: %w", err)
	}
	messages := make([]Message, 0, len(out.Messages))
	for _, m := range out.Messages {
		messages = append(messages, Message{Body: *m.Body, ReceiptHandle: *m.ReceiptHandle})
	}
	return messages, nil
}

// Delete は処理が終わったメッセージをキューから消す。
func (q *SQSQueue) Delete(ctx context.Context, receiptHandle string) error {
	_, err := q.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      &q.queueURL,
		ReceiptHandle: &receiptHandle,
	})
	if err != nil {
		return fmt.Errorf("SQSのメッセージ削除に失敗しました: %w", err)
	}
	return nil
}
