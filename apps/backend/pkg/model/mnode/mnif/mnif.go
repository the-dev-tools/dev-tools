package mnif

import "the-dev-tools/backend/pkg/idwrap"

type ConditionType int8

/*
  CONDITION_TYPE_UNSPECIFIED: 0,
  CONDITION_TYPE_EQUAL: 1,
  CONDITION_TYPE_NOT_EQUAL: 2,
  CONDITION_TYPE_CONTAINS: 3,
  CONDITION_TYPE_NOT_CONTAINS: 4,
  CONDITION_TYPE_GREATER: 5,
  CONDITION_TYPE_ASSERT_TYPE_LESS: 6,
  CONDITION_TYPE_GREATER_OR_EQUAL: 7,
  CONDITION_TYPE_LESS_OR_EQUAL: 8,
  CONDITION_TYPE_EXISTS: 9,
  CONDITION_TYPE_NOT_EXISTS: 10,
  CONDITION_TYPE_CUSTOM: 11,
*/

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
	ConditionTypeCustom         ConditionType = 11
)

type MNIF struct {
	FlowNodeID    idwrap.IDWrap
	Name          string
	ConditionType ConditionType
	Condition     string
	// TODO: Condition type
}
