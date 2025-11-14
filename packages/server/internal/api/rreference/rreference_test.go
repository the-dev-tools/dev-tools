package rreference

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"the-dev-tools/server/pkg/reference"
	"the-dev-tools/server/pkg/referencecompletion"
	referencev1 "the-dev-tools/spec/dist/buf/go/api/reference/v1"
)

func TestReferenceKindToProto(t *testing.T) {
	tests := []struct {
		name string
		kind reference.ReferenceKind
		want referencev1.ReferenceKind
	}{
		{"unspecified", reference.ReferenceKind_REFERENCE_KIND_UNSPECIFIED, referencev1.ReferenceKind_REFERENCE_KIND_UNSPECIFIED},
		{"map", reference.ReferenceKind_REFERENCE_KIND_MAP, referencev1.ReferenceKind_REFERENCE_KIND_MAP},
		{"array", reference.ReferenceKind_REFERENCE_KIND_ARRAY, referencev1.ReferenceKind_REFERENCE_KIND_ARRAY},
		{"value", reference.ReferenceKind_REFERENCE_KIND_VALUE, referencev1.ReferenceKind_REFERENCE_KIND_VALUE},
		{"variable", reference.ReferenceKind_REFERENCE_KIND_VARIABLE, referencev1.ReferenceKind_REFERENCE_KIND_VARIABLE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := referenceKindToProto(tt.kind)
			if err != nil {
				t.Fatalf("referenceKindToProto(%v) unexpected error: %v", tt.kind, err)
			}
			if got != tt.want {
				t.Fatalf("referenceKindToProto(%v) got %v, want %v", tt.kind, got, tt.want)
			}
		})
	}
}

func TestReferenceKindToProtoFallback(t *testing.T) {
	got, err := referenceKindToProto(reference.ReferenceKind(-1))
	if err == nil {
		t.Fatal("expected error for unknown reference kind")
	}
	if got != referenceKindProtoFallback {
		t.Fatalf("expected fallback kind %v, got %v", referenceKindProtoFallback, got)
	}
}

func TestReferenceKindFromProto(t *testing.T) {
	tests := []struct {
		name string
		kind referencev1.ReferenceKind
		want reference.ReferenceKind
	}{
		{"unspecified", referencev1.ReferenceKind_REFERENCE_KIND_UNSPECIFIED, reference.ReferenceKind_REFERENCE_KIND_UNSPECIFIED},
		{"map", referencev1.ReferenceKind_REFERENCE_KIND_MAP, reference.ReferenceKind_REFERENCE_KIND_MAP},
		{"array", referencev1.ReferenceKind_REFERENCE_KIND_ARRAY, reference.ReferenceKind_REFERENCE_KIND_ARRAY},
		{"value", referencev1.ReferenceKind_REFERENCE_KIND_VALUE, reference.ReferenceKind_REFERENCE_KIND_VALUE},
		{"variable", referencev1.ReferenceKind_REFERENCE_KIND_VARIABLE, reference.ReferenceKind_REFERENCE_KIND_VARIABLE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := referenceKindFromProto(tt.kind)
			if err != nil {
				t.Fatalf("referenceKindFromProto(%v) unexpected error: %v", tt.kind, err)
			}
			if got != tt.want {
				t.Fatalf("referenceKindFromProto(%v) got %v, want %v", tt.kind, got, tt.want)
			}
		})
	}
}

func TestReferenceKindFromProtoFallback(t *testing.T) {
	got, err := referenceKindFromProto(referencev1.ReferenceKind(-1))
	if err == nil {
		t.Fatal("expected error for unknown proto reference kind")
	}
	if got != reference.ReferenceKind_REFERENCE_KIND_UNSPECIFIED {
		t.Fatalf("expected fallback kind %v, got %v", reference.ReferenceKind_REFERENCE_KIND_UNSPECIFIED, got)
	}
}

func TestReferenceCompletionInvalidKind(t *testing.T) {
	t.Cleanup(func() {
		convertReferenceCompletionItemsFn = convertReferenceCompletionItems
	})

	convertReferenceCompletionItemsFn = func(items []referencecompletion.ReferenceCompletionItem) ([]*referencev1.ReferenceCompletion, error) {
		invalid := []referencecompletion.ReferenceCompletionItem{
			{Kind: reference.ReferenceKind(99)},
		}
		return convertReferenceCompletionItems(invalid)
	}

	svc := &ReferenceServiceRPC{}
	req := connect.NewRequest(&referencev1.ReferenceCompletionRequest{})

	_, err := svc.ReferenceCompletion(context.Background(), req)
	if err == nil {
		t.Fatal("expected ReferenceCompletion to return an error for invalid kind")
	}

	if connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("expected internal error code, got %v", connect.CodeOf(err))
	}
}
