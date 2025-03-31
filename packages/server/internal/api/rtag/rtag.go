package rtag

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/rworkspace"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/service/stag"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/translate/tgeneric"
	"the-dev-tools/server/pkg/translate/ttag"
	"the-dev-tools/spec/dist/buf/go/tag/v1/tagv1connect"

	tagv1 "the-dev-tools/spec/dist/buf/go/tag/v1"

	"connectrpc.com/connect"
)

type TagServiceRPC struct {
	DB *sql.DB
	ws sworkspace.WorkspaceService
	us suser.UserService
	ts stag.TagService
}

func New(db *sql.DB, ws sworkspace.WorkspaceService, us suser.UserService, ts stag.TagService) TagServiceRPC {
	return TagServiceRPC{
		DB: db,
		ws: ws,
		us: us,
		ts: ts,
	}
}

func CreateService(srv TagServiceRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := tagv1connect.NewTagServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c TagServiceRPC) TagList(ctx context.Context, req *connect.Request[tagv1.TagListRequest]) (*connect.Response[tagv1.TagListResponse], error) {
	wsID, err := idwrap.NewFromBytes(req.Msg.WorkspaceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, c.us, wsID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	tags, err := c.ts.GetTagByWorkspace(ctx, wsID)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnknown, err)
	}

	tgeneric.MassConvert(tags, ttag.SeralizeModelToRPCItem)

	resp := &tagv1.TagListResponse{
		Items: tgeneric.MassConvert(tags, ttag.SeralizeModelToRPCItem),
	}

	return connect.NewResponse(resp), nil
}

func (c TagServiceRPC) TagGet(ctx context.Context, req *connect.Request[tagv1.TagGetRequest]) (*connect.Response[tagv1.TagGetResponse], error) {
	tagID, err := idwrap.NewFromBytes(req.Msg.TagId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerTag(ctx, c.ts, c.us, tagID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	tags, err := c.ts.GetTag(ctx, tagID)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnknown, err)
	}

	rpcTag := ttag.SeralizeModelToRPC(tags)

	resp := &tagv1.TagGetResponse{
		TagId: rpcTag.TagId,
		Name:  rpcTag.Name,
		Color: rpcTag.Color,
	}

	return connect.NewResponse(resp), nil
}

func (c TagServiceRPC) TagCreate(ctx context.Context, req *connect.Request[tagv1.TagCreateRequest]) (*connect.Response[tagv1.TagCreateResponse], error) {
	wsID, err := idwrap.NewFromBytes(req.Msg.WorkspaceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, c.us, wsID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	rpcTag := tagv1.Tag{
		Name:  req.Msg.Name,
		Color: req.Msg.Color,
	}

	tagID := idwrap.NewNow()
	tag := ttag.SeralizeRpcToModelWithoutID(&rpcTag, wsID)
	tag.ID = tagID
	err = c.ts.CreateTag(ctx, *tag)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnknown, err)
	}

	resp := &tagv1.TagCreateResponse{
		TagId: tagID.Bytes(),
	}
	return connect.NewResponse(resp), nil
}

func (c TagServiceRPC) TagUpdate(ctx context.Context, req *connect.Request[tagv1.TagUpdateRequest]) (*connect.Response[tagv1.TagUpdateResponse], error) {
	if req.Msg.TagId == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("tag id is required"))
	}
	if req.Msg.Name == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}

	rpcTag := tagv1.Tag{
		TagId: req.Msg.TagId,
		Name:  *req.Msg.Name,
		Color: *req.Msg.Color,
	}
	tag, err := ttag.SeralizeRpcToModel(&rpcTag, idwrap.IDWrap{})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerTag(ctx, c.ts, c.us, tag.ID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	err = c.ts.UpdateTag(ctx, *tag)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnknown, err)
	}

	return connect.NewResponse(&tagv1.TagUpdateResponse{}), nil
}

func (c TagServiceRPC) TagDelete(ctx context.Context, req *connect.Request[tagv1.TagDeleteRequest]) (*connect.Response[tagv1.TagDeleteResponse], error) {
	tagID, err := idwrap.NewFromBytes(req.Msg.TagId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerTag(ctx, c.ts, c.us, tagID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	err = c.ts.DeleteTag(ctx, tagID)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnknown, err)
	}
	return connect.NewResponse(&tagv1.TagDeleteResponse{}), nil
}

func CheckOwnerTag(ctx context.Context, ts stag.TagService, us suser.UserService, tagID idwrap.IDWrap) (bool, error) {
	// TODO: add sql query to make it faster
	flow, err := ts.GetTag(ctx, tagID)
	if err != nil {
		return false, err
	}
	return rworkspace.CheckOwnerWorkspace(ctx, us, flow.WorkspaceID)
}
