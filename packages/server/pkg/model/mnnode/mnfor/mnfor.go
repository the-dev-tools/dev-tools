package mnfor

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
)

type ErrorHandling int8

const (
	ErrorHandling_ERROR_HANDLING_UNSPECIFIED ErrorHandling = 0
	ErrorHandling_ERROR_HANDLING_IGNORE      ErrorHandling = 1
	ErrorHandling_ERROR_HANDLING_BREAK       ErrorHandling = 2
)

type MNFor struct {
	FlowNodeID    idwrap.IDWrap
	IterCount     int64
	Condition     mcondition.Condition
	ErrorHandling ErrorHandling
}
