package mexamplequery

import (
	"the-dev-tools/server/pkg/idwrap"
	deltav1 "the-dev-tools/spec/dist/buf/go/delta/v1"
)

// QuerySource represents the source kind of query for delta operations
type QuerySource int8

const (
	QuerySourceOrigin QuerySource = 1 // SOURCE_KIND_ORIGIN
	QuerySourceMixed  QuerySource = 2 // SOURCE_KIND_MIXED
	QuerySourceDelta  QuerySource = 3 // SOURCE_KIND_DELTA
)

// ToSourceKind converts QuerySource to deltav1.SourceKind
func (s QuerySource) ToSourceKind() deltav1.SourceKind {
	switch s {
	case QuerySourceOrigin:
		return deltav1.SourceKind_SOURCE_KIND_ORIGIN
	case QuerySourceMixed:
		return deltav1.SourceKind_SOURCE_KIND_MIXED
	case QuerySourceDelta:
		return deltav1.SourceKind_SOURCE_KIND_DELTA
	default:
		return deltav1.SourceKind_SOURCE_KIND_UNSPECIFIED
	}
}

// FromSourceKind converts deltav1.SourceKind to QuerySource
func FromSourceKind(kind deltav1.SourceKind) QuerySource {
	switch kind {
	case deltav1.SourceKind_SOURCE_KIND_ORIGIN:
		return QuerySourceOrigin
	case deltav1.SourceKind_SOURCE_KIND_MIXED:
		return QuerySourceMixed
	case deltav1.SourceKind_SOURCE_KIND_DELTA:
		return QuerySourceDelta
	default:
		return QuerySourceOrigin // default to origin
	}
}

type Query struct {
	QueryKey      string
	Description   string
	Value         string
	Enable        bool
	DeltaParentID *idwrap.IDWrap
	ID            idwrap.IDWrap
	ExampleID     idwrap.IDWrap
}

func (q Query) IsEnabled() bool {
	return q.Enable
}

// DetermineDeltaType determines the delta type based on the query's relationships
// This function replaces the need for storing Source explicitly
func (q *Query) DetermineDeltaType(exampleHasVersionParent bool) QuerySource {
	// If no DeltaParentID, this is not a delta query
	if q.DeltaParentID == nil {
		return QuerySourceOrigin
	}
	
	// If example has VersionParentID, this is a delta example
	if exampleHasVersionParent {
		// Query has DeltaParentID and example is delta -> DELTA query
		return QuerySourceDelta
	}
	
	// If example has no VersionParentID, it's an original example
	// Query has DeltaParentID but example is original -> MIXED query
	return QuerySourceMixed
}
