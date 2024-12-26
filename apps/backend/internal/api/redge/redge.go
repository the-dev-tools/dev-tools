package redge

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/backend/internal/api"
	"the-dev-tools/backend/internal/api/rflow"
	"the-dev-tools/backend/pkg/flow/edge"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/permcheck"
	"the-dev-tools/backend/pkg/service/sedge"
	"the-dev-tools/backend/pkg/service/sflow"
	"the-dev-tools/backend/pkg/service/snode"
	"the-dev-tools/backend/pkg/service/suser"
	edgev1 "the-dev-tools/spec/dist/buf/go/flow/edge/v1"
	"the-dev-tools/spec/dist/buf/go/flow/edge/v1/edgev1connect"

	"connectrpc.com/connect"
)

type EdgeServiceRPC struct {
	DB *sql.DB

	// parent
	fs sflow.FlowService
	us suser.UserService

	es sedge.EdgeService
	ns snode.NodeService
}

func NewEdgeServiceRPC(db *sql.DB, fs sflow.FlowService, us suser.UserService, es sedge.EdgeService, ns snode.NodeService) *EdgeServiceRPC {
	return &EdgeServiceRPC{
		DB: db,
		fs: fs,
		us: us,
		es: es,
		ns: ns,
	}
}

func CreateService(srv *EdgeServiceRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := edgev1connect.NewEdgeServiceHandler(srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *EdgeServiceRPC) EdgeList(ctx context.Context, req *connect.Request[edgev1.EdgeListRequest]) (*connect.Response[edgev1.EdgeListResponse], error) {
	flowID, err := idwrap.NewFromBytes(req.Msg.FlowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(rflow.CheckOwnerFlow(ctx, c.fs, c.us, flowID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	edges, err := c.es.GetEdgesByFlowID(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rpcItems := make([]*edgev1.EdgeListItem, len(edges))
	for i, edge := range edges {
		rpcItems[i] = &edgev1.EdgeListItem{
			EdgeId:       edge.ID.Bytes(),
			SourceId:     edge.SourceID.Bytes(),
			TargetId:     edge.TargetID.Bytes(),
			SourceHandle: edgev1.Handle(edge.SourceHandler),
		}
	}

	resp := &edgev1.EdgeListResponse{
		Items: rpcItems,
	}

	return connect.NewResponse(resp), nil
}

func (c *EdgeServiceRPC) EdgeGet(ctx context.Context, req *connect.Request[edgev1.EdgeGetRequest]) (*connect.Response[edgev1.EdgeGetResponse], error) {
	EdgeID, err := idwrap.NewFromBytes(req.Msg.EdgeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerEdge(ctx, c.fs, c.us, c.es, EdgeID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (c *EdgeServiceRPC) EdgeCreate(ctx context.Context, req *connect.Request[edgev1.EdgeCreateRequest]) (*connect.Response[edgev1.EdgeCreateResponse], error) {
	flowID, err := idwrap.NewFromBytes(req.Msg.FlowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	a, b := rflow.CheckOwnerFlow(ctx, c.fs, c.us, flowID)
	rpcErr := permcheck.CheckPerm(a, b)
	if rpcErr != nil {
		return nil, rpcErr
	}

	sourceID, err := idwrap.NewFromBytes(req.Msg.SourceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	targetID, err := idwrap.NewFromBytes(req.Msg.TargetId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	sourceNode, err := c.ns.GetNode(ctx, sourceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	targetNode, err := c.ns.GetNode(ctx, targetID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if sourceNode.FlowID != flowID || targetNode.FlowID != flowID {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("source and target nodes must be in the same flow"))
	}

	edgeID := idwrap.NewNow()
	modelEdge := &edge.Edge{
		ID:            edgeID,
		FlowID:        flowID,
		SourceID:      sourceID,
		TargetID:      targetID,
		SourceHandler: edge.EdgeHandle(req.Msg.SourceHandle),
	}

	err = c.es.CreateEdge(ctx, *modelEdge)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &edgev1.EdgeCreateResponse{
		EdgeId: edgeID.Bytes(),
	}
	return connect.NewResponse(resp), nil
}

func (c *EdgeServiceRPC) EdgeUpdate(ctx context.Context, req *connect.Request[edgev1.EdgeUpdateRequest]) (*connect.Response[edgev1.EdgeUpdateResponse], error) {
	EdgeID, err := idwrap.NewFromBytes(req.Msg.EdgeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerEdge(ctx, c.fs, c.us, c.es, EdgeID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	requestedEdge, err := c.es.GetEdge(ctx, EdgeID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	flowID := requestedEdge.FlowID

	sourceID, err := idwrap.NewFromBytes(req.Msg.SourceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	targetID, err := idwrap.NewFromBytes(req.Msg.TargetId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	sourceNode, err := c.ns.GetNode(ctx, sourceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	targetNode, err := c.ns.GetNode(ctx, targetID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if sourceNode.FlowID != flowID || targetNode.FlowID != flowID {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("source and target nodes must be in the same flow"))
	}
	if sourceID.Bytes() != nil {
		requestedEdge.SourceID = sourceID
	}
	if targetID.Bytes() != nil {
		requestedEdge.TargetID = targetID
	}
	err = c.es.UpdateEdge(ctx, *requestedEdge)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&edgev1.EdgeUpdateResponse{}), nil
}

func (c *EdgeServiceRPC) EdgeDelete(ctx context.Context, req *connect.Request[edgev1.EdgeDeleteRequest]) (*connect.Response[edgev1.EdgeDeleteResponse], error) {
	EdgeID, err := idwrap.NewFromBytes(req.Msg.EdgeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerEdge(ctx, c.fs, c.us, c.es, EdgeID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	err = c.es.DeleteEdge(ctx, EdgeID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&edgev1.EdgeDeleteResponse{}), nil
}

func CheckOwnerEdge(ctx context.Context, fs sflow.FlowService, us suser.UserService, es sedge.EdgeService, edgeID idwrap.IDWrap) (bool, error) {
	edge, err := es.GetEdge(ctx, edgeID)
	if err != nil {
		return false, err
	}

	return rflow.CheckOwnerFlow(ctx, fs, us, edge.FlowID)
}
