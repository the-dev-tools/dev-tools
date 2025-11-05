package rflowv2

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"connectrpc.com/connect"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/sworkspace"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
	"the-dev-tools/spec/dist/buf/go/api/flow/v1/flowv1connect"
)

var errUnimplemented = errors.New("rflowv2: method not implemented")

type FlowServiceV2RPC struct {
	ws   *sworkspace.WorkspaceService
	fs   *sflow.FlowService
	es   *sedge.EdgeService
	ns   *snode.NodeService
	nrs  *snoderequest.NodeRequestService
	nfs  *snodefor.NodeForService
	nfes *snodeforeach.NodeForEachService
	nifs *snodeif.NodeIfService
	nnos *snodenoop.NodeNoopService
	njss *snodejs.NodeJSService
}

func New(
	ws *sworkspace.WorkspaceService,
	fs *sflow.FlowService,
	es *sedge.EdgeService,
	ns *snode.NodeService,
	nrs *snoderequest.NodeRequestService,
	nfs *snodefor.NodeForService,
	nfes *snodeforeach.NodeForEachService,
	nifs *snodeif.NodeIfService,
	nnos *snodenoop.NodeNoopService,
	njss *snodejs.NodeJSService,
) *FlowServiceV2RPC {
	return &FlowServiceV2RPC{
		ws:   ws,
		fs:   fs,
		es:   es,
		ns:   ns,
		nrs:  nrs,
		nfs:  nfs,
		nfes: nfes,
		nifs: nifs,
		nnos: nnos,
		njss: njss,
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

func (s *FlowServiceV2RPC) FlowCreate(ctx context.Context, req *connect.Request[flowv1.FlowCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one flow is required"))
	}

	workspaceID, err := workspaceIDFromHeaders(req.Header())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := s.ensureWorkspaceAccess(ctx, workspaceID); err != nil {
		return nil, err
	}

	workspace, err := s.ws.Get(ctx, workspaceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, item := range req.Msg.GetItems() {
		name := strings.TrimSpace(item.GetName())
		if name == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow name is required"))
		}

		flowID := idwrap.NewNow()
		if len(item.GetFlowId()) != 0 {
			flowID, err = idwrap.NewFromBytes(item.GetFlowId())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
			}
		}

		flow := mflow.Flow{
			ID:          flowID,
			WorkspaceID: workspaceID,
			Name:        name,
		}

		if err := s.fs.CreateFlow(ctx, flow); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Seed start node so the flow is immediately runnable.
		startNodeID := idwrap.NewNow()
		startNode := mnnode.MNode{
			ID:        startNodeID,
			FlowID:    flowID,
			Name:      "Start",
			NodeKind:  mnnode.NODE_KIND_NO_OP,
			PositionX: 0,
			PositionY: 0,
		}
		if err := s.ns.CreateNode(ctx, startNode); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.nnos.CreateNodeNoop(ctx, mnnoop.NoopNode{
			FlowNodeID: startNodeID,
			Type:       mnnoop.NODE_NO_OP_KIND_START,
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		workspace.FlowCount++
	}

	workspace.Updated = dbtime.DBNow()
	if err := s.ws.Update(ctx, workspace); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowUpdate(ctx context.Context, req *connect.Request[flowv1.FlowUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		if len(item.GetFlowId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow id is required"))
		}

		flowID, err := idwrap.NewFromBytes(item.GetFlowId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
		}

		flow, err := s.fs.GetFlow(ctx, flowID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("flow %s not found", flowID.String()))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.ensureFlowAccess(ctx, flowID); err != nil {
			return nil, err
		}

		if item.Name != nil {
			flow.Name = strings.TrimSpace(item.GetName())
			if flow.Name == "" {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow name cannot be empty"))
			}
		}

		if du := item.GetDuration(); du != nil {
			switch du.GetKind() {
			case flowv1.FlowUpdate_DurationUnion_KIND_UNSET:
				flow.Duration = 0
			case flowv1.FlowUpdate_DurationUnion_KIND_INT32:
				flow.Duration = du.GetInt32()
			}
		}

		if err := s.fs.UpdateFlow(ctx, flow); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowDelete(ctx context.Context, req *connect.Request[flowv1.FlowDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		flowID, err := idwrap.NewFromBytes(item.GetFlowId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
		}

		flow, err := s.fs.GetFlow(ctx, flowID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.ensureFlowAccess(ctx, flowID); err != nil {
			return nil, err
		}

		if err := s.fs.DeleteFlow(ctx, flowID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		workspace, err := s.ws.Get(ctx, flow.WorkspaceID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if workspace.FlowCount > 0 {
			workspace.FlowCount--
		}
		workspace.Updated = dbtime.DBNow()
		if err := s.ws.Update(ctx, workspace); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.FlowSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) FlowRun(context.Context, *connect.Request[flowv1.FlowRunRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) FlowVersionCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[flowv1.FlowVersionCollectionResponse], error) {
	flowID, err := flowIDFromHeaders(req.Header())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := s.ensureFlowAccess(ctx, flowID); err != nil {
		return nil, err
	}

	versions, err := s.fs.GetFlowsByVersionParentID(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	items := make([]*flowv1.FlowVersion, 0, len(versions))
	for _, version := range versions {
		items = append(items, &flowv1.FlowVersion{
			FlowVersionId: version.ID.Bytes(),
			FlowId:        flowID.Bytes(),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return bytes.Compare(items[i].GetFlowVersionId(), items[j].GetFlowVersionId()) < 0
	})

	return connect.NewResponse(&flowv1.FlowVersionCollectionResponse{Items: items}), nil
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

func (s *FlowServiceV2RPC) EdgeCreate(ctx context.Context, req *connect.Request[flowv1.EdgeCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		if len(item.GetFlowId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow id is required"))
		}
		if len(item.GetSourceId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("source id is required"))
		}
		if len(item.GetTargetId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("target id is required"))
		}

		flowID, err := idwrap.NewFromBytes(item.GetFlowId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
		}
		if err := s.ensureFlowAccess(ctx, flowID); err != nil {
			return nil, err
		}

		sourceID, err := idwrap.NewFromBytes(item.GetSourceId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid source id: %w", err))
		}
		if _, err := s.ensureNodeAccess(ctx, sourceID); err != nil {
			return nil, err
		}

		targetID, err := idwrap.NewFromBytes(item.GetTargetId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid target id: %w", err))
		}
		if _, err := s.ensureNodeAccess(ctx, targetID); err != nil {
			return nil, err
		}

		edgeID := idwrap.NewNow()
		if len(item.GetEdgeId()) != 0 {
			edgeID, err = idwrap.NewFromBytes(item.GetEdgeId())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid edge id: %w", err))
			}
		}

		model := edge.Edge{
			ID:            edgeID,
			FlowID:        flowID,
			SourceID:      sourceID,
			TargetID:      targetID,
			SourceHandler: convertHandle(item.GetSourceHandle()),
			Kind:          int32(item.GetKind()),
		}

		if err := s.es.CreateEdge(ctx, model); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) EdgeUpdate(ctx context.Context, req *connect.Request[flowv1.EdgeUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		edgeID, err := idwrap.NewFromBytes(item.GetEdgeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid edge id: %w", err))
		}

		existing, err := s.ensureEdgeAccess(ctx, edgeID)
		if err != nil {
			return nil, err
		}

		if len(item.GetFlowId()) != 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow reassignment is not supported"))
		}

		if len(item.GetSourceId()) != 0 {
			sourceID, err := idwrap.NewFromBytes(item.GetSourceId())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid source id: %w", err))
			}
			if _, err := s.ensureNodeAccess(ctx, sourceID); err != nil {
				return nil, err
			}
			existing.SourceID = sourceID
		}

		if len(item.GetTargetId()) != 0 {
			targetID, err := idwrap.NewFromBytes(item.GetTargetId())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid target id: %w", err))
			}
			if _, err := s.ensureNodeAccess(ctx, targetID); err != nil {
				return nil, err
			}
			existing.TargetID = targetID
		}

		if item.SourceHandle != nil {
			existing.SourceHandler = convertHandle(item.GetSourceHandle())
		}

		if item.Kind != nil {
			existing.Kind = int32(item.GetKind())
		}

		if err := s.es.UpdateEdge(ctx, *existing); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) EdgeDelete(ctx context.Context, req *connect.Request[flowv1.EdgeDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		edgeID, err := idwrap.NewFromBytes(item.GetEdgeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid edge id: %w", err))
		}

		if _, err := s.ensureEdgeAccess(ctx, edgeID); err != nil {
			return nil, err
		}

		if err := s.es.DeleteEdge(ctx, edgeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
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

func (s *FlowServiceV2RPC) NodeNoOpCreate(ctx context.Context, req *connect.Request[flowv1.NodeNoOpCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		if err := s.nnos.DeleteNodeNoop(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		noop := mnnoop.NoopNode{
			FlowNodeID: nodeID,
			Type:       mnnoop.NoopTypes(item.GetKind()),
		}
		if err := s.nnos.CreateNodeNoop(ctx, noop); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeNoOpUpdate(ctx context.Context, req *connect.Request[flowv1.NodeNoOpUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		if item.Kind == nil {
			continue
		}

		if err := s.nnos.DeleteNodeNoop(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		noop := mnnoop.NoopNode{
			FlowNodeID: nodeID,
			Type:       mnnoop.NoopTypes(item.GetKind()),
		}
		if err := s.nnos.CreateNodeNoop(ctx, noop); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeNoOpDelete(ctx context.Context, req *connect.Request[flowv1.NodeNoOpDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		if err := s.nnos.DeleteNodeNoop(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeNoOpSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.NodeNoOpSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeHttpCreate(ctx context.Context, req *connect.Request[flowv1.NodeHttpCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		if len(item.GetHttpId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http id is required"))
		}

		httpID, err := idwrap.NewFromBytes(item.GetHttpId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid http id: %w", err))
		}

		if err := s.nrs.CreateNodeRequest(ctx, mnrequest.MNRequest{
			FlowNodeID:       nodeID,
			HttpID:           httpID,
			HasRequestConfig: !isZeroID(httpID),
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeHttpUpdate(ctx context.Context, req *connect.Request[flowv1.NodeHttpUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		var httpID idwrap.IDWrap
		if len(item.GetHttpId()) != 0 {
			httpID, err = idwrap.NewFromBytes(item.GetHttpId())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid http id: %w", err))
			}
		}

		if err := s.nrs.UpdateNodeRequest(ctx, mnrequest.MNRequest{
			FlowNodeID:       nodeID,
			HttpID:           httpID,
			HasRequestConfig: !isZeroID(httpID),
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeHttpDelete(ctx context.Context, req *connect.Request[flowv1.NodeHttpDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		if err := s.nrs.DeleteNodeRequest(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeHttpSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.NodeHttpSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeForCollection(context.Context, *connect.Request[emptypb.Empty]) (*connect.Response[flowv1.NodeForCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeForCreate(ctx context.Context, req *connect.Request[flowv1.NodeForCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		model := mnfor.MNFor{
			FlowNodeID:    nodeID,
			IterCount:     int64(item.GetIterations()),
			Condition:     buildCondition(item.GetCondition()),
			ErrorHandling: mnfor.ErrorHandling(item.GetErrorHandling()),
		}

		if err := s.nfs.CreateNodeFor(ctx, model); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForUpdate(ctx context.Context, req *connect.Request[flowv1.NodeForUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		existing, err := s.nfs.GetNodeFor(ctx, nodeID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("node %s does not have FOR config", nodeID.String()))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if item.Iterations != nil {
			existing.IterCount = int64(item.GetIterations())
		}
		if item.Condition != nil {
			existing.Condition = buildCondition(item.GetCondition())
		}
		if item.ErrorHandling != nil {
			existing.ErrorHandling = mnfor.ErrorHandling(item.GetErrorHandling())
		}

		if err := s.nfs.UpdateNodeFor(ctx, *existing); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForDelete(ctx context.Context, req *connect.Request[flowv1.NodeForDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		if err := s.nfs.DeleteNodeFor(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.NodeForSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeForEachCollection(context.Context, *connect.Request[emptypb.Empty]) (*connect.Response[flowv1.NodeForEachCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeForEachCreate(ctx context.Context, req *connect.Request[flowv1.NodeForEachCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		model := mnforeach.MNForEach{
			FlowNodeID:     nodeID,
			IterExpression: item.GetPath(),
			Condition:      buildCondition(item.GetCondition()),
			ErrorHandling:  mnfor.ErrorHandling(item.GetErrorHandling()),
		}

		if err := s.nfes.CreateNodeForEach(ctx, model); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForEachUpdate(ctx context.Context, req *connect.Request[flowv1.NodeForEachUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		existing, err := s.nfes.GetNodeForEach(ctx, nodeID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("node %s does not have FOREACH config", nodeID.String()))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if item.Path != nil {
			existing.IterExpression = item.GetPath()
		}
		if item.Condition != nil {
			existing.Condition = buildCondition(item.GetCondition())
		}
		if item.ErrorHandling != nil {
			existing.ErrorHandling = mnfor.ErrorHandling(item.GetErrorHandling())
		}

		if err := s.nfes.UpdateNodeForEach(ctx, *existing); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForEachDelete(ctx context.Context, req *connect.Request[flowv1.NodeForEachDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		if err := s.nfes.DeleteNodeForEach(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForEachSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.NodeForEachSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeConditionCollection(context.Context, *connect.Request[emptypb.Empty]) (*connect.Response[flowv1.NodeConditionCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeConditionCreate(ctx context.Context, req *connect.Request[flowv1.NodeConditionCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		model := mnif.MNIF{
			FlowNodeID: nodeID,
			Condition:  buildCondition(item.GetCondition()),
		}

		if err := s.nifs.CreateNodeIf(ctx, model); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeConditionUpdate(ctx context.Context, req *connect.Request[flowv1.NodeConditionUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		existing, err := s.nifs.GetNodeIf(ctx, nodeID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("node %s does not have CONDITION config", nodeID.String()))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if item.Condition != nil {
			existing.Condition = buildCondition(item.GetCondition())
		}

		if err := s.nifs.UpdateNodeIf(ctx, *existing); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeConditionDelete(ctx context.Context, req *connect.Request[flowv1.NodeConditionDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		if err := s.nifs.DeleteNodeIf(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeConditionSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.NodeConditionSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeJsCollection(context.Context, *connect.Request[emptypb.Empty]) (*connect.Response[flowv1.NodeJsCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeJsCreate(ctx context.Context, req *connect.Request[flowv1.NodeJsCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		model := mnjs.MNJS{
			FlowNodeID:       nodeID,
			Code:             []byte(item.GetCode()),
			CodeCompressType: compress.CompressTypeNone,
		}

		if err := s.njss.CreateNodeJS(ctx, model); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeJsUpdate(ctx context.Context, req *connect.Request[flowv1.NodeJsUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		existing, err := s.njss.GetNodeJS(ctx, nodeID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("node %s does not have JS config", nodeID.String()))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if item.Code != nil {
			existing.Code = []byte(item.GetCode())
		}

		if err := s.njss.UpdateNodeJS(ctx, existing); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeJsDelete(ctx context.Context, req *connect.Request[flowv1.NodeJsDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		if err := s.njss.DeleteNodeJS(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
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

func buildCondition(expression string) mcondition.Condition {
	return mcondition.Condition{
		Comparisons: mcondition.Comparison{
			Expression: expression,
		},
	}
}

func convertHandle(h flowv1.Handle) edge.EdgeHandle {
	return edge.EdgeHandle(h)
}

func workspaceIDFromHeaders(header http.Header) (idwrap.IDWrap, error) {
	value := header.Get("workspace-id")
	if value == "" {
		value = header.Get("x-workspace-id")
	}
	if value == "" {
		return idwrap.IDWrap{}, errors.New("workspace id header is required")
	}
	return idwrap.NewText(value)
}

func flowIDFromHeaders(header http.Header) (idwrap.IDWrap, error) {
	value := header.Get("flow-id")
	if value == "" {
		value = header.Get("x-flow-id")
	}
	if value == "" {
		return idwrap.IDWrap{}, errors.New("flow id header is required")
	}
	return idwrap.NewText(value)
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

func (s *FlowServiceV2RPC) ensureEdgeAccess(ctx context.Context, edgeID idwrap.IDWrap) (*edge.Edge, error) {
	edgeModel, err := s.es.GetEdge(ctx, edgeID)
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

// Ensure FlowServiceV2RPC implements the generated interface.
var _ flowv1connect.FlowServiceHandler = (*FlowServiceV2RPC)(nil)
