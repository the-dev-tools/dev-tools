package flowlocalrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"the-dev-tools/backend/pkg/flow/edge"
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/flow/runner"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mnnode"
	"time"
)

type FlowLocalRunner struct {
	ID          idwrap.IDWrap
	FlowID      idwrap.IDWrap
	FlowNodeMap map[idwrap.IDWrap]node.FlowNode
	EdgesMap    edge.EdgesMap
	StartNodeID idwrap.IDWrap
	Timeout     time.Duration
}

func CreateFlowRunner(id, flowID, StartNodeID idwrap.IDWrap, FlowNodeMap map[idwrap.IDWrap]node.FlowNode, edgesMap edge.EdgesMap, timeout time.Duration) *FlowLocalRunner {
	return &FlowLocalRunner{
		ID:          id,
		FlowID:      flowID,
		StartNodeID: StartNodeID,
		FlowNodeMap: FlowNodeMap,
		EdgesMap:    edgesMap,
		Timeout:     timeout,
	}
}

func (r FlowLocalRunner) Run(ctx context.Context, flowNodeStatusChan chan runner.FlowNodeStatus, flowStatusChan chan runner.FlowStatus) error {
	nextNodeID := &r.StartNodeID
	var err error

	logWorkaround := func(status runner.FlowNodeStatus) {
		flowNodeStatusChan <- status
	}

	req := &node.FlowNodeRequest{
		VarMap:        map[string]interface{}{},
		ReadWriteLock: &sync.RWMutex{},
		NodeMap:       r.FlowNodeMap,
		EdgeSourceMap: r.EdgesMap,
		LogPushFunc:   node.LogPushFunc(logWorkaround),
		Timeout:       r.Timeout,
	}
	flowStatusChan <- runner.FlowStatusStarting
	currentNode, ok := r.FlowNodeMap[*nextNodeID]
	if !ok {
		flowStatusChan <- runner.FlowStatusFailed
		return fmt.Errorf("node not found: %v", nextNodeID)
	}
	if r.Timeout == 0 {
		_, err = RunNodeSync(ctx, currentNode, req, logWorkaround)
	} else {
		_, err = RunNodeASync(ctx, currentNode, req, logWorkaround)
	}

	if err != nil {
		flowStatusChan <- runner.FlowStatusFailed
		return err
	}
	fmt.Println("FlowLocalRunner.Run: flowStatusChan <- runner.FlowStatusSuccess")
	flowStatusChan <- runner.FlowStatusSuccess
	fmt.Println("FlowLocalRunner.Run: return nil")
	close(flowNodeStatusChan)
	close(flowStatusChan)
	return nil
}

// Common node processing result
type processResult struct {
	nextNodes []node.FlowNode
	err       error
}

// Helper function to handle a single node execution
func processNode(ctx context.Context, n node.FlowNode, req *node.FlowNodeRequest,
	statusLogFunc node.LogPushFunc,
) ([]idwrap.IDWrap, error) {
	id := n.GetID()
	status := runner.FlowNodeStatus{
		NodeID: id,
		State:  mnnode.NODE_STATE_RUNNING,
	}
	statusLogFunc(status)

	res := n.RunSync(ctx, req)

	if res.Err != nil {
		status.State = mnnode.NODE_STATE_FAILURE
		statusLogFunc(status)
		return nil, res.Err
	}

	status.State = mnnode.NODE_STATE_SUCCESS
	statusLogFunc(status)
	return res.NextNodeID, nil
}

// Helper function to process parallel nodes
func processParallelNodes(ctx context.Context, nodeIDs []idwrap.IDWrap,
	req *node.FlowNodeRequest, statusLogFunc node.LogPushFunc,
) ([]idwrap.IDWrap, error) {
	var wg sync.WaitGroup
	// Using buffered channel with enough capacity for all goroutines to send without blocking
	results := make(chan node.FlowNodeResult, len(nodeIDs))
	errCh := make(chan error, len(nodeIDs)) // Channel for potential errors

	for _, id := range nodeIDs {
		nextNode, ok := req.NodeMap[id]
		if !ok {
			return nil, fmt.Errorf("node not found: %v", id)
		}

		wg.Add(1)
		go func(n node.FlowNode, nodeID idwrap.IDWrap) {
			defer func() {
				if r := recover(); r != nil {
					errCh <- fmt.Errorf("panic in node %v: %v", nodeID, r)
				}
				wg.Done()
			}()

			status := runner.FlowNodeStatus{
				NodeID: id,
				State:  mnnode.NODE_STATE_RUNNING,
			}
			statusLogFunc(status)
			res := n.RunSync(ctx, req)

			if res.Err != nil {
				status.State = mnnode.NODE_STATE_FAILURE
				statusLogFunc(status)
				errCh <- res.Err
			} else {
				status.State = mnnode.NODE_STATE_SUCCESS
				statusLogFunc(status)
				results <- res
			}
		}(nextNode, id)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(results)
	close(errCh)

	// Check for errors first
	for err := range errCh {
		if err != nil {
			return nil, err
		}
	}

	// Collect all next node IDs
	mergedNextNodeIDs := make(map[idwrap.IDWrap]bool)
	for res := range results {
		for _, nextID := range res.NextNodeID {
			mergedNextNodeIDs[nextID] = true
		}
	}

	// Convert map keys to slice
	nextNodeIDs := make([]idwrap.IDWrap, 0, len(mergedNextNodeIDs))
	for id := range mergedNextNodeIDs {
		nextNodeIDs = append(nextNodeIDs, id)
	}

	return nextNodeIDs, nil
}

// RunNodeSync runs nodes in synchronous mode
func RunNodeSync(ctx context.Context, startNode node.FlowNode, req *node.FlowNodeRequest,
	statusLogFunc node.LogPushFunc,
) ([]idwrap.IDWrap, error) {
	queue := []node.FlowNode{startNode}
	processedNodes := make(map[idwrap.IDWrap]bool)
	var finalNextNodeIDs []idwrap.IDWrap

	for len(queue) > 0 {
		// Pop the first node
		currentNode := queue[0]
		queue = queue[1:]

		// Skip if already processed
		id := currentNode.GetID()
		if processedNodes[id] {
			continue
		}
		processedNodes[id] = true

		// Process the current node
		nextNodeIDs, err := processNode(ctx, currentNode, req, statusLogFunc)
		if err != nil {
			return nil, err
		}

		// Store the last set of next nodes for return value
		if len(nextNodeIDs) == 0 {
			continue
		} else if len(nextNodeIDs) == 1 {
			// Handle single next node case
			nextNode, ok := req.NodeMap[nextNodeIDs[0]]
			if !ok {
				return nil, fmt.Errorf("node not found: %v", nextNodeIDs[0])
			}
			queue = append(queue, nextNode)
		} else {
			// Handle multiple next nodes case
			parallelNextIDs, err := processParallelNodes(ctx, nextNodeIDs, req, statusLogFunc)
			if err != nil {
				return nil, err
			}

			// Add all next nodes to the queue
			for _, nextID := range parallelNextIDs {
				nextNode, ok := req.NodeMap[nextID]
				if !ok {
					return nil, fmt.Errorf("node not found: %v", nextID)
				}
				queue = append(queue, nextNode)
			}
		}

		// Update final result
		finalNextNodeIDs = nextNodeIDs
	}

	return finalNextNodeIDs, nil
}

// RunNodeASync runs nodes with timeout handling
func RunNodeASync(ctx context.Context, startNode node.FlowNode, req *node.FlowNodeRequest,
	statusLogFunc node.LogPushFunc,
) ([]idwrap.IDWrap, error) {
	queue := []node.FlowNode{startNode}
	processedNodes := make(map[idwrap.IDWrap]bool)
	var finalNextNodeIDs []idwrap.IDWrap

	for len(queue) > 0 {
		// Pop the first node
		currentNode := queue[0]
		queue = queue[1:]

		// Skip if already processed
		id := currentNode.GetID()
		if processedNodes[id] {
			continue
		}
		processedNodes[id] = true

		// Run with timeout
		status := runner.FlowNodeStatus{
			NodeID: id,
			State:  mnnode.NODE_STATE_RUNNING,
		}
		statusLogFunc(status)

		// Create a context that will be cancelled when we're done with this operation
		resultChan := make(chan node.FlowNodeResult, 1)

		// Wait for result with timeout
		timedCtx, timeoutCancel := context.WithTimeout(ctx, req.Timeout)
		defer timeoutCancel()
		var err error

		currentNode.RunAsync(timedCtx, req, resultChan)

		var result node.FlowNodeResult
		select {
		case resultTemp := <-resultChan:
			result = resultTemp
			if result.Err != nil {
				status.State = mnnode.NODE_STATE_FAILURE
				statusLogFunc(status)
				return nil, result.Err
			}
			// Success - continue processing
		case <-timedCtx.Done():
			err = fmt.Errorf("timeout")
			return nil, err
		}

		timeoutCancel()

		// Handle result
		outputData, ok := req.VarMap[node.NodeVarPrefix+id.String()]
		if ok {
			// TODO: change json.Marshal to faster json implementation
			outputDataBytes, err := json.Marshal(outputData)
			if err != nil {
				return nil, err
			}
			status.OutputData = outputDataBytes
		}

		status.State = mnnode.NODE_STATE_SUCCESS
		statusLogFunc(status)

		// Store the last set of next nodes for return value
		nextNodeIDs := result.NextNodeID
		finalNextNodeIDs = nextNodeIDs

		if len(nextNodeIDs) == 0 {
			continue
		} else if len(nextNodeIDs) == 1 {
			// Handle single next node case
			nextNode, ok := req.NodeMap[nextNodeIDs[0]]
			if !ok {
				return nil, fmt.Errorf("node not found: %v", nextNodeIDs[0])
			}
			queue = append(queue, nextNode)
		} else {
			// Handle multiple next nodes case
			parallelNextIDs, err := processParallelNodes(ctx, nextNodeIDs, req, statusLogFunc)
			if err != nil {
				return nil, err
			}

			// Add all next nodes to the queue
			for _, nextID := range parallelNextIDs {
				nextNode, ok := req.NodeMap[nextID]
				if !ok {
					return nil, fmt.Errorf("node not found: %v", nextID)
				}
				queue = append(queue, nextNode)
			}
		}
	}

	return finalNextNodeIDs, nil
}
