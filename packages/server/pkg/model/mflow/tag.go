//nolint:revive // exported
package mflow

import "the-dev-tools/server/pkg/idwrap"

type FlowTag struct {
	ID     idwrap.IDWrap
	FlowID idwrap.IDWrap
	TagID  idwrap.IDWrap
}
