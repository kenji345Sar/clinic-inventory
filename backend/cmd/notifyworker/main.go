// notifyworker は発注確定のお知らせをSQSから取り出してSESで送る常駐プロセス。
// backend API(cmd/api)は確定時にSQSへ発注IDを積むだけで、メールはここが送る。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/google/uuid"

	procapp "clinic-inventory/internal/application/procurement"
	shareddomain "clinic-inventory/internal/domain/shared"
	"clinic-inventory/internal/infrastructure/database"
	distinfra "clinic-inventory/internal/infrastructure/distributorcatalog"
	"clinic-inventory/internal/infrastructure/mail"
	orginfra "clinic-inventory/internal/infrastructure/organization"
	procinfra "clinic-inventory/internal/infrastructure/procurement"
	"clinic-inventory/internal/infrastructure/queue"
)

func dsn() string {
	if v := os.Getenv("DATABASE_DSN"); v != "" {
		return v
	}
	return "host=localhost user=apple dbname=clinic_inventory port=5432 sslmode=disable"
}

func main() {
	queueURL := os.Getenv("NOTIFY_QUEUE_URL")
	if queueURL == "" {
		log.Fatal("NOTIFY_QUEUE_URL が未設定です")
	}

	// テーブルの作成・変更はcmd/apiが行うので、ここではAutoMigrateしない。
	db, err := database.Connect(dsn())
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}
	awsCfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("failed to load AWS config: %v", err)
	}

	q := queue.NewSQSQueue(sqs.NewFromConfig(awsCfg), queueURL)
	// 実際に送るのはcmd/apiで直接送っていたときと同じSES版の PurchaseOrderNotifier。
	sesNotifier := procinfra.NewPurchaseOrderNotifier(mail.NewSESSender(sesv2.NewFromConfig(awsCfg), os.Getenv("SES_FROM_ADDRESS")))
	sendNotification := procapp.NewSendOrderConfirmedNotificationUseCase(
		procinfra.NewPurchaseOrderRepository(db),
		distinfra.NewDistributorRepository(db),
		orginfra.NewFacilityRepository(db),
		sesNotifier,
	)

	// Ctrl+C(SIGINT)・SIGTERMで受信待ちを抜けて終了する。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Println("notifyworker started")
	for {
		messages, err := q.Receive(ctx)
		if ctx.Err() != nil {
			fmt.Println("notifyworker stopped")
			return
		}
		if err != nil {
			// 権限不足などで受信自体が失敗し続けるとき、間をあけずに問い合わせ続けないよう少し待つ。
			log.Printf("%v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		for _, m := range messages {
			handle(ctx, q, sendNotification, m)
		}
	}
}

// handle は1件を処理する。成功したときだけメッセージを消す。
// 失敗したら消さずに残し、可視性タイムアウトが過ぎるとSQSがもう一度渡してくる。
// キューに設定した回数だけ失敗すると、SQSがDLQ(失敗置き場のキュー)へ移す。
func handle(ctx context.Context, q *queue.SQSQueue, uc *procapp.SendOrderConfirmedNotificationUseCase, m queue.Message) {
	var msg procinfra.OrderConfirmedMessage
	if err := json.Unmarshal([]byte(m.Body), &msg); err != nil {
		log.Printf("メッセージを読めません(body=%s): %v", m.Body, err)
		return
	}
	id, err := uuid.Parse(msg.OrderID)
	if err != nil {
		log.Printf("発注IDの形式が正しくありません(orderId=%s): %v", msg.OrderID, err)
		return
	}
	if err := uc.Execute(ctx, shareddomain.ID(id)); err != nil {
		log.Printf("お知らせメールを送れませんでした。時間をおいて再試行されます(orderId=%s): %v", msg.OrderID, err)
		return
	}
	if err := q.Delete(ctx, m.ReceiptHandle); err != nil {
		// 送信は済んでいるので、消せないと次に受け取ったときにもう一度送ってしまう。
		log.Printf("送信済みのメッセージを消せませんでした。同じメールがもう一度送られる可能性があります(orderId=%s): %v", msg.OrderID, err)
		return
	}
	fmt.Printf("お知らせメールを送りました(orderId=%s)\n", msg.OrderID)
}
