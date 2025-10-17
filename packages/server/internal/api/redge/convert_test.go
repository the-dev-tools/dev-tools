package redge

import (
	"errors"
	"testing"

	"the-dev-tools/server/pkg/flow/edge"
	edgev1 "the-dev-tools/spec/dist/buf/go/flow/edge/v1"
)

func TestEdgeHandleToProto(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   edge.EdgeHandle
		want    edgev1.Handle
		wantErr error
	}{
		{
			name:    "unspecified",
			input:   edge.HandleUnspecified,
			want:    edgev1.Handle_HANDLE_UNSPECIFIED,
			wantErr: nil,
		},
		{
			name:    "then",
			input:   edge.HandleThen,
			want:    edgev1.Handle_HANDLE_THEN,
			wantErr: nil,
		},
		{
			name:    "else",
			input:   edge.HandleElse,
			want:    edgev1.Handle_HANDLE_ELSE,
			wantErr: nil,
		},
		{
			name:    "loop",
			input:   edge.HandleLoop,
			want:    edgev1.Handle_HANDLE_LOOP,
			wantErr: nil,
		},
		{
			name:    "unknown",
			input:   edge.EdgeHandle(99),
			want:    protoHandleFallback,
			wantErr: errUnknownEdgeHandle,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := edgeHandleToProto(tt.input)
			if got != tt.want {
				t.Fatalf("edgeHandleToProto(%d) = %v, want %v", tt.input, got, tt.want)
			}

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("edgeHandleToProto(%d) error = %v, want %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestEdgeHandleFromProto(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   edgev1.Handle
		want    edge.EdgeHandle
		wantErr error
	}{
		{
			name:    "unspecified",
			input:   edgev1.Handle_HANDLE_UNSPECIFIED,
			want:    edge.HandleUnspecified,
			wantErr: nil,
		},
		{
			name:    "then",
			input:   edgev1.Handle_HANDLE_THEN,
			want:    edge.HandleThen,
			wantErr: nil,
		},
		{
			name:    "else",
			input:   edgev1.Handle_HANDLE_ELSE,
			want:    edge.HandleElse,
			wantErr: nil,
		},
		{
			name:    "loop",
			input:   edgev1.Handle_HANDLE_LOOP,
			want:    edge.HandleLoop,
			wantErr: nil,
		},
		{
			name:    "unknown",
			input:   edgev1.Handle(99),
			want:    edge.HandleUnspecified,
			wantErr: errUnknownEdgeHandle,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := edgeHandleFromProto(tt.input)
			if got != tt.want {
				t.Fatalf("edgeHandleFromProto(%v) = %d, want %d", tt.input, got, tt.want)
			}

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("edgeHandleFromProto(%v) error = %v, want %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestEdgeKindToProto(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   edge.EdgeKind
		want    edgev1.EdgeKind
		wantErr error
	}{
		{
			name:    "unspecified",
			input:   edge.EdgeKindUnspecified,
			want:    edgev1.EdgeKind_EDGE_KIND_UNSPECIFIED,
			wantErr: nil,
		},
		{
			name:    "no-op",
			input:   edge.EdgeKindNoOp,
			want:    edgev1.EdgeKind_EDGE_KIND_NO_OP,
			wantErr: nil,
		},
		{
			name:    "unknown",
			input:   edge.EdgeKind(42),
			want:    protoEdgeKindFallback,
			wantErr: errUnknownEdgeKind,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := edgeKindToProto(tt.input)
			if got != tt.want {
				t.Fatalf("edgeKindToProto(%d) = %v, want %v", tt.input, got, tt.want)
			}

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("edgeKindToProto(%d) error = %v, want %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestEdgeKindFromProto(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   edgev1.EdgeKind
		want    edge.EdgeKind
		wantErr error
	}{
		{
			name:    "unspecified",
			input:   edgev1.EdgeKind_EDGE_KIND_UNSPECIFIED,
			want:    edge.EdgeKindUnspecified,
			wantErr: nil,
		},
		{
			name:    "no-op",
			input:   edgev1.EdgeKind_EDGE_KIND_NO_OP,
			want:    edge.EdgeKindNoOp,
			wantErr: nil,
		},
		{
			name:    "unknown",
			input:   edgev1.EdgeKind(99),
			want:    edge.EdgeKindUnspecified,
			wantErr: errUnknownEdgeKind,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := edgeKindFromProto(tt.input)
			if got != tt.want {
				t.Fatalf("edgeKindFromProto(%v) = %d, want %d", tt.input, got, tt.want)
			}

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("edgeKindFromProto(%v) error = %v, want %v", tt.input, err, tt.wantErr)
			}
		})
	}
}
