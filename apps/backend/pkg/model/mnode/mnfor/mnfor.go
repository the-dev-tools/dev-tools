package mnfor

import "the-dev-tools/backend/pkg/idwrap"

type MNFor struct {
	FlowNodeID      idwrap.IDWrap
	Name            string
	IterCount       int64
	LoopStartNodeID idwrap.IDWrap
	Next            idwrap.IDWrap
}
