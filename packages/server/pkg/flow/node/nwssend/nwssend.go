//nolint:revive // exported
package nwssend

import (
	"context"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/expression"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"nhooyr.io/websocket" //nolint:staticcheck // nhooyr.io/websocket is the project's current WS library
)

// Compile-time check that NodeWsSend implements VariableIntrospector.
var _ node.VariableIntrospector = (*NodeWsSend)(nil)

// NodeWsSend sends a message to a WebSocket connection established by a WsConnection node.
type NodeWsSend struct {
	FlowNodeID           idwrap.IDWrap
	Name                 string
	WsConnectionNodeName string
	Message              string
}

func New(id idwrap.IDWrap, name string, wsConnectionNodeName string, message string) *NodeWsSend {
	return &NodeWsSend{
		FlowNodeID:           id,
		Name:                 name,
		WsConnectionNodeName: wsConnectionNodeName,
		Message:              message,
	}
}

func (n *NodeWsSend) GetID() idwrap.IDWrap {
	return n.FlowNodeID
}

func (n *NodeWsSend) SetID(id idwrap.IDWrap) {
	n.FlowNodeID = id
}

func (n *NodeWsSend) GetName() string {
	return n.Name
}

// GetRequiredVariables implements node.VariableIntrospector.
func (n *NodeWsSend) GetRequiredVariables() []string {
	return expression.ExtractVarKeysFromMultiple(n.Message, n.WsConnectionNodeName)
}

// GetOutputVariables implements node.VariableIntrospector.
func (n *NodeWsSend) GetOutputVariables() []string {
	return []string{
		"type",
		"message",
		"connectionNode",
	}
}

func (n *NodeWsSend) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	// Interpolate the message template
	varMapCopy := node.DeepCopyVarMap(req)
	env := expression.NewUnifiedEnv(varMapCopy)
	interpolated, err := env.InterpolateCtx(ctx, n.Message)
	if err != nil {
		return node.FlowNodeResult{Err: fmt.Errorf("interpolate message: %w", err)}
	}

	// Read the WebSocket connection from the WsConnection node's VarMap
	connRaw, err := node.ReadNodeVar(req, n.WsConnectionNodeName, "_conn")
	if err != nil {
		return node.FlowNodeResult{Err: fmt.Errorf("read ws connection from node %q: %w", n.WsConnectionNodeName, err)}
	}

	conn, ok := connRaw.(*websocket.Conn) //nolint:staticcheck // nhooyr.io/websocket
	if !ok {
		return node.FlowNodeResult{Err: fmt.Errorf("ws connection from node %q is not a valid WebSocket connection", n.WsConnectionNodeName)}
	}

	// Send the message
	if err := conn.Write(ctx, websocket.MessageText, []byte(interpolated)); err != nil { //nolint:staticcheck // nhooyr.io/websocket
		return node.FlowNodeResult{Err: fmt.Errorf("websocket write: %w", err)}
	}

	// Write the sent message to output vars
	writeVar := func(key string, v any) error {
		if req.VariableTracker != nil {
			return node.WriteNodeVarWithTracking(req, n.Name, key, v, req.VariableTracker)
		}
		return node.WriteNodeVar(req, n.Name, key, v)
	}
	if err := writeVar("type", "sent"); err != nil {
		return node.FlowNodeResult{Err: fmt.Errorf("write type var: %w", err)}
	}
	if err := writeVar("message", interpolated); err != nil {
		return node.FlowNodeResult{Err: fmt.Errorf("write message var: %w", err)}
	}
	if err := writeVar("connectionNode", n.WsConnectionNodeName); err != nil {
		return node.FlowNodeResult{Err: fmt.Errorf("write connectionNode var: %w", err)}
	}

	nextID := mflow.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, mflow.HandleUnspecified)
	return node.FlowNodeResult{
		NextNodeID: nextID,
	}
}

func (n *NodeWsSend) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	resultChan <- n.RunSync(ctx, req)
}
