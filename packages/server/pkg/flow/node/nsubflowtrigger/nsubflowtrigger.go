//nolint:revive // exported
package nsubflowtrigger

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// NodeSubFlowTrigger is an entry node for sub-flows. It defines input parameters
// that the calling flow must provide. When a sub-flow is invoked, the RunSubFlow
// node injects parameter values into the VarMap before execution starts. This node
// validates required params exist and applies defaults for missing optional params.
type NodeSubFlowTrigger struct {
	FlowNodeID idwrap.IDWrap
	Name       string
	Params     []mflow.SubFlowParam
}

func New(id idwrap.IDWrap, name string, params []mflow.SubFlowParam) *NodeSubFlowTrigger {
	return &NodeSubFlowTrigger{
		FlowNodeID: id,
		Name:       name,
		Params:     params,
	}
}

func (n *NodeSubFlowTrigger) GetID() idwrap.IDWrap {
	return n.FlowNodeID
}

func (n *NodeSubFlowTrigger) GetName() string {
	return n.Name
}

func (n *NodeSubFlowTrigger) IsEntryNode() bool {
	return true
}

func (n *NodeSubFlowTrigger) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	// Validate required params and apply defaults under a single lock to avoid TOCTOU.
	// Input values are already injected into VarMap by the RunSubFlow caller.
	req.ReadWriteLock.Lock()
	for _, p := range n.Params {
		if _, exists := req.VarMap[p.Name]; !exists {
			if p.Required {
				req.ReadWriteLock.Unlock()
				return node.FlowNodeResult{
					Err: fmt.Errorf("missing required sub-flow parameter: %s", p.Name),
				}
			}
			if p.DefaultValue != "" {
				req.VarMap[p.Name] = parseDefaultValue(p.DefaultValue, p.Type)
			}
		}
	}

	// Write param metadata under node name for introspection
	paramInfo := make(map[string]interface{}, len(n.Params))
	for _, p := range n.Params {
		if v, ok := req.VarMap[p.Name]; ok {
			paramInfo[p.Name] = v
		}
	}
	req.VarMap[n.Name] = paramInfo
	req.ReadWriteLock.Unlock()

	nextID := mflow.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, mflow.HandleUnspecified)
	return node.FlowNodeResult{NextNodeID: nextID}
}

func (n *NodeSubFlowTrigger) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	resultChan <- n.RunSync(ctx, req)
}

// GetRequiredVariables implements node.VariableIntrospector.
func (n *NodeSubFlowTrigger) GetRequiredVariables() []string {
	vars := make([]string, 0, len(n.Params))
	for _, p := range n.Params {
		vars = append(vars, p.Name)
	}
	return vars
}

// GetOutputVariables implements node.VariableIntrospector.
func (n *NodeSubFlowTrigger) GetOutputVariables() []string {
	vars := make([]string, 0, len(n.Params))
	for _, p := range n.Params {
		vars = append(vars, p.Name)
	}
	return vars
}

// parseDefaultValue converts a string default value to the appropriate Go type
// based on the parameter's declared type.
func parseDefaultValue(raw string, typ string) interface{} {
	switch typ {
	case "number":
		d := json.NewDecoder(strings.NewReader(raw))
		d.UseNumber()
		var n json.Number
		if err := d.Decode(&n); err == nil {
			if i, err := n.Int64(); err == nil {
				return i
			}
			if f, err := n.Float64(); err == nil {
				return f
			}
		}
		return raw
	case "boolean":
		switch raw {
		case "true":
			return true
		case "false":
			return false
		}
		return raw
	case "json":
		var v interface{}
		if err := json.Unmarshal([]byte(raw), &v); err == nil {
			return v
		}
		return raw
	default: // "string", "any", ""
		return raw
	}
}
