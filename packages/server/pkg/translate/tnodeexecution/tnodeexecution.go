package tnodeexecution

import (
	"encoding/json"
	"fmt"
	"time"

	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnodeexecution"
	nodeexecutionv1 "the-dev-tools/spec/dist/buf/go/flow/node/execution/v1"
	nodev1 "the-dev-tools/spec/dist/buf/go/flow/node/v1"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func fallbackNodeState() nodev1.NodeState {
	// fallbackNodeState returns the proto value used when the state is unknown.
	return nodev1.NodeState_NODE_STATE_UNSPECIFIED
}

func modelNodeStateToProto(state mnnode.NodeState) (nodev1.NodeState, error) {
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
		return fallbackNodeState(), fmt.Errorf("unknown model node state %d", state)
	}
}

func modelNodeExecutionStateToProto(state int8) (nodev1.NodeState, error) {
	return modelNodeStateToProto(mnnode.NodeState(state))
}

func SerializeNodeExecutionModelToRPC(ne *mnodeexecution.NodeExecution) (*nodeexecutionv1.NodeExecution, error) {
	rpc := &nodeexecutionv1.NodeExecution{
		NodeExecutionId: ne.ID.Bytes(),
		NodeId:          ne.NodeID.Bytes(),
		Name:            ne.Name,
	}

	state, stateErr := modelNodeExecutionStateToProto(ne.State)
	if stateErr != nil {
		state = fallbackNodeState()
	}
	rpc.State = state

	// Handle optional error
	if ne.Error != nil {
		rpc.Error = ne.Error
	}

	// Decompress and convert input JSON to structpb.Value
	if inputJSON, err := ne.GetInputJSON(); err == nil && inputJSON != nil {
		var inputValue interface{}
		if err := json.Unmarshal(inputJSON, &inputValue); err == nil {
			rpc.Input, _ = structpb.NewValue(inputValue)
		}
	}

	// Decompress and convert output JSON to structpb.Value
	if outputJSON, err := ne.GetOutputJSON(); err == nil && outputJSON != nil {
		var outputValue interface{}
		if err := json.Unmarshal(outputJSON, &outputValue); err == nil {
			rpc.Output, _ = structpb.NewValue(outputValue)
		}
	}

	// Convert CompletedAt timestamp
	if ne.CompletedAt != nil {
		rpc.CompletedAt = timestamppb.New(time.UnixMilli(*ne.CompletedAt))
	}

	// Handle optional ResponseID
	if ne.ResponseID != nil {
		rpc.ResponseId = ne.ResponseID.Bytes()
	}

	if stateErr != nil {
		return rpc, fmt.Errorf("serialize node execution state: %w", stateErr)
	}

	return rpc, nil
}

func SerializeNodeExecutionModelToRPCListItem(ne *mnodeexecution.NodeExecution) (*nodeexecutionv1.NodeExecutionListItem, error) {
	rpc := &nodeexecutionv1.NodeExecutionListItem{
		NodeExecutionId: ne.ID.Bytes(),
		NodeId:          ne.NodeID.Bytes(),
		Name:            ne.Name,
	}

	state, stateErr := modelNodeExecutionStateToProto(ne.State)
	if stateErr != nil {
		state = fallbackNodeState()
	}
	rpc.State = state

	// Handle optional error
	if ne.Error != nil {
		rpc.Error = ne.Error
	}

	// Convert CompletedAt timestamp
	if ne.CompletedAt != nil {
		rpc.CompletedAt = timestamppb.New(time.UnixMilli(*ne.CompletedAt))
	}

	// Handle optional ResponseID
	if ne.ResponseID != nil {
		rpc.ResponseId = ne.ResponseID.Bytes()
	}

	if stateErr != nil {
		return rpc, fmt.Errorf("serialize node execution list item state: %w", stateErr)
	}

	return rpc, nil
}

func SerializeNodeExecutionModelToRPCGetResponse(ne *mnodeexecution.NodeExecution) (*nodeexecutionv1.NodeExecutionGetResponse, error) {
	rpc := &nodeexecutionv1.NodeExecutionGetResponse{
		NodeExecutionId: ne.ID.Bytes(),
		NodeId:          ne.NodeID.Bytes(),
		Name:            ne.Name,
	}

	state, stateErr := modelNodeExecutionStateToProto(ne.State)
	if stateErr != nil {
		state = fallbackNodeState()
	}
	rpc.State = state

	// Handle optional error
	if ne.Error != nil {
		rpc.Error = ne.Error
	}

	// Decompress and convert input JSON to structpb.Value
	if inputJSON, err := ne.GetInputJSON(); err == nil && inputJSON != nil {
		var inputValue interface{}
		if err := json.Unmarshal(inputJSON, &inputValue); err == nil {
			rpc.Input, _ = structpb.NewValue(inputValue)
		}
	}

	// Decompress and convert output JSON to structpb.Value
	if outputJSON, err := ne.GetOutputJSON(); err == nil && outputJSON != nil {
		var outputValue interface{}
		if err := json.Unmarshal(outputJSON, &outputValue); err == nil {
			rpc.Output, _ = structpb.NewValue(outputValue)
		}
	}

	// Convert CompletedAt timestamp
	if ne.CompletedAt != nil {
		rpc.CompletedAt = timestamppb.New(time.UnixMilli(*ne.CompletedAt))
	}

	// Handle optional ResponseID
	if ne.ResponseID != nil {
		rpc.ResponseId = ne.ResponseID.Bytes()
	}

	if stateErr != nil {
		return rpc, fmt.Errorf("serialize node execution get response state: %w", stateErr)
	}

	return rpc, nil
}
