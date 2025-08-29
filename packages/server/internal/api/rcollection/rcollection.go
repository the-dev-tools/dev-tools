package rcollection

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/translate/tcollection"
	"the-dev-tools/server/pkg/translate/tgeneric"
	collectionv1 "the-dev-tools/spec/dist/buf/go/collection/v1"
	"the-dev-tools/spec/dist/buf/go/collection/v1/collectionv1connect"
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"

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

	simpleCollections, err := c.cs.GetCollectionsOrdered(ctx, workspaceID.ID)
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
		ID:          collectionID,
		WorkspaceID: workspaceUlid,
		Name:        name,
		Updated:     dbtime.DBNow(),
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
		ID:          idWrap,
		Name:        req.Msg.GetName(),
		WorkspaceID: collectionOld.WorkspaceID,
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

	ws, err := c.ws.Get(ctx, cs.WorkspaceID)
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
	workspaceID, err := cs.GetWorkspaceID(ctx, collectionID)
	if err != nil {
		if err == scollection.ErrNoCollectionFound {
			// Return CodeNotFound for non-existent collections
			return false, connect.NewError(connect.CodeNotFound, errors.New("collection not found"))
		}
		return false, connect.NewError(connect.CodeInternal, err)
	}

	return CheckOwnerWorkspace(ctx, us, workspaceID)
}

func (c *CollectionServiceRPC) CollectionMove(ctx context.Context, req *connect.Request[collectionv1.CollectionMoveRequest]) (*connect.Response[collectionv1.CollectionMoveResponse], error) {
	// Validate collection ID
	collectionID, err := idwrap.NewFromBytes(req.Msg.GetCollectionId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Check permissions for the collection being moved
	rpcErr := permcheck.CheckPerm(CheckOwnerCollection(ctx, c.cs, c.us, collectionID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Validate workspace ID if provided (for additional permission checking)
	if len(req.Msg.GetWorkspaceId()) > 0 {
		workspaceID, err := idwrap.NewFromBytes(req.Msg.GetWorkspaceId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		
		rpcErr := permcheck.CheckPerm(CheckOwnerWorkspace(ctx, c.us, workspaceID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		// Verify collection belongs to the specified workspace
		collectionWorkspaceID, err := c.cs.GetWorkspaceID(ctx, collectionID)
		if err != nil {
			if err == scollection.ErrNoCollectionFound {
				return nil, connect.NewError(connect.CodeNotFound, errors.New("collection not found"))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		
		if collectionWorkspaceID.Compare(workspaceID) != 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("collection does not belong to specified workspace"))
		}
	}

	// Validate position first (before checking permissions)
	position := req.Msg.GetPosition()
	if position == resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("position must be specified"))
	}

	// Validate target collection ID
	targetCollectionID, err := idwrap.NewFromBytes(req.Msg.GetTargetCollectionId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Prevent moving collection relative to itself (before checking permissions)
	if collectionID.Compare(targetCollectionID) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cannot move collection relative to itself"))
	}

	// Check permissions for target collection (must be in same workspace)
	rpcErr = permcheck.CheckPerm(CheckOwnerCollection(ctx, c.cs, c.us, targetCollectionID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Verify both collections are in the same workspace
	sourceWorkspaceID, err := c.cs.GetWorkspaceID(ctx, collectionID)
	if err != nil {
		if err == scollection.ErrNoCollectionFound {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("collection not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	targetWorkspaceID, err := c.cs.GetWorkspaceID(ctx, targetCollectionID)
	if err != nil {
		if err == scollection.ErrNoCollectionFound {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("target collection not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if sourceWorkspaceID.Compare(targetWorkspaceID) != 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("collections must be in the same workspace"))
	}

	// Execute the move operation
	switch position {
	case resourcesv1.MovePosition_MOVE_POSITION_AFTER:
		err = c.cs.MoveCollectionAfter(ctx, collectionID, targetCollectionID)
	case resourcesv1.MovePosition_MOVE_POSITION_BEFORE:
		err = c.cs.MoveCollectionBefore(ctx, collectionID, targetCollectionID)
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid position"))
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&collectionv1.CollectionMoveResponse{}), nil
}
