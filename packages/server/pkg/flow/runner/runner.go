//nolint:revive // exported
package runner

import (
	"context"
	"errors"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"time"
)

var (
	ErrFlowRunnerNotImplemented = errors.New("flowrunner not implemented")
	ErrNodeNotFound             = errors.New("next node not found")
)

type FlowRunner interface {
	RunWithEvents(context.Context, FlowEventChannels, map[string]any) error
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

func FlowStatusStringWithIcons(f FlowStatus) string {
	return [...]string{"🔄 Starting", "⏳ Running", "✅ Success", "❌ Failed", "⏰ Timeout"}[f]
}

func IsFlowStatusDone(f FlowStatus) bool {
	return f == FlowStatusSuccess || f == FlowStatusFailed || f == FlowStatusTimeout
}

type IterationContext struct {
	IterationPath  []int            `json:"iteration_path"`         // [1, 2, 3] for nested loops
	ExecutionIndex int              `json:"execution_index"`        // Current execution within current loop
	ParentNodes    []idwrap.IDWrap  `json:"parent_nodes,omitempty"` // Parent loop node IDs for hierarchical naming
	Labels         []IterationLabel `json:"labels,omitempty"`
}

// IterationLabel captures a single segment of a loop execution chain.
type IterationLabel struct {
	NodeID    idwrap.IDWrap `json:"node_id"`
	Name      string        `json:"name"`
	Iteration int           `json:"iteration"`
}

type FlowNodeStatus struct {
	ExecutionID      idwrap.IDWrap
	NodeID           idwrap.IDWrap
	Name             string
	State            mflow.NodeState
	OutputData       any
	InputData        any // Data that was read by this node during execution
	RunDuration      time.Duration
	Error            error
	IterationContext *IterationContext `json:"iteration_context,omitempty"`
	IterationEvent   bool              `json:"iteration_event,omitempty"`
	IterationIndex   int               `json:"iteration_index,omitempty"`
	LoopNodeID       idwrap.IDWrap     `json:"loop_node_id,omitempty"`
	AuxiliaryID      *idwrap.IDWrap
}

func NewFlowNodeStatus(nodeID idwrap.IDWrap, status mflow.NodeState, output []byte) FlowNodeStatus {
	return FlowNodeStatus{
		NodeID:     nodeID,
		State:      status,
		OutputData: output,
		Error:      nil,
	}
}

type FlowNodeEventTarget uint8

const (
	FlowNodeEventTargetState FlowNodeEventTarget = 1 << iota
	FlowNodeEventTargetLog
)

func (t FlowNodeEventTarget) includes(target FlowNodeEventTarget) bool {
	return t&target != 0
}

type FlowNodeLogPayload struct {
	ExecutionID      idwrap.IDWrap
	NodeID           idwrap.IDWrap
	Name             string
	State            mflow.NodeState
	Error            error
	OutputData       any
	RunDuration      time.Duration
	IterationContext *IterationContext
	IterationEvent   bool
	IterationIndex   int
	LoopNodeID       idwrap.IDWrap
}

type FlowNodeEvent struct {
	Status     FlowNodeStatus
	Targets    FlowNodeEventTarget
	LogPayload *FlowNodeLogPayload
}

func (e FlowNodeEvent) ShouldSend(target FlowNodeEventTarget) bool {
	return e.Targets.includes(target)
}

type FlowEventChannels struct {
	NodeStates chan FlowNodeStatus
	NodeLogs   chan FlowNodeLogPayload
	FlowStatus chan FlowStatus
}

func (c FlowEventChannels) HasLogChannel() bool {
	return c.NodeLogs != nil
}

func LegacyFlowEventChannels(nodeStates chan FlowNodeStatus, flowStatus chan FlowStatus) FlowEventChannels {
	return FlowEventChannels{
		NodeStates: nodeStates,
		FlowStatus: flowStatus,
	}
}
