package mexamplequery

import (
	"the-dev-tools/server/pkg/idwrap"
	// deltav1 "the-dev-tools/spec/dist/buf/go/api/delta/v1" // Temporarily commented out
)

// QuerySource represents the source kind of query for delta operations
type QuerySource int8

const (
	QuerySourceOrigin QuerySource = 1 // SOURCE_KIND_ORIGIN
	QuerySourceMixed  QuerySource = 2 // SOURCE_KIND_MIXED
	QuerySourceDelta  QuerySource = 3 // SOURCE_KIND_DELTA
)

// ToSourceKind converts QuerySource to deltav1.SourceKind
// TODO: Re-enable when delta spec is available
/*
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
*/

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
// This function dynamically determines the source type without storing it explicitly
//
// Logic Matrix:
// | Has DeltaParentID | Example Has VersionParent | Result | Meaning |
// |-------------------|---------------------------|--------|---------|
// | No                | No                        | ORIGIN | Standalone item in origin example |
// | No                | Yes                       | DELTA  | New item created in delta example |
// | Yes               | No                        | MIXED  | Item referencing another in origin example |
// | Yes               | Yes                       | MIXED  | Item in delta example modified from parent |
//
// The key principle is:
// - **Has parent + unchanged** = ORIGIN (determined by value comparison at runtime)
// - **Has parent + changed** = MIXED (items that differ from their parent)
// - **No parent** = DELTA (standalone items created in delta example)
func (q *Query) DetermineDeltaType(exampleHasVersionParent bool) QuerySource {
	// If no DeltaParentID, determine based on example type
	if q.DeltaParentID == nil {
		if exampleHasVersionParent {
			// No parent in a delta example = standalone DELTA item
			return QuerySourceDelta
		}
		// No parent in origin example = ORIGIN item
		return QuerySourceOrigin
	}

	// Has DeltaParentID - determine based on example type
	if exampleHasVersionParent {
		// In delta example with parent reference = MIXED (modified from parent)
		return QuerySourceMixed
	}

	// In origin example with parent reference = MIXED
	return QuerySourceMixed
}
