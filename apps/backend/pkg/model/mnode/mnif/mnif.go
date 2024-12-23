package mnif

import "the-dev-tools/backend/pkg/idwrap"

type ConditionType int8

const (
	ConditionTypeEqual          ConditionType = 1
	ConditionTypeNotEqual       ConditionType = 2
	ConditionTypeContains       ConditionType = 3
	ConditionTypeNotContains    ConditionType = 4
	ConditionTypeGreater        ConditionType = 5
	ConditionTypeAssertTypeLess ConditionType = 6
	ConditionTypeGreaterOrEqual ConditionType = 7
	ConditionTypeLessOrEqual    ConditionType = 8
	ConditionTypeExists         ConditionType = 9
	ConditionTypeNotExists      ConditionType = 10
)

type MNIF struct {
	FlowNodeID    idwrap.IDWrap
	Name          string
	ConditionType ConditionType
	Path          string
	Value         string
	// TODO: Condition type
}
