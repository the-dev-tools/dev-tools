package rnode

import (
	"fmt"

	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	nodev1 "the-dev-tools/spec/dist/buf/go/flow/node/v1"
)

// nodeKindProtoFallback is returned when a node kind cannot be mapped from the model.
const nodeKindProtoFallback = nodev1.NodeKind_NODE_KIND_UNSPECIFIED

// nodeStateProtoFallback is returned when a node state cannot be mapped from the model.
const nodeStateProtoFallback = nodev1.NodeState_NODE_STATE_UNSPECIFIED

// nodeNoOpProtoFallback is returned when a noop kind cannot be mapped from the model.
const nodeNoOpProtoFallback = nodev1.NodeNoOpKind_NODE_NO_OP_KIND_UNSPECIFIED

// loopErrorHandlingProtoFallback is returned when loop error handling cannot be mapped from the model.
const loopErrorHandlingProtoFallback = nodev1.ErrorHandling_ERROR_HANDLING_UNSPECIFIED

func nodeKindModelToProto(kind mnnode.NodeKind) (nodev1.NodeKind, error) {
	switch kind {
	case mnnode.NODE_KIND_UNSPECIFIED:
		return nodev1.NodeKind_NODE_KIND_UNSPECIFIED, nil
	case mnnode.NODE_KIND_NO_OP:
		return nodev1.NodeKind_NODE_KIND_NO_OP, nil
	case mnnode.NODE_KIND_REQUEST:
		return nodev1.NodeKind_NODE_KIND_REQUEST, nil
	case mnnode.NODE_KIND_CONDITION:
		return nodev1.NodeKind_NODE_KIND_CONDITION, nil
	case mnnode.NODE_KIND_FOR:
		return nodev1.NodeKind_NODE_KIND_FOR, nil
	case mnnode.NODE_KIND_FOR_EACH:
		return nodev1.NodeKind_NODE_KIND_FOR_EACH, nil
	case mnnode.NODE_KIND_JS:
		return nodev1.NodeKind_NODE_KIND_JS, nil
	default:
		return nodeKindProtoFallback, fmt.Errorf("unknown node kind %d", kind)
	}
}

func nodeKindProtoToModel(kind nodev1.NodeKind) (mnnode.NodeKind, error) {
	switch kind {
	case nodev1.NodeKind_NODE_KIND_UNSPECIFIED:
		return mnnode.NODE_KIND_UNSPECIFIED, nil
	case nodev1.NodeKind_NODE_KIND_NO_OP:
		return mnnode.NODE_KIND_NO_OP, nil
	case nodev1.NodeKind_NODE_KIND_REQUEST:
		return mnnode.NODE_KIND_REQUEST, nil
	case nodev1.NodeKind_NODE_KIND_CONDITION:
		return mnnode.NODE_KIND_CONDITION, nil
	case nodev1.NodeKind_NODE_KIND_FOR:
		return mnnode.NODE_KIND_FOR, nil
	case nodev1.NodeKind_NODE_KIND_FOR_EACH:
		return mnnode.NODE_KIND_FOR_EACH, nil
	case nodev1.NodeKind_NODE_KIND_JS:
		return mnnode.NODE_KIND_JS, nil
	default:
		return mnnode.NODE_KIND_UNSPECIFIED, fmt.Errorf("unknown node kind %v", kind)
	}
}

func nodeStateModelToProto(state int8) (nodev1.NodeState, error) {
	switch mnnode.NodeState(state) {
	case mnnode.NODE_STATE_UNSPECIFIED:
		return nodev1.NodeState_NODE_STATE_UNSPECIFIED, nil
	case mnnode.NODE_STATE_RUNNING:
		return nodev1.NodeState_NODE_STATE_RUNNING, nil
	case mnnode.NODE_STATE_SUCCESS:
		return nodev1.NodeState_NODE_STATE_SUCCESS, nil
	case mnnode.NODE_STATE_FAILURE:
		return nodev1.NodeState_NODE_STATE_FAILURE, nil
	case mnnode.NODE_STATE_CANCELED:
		return nodev1.NodeState_NODE_STATE_CANCELED, nil
	default:
		return nodeStateProtoFallback, fmt.Errorf("unknown node state %d", state)
	}
}

func nodeStateProtoToModel(state nodev1.NodeState) (mnnode.NodeState, error) {
	switch state {
	case nodev1.NodeState_NODE_STATE_UNSPECIFIED:
		return mnnode.NODE_STATE_UNSPECIFIED, nil
	case nodev1.NodeState_NODE_STATE_RUNNING:
		return mnnode.NODE_STATE_RUNNING, nil
	case nodev1.NodeState_NODE_STATE_SUCCESS:
		return mnnode.NODE_STATE_SUCCESS, nil
	case nodev1.NodeState_NODE_STATE_FAILURE:
		return mnnode.NODE_STATE_FAILURE, nil
	case nodev1.NodeState_NODE_STATE_CANCELED:
		return mnnode.NODE_STATE_CANCELED, nil
	default:
		return mnnode.NODE_STATE_UNSPECIFIED, fmt.Errorf("unknown node state %v", state)
	}
}

func nodeNoOpModelToProto(kind mnnoop.NoopTypes) (nodev1.NodeNoOpKind, error) {
	switch kind {
	case mnnoop.NODE_NO_OP_KIND_UNSPECIFIED:
		return nodev1.NodeNoOpKind_NODE_NO_OP_KIND_UNSPECIFIED, nil
	case mnnoop.NODE_NO_OP_KIND_START:
		return nodev1.NodeNoOpKind_NODE_NO_OP_KIND_START, nil
	case mnnoop.NODE_NO_OP_KIND_CREATE:
		return nodev1.NodeNoOpKind_NODE_NO_OP_KIND_CREATE, nil
	case mnnoop.NODE_NO_OP_KIND_THEN:
		return nodev1.NodeNoOpKind_NODE_NO_OP_KIND_THEN, nil
	case mnnoop.NODE_NO_OP_KIND_ELSE:
		return nodev1.NodeNoOpKind_NODE_NO_OP_KIND_ELSE, nil
	case mnnoop.NODE_NO_OP_KIND_LOOP:
		return nodev1.NodeNoOpKind_NODE_NO_OP_KIND_LOOP, nil
	default:
		return nodeNoOpProtoFallback, fmt.Errorf("unknown node noop kind %d", kind)
	}
}

func nodeNoOpProtoToModel(kind nodev1.NodeNoOpKind) (mnnoop.NoopTypes, error) {
	switch kind {
	case nodev1.NodeNoOpKind_NODE_NO_OP_KIND_UNSPECIFIED:
		return mnnoop.NODE_NO_OP_KIND_UNSPECIFIED, nil
	case nodev1.NodeNoOpKind_NODE_NO_OP_KIND_START:
		return mnnoop.NODE_NO_OP_KIND_START, nil
	case nodev1.NodeNoOpKind_NODE_NO_OP_KIND_CREATE:
		return mnnoop.NODE_NO_OP_KIND_CREATE, nil
	case nodev1.NodeNoOpKind_NODE_NO_OP_KIND_THEN:
		return mnnoop.NODE_NO_OP_KIND_THEN, nil
	case nodev1.NodeNoOpKind_NODE_NO_OP_KIND_ELSE:
		return mnnoop.NODE_NO_OP_KIND_ELSE, nil
	case nodev1.NodeNoOpKind_NODE_NO_OP_KIND_LOOP:
		return mnnoop.NODE_NO_OP_KIND_LOOP, nil
	default:
		return mnnoop.NODE_NO_OP_KIND_UNSPECIFIED, fmt.Errorf("unknown node noop kind %v", kind)
	}
}

func loopErrorHandlingModelToProto(h mnfor.ErrorHandling) (nodev1.ErrorHandling, error) {
	switch h {
	case mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED:
		return nodev1.ErrorHandling_ERROR_HANDLING_UNSPECIFIED, nil
	case mnfor.ErrorHandling_ERROR_HANDLING_IGNORE:
		return nodev1.ErrorHandling_ERROR_HANDLING_IGNORE, nil
	case mnfor.ErrorHandling_ERROR_HANDLING_BREAK:
		return nodev1.ErrorHandling_ERROR_HANDLING_BREAK, nil
	default:
		return loopErrorHandlingProtoFallback, fmt.Errorf("unknown loop error handling %d", h)
	}
}

func loopErrorHandlingProtoToModel(h nodev1.ErrorHandling) (mnfor.ErrorHandling, error) {
	switch h {
	case nodev1.ErrorHandling_ERROR_HANDLING_UNSPECIFIED:
		return mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED, nil
	case nodev1.ErrorHandling_ERROR_HANDLING_IGNORE:
		return mnfor.ErrorHandling_ERROR_HANDLING_IGNORE, nil
	case nodev1.ErrorHandling_ERROR_HANDLING_BREAK:
		return mnfor.ErrorHandling_ERROR_HANDLING_BREAK, nil
	default:
		return mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED, fmt.Errorf("unknown loop error handling %v", h)
	}
}
