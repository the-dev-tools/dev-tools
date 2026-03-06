package flowlocalrunner

import (
	"context"
	"sync"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/tracking"
)

// ExecutionOutcome is the raw result from executing a single node,
// including optional tracked variable data.
type ExecutionOutcome struct {
	Result        node.FlowNodeResult
	TrackedInput  map[string]any
	TrackedOutput map[string]any
}

// LocalExecutor runs nodes in the current process with optional variable tracking.
// It owns the tracker pool, replacing the previous global variable.
//
// For a remote runner, a RemoteExecutor would serialize the request and dispatch
// to a worker instead of calling RunSync directly.
type LocalExecutor struct {
	trackerPool *sync.Pool
	trackData   bool
}

// NewLocalExecutor creates an executor with the given data tracking setting.
func NewLocalExecutor(trackData bool) *LocalExecutor {
	return &LocalExecutor{
		trackerPool: &sync.Pool{New: func() any { return tracking.NewVariableTracker() }},
		trackData:   trackData,
	}
}

// Execute runs a node with optional variable tracking, returning the result
// and any tracked input/output data.
func (e *LocalExecutor) Execute(ctx context.Context, n node.FlowNode, req *node.FlowNodeRequest) ExecutionOutcome {
	var tracker *tracking.VariableTracker
	if e.trackData {
		tracker = e.trackerPool.Get().(*tracking.VariableTracker)
		tracker.Reset()
		req.VariableTracker = tracker
	} else {
		req.VariableTracker = nil
	}

	result := n.RunSync(ctx, req)

	var trackedInput, trackedOutput map[string]any
	if tracker != nil {
		trackedOutput = tracker.GetWrittenVarsAsTree()
		reads := tracker.GetReadVarsAsTree()
		if len(reads) > 0 {
			trackedInput = reads
		}
		tracker.Reset()
		e.trackerPool.Put(tracker)
	}

	return ExecutionOutcome{
		Result:        result,
		TrackedInput:  trackedInput,
		TrackedOutput: trackedOutput,
	}
}
