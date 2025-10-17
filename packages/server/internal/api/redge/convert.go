package redge

import (
	"errors"
	"fmt"

	"the-dev-tools/server/pkg/flow/edge"
	edgev1 "the-dev-tools/spec/dist/buf/go/flow/edge/v1"
)

var (
	errUnknownEdgeHandle = errors.New("unknown edge handle")
	errUnknownEdgeKind   = errors.New("unknown edge kind")
)

// protoHandleFallback is the proto enum returned while serialising persisted
// handles that no longer map to a known model value. The helper still surfaces
// an error alongside this fallback so the caller can record the mismatch.
const protoHandleFallback edgev1.Handle = edgev1.Handle_HANDLE_UNSPECIFIED

// protoEdgeKindFallback is the proto enum returned while serialising persisted
// kinds that no longer map to a known model value. The helper still surfaces an
// error alongside this fallback so the caller can record the mismatch.
const protoEdgeKindFallback edgev1.EdgeKind = edgev1.EdgeKind_EDGE_KIND_UNSPECIFIED

func edgeHandleToProto(handle edge.EdgeHandle) (edgev1.Handle, error) {
	switch handle {
	case edge.HandleUnspecified:
		return edgev1.Handle_HANDLE_UNSPECIFIED, nil
	case edge.HandleThen:
		return edgev1.Handle_HANDLE_THEN, nil
	case edge.HandleElse:
		return edgev1.Handle_HANDLE_ELSE, nil
	case edge.HandleLoop:
		return edgev1.Handle_HANDLE_LOOP, nil
	default:
		return protoHandleFallback, fmt.Errorf("redge: %w (%d)", errUnknownEdgeHandle, handle)
	}
}

func edgeHandleFromProto(handle edgev1.Handle) (edge.EdgeHandle, error) {
	switch handle {
	case edgev1.Handle_HANDLE_UNSPECIFIED:
		return edge.HandleUnspecified, nil
	case edgev1.Handle_HANDLE_THEN:
		return edge.HandleThen, nil
	case edgev1.Handle_HANDLE_ELSE:
		return edge.HandleElse, nil
	case edgev1.Handle_HANDLE_LOOP:
		return edge.HandleLoop, nil
	default:
		return edge.HandleUnspecified, fmt.Errorf("redge: %w (%d)", errUnknownEdgeHandle, handle)
	}
}

func edgeKindToProto(kind edge.EdgeKind) (edgev1.EdgeKind, error) {
	switch kind {
	case edge.EdgeKindUnspecified:
		return edgev1.EdgeKind_EDGE_KIND_UNSPECIFIED, nil
	case edge.EdgeKindNoOp:
		return edgev1.EdgeKind_EDGE_KIND_NO_OP, nil
	default:
		return protoEdgeKindFallback, fmt.Errorf("redge: %w (%d)", errUnknownEdgeKind, kind)
	}
}

func edgeKindFromProto(kind edgev1.EdgeKind) (edge.EdgeKind, error) {
	switch kind {
	case edgev1.EdgeKind_EDGE_KIND_UNSPECIFIED:
		return edge.EdgeKindUnspecified, nil
	case edgev1.EdgeKind_EDGE_KIND_NO_OP:
		return edge.EdgeKindNoOp, nil
	default:
		return edge.EdgeKindUnspecified, fmt.Errorf("redge: %w (%d)", errUnknownEdgeKind, kind)
	}
}
