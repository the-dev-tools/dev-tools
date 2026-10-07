package model

import (
	"time"
)

type IterationContextResult struct {
	IterationPath  []int    `json:"iteration_path,omitempty"`
	ExecutionIndex int      `json:"execution_index,omitempty"`
	ParentNodes    []string `json:"parent_nodes,omitempty"`
}

// NodeStateSkipped is the State of a cleanup step that did not run, because a
// step it reads or depends on produced no output. It has no mflow.NodeState
// counterpart: only the CLI's cleanup runner skips steps.
const NodeStateSkipped = "Skipped"

type NodeRunResult struct {
	NodeID           string                  `json:"node_id"`
	ExecutionID      string                  `json:"execution_id"`
	Name             string                  `json:"name"`
	State            string                  `json:"state"`
	Duration         time.Duration           `json:"duration"`
	Error            string                  `json:"error,omitempty"`
	IterationContext *IterationContextResult `json:"iteration_context,omitempty"`
	OutputData       any                     `json:"output_data,omitempty"`
	// Cleanup marks a step from the flow's cleanup: block.
	Cleanup bool `json:"cleanup,omitempty"`
	// SkipReason says why a skipped step did not run (State NodeStateSkipped).
	SkipReason string `json:"skip_reason,omitempty"`
}

type FlowRunResult struct {
	FlowID   string          `json:"flow_id"`
	FlowName string          `json:"flow_name"`
	Started  time.Time       `json:"started_at"`
	Duration time.Duration   `json:"duration"`
	Status   string          `json:"status"`
	Error    string          `json:"error,omitempty"`
	Nodes    []NodeRunResult `json:"nodes"`
}
