package model

import (
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/aicheck"
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
	// Checks are the step's AI checks (its expect: block), evaluated after the flow's steps
	// finished. Only the step's last execution carries them.
	Checks []aicheck.Result `json:"checks,omitempty"`
}

// JudgeStats is the judge's work for one flow.
type JudgeStats struct {
	Calls        int   `json:"calls"`
	Cached       int   `json:"cached"`
	InputTokens  int   `json:"input_tokens"`
	OutputTokens int   `json:"output_tokens"`
	ElapsedMS    int64 `json:"elapsed_ms"`
}

type FlowRunResult struct {
	FlowID   string          `json:"flow_id"`
	FlowName string          `json:"flow_name"`
	Started  time.Time       `json:"started_at"`
	Duration time.Duration   `json:"duration"`
	Status   string          `json:"status"`
	Error    string          `json:"error,omitempty"`
	Nodes    []NodeRunResult `json:"nodes"`
	// ChecksStatus is the worst AI check status: passed, skipped, warn or failed. Empty
	// when the flow has no expect: blocks.
	ChecksStatus string `json:"checks_status,omitempty"`
	// Judge is the judge's work for this flow, when a step has a judge check.
	Judge *JudgeStats `json:"judge,omitempty"`
	// CheckLines and JudgeLine are the console's check lines.
	CheckLines []string `json:"-"`
	JudgeLine  string   `json:"-"`
}
