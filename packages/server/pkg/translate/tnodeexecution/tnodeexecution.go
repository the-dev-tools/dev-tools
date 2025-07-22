package tnodeexecution

import (
	"encoding/json"
	"time"
	nodeexecutionv1 "the-dev-tools/spec/dist/buf/go/flow/node/execution/v1"
	nodev1 "the-dev-tools/spec/dist/buf/go/flow/node/v1"
	"the-dev-tools/server/pkg/model/mnodeexecution"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func SerializeNodeExecutionModelToRPC(ne *mnodeexecution.NodeExecution) (*nodeexecutionv1.NodeExecution, error) {
	rpc := &nodeexecutionv1.NodeExecution{
		NodeExecutionId: ne.ID.Bytes(),
		NodeId:          ne.NodeID.Bytes(),
		State:           nodev1.NodeState(ne.State),
	}
	
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
	
	// Convert OutputKind
	if ne.OutputKind != nil {
		outputKind := nodeexecutionv1.OutputKind(*ne.OutputKind)
		rpc.OutputKind = &outputKind
	}
	
	// Convert CompletedAt timestamp
	if ne.CompletedAt != nil {
		rpc.CompletedAt = timestamppb.New(time.UnixMilli(*ne.CompletedAt))
	}
	
	return rpc, nil
}

func SerializeNodeExecutionModelToRPCListItem(ne *mnodeexecution.NodeExecution) (*nodeexecutionv1.NodeExecutionListItem, error) {
	rpc := &nodeexecutionv1.NodeExecutionListItem{
		NodeExecutionId: ne.ID.Bytes(),
		NodeId:          ne.NodeID.Bytes(),
		State:           nodev1.NodeState(ne.State),
	}
	
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
	
	// Convert OutputKind
	if ne.OutputKind != nil {
		outputKind := nodeexecutionv1.OutputKind(*ne.OutputKind)
		rpc.OutputKind = &outputKind
	}
	
	// Convert CompletedAt timestamp
	if ne.CompletedAt != nil {
		rpc.CompletedAt = timestamppb.New(time.UnixMilli(*ne.CompletedAt))
	}
	
	return rpc, nil
}

func SerializeNodeExecutionModelToRPCGetResponse(ne *mnodeexecution.NodeExecution) (*nodeexecutionv1.NodeExecutionGetResponse, error) {
	rpc := &nodeexecutionv1.NodeExecutionGetResponse{
		NodeExecutionId: ne.ID.Bytes(),
		NodeId:          ne.NodeID.Bytes(),
		State:           nodev1.NodeState(ne.State),
	}
	
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
	
	// Convert OutputKind
	if ne.OutputKind != nil {
		outputKind := nodeexecutionv1.OutputKind(*ne.OutputKind)
		rpc.OutputKind = &outputKind
	}
	
	// Convert CompletedAt timestamp
	if ne.CompletedAt != nil {
		rpc.CompletedAt = timestamppb.New(time.UnixMilli(*ne.CompletedAt))
	}
	
	return rpc, nil
}