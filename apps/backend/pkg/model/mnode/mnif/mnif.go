package mnif

import "the-dev-tools/backend/pkg/idwrap"

type ConditionType int8

const (
	ConditionTypeEqual ConditionType = 1
)

type MNIF struct {
	FlowNodeID    idwrap.IDWrap
	Name          string
	ConditionType ConditionType
	Condition     string
	NextTrue      idwrap.IDWrap
	NextFalse     idwrap.IDWrap
	// TODO: Condition type
}
