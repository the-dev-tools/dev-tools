//nolint:revive // exported
package nrunsubflow

import (
	"context"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/expression"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nai"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// SubFlowExecutor loads and runs a target sub-flow. Implementations live in
// the flowbuilder or CLI packages where they have access to all required services.
type SubFlowExecutor interface {
	// ExecuteSubFlow runs the target flow with the given input variables.
	// It returns the output variables produced by the sub-flow's Return node.
	ExecuteSubFlow(ctx context.Context, targetFlowID *idwrap.IDWrap, targetFlowName string, inputVars map[string]any) (map[string]any, error)
}

// NodeRunSubFlow invokes another flow from the parent flow. It evaluates input
// expressions, calls the SubFlowExecutor, and writes the sub-flow outputs to
// the parent VarMap under this node's name.
type NodeRunSubFlow struct {
	FlowNodeID     idwrap.IDWrap
	Name           string
	TargetFlowID   *idwrap.IDWrap
	TargetFlowName string
	Inputs         []mflow.SubFlowInputMapping
	Executor       SubFlowExecutor

	// TargetParams holds the target sub-flow's trigger parameter definitions.
	// Populated by the builder when it can resolve the target flow.
	// Used by GetAIParams() for AI tool integration.
	TargetParams []mflow.SubFlowParam
}

func New(
	id idwrap.IDWrap,
	name string,
	targetFlowID *idwrap.IDWrap,
	targetFlowName string,
	inputs []mflow.SubFlowInputMapping,
	executor SubFlowExecutor,
) *NodeRunSubFlow {
	return &NodeRunSubFlow{
		FlowNodeID:     id,
		Name:           name,
		TargetFlowID:   targetFlowID,
		TargetFlowName: targetFlowName,
		Inputs:         inputs,
		Executor:       executor,
	}
}

func (n *NodeRunSubFlow) GetID() idwrap.IDWrap {
	return n.FlowNodeID
}

func (n *NodeRunSubFlow) GetName() string {
	return n.Name
}

func (n *NodeRunSubFlow) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	if n.Executor == nil {
		return node.FlowNodeResult{Err: fmt.Errorf("sub-flow executor not configured for node %s", n.Name)}
	}

	// Evaluate input expressions against parent VarMap
	varMapCopy := node.DeepCopyVarMap(req)
	env := expression.NewUnifiedEnv(varMapCopy)
	if req.VariableTracker != nil {
		env = env.WithTracking(req.VariableTracker)
	}

	inputVars := make(map[string]any, len(n.Inputs))
	for _, input := range n.Inputs {
		if input.Expression == "" {
			continue
		}
		val, err := env.EvalInterpolated(ctx, input.Expression)
		if err != nil {
			return node.FlowNodeResult{
				Err: fmt.Errorf("failed to evaluate sub-flow input '%s' expression '%s': %w", input.ParamName, input.Expression, err),
			}
		}
		inputVars[input.ParamName] = val
	}

	// Execute the sub-flow
	outputs, err := n.Executor.ExecuteSubFlow(ctx, n.TargetFlowID, n.TargetFlowName, inputVars)
	if err != nil {
		return node.FlowNodeResult{
			Err: fmt.Errorf("sub-flow execution failed: %w", err),
		}
	}

	// Write outputs to parent VarMap under this node's name
	if outputs == nil {
		outputs = make(map[string]interface{})
	}
	if req.VariableTracker != nil {
		if err := node.WriteNodeVarBulkWithTracking(req, n.Name, outputs, req.VariableTracker); err != nil {
			return node.FlowNodeResult{Err: fmt.Errorf("failed to write sub-flow outputs: %w", err)}
		}
	} else {
		if err := node.WriteNodeVarBulk(req, n.Name, outputs); err != nil {
			return node.FlowNodeResult{Err: fmt.Errorf("failed to write sub-flow outputs: %w", err)}
		}
	}

	nextID := mflow.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, mflow.HandleUnspecified)
	return node.FlowNodeResult{NextNodeID: nextID}
}

func (n *NodeRunSubFlow) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	resultChan <- n.RunSync(ctx, req)
}

// GetAIParams implements nai.AIParamProvider. When RunSubFlow is connected to
// an AI node via HandleAiTools, the AI agent can invoke it as a tool. The
// parameter schema comes from the target flow's SubFlowTrigger params.
func (n *NodeRunSubFlow) GetAIParams() []nai.AIParam {
	if len(n.TargetParams) == 0 {
		return nil
	}

	params := make([]nai.AIParam, 0, len(n.TargetParams))
	for _, p := range n.TargetParams {
		aiType := p.Type
		if aiType == "" {
			aiType = nai.AIParamTypeString
		}
		params = append(params, nai.AIParam{
			Name:     p.Name,
			Type:     aiType,
			Required: p.Required,
		})
	}
	return params
}

// GetRequiredVariables implements node.VariableIntrospector.
func (n *NodeRunSubFlow) GetRequiredVariables() []string {
	var vars []string
	for _, input := range n.Inputs {
		if input.Expression != "" {
			vars = append(vars, expression.ExtractExprIdentifiers(input.Expression)...)
		}
	}
	return vars
}

// GetOutputVariables implements node.VariableIntrospector.
func (n *NodeRunSubFlow) GetOutputVariables() []string {
	// We don't know the exact outputs at build time without resolving the
	// target flow, so return a generic indicator.
	return []string{"*"}
}
