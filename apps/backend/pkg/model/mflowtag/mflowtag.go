package mflowtag

import "the-dev-tools/backend/pkg/idwrap"

type FlowTag struct {
	ID     idwrap.IDWrap
	FlowID idwrap.IDWrap
	TagID  idwrap.IDWrap
}
