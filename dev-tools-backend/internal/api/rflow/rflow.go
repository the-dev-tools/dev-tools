package rflow

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/service/scollection"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/service/sworkspace"
	flowv1 "dev-tools-spec/dist/buf/go/flow/v1"

	"connectrpc.com/connect"
)

type FlowServiceRPC struct {
	DB *sql.DB
	cs scollection.CollectionService
	ws sworkspace.WorkspaceService
	us suser.UserService
}

func New(db *sql.DB, cs scollection.CollectionService, ws sworkspace.WorkspaceService,
	us suser.UserService,
) FlowServiceRPC {
	return FlowServiceRPC{
		DB: db,
		cs: cs,
		ws: ws,
		us: us,
	}
}

func (c *FlowServiceRPC) FlowList(ctx context.Context, req *connect.Request[flowv1.FlowListRequest]) (*connect.Response[flowv1.FlowListResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (c *FlowServiceRPC) FlowGet(ctx context.Context, req *connect.Request[flowv1.FlowGetRequest]) (*connect.Response[flowv1.FlowGetResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (c *FlowServiceRPC) FlowCreate(ctx context.Context, req *connect.Request[flowv1.FlowCreateRequest]) (*connect.Response[flowv1.FlowCreateResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (c *FlowServiceRPC) FlowUpdate(ctx context.Context, req *connect.Request[flowv1.FlowUpdateRequest]) (*connect.Response[flowv1.FlowUpdateResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (c *FlowServiceRPC) FlowDelete(ctx context.Context, req *connect.Request[flowv1.FlowDeleteRequest]) (*connect.Response[flowv1.FlowDeleteResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (c *FlowServiceRPC) FlowRun(ctx context.Context, req *connect.Request[flowv1.FlowRunRequest]) (*connect.ServerStreamForClient[flowv1.FlowRunResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
