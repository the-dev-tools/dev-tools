package massert

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
	deltav1 "the-dev-tools/spec/dist/buf/go/delta/v1"
)

type AssertType int8

type AssertService interface {
	Get(key, value string) (interface{}, error)
}

// AssertSource represents the source kind of assert for delta operations
type AssertSource int8

const (
	AssertSourceOrigin AssertSource = 1 // SOURCE_KIND_ORIGIN
	AssertSourceMixed  AssertSource = 2 // SOURCE_KIND_MIXED
	AssertSourceDelta  AssertSource = 3 // SOURCE_KIND_DELTA
)

// ToSourceKind converts AssertSource to deltav1.SourceKind
func (s AssertSource) ToSourceKind() deltav1.SourceKind {
	switch s {
	case AssertSourceOrigin:
		return deltav1.SourceKind_SOURCE_KIND_ORIGIN
	case AssertSourceMixed:
		return deltav1.SourceKind_SOURCE_KIND_MIXED
	case AssertSourceDelta:
		return deltav1.SourceKind_SOURCE_KIND_DELTA
	default:
		return deltav1.SourceKind_SOURCE_KIND_UNSPECIFIED
	}
}

// FromSourceKind converts deltav1.SourceKind to AssertSource
func FromSourceKind(kind deltav1.SourceKind) AssertSource {
	switch kind {
	case deltav1.SourceKind_SOURCE_KIND_ORIGIN:
		return AssertSourceOrigin
	case deltav1.SourceKind_SOURCE_KIND_MIXED:
		return AssertSourceMixed
	case deltav1.SourceKind_SOURCE_KIND_DELTA:
		return AssertSourceDelta
	default:
		return AssertSourceOrigin // default to origin
	}
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
	Condition     mcondition.Condition

	Enable bool
	Prev   *idwrap.IDWrap
	Next   *idwrap.IDWrap
}

// DetermineDeltaType determines the delta type based on the assert's relationships
// This function replaces the need for storing Source explicitly
func (a *Assert) DetermineDeltaType(exampleHasVersionParent bool) AssertSource {
	// If no DeltaParentID, this is not a delta assert
	if a.DeltaParentID == nil {
		return AssertSourceOrigin
	}
	
	// If example has VersionParentID, this is a delta example
	if exampleHasVersionParent {
		// Assert has DeltaParentID and example is delta -> DELTA assert
		return AssertSourceDelta
	}
	
	// If example has no VersionParentID, it's an original example
	// Assert has DeltaParentID but example is original -> MIXED assert
	return AssertSourceMixed
}
