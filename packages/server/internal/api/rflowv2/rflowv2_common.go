package rflowv2

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/converter"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcondition"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

//nolint:unused // used by tests
func isStartNode(node mflow.Node) bool {
	return node.NodeKind == mflow.NODE_KIND_MANUAL_START
}

func serializeFlow(flow mflow.Flow) *flowv1.Flow {
	msg := &flowv1.Flow{
		FlowId:      flow.ID.Bytes(),
		WorkspaceId: flow.WorkspaceID.Bytes(),
		Name:        flow.Name,
		Running:     flow.Running,
	}
	if flow.Duration != 0 {
		duration := flow.Duration
		msg.Duration = &duration
	}
	return msg
}

func serializeEdge(e mflow.Edge) *flowv1.Edge {
	return &flowv1.Edge{
		EdgeId:       e.ID.Bytes(),
		FlowId:       e.FlowID.Bytes(),
		SourceId:     e.SourceID.Bytes(),
		TargetId:     e.TargetID.Bytes(),
		SourceHandle: flowv1.HandleKind(e.SourceHandler),
		State:        flowv1.FlowItemState(e.State),
	}
}

func serializeNode(n mflow.Node) *flowv1.Node {
	position := &flowv1.Position{
		X: float32(n.PositionX),
		Y: float32(n.PositionY),
	}

	return &flowv1.Node{
		NodeId:   n.ID.Bytes(),
		FlowId:   n.FlowID.Bytes(),
		Kind:     converter.ToAPINodeKind(n.NodeKind),
		Name:     n.Name,
		Position: position,
		State:    flowv1.FlowItemState(n.State),
	}
}

func serializeNodeHTTP(n mflow.NodeRequest) *flowv1.NodeHttp {
	if n.HttpID == nil {
		return &flowv1.NodeHttp{
			NodeId: n.FlowNodeID.Bytes(),
		}
	}
	msg := &flowv1.NodeHttp{
		NodeId: n.FlowNodeID.Bytes(),
		HttpId: n.HttpID.Bytes(),
	}
	if n.DeltaHttpID != nil {
		msg.DeltaHttpId = n.DeltaHttpID.Bytes()
	}
	return msg
}

func serializeNodeFor(n mflow.NodeFor) *flowv1.NodeFor {
	return &flowv1.NodeFor{
		NodeId:        n.FlowNodeID.Bytes(),
		Iterations:    int32(n.IterCount), // nolint:gosec // G115
		Condition:     n.Condition.Comparisons.Expression,
		ErrorHandling: converter.ToAPIErrorHandling(n.ErrorHandling),
	}
}

func serializeNodeCondition(n mflow.NodeIf) *flowv1.NodeCondition {
	return &flowv1.NodeCondition{
		NodeId:    n.FlowNodeID.Bytes(),
		Condition: n.Condition.Comparisons.Expression,
	}
}

func serializeNodeForEach(n mflow.NodeForEach) *flowv1.NodeForEach {
	return &flowv1.NodeForEach{
		NodeId:        n.FlowNodeID.Bytes(),
		Path:          n.IterExpression,
		Condition:     n.Condition.Comparisons.Expression,
		ErrorHandling: converter.ToAPIErrorHandling(n.ErrorHandling),
	}
}

func serializeNodeJs(n mflow.NodeJS) *flowv1.NodeJs {
	return &flowv1.NodeJs{
		NodeId: n.FlowNodeID.Bytes(),
		Code:   string(n.Code),
	}
}

func serializeNodeAI(n mflow.NodeAI) *flowv1.NodeAi {
	return &flowv1.NodeAi{
		NodeId:        n.FlowNodeID.Bytes(),
		Prompt:        n.Prompt,
		MaxIterations: n.MaxIterations,
	}
}

func serializeNodeGraphQL(n mflow.NodeGraphQL) *flowv1.NodeGraphQL {
	msg := &flowv1.NodeGraphQL{
		NodeId: n.FlowNodeID.Bytes(),
	}
	if n.GraphQLID != nil && !isZeroID(*n.GraphQLID) {
		msg.GraphqlId = n.GraphQLID.Bytes()
	}
	return msg
}

func serializeNodeExecution(execution mflow.NodeExecution) *flowv1.NodeExecution {
	result := &flowv1.NodeExecution{
		NodeExecutionId: execution.ID.Bytes(),
		NodeId:          execution.NodeID.Bytes(),
		Name:            execution.Name,
		State:           flowv1.FlowItemState(execution.State),
	}

	// Handle optional fields
	if execution.Error != nil {
		result.Error = execution.Error
	}

	// Handle input data - decompress if needed
	if execution.InputData != nil {
		if inputDataJSON, err := execution.GetInputJSON(); err == nil && len(inputDataJSON) > 0 {
			var v interface{}
			if err := json.Unmarshal(inputDataJSON, &v); err == nil {
				// Defensive: If v is a string (e.g. double-encoded JSON), try to unmarshal it again
				if s, ok := v.(string); ok {
					var v2 interface{}
					if err := json.Unmarshal([]byte(s), &v2); err == nil {
						v = v2
					}
				}

				if inputValue, err := structpb.NewValue(v); err == nil {
					result.Input = inputValue
				}
			}
		}
	}

	// Handle output data - decompress if needed
	if execution.OutputData != nil {
		if outputDataJSON, err := execution.GetOutputJSON(); err == nil && len(outputDataJSON) > 0 {
			var v interface{}
			if err := json.Unmarshal(outputDataJSON, &v); err == nil {
				// Defensive: If v is a string (e.g. double-encoded JSON), try to unmarshal it again
				if s, ok := v.(string); ok {
					var v2 interface{}
					if err := json.Unmarshal([]byte(s), &v2); err == nil {
						v = v2
					}
				}

				if outputValue, err := structpb.NewValue(v); err == nil {
					result.Output = outputValue
				}
			}
		}
	}

	// Handle HTTP response ID
	if execution.ResponseID != nil {
		result.HttpResponseId = execution.ResponseID.Bytes()
	}

	// Handle GraphQL response ID
	if execution.GraphQLResponseID != nil {
		result.GraphqlResponseId = execution.GraphQLResponseID.Bytes()
	}

	// Handle completion timestamp
	if execution.CompletedAt != nil {
		result.CompletedAt = timestamppb.New(time.Unix(*execution.CompletedAt, 0))
	}

	return result
}

func serializeFlowVariable(variable mflow.FlowVariable) *flowv1.FlowVariable {
	return &flowv1.FlowVariable{
		FlowVariableId: variable.ID.Bytes(),
		FlowId:         variable.FlowID.Bytes(),
		Key:            variable.Name,
		Value:          variable.Value,
		Enabled:        variable.Enabled,
		Description:    variable.Description,
		Order:          float32(variable.Order),
	}
}

func isZeroID(id idwrap.IDWrap) bool {
	return id == (idwrap.IDWrap{})
}

func buildCondition(expression string) mcondition.Condition {
	return mcondition.Condition{
		Comparisons: mcondition.Comparison{
			Expression: expression,
		},
	}
}

func convertHandle(h flowv1.HandleKind) mflow.EdgeHandle {
	return mflow.EdgeHandle(h)
}

func (s *FlowServiceV2RPC) deserializeNodeInsert(item *flowv1.NodeInsert) (*mflow.Node, error) {
	if item == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node insert item is required"))
	}

	if len(item.GetFlowId()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow id is required"))
	}

	flowID, err := idwrap.NewFromBytes(item.GetFlowId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
	}

	nodeID := idwrap.NewNow()
	if len(item.GetNodeId()) != 0 {
		nodeID, err = idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}
	}

	var posX, posY float64
	if p := item.GetPosition(); p != nil {
		posX = float64(p.GetX())
		posY = float64(p.GetY())
	}

	return &mflow.Node{
		ID:        nodeID,
		FlowID:    flowID,
		Name:      item.GetName(),
		NodeKind:  mflow.NodeKind(item.GetKind()),
		PositionX: posX,
		PositionY: posY,
	}, nil
}

func (s *FlowServiceV2RPC) ensureWorkspaceAccess(ctx context.Context, workspaceID idwrap.IDWrap) error {
	workspaces, err := s.listUserWorkspaces(ctx)
	if err != nil {
		return err
	}
	for _, ws := range workspaces {
		if ws.ID == workspaceID {
			return nil
		}
	}
	return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("workspace %s not accessible to current user", workspaceID.String()))
}

func (s *FlowServiceV2RPC) ensureFlowAccess(ctx context.Context, flowID idwrap.IDWrap) error {
	flow, err := s.fsReader.GetFlow(ctx, flowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return connect.NewError(connect.CodeNotFound, fmt.Errorf("flow %s not found", flowID.String()))
		}
		return connect.NewError(connect.CodeInternal, err)
	}

	workspaces, err := s.listUserWorkspaces(ctx)
	if err != nil {
		return err
	}
	for _, ws := range workspaces {
		if ws.ID == flow.WorkspaceID {
			return nil
		}
	}
	return connect.NewError(connect.CodeNotFound, fmt.Errorf("flow %s not found", flowID.String()))
}

func (s *FlowServiceV2RPC) ensureNodeAccess(ctx context.Context, nodeID idwrap.IDWrap) (*mflow.Node, error) {
	node, err := s.nsReader.GetNode(ctx, nodeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("node %s not found", nodeID.String()))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := s.ensureFlowAccess(ctx, node.FlowID); err != nil {
		return nil, err
	}
	return node, nil
}

func (s *FlowServiceV2RPC) ensureEdgeAccess(ctx context.Context, edgeID idwrap.IDWrap) (*mflow.Edge, error) {
	edgeModel, err := s.flowEdgeReader.GetEdge(ctx, edgeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("edge %s not found", edgeID.String()))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := s.ensureFlowAccess(ctx, edgeModel.FlowID); err != nil {
		return nil, err
	}
	return edgeModel, nil
}

func (s *FlowServiceV2RPC) listAccessibleFlows(ctx context.Context) ([]mflow.Flow, error) {
	workspaces, err := s.listUserWorkspaces(ctx)
	if err != nil {
		return nil, err
	}

	var allFlows []mflow.Flow
	for _, ws := range workspaces {
		// Use GetAllFlowsByWorkspaceID to include flow versions for TanStack DB sync
		flows, err := s.fsReader.GetAllFlowsByWorkspaceID(ctx, ws.ID)
		if err != nil {
			if errors.Is(err, sflow.ErrNoFlowFound) {
				continue
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		allFlows = append(allFlows, flows...)
	}
	return allFlows, nil
}

func (s *FlowServiceV2RPC) listUserWorkspaces(ctx context.Context) ([]mworkspace.Workspace, error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	workspaces, err := s.wsReader.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return workspaces, nil
}
