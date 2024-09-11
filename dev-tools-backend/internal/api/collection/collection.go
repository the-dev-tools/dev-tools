package collection

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/pkg/dbtime"
	"dev-tools-backend/pkg/model/mcollection"
	"dev-tools-backend/pkg/service/scollection"
	"dev-tools-backend/pkg/service/sheader"
	"dev-tools-backend/pkg/service/sitemapi"
	"dev-tools-backend/pkg/service/sitemapiexample"
	"dev-tools-backend/pkg/service/sitemfolder"
	"dev-tools-backend/pkg/service/sresultapi"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/service/sworkspace"
	"dev-tools-backend/pkg/translate/titemnest"
	"dev-tools-backend/pkg/translate/tpostman"
	collectionv1 "dev-tools-services/gen/collection/v1"
	"dev-tools-services/gen/collection/v1/collectionv1connect"
	"errors"
	"log"

	"connectrpc.com/connect"
	"github.com/oklog/ulid/v2"
)

type CollectionServiceRPC struct {
	DB     *sql.DB
	cs     scollection.CollectionService
	ws     sworkspace.WorkspaceService
	us     suser.UserService
	ias    sitemapi.ItemApiService
	ifs    sitemfolder.ItemFolderService
	ras    sresultapi.ResultApiService
	iaes   sitemapiexample.ItemApiExampleService
	hs     sheader.HeaderService
	secret []byte
}

type ContextKeyStr string

const (
	UserIDKeyCtx ContextKeyStr = "auth"
)

// ListCollections calls collection.v1.CollectionService.ListCollections.
func (c *CollectionServiceRPC) ListCollections(ctx context.Context, req *connect.Request[collectionv1.ListCollectionsRequest]) (*connect.Response[collectionv1.ListCollectionsResponse], error) {
	workspaceUlid, err := ulid.Parse(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := CheckOwnerWorkspace(ctx, c.us, workspaceUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no workspace found"))
	}

	org, err := c.ws.Get(ctx, workspaceUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	simpleCollections, err := c.cs.ListCollections(ctx, org.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	metaCollections := make([]*collectionv1.CollectionMeta, 0, len(simpleCollections))
	for _, collection := range simpleCollections {
		ulidID := collection.ID
		folderItems, err := c.ifs.GetFoldersWithCollectionID(ctx, ulidID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		apiItems, err := c.ias.GetApisWithCollectionID(ctx, ulidID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		apiExampleItems, err := c.iaes.GetApiExampleByCollection(ctx, ulidID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		pair, err := titemnest.TranslateItemFolderNested(folderItems, apiItems, apiExampleItems)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		items := pair.GetItemsMeta()

		metaCollections = append(metaCollections, &collectionv1.CollectionMeta{
			Id:    collection.ID.String(),
			Name:  collection.Name,
			Items: items,
		})
	}

	respRaw := &collectionv1.ListCollectionsResponse{
		MetaCollections: metaCollections,
	}
	return connect.NewResponse(respRaw), nil
}

// CreateCollection calls collection.v1.CollectionService.CreateCollection.
func (c *CollectionServiceRPC) CreateCollection(ctx context.Context, req *connect.Request[collectionv1.CreateCollectionRequest]) (*connect.Response[collectionv1.CreateCollectionResponse], error) {
	workspaceUlid, err := ulid.Parse(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := CheckOwnerWorkspace(ctx, c.us, workspaceUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		// INFO: don't send leaked information to the client
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no workspace found"))
	}
	name := req.Msg.GetName()
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is empty"))
	}
	ulidID := ulid.Make()
	dbTimeNow := dbtime.DBNow()
	collection := mcollection.Collection{
		ID:      ulidID,
		OwnerID: workspaceUlid,
		Name:    name,
		Updated: dbTimeNow,
	}
	err = c.cs.CreateCollection(ctx, &collection)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&collectionv1.CreateCollectionResponse{
		Id:   ulidID.String(),
		Name: name,
	}), nil
}

// GetCollection calls collection.v1.CollectionService.GetCollection.
func (c *CollectionServiceRPC) GetCollection(ctx context.Context, req *connect.Request[collectionv1.GetCollectionRequest]) (*connect.Response[collectionv1.GetCollectionResponse], error) {
	id := req.Msg.GetId()
	// convert id to ulid
	ulidID, err := ulid.Parse(id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := CheckOwnerCollection(ctx, c.cs, c.us, ulidID)
	if err != nil {
		log.Print("try to get collection error: ", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	collection, err := c.cs.GetCollection(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	folderItems, err := c.ifs.GetFoldersWithCollectionID(ctx, ulidID)
	if err != nil && err != sitemfolder.ErrNoItemFolderFound {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	apiItems, err := c.ias.GetApisWithCollectionID(ctx, ulidID)
	if err != nil && err != sitemapi.ErrNoItemApiFound {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	apiExampleItems, err := c.iaes.GetApiExampleByCollection(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	pair, err := titemnest.TranslateItemFolderNested(folderItems, apiItems, apiExampleItems)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	items := pair.GetItemsFull()

	respRaw := &collectionv1.GetCollectionResponse{
		Id:    collection.ID.String(),
		Name:  collection.Name,
		Items: items,
	}

	return connect.NewResponse(respRaw), nil
}

// UpdateCollection calls collection.v1.CollectionService.UpdateCollection.
func (c *CollectionServiceRPC) UpdateCollection(ctx context.Context, req *connect.Request[collectionv1.UpdateCollectionRequest]) (*connect.Response[collectionv1.UpdateCollectionResponse], error) {
	id := req.Msg.GetId()
	// convert id to ulid
	ulidID, err := ulid.Parse(id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := CheckOwnerCollection(ctx, c.cs, c.us, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	// TODO: can be merge with check
	collectionOld, err := c.cs.GetCollection(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	collection := mcollection.Collection{
		ID:      ulidID,
		Name:    req.Msg.GetName(),
		OwnerID: collectionOld.OwnerID,
	}
	err = c.cs.UpdateCollection(ctx, &collection)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&collectionv1.UpdateCollectionResponse{}), nil
}

// DeleteCollection calls collection.v1.CollectionService.DeleteCollection.
func (c *CollectionServiceRPC) DeleteCollection(ctx context.Context, req *connect.Request[collectionv1.DeleteCollectionRequest]) (*connect.Response[collectionv1.DeleteCollectionResponse], error) {
	id := req.Msg.GetId()
	// convert id
	wsUlid, err := ulid.Parse(id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := CheckOwnerCollection(ctx, c.cs, c.us, wsUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	err = c.cs.DeleteCollection(ctx, wsUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&collectionv1.DeleteCollectionResponse{}), nil
}

// ImportPostman calls collection.v1.CollectionService.ImportPostman.
func (c *CollectionServiceRPC) ImportPostman(ctx context.Context, req *connect.Request[collectionv1.ImportPostmanRequest]) (*connect.Response[collectionv1.ImportPostmanResponse], error) {
	wsUlid, err := ulid.Parse(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	isOwner, err := CheckOwnerWorkspace(ctx, c.us, wsUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	org, err := c.ws.Get(ctx, wsUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	postmanCollection, err := tpostman.ParsePostmanCollection(req.Msg.GetData())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	collectionUlid := ulid.Make()
	collection := mcollection.Collection{
		ID:      collectionUlid,
		Name:    req.Msg.GetName(),
		OwnerID: org.ID,
	}

	// TODO: add ownerID
	items, err := tpostman.ConvertPostmanCollection(postmanCollection, collectionUlid)
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

	txItemFolderService, err := sitemfolder.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txItemFolderService.CreateItemFolderBulk(ctx, items.Folder)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txItemApiService, err := sitemapi.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txItemApiService.CreateItemApiBulk(ctx, items.Api)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txItemApiExampleService, err := sitemapiexample.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = txItemApiExampleService.CreateApiExampleBulk(ctx, items.ApiExample)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = tx.Commit()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	tx, err = c.DB.Begin()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txHeaderService, err := sheader.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txHeaderService.CreateBulkHeader(ctx, items.Headers)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = tx.Commit()

	respRaw := &collectionv1.ImportPostmanResponse{
		Id: collectionUlid.String(),
	}
	resp := connect.NewResponse(respRaw)
	return resp, nil
}

func CreateService(ctx context.Context, db *sql.DB, secret []byte) (*api.Service, error) {
	collectionService, err := scollection.New(ctx, db)
	if err != nil {
		return nil, err
	}

	ias, err := sitemapi.New(ctx, db)
	if err != nil {
		return nil, err
	}

	ifs, err := sitemfolder.New(ctx, db)
	if err != nil {
		return nil, err
	}

	ws, err := sworkspace.New(ctx, db)
	if err != nil {
		return nil, err
	}

	us, err := suser.New(ctx, db)
	if err != nil {
		return nil, err
	}

	ras, err := sresultapi.New(ctx, db)
	if err != nil {
		return nil, err
	}

	iaes, err := sitemapiexample.New(ctx, db)
	if err != nil {
		return nil, err
	}

	hs, err := sheader.New(ctx, db)
	if err != nil {
		return nil, err
	}

	authInterceptor := mwauth.NewAuthInterceptor(secret)
	interceptors := connect.WithInterceptors(authInterceptor)
	server := &CollectionServiceRPC{
		DB:     db,
		secret: secret,
		cs:     *collectionService,
		ias:    *ias,
		ifs:    *ifs,
		ws:     *ws,
		us:     *us,
		ras:    *ras,
		iaes:   *iaes,
		hs:     *hs,
	}

	path, handler := collectionv1connect.NewCollectionServiceHandler(server, interceptors)
	return &api.Service{Path: path, Handler: handler}, nil
}

func CheckOwnerWorkspace(ctx context.Context, us suser.UserService, workspaceID ulid.ULID) (bool, error) {
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

func CheckOwnerCollection(ctx context.Context, cs scollection.CollectionService, us suser.UserService, collectionID ulid.ULID) (bool, error) {
	workspaceID, err := cs.GetOwner(ctx, collectionID)
	if err != nil {
		return false, connect.NewError(connect.CodeInternal, err)
	}

	return CheckOwnerWorkspace(ctx, us, workspaceID)
}
