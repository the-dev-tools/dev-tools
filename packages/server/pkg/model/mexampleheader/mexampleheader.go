package mexampleheader

import (
	"the-dev-tools/server/pkg/idwrap"
)

// Temporary delta types until delta.tsp generation is fixed
type SourceKind int32

const (
	SourceKind_SOURCE_KIND_UNSPECIFIED SourceKind = 0
	SourceKind_SOURCE_KIND_ORIGIN      SourceKind = 1
	SourceKind_SOURCE_KIND_MIXED       SourceKind = 2
	SourceKind_SOURCE_KIND_DELTA       SourceKind = 3
)

// HeaderSource represents the source kind of header for delta operations
type HeaderSource int8

const (
	HeaderSourceOrigin HeaderSource = 1 // SOURCE_KIND_ORIGIN
	HeaderSourceMixed  HeaderSource = 2 // SOURCE_KIND_MIXED
	HeaderSourceDelta  HeaderSource = 3 // SOURCE_KIND_DELTA
)

// ToSourceKind converts HeaderSource to SourceKind
func (s HeaderSource) ToSourceKind() SourceKind {
	switch s {
	case HeaderSourceOrigin:
		return SourceKind_SOURCE_KIND_ORIGIN
	case HeaderSourceMixed:
		return SourceKind_SOURCE_KIND_MIXED
	case HeaderSourceDelta:
		return SourceKind_SOURCE_KIND_DELTA
	default:
		return SourceKind_SOURCE_KIND_UNSPECIFIED
	}
}

// FromSourceKind converts SourceKind to HeaderSource
func FromSourceKind(kind SourceKind) HeaderSource {
	switch kind {
	case SourceKind_SOURCE_KIND_ORIGIN:
		return HeaderSourceOrigin
	case SourceKind_SOURCE_KIND_MIXED:
		return HeaderSourceMixed
	case SourceKind_SOURCE_KIND_DELTA:
		return HeaderSourceDelta
	default:
		return HeaderSourceOrigin // default to origin
	}
}

type Header struct {
	HeaderKey     string
	Description   string
	Value         string
	Enable        bool
	ID            idwrap.IDWrap
	DeltaParentID *idwrap.IDWrap
	ExampleID     idwrap.IDWrap
	Prev          *idwrap.IDWrap
	Next          *idwrap.IDWrap
}

func (h Header) IsEnabled() bool {
	return h.Enable
}

// DetermineDeltaType determines the delta type based on the header's relationships
// This function will replace the need for storing Source explicitly
func (h *Header) DetermineDeltaType(exampleHasVersionParent bool) HeaderSource {
	// If no DeltaParentID, determine based on example type
	if h.DeltaParentID == nil {
		if exampleHasVersionParent {
			// No parent in a delta example = standalone DELTA item
			return HeaderSourceDelta
		}
		// No parent in origin example = ORIGIN item
		return HeaderSourceOrigin
	}

	// Has DeltaParentID - determine based on example type
	if exampleHasVersionParent {
		// In delta example with parent reference = MIXED (modified from parent)
		return HeaderSourceMixed
	}

	// In origin example with parent reference = MIXED
	return HeaderSourceMixed
}

// IsModified checks if the header has been modified from its parent
// This would need to be implemented based on your modification tracking logic
func (h *Header) IsModified() bool {
	// TODO: Implement modification detection logic
	// This could involve:
	// 1. Comparing with parent header values
	// 2. Checking a modification timestamp
	// 3. Using a separate modification tracking table
	return false
}
