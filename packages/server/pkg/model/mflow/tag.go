//nolint:revive // exported
package mflow

import "github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"

type FlowTag struct {
	ID     idwrap.IDWrap
	FlowID idwrap.IDWrap
	TagID  idwrap.IDWrap
}
