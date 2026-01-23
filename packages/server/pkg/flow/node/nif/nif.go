//nolint:revive // exported
package nif

import (
	"context"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/expression"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcondition"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
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
	trueID := mflow.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, mflow.HandleThen)
	falseID := mflow.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, mflow.HandleElse)
	var result node.FlowNodeResult

	// Create a deep copy of VarMap to prevent concurrent access issues
	varMapCopy := node.DeepCopyVarMap(req)

	// Build unified environment with optional tracking
	env := expression.NewUnifiedEnv(varMapCopy)
	if req.VariableTracker != nil {
		env = env.WithTracking(req.VariableTracker)
	}

	// Evaluate the condition expression (pure expr-lang, no {{ }} interpolation)
	conditionExpr := n.Condition.Comparisons.Expression
	var ok bool
	var err error
	if conditionExpr == "" {
		ok = false
	} else {
		ok, err = env.EvalBool(ctx, conditionExpr)
		if err != nil {
			result.Err = fmt.Errorf("failed to evaluate condition expression '%s': %w", conditionExpr, err)
			return result
		}
	}

	// Write the decision result
	outputData := map[string]interface{}{
		"condition": conditionExpr,
		"result":    ok,
	}
	if req.VariableTracker != nil {
		err = node.WriteNodeVarBulkWithTracking(req, n.Name, outputData, req.VariableTracker)
	} else {
		err = node.WriteNodeVarBulk(req, n.Name, outputData)
	}
	if err != nil {
		result.Err = fmt.Errorf("failed to write node output: %w", err)
		return result
	}

	switch {
	case ok:
		if len(trueID) > 0 {
			result.NextNodeID = trueID
		}
	case len(falseID) > 0:
		result.NextNodeID = falseID
	}
	return result
}

func (n NodeIf) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	trueID := mflow.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, mflow.HandleThen)
	falseID := mflow.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, mflow.HandleElse)
	var result node.FlowNodeResult

	// Create a deep copy of VarMap to prevent concurrent access issues
	varMapCopy := node.DeepCopyVarMap(req)

	// Build unified environment with optional tracking
	env := expression.NewUnifiedEnv(varMapCopy)
	if req.VariableTracker != nil {
		env = env.WithTracking(req.VariableTracker)
	}

	// Evaluate the condition expression (pure expr-lang, no {{ }} interpolation)
	conditionExpr := n.Condition.Comparisons.Expression
	var ok bool
	var err error
	if conditionExpr == "" {
		ok = false
	} else {
		ok, err = env.EvalBool(ctx, conditionExpr)
		if err != nil {
			result.Err = fmt.Errorf("failed to evaluate condition expression '%s': %w", conditionExpr, err)
			resultChan <- result
			return
		}
	}

	// Write the decision result
	outputData := map[string]interface{}{
		"condition": conditionExpr,
		"result":    ok,
	}
	if req.VariableTracker != nil {
		err = node.WriteNodeVarBulkWithTracking(req, n.Name, outputData, req.VariableTracker)
	} else {
		err = node.WriteNodeVarBulk(req, n.Name, outputData)
	}
	if err != nil {
		result.Err = fmt.Errorf("failed to write node output: %w", err)
		resultChan <- result
		return
	}

	switch {
	case ok:
		if len(trueID) > 0 {
			result.NextNodeID = trueID
		}
	case len(falseID) > 0:
		result.NextNodeID = falseID
	}

	resultChan <- result
}
