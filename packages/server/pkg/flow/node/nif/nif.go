package nif

import (
	"context"
	"fmt"
	"the-dev-tools/server/pkg/expression"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/varsystem"
)

type NodeIf struct {
	FlowNodeID idwrap.IDWrap
	Name       string
	Condition  mcondition.Condition
}

func New(id idwrap.IDWrap, name string, condition mcondition.Condition) *NodeIf {
	return &NodeIf{
		FlowNodeID: id,
		Name:       name,
		Condition:  condition,
	}
}

func (n NodeIf) GetID() idwrap.IDWrap {
	return n.FlowNodeID
}

func (n *NodeIf) SetID(id idwrap.IDWrap) {
	n.FlowNodeID = id
}

func (n NodeIf) GetName() string {
	return n.Name
}

func (n NodeIf) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	trueID := edge.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, edge.HandleThen)
	falseID := edge.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, edge.HandleElse)
	var result node.FlowNodeResult
	if trueID == nil || falseID == nil {
		result.Err = fmt.Errorf("%w: missing true or false branch for node %s", node.ErrNodeNotFound, n.FlowNodeID)
		return result
	}
	exprEnv := expression.NewEnv(req.VarMap)

	// Normalize the condition expression
	conditionExpr := n.Condition.Comparisons.Expression
	varMap := varsystem.NewVarMapFromAnyMap(req.VarMap)
	normalizedExpression, err := expression.NormalizeExpression(ctx, conditionExpr, varMap)
	if err != nil {
		result.Err = fmt.Errorf("failed to normalize condition expression '%s': %w", conditionExpr, err)
		return result
	}

	// Evaluate the condition expression
	ok, err := expression.ExpressionEvaluteAsBool(ctx, exprEnv, normalizedExpression)
	if err != nil {
		result.Err = fmt.Errorf("failed to evaluate condition expression '%s': %w", normalizedExpression, err)
		return result
	}

	// Write the decision result
	outputData := map[string]interface{}{
		"condition": normalizedExpression,
		"result":    ok,
	}
	if err := node.WriteNodeVarBulk(req, n.Name, outputData); err != nil {
		result.Err = fmt.Errorf("failed to write node output: %w", err)
		return result
	}

	if ok {
		result.NextNodeID = trueID
	} else {
		result.NextNodeID = falseID
	}
	return result
}

func (n NodeIf) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	trueID := edge.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, edge.HandleThen)
	falseID := edge.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, edge.HandleElse)
	var result node.FlowNodeResult
	if trueID == nil || falseID == nil {
		result.Err = fmt.Errorf("%w: missing true or false branch for node %s", node.ErrNodeNotFound, n.FlowNodeID)
		resultChan <- result
		return
	}

	exprEnv := expression.NewEnv(req.VarMap)

	// Normalize the condition expression
	conditionExpr := n.Condition.Comparisons.Expression
	varMap := varsystem.NewVarMapFromAnyMap(req.VarMap)
	normalizedExpression, err := expression.NormalizeExpression(ctx, conditionExpr, varMap)
	if err != nil {
		result.Err = fmt.Errorf("failed to normalize condition expression '%s': %w", conditionExpr, err)
		resultChan <- result
		return
	}

	// Evaluate the condition expression
	ok, err := expression.ExpressionEvaluteAsBool(ctx, exprEnv, normalizedExpression)
	if err != nil {
		result.Err = fmt.Errorf("failed to evaluate condition expression '%s': %w", normalizedExpression, err)
		resultChan <- result
		return
	}

	// Write the decision result
	outputData := map[string]interface{}{
		"condition": normalizedExpression,
		"result":    ok,
	}
	if err := node.WriteNodeVarBulk(req, n.Name, outputData); err != nil {
		result.Err = fmt.Errorf("failed to write node output: %w", err)
		resultChan <- result
		return
	}

	if ok {
		result.NextNodeID = trueID
	} else {
		result.NextNodeID = falseID
	}

	resultChan <- result
}
