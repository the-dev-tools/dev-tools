//nolint:revive // exported
package mflow

import "github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"

// SubFlowParam defines a single input parameter for a sub-flow trigger.
type SubFlowParam struct {
	Name         string `json:"name"`
	Type         string `json:"type"`          // string | number | boolean | json
	DefaultValue string `json:"default_value"` // JSON-encoded default
	Required     bool   `json:"required"`
}

// NodeSubFlowTrigger is the entry node for a sub-flow that receives parameters.
type NodeSubFlowTrigger struct {
	FlowNodeID idwrap.IDWrap
	Params     []SubFlowParam // Stored as JSON blob in DB
}

// SubFlowOutput defines a single output mapping from a sub-flow.
type SubFlowOutput struct {
	Name       string `json:"name"`
	Expression string `json:"expression"` // Expression evaluated against VarMap
}

// NodeSubFlowReturn is the terminal node that captures and returns output data.
type NodeSubFlowReturn struct {
	FlowNodeID idwrap.IDWrap
	Outputs    []SubFlowOutput // Stored as JSON blob in DB
}

// SubFlowInputMapping maps a parent expression to a sub-flow parameter.
type SubFlowInputMapping struct {
	ParamName  string `json:"param_name"`
	Expression string `json:"expression"` // Expression evaluated from parent VarMap
}

// NodeRunSubFlow orchestrates calling another flow from the parent flow.
type NodeRunSubFlow struct {
	FlowNodeID     idwrap.IDWrap
	TargetFlowID   *idwrap.IDWrap
	TargetFlowName string
	Inputs         []SubFlowInputMapping // Stored as JSON blob in DB
}
