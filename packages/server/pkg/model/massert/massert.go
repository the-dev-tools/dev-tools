package massert

import "the-dev-tools/server/pkg/idwrap"

type AssertType int8

type AssertService interface {
	Get(key, value string) (interface{}, error)
}

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

func MapAssertType() map[AssertType]string {
	return map[AssertType]string{
		AssertTypeUndefined:      "undefined",
		AssertTypeEqual:          "==",
		AssertTypeNotEqual:       "!=",
		AssertTypeContains:       "contains",
		AssertTypeNotContains:    "not contains",
		AssertTypeGreater:        ">",
		AssertTypeLess:           "<",
		AssertTypeGreaterOrEqual: ">=",
		AssertTypeLessOrEqual:    "<=",
	}
}

type AssertDotPath string

// Dot notation paths keys
// Root
const (
	RDNResp   = "response"
	RDNBody   = "body"
	RDNHeader = "header"
)

type Assert struct {
	ID            idwrap.IDWrap
	ExampleID     idwrap.IDWrap
	DeltaParentID *idwrap.IDWrap
	Path          string
	Value         string
	Type          AssertType
	Enable        bool
}
