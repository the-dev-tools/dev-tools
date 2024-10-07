package massert

import "dev-tools-backend/pkg/idwrap"

type AssertType int8

const (
	AssertTypeUndefined      AssertType = 0
	AssertTypeEqual          AssertType = 1
	AssertTypeNotEqual       AssertType = 2
	AssertTypeContains       AssertType = 3
	AssertTypeNotContains    AssertType = 4
	AssertTypeGreater        AssertType = 5
	AssertTypeLess           AssertType = 6
	AssertTypeGreaterOrEqual AssertType = 7
	AssertTypeLessOrEqual    AssertType = 8
)

type AssertTarget int8

const (
	AssertTargetUndefined AssertTarget = 0
	AssertTargetHeader    AssertTarget = 1
	AssertTargetBody      AssertTarget = 2
)

type Assert struct {
	ID        idwrap.IDWrap
	ExampleID idwrap.IDWrap
	Name      string

	Value  string
	Type   AssertType
	Target AssertTarget
}
