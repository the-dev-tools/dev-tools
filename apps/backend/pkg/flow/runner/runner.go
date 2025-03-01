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
	Run(context.Context, chan FlowNodeStatus, chan FlowStatus) error
}

type FlowStatus int8

const (
	FlowStatusStarting FlowStatus = iota
	FlowStatusRunning
	FlowStatusSuccess
	FlowStatusFailed
	FlowStatusTimeout
)

func FlowStatusString(f FlowStatus) string {
	return [...]string{"Starting", "Running", "Success", "Failed", "Timeout"}[f]
}

func IsFlowStatusDone(f FlowStatus) bool {
	return f == FlowStatusSuccess || f == FlowStatusFailed || f == FlowStatusTimeout
}

type FlowNodeStatus struct {
	NodeID     idwrap.IDWrap
	State      mnnode.NodeState
	OutputData []byte
}

func NewFlowNodeStatus(nodeID idwrap.IDWrap, status mnnode.NodeState, output []byte) FlowNodeStatus {
	return FlowNodeStatus{
		NodeID:     nodeID,
		State:      status,
		OutputData: output,
	}
}
