package runner

import (
	"context"
	"errors"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mnnode"
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
	NodeStatus    mnnode.NodeState
	CurrentNodeID *idwrap.IDWrap
}

func NewFlowStatus(flowStatus FlowStatus, nodeStatus mnnode.NodeState, currentNodeID *idwrap.IDWrap) FlowStatusResp {
	return FlowStatusResp{
		FlowStatus:    flowStatus,
		NodeStatus:    nodeStatus,
		CurrentNodeID: currentNodeID,
	}
}

func (f FlowStatusResp) Done() bool {
	return f.FlowStatus == FlowStatusSuccess || f.FlowStatus == FlowStatusFailed || f.FlowStatus == FlowStatusTimeout
}
