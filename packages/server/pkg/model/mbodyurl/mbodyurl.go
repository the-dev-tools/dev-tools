//nolint:revive // exported
package mbodyurl

import (
	"the-dev-tools/server/pkg/idwrap"
	// deltav1 "the-dev-tools/spec/dist/buf/go/api/delta/v1" // TODO: Re-enable when delta v1 is available
)

type BodyURLEncodedSource int8

const (
	BodyURLEncodedSourceOrigin BodyURLEncodedSource = 1
	BodyURLEncodedSourceDelta  BodyURLEncodedSource = 2
	BodyURLEncodedSourceMixed  BodyURLEncodedSource = 3
)

// TODO: Re-enable when delta v1 is available
/*
func (s BodyURLEncodedSource) ToSourceKind() deltav1.SourceKind {
	switch s {
	case BodyURLEncodedSourceOrigin:
		return deltav1.SourceKind_SOURCE_KIND_ORIGIN
	case BodyURLEncodedSourceDelta:
		return deltav1.SourceKind_SOURCE_KIND_DELTA
	case BodyURLEncodedSourceMixed:
		return deltav1.SourceKind_SOURCE_KIND_MIXED
	default:
		return deltav1.SourceKind_SOURCE_KIND_UNSPECIFIED
	}
}

func FromSourceKind(kind deltav1.SourceKind) BodyURLEncodedSource {
	switch kind {
	case deltav1.SourceKind_SOURCE_KIND_ORIGIN:
		return BodyURLEncodedSourceOrigin
	case deltav1.SourceKind_SOURCE_KIND_DELTA:
		return BodyURLEncodedSourceDelta
	case deltav1.SourceKind_SOURCE_KIND_MIXED:
		return BodyURLEncodedSourceMixed
	default:
		return BodyURLEncodedSourceOrigin
	}
}
*/

type BodyURLEncoded struct {
	BodyKey       string         `json:"body_key"`
	Description   string         `json:"description"`
	Value         string         `json:"value"`
	Enable        bool           `json:"enable"`
	DeltaParentID *idwrap.IDWrap `json:"delta_parent_id"`
	ID            idwrap.IDWrap  `json:"id"`
	ExampleID     idwrap.IDWrap  `json:"example_id"`
}

func (bue BodyURLEncoded) IsEnabled() bool {
	return bue.Enable
}

// DetermineDeltaType determines the delta type based on the body URL encoded's relationships
// This function replaces the need for storing Source explicitly
func (bue *BodyURLEncoded) DetermineDeltaType(exampleHasVersionParent bool) BodyURLEncodedSource {
    // New items created directly in a delta example (no parent) are DELTA
    if exampleHasVersionParent && bue.DeltaParentID == nil {
        return BodyURLEncodedSourceDelta
    }
    // Items with parent:
    if bue.DeltaParentID != nil {
        if exampleHasVersionParent {
            return BodyURLEncodedSourceDelta
        }
        return BodyURLEncodedSourceMixed
    }
    // Default: origin example, no parent
    return BodyURLEncodedSourceOrigin
}
