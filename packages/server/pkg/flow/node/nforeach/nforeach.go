package nforeach

import (
	"context"
	"fmt"
	"iter"
	"the-dev-tools/server/pkg/expression"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
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
	result, err := expression.ExpressionEvaluateAsIter(ctx, exprEnv, normalizedExpressionIterPath)
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
				ok, err := expression.ExpressionEvaluteAsBool(ctx, exprEnv, normalizedExpressionBreak)
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
		fmt.Println("Got a sequence (from a slice/array):")
		itemIndex := 0
		for item := range seq {
			// Write the item and key (index) to the node variables
			err := node.WriteNodeVar(req, nr.Name, "item", item)
			if err != nil {
				return node.FlowNodeResult{
					Err: err,
				}
			}
			err = node.WriteNodeVar(req, nr.Name, "key", itemIndex)
			if err != nil {
				return node.FlowNodeResult{
					Err: err,
				}
			}
			itemIndex++

			result := processNode()
			if result.Err != nil {
				return result
			}
		}
	case iter.Seq2[string, any]:
		fmt.Println("Got a key-value sequence (from a map):")
		for key, value := range seq {
			// Write the key and item (value) to the node variables
			err := node.WriteNodeVar(req, nr.Name, "key", key)
			if err != nil {
				return node.FlowNodeResult{
					Err: err,
				}
			}
			err = node.WriteNodeVar(req, nr.Name, "item", value)
			if err != nil {
				return node.FlowNodeResult{
					Err: err,
				}
			}

			result := processNode()
			if result.Err != nil {
				return result
			}
		}
	default:
		fmt.Println("Unexpected result type")
	}
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

	// Evaluate the iteration path expression to get an iterator
	result, err := expression.ExpressionEvaluateAsIter(ctx, exprEnv, normalizedExpressionIterPath)
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
				ok, err := expression.ExpressionEvaluteAsBool(ctx, exprEnv, normalizedExpressionBreak)
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
				return node.FlowNodeResult{Err: err}
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
			for item := range seq {
				// Write the item and key (index) to the node variables
				err := node.WriteNodeVar(req, nr.Name, "item", item)
				if err != nil {
					resultChan <- node.FlowNodeResult{
						Err: err,
					}
					return
				}
				err = node.WriteNodeVar(req, nr.Name, "key", itemIndex)
				if err != nil {
					resultChan <- node.FlowNodeResult{
						Err: err,
					}
					return
				}
				itemIndex++

				loopResult := processNode()
				if loopResult.Err != nil {
					resultChan <- loopResult
					return // Stop processing on error
				}
			}
			// Send success result after loop finishes
			resultChan <- node.FlowNodeResult{NextNodeID: nextID, Err: nil}
		}()
	case iter.Seq2[string, any]:
		// Handle map sequence
		go func() {
			for key, value := range seq {
				// Write the key and item (value) to the node variables
				err := node.WriteNodeVar(req, nr.Name, "key", key)
				if err != nil {
					resultChan <- node.FlowNodeResult{
						Err: err,
					}
					return
				}
				err = node.WriteNodeVar(req, nr.Name, "item", value)
				if err != nil {
					resultChan <- node.FlowNodeResult{
						Err: err,
					}
					return
				}

				loopResult := processNode()
				if loopResult.Err != nil {
					resultChan <- loopResult
					return // Stop processing on error
				}
			}
			// Send success result after loop finishes
			resultChan <- node.FlowNodeResult{NextNodeID: nextID, Err: nil}
		}()
	default:
		// Should not happen if ExpressionEvaluateAsIter works correctly
		resultChan <- node.FlowNodeResult{Err: fmt.Errorf("unexpected iterator type: %T", result)}
	}
}
