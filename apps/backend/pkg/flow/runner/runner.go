package runner

import (
	"context"
	"errors"
	"fmt"
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/idwrap"
)

var (
	ErrFlowRunnerNotImplemented = errors.New("flowrunner not implemented")
	ErrNodeNotFound             = errors.New("next node not found")
)

type FlowRunner interface {
	Run(context.Context, chan FlowStatusResp) error
}

type FlowStatus int8

const (
	FlowStatusStarting FlowStatus = iota
	FlowStatusRunning
	FlowStatusSuccess
	FlowStatusFailed
	FlowStatusTimeout
)

func (f FlowStatus) String() string {
	return [...]string{"Starting", "Running", "Success", "Failed", "Timeout"}[f]
}

type FlowStatusResp struct {
	FlowStatus    FlowStatus
	NodeStatus    node.NodeStatus
	CurrentNodeID *idwrap.IDWrap
}

func NewFlowStatus(flowStatus FlowStatus, nodeStatus node.NodeStatus, currentNodeID *idwrap.IDWrap) FlowStatusResp {
	return FlowStatusResp{
		FlowStatus:    flowStatus,
		NodeStatus:    nodeStatus,
		CurrentNodeID: currentNodeID,
	}
}

func (f FlowStatusResp) Log() string {
	var flowStatus string
	if f.FlowStatus == FlowStatusRunning {
		flowStatus = fmt.Sprintf("FlowStatus: %v, NodeStatus: %v, CurrentNodeID: %v", f.FlowStatus.String(), f.NodeStatus.String(), f.CurrentNodeID.String())
	} else {
		flowStatus = fmt.Sprintf("FlowStatus: %v", f.FlowStatus.String())
	}
	return flowStatus
}

func (f FlowStatusResp) Done() bool {
	return f.FlowStatus == FlowStatusSuccess || f.FlowStatus == FlowStatusFailed || f.FlowStatus == FlowStatusTimeout
}
