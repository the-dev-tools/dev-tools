package nforeach

import (
	"context"
	"fmt"
	"iter"
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

	varMap := varsystem.NewVarMapFromAnyMap(req.VarMap)
	normalizedExpressionIterPath, err := expression.NormalizeExpression(ctx, nr.IterPath, varMap)
	if err != nil {
		return node.FlowNodeResult{
			Err: err,
		}
	}

	exprEnv := expression.NewEnv(req.VarMap)
	
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

	processNode := func() node.FlowNodeResult {
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

			err := flowlocalrunner.RunNodeSync(ctx, nextNodeID, req, req.LogPushFunc)
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
			itemIndex++
			
			// Log iteration data
			if req.LogPushFunc != nil {
				iterationData := map[string]any{
					"index": itemIndex - 1,
					"value": item,
				}
				req.LogPushFunc(runner.FlowNodeStatus{
					ExecutionID: idwrap.NewNow(),
					NodeID:     nr.FlowNodeID,
					Name:       nr.Name,
					State:      mnnode.NODE_STATE_RUNNING,
					OutputData: iterationData,
				})
			}
			
			totalItems++

			result := processNode()
			if result.Err != nil {
				switch nr.ErrorHandling {
				case mnfor.ErrorHandling_ERROR_HANDLING_IGNORE:
					// Log error but continue to next iteration
					continue
				case mnfor.ErrorHandling_ERROR_HANDLING_BREAK:
					// Stop the loop but don't propagate error
					goto Exit
				case mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED:
					// Default behavior: fail the entire flow
					return result
				}
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
			totalItems++

			// Log iteration tracking data
			if req.LogPushFunc != nil {
				iterationData := map[string]any{
					"key":   key,
					"value": value,
				}
				req.LogPushFunc(runner.FlowNodeStatus{
					ExecutionID: idwrap.NewNow(),
					NodeID:     nr.FlowNodeID,
					Name:       nr.Name,
					State:      mnnode.NODE_STATE_RUNNING,
					OutputData: iterationData,
				})
			}

			result := processNode()
			if result.Err != nil {
				switch nr.ErrorHandling {
				case mnfor.ErrorHandling_ERROR_HANDLING_IGNORE:
					// Log error but continue to next iteration
					continue
				case mnfor.ErrorHandling_ERROR_HANDLING_BREAK:
					// Stop the loop but don't propagate error
					goto Exit
				case mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED:
					// Default behavior: fail the entire flow
					return result
				}
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
Exit:
	return node.FlowNodeResult{
		NextNodeID: nextID,
		Err:        nil,
	}
}

func (nr *NodeForEach) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	loopID := edge.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, edge.HandleLoop)
	nextID := edge.GetNextNodeID(req.EdgeSourceMap, nr.FlowNodeID, edge.HandleThen)

	// Create the expression environment
	exprEnv := expression.NewEnv(req.VarMap)

	// Normalize the iteration path expression
	varMap := varsystem.NewVarMapFromAnyMap(req.VarMap)
	normalizedExpressionIterPath, err := expression.NormalizeExpression(ctx, nr.IterPath, varMap)
	if err != nil {
		resultChan <- node.FlowNodeResult{Err: err}
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
		resultChan <- node.FlowNodeResult{Err: err}
		return
	}

	// Normalize the break condition expression
	breakExpr := nr.Condition.Comparisons.Expression
	var normalizedExpressionBreak string
	if breakExpr != "" {
		normalizedExpressionBreak, err = expression.NormalizeExpression(ctx, breakExpr, varMap)
		if err != nil {
			resultChan <- node.FlowNodeResult{Err: err}
			return
		}
	}

	// Define the function to process the child node(s) within the loop
	processNode := func() node.FlowNodeResult {
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

			// Run the child node asynchronously
			err := flowlocalrunner.RunNodeASync(ctx, nextNodeID, req, req.LogPushFunc)
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
			for item := range seq {
				// Write the item and key (index) to the node variables
				var err error
				if req.VariableTracker != nil {
					err = node.WriteNodeVarWithTracking(req, nr.Name, "item", item, req.VariableTracker)
				} else {
					err = node.WriteNodeVar(req, nr.Name, "item", item)
				}
				if err != nil {
					resultChan <- node.FlowNodeResult{
						Err: err,
					}
					return
				}
				
				if req.VariableTracker != nil {
					err = node.WriteNodeVarWithTracking(req, nr.Name, "key", itemIndex, req.VariableTracker)
				} else {
					err = node.WriteNodeVar(req, nr.Name, "key", itemIndex)
				}
				if err != nil {
					resultChan <- node.FlowNodeResult{
						Err: err,
					}
					return
				}
				itemIndex++
				
				// Log iteration data
				if req.LogPushFunc != nil {
					iterationData := map[string]any{
						"index": itemIndex - 1,
						"value": item,
					}
					req.LogPushFunc(runner.FlowNodeStatus{
						NodeID:     nr.FlowNodeID,
						Name:       nr.Name,
						State:      mnnode.NODE_STATE_RUNNING,
						OutputData: iterationData,
					})
				}
				
				totalItems++

				loopResult := processNode()
				if loopResult.Err != nil {
					switch nr.ErrorHandling {
					case mnfor.ErrorHandling_ERROR_HANDLING_IGNORE:
						// Log error but continue to next iteration
						continue
					case mnfor.ErrorHandling_ERROR_HANDLING_BREAK:
						// Stop the loop but don't propagate error
						resultChan <- node.FlowNodeResult{NextNodeID: nextID, Err: nil}
						return
					case mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED:
						// Default behavior: fail the entire flow
						resultChan <- loopResult
						return
					}
				}
			}
			// Write total items processed
			if req.VariableTracker != nil {
				err := node.WriteNodeVarWithTracking(req, nr.Name, "totalItems", totalItems, req.VariableTracker)
				if err != nil {
					resultChan <- node.FlowNodeResult{Err: err}
					return
				}
			} else {
				if err := node.WriteNodeVar(req, nr.Name, "totalItems", totalItems); err != nil {
					resultChan <- node.FlowNodeResult{Err: err}
					return
				}
			}
			if err != nil {
				resultChan <- node.FlowNodeResult{Err: err}
				return
			}
			// Send success result after loop finishes
			resultChan <- node.FlowNodeResult{NextNodeID: nextID, Err: nil}
		}()
	case iter.Seq2[string, any]:
		// Handle map sequence
		go func() {
			totalItems := 0
			for key, value := range seq {
				// Write the key and item (value) to the node variables
				var err error
				if req.VariableTracker != nil {
					err = node.WriteNodeVarWithTracking(req, nr.Name, "key", key, req.VariableTracker)
				} else {
					err = node.WriteNodeVar(req, nr.Name, "key", key)
				}
				if err != nil {
					resultChan <- node.FlowNodeResult{
						Err: err,
					}
					return
				}
				
				if req.VariableTracker != nil {
					err = node.WriteNodeVarWithTracking(req, nr.Name, "item", value, req.VariableTracker)
				} else {
					err = node.WriteNodeVar(req, nr.Name, "item", value)
				}
				if err != nil {
					resultChan <- node.FlowNodeResult{
						Err: err,
					}
					return
				}
				totalItems++

				// Log iteration tracking data
				if req.LogPushFunc != nil {
					iterationData := map[string]any{
						"key":   key,
						"value": value,
					}
					req.LogPushFunc(runner.FlowNodeStatus{
						NodeID:     nr.FlowNodeID,
						Name:       nr.Name,
						State:      mnnode.NODE_STATE_RUNNING,
						OutputData: iterationData,
					})
				}

				loopResult := processNode()
				if loopResult.Err != nil {
					switch nr.ErrorHandling {
					case mnfor.ErrorHandling_ERROR_HANDLING_IGNORE:
						// Log error but continue to next iteration
						continue
					case mnfor.ErrorHandling_ERROR_HANDLING_BREAK:
						// Stop the loop but don't propagate error
						resultChan <- node.FlowNodeResult{NextNodeID: nextID, Err: nil}
						return
					case mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED:
						// Default behavior: fail the entire flow
						resultChan <- loopResult
						return
					}
				}
			}
			// Write total items processed
			if req.VariableTracker != nil {
				err := node.WriteNodeVarWithTracking(req, nr.Name, "totalItems", totalItems, req.VariableTracker)
				if err != nil {
					resultChan <- node.FlowNodeResult{Err: err}
					return
				}
			} else {
				if err := node.WriteNodeVar(req, nr.Name, "totalItems", totalItems); err != nil {
					resultChan <- node.FlowNodeResult{Err: err}
					return
				}
			}
			if err != nil {
				resultChan <- node.FlowNodeResult{Err: err}
				return
			}
			// Send success result after loop finishes
			resultChan <- node.FlowNodeResult{NextNodeID: nextID, Err: nil}
		}()
	default:
		// Should not happen if ExpressionEvaluateAsIter works correctly
		resultChan <- node.FlowNodeResult{Err: fmt.Errorf("unexpected iterator type: %T", result)}
	}
}
