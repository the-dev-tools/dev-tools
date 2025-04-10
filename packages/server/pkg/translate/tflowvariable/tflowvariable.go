package tflowvariable

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflowvariable"
	flowvariablev1 "the-dev-tools/spec/dist/buf/go/flowvariable/v1"
)

// ModelToRPC converts a FlowVariable model to its RPC representation
func ModelToRPC(v mflowvariable.FlowVariable) *flowvariablev1.FlowVariable {
	return &flowvariablev1.FlowVariable{
		VariableId:  v.ID.Bytes(),
		Name:        v.Name,
		Value:       v.Value,
		Enabled:     v.Enabled,
		Description: v.Description,
	}
}

// ModelToRPCListItem converts a FlowVariable model to a list item representation for RPC
func ModelToRPCListItem(v mflowvariable.FlowVariable) *flowvariablev1.FlowVariableListItem {
	return &flowvariablev1.FlowVariableListItem{
		VariableId:  v.ID.Bytes(),
		Name:        v.Name,
		Value:       v.Value,
		Enabled:     v.Enabled,
		Description: v.Description,
	}
}

// RPCToModel converts an RPC FlowVariable to its model representation
func RPCToModel(v *flowvariablev1.FlowVariable) (mflowvariable.FlowVariable, error) {
	id, err := idwrap.NewFromBytes(v.VariableId)
	if err != nil {
		return mflowvariable.FlowVariable{}, err
	}

	return mflowvariable.FlowVariable{
		ID:          id,
		Name:        v.Name,
		Value:       v.Value,
		Enabled:     v.Enabled,
		Description: v.Description,
	}, nil
}

// RPCToModelWithID creates a model with provided IDs from RPC representation
func RPCToModelWithID(variableID, flowID idwrap.IDWrap, v *flowvariablev1.FlowVariable) mflowvariable.FlowVariable {
	return mflowvariable.FlowVariable{
		ID:          variableID,
		FlowID:      flowID,
		Name:        v.Name,
		Value:       v.Value,
		Enabled:     v.Enabled,
		Description: v.Description,
	}
}

// CreateUpdateFromRPC creates a FlowVariableUpdate from RPC update request fields
func CreateUpdateFromRPC(id idwrap.IDWrap, name, value *string, enabled *bool, description *string) mflowvariable.FlowVariableUpdate {
	update := mflowvariable.FlowVariableUpdate{
		ID: id,
	}

	if name != nil {
		update.Name = name
	}

	if value != nil {
		update.Value = value
	}

	if enabled != nil {
		update.Enabled = enabled
	}

	if description != nil {
		update.Description = description
	}

	return update
}
