package assertv2

/*
const (
	AssertTypeUndefined AssertType = iota
	AssertTypeEqual
	AssertTypeNotEqual
	AssertTypeContains
	AssertTypeNotContains
	AssertTypeGreater
	AssertTypeLess
	AssertTypeGreaterOrEqual
	AssertTypeLessOrEqual
)

const (
	AssertTypeEqualStr       = "=="
	AssertTypeNotEqualStr    = "!="
	AssertTypeContainsStr    = "in"
	AssertTypeNotContainsStr = "not in"
	TypeGreaterStr           = ">"
	TypeLessStr              = "<"
	TypeGreaterOrEqualStr    = ">="
	TypeLessOrEqualStr       = "<="
)
*/

func ConvertAssertTypeToExpr(assertType AssertType) string {
	switch assertType {
	case AssertTypeEqual:
		return AssertTypeEqualStr
	case AssertTypeNotEqual:
		return AssertTypeNotEqualStr
	case AssertTypeContains:
		return AssertTypeContainsStr
	case AssertTypeNotContains:
		return AssertTypeNotContainsStr
	case AssertTypeGreater:
		return AssertTypeGreaterStr
	case AssertTypeLess:
		return AssertTypeLessStr
	case AssertTypeGreaterOrEqual:
		return AssertTypeGreaterOrEqualStr
	case AssertTypeLessOrEqual:
		return AssertTypeLessOrEqualStr
	}
	return ""
}

func ConvertAssertTargetTypeToPath(assertTargetType AssertTargetType) string {
	switch assertTargetType {
	case AssertTargetTypeBody:
		return AssertPathBody
	case AssertTargetTypeHeader:
		return AssertPathHeader
	case AssertTargetTypeResponse:
		return AssertPathResponse
	case AssertTargetTypeQuery:
		return AssertPathQuery
	case AssertTargetTypeNode:
		return AssertPathNode
	}
	return ""
}
