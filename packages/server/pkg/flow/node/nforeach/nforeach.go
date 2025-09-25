package nforeach

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"sync"
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

type NodeForEach struct {
	FlowNodeID    idwrap.IDWrap
	Name          string
	IterPath      string
	Timeout       time.Duration
	Condition     mcondition.Condition
	ErrorHandling mnfor.ErrorHandling
}

func New(id idwrap.IDWrap, name string, iterPath string, timeout time.Duration,
	Condition mcondition.Condition, ErrorHandling mnfor.ErrorHandling,
) *NodeForEach {
	return &NodeForEach{
		FlowNodeID:    id,
		Name:          name,
		IterPath:      iterPath,
		Timeout:       timeout,
		Condition:     Condition,
		ErrorHandling: ErrorHandling,
	}
}

func (nr *NodeForEach) GetID() idwrap.IDWrap {
	return nr.FlowNodeID
}

func (nr *NodeForEach) SetID(id idwrap.IDWrap) {
	nr.FlowNodeID = id
}

func (n *NodeForEach) GetName() string {
	return n.Name
}

func (nr *NodeForEach) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	loopID := edge.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, edge.HandleLoop)
	nextID := edge.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, edge.HandleThen)
	predecessorMap := flowlocalrunner.BuildPredecessorMap(req.EdgeSourceMap)

	// Create a deep copy of VarMap to prevent concurrent access issues
	varMapCopy := node.DeepCopyVarMap(req)

	varMap := varsystem.NewVarMapFromAnyMap(varMapCopy)
	normalizedExpressionIterPath, err := expression.NormalizeExpression(ctx, nr.IterPath, varMap)
	if err != nil {
		return node.FlowNodeResult{
			Err: err,
		}
	}

	exprEnv := expression.NewEnv(varMapCopy)

	// Use tracking version if tracker is available
	var result any
	if req.VariableTracker != nil {
		result, err = expression.ExpressionEvaluateAsIterWithTracking(ctx, exprEnv, normalizedExpressionIterPath, req.VariableTracker)
	} else {
		result, err = expression.ExpressionEvaluateAsIter(ctx, exprEnv, normalizedExpressionIterPath)
	}
	if err != nil {
		return node.FlowNodeResult{
			Err: err,
		}
	}

	breakExpr := nr.Condition.Comparisons.Expression
	normalizedExpressionBreak, err := expression.NormalizeExpression(ctx, breakExpr, varMap)
	if err != nil {
		return node.FlowNodeResult{
			Err: err,
		}
	}

	processNode := func(iterationIndex int) node.FlowNodeResult {
		for _, nextNodeID := range loopID {
			if breakExpr != "" {
				// Use tracking version if tracker is available
				var ok bool
				var err error
				if req.VariableTracker != nil {
					ok, err = expression.ExpressionEvaluteAsBoolWithTracking(ctx, exprEnv, normalizedExpressionBreak, req.VariableTracker)
				} else {
					ok, err = expression.ExpressionEvaluteAsBool(ctx, exprEnv, normalizedExpressionBreak)
				}
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
			var parentLabels []runner.IterationLabel
			if req.IterationContext != nil {
				parentPath = req.IterationContext.IterationPath
				parentNodes = req.IterationContext.ParentNodes
				parentLabels = node.CloneIterationLabels(req.IterationContext.Labels)
			}
			labels := append(parentLabels, runner.IterationLabel{
				NodeID:    nr.FlowNodeID,
				Name:      nr.Name,
				Iteration: iterationIndex + 1,
			})
			childIterationContext := &runner.IterationContext{
				IterationPath:  append(parentPath, iterationIndex),
				ExecutionIndex: iterationIndex,                     // Use iteration index to differentiate executions
				ParentNodes:    append(parentNodes, nr.FlowNodeID), // Add current loop node to parent chain
				Labels:         labels,
			}

			// Generate unique execution ID for child node
			childExecutionID := idwrap.NewNow()

			// Create new request with iteration context for child nodes
			childReq := *req // Copy the request
			childReq.IterationContext = childIterationContext
			childReq.ExecutionID = childExecutionID // Set unique execution ID

			err := flowlocalrunner.RunNodeSync(ctx, nextNodeID, &childReq, req.LogPushFunc, predecessorMap)
			if err != nil {
				return node.FlowNodeResult{
					Err: err,
				}
			}
		}
		return node.FlowNodeResult{}
	}

	switch seq := result.(type) {
	case iter.Seq[any]:
		// Handle slice/array sequence
		itemIndex := 0
		totalItems := 0
		var loopError error

		for item := range seq {
			// Write the item and key (index) to the node variables
			var err error
			if req.VariableTracker != nil {
				err = node.WriteNodeVarWithTracking(req, nr.Name, "item", item, req.VariableTracker)
			} else {
				err = node.WriteNodeVar(req, nr.Name, "item", item)
			}
			if err != nil {
				return node.FlowNodeResult{
					Err: err,
				}
			}

			if req.VariableTracker != nil {
				err = node.WriteNodeVarWithTracking(req, nr.Name, "key", itemIndex, req.VariableTracker)
			} else {
				err = node.WriteNodeVar(req, nr.Name, "key", itemIndex)
			}
			if err != nil {
				return node.FlowNodeResult{
					Err: err,
				}
			}

			// Store execution ID for later update
			executionID := idwrap.NewNow()

			currentIndex := itemIndex

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
				Iteration: currentIndex + 1,
			})
			iterContext := &runner.IterationContext{
				IterationPath:  append(parentPath, currentIndex),
				ExecutionIndex: currentIndex,
				ParentNodes:    append(parentNodes, nr.FlowNodeID),
				Labels:         labels,
			}

			// Create initial RUNNING record
			if req.LogPushFunc != nil {
				iterationData := map[string]any{
					"item": item,
					"key":  itemIndex,
				}
				executionName := fmt.Sprintf("%s Iteration %d", nr.Name, currentIndex+1)
				req.LogPushFunc(runner.FlowNodeStatus{
					ExecutionID:      executionID, // Store this ID for update
					NodeID:           nr.FlowNodeID,
					Name:             executionName,
					State:            mnnode.NODE_STATE_RUNNING,
					OutputData:       iterationData,
					IterationEvent:   true,
					IterationIndex:   currentIndex,
					LoopNodeID:       nr.FlowNodeID,
					IterationContext: iterContext,
				})
			}

			itemIndex++
			totalItems++

			result := processNode(currentIndex)

			// Update iteration record based on result
			if req.LogPushFunc != nil && result.Err == nil {
				// Update to SUCCESS (iteration completed successfully)
				executionName := fmt.Sprintf("%s Iteration %d", nr.Name, currentIndex+1)
				req.LogPushFunc(runner.FlowNodeStatus{
					ExecutionID:      executionID, // Same ID = UPDATE
					NodeID:           nr.FlowNodeID,
					Name:             executionName,
					State:            mnnode.NODE_STATE_SUCCESS,
					OutputData:       map[string]any{"item": item, "key": currentIndex},
					IterationEvent:   true,
					IterationIndex:   currentIndex,
					LoopNodeID:       nr.FlowNodeID,
					IterationContext: iterContext,
				})
			}
			// Loop node avoids emitting FAILURE updates; final state handled via FlowNodeResult.

			// Handle iteration error according to error policy
			if result.Err != nil {
				switch nr.ErrorHandling {
				case mnfor.ErrorHandling_ERROR_HANDLING_IGNORE:
					continue // Continue to next iteration
				case mnfor.ErrorHandling_ERROR_HANDLING_BREAK:
					goto ExitSeq // Stop loop but don't propagate error
				case mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED:
					loopError = result.Err
					goto ExitSeq // Fail entire flow
				}
			}
		}

	ExitSeq:
		if loopError != nil {
			if !runner.IsCancellationError(loopError) {
				loopError = errors.Join(runner.ErrFlowCanceledByThrow, loopError)
			}
			return node.FlowNodeResult{
				Err: loopError,
			}
		}
		// Write total items processed
		if req.VariableTracker != nil {
			err = node.WriteNodeVarWithTracking(req, nr.Name, "totalItems", totalItems, req.VariableTracker)
		} else {
			err = node.WriteNodeVar(req, nr.Name, "totalItems", totalItems)
		}
		if err != nil {
			return node.FlowNodeResult{
				Err: err,
			}
		}
	case iter.Seq2[string, any]:
		// Handle map sequence
		totalItems := 0
		var loopError error

		for key, value := range seq {
			// Write the key and item (value) to the node variables
			var err error
			if req.VariableTracker != nil {
				err = node.WriteNodeVarWithTracking(req, nr.Name, "key", key, req.VariableTracker)
			} else {
				err = node.WriteNodeVar(req, nr.Name, "key", key)
			}
			if err != nil {
				return node.FlowNodeResult{
					Err: err,
				}
			}

			if req.VariableTracker != nil {
				err = node.WriteNodeVarWithTracking(req, nr.Name, "item", value, req.VariableTracker)
			} else {
				err = node.WriteNodeVar(req, nr.Name, "item", value)
			}
			if err != nil {
				return node.FlowNodeResult{
					Err: err,
				}
			}

			// Store execution ID for later update
			executionID := idwrap.NewNow()

			currentIndex := totalItems

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
				Iteration: currentIndex + 1,
			})
			iterContext := &runner.IterationContext{
				IterationPath:  append(parentPath, currentIndex),
				ExecutionIndex: currentIndex,
				ParentNodes:    append(parentNodes, nr.FlowNodeID),
				Labels:         labels,
			}

			// Create initial RUNNING record
			if req.LogPushFunc != nil {
				iterationData := map[string]any{
					"item": value,
					"key":  key,
				}
				executionName := fmt.Sprintf("%s Iteration %d", nr.Name, currentIndex+1)
				req.LogPushFunc(runner.FlowNodeStatus{
					ExecutionID:      executionID, // Store this ID for update
					NodeID:           nr.FlowNodeID,
					Name:             executionName,
					State:            mnnode.NODE_STATE_RUNNING,
					OutputData:       iterationData,
					IterationEvent:   true,
					IterationIndex:   currentIndex,
					LoopNodeID:       nr.FlowNodeID,
					IterationContext: iterContext,
				})
			}

			totalItems++

			result := processNode(currentIndex)

			// Update iteration record based on result
			if req.LogPushFunc != nil && result.Err == nil {
				// Update to SUCCESS (iteration completed successfully)
				executionName := fmt.Sprintf("%s Iteration %d", nr.Name, currentIndex+1)
				req.LogPushFunc(runner.FlowNodeStatus{
					ExecutionID:      executionID, // Same ID = UPDATE
					NodeID:           nr.FlowNodeID,
					Name:             executionName,
					State:            mnnode.NODE_STATE_SUCCESS,
					OutputData:       map[string]any{"item": value, "key": key},
					IterationEvent:   true,
					IterationIndex:   currentIndex,
					LoopNodeID:       nr.FlowNodeID,
					IterationContext: iterContext,
				})
			}
			// Loop node avoids emitting FAILURE updates; final state handled via FlowNodeResult.

			// Handle iteration error according to error policy
			if result.Err != nil {
				switch nr.ErrorHandling {
				case mnfor.ErrorHandling_ERROR_HANDLING_IGNORE:
					continue // Continue to next iteration
				case mnfor.ErrorHandling_ERROR_HANDLING_BREAK:
					goto ExitSeq2 // Stop loop but don't propagate error
				case mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED:
					loopError = result.Err
					goto ExitSeq2 // Fail entire flow
				}
			}
		}

	ExitSeq2:
		if loopError != nil {
			if !runner.IsCancellationError(loopError) {
				loopError = errors.Join(runner.ErrFlowCanceledByThrow, loopError)
			}
			return node.FlowNodeResult{
				Err: loopError,
			}
		}
		// Write total items processed
		if req.VariableTracker != nil {
			err = node.WriteNodeVarWithTracking(req, nr.Name, "totalItems", totalItems, req.VariableTracker)
		} else {
			err = node.WriteNodeVar(req, nr.Name, "totalItems", totalItems)
		}
		if err != nil {
			return node.FlowNodeResult{
				Err: err,
			}
		}
	default:
		// Unexpected result type
		return node.FlowNodeResult{
			Err: fmt.Errorf("unexpected iterator type: %T", result),
		}
	}
	// Only skip final status if loop completed all iterations without any errors
	// If we had errors (IGNORE/BREAK), we need final status to show overall success
	return node.FlowNodeResult{
		NextNodeID:      nextID,
		Err:             nil,
		SkipFinalStatus: false,
	}
}

func (nr *NodeForEach) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	loopID := edge.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, edge.HandleLoop)
	nextID := edge.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, edge.HandleThen)
	predecessorMap := flowlocalrunner.BuildPredecessorMap(req.EdgeSourceMap)

	// Use mutex and sync.Once to ensure thread-safe channel access
	var once sync.Once
	var resultMutex sync.Mutex
	resultSent := false

	sendResult := func(result node.FlowNodeResult) {
		resultMutex.Lock()
		defer resultMutex.Unlock()

		if resultSent {
			return // Result already sent
		}

		once.Do(func() {
			// Double-check inside once.Do to prevent race
			if !resultSent {
				// Recover from panic if channel is closed
				defer func() {
					_ = recover() // Ignore panic from closed channel
				}()

				select {
				case resultChan <- result:
					resultSent = true
				case <-ctx.Done():
					// Context cancelled, don't send
					resultSent = true
				default:
					// Channel might be full or closed, don't block
					resultSent = true
				}
			}
		})
	}

	// Safely read VarMap with lock protection
	req.ReadWriteLock.RLock()
	varMapCopy := make(map[string]any)
	for k, v := range req.VarMap {
		varMapCopy[k] = v
	}
	req.ReadWriteLock.RUnlock()

	// Create the expression environment
	exprEnv := expression.NewEnv(varMapCopy)

	// Normalize the iteration path expression
	varMap := varsystem.NewVarMapFromAnyMap(varMapCopy)
	normalizedExpressionIterPath, err := expression.NormalizeExpression(ctx, nr.IterPath, varMap)
	if err != nil {
		sendResult(node.FlowNodeResult{Err: err})
		return
	}

	// Use tracking version if tracker is available
	var result any
	if req.VariableTracker != nil {
		result, err = expression.ExpressionEvaluateAsIterWithTracking(ctx, exprEnv, normalizedExpressionIterPath, req.VariableTracker)
	} else {
		result, err = expression.ExpressionEvaluateAsIter(ctx, exprEnv, normalizedExpressionIterPath)
	}
	if err != nil {
		sendResult(node.FlowNodeResult{Err: err})
		return
	}

	// Normalize the break condition expression
	breakExpr := nr.Condition.Comparisons.Expression
	var normalizedExpressionBreak string
	if breakExpr != "" {
		normalizedExpressionBreak, err = expression.NormalizeExpression(ctx, breakExpr, varMap)
		if err != nil {
			sendResult(node.FlowNodeResult{Err: err})
			return
		}
	}

	// Define the function to process the child node(s) within the loop
	processNode := func(iterationIndex int) node.FlowNodeResult {
		for _, nextNodeID := range loopID {
			// Evaluate the break condition if it exists
			if breakExpr != "" {
				// Use tracking version if tracker is available
				var ok bool
				var err error
				if req.VariableTracker != nil {
					ok, err = expression.ExpressionEvaluteAsBoolWithTracking(ctx, exprEnv, normalizedExpressionBreak, req.VariableTracker)
				} else {
					ok, err = expression.ExpressionEvaluteAsBool(ctx, exprEnv, normalizedExpressionBreak)
				}
				if err != nil {
					return node.FlowNodeResult{Err: err}
				}
				if !ok {
					break
				}
			}

			// Create iteration context for child nodes
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
				Iteration: iterationIndex + 1,
			})
			childIterationContext := &runner.IterationContext{
				IterationPath:  append(parentPath, iterationIndex),
				ExecutionIndex: iterationIndex,                     // Use iteration index to differentiate executions
				ParentNodes:    append(parentNodes, nr.FlowNodeID), // Add current loop node to parent chain
				Labels:         labels,
			}

			// Generate unique execution ID for child node
			childExecutionID := idwrap.NewNow()

			// Create new request with iteration context for child nodes
			childReq := *req // Copy the request
			childReq.IterationContext = childIterationContext
			childReq.ExecutionID = childExecutionID // Set unique execution ID

			// Run the child node asynchronously
			err := flowlocalrunner.RunNodeASync(ctx, nextNodeID, &childReq, req.LogPushFunc, predecessorMap)
			if err != nil {
				switch nr.ErrorHandling {
				case mnfor.ErrorHandling_ERROR_HANDLING_IGNORE:
					// Log error but continue to next iteration
					continue
				case mnfor.ErrorHandling_ERROR_HANDLING_BREAK:
					// Stop the loop but don't propagate error
					return node.FlowNodeResult{}
				case mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED:
					// Default behavior: fail the entire flow
					return node.FlowNodeResult{Err: err}
				}
			}
		}
		return node.FlowNodeResult{}
	}

	// Iterate over the sequence based on its type
	switch seq := result.(type) {
	case iter.Seq[any]:
		// Handle slice/array sequence
		go func() {
			itemIndex := 0
			totalItems := 0
			var loopError error

			for item := range seq {
				// Write the item and key (index) to the node variables
				var err error
				if req.VariableTracker != nil {
					err = node.WriteNodeVarWithTracking(req, nr.Name, "item", item, req.VariableTracker)
				} else {
					err = node.WriteNodeVar(req, nr.Name, "item", item)
				}
				if err != nil {
					sendResult(node.FlowNodeResult{
						Err: err,
					})
					return
				}

				if req.VariableTracker != nil {
					err = node.WriteNodeVarWithTracking(req, nr.Name, "key", itemIndex, req.VariableTracker)
				} else {
					err = node.WriteNodeVar(req, nr.Name, "key", itemIndex)
				}
				if err != nil {
					sendResult(node.FlowNodeResult{
						Err: err,
					})
					return
				}

				// Store execution ID for later update
				executionID := idwrap.NewNow()

				currentIndex := itemIndex

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
					Iteration: currentIndex + 1,
				})
				iterContext := &runner.IterationContext{
					IterationPath:  append(parentPath, currentIndex),
					ExecutionIndex: currentIndex,
					ParentNodes:    append(parentNodes, nr.FlowNodeID),
					Labels:         labels,
				}

				// Create initial RUNNING record
				if req.LogPushFunc != nil {
					iterationData := map[string]any{
						"item": item,
						"key":  itemIndex,
					}
					executionName := fmt.Sprintf("%s Iteration %d", nr.Name, currentIndex+1)
					req.LogPushFunc(runner.FlowNodeStatus{
						ExecutionID:      executionID, // Store this ID for update
						NodeID:           nr.FlowNodeID,
						Name:             executionName,
						State:            mnnode.NODE_STATE_RUNNING,
						OutputData:       iterationData,
						IterationEvent:   true,
						IterationIndex:   currentIndex,
						LoopNodeID:       nr.FlowNodeID,
						IterationContext: iterContext,
					})
				}

				itemIndex++
				totalItems++

				loopResult := processNode(currentIndex)

				// Update iteration record based on result
				if req.LogPushFunc != nil && loopResult.Err == nil {
					// Update to SUCCESS (iteration completed successfully)
					executionName := fmt.Sprintf("%s Iteration %d", nr.Name, currentIndex+1)
					req.LogPushFunc(runner.FlowNodeStatus{
						ExecutionID:      executionID, // Same ID = UPDATE
						NodeID:           nr.FlowNodeID,
						Name:             executionName,
						State:            mnnode.NODE_STATE_SUCCESS,
						OutputData:       map[string]any{"item": item, "key": currentIndex},
						IterationEvent:   true,
						IterationIndex:   currentIndex,
						LoopNodeID:       nr.FlowNodeID,
						IterationContext: iterContext,
					})
				}
				// Loop node avoids emitting FAILURE updates; final state handled via FlowNodeResult.

				// Handle iteration error according to error policy
				if loopResult.Err != nil {
					switch nr.ErrorHandling {
					case mnfor.ErrorHandling_ERROR_HANDLING_IGNORE:
						continue // Continue to next iteration
					case mnfor.ErrorHandling_ERROR_HANDLING_BREAK:
						sendResult(node.FlowNodeResult{NextNodeID: nextID, Err: nil})
						return // Stop loop but don't propagate error
					case mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED:
						loopError = loopResult.Err
						goto ExitSeqAsync // Exit the loop immediately on error
					}
				}
			}

		ExitSeqAsync:
			if loopError != nil {
				if !runner.IsCancellationError(loopError) {
					loopError = errors.Join(runner.ErrFlowCanceledByThrow, loopError)
				}
				sendResult(node.FlowNodeResult{Err: loopError})
				return
			}
			// Write total items processed
			if req.VariableTracker != nil {
				err := node.WriteNodeVarWithTracking(req, nr.Name, "totalItems", totalItems, req.VariableTracker)
				if err != nil {
					sendResult(node.FlowNodeResult{Err: err})
					return
				}
			} else {
				if err := node.WriteNodeVar(req, nr.Name, "totalItems", totalItems); err != nil {
					sendResult(node.FlowNodeResult{Err: err})
					return
				}
			}
			// Send success result after loop finishes
			sendResult(node.FlowNodeResult{NextNodeID: nextID, Err: nil})
		}()
	case iter.Seq2[string, any]:
		// Handle map sequence
		go func() {
			totalItems := 0
			var loopError error

			for key, value := range seq {
				// Write the key and item (value) to the node variables
				var err error
				if req.VariableTracker != nil {
					err = node.WriteNodeVarWithTracking(req, nr.Name, "key", key, req.VariableTracker)
				} else {
					err = node.WriteNodeVar(req, nr.Name, "key", key)
				}
				if err != nil {
					sendResult(node.FlowNodeResult{
						Err: err,
					})
					return
				}

				if req.VariableTracker != nil {
					err = node.WriteNodeVarWithTracking(req, nr.Name, "item", value, req.VariableTracker)
				} else {
					err = node.WriteNodeVar(req, nr.Name, "item", value)
				}
				if err != nil {
					sendResult(node.FlowNodeResult{
						Err: err,
					})
					return
				}

				// Store execution ID for later update
				executionID := idwrap.NewNow()

				currentIndex := totalItems

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
					Iteration: currentIndex + 1,
				})
				iterContext := &runner.IterationContext{
					IterationPath:  append(parentPath, currentIndex),
					ExecutionIndex: currentIndex,
					ParentNodes:    append(parentNodes, nr.FlowNodeID),
					Labels:         labels,
				}

				// Create initial RUNNING record
				if req.LogPushFunc != nil {
					iterationData := map[string]any{
						"item": value,
						"key":  key,
					}
					executionName := fmt.Sprintf("%s Iteration %d", nr.Name, currentIndex+1)
					req.LogPushFunc(runner.FlowNodeStatus{
						ExecutionID:      executionID, // Store this ID for update
						NodeID:           nr.FlowNodeID,
						Name:             executionName,
						State:            mnnode.NODE_STATE_RUNNING,
						OutputData:       iterationData,
						IterationEvent:   true,
						IterationIndex:   currentIndex,
						LoopNodeID:       nr.FlowNodeID,
						IterationContext: iterContext,
					})
				}

				totalItems++

				loopResult := processNode(currentIndex)

				// Update iteration record based on result
				if req.LogPushFunc != nil && loopResult.Err == nil {
					// Update to SUCCESS (iteration completed successfully)
					executionName := fmt.Sprintf("%s Iteration %d", nr.Name, currentIndex+1)
					req.LogPushFunc(runner.FlowNodeStatus{
						ExecutionID:      executionID, // Same ID = UPDATE
						NodeID:           nr.FlowNodeID,
						Name:             executionName,
						State:            mnnode.NODE_STATE_SUCCESS,
						OutputData:       map[string]any{"item": value, "key": key},
						IterationEvent:   true,
						IterationIndex:   currentIndex,
						LoopNodeID:       nr.FlowNodeID,
						IterationContext: iterContext,
					})
				}
				// Loop node avoids emitting FAILURE updates; final state handled via FlowNodeResult.

				// Handle iteration error according to error policy
				if loopResult.Err != nil {
					switch nr.ErrorHandling {
					case mnfor.ErrorHandling_ERROR_HANDLING_IGNORE:
						continue // Continue to next iteration
					case mnfor.ErrorHandling_ERROR_HANDLING_BREAK:
						sendResult(node.FlowNodeResult{NextNodeID: nextID, Err: nil})
						return // Stop loop but don't propagate error
					case mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED:
						loopError = loopResult.Err
						goto ExitSeq2Async
					}
				}
			}

		ExitSeq2Async:
			if loopError != nil {
				if !runner.IsCancellationError(loopError) {
					loopError = errors.Join(runner.ErrFlowCanceledByThrow, loopError)
				}
				sendResult(node.FlowNodeResult{Err: loopError})
				return
			}
			// Write total items processed
			if req.VariableTracker != nil {
				err := node.WriteNodeVarWithTracking(req, nr.Name, "totalItems", totalItems, req.VariableTracker)
				if err != nil {
					sendResult(node.FlowNodeResult{Err: err})
					return
				}
			} else {
				if err := node.WriteNodeVar(req, nr.Name, "totalItems", totalItems); err != nil {
					sendResult(node.FlowNodeResult{Err: err})
					return
				}
			}
			// Send success result after loop finishes
			sendResult(node.FlowNodeResult{NextNodeID: nextID, Err: nil})
		}()
	default:
		// Should not happen if ExpressionEvaluateAsIter works correctly
		sendResult(node.FlowNodeResult{Err: fmt.Errorf("unexpected iterator type: %T", result)})
	}
}
