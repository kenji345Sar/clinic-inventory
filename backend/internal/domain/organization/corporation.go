package organization

import (
	"errors"

	shareddomain "clinic-inventory/internal/domain/shared"
)

// Corporation（法人）。配下Facilityの一覧は持たず、Facility側がCorporationIDを参照する。
// 単体クリニックでも「一人法人」としてCorporationを1つ作ることでモデルをシンプルに保つ。
// (docs/architecture/domain-rules.md「組織（Organization）コンテキスト」参照)
type Corporation struct {
	id   shareddomain.ID
	name string
}

func NewCorporation(name string) (*Corporation, error) {
	if name == "" {
		return nil, errors.New("法人名は必須です")
	}
	return &Corporation{id: shareddomain.NewID(), name: name}, nil
}

func (c *Corporation) ID() shareddomain.ID { return c.id }
func (c *Corporation) Name() string        { return c.name }

// ReconstructCorporation は永続化データからCorporationを復元する。バリデーションは行わない
// （すでに永続化された時点で妥当な状態であることが前提のため）。
func ReconstructCorporation(id shareddomain.ID, name string) *Corporation {
	return &Corporation{id: id, name: name}
}
