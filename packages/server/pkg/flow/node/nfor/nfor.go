package nfor

import (
	"context"
	"fmt"
	"strconv"
	"the-dev-tools/server/pkg/assertv2"
	"the-dev-tools/server/pkg/assertv2/leafs/leafmock"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"time"
)

// TODO: this is dupe should me refactored
const NodeVarKey = "var"

type NodeFor struct {
	FlowNodeID    idwrap.IDWrap
	Name          string
	IterCount     int64
	Timeout       time.Duration
	ConditionType mcondition.ComparisonKind
	Path          string
	Value         string
	ErrorHandling mnfor.ErrorHandling
}

func New(id idwrap.IDWrap, name string, iterCount int64, timeout time.Duration, errorHandling mnfor.ErrorHandling) *NodeFor {
	return &NodeFor{
		FlowNodeID:    id,
		Name:          name,
		IterCount:     iterCount,
		Timeout:       timeout,
		ErrorHandling: errorHandling,
	}
}

func (nr *NodeFor) GetID() idwrap.IDWrap {
	return nr.FlowNodeID
}

func (nr *NodeFor) SetID(id idwrap.IDWrap) {
	nr.FlowNodeID = id
}

func (n *NodeFor) GetName() string {
	return n.Name
}

func (nr *NodeFor) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	loopID := edge.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, edge.HandleLoop)
	nextID := edge.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, edge.HandleThen)

	a := map[string]any{
		NodeVarKey: req.VarMap,
	}

	rootLeaf := &leafmock.LeafMock{
		Leafs: a,
	}
	root := assertv2.NewAssertRoot(rootLeaf)
	assertSys := assertv2.NewAssertSystem(root)

	var loopError error
	var failedAtIteration int64 = -1

	for i := int64(0); i < nr.IterCount; i++ {
		// Write the iteration index to the node variables
		var err error
		if req.VariableTracker != nil {
			err = node.WriteNodeVarWithTracking(req, nr.Name, "index", i, req.VariableTracker)
		} else {
			err = node.WriteNodeVar(req, nr.Name, "index", i)
		}
		if err != nil {
			return node.FlowNodeResult{
				Err: err,
			}
		}

		// Store execution ID and iteration context for later update
		executionID := idwrap.NewNow()

		// Create iteration context for this execution
		var parentPath []int
		var parentNodes []idwrap.IDWrap
		if req.IterationContext != nil {
			parentPath = req.IterationContext.IterationPath
			parentNodes = req.IterationContext.ParentNodes
		}
		iterContext := &runner.IterationContext{
			IterationPath: append(parentPath, int(i)),
			ParentNodes:   append(parentNodes, nr.FlowNodeID),
		}

		// Create initial RUNNING record
		if req.LogPushFunc != nil {
			outputData := map[string]any{
				"index": i,
			}
			executionName := fmt.Sprintf("%s iteration %d", nr.Name, i+1)
			
			req.LogPushFunc(runner.FlowNodeStatus{
				ExecutionID: executionID, // Store this ID for update
				NodeID:      nr.FlowNodeID,
				Name:        executionName,
				State:       mnnode.NODE_STATE_RUNNING,
				OutputData:  outputData,
				IterationContext: iterContext,
			})
		}

		// Execute child nodes
		var iterationError error
		for _, nextNodeID := range loopID {

			var val any
			// parse int, float or bool if all fails make it string
			if v, err := strconv.ParseInt(nr.Value, 0, 64); err == nil {
				val = v
			} else if v, err := strconv.ParseFloat(nr.Value, 64); err == nil {
				val = v
			} else if v, err := strconv.ParseBool(nr.Value); err == nil {
				val = v
			} else {
				val = nr
			}

			if nr.Path != "" {
				ok, err := assertSys.AssertSimple(ctx, assertv2.AssertType(nr.ConditionType), nr.Path, val)
				if err != nil {
					return node.FlowNodeResult{
						Err: err,
					}
				}

				if !ok {
					break
				}
			}

			// Create iteration context for child nodes
			var parentPath []int
			var parentNodes []idwrap.IDWrap
			if req.IterationContext != nil {
				parentPath = req.IterationContext.IterationPath
				parentNodes = req.IterationContext.ParentNodes
			}
			childIterationContext := &runner.IterationContext{
				IterationPath:  append(parentPath, int(i)),
				ExecutionIndex: int(i), // Use iteration index to differentiate executions
				ParentNodes:    append(parentNodes, nr.FlowNodeID), // Add current loop node to parent chain
			}

			// Generate unique execution ID for child node
			childExecutionID := idwrap.NewNow()
			
			// Create new request with iteration context for child nodes
			childReq := *req // Copy the request
			childReq.IterationContext = childIterationContext
			childReq.ExecutionID = childExecutionID  // Set unique execution ID

			err := flowlocalrunner.RunNodeSync(ctx, nextNodeID, &childReq, req.LogPushFunc)
			if err != nil {
				iterationError = err
				break // Exit inner loop on error
			}
		}

		// Update iteration record based on result
		if req.LogPushFunc != nil {
			executionName := fmt.Sprintf("%s iteration %d", nr.Name, i+1)
			if iterationError != nil {
				// Update to FAILURE
				req.LogPushFunc(runner.FlowNodeStatus{
					ExecutionID: executionID, // Same ID = UPDATE
					NodeID:      nr.FlowNodeID,
					Name:        executionName,
					State:       mnnode.NODE_STATE_FAILURE,
					Error:       iterationError,
					IterationContext: iterContext,
				})
			} else {
				// Update to SUCCESS (iteration completed successfully)
				req.LogPushFunc(runner.FlowNodeStatus{
					ExecutionID: executionID, // Same ID = UPDATE
					NodeID:      nr.FlowNodeID,
					Name:        executionName,
					State:       mnnode.NODE_STATE_SUCCESS,
					OutputData:  map[string]any{"index": i, "completed": true},
					IterationContext: iterContext,
				})
			}
		}

		// Handle iteration error according to error policy
		if iterationError != nil {
			switch nr.ErrorHandling {
			case mnfor.ErrorHandling_ERROR_HANDLING_IGNORE:
				continue // Continue to next iteration
			case mnfor.ErrorHandling_ERROR_HANDLING_BREAK:
				failedAtIteration = i // Track where we stopped
				goto Exit // Stop loop but don't propagate error
			case mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED:
				loopError = iterationError
				failedAtIteration = i
				goto Exit // Fail entire flow
			}
		}
	}

Exit:
	// Create final summary record
	if loopError != nil {
		// Failure case: loop failed with error propagation
		if req.LogPushFunc != nil {
			outputData := map[string]any{
				"failedAtIteration": failedAtIteration,
				"totalIterations":   nr.IterCount,
			}
			executionName := "Error Summary"
			req.LogPushFunc(runner.FlowNodeStatus{
				ExecutionID: idwrap.NewNow(),
				NodeID:      nr.FlowNodeID,
				Name:        executionName,
				State:       mnnode.NODE_STATE_FAILURE,
				OutputData:  outputData,
			})
		}
		return node.FlowNodeResult{
			Err: loopError,
		}
	}
	// Note: Break case (failedAtIteration >= 0) doesn't create summary record per test expectations

	// Write final output with total iterations completed (for variable system)
	var err error
	if req.VariableTracker != nil {
		err = node.WriteNodeVarWithTracking(req, nr.Name, "totalIterations", nr.IterCount, req.VariableTracker)
	} else {
		err = node.WriteNodeVar(req, nr.Name, "totalIterations", nr.IterCount)
	}
	if err != nil {
		return node.FlowNodeResult{
			Err: err,
		}
	}

	// Success case: No final summary record needed - last iteration record shows completion
	return node.FlowNodeResult{
		NextNodeID: nextID,
		Err:        nil,
	}
}

func (nr *NodeFor) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	loopID := edge.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, edge.HandleLoop)
	nextID := edge.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, edge.HandleThen)

	a := map[string]any{
		NodeVarKey: req.VarMap,
	}

	rootLeaf := &leafmock.LeafMock{
		Leafs: a,
	}
	root := assertv2.NewAssertRoot(rootLeaf)
	assertSys := assertv2.NewAssertSystem(root)

	var loopError error
	var failedAtIteration int64 = -1

	for i := int64(0); i < nr.IterCount; i++ {
		// Write the iteration index to the node variables
		var err error
		if req.VariableTracker != nil {
			err = node.WriteNodeVarWithTracking(req, nr.Name, "index", i, req.VariableTracker)
		} else {
			err = node.WriteNodeVar(req, nr.Name, "index", i)
		}
		if err != nil {
			resultChan <- node.FlowNodeResult{
				Err: err,
			}
			return
		}

		// Store execution ID and iteration context for later update
		executionID := idwrap.NewNow()

		// Create iteration context for this execution
		var parentPath []int
		var parentNodes []idwrap.IDWrap
		if req.IterationContext != nil {
			parentPath = req.IterationContext.IterationPath
			parentNodes = req.IterationContext.ParentNodes
		}
		iterContext := &runner.IterationContext{
			IterationPath: append(parentPath, int(i)),
			ParentNodes:   append(parentNodes, nr.FlowNodeID),
		}

		// Create initial RUNNING record
		if req.LogPushFunc != nil {
			outputData := map[string]any{
				"index": i,
			}
			executionName := fmt.Sprintf("%s iteration %d", nr.Name, i+1)
			
			req.LogPushFunc(runner.FlowNodeStatus{
				ExecutionID: executionID, // Store this ID for update
				NodeID:      nr.FlowNodeID,
				Name:        executionName,
				State:       mnnode.NODE_STATE_RUNNING,
				OutputData:  outputData,
				IterationContext: iterContext,
			})
		}

		// Execute child nodes
		var iterationError error
		for _, nextNodeID := range loopID {

			var val any
			// parse int, float or bool if all fails make it string
			if v, err := strconv.ParseInt(nr.Value, 0, 64); err == nil {
				val = v
			} else if v, err := strconv.ParseFloat(nr.Value, 64); err == nil {
				val = v
			} else if v, err := strconv.ParseBool(nr.Value); err == nil {
				val = v
			} else {
				val = nr
			}

			if nr.Path != "" {
				ok, err := assertSys.AssertSimple(ctx, assertv2.AssertType(nr.ConditionType), nr.Path, val)
				if err != nil {
					resultChan <- node.FlowNodeResult{
						Err: err,
					}
					return
				}

				if !ok {
					break
				}
			}

			// Create iteration context for child nodes
			var parentPath []int
			var parentNodes []idwrap.IDWrap
			if req.IterationContext != nil {
				parentPath = req.IterationContext.IterationPath
				parentNodes = req.IterationContext.ParentNodes
			}
			childIterationContext := &runner.IterationContext{
				IterationPath:  append(parentPath, int(i)),
				ExecutionIndex: int(i), // Use iteration index to differentiate executions
				ParentNodes:    append(parentNodes, nr.FlowNodeID), // Add current loop node to parent chain
			}

			// Generate unique execution ID for child node
			childExecutionID := idwrap.NewNow()
			
			// Create new request with iteration context for child nodes
			childReq := *req // Copy the request
			childReq.IterationContext = childIterationContext
			childReq.ExecutionID = childExecutionID  // Set unique execution ID

			err := flowlocalrunner.RunNodeASync(ctx, nextNodeID, &childReq, req.LogPushFunc)
			if err != nil {
				iterationError = err
				break // Exit inner loop on error
			}
		}

		// Update iteration record based on result
		if req.LogPushFunc != nil {
			executionName := fmt.Sprintf("%s iteration %d", nr.Name, i+1)
			if iterationError != nil {
				// Update to FAILURE
				req.LogPushFunc(runner.FlowNodeStatus{
					ExecutionID: executionID, // Same ID = UPDATE
					NodeID:      nr.FlowNodeID,
					Name:        executionName,
					State:       mnnode.NODE_STATE_FAILURE,
					Error:       iterationError,
					IterationContext: iterContext,
				})
			} else {
				// Update to SUCCESS
				req.LogPushFunc(runner.FlowNodeStatus{
					ExecutionID: executionID, // Same ID = UPDATE
					NodeID:      nr.FlowNodeID,
					Name:        executionName,
					State:       mnnode.NODE_STATE_SUCCESS,
					OutputData:  map[string]interface{}{"index": i, "completed": true},
					IterationContext: iterContext,
				})
			}
		}

		// Handle iteration error according to error policy
		if iterationError != nil {
			switch nr.ErrorHandling {
			case mnfor.ErrorHandling_ERROR_HANDLING_IGNORE:
				continue // Continue to next iteration
			case mnfor.ErrorHandling_ERROR_HANDLING_BREAK:
				failedAtIteration = i // Track where we stopped
				goto Exit // Stop loop but don't propagate error
			case mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED:
				loopError = iterationError
				failedAtIteration = i
				goto Exit // Fail entire flow
			}
		}
	}

Exit:
	// Only create final summary record on failure
	if loopError != nil {
		if req.LogPushFunc != nil {
			outputData := map[string]interface{}{
				"failedAtIteration": failedAtIteration,
				"totalIterations":   nr.IterCount,
			}
			executionName := "Error Summary"
			req.LogPushFunc(runner.FlowNodeStatus{
				ExecutionID: idwrap.NewNow(),
				NodeID:      nr.FlowNodeID,
				Name:        executionName,
				State:       mnnode.NODE_STATE_FAILURE,
				OutputData:  outputData,
			})
		}
		resultChan <- node.FlowNodeResult{
			Err: loopError,
		}
		return
	}

	// Write final output with total iterations completed (for variable system)
	var err error
	if req.VariableTracker != nil {
		err = node.WriteNodeVarWithTracking(req, nr.Name, "totalIterations", nr.IterCount, req.VariableTracker)
	} else {
		err = node.WriteNodeVar(req, nr.Name, "totalIterations", nr.IterCount)
	}
	if err != nil {
		resultChan <- node.FlowNodeResult{
			Err: err,
		}
		return
	}

	// Success case: No final summary record needed - last iteration record shows completion
	resultChan <- node.FlowNodeResult{
		NextNodeID: nextID,
		Err:        nil,
	}
}
