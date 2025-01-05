package collection

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/backend/internal/api"
	"the-dev-tools/backend/internal/api/middleware/mwauth"
	"the-dev-tools/backend/internal/api/rworkspace"
	"the-dev-tools/backend/pkg/dbtime"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mcollection"
	"the-dev-tools/backend/pkg/permcheck"
	"the-dev-tools/backend/pkg/service/sbodyform"
	"the-dev-tools/backend/pkg/service/sbodyraw"
	"the-dev-tools/backend/pkg/service/sbodyurl"
	"the-dev-tools/backend/pkg/service/scollection"
	"the-dev-tools/backend/pkg/service/sexampleheader"
	"the-dev-tools/backend/pkg/service/sexamplequery"
	"the-dev-tools/backend/pkg/service/sitemapi"
	"the-dev-tools/backend/pkg/service/sitemapiexample"
	"the-dev-tools/backend/pkg/service/sitemfolder"
	"the-dev-tools/backend/pkg/service/suser"
	"the-dev-tools/backend/pkg/service/sworkspace"
	"the-dev-tools/backend/pkg/translate/tcollection"
	"the-dev-tools/backend/pkg/translate/tgeneric"
	"the-dev-tools/backend/pkg/translate/thar"
	"the-dev-tools/backend/pkg/translate/tpostman"
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

	org, err := c.ws.Get(ctx, workspaceUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	simpleCollections, err := c.cs.ListCollections(ctx, org.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	respRaw := &collectionv1.CollectionListResponse{
		Items: tgeneric.MassConvert(simpleCollections, tcollection.SerializeCollectionModelToRPC),
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

	err = c.cs.DeleteCollection(ctx, idWrap)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&collectionv1.CollectionDeleteResponse{}), nil
}

func (c *CollectionServiceRPC) CollectionImportPostman(ctx context.Context, req *connect.Request[collectionv1.CollectionImportPostmanRequest]) (*connect.Response[collectionv1.CollectionImportPostmanResponse], error) {
	wsUlid, err := idwrap.NewFromBytes(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerWorkspace(ctx, c.us, wsUlid))
	if rpcErr != nil {
		return nil, rpcErr
	}
	org, err := c.ws.Get(ctx, wsUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	postmanCollection, err := tpostman.ParsePostmanCollection(req.Msg.GetData())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	collectionidWrap := idwrap.NewNow()
	collection := mcollection.Collection{
		ID:      collectionidWrap,
		Name:    req.Msg.GetName(),
		OwnerID: org.ID,
	}

	items, err := tpostman.ConvertPostmanCollection(postmanCollection, collectionidWrap)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	tx, err := c.DB.Begin()
	defer tx.Rollback()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txCollectionService, err := scollection.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txCollectionService.CreateCollection(ctx, &collection)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txItemFolderService, err := sitemfolder.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txItemFolderService.CreateItemFolderBulk(ctx, items.Folders)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txItemApiService, err := sitemapi.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txItemApiService.CreateItemApiBulk(ctx, items.Apis)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txItemApiExampleService, err := sitemapiexample.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = txItemApiExampleService.CreateApiExampleBulk(ctx, items.ApiExamples)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// START BODY
	txBodyRawService, err := sbodyraw.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txBodyRawService.CreateBulkBodyRaw(ctx, items.BodyRaw)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txBodyFormService, err := sbodyform.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txBodyFormService.CreateBulkBodyForm(ctx, items.BodyForm)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txBodyUrlEncodedService, err := sbodyurl.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txBodyUrlEncodedService.CreateBulkBodyURLEncoded(ctx, items.BodyUrlEncoded)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	// END BODY

	txHeaderService, err := sexampleheader.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txHeaderService.CreateBulkHeader(ctx, items.Headers)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	txQueriesService, err := sexamplequery.NewTX(ctx, tx)
	if err != nil {
		return nil, err
	}
	err = txQueriesService.CreateBulkQuery(ctx, items.Queries)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	respRaw := &collectionv1.CollectionImportPostmanResponse{
		CollectionId: collectionidWrap.Bytes(),
	}
	resp := connect.NewResponse(respRaw)
	return resp, nil
}

func (c *CollectionServiceRPC) CollectionImportHar(ctx context.Context, req *connect.Request[collectionv1.CollectionImportHarRequest]) (*connect.Response[collectionv1.CollectionImportHarResponse], error) {
	wsID, err := idwrap.NewFromBytes(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, c.us, wsID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	harFileData := req.Msg.Data

	harData, err := thar.ConvertRaw(harFileData)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	collectionID := idwrap.NewNow()
	resolved, err := thar.ConvertHAR(harData, collectionID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	collectionData := mcollection.Collection{
		ID:      collectionID,
		Name:    req.Msg.GetName(),
		OwnerID: wsID,
	}

	tx, err := c.DB.Begin()
	defer tx.Rollback()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txCollectionService, err := scollection.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txCollectionService.CreateCollection(ctx, &collectionData)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txItemApiService, err := sitemapi.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for i := range resolved.Apis {
		resolved.Apis[i].CollectionID = collectionID
	}

	err = txItemApiService.CreateItemApiBulk(ctx, resolved.Apis)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txItemApiExampleService, err := sitemapiexample.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txItemApiExampleService.CreateApiExampleBulk(ctx, resolved.Examples)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txExampleHeaderService, err := sexampleheader.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txExampleHeaderService.CreateBulkHeader(ctx, resolved.Headers)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txExampleQueryService, err := sexamplequery.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txExampleQueryService.CreateBulkQuery(ctx, resolved.Queries)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txBodyRawService, err := sbodyraw.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txBodyRawService.CreateBulkBodyRaw(ctx, resolved.RawBodies)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txBodyFormService, err := sbodyform.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txBodyFormService.CreateBulkBodyForm(ctx, resolved.FormBodies)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txBodyUrlEncodedService, err := sbodyurl.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txBodyUrlEncodedService.CreateBulkBodyURLEncoded(ctx, resolved.UrlEncodedBodies)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &collectionv1.CollectionImportHarResponse{
		CollectionId: collectionID.Bytes(),
	}

	return connect.NewResponse(resp), nil
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
