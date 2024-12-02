package mnif

import "the-dev-tools/backend/pkg/idwrap"

type MNIF struct {
	ID        idwrap.IDWrap
	Name      string
	NextTrue  idwrap.IDWrap
	NextFalse idwrap.IDWrap
	Condition string
	// TODO: Condition type
}
