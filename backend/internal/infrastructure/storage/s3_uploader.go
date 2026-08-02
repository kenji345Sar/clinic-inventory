package storage

import (
	"bytes"
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Uploader はS3への単純なオブジェクトアップロードだけを担う薄いラッパー。
// 卸連携CSV基盤(docs/architecture/domain-rules.md)のように、S3をトリガーとする
// 複数のコンテキストから共通で使うため、特定コンテキストに属さないinfrastructure直下に置く。
type S3Uploader struct {
	client *s3.Client
	bucket string
}

func NewS3Uploader(client *s3.Client, bucket string) *S3Uploader {
	return &S3Uploader{client: client, bucket: bucket}
}

// Put はkeyの位置にbodyをアップロードする。
func (u *S3Uploader) Put(ctx context.Context, key string, body []byte, contentType string) error {
	_, err := u.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &u.bucket,
		Key:         &key,
		Body:        bytes.NewReader(body),
		ContentType: &contentType,
	})
	if err != nil {
		return fmt.Errorf("S3へのアップロードに失敗しました(key=%s): %w", key, err)
	}
	return nil
}
