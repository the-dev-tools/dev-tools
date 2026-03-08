//nolint:revive // exported
package nsubflowreturn

import (
	"context"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/expression"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// NodeSubFlowReturn is a terminal node in a sub-flow. It evaluates output
// expressions against the sub-flow's VarMap and writes results under the node
// name. The RunSubFlow caller reads these outputs after execution completes.
type NodeSubFlowReturn struct {
	FlowNodeID idwrap.IDWrap
	Name       string
	Outputs    []mflow.SubFlowOutput
}

func New(id idwrap.IDWrap, name string, outputs []mflow.SubFlowOutput) *NodeSubFlowReturn {
	return &NodeSubFlowReturn{
		FlowNodeID: id,
		Name:       name,
		Outputs:    outputs,
	}
}

func (n *NodeSubFlowReturn) GetID() idwrap.IDWrap {
	return n.FlowNodeID
}

func (n *NodeSubFlowReturn) GetName() string {
	return n.Name
}

func (n *NodeSubFlowReturn) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	varMapCopy := node.DeepCopyVarMap(req)
	env := expression.NewUnifiedEnv(varMapCopy)
	if req.VariableTracker != nil {
		env = env.WithTracking(req.VariableTracker)
	}

	outputData := make(map[string]interface{}, len(n.Outputs))
	for _, out := range n.Outputs {
		if out.Expression == "" {
			outputData[out.Name] = nil
			continue
		}
		val, err := env.EvalInterpolated(ctx, out.Expression)
		if err != nil {
			return node.FlowNodeResult{
				Err: fmt.Errorf("failed to evaluate sub-flow output '%s' expression '%s': %w", out.Name, out.Expression, err),
			}
		}
		outputData[out.Name] = val
	}

	if req.VariableTracker != nil {
		if err := node.WriteNodeVarBulkWithTracking(req, n.Name, outputData, req.VariableTracker); err != nil {
			return node.FlowNodeResult{Err: fmt.Errorf("failed to write return output: %w", err)}
		}
	} else {
		if err := node.WriteNodeVarBulk(req, n.Name, outputData); err != nil {
			return node.FlowNodeResult{Err: fmt.Errorf("failed to write return output: %w", err)}
		}
	}

	// Terminal node — no next nodes
	return node.FlowNodeResult{}
}

func (n *NodeSubFlowReturn) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	resultChan <- n.RunSync(ctx, req)
}

// GetRequiredVariables implements node.VariableIntrospector.
func (n *NodeSubFlowReturn) GetRequiredVariables() []string {
	var vars []string
	for _, out := range n.Outputs {
		if out.Expression != "" {
			vars = append(vars, expression.ExtractExprIdentifiers(out.Expression)...)
		}
	}
	return vars
}

// GetOutputVariables implements node.VariableIntrospector.
func (n *NodeSubFlowReturn) GetOutputVariables() []string {
	vars := make([]string, 0, len(n.Outputs))
	for _, out := range n.Outputs {
		vars = append(vars, out.Name)
	}
	return vars
}
