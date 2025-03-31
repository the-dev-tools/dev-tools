package mnforeach

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
)

type MNForEach struct {
	FlowNodeID    idwrap.IDWrap
	IterPath      string
	Condition     mcondition.Condition
	ErrorHandling mnfor.ErrorHandling
}
