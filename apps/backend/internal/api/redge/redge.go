package redge

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/backend/internal/api"
	"the-dev-tools/backend/internal/api/rflow"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/permcheck"
	"the-dev-tools/backend/pkg/service/sedge"
	"the-dev-tools/backend/pkg/service/sflow"
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
}

func NewEdgeServiceRPC(db *sql.DB, fs sflow.FlowService, us suser.UserService, es sedge.EdgeService) *EdgeServiceRPC {
	return &EdgeServiceRPC{
		DB: db,
		fs: fs,
		us: us,
		es: es,
	}
}

func CreateService(srv EdgeServiceRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := edgev1connect.NewEdgeServiceHandler(&srv, options...)
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
			SourceId:     edge.FlowID.Bytes(),
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
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (c *EdgeServiceRPC) EdgeCreate(ctx context.Context, req *connect.Request[edgev1.EdgeCreateRequest]) (*connect.Response[edgev1.EdgeCreateResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (c *EdgeServiceRPC) EdgeUpdate(ctx context.Context, req *connect.Request[edgev1.EdgeUpdateRequest]) (*connect.Response[edgev1.EdgeUpdateResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (c *EdgeServiceRPC) EdgeDelete(ctx context.Context, req *connect.Request[edgev1.EdgeDeleteRequest]) (*connect.Response[edgev1.EdgeDeleteResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func CheckOwnerEdge(ctx context.Context, fs sflow.FlowService, us suser.UserService, es sedge.EdgeService, edgeID idwrap.IDWrap) (bool, error) {
	edge, err := es.GetEdge(ctx, edgeID)
	if err != nil {
		return false, err
	}

	return rflow.CheckOwnerFlow(ctx, fs, us, edge.FlowID)
}
