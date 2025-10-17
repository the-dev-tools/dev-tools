package rflow

import (
	"fmt"

	"the-dev-tools/server/pkg/model/mnnode"
	nodev1 "the-dev-tools/spec/dist/buf/go/flow/node/v1"
)

// nodeStateProtoFallback is used when persisted node state cannot be mapped.
const nodeStateProtoFallback = nodev1.NodeState_NODE_STATE_UNSPECIFIED

func nodeStateModelToProto(state mnnode.NodeState) (nodev1.NodeState, error) {
	switch state {
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

func nodeStateIntToProto(state int8) (nodev1.NodeState, error) {
	return nodeStateModelToProto(mnnode.NodeState(state))
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
