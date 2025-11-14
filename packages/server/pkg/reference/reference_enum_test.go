package reference

import (
	"testing"

	referencev1 "the-dev-tools/spec/dist/buf/go/api/reference/v1"
)

func TestReferenceKindToProto(t *testing.T) {
	tests := []struct {
		name    string
		kind    ReferenceKind
		want    referencev1.ReferenceKind
		wantErr bool
	}{
		{
			name: "unspecified",
			kind: ReferenceKind_REFERENCE_KIND_UNSPECIFIED,
			want: referencev1.ReferenceKind_REFERENCE_KIND_UNSPECIFIED,
		},
		{
			name: "map",
			kind: ReferenceKind_REFERENCE_KIND_MAP,
			want: referencev1.ReferenceKind_REFERENCE_KIND_MAP,
		},
		{
			name: "array",
			kind: ReferenceKind_REFERENCE_KIND_ARRAY,
			want: referencev1.ReferenceKind_REFERENCE_KIND_ARRAY,
		},
		{
			name: "value",
			kind: ReferenceKind_REFERENCE_KIND_VALUE,
			want: referencev1.ReferenceKind_REFERENCE_KIND_VALUE,
		},
		{
			name: "variable",
			kind: ReferenceKind_REFERENCE_KIND_VARIABLE,
			want: referencev1.ReferenceKind_REFERENCE_KIND_VARIABLE,
		},
		{
			name:    "unknown",
			kind:    ReferenceKind(99),
			want:    referencev1.ReferenceKind_REFERENCE_KIND_UNSPECIFIED,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := referenceKindToProto(tt.kind)
			if (err != nil) != tt.wantErr {
				t.Fatalf("referenceKindToProto error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("referenceKindToProto = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReferenceKindFromProto(t *testing.T) {
	tests := []struct {
		name    string
		kind    referencev1.ReferenceKind
		want    ReferenceKind
		wantErr bool
	}{
		{
			name: "unspecified",
			kind: referencev1.ReferenceKind_REFERENCE_KIND_UNSPECIFIED,
			want: ReferenceKind_REFERENCE_KIND_UNSPECIFIED,
		},
		{
			name: "map",
			kind: referencev1.ReferenceKind_REFERENCE_KIND_MAP,
			want: ReferenceKind_REFERENCE_KIND_MAP,
		},
		{
			name: "array",
			kind: referencev1.ReferenceKind_REFERENCE_KIND_ARRAY,
			want: ReferenceKind_REFERENCE_KIND_ARRAY,
		},
		{
			name: "value",
			kind: referencev1.ReferenceKind_REFERENCE_KIND_VALUE,
			want: ReferenceKind_REFERENCE_KIND_VALUE,
		},
		{
			name: "variable",
			kind: referencev1.ReferenceKind_REFERENCE_KIND_VARIABLE,
			want: ReferenceKind_REFERENCE_KIND_VARIABLE,
		},
		{
			name:    "unknown",
			kind:    referencev1.ReferenceKind(99),
			want:    ReferenceKind_REFERENCE_KIND_UNSPECIFIED,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := referenceKindFromProto(tt.kind)
			if (err != nil) != tt.wantErr {
				t.Fatalf("referenceKindFromProto error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("referenceKindFromProto = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReferenceKeyKindToProto(t *testing.T) {
	tests := []struct {
		name    string
		kind    ReferenceKeyKind
		want    referencev1.ReferenceKeyKind
		wantErr bool
	}{
		{
			name: "unspecified",
			kind: ReferenceKeyKind_REFERENCE_KEY_KIND_UNSPECIFIED,
			want: referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_UNSPECIFIED,
		},
		{
			name: "group",
			kind: ReferenceKeyKind_REFERENCE_KEY_KIND_GROUP,
			want: referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_GROUP,
		},
		{
			name: "key",
			kind: ReferenceKeyKind_REFERENCE_KEY_KIND_KEY,
			want: referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY,
		},
		{
			name: "index",
			kind: ReferenceKeyKind_REFERENCE_KEY_KIND_INDEX,
			want: referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_INDEX,
		},
		{
			name: "any",
			kind: ReferenceKeyKind_REFERENCE_KEY_KIND_ANY,
			want: referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_ANY,
		},
		{
			name:    "unknown",
			kind:    ReferenceKeyKind(99),
			want:    referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_UNSPECIFIED,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := referenceKeyKindToProto(tt.kind)
			if (err != nil) != tt.wantErr {
				t.Fatalf("referenceKeyKindToProto error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("referenceKeyKindToProto = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReferenceKeyKindFromProto(t *testing.T) {
	tests := []struct {
		name    string
		kind    referencev1.ReferenceKeyKind
		want    ReferenceKeyKind
		wantErr bool
	}{
		{
			name: "unspecified",
			kind: referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_UNSPECIFIED,
			want: ReferenceKeyKind_REFERENCE_KEY_KIND_UNSPECIFIED,
		},
		{
			name: "group",
			kind: referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_GROUP,
			want: ReferenceKeyKind_REFERENCE_KEY_KIND_GROUP,
		},
		{
			name: "key",
			kind: referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY,
			want: ReferenceKeyKind_REFERENCE_KEY_KIND_KEY,
		},
		{
			name: "index",
			kind: referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_INDEX,
			want: ReferenceKeyKind_REFERENCE_KEY_KIND_INDEX,
		},
		{
			name: "any",
			kind: referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_ANY,
			want: ReferenceKeyKind_REFERENCE_KEY_KIND_ANY,
		},
		{
			name:    "unknown",
			kind:    referencev1.ReferenceKeyKind(99),
			want:    ReferenceKeyKind_REFERENCE_KEY_KIND_UNSPECIFIED,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := referenceKeyKindFromProto(tt.kind)
			if (err != nil) != tt.wantErr {
				t.Fatalf("referenceKeyKindFromProto error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("referenceKeyKindFromProto = %v, want %v", got, tt.want)
			}
		})
	}
}
