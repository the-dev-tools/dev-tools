package rflow

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api/rworkspace"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mflow"
	"dev-tools-backend/pkg/permcheck"
	"dev-tools-backend/pkg/service/sflow"
	"dev-tools-backend/pkg/service/sflowtag"
	"dev-tools-backend/pkg/service/stag"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/service/sworkspace"
	"dev-tools-backend/pkg/translate/tflow"
	"dev-tools-backend/pkg/translate/tgeneric"
	flowv1 "dev-tools-spec/dist/buf/go/flow/v1"

	"connectrpc.com/connect"
)

type FlowServiceRPC struct {
	DB  *sql.DB
	fs  sflow.FlowService
	ws  sworkspace.WorkspaceService
	us  suser.UserService
	ts  stag.TagService
	fts sflowtag.FlowTagService
}

func New(db *sql.DB, ws sworkspace.WorkspaceService,
	us suser.UserService, ts stag.TagService, fs sflow.FlowService, fts sflowtag.FlowTagService,
) FlowServiceRPC {
	return FlowServiceRPC{
		DB:  db,
		fs:  fs,
		ws:  ws,
		us:  us,
		ts:  ts,
		fts: fts,
	}
}

func (c *FlowServiceRPC) FlowList(ctx context.Context, req *connect.Request[flowv1.FlowListRequest]) (*connect.Response[flowv1.FlowListResponse], error) {
	workspaceID, err := idwrap.NewFromBytes(req.Msg.WorkspaceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	var tagIDPtr *idwrap.IDWrap = nil
	if req.Msg.TagId != nil {
		tagID, err := idwrap.NewFromBytes(req.Msg.TagId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		tagIDPtr = &tagID
	}

	rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, c.us, workspaceID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	var flows []mflow.Flow

	if tagIDPtr == nil {
		flows, err = c.fs.GetFlowsByWorkspace(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
	} else {
		// TODO: can be better with sql query for now it's a workaround
		tag, err := c.ts.GetTag(ctx, *tagIDPtr)
		if err != nil {
			if err == stag.ErrNoTag {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, err
		}
		flowTags, err := c.fts.GetFlowTagsByTagID(ctx, tag.ID)
		if err != nil {
			return nil, err
		}
		flows = make([]mflow.Flow, len(flowTags))
		for i, flowTag := range flowTags {
			flow, err := c.fs.GetFlow(ctx, flowTag.FlowID)
			if err != nil {
				return nil, err
			}

			flows[i] = flow
		}
	}
	rpcResp := &flowv1.FlowListResponse{
		Items: tgeneric.MassConvert(flows, tflow.SeralizeModelToRPCItem),
	}
	return connect.NewResponse(rpcResp), nil
}

func (c *FlowServiceRPC) FlowGet(ctx context.Context, req *connect.Request[flowv1.FlowGetRequest]) (*connect.Response[flowv1.FlowGetResponse], error) {
	flowID, err := idwrap.NewFromBytes(req.Msg.FlowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerFlow(ctx, c.fs, c.us, flowID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	flow, err := c.fs.GetFlow(ctx, flowID)
	if err != nil {
		return nil, err
	}
	rpcFlow := tflow.SeralizeModelToRPC(flow)
	rpcResp := &flowv1.FlowGetResponse{
		FlowId: rpcFlow.FlowId,
		Name:   rpcFlow.Name,
	}
	return connect.NewResponse(rpcResp), nil
}

func (c *FlowServiceRPC) FlowCreate(ctx context.Context, req *connect.Request[flowv1.FlowCreateRequest]) (*connect.Response[flowv1.FlowCreateResponse], error) {
	workspaceID, err := idwrap.NewFromBytes(req.Msg.WorkspaceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, c.us, workspaceID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	rpcFlow := flowv1.Flow{
		Name: req.Msg.Name,
	}
	flow := tflow.SeralizeRpcToModelWithoutID(&rpcFlow)
	flowID := idwrap.NewNow()
	flow.ID = flowID
	err = c.fs.CreateFlow(ctx, *flow)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&flowv1.FlowCreateResponse{
		FlowId: flowID.Bytes(),
	}), nil
}

func (c *FlowServiceRPC) FlowUpdate(ctx context.Context, req *connect.Request[flowv1.FlowUpdateRequest]) (*connect.Response[flowv1.FlowUpdateResponse], error) {
	rpcFlow := flowv1.Flow{
		FlowId: req.Msg.FlowId,
		Name:   req.Msg.Name,
	}
	flow, err := tflow.SeralizeRpcToModel(&rpcFlow)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerFlow(ctx, c.fs, c.us, flow.ID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	err = c.fs.UpdateFlow(ctx, *flow)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&flowv1.FlowUpdateResponse{}), nil
}

func (c *FlowServiceRPC) FlowDelete(ctx context.Context, req *connect.Request[flowv1.FlowDeleteRequest]) (*connect.Response[flowv1.FlowDeleteResponse], error) {
	flowID, err := idwrap.NewFromBytes(req.Msg.FlowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerFlow(ctx, c.fs, c.us, flowID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	err = c.fs.DeleteFlow(ctx, flowID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&flowv1.FlowDeleteResponse{}), nil
}

func (c *FlowServiceRPC) FlowRun(ctx context.Context, req *connect.Request[flowv1.FlowRunRequest]) (*connect.ServerStreamForClient[flowv1.FlowRunResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func CheckOwnerFlow(ctx context.Context, fs sflow.FlowService, us suser.UserService, flowID idwrap.IDWrap) (bool, error) {
	// TODO: add sql query to make it faster
	flow, err := fs.GetFlow(ctx, flowID)
	if err != nil {
		return false, err
	}
	return rworkspace.CheckOwnerWorkspace(ctx, us, flow.WorkspaceID)
}
