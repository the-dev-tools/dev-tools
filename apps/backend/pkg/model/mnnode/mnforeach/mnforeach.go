package mnforeach

import (
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mcondition"
	"the-dev-tools/backend/pkg/model/mnnode/mnfor"
)

type MNForEach struct {
	FlowNodeID    idwrap.IDWrap
	IterPath      string
	Condition     mcondition.Condition
	ErrorHandling mnfor.ErrorHandling
}
