//nolint:revive // exported
package nwsconnection

import (
	"context"
	"fmt"
	"net/http"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/expression"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner/flowlocalrunner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"nhooyr.io/websocket" //nolint:staticcheck // nhooyr.io/websocket is the project's current WS library
)

// Compile-time check that NodeWsConnection implements VariableIntrospector.
var _ node.VariableIntrospector = (*NodeWsConnection)(nil)

// NodeWsConnection is a listener entry node that connects to a WebSocket
// and dispatches HandleWsMessage chains for each incoming message.
type NodeWsConnection struct {
	FlowNodeID idwrap.IDWrap
	Name       string
	URL        string
	Headers    map[string]string
}

func New(id idwrap.IDWrap, name string, url string, headers map[string]string) *NodeWsConnection {
	return &NodeWsConnection{
		FlowNodeID: id,
		Name:       name,
		URL:        url,
		Headers:    headers,
	}
}

func (n *NodeWsConnection) GetID() idwrap.IDWrap {
	return n.FlowNodeID
}

func (n *NodeWsConnection) SetID(id idwrap.IDWrap) {
	n.FlowNodeID = id
}

func (n *NodeWsConnection) GetName() string {
	return n.Name
}

// IsEntryNode marks this as a valid flow entry point (no incoming edges).
func (n *NodeWsConnection) IsEntryNode() bool {
	return true
}

// IsLoopCoordinator prevents the runner from applying per-node timeout.
func (n *NodeWsConnection) IsLoopCoordinator() bool {
	return true
}

// GetRequiredVariables implements node.VariableIntrospector.
func (n *NodeWsConnection) GetRequiredVariables() []string {
	sources := []string{n.URL}
	for _, v := range n.Headers {
		sources = append(sources, v)
	}
	return expression.ExtractVarKeysFromMultiple(sources...)
}

// GetOutputVariables implements node.VariableIntrospector.
func (n *NodeWsConnection) GetOutputVariables() []string {
	return []string{
		"url",
		"connected",
		"lastMessage",
	}
}

func (n *NodeWsConnection) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	// Interpolate URL with variables
	varMapCopy := node.DeepCopyVarMap(req)
	env := newExprEnv(varMapCopy)
	url, err := env.InterpolateCtx(ctx, n.URL)
	if err != nil {
		return node.FlowNodeResult{Err: fmt.Errorf("interpolate url: %w", err)}
	}

	// Build HTTP headers
	httpHeaders := http.Header{}
	for k, v := range n.Headers {
		interpolatedVal, err := env.InterpolateCtx(ctx, v)
		if err != nil {
			return node.FlowNodeResult{Err: fmt.Errorf("interpolate header %s: %w", k, err)}
		}
		httpHeaders.Set(k, interpolatedVal)
	}

	// Dial WebSocket
	conn, resp, err := websocket.Dial(ctx, url, &websocket.DialOptions{ //nolint:staticcheck // nhooyr.io/websocket in use
		HTTPHeader: httpHeaders,
	})
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return node.FlowNodeResult{Err: fmt.Errorf("websocket dial %s: %w", url, err)}
	}

	closeConn := func() {
		_ = conn.Close(websocket.StatusNormalClosure, "") //nolint:staticcheck // best-effort cleanup
	}

	// Store connection in VarMap so WsSend nodes can use it
	if err := node.WriteNodeVar(req, n.Name, "url", url); err != nil {
		closeConn()
		return node.FlowNodeResult{Err: fmt.Errorf("write url var: %w", err)}
	}
	if err := node.WriteNodeVar(req, n.Name, "connected", true); err != nil {
		closeConn()
		return node.FlowNodeResult{Err: fmt.Errorf("write connected var: %w", err)}
	}
	// Store the actual connection object for WsSend nodes to use
	if err := node.WriteNodeVar(req, n.Name, "_conn", conn); err != nil {
		closeConn()
		return node.FlowNodeResult{Err: fmt.Errorf("write conn var: %w", err)}
	}

	nextID := mflow.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, mflow.HandleUnspecified)

	// Check for HandleWsMessage targets — if present, read messages and dispatch child chains
	msgTargets := mflow.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, mflow.HandleWsMessage)

	// No HandleWsMessage targets — just read and log messages passively
	if msgTargets == nil {
		go func() {
			defer conn.Close(websocket.StatusNormalClosure, "done") //nolint:errcheck,staticcheck // best-effort cleanup
			var msgIndex int
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				_, msg, err := conn.Read(ctx) //nolint:staticcheck // nhooyr.io/websocket
				if err != nil {
					return
				}
				msgStr := string(msg)
				_ = node.WriteNodeVar(req, n.Name, "lastMessage", msgStr)

				if req.LogPushFunc != nil {
					executionID := idwrap.NewMonotonic()
					req.LogPushFunc(runner.FlowNodeStatus{
						ExecutionID:    executionID,
						NodeID:         n.FlowNodeID,
						Name:           fmt.Sprintf("%s Message %d", n.Name, msgIndex+1),
						State:          mflow.NODE_STATE_SUCCESS,
						OutputData:     map[string]any{"message": msgStr, "index": msgIndex},
						IterationEvent: true,
						IterationIndex: msgIndex,
						LoopNodeID:     n.FlowNodeID,
					})
				}
				msgIndex++
			}
		}()
		return node.FlowNodeResult{NextNodeID: nextID}
	}

	msgTargets = node.FilterLoopEntryNodes(req.EdgeSourceMap, msgTargets)
	msgEdgeMap := node.BuildHandleExecutionEdgeMap(req.EdgeSourceMap, n.FlowNodeID, mflow.HandleWsMessage, msgTargets)
	predecessorMap := flowlocalrunner.BuildPredecessorMap(msgEdgeMap)
	pendingTemplate := node.BuildPendingMap(predecessorMap)

	// Read messages in a loop until context cancellation, executing the message handler chain per message
	go func() {
		defer conn.Close(websocket.StatusNormalClosure, "done") //nolint:errcheck,staticcheck // best-effort cleanup
		var msgIndex int
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			_, msg, err := conn.Read(ctx) //nolint:staticcheck // nhooyr.io/websocket
			if err != nil {
				return
			}

			msgStr := string(msg)
			_ = node.WriteNodeVar(req, n.Name, "lastMessage", msgStr)

			executionID := idwrap.NewMonotonic()

			// Build iteration context for this message
			var parentPath []int
			var parentNodes []idwrap.IDWrap
			var parentLabels []runner.IterationLabel
			if req.IterationContext != nil {
				parentPath = req.IterationContext.IterationPath
				parentNodes = req.IterationContext.ParentNodes
				parentLabels = node.CloneIterationLabels(req.IterationContext.Labels)
			}
			labels := make([]runner.IterationLabel, len(parentLabels), len(parentLabels)+1)
			copy(labels, parentLabels)
			labels = append(labels, runner.IterationLabel{
				NodeID:    n.FlowNodeID,
				Name:      n.Name,
				Iteration: msgIndex + 1,
			})
			iterContext := &runner.IterationContext{
				IterationPath: append(parentPath, msgIndex),
				ParentNodes:   append(parentNodes, n.FlowNodeID),
				Labels:        labels,
			}

			executionName := fmt.Sprintf("%s Message %d", n.Name, msgIndex+1)
			if req.LogPushFunc != nil {
				req.LogPushFunc(runner.FlowNodeStatus{
					ExecutionID:      executionID,
					NodeID:           n.FlowNodeID,
					Name:             executionName,
					State:            mflow.NODE_STATE_RUNNING,
					OutputData:       map[string]any{"message": msgStr, "index": msgIndex},
					IterationEvent:   true,
					IterationIndex:   msgIndex,
					LoopNodeID:       n.FlowNodeID,
					IterationContext: iterContext,
				})
			}

			// Execute message handler chain
			var iterErr error
			for _, targetID := range msgTargets {
				childIterCtx := &runner.IterationContext{
					IterationPath:  append([]int(nil), iterContext.IterationPath...),
					ExecutionIndex: msgIndex,
					ParentNodes:    append([]idwrap.IDWrap(nil), iterContext.ParentNodes...),
					Labels:         node.CloneIterationLabels(iterContext.Labels),
				}

				childReq := *req
				childReq.EdgeSourceMap = msgEdgeMap
				childReq.PendingAtmoicMap = node.ClonePendingMap(pendingTemplate)
				childReq.IterationContext = childIterCtx
				childReq.ExecutionID = idwrap.NewMonotonic()

				if err := flowlocalrunner.RunNodeSync(ctx, targetID, &childReq, req.LogPushFunc, predecessorMap); err != nil {
					iterErr = err
					break
				}
			}

			if req.LogPushFunc != nil {
				state := mflow.NODE_STATE_SUCCESS
				if iterErr != nil {
					state = mflow.NODE_STATE_FAILURE
				}
				req.LogPushFunc(runner.FlowNodeStatus{
					ExecutionID:      executionID,
					NodeID:           n.FlowNodeID,
					Name:             executionName,
					State:            state,
					Error:            iterErr,
					OutputData:       map[string]any{"message": msgStr, "index": msgIndex},
					IterationEvent:   true,
					IterationIndex:   msgIndex,
					LoopNodeID:       n.FlowNodeID,
					IterationContext: iterContext,
				})
			}

			msgIndex++
		}
	}()

	return node.FlowNodeResult{NextNodeID: nextID}
}

func (n *NodeWsConnection) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	resultChan <- n.RunSync(ctx, req)
}

func newExprEnv(varMap map[string]any) *expression.UnifiedEnv {
	return expression.NewUnifiedEnv(varMap)
}
