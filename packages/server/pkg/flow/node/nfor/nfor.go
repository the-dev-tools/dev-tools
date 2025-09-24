package nfor

import (
	"context"
	"errors"
	"fmt"
	"the-dev-tools/server/pkg/expression"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/varsystem"
	"time"
)

// TODO: this is dupe should me refactored
const NodeVarKey = "var"

type NodeFor struct {
	FlowNodeID    idwrap.IDWrap
	Name          string
	IterCount     int64
	Timeout       time.Duration
	Condition     mcondition.Condition
	ErrorHandling mnfor.ErrorHandling
}

// NewWithCondition creates a NodeFor with condition data for break logic
func NewWithCondition(id idwrap.IDWrap, name string, iterCount int64, timeout time.Duration, errorHandling mnfor.ErrorHandling, condition mcondition.Condition) *NodeFor {
	return &NodeFor{
		FlowNodeID:    id,
		Name:          name,
		IterCount:     iterCount,
		Timeout:       timeout,
		ErrorHandling: errorHandling,
		Condition:     condition,
	}
}

// New creates a NodeFor without condition data (for backward compatibility)
func New(id idwrap.IDWrap, name string, iterCount int64, timeout time.Duration, errorHandling mnfor.ErrorHandling) *NodeFor {
	return &NodeFor{
		FlowNodeID:    id,
		Name:          name,
		IterCount:     iterCount,
		Timeout:       timeout,
		ErrorHandling: errorHandling,
		Condition:     mcondition.Condition{}, // Empty condition
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

// checkBreakCondition evaluates the break condition and returns (shouldBreak, error)
func (nr *NodeFor) checkBreakCondition(ctx context.Context, req *node.FlowNodeRequest) (bool, error) {
	if nr.Condition.Comparisons.Expression == "" {
		return false, nil // No condition, don't break
	}

	// Create a deep copy of VarMap to prevent concurrent access issues
	varMapCopy := node.DeepCopyVarMap(req)
	exprEnv := expression.NewEnv(varMapCopy)

	// Normalize the condition expression
	conditionExpr := nr.Condition.Comparisons.Expression
	varMap := varsystem.NewVarMapFromAnyMap(varMapCopy)
	normalizedExpression, err := expression.NormalizeExpression(ctx, conditionExpr, varMap)
	if err != nil {
		return false, fmt.Errorf("failed to normalize break condition '%s': %w", conditionExpr, err)
	}

	// Evaluate the condition expression
	var shouldBreak bool
	if req.VariableTracker != nil {
		shouldBreak, err = expression.ExpressionEvaluteAsBoolWithTracking(ctx, exprEnv, normalizedExpression, req.VariableTracker)
	} else {
		shouldBreak, err = expression.ExpressionEvaluteAsBool(ctx, exprEnv, normalizedExpression)
	}
	if err != nil {
		return false, fmt.Errorf("failed to evaluate break condition '%s': %w", normalizedExpression, err)
	}

	return shouldBreak, nil
}

func (nr *NodeFor) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	loopID := edge.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, edge.HandleLoop)
	nextID := edge.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, edge.HandleThen)
	// Track if we had any iteration errors to determine if we need final status
	predecessorMap := flowlocalrunner.BuildPredecessorMap(req.EdgeSourceMap)

	// Note: assertSys not needed for simple index comparison

	var loopError error

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

		// Check break condition AFTER setting index variable, BEFORE executing iteration
		shouldBreak, err := nr.checkBreakCondition(ctx, req)
		if err != nil {
			return node.FlowNodeResult{
				Err: err,
			}
		}
		if shouldBreak {
			// Break condition met - exit loop
			goto Exit
		}

		// Store execution ID and iteration context for later update
		executionID := idwrap.NewNow()

		// Create iteration context for this execution
		var parentPath []int
		var parentNodes []idwrap.IDWrap
		var parentLabels []runner.IterationLabel
		if req.IterationContext != nil {
			parentPath = req.IterationContext.IterationPath
			parentNodes = req.IterationContext.ParentNodes
			parentLabels = node.CloneIterationLabels(req.IterationContext.Labels)
		}
		labels := append(parentLabels, runner.IterationLabel{
			NodeID:    nr.FlowNodeID,
			Name:      nr.Name,
			Iteration: int(i) + 1,
		})
		iterContext := &runner.IterationContext{
			IterationPath: append(parentPath, int(i)),
			ParentNodes:   append(parentNodes, nr.FlowNodeID),
			Labels:        labels,
		}

		// Create initial RUNNING record
		if req.LogPushFunc != nil {
			outputData := map[string]any{
				"index": i,
			}
			executionName := fmt.Sprintf("%s Iteration %d", nr.Name, i+1)

			req.LogPushFunc(runner.FlowNodeStatus{
				ExecutionID:      executionID, // Store this ID for update
				NodeID:           nr.FlowNodeID,
				Name:             executionName,
				State:            mnnode.NODE_STATE_RUNNING,
				OutputData:       outputData,
				IterationContext: iterContext,
			})
		}

		// Execute child nodes
		var iterationError error
		for _, nextNodeID := range loopID {

			// Create iteration context for child nodes
			childIterationContext := &runner.IterationContext{
				IterationPath:  append([]int(nil), iterContext.IterationPath...),
				ExecutionIndex: int(i),
				ParentNodes:    append([]idwrap.IDWrap(nil), iterContext.ParentNodes...),
				Labels:         node.CloneIterationLabels(iterContext.Labels),
			}

			// Generate unique execution ID for child node
			childExecutionID := idwrap.NewNow()

			// Create new request with iteration context for child nodes
			childReq := *req // Copy the request
			childReq.IterationContext = childIterationContext
			childReq.ExecutionID = childExecutionID // Set unique execution ID

			err := flowlocalrunner.RunNodeSync(ctx, nextNodeID, &childReq, req.LogPushFunc, predecessorMap)
			if err != nil {
				iterationError = err
				break // Exit inner loop on error
			}
		}

		// Update iteration record based on result
		if req.LogPushFunc != nil {
			executionName := fmt.Sprintf("%s Iteration %d", nr.Name, i+1)
			if iterationError == nil {
				// Update to SUCCESS (iteration completed successfully)
				req.LogPushFunc(runner.FlowNodeStatus{
					ExecutionID:      executionID, // Same ID = UPDATE
					NodeID:           nr.FlowNodeID,
					Name:             executionName,
					State:            mnnode.NODE_STATE_SUCCESS,
					OutputData:       map[string]any{"index": i, "completed": true},
					IterationContext: iterContext,
				})
			}
			// Note: Do not emit iteration-level FAILURE for loop parent.
			// Cancellation state is emitted after the loop finishes processing.
		}

		// Handle iteration error according to error policy
		if iterationError != nil {
			switch nr.ErrorHandling {
			case mnfor.ErrorHandling_ERROR_HANDLING_IGNORE:
				continue // Continue to next iteration
			case mnfor.ErrorHandling_ERROR_HANDLING_BREAK:
				goto Exit // Stop loop but don't propagate error
			case mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED:
				loopError = iterationError
				goto Exit // Fail entire flow
			}
		}
	}

Exit:
	if loopError != nil {
		if !runner.IsCancellationError(loopError) {
			loopError = errors.Join(runner.ErrFlowCanceledByThrow, loopError)
		}
		return node.FlowNodeResult{
			Err: loopError,
		}
	}

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
		NextNodeID:      nextID,
		Err:             nil,
		SkipFinalStatus: false,
	}
}

func (nr *NodeFor) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	loopID := edge.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, edge.HandleLoop)
	nextID := edge.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, edge.HandleThen)
	// Track if we had any iteration errors to determine if we need final status
	predecessorMap := flowlocalrunner.BuildPredecessorMap(req.EdgeSourceMap)

	// Note: assertSys not needed for simple index comparison

	var loopError error

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

		// Check break condition AFTER setting index variable, BEFORE executing iteration
		shouldBreak, err := nr.checkBreakCondition(ctx, req)
		if err != nil {
			resultChan <- node.FlowNodeResult{
				Err: err,
			}
			return
		}
		if shouldBreak {
			// Break condition met - exit loop
			goto Exit
		}

		// Store execution ID and iteration context for later update
		executionID := idwrap.NewNow()

		// Create iteration context for this execution
		var parentPath []int
		var parentNodes []idwrap.IDWrap
		var parentLabels []runner.IterationLabel
		if req.IterationContext != nil {
			parentPath = req.IterationContext.IterationPath
			parentNodes = req.IterationContext.ParentNodes
			parentLabels = node.CloneIterationLabels(req.IterationContext.Labels)
		}
		labels := append(parentLabels, runner.IterationLabel{
			NodeID:    nr.FlowNodeID,
			Name:      nr.Name,
			Iteration: int(i) + 1,
		})
		iterContext := &runner.IterationContext{
			IterationPath: append(parentPath, int(i)),
			ParentNodes:   append(parentNodes, nr.FlowNodeID),
			Labels:        labels,
		}

		// Create initial RUNNING record
		if req.LogPushFunc != nil {
			outputData := map[string]any{
				"index": i,
			}
			executionName := fmt.Sprintf("%s iteration %d", nr.Name, i+1)

			req.LogPushFunc(runner.FlowNodeStatus{
				ExecutionID:      executionID, // Store this ID for update
				NodeID:           nr.FlowNodeID,
				Name:             executionName,
				State:            mnnode.NODE_STATE_RUNNING,
				OutputData:       outputData,
				IterationContext: iterContext,
			})
		}

		// Execute child nodes
		var iterationError error
		for _, nextNodeID := range loopID {

			// Create iteration context for child nodes
			childIterationContext := &runner.IterationContext{
				IterationPath:  append([]int(nil), iterContext.IterationPath...),
				ExecutionIndex: int(i),
				ParentNodes:    append([]idwrap.IDWrap(nil), iterContext.ParentNodes...),
				Labels:         node.CloneIterationLabels(iterContext.Labels),
			}

			// Generate unique execution ID for child node
			childExecutionID := idwrap.NewNow()

			// Create new request with iteration context for child nodes
			childReq := *req // Copy the request
			childReq.IterationContext = childIterationContext
			childReq.ExecutionID = childExecutionID // Set unique execution ID

			err := flowlocalrunner.RunNodeASync(ctx, nextNodeID, &childReq, req.LogPushFunc, predecessorMap)
			if err != nil {
				iterationError = err
				break // Exit inner loop on error
			}
		}

		// Update iteration record based on result
		if req.LogPushFunc != nil {
			executionName := fmt.Sprintf("%s iteration %d", nr.Name, i+1)
			if iterationError == nil {
				// Update to SUCCESS (iteration completed successfully)
				req.LogPushFunc(runner.FlowNodeStatus{
					ExecutionID:      executionID, // Same ID = UPDATE
					NodeID:           nr.FlowNodeID,
					Name:             executionName,
					State:            mnnode.NODE_STATE_SUCCESS,
					OutputData:       map[string]interface{}{"index": i, "completed": true},
					IterationContext: iterContext,
				})
			}
			// Note: Do not emit iteration-level FAILURE for loop parent in FOR node.
		}

		// Handle iteration error according to error policy
		if iterationError != nil {
			switch nr.ErrorHandling {
			case mnfor.ErrorHandling_ERROR_HANDLING_IGNORE:
				continue // Continue to next iteration
			case mnfor.ErrorHandling_ERROR_HANDLING_BREAK:
				goto Exit // Stop loop but don't propagate error
			case mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED:
				loopError = iterationError
				goto Exit // Fail entire flow
			}
		}
	}

Exit:
	if loopError != nil {
		if !runner.IsCancellationError(loopError) {
			loopError = errors.Join(runner.ErrFlowCanceledByThrow, loopError)
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
	// Only skip final status if loop completed all iterations without any errors
	// If we had errors (IGNORE/BREAK), we need final status to show overall success
	resultChan <- node.FlowNodeResult{
		NextNodeID:      nextID,
		Err:             nil,
		SkipFinalStatus: false,
	}
}
