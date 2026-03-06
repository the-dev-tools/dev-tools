//nolint:revive // exported
package flowlocalrunner

import (
	"context"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// ExecutionMode controls how FlowLocalRunner schedules nodes.
type ExecutionMode int

const (
	ExecutionModeAuto ExecutionMode = iota
	ExecutionModeSingle
	ExecutionModeMulti
)

type FlowLocalRunner struct {
	ID          idwrap.IDWrap
	FlowID      idwrap.IDWrap
	FlowNodeMap map[idwrap.IDWrap]node.FlowNode
	Timeout     time.Duration

	graph          *runner.FlowGraph
	maxConcurrency int
	mode           ExecutionMode
	selectedMode   ExecutionMode

	enableDataTracking bool
	logger             *slog.Logger
}

var _ runner.FlowRunner = (*FlowLocalRunner)(nil)

func CreateFlowRunner(id, flowID idwrap.IDWrap, startNodeIDs []idwrap.IDWrap, flowNodeMap map[idwrap.IDWrap]node.FlowNode, edgesMap mflow.EdgesMap, timeout time.Duration, logger *slog.Logger) *FlowLocalRunner {
	return &FlowLocalRunner{
		ID:                 id,
		FlowID:             flowID,
		FlowNodeMap:        flowNodeMap,
		Timeout:            timeout,
		graph:              runner.NewFlowGraph(edgesMap, startNodeIDs),
		maxConcurrency:     goroutineCount,
		mode:               ExecutionModeAuto,
		selectedMode:       ExecutionModeMulti,
		enableDataTracking: true,
		logger:             logger,
	}
}

// SetExecutionMode overrides the default Auto mode for the next run.
func (r *FlowLocalRunner) SetExecutionMode(mode ExecutionMode) {
	if mode < ExecutionModeAuto || mode > ExecutionModeMulti {
		mode = ExecutionModeAuto
	}
	r.mode = mode
}

// SelectedMode reports the effective mode used during the last Run invocation.
func (r *FlowLocalRunner) SelectedMode() ExecutionMode {
	return r.selectedMode
}

// SetDataTrackingEnabled toggles variable tracking during execution.
func (r *FlowLocalRunner) SetDataTrackingEnabled(enabled bool) {
	r.enableDataTracking = enabled
}

func runNodes(ctx context.Context, startNodeID idwrap.IDWrap, req *node.FlowNodeRequest,
	statusLogFunc node.LogPushFunc, predecessorMap map[idwrap.IDWrap][]idwrap.IDWrap,
	mode ExecutionMode, timeout time.Duration, trackData bool,
	emitter *runner.StatusEmitter, maxConcurrency int,
) error {
	executor := NewLocalExecutor(trackData)
	switch mode {
	case ExecutionModeSingle:
		return runNodesSingle(ctx, startNodeID, req, statusLogFunc, predecessorMap, timeout, trackData, executor)
	default:
		return runNodesMultiEventDriven(ctx, startNodeID, req, statusLogFunc, predecessorMap, timeout, trackData, emitter, executor, maxConcurrency)
	}
}

// RunNodeSync retains the legacy behaviour for packages that directly invoke the runner.
func RunNodeSync(ctx context.Context, startNodeID idwrap.IDWrap, req *node.FlowNodeRequest,
	statusLogFunc node.LogPushFunc, predecessorMap map[idwrap.IDWrap][]idwrap.IDWrap,
) error {
	emitter := runner.NewStatusEmitter(func(s runner.FlowNodeStatus) { statusLogFunc(s) })
	return runNodes(ctx, startNodeID, req, statusLogFunc, predecessorMap, ExecutionModeMulti, 0, true, emitter, goroutineCount)
}

// RunNodeASync retains the legacy behaviour for packages that directly invoke the runner with timeouts.
func RunNodeASync(ctx context.Context, startNodeID idwrap.IDWrap, req *node.FlowNodeRequest,
	statusLogFunc node.LogPushFunc, predecessorMap map[idwrap.IDWrap][]idwrap.IDWrap,
) error {
	emitter := runner.NewStatusEmitter(func(s runner.FlowNodeStatus) { statusLogFunc(s) })
	return runNodes(ctx, startNodeID, req, statusLogFunc, predecessorMap, ExecutionModeMulti, req.Timeout, true, emitter, goroutineCount)
}

func (r *FlowLocalRunner) Run(ctx context.Context, flowNodeStatusChan chan runner.FlowNodeStatus, flowStatusChan chan runner.FlowStatus, baseVars map[string]any) error {
	return r.RunWithEvents(ctx, runner.LegacyFlowEventChannels(flowNodeStatusChan, flowStatusChan), baseVars)
}

func (r *FlowLocalRunner) RunWithEvents(ctx context.Context, channels runner.FlowEventChannels, baseVars map[string]any) error {
	// Cancel context before closing channels (LIFO order) so background
	// goroutines (e.g., WebSocket readers) get the stop signal first.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if channels.NodeStates != nil {
		defer close(channels.NodeStates)
	}
	if channels.NodeLogs != nil {
		defer close(channels.NodeLogs)
	}
	if channels.FlowStatus != nil {
		defer close(channels.FlowStatus)
	}

	// Clone convergence counts for per-execution mutable pending map
	pendingAtmoicMap := make(map[idwrap.IDWrap]uint32, len(r.graph.ConvergeCounts))
	for k, v := range r.graph.ConvergeCounts {
		pendingAtmoicMap[k] = v
	}

	if baseVars == nil {
		baseVars = make(map[string]any)
	}

	var emitFn func(runner.FlowNodeStatus)
	if channels.NodeStates != nil || channels.NodeLogs != nil {
		emitFn = runner.NewChannelEmitFunc(channels)
	} else {
		emitFn = func(runner.FlowNodeStatus) {}
	}
	emitter := runner.NewStatusEmitter(emitFn)
	statusFunc := node.LogPushFunc(emitFn)

	// Shared mutex for PendingAtmoicMap across concurrent entry chains
	pendingMu := &sync.Mutex{}

	req := &node.FlowNodeRequest{
		VarMap:           baseVars,
		ReadWriteLock:    &sync.RWMutex{},
		NodeMap:          r.FlowNodeMap,
		EdgeSourceMap:    r.graph.Edges,
		LogPushFunc:      statusFunc,
		Timeout:          r.Timeout,
		PendingAtmoicMap: pendingAtmoicMap,
		PendingMapMu:     pendingMu,
		Logger:           r.logger,
	}

	mode := r.mode
	if mode == ExecutionModeAuto {
		mode = selectExecutionMode(r.FlowNodeMap, r.graph.Edges)
	}
	r.selectedMode = mode

	if channels.FlowStatus != nil {
		channels.FlowStatus <- runner.FlowStatusStarting
	}

	var err error
	if len(r.graph.StartNodeIDs) == 1 {
		// Single entry — fast path, no errgroup overhead
		err = runNodes(ctx, r.graph.StartNodeIDs[0], req, statusFunc, r.graph.Predecessors, mode, r.Timeout, r.enableDataTracking, emitter, r.maxConcurrency)
	} else {
		// Multiple entries — run each chain concurrently
		eg, egCtx := errgroup.WithContext(ctx)
		for _, startID := range r.graph.StartNodeIDs {
			eg.Go(func() error {
				return runNodes(egCtx, startID, req, statusFunc, r.graph.Predecessors, mode, r.Timeout, r.enableDataTracking, emitter, r.maxConcurrency)
			})
		}
		err = eg.Wait()
	}

	if channels.FlowStatus != nil {
		if err != nil {
			channels.FlowStatus <- runner.FlowStatusFailed
		} else {
			channels.FlowStatus <- runner.FlowStatusSuccess
		}
	}
	return err
}

func MaxParallelism() int {
	maxProcs := runtime.GOMAXPROCS(0)
	numCPU := runtime.NumCPU()
	if maxProcs < numCPU {
		return maxProcs
	}
	return numCPU
}

var goroutineCount = MaxParallelism()

// SetGoroutineCountForTesting overrides the goroutine count for testing.
// Returns a cleanup function that restores the original value.
func SetGoroutineCountForTesting(n int) func() {
	old := goroutineCount
	goroutineCount = n
	return func() { goroutineCount = old }
}

// BuildPredecessorMap forwards to runner.BuildPredecessorMap.
// Kept for backward compatibility with node packages (nfor, nforeach, nwsconnection, nai).
func BuildPredecessorMap(edgesMap mflow.EdgesMap) map[idwrap.IDWrap][]idwrap.IDWrap {
	return runner.BuildPredecessorMap(edgesMap)
}
