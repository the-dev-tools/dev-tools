package mexampleheader

import (
	"the-dev-tools/server/pkg/idwrap"
	deltav1 "the-dev-tools/spec/dist/buf/go/delta/v1"
)

// HeaderSource represents the source kind of header for delta operations
type HeaderSource int8

const (
	HeaderSourceOrigin HeaderSource = 1 // SOURCE_KIND_ORIGIN
	HeaderSourceMixed  HeaderSource = 2 // SOURCE_KIND_MIXED
	HeaderSourceDelta  HeaderSource = 3 // SOURCE_KIND_DELTA
)

// ToSourceKind converts HeaderSource to deltav1.SourceKind
func (s HeaderSource) ToSourceKind() deltav1.SourceKind {
	switch s {
	case HeaderSourceOrigin:
		return deltav1.SourceKind_SOURCE_KIND_ORIGIN
	case HeaderSourceMixed:
		return deltav1.SourceKind_SOURCE_KIND_MIXED
	case HeaderSourceDelta:
		return deltav1.SourceKind_SOURCE_KIND_DELTA
	default:
		return deltav1.SourceKind_SOURCE_KIND_UNSPECIFIED
	}
}

// FromSourceKind converts deltav1.SourceKind to HeaderSource
func FromSourceKind(kind deltav1.SourceKind) HeaderSource {
	switch kind {
	case deltav1.SourceKind_SOURCE_KIND_ORIGIN:
		return HeaderSourceOrigin
	case deltav1.SourceKind_SOURCE_KIND_MIXED:
		return HeaderSourceMixed
	case deltav1.SourceKind_SOURCE_KIND_DELTA:
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
