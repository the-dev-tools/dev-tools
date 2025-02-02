package mnif

import (
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mcondition"
)

type MNIF struct {
	FlowNodeID idwrap.IDWrap
	Name       string
	Condition  mcondition.Condition
	// TODO: Condition type
}
