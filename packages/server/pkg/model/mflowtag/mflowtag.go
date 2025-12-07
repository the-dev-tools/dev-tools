//nolint:revive // exported
package mflowtag

import "the-dev-tools/server/pkg/idwrap"

type FlowTag struct {
	ID     idwrap.IDWrap
	FlowID idwrap.IDWrap
	TagID  idwrap.IDWrap
}
