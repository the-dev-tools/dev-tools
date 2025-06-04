package mbodyform

import (
	"the-dev-tools/server/pkg/idwrap"
	deltav1 "the-dev-tools/spec/dist/buf/go/delta/v1"
)

// BodyFormSource represents the source type for body forms
type BodyFormSource int8

const (
	BodyFormSourceOrigin BodyFormSource = 1
	BodyFormSourceMixed  BodyFormSource = 2
	BodyFormSourceDelta  BodyFormSource = 3
)

// ToSourceKind converts BodyFormSource to deltav1.SourceKind
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

type BodyForm struct {
	BodyKey       string         `json:"body_key"`
	Description   string         `json:"description"`
	Value         string         `json:"value"`
	Enable        bool           `json:"enable"`
	DeltaParentID *idwrap.IDWrap `json:"delta_parent_id"`
	ID            idwrap.IDWrap  `json:"id"`
	ExampleID     idwrap.IDWrap  `json:"example_id"`
	Source        BodyFormSource `json:"source"`
}

func (bf BodyForm) IsEnabled() bool {
	return bf.Enable
}
