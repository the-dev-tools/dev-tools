package flowresult

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
)

// ExecutionStateTracker handles node execution lifecycle events:
// - Persists NodeExecution records (with dedup via execution cache)
// - Updates node and edge states in the database
// - Waits for response signals from ResponseDrain before publishing execution events
// - Publishes node state, edge state, and log events
type ExecutionStateTracker struct {
	flowID idwrap.IDWrap

	nodeKindMap      map[idwrap.IDWrap]mflow.NodeKind
	edgesBySource    map[idwrap.IDWrap][]mflow.Edge
	inverseNodeIDMap map[string]idwrap.IDWrap

	stateChan chan runner.FlowNodeStatus

	drain *ResponseDrain

	nodeExecSvc *sflow.NodeExecutionService
	nodeSvc     *sflow.NodeService
	edgeSvc     *sflow.EdgeService

	publisher EventPublisher
	logger    *slog.Logger

	wg sync.WaitGroup
}

// ExecutionStateTrackerOpts configures an ExecutionStateTracker.
type ExecutionStateTrackerOpts struct {
	FlowID  idwrap.IDWrap
	BufSize int

	NodeKindMap      map[idwrap.IDWrap]mflow.NodeKind
	EdgesBySource    map[idwrap.IDWrap][]mflow.Edge
	InverseNodeIDMap map[string]idwrap.IDWrap

	Drain *ResponseDrain

	NodeExecutionService *sflow.NodeExecutionService
	NodeService          *sflow.NodeService
	EdgeService          *sflow.EdgeService

	Publisher EventPublisher
	Logger    *slog.Logger
}

func newExecutionStateTracker(opts ExecutionStateTrackerOpts) *ExecutionStateTracker {
	return &ExecutionStateTracker{
		flowID:           opts.FlowID,
		nodeKindMap:      opts.NodeKindMap,
		edgesBySource:    opts.EdgesBySource,
		inverseNodeIDMap: opts.InverseNodeIDMap,
		stateChan:        make(chan runner.FlowNodeStatus, opts.BufSize),
		drain:            opts.Drain,
		nodeExecSvc:      opts.NodeExecutionService,
		nodeSvc:          opts.NodeService,
		edgeSvc:          opts.EdgeService,
		publisher:        opts.Publisher,
		logger:           opts.Logger,
	}
}

func (t *ExecutionStateTracker) start(ctx context.Context) {
	t.wg.Add(1)
	go t.run(ctx)
}

func (t *ExecutionStateTracker) wait() {
	t.wg.Wait()
}

func (t *ExecutionStateTracker) run(ctx context.Context) {
	defer t.wg.Done()

	// Execution cache: prevents duplicate NodeExecution creation for same iteration
	executionCache := make(map[string]idwrap.IDWrap)

	for status := range t.stateChan {
		t.processStatus(ctx, status, executionCache)
	}
}

func (t *ExecutionStateTracker) processStatus(ctx context.Context, status runner.FlowNodeStatus, executionCache map[string]idwrap.IDWrap) {
	// Find the original node ID if this is a versioned ID
	originalNodeID := status.NodeID
	if origID, ok := t.inverseNodeIDMap[status.NodeID.String()]; ok {
		originalNodeID = origID
	}

	// Check if this is a loop coordinator wrapper status
	nodeKind := t.nodeKindMap[status.NodeID]
	isLoopNode := nodeKind == mflow.NODE_KIND_FOR || nodeKind == mflow.NODE_KIND_FOR_EACH || nodeKind == mflow.NODE_KIND_WS_CONNECTION
	skipExecution := isLoopNode && !status.IterationEvent

	// Persist execution state (skip for loop node wrapper statuses)
	if !skipExecution {
		t.persistExecution(ctx, status, executionCache)
	}

	// Update node state in database
	if err := t.nodeSvc.UpdateNodeState(ctx, status.NodeID, status.State); err != nil {
		t.logger.Error("failed to update node state", "node_id", status.NodeID.String(), "error", err)
	}

	// Update edge states based on node execution state
	if status.State == mflow.NODE_STATE_SUCCESS || status.State == mflow.NODE_STATE_FAILURE {
		edgesFromNode := t.edgesBySource[status.NodeID]
		edgeState := mflow.NODE_STATE_SUCCESS
		if status.State == mflow.NODE_STATE_FAILURE {
			edgeState = mflow.NODE_STATE_FAILURE
		}
		for _, edge := range edgesFromNode {
			if err := t.edgeSvc.UpdateEdgeState(ctx, edge.ID, edgeState); err != nil {
				t.logger.Error("failed to update edge state", "edge_id", edge.ID.String(), "error", err)
			} else {
				updatedEdge := edge
				updatedEdge.State = edgeState
				t.publisher.PublishEdgeState(updatedEdge)
			}
		}
	}

	// Publish node state event (map versioned ID back to original for live sync)
	var info string
	if status.Error != nil {
		info = status.Error.Error()
	} else {
		iterIndex := -1
		if status.IterationEvent {
			iterIndex = status.IterationIndex
		} else if status.IterationContext != nil {
			iterIndex = status.IterationContext.ExecutionIndex
		}
		if iterIndex >= 0 {
			info = fmt.Sprintf("Iter: %d", iterIndex+1)
		}
	}
	t.publisher.PublishNodeState(t.flowID, originalNodeID, status.State, info)

	// Publish log event for terminal states
	if status.State != mflow.NODE_STATE_RUNNING {
		t.publisher.PublishLog(t.flowID, status)
	}
}

func (t *ExecutionStateTracker) persistExecution(ctx context.Context, status runner.FlowNodeStatus, executionCache map[string]idwrap.IDWrap) {
	execID := status.ExecutionID
	isNewExecution := false

	if isZeroID(execID) {
		// Construct cache key based on node and iteration context
		cacheKey := status.NodeID.String()
		if status.IterationContext != nil {
			cacheKey = fmt.Sprintf("%s:%v:%d", cacheKey, status.IterationContext.IterationPath, status.IterationContext.ExecutionIndex)
		} else if status.IterationIndex >= 0 {
			cacheKey = fmt.Sprintf("%s:%d", cacheKey, status.IterationIndex)
		}

		if cachedID, ok := executionCache[cacheKey]; ok {
			execID = cachedID
		} else {
			execID = idwrap.NewMonotonic()
			executionCache[cacheKey] = execID
			isNewExecution = true
		}
	}

	executionName := fmt.Sprintf("%s - %s", status.Name, time.Now().Format("2006-01-02 15:04"))

	model := mflow.NodeExecution{
		ID:     execID,
		NodeID: status.NodeID,
		Name:   executionName,
		State:  status.State,
	}

	// Set the appropriate response ID based on node kind
	nodeKindForAux := t.nodeKindMap[status.NodeID]
	if status.AuxiliaryID != nil {
		if nodeKindForAux == mflow.NODE_KIND_GRAPHQL {
			model.GraphQLResponseID = status.AuxiliaryID
		} else {
			model.ResponseID = status.AuxiliaryID
		}
	}

	if status.Error != nil {
		errStr := status.Error.Error()
		model.Error = &errStr
	}

	if status.InputData != nil {
		if b, err := json.Marshal(status.InputData); err == nil {
			_ = model.SetInputJSON(b)
		}
	}
	if status.OutputData != nil {
		if b, err := json.Marshal(status.OutputData); err == nil {
			_ = model.SetOutputJSON(b)
		}
	}

	// Set CompletedAt for terminal states
	if status.State == mflow.NODE_STATE_SUCCESS ||
		status.State == mflow.NODE_STATE_FAILURE ||
		status.State == mflow.NODE_STATE_CANCELED {
		now := time.Now().Unix()
		model.CompletedAt = &now
	}

	eventType := "insert"
	if !isNewExecution && (status.State == mflow.NODE_STATE_SUCCESS ||
		status.State == mflow.NODE_STATE_FAILURE ||
		status.State == mflow.NODE_STATE_CANCELED) {
		eventType = "update"
	}

	if err := t.nodeExecSvc.UpsertNodeExecution(ctx, model); err != nil {
		t.logger.Error("failed to persist node execution", "error", err)
	}

	// Wait for response to be published before publishing execution event
	if status.AuxiliaryID != nil {
		t.drain.WaitForResponse(status.AuxiliaryID.String())
	}

	t.publisher.PublishExecution(eventType, model, t.flowID)
}

func isZeroID(id idwrap.IDWrap) bool {
	return id == idwrap.IDWrap{}
}
