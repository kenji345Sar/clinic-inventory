package distributorcatalog

import (
	"errors"

	shareddomain "clinic-inventory/internal/domain/shared"
)

// Distributor（卸業者）。発注先マスタ。
type Distributor struct {
	id   shareddomain.ID
	name string
}

func NewDistributor(name string) (*Distributor, error) {
	if name == "" {
		return nil, errors.New("卸業者名は必須です")
	}
	return &Distributor{id: shareddomain.NewID(), name: name}, nil
}

func (d *Distributor) ID() shareddomain.ID { return d.id }
func (d *Distributor) Name() string        { return d.name }

// ReconstructDistributor は永続化データからDistributorを復元する。バリデーションは行わない。
func ReconstructDistributor(id shareddomain.ID, name string) *Distributor {
	return &Distributor{id: id, name: name}
}
