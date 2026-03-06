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
	statusLogFunc node.LogPushFunc, predecessorMap map[idwrap.IDWrap][]idwrap.IDWrap,
	timeout time.Duration, trackData bool, executor *LocalExecutor,
) error {
	queue := []idwrap.IDWrap{startNodeID}

	for len(queue) > 0 {
		if ctx.Err() != nil {
			sendQueuedCancellationStatuses(queue, req, statusLogFunc, ctx.Err())
			return ctx.Err()
		}

		nodeID := queue[0]
		queue = queue[1:]

		currentNode, ok := req.NodeMap[nodeID]
		if !ok {
			return fmt.Errorf("node not found: %v", nodeID)
		}

		var inputData map[string]any
		if trackData {
			inputData = gatherSingleModeInputData(req, predecessorMap[nodeID])
		}

		executionID := idwrap.NewMonotonic()
		runningStatus := runner.FlowNodeStatus{
			ExecutionID:      executionID,
			NodeID:           nodeID,
			Name:             currentNode.GetName(),
			State:            mflow.NODE_STATE_RUNNING,
			IterationContext: req.IterationContext,
		}
		statusLogFunc(runningStatus)

		nodeReq := *req
		nodeReq.ExecutionID = executionID

		nodeCtx := ctx
		cancelNodeCtx := func() {}
		if timeout > 0 {
			nodeCtx, cancelNodeCtx = context.WithTimeout(ctx, timeout)
		}
		startTime := time.Now()

		outcome := executor.Execute(nodeCtx, currentNode, &nodeReq)
		trackedInput := outcome.TrackedInput
		trackedOutput := outcome.TrackedOutput
		result := outcome.Result

		nodeCtxErr := nodeCtx.Err()
		cancelNodeCtx()

		status := runner.FlowNodeStatus{
			ExecutionID:      executionID,
			NodeID:           nodeID,
			Name:             currentNode.GetName(),
			IterationContext: req.IterationContext,
			RunDuration:      time.Since(startTime),
			AuxiliaryID:      result.AuxiliaryID,
		}

		if trackData {
			if len(trackedInput) > 0 {
				status.InputData = node.DeepCopyValue(trackedInput)
			} else if len(inputData) > 0 {
				status.InputData = node.DeepCopyValue(inputData)
			}
		}

		if result.Err != nil {
			if runner.IsCancellationError(result.Err) {
				status.State = mflow.NODE_STATE_CANCELED
			} else {
				status.State = mflow.NODE_STATE_FAILURE
			}
			status.Error = result.Err
			if trackData {
				if len(trackedOutput) > 0 {
					status.OutputData = node.DeepCopyValue(trackedOutput)
				} else {
					status.OutputData = collectSingleModeOutput(&nodeReq, currentNode.GetName())
				}
			}
			status.OutputData = flattenNodeOutput(status.Name, status.OutputData)
			statusLogFunc(status)
			return result.Err
		}

		if nodeCtxErr != nil {
			status.State = mflow.NODE_STATE_CANCELED
			status.Error = nodeCtxErr
			if trackData {
				if len(trackedOutput) > 0 {
					status.OutputData = node.DeepCopyValue(trackedOutput)
				} else {
					status.OutputData = collectSingleModeOutput(&nodeReq, currentNode.GetName())
				}
			}
			status.OutputData = flattenNodeOutput(status.Name, status.OutputData)
			statusLogFunc(status)
			return nodeCtxErr
		}

		if !result.SkipFinalStatus {
			status.State = mflow.NODE_STATE_SUCCESS
			if trackData {
				if len(trackedOutput) > 0 {
					status.OutputData = node.DeepCopyValue(trackedOutput)
				} else {
					status.OutputData = collectSingleModeOutput(&nodeReq, currentNode.GetName())
				}
			}
			status.OutputData = flattenNodeOutput(status.Name, status.OutputData)
			statusLogFunc(status)
		}

		for _, nextID := range result.NextNodeID {
			if remaining, ok := req.PendingAtmoicMap[nextID]; ok && remaining > 1 {
				req.PendingAtmoicMap[nextID] = remaining - 1
				continue
			}
			queue = append(queue, nextID)
		}
	}

	return nil
}
