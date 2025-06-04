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
	Source        HeaderSource
	ID            idwrap.IDWrap
	DeltaParentID *idwrap.IDWrap
	ExampleID     idwrap.IDWrap
}

func (h Header) IsEnabled() bool {
	return h.Enable
}
