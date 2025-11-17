package mbodyform

import (
	"the-dev-tools/server/pkg/idwrap"
	// deltav1 "the-dev-tools/spec/dist/buf/go/api/delta/v1" // TODO: Re-enable when delta v1 is available
)

// BodyFormSource represents the source type for body forms
type BodyFormSource int8

const (
	BodyFormSourceOrigin BodyFormSource = 1
	BodyFormSourceMixed  BodyFormSource = 2
	BodyFormSourceDelta  BodyFormSource = 3
)

// ToSourceKind converts BodyFormSource to deltav1.SourceKind
// TODO: Re-enable when delta v1 is available
/*
func (s BodyFormSource) ToSourceKind() deltav1.SourceKind {
	switch s {
	case BodyFormSourceOrigin:
		return deltav1.SourceKind_SOURCE_KIND_ORIGIN
	case BodyFormSourceMixed:
		return deltav1.SourceKind_SOURCE_KIND_MIXED
	case BodyFormSourceDelta:
		return deltav1.SourceKind_SOURCE_KIND_DELTA
	default:
		return deltav1.SourceKind_SOURCE_KIND_UNSPECIFIED
	}
}

// FromSourceKind converts deltav1.SourceKind to BodyFormSource
func FromSourceKind(kind deltav1.SourceKind) BodyFormSource {
	switch kind {
	case deltav1.SourceKind_SOURCE_KIND_ORIGIN:
		return BodyFormSourceOrigin
	case deltav1.SourceKind_SOURCE_KIND_MIXED:
		return BodyFormSourceMixed
	case deltav1.SourceKind_SOURCE_KIND_DELTA:
		return BodyFormSourceDelta
	default:
		return BodyFormSourceOrigin
	}
}
*/

type BodyForm struct {
	BodyKey       string         `json:"body_key"`
	Description   string         `json:"description"`
	Value         string         `json:"value"`
	Enable        bool           `json:"enable"`
	DeltaParentID *idwrap.IDWrap `json:"delta_parent_id"`
	ID            idwrap.IDWrap  `json:"id"`
	ExampleID     idwrap.IDWrap  `json:"example_id"`
}

func (bf BodyForm) IsEnabled() bool {
	return bf.Enable
}

// DetermineDeltaType determines the delta type based on the body form's relationships
// This function replaces the need for storing Source explicitly
func (bf *BodyForm) DetermineDeltaType(exampleHasVersionParent bool) BodyFormSource {
	// If no DeltaParentID, this is not a delta body form
	if bf.DeltaParentID == nil {
		return BodyFormSourceOrigin
	}

	// If example has VersionParentID, this is a delta example
	if exampleHasVersionParent {
		// BodyForm has DeltaParentID and example is delta -> DELTA body form
		return BodyFormSourceDelta
	}

	// If example has no VersionParentID, it's an original example
	// BodyForm has DeltaParentID but example is original -> MIXED body form
	return BodyFormSourceMixed
}
