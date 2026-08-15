package distributorcatalog

import (
	"errors"
	"regexp"

	shareddomain "clinic-inventory/internal/domain/shared"
)

// distributorCodePattern は卸コードに使える文字。S3のキー（フォルダ名）にそのまま使うため、
// 記号や日本語を許すとURLエンコードや大文字小文字の扱いで事故りやすい。
// 小文字英数字とハイフンだけに絞る。
var distributorCodePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Distributor（卸業者）。発注先マスタ。
//
// code（卸コード）はS3上のフォルダ名として使う識別子。
// 発注CSVを置く `orders/{卸コード}/...`、卸から商品マスタCSVを受け取る `catalogs/{卸コード}/...` の
// どちらもこのコードでフォルダを分ける（docs/architecture/s3-storage.md）。
// UUIDではなくコードを使うのは、卸業者自身にフォルダを案内する場面で人が読めるようにするため。
type Distributor struct {
	id   shareddomain.ID
	code string
	name string
}

func NewDistributor(code, name string) (*Distributor, error) {
	if code == "" {
		return nil, errors.New("卸コードは必須です")
	}
	if !distributorCodePattern.MatchString(code) {
		return nil, errors.New("卸コードは小文字英数字とハイフンで指定してください（例: oroshi-b）")
	}
	if name == "" {
		return nil, errors.New("卸業者名は必須です")
	}
	return &Distributor{id: shareddomain.NewID(), code: code, name: name}, nil
}

func (d *Distributor) ID() shareddomain.ID { return d.id }
func (d *Distributor) Code() string        { return d.code }
func (d *Distributor) Name() string        { return d.name }

// ReconstructDistributor は永続化データからDistributorを復元する。バリデーションは行わない。
func ReconstructDistributor(id shareddomain.ID, code, name string) *Distributor {
	return &Distributor{id: id, code: code, name: name}
}
