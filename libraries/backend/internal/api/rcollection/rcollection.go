package rcollection

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/backend/internal/api"
	"the-dev-tools/backend/internal/api/middleware/mwauth"
	"the-dev-tools/backend/pkg/dbtime"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mcollection"
	"the-dev-tools/backend/pkg/permcheck"
	"the-dev-tools/backend/pkg/service/scollection"
	"the-dev-tools/backend/pkg/service/suser"
	"the-dev-tools/backend/pkg/service/sworkspace"
	"the-dev-tools/backend/pkg/translate/tcollection"
	"the-dev-tools/backend/pkg/translate/tgeneric"
	collectionv1 "the-dev-tools/spec/dist/buf/go/collection/v1"
	"the-dev-tools/spec/dist/buf/go/collection/v1/collectionv1connect"

	"connectrpc.com/connect"
)

type CollectionServiceRPC struct {
	DB *sql.DB
	cs scollection.CollectionService
	ws sworkspace.WorkspaceService
	us suser.UserService
}

func New(db *sql.DB, cs scollection.CollectionService, ws sworkspace.WorkspaceService,
	us suser.UserService,
) CollectionServiceRPC {
	return CollectionServiceRPC{
		DB: db,
		cs: cs,
		ws: ws,
		us: us,
	}
}

func CreateService(deps CollectionServiceRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := collectionv1connect.NewCollectionServiceHandler(&deps, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *CollectionServiceRPC) CollectionList(ctx context.Context, req *connect.Request[collectionv1.CollectionListRequest]) (*connect.Response[collectionv1.CollectionListResponse], error) {
	workspaceUlid, err := idwrap.NewFromBytes(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerWorkspace(ctx, c.us, workspaceUlid))
	if rpcErr != nil {
		return nil, rpcErr
	}

	workspaceID, err := c.ws.Get(ctx, workspaceUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	simpleCollections, err := c.cs.ListCollections(ctx, workspaceID.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	respRaw := &collectionv1.CollectionListResponse{
		WorkspaceId: req.Msg.WorkspaceId,
		Items:       tgeneric.MassConvert(simpleCollections, tcollection.SerializeCollectionModelToRPC),
	}
	return connect.NewResponse(respRaw), nil
}

func (c *CollectionServiceRPC) CollectionCreate(ctx context.Context, req *connect.Request[collectionv1.CollectionCreateRequest]) (*connect.Response[collectionv1.CollectionCreateResponse], error) {
	workspaceUlid, err := idwrap.NewFromBytes(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerWorkspace(ctx, c.us, workspaceUlid))
	if rpcErr != nil {
		return nil, rpcErr
	}

	ws, err := c.ws.Get(ctx, workspaceUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	name := req.Msg.GetName()
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is empty"))
	}
	collectionID := idwrap.NewNow()
	collection := mcollection.Collection{
		ID:      collectionID,
		OwnerID: workspaceUlid,
		Name:    name,
		Updated: dbtime.DBNow(),
	}
	err = c.cs.CreateCollection(ctx, &collection)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	ws.CollectionCount++
	ws.Updated = dbtime.DBNow()
	err = c.ws.Update(ctx, ws)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&collectionv1.CollectionCreateResponse{
		CollectionId: collectionID.Bytes(),
	}), nil
}

func (c *CollectionServiceRPC) CollectionGet(ctx context.Context, req *connect.Request[collectionv1.CollectionGetRequest]) (*connect.Response[collectionv1.CollectionGetResponse], error) {
	idWrap, err := idwrap.NewFromBytes(req.Msg.GetCollectionId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerCollection(ctx, c.cs, c.us, idWrap))
	if rpcErr != nil {
		return nil, rpcErr
	}

	collection, err := c.cs.GetCollection(ctx, idWrap)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	respRaw := &collectionv1.CollectionGetResponse{
		CollectionId: collection.ID.Bytes(),
		Name:         collection.Name,
	}

	return connect.NewResponse(respRaw), nil
}

func (c *CollectionServiceRPC) CollectionUpdate(ctx context.Context, req *connect.Request[collectionv1.CollectionUpdateRequest]) (*connect.Response[collectionv1.CollectionUpdateResponse], error) {
	idWrap, err := idwrap.NewFromBytes(req.Msg.GetCollectionId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerCollection(ctx, c.cs, c.us, idWrap))
	if rpcErr != nil {
		return nil, rpcErr
	}

	collectionOld, err := c.cs.GetCollection(ctx, idWrap)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	collection := mcollection.Collection{
		ID:      idWrap,
		Name:    req.Msg.GetName(),
		OwnerID: collectionOld.OwnerID,
	}
	err = c.cs.UpdateCollection(ctx, &collection)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&collectionv1.CollectionUpdateResponse{}), nil
}

func (c *CollectionServiceRPC) CollectionDelete(ctx context.Context, req *connect.Request[collectionv1.CollectionDeleteRequest]) (*connect.Response[collectionv1.CollectionDeleteResponse], error) {
	idWrap, err := idwrap.NewFromBytes(req.Msg.GetCollectionId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerCollection(ctx, c.cs, c.us, idWrap))
	if rpcErr != nil {
		return nil, rpcErr
	}

	cs, err := c.cs.GetCollection(ctx, idWrap)
	if err != nil {
		return nil, err
	}

	ws, err := c.ws.Get(ctx, cs.OwnerID)
	if err != nil {
		return nil, err
	}

	ws.CollectionCount--
	ws.Updated = dbtime.DBNow()
	err = c.ws.Update(ctx, ws)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = c.cs.DeleteCollection(ctx, idWrap)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&collectionv1.CollectionDeleteResponse{}), nil
}

func CheckOwnerWorkspace(ctx context.Context, us suser.UserService, workspaceID idwrap.IDWrap) (bool, error) {
	userUlid, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return false, connect.NewError(connect.CodeInternal, err)
	}

	ok, err := us.CheckUserBelongsToWorkspace(ctx, userUlid, workspaceID)
	if err != nil {
		if err == sql.ErrNoRows {
			// INFO: this mean that workspace not belong to user
			// So for avoid information leakage, we should return not found
			return false, connect.NewError(connect.CodeNotFound, errors.New("workspace not found"))
		}
		return false, err
	}
	return ok, nil
}

func CheckOwnerCollection(ctx context.Context, cs scollection.CollectionService, us suser.UserService, collectionID idwrap.IDWrap) (bool, error) {
	workspaceID, err := cs.GetOwner(ctx, collectionID)
	if err != nil {
		if err == scollection.ErrNoCollectionFound {
			err = errors.New("collection not found")
			return false, connect.NewError(connect.CodePermissionDenied, err)
		}
		return false, connect.NewError(connect.CodeInternal, err)
	}

	return CheckOwnerWorkspace(ctx, us, workspaceID)
}
