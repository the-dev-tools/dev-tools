package rflowv2

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/sworkspace"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
	"the-dev-tools/spec/dist/buf/go/api/flow/v1/flowv1connect"
)

var errUnimplemented = errors.New("rflowv2: method not implemented")

type FlowServiceV2RPC struct {
	ws  *sworkspace.WorkspaceService
	fs  *sflow.FlowService
	es  *sedge.EdgeService
	ns  *snode.NodeService
	nrs *snoderequest.NodeRequestService
}

func New(
	ws *sworkspace.WorkspaceService,
	fs *sflow.FlowService,
	es *sedge.EdgeService,
	ns *snode.NodeService,
	nrs *snoderequest.NodeRequestService,
) *FlowServiceV2RPC {
	return &FlowServiceV2RPC{
		ws:  ws,
		fs:  fs,
		es:  es,
		ns:  ns,
		nrs: nrs,
	}
}

func CreateService(srv *FlowServiceV2RPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := flowv1connect.NewFlowServiceHandler(srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (s *FlowServiceV2RPC) FlowCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.FlowCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*flowv1.Flow, 0, len(flows))
	for _, flow := range flows {
		items = append(items, serializeFlow(flow))
	}

	return connect.NewResponse(&flowv1.FlowCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) EdgeCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.EdgeCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	var edgesPB []*flowv1.Edge

	for _, flow := range flows {
		edges, err := s.es.GetEdgesByFlowID(ctx, flow.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, e := range edges {
			edgesPB = append(edgesPB, serializeEdge(e))
		}
	}

	return connect.NewResponse(&flowv1.EdgeCollectionResponse{Items: edgesPB}), nil
}

func (s *FlowServiceV2RPC) NodeCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	var nodesPB []*flowv1.Node
	for _, flow := range flows {
		nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, node := range nodes {
			nodesPB = append(nodesPB, serializeNode(node))
		}
	}

	return connect.NewResponse(&flowv1.NodeCollectionResponse{Items: nodesPB}), nil
}

func (s *FlowServiceV2RPC) NodeHttpCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeHttpCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*flowv1.NodeHttp, 0)

	for _, flow := range flows {
		nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, n := range nodes {
			if n.NodeKind != mnnode.NODE_KIND_REQUEST {
				continue
			}
			nodeReq, err := s.nrs.GetNodeRequest(ctx, n.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if nodeReq == nil || isZeroID(nodeReq.HttpID) {
				continue
			}
			items = append(items, serializeNodeHTTP(*nodeReq))
		}
	}

	return connect.NewResponse(&flowv1.NodeHttpCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) FlowCreate(context.Context, *connect.Request[flowv1.FlowCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) FlowUpdate(context.Context, *connect.Request[flowv1.FlowUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) FlowDelete(context.Context, *connect.Request[flowv1.FlowDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) FlowSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.FlowSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) FlowRun(context.Context, *connect.Request[flowv1.FlowRunRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) FlowVersionCollection(context.Context, *connect.Request[emptypb.Empty]) (*connect.Response[flowv1.FlowVersionCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) FlowVersionSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.FlowVersionSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) FlowVariableCollection(context.Context, *connect.Request[emptypb.Empty]) (*connect.Response[flowv1.FlowVariableCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) FlowVariableCreate(context.Context, *connect.Request[flowv1.FlowVariableCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) FlowVariableUpdate(context.Context, *connect.Request[flowv1.FlowVariableUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) FlowVariableDelete(context.Context, *connect.Request[flowv1.FlowVariableDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) FlowVariableSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.FlowVariableSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) EdgeCreate(context.Context, *connect.Request[flowv1.EdgeCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) EdgeUpdate(context.Context, *connect.Request[flowv1.EdgeUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) EdgeDelete(context.Context, *connect.Request[flowv1.EdgeDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) EdgeSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.EdgeSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeCreate(
	ctx context.Context,
	req *connect.Request[flowv1.NodeCreateRequest],
) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeModel, err := s.deserializeNodeCreate(item)
		if err != nil {
			return nil, err
		}

		if err := s.ensureFlowAccess(ctx, nodeModel.FlowID); err != nil {
			return nil, err
		}

		if err := s.ns.CreateNode(ctx, *nodeModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeUpdate(
	ctx context.Context,
	req *connect.Request[flowv1.NodeUpdateRequest],
) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		existing, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
			return nil, err
		}

		if item.Kind != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node kind updates are not supported"))
		}
		if len(item.GetFlowId()) != 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node flow reassignment is not supported"))
		}

		if item.Name != nil {
			existing.Name = item.GetName()
		}

		if item.Position != nil {
			existing.PositionX = float64(item.Position.GetX())
			existing.PositionY = float64(item.Position.GetY())
		}

		if err := s.ns.UpdateNode(ctx, *existing); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeDelete(
	ctx context.Context,
	req *connect.Request[flowv1.NodeDeleteRequest],
) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		if err := s.ns.DeleteNode(ctx, nodeID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.NodeSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeNoOpCollection(context.Context, *connect.Request[emptypb.Empty]) (*connect.Response[flowv1.NodeNoOpCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeNoOpCreate(context.Context, *connect.Request[flowv1.NodeNoOpCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeNoOpUpdate(context.Context, *connect.Request[flowv1.NodeNoOpUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeNoOpDelete(context.Context, *connect.Request[flowv1.NodeNoOpDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeNoOpSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.NodeNoOpSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeHttpCreate(context.Context, *connect.Request[flowv1.NodeHttpCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeHttpUpdate(context.Context, *connect.Request[flowv1.NodeHttpUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeHttpDelete(context.Context, *connect.Request[flowv1.NodeHttpDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeHttpSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.NodeHttpSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeForCollection(context.Context, *connect.Request[emptypb.Empty]) (*connect.Response[flowv1.NodeForCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeForCreate(context.Context, *connect.Request[flowv1.NodeForCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeForUpdate(context.Context, *connect.Request[flowv1.NodeForUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeForDelete(context.Context, *connect.Request[flowv1.NodeForDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeForSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.NodeForSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeForEachCollection(context.Context, *connect.Request[emptypb.Empty]) (*connect.Response[flowv1.NodeForEachCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeForEachCreate(context.Context, *connect.Request[flowv1.NodeForEachCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeForEachUpdate(context.Context, *connect.Request[flowv1.NodeForEachUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeForEachDelete(context.Context, *connect.Request[flowv1.NodeForEachDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeForEachSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.NodeForEachSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeConditionCollection(context.Context, *connect.Request[emptypb.Empty]) (*connect.Response[flowv1.NodeConditionCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeConditionCreate(context.Context, *connect.Request[flowv1.NodeConditionCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeConditionUpdate(context.Context, *connect.Request[flowv1.NodeConditionUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeConditionDelete(context.Context, *connect.Request[flowv1.NodeConditionDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeConditionSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.NodeConditionSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeJsCollection(context.Context, *connect.Request[emptypb.Empty]) (*connect.Response[flowv1.NodeJsCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeJsCreate(context.Context, *connect.Request[flowv1.NodeJsCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeJsUpdate(context.Context, *connect.Request[flowv1.NodeJsUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeJsDelete(context.Context, *connect.Request[flowv1.NodeJsDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeJsSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.NodeJsSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeExecutionCollection(context.Context, *connect.Request[emptypb.Empty]) (*connect.Response[flowv1.NodeExecutionCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeExecutionSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.NodeExecutionSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) listAccessibleFlows(ctx context.Context) ([]mflow.Flow, error) {
	workspaces, err := s.listUserWorkspaces(ctx)
	if err != nil {
		return nil, err
	}

	var allFlows []mflow.Flow
	for _, ws := range workspaces {
		flows, err := s.fs.GetFlowsByWorkspaceID(ctx, ws.ID)
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

	workspaces, err := s.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return workspaces, nil
}

func serializeFlow(flow mflow.Flow) *flowv1.Flow {
	msg := &flowv1.Flow{
		FlowId: flow.ID.Bytes(),
		Name:   flow.Name,
	}
	if flow.Duration != 0 {
		duration := flow.Duration
		msg.Duration = &duration
	}
	return msg
}

func serializeEdge(e edge.Edge) *flowv1.Edge {
	return &flowv1.Edge{
		EdgeId:       e.ID.Bytes(),
		FlowId:       e.FlowID.Bytes(),
		Kind:         flowv1.EdgeKind(e.Kind),
		SourceId:     e.SourceID.Bytes(),
		TargetId:     e.TargetID.Bytes(),
		SourceHandle: flowv1.Handle(e.SourceHandler),
	}
}

func serializeNode(n mnnode.MNode) *flowv1.Node {
	position := &flowv1.Position{
		X: float32(n.PositionX),
		Y: float32(n.PositionY),
	}

	return &flowv1.Node{
		NodeId:   n.ID.Bytes(),
		FlowId:   n.FlowID.Bytes(),
		Kind:     flowv1.NodeKind(n.NodeKind),
		Name:     n.Name,
		Position: position,
		State:    flowv1.NodeState_NODE_STATE_UNSPECIFIED,
	}
}

func serializeNodeHTTP(n mnrequest.MNRequest) *flowv1.NodeHttp {
	return &flowv1.NodeHttp{
		NodeId: n.FlowNodeID.Bytes(),
		HttpId: n.HttpID.Bytes(),
	}
}

func isZeroID(id idwrap.IDWrap) bool {
	return id == (idwrap.IDWrap{})
}

func (s *FlowServiceV2RPC) deserializeNodeCreate(item *flowv1.NodeCreate) (*mnnode.MNode, error) {
	if item == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node create item is required"))
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

	return &mnnode.MNode{
		ID:        nodeID,
		FlowID:    flowID,
		Name:      item.GetName(),
		NodeKind:  mnnode.NodeKind(item.GetKind()),
		PositionX: posX,
		PositionY: posY,
	}, nil
}

func (s *FlowServiceV2RPC) ensureFlowAccess(ctx context.Context, flowID idwrap.IDWrap) error {
	flow, err := s.fs.GetFlow(ctx, flowID)
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
	return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("flow %s not accessible to current user", flowID.String()))
}

func (s *FlowServiceV2RPC) ensureNodeAccess(ctx context.Context, nodeID idwrap.IDWrap) (*mnnode.MNode, error) {
	node, err := s.ns.GetNode(ctx, nodeID)
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

// Ensure FlowServiceV2RPC implements the generated interface.
var _ flowv1connect.FlowServiceHandler = (*FlowServiceV2RPC)(nil)
