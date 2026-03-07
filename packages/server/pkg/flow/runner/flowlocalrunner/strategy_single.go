package flowlocalrunner

import (
	"context"
	"fmt"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func sendQueuedCancellationStatuses(queue []idwrap.IDWrap, req *node.FlowNodeRequest, statusLogFunc node.LogPushFunc, cancelErr error) {
	for _, nodeID := range queue {
		if nodeRef, ok := req.NodeMap[nodeID]; ok {
			statusLogFunc(runner.FlowNodeStatus{
				ExecutionID:      idwrap.NewMonotonic(),
				NodeID:           nodeID,
				Name:             nodeRef.GetName(),
				State:            mflow.NODE_STATE_CANCELED,
				Error:            cancelErr,
				IterationContext: req.IterationContext,
			})
		}
	}
}

func runNodesSingle(ctx context.Context, startNodeID idwrap.IDWrap, req *node.FlowNodeRequest,
	cfg RunConfig, executor *LocalExecutor, tracker *runner.ConvergenceTracker,
) error {
	queue := []idwrap.IDWrap{startNodeID}

	for len(queue) > 0 {
		if ctx.Err() != nil {
			sendQueuedCancellationStatuses(queue, req, cfg.StatusLogFunc, ctx.Err())
			return ctx.Err()
		}

		nodeID := queue[0]
		queue = queue[1:]

		currentNode, ok := req.NodeMap[nodeID]
		if !ok {
			return fmt.Errorf("node not found: %v", nodeID)
		}

		executionID := idwrap.NewMonotonic()
		cfg.StatusLogFunc(runner.FlowNodeStatus{
			ExecutionID:      executionID,
			NodeID:           nodeID,
			Name:             currentNode.GetName(),
			State:            mflow.NODE_STATE_RUNNING,
			IterationContext: req.IterationContext,
		})

		nodeReq := *req
		nodeReq.ExecutionID = executionID

		nodeCtx := ctx
		cancelNodeCtx := func() {}
		if cfg.Timeout > 0 {
			nodeCtx, cancelNodeCtx = context.WithTimeout(ctx, cfg.Timeout)
		}
		startTime := time.Now()

		outcome := executor.Execute(nodeCtx, currentNode, &nodeReq)
		nodeCtxErr := nodeCtx.Err()
		cancelNodeCtx()

		// Merge node error with context timeout
		nodeErr := outcome.Result.Err
		if nodeErr == nil && nodeCtxErr != nil {
			nodeErr = nodeCtxErr
		}

		base := runner.FlowNodeStatus{
			ExecutionID:      executionID,
			NodeID:           nodeID,
			Name:             currentNode.GetName(),
			IterationContext: req.IterationContext,
			RunDuration:      time.Since(startTime),
			AuxiliaryID:      outcome.Result.AuxiliaryID,
		}

		if nodeErr != nil {
			status := buildTerminalStatus(base, nodeErr, false, outcome.TrackedInput, outcome.TrackedOutput, req, cfg.TrackData)
			cfg.StatusLogFunc(status)
			return nodeErr
		}

		if !outcome.Result.SkipFinalStatus {
			status := buildTerminalStatus(base, nil, false, outcome.TrackedInput, outcome.TrackedOutput, req, cfg.TrackData)
			cfg.StatusLogFunc(status)
		}

		for _, nextID := range outcome.Result.NextNodeID {
			if !tracker.Arrive(nextID) {
				continue
			}
			queue = append(queue, nextID)
		}
	}

	return nil
}
