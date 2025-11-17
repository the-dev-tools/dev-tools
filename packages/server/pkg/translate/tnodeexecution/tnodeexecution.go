package tnodeexecution

import (
	"encoding/json"
	"fmt"
	"time"

	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnodeexecution"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TODO: These protobuf packages don't exist. Stub types provided for compilation.

type NodeState int32
const (
	NodeState_NODE_STATE_UNSPECIFIED NodeState = 0
	NodeState_NODE_STATE_RUNNING NodeState = 1
	NodeState_NODE_STATE_SUCCESS NodeState = 2
	NodeState_NODE_STATE_FAILED NodeState = 3
	NodeState_NODE_STATE_SKIPPED NodeState = 4
)

type NodeExecution struct {
	NodeExecutionId []byte
	NodeId []byte
	FlowExecutionId []byte
	State NodeState
	StartTime *timestamppb.Timestamp
	EndTime *timestamppb.Timestamp
	Input *structpb.Struct
	Output *structpb.Struct
	Error string
}

type NodeExecutionListItem struct {
	NodeExecutionId []byte
	NodeId []byte
	FlowExecutionId []byte
	State NodeState
	StartTime *timestamppb.Timestamp
	EndTime *timestamppb.Timestamp
}

func fallbackNodeState() NodeState {
	return NodeState_NODE_STATE_UNSPECIFIED
}

func modelNodeStateToProto(state mnnode.NodeState) (NodeState, error) {
	switch state {
	case mnnode.NODE_STATE_UNSPECIFIED:
		return NodeState_NODE_STATE_UNSPECIFIED, nil
	case mnnode.NODE_STATE_RUNNING:
		return NodeState_NODE_STATE_RUNNING, nil
	case mnnode.NODE_STATE_SUCCESS:
		return NodeState_NODE_STATE_SUCCESS, nil
	case mnnode.NODE_STATE_FAILED:
		return NodeState_NODE_STATE_FAILED, nil
	case mnnode.NODE_STATE_SKIPPED:
		return NodeState_NODE_STATE_SKIPPED, nil
	default:
		return fallbackNodeState(), fmt.Errorf("unknown node state %d", state)
	}
}

func protoNodeStateToModel(state NodeState) (mnnode.NodeState, error) {
	switch state {
	case NodeState_NODE_STATE_UNSPECIFIED:
		return mnnode.NODE_STATE_UNSPECIFIED, nil
	case NodeState_NODE_STATE_RUNNING:
		return mnnode.NODE_STATE_RUNNING, nil
	case NodeState_NODE_STATE_SUCCESS:
		return mnnode.NODE_STATE_SUCCESS, nil
	case NodeState_NODE_STATE_FAILED:
		return mnnode.NODE_STATE_FAILED, nil
	case NodeState_NODE_STATE_SKIPPED:
		return mnnode.NODE_STATE_SKIPPED, nil
	default:
		return 0, fmt.Errorf("unknown node state enum %v", state)
	}
}

func serializeModelToRPC(ex mnodeexecution.NodeExecution) *NodeExecution {
	inputStruct, _ := structpb.NewStruct(ex.Input)
	outputStruct, _ := structpb.NewStruct(ex.Output)

	var startTime, endTime *timestamppb.Timestamp
	if ex.StartTime != nil {
		startTime = timestamppb.New(*ex.StartTime)
	}
	if ex.EndTime != nil {
		endTime = timestamppb.New(*ex.EndTime)
	}

	state, err := modelNodeStateToProto(ex.State)
	if err != nil {
		state = fallbackNodeState()
	}

	return &NodeExecution{
		NodeExecutionId: ex.ID.Bytes(),
		NodeId: ex.NodeID.Bytes(),
		FlowExecutionId: ex.FlowExecutionID.Bytes(),
		State: state,
		StartTime: startTime,
		EndTime: endTime,
		Input: inputStruct,
		Output: outputStruct,
		Error: ex.Error,
	}
}

func serializeModelToRPCItem(ex mnodeexecution.NodeExecution) *NodeExecutionListItem {
	var startTime, endTime *timestamppb.Timestamp
	if ex.StartTime != nil {
		startTime = timestamppb.New(*ex.StartTime)
	}
	if ex.EndTime != nil {
		endTime = timestamppb.New(*ex.EndTime)
	}

	state, err := modelNodeStateToProto(ex.State)
	if err != nil {
		state = fallbackNodeState()
	}

	return &NodeExecutionListItem{
		NodeExecutionId: ex.ID.Bytes(),
		NodeId: ex.NodeID.Bytes(),
		FlowExecutionId: ex.FlowExecutionID.Bytes(),
		State: state,
		StartTime: startTime,
		EndTime: endTime,
	}
}

func deserializeRPCToModel(ex *NodeExecution) (mnodeexecution.NodeExecution, error) {
	if ex == nil {
		return mnodeexecution.NodeExecution{}, nil
	}

	id, err := mnodeexecution.NewIDFromBytes(ex.NodeExecutionId)
	if err != nil {
		return mnodeexecution.NodeExecution{}, err
	}

	nodeID, err := mnnode.NewIDFromBytes(ex.NodeId)
	if err != nil {
		return mnodeexecution.NodeExecution{}, err
	}

	flowExecutionID, err := mnodeexecution.NewFlowExecutionIDFromBytes(ex.FlowExecutionId)
	if err != nil {
		return mnodeexecution.NodeExecution{}, err
	}

	state, err := protoNodeStateToModel(ex.State)
	if err != nil {
		return mnodeexecution.NodeExecution{}, err
	}

	var input, output map[string]interface{}
	if ex.Input != nil {
		input = ex.Input.AsMap()
	}
	if ex.Output != nil {
		output = ex.Output.AsMap()
	}

	var startTime, endTime *time.Time
	if ex.StartTime != nil {
		startTime = &ex.StartTime.AsTime()
	}
	if ex.EndTime != nil {
		endTime = &ex.EndTime.AsTime()
	}

	return mnodeexecution.NodeExecution{
		ID: id,
		NodeID: nodeID,
		FlowExecutionID: flowExecutionID,
		State: state,
		StartTime: startTime,
		EndTime: endTime,
		Input: input,
		Output: output,
		Error: ex.Error,
	}, nil
}

func deserializeRPCToModelList(items []*NodeExecutionListItem) ([]mnodeexecution.NodeExecution, error) {
	if len(items) == 0 {
		return []mnodeexecution.NodeExecution{}, nil
	}

	result := make([]mnodeexecution.NodeExecution, 0, len(items))
	for _, item := range items {
		ex, err := deserializeRPCToModelListItem(item)
		if err != nil {
			return nil, err
		}
		result = append(result, ex)
	}
	return result, nil
}

func deserializeRPCToModelListItem(item *NodeExecutionListItem) (mnodeexecution.NodeExecution, error) {
	if item == nil {
		return mnodeexecution.NodeExecution{}, nil
	}

	id, err := mnodeexecution.NewIDFromBytes(item.NodeExecutionId)
	if err != nil {
		return mnodeexecution.NodeExecution{}, err
	}

	nodeID, err := mnnode.NewIDFromBytes(item.NodeId)
	if err != nil {
		return mnodeexecution.NodeExecution{}, err
	}

	flowExecutionID, err := mnodeexecution.NewFlowExecutionIDFromBytes(item.FlowExecutionId)
	if err != nil {
		return mnodeexecution.NodeExecution{}, err
	}

	state, err := protoNodeStateToModel(item.State)
	if err != nil {
		return mnodeexecution.NodeExecution{}, err
	}

	var startTime, endTime *time.Time
	if item.StartTime != nil {
		startTime = &item.StartTime.AsTime()
	}
	if item.EndTime != nil {
		endTime = &item.EndTime.AsTime()
	}

	return mnodeexecution.NodeExecution{
		ID: id,
		NodeID: nodeID,
		FlowExecutionID: flowExecutionID,
		State: state,
		StartTime: startTime,
		EndTime: endTime,
	}, nil
}