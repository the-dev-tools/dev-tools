package collection

import (
	"context"
	"database/sql"
	"devtools-backend/internal/api"
	"devtools-backend/pkg/model/mcollection"
	"devtools-backend/pkg/model/mcollection/mitemapi"
	"devtools-backend/pkg/model/mcollection/mitemfolder"
	"devtools-backend/pkg/service/scollection"
	"devtools-backend/pkg/service/scollection/sitemapi"
	"devtools-backend/pkg/service/scollection/sitemfolder"
	"devtools-backend/pkg/translate/tpostman"
	collectionv1 "devtools-services/gen/collection/v1"
	"devtools-services/gen/collection/v1/collectionv1connect"
	nodedatav1 "devtools-services/gen/nodedata/v1"

	"connectrpc.com/connect"
	"github.com/oklog/ulid/v2"
)

type CollectionService struct {
	DB *sql.DB
}

// ListCollections calls collection.v1.CollectionService.ListCollections.
func (c *CollectionService) ListCollections(ctx context.Context, req *connect.Request[collectionv1.ListCollectionsRequest]) (*connect.Response[collectionv1.ListCollectionsResponse], error) {
	simpleCollections, err := scollection.ListCollections()
	if err != nil {
		return nil, err
	}

	metaCollections := make([]*collectionv1.CollectionMeta, 0, len(simpleCollections))
	for _, collection := range simpleCollections {
		metaCollections = append(metaCollections, &collectionv1.CollectionMeta{
			Id:   collection.ID.String(),
			Name: collection.Name,
		})
	}

	respRaw := &collectionv1.ListCollectionsResponse{
		MetaCollections: metaCollections,
	}
	return connect.NewResponse(respRaw), nil
}

// CreateCollection calls collection.v1.CollectionService.CreateCollection.
func (c *CollectionService) CreateCollection(ctx context.Context, req *connect.Request[collectionv1.CreateCollectionRequest]) (*connect.Response[collectionv1.CreateCollectionResponse], error) {
	ulidID := ulid.Make()
	collection := mcollection.Collection{
		ID:   ulidID,
		Name: req.Msg.Name,
	}
	err := scollection.CreateCollection(&collection)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&collectionv1.CreateCollectionResponse{
		Id:    ulidID.String(),
		Name:  req.Msg.Name,
		Items: []*collectionv1.Item{},
	}), nil
}

// GetCollection calls collection.v1.CollectionService.GetCollection.
func (c *CollectionService) GetCollection(ctx context.Context, req *connect.Request[collectionv1.GetCollectionRequest]) (*connect.Response[collectionv1.GetCollectionResponse], error) {
	id := req.Msg.Id
	// convert id to ulid
	ulidID, err := ulid.Parse(id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	collection, err := scollection.GetCollection(ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	items := make([]*collectionv1.Item, 0)
	apiItems, err := sitemapi.GetApisWithCollectionID(ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, item := range apiItems {
		apiItem := &collectionv1.Item{
			Data: &collectionv1.Item_ApiCall{
				ApiCall: &collectionv1.ApiCall{
					Meta: &collectionv1.ApiCallMeta{
						Name: item.Name,
						Id:   item.ID.String(),
					},
					CollectionId: item.CollectionID.String(),
					Data: &nodedatav1.NodeApiCallData{
						Url:         item.Url,
						Method:      item.Method,
						QueryParams: item.QueryParams.QueryMap,
						Headers:     item.Headers.HeaderMap,
					},
				},
			},
		}
		items = append(items, apiItem)
	}

	folderItems, err := sitemfolder.GetFoldersWithCollectionID(ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, item := range folderItems {
		folderItem := &collectionv1.Item{
			Data: &collectionv1.Item_Folder{
				Folder: &collectionv1.Folder{
					Meta: &collectionv1.FolderMeta{
						Id:   item.ID.String(),
						Name: item.Name,
					},
				},
			},
		}
		items = append(items, folderItem)
	}

	respRaw := &collectionv1.GetCollectionResponse{
		Id:    collection.ID.String(),
		Name:  collection.Name,
		Items: items,
	}
	resp := connect.NewResponse(respRaw)

	return resp, nil
}

// UpdateCollection calls collection.v1.CollectionService.UpdateCollection.
func (c *CollectionService) UpdateCollection(ctx context.Context, req *connect.Request[collectionv1.UpdateCollectionRequest]) (*connect.Response[collectionv1.UpdateCollectionResponse], error) {
	id := req.Msg.Id
	// convert id to ulid
	ulidID, err := ulid.Parse(id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	collection := mcollection.Collection{
		ID:   ulidID,
		Name: req.Msg.Name,
	}
	err = scollection.UpdateCollection(&collection)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&collectionv1.UpdateCollectionResponse{}), nil
}

// DeleteCollection calls collection.v1.CollectionService.DeleteCollection.
func (c *CollectionService) DeleteCollection(ctx context.Context, req *connect.Request[collectionv1.DeleteCollectionRequest]) (*connect.Response[collectionv1.DeleteCollectionResponse], error) {
	id := req.Msg.Id
	// convert id
	ulidID, err := ulid.Parse(id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	err = scollection.DeleteCollection(ulidID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&collectionv1.DeleteCollectionResponse{}), nil
}

// ImportPostman calls collection.v1.CollectionService.ImportPostman.
func (c *CollectionService) ImportPostman(ctx context.Context, req *connect.Request[collectionv1.ImportPostmanRequest]) (*connect.Response[collectionv1.ImportPostmanResponse], error) {
	postmanCollection, err := tpostman.ParsePostmanCollection(req.Msg.Data)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	ulidID := ulid.Make()
	collection := mcollection.Collection{
		ID:   ulidID,
		Name: req.Msg.Name,
	}
	err = scollection.CreateCollection(&collection)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// TODO: add ownerID
	items, err := tpostman.ConvertPostmanCollection(postmanCollection, ulidID, "")
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, folder := range items.Folder {
		err = sitemfolder.CreateItemFolder(&folder)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	for _, api := range items.Api {
		err = sitemapi.CreateItemApi(&api)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	respRaw := &collectionv1.ImportPostmanResponse{
		Id: ulidID.String(),
	}
	resp := connect.NewResponse(respRaw)
	return resp, nil
}

// CreateFolder calls collection.v1.CollectionService.CreateFolder.
func (c *CollectionService) CreateFolder(ctx context.Context, req *connect.Request[collectionv1.CreateFolderRequest]) (*connect.Response[collectionv1.CreateFolderResponse], error) {
	ulidID := ulid.Make()
	collectionUlidID, err := ulid.Parse(req.Msg.CollectionId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// TODO: parentID
	folder := mitemfolder.ItemFolder{
		ID:           ulidID,
		CollectionID: collectionUlidID,
		Name:         req.Msg.Name,
	}
	err = sitemfolder.CreateItemFolder(&folder)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	respRaw := &collectionv1.CreateFolderResponse{
		Id:   ulidID.String(),
		Name: req.Msg.Name,
	}
	resp := connect.NewResponse(respRaw)
	return resp, nil
}

// CreateApiCall calls collection.v1.CollectionService.CreateApiCall.
func (c *CollectionService) CreateApiCall(ctx context.Context, req *connect.Request[collectionv1.CreateApiCallRequest]) (*connect.Response[collectionv1.CreateApiCallResponse], error) {
	ulidID := ulid.Make()
	collectionUlidID, err := ulid.Parse(req.Msg.CollectionId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	apiCall := &mitemapi.ItemApi{
		ID:           ulidID,
		CollectionID: collectionUlidID,
		Name:         req.Msg.Name,
		// TODO: ParentID:
	}
	err = sitemapi.CreateItemApi(apiCall)
	if err != nil {
		return nil, err
	}

	respRaw := &collectionv1.CreateApiCallResponse{
		Id: ulidID.String(),
	}
	resp := connect.NewResponse(respRaw)
	return resp, nil
}

// GetFolder calls collection.v1.CollectionService.GetFolder.
func (c *CollectionService) GetFolder(ctx context.Context, req *connect.Request[collectionv1.GetFolderRequest]) (*connect.Response[collectionv1.GetFolderResponse], error) {
	ulidID, err := ulid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	folder, err := sitemfolder.GetItemFolder(ulidID)
	if err != nil {
		return nil, err
	}

	respRaw := &collectionv1.GetFolderResponse{
		Folder: &collectionv1.Folder{
			Meta: &collectionv1.FolderMeta{
				Id:   folder.ID.String(),
				Name: folder.Name,
			},
			Items: []*collectionv1.Item{},
		},
	}

	return connect.NewResponse(respRaw), nil
}

// GetApiCall calls collection.v1.CollectionService.GetApiCall.
func (c *CollectionService) GetApiCall(ctx context.Context, req *connect.Request[collectionv1.GetApiCallRequest]) (*connect.Response[collectionv1.GetApiCallResponse], error) {
	ulidID, err := ulid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	item, err := sitemapi.GetItemApi(ulidID)
	if err != nil {
		return nil, err
	}

	respRaw := &collectionv1.GetApiCallResponse{
		ApiCall: &collectionv1.ApiCall{
			Meta: &collectionv1.ApiCallMeta{
				Id:   item.ID.String(),
				Name: item.Name,
			},
			CollectionId: item.CollectionID.String(),
			ParentId:     item.ParentID.String(),
			Data: &nodedatav1.NodeApiCallData{
				Url:         item.Url,
				Method:      item.Method,
				QueryParams: item.QueryParams.QueryMap,
				Headers:     item.Headers.HeaderMap,
			},
		},
	}

	return connect.NewResponse(respRaw), nil
}

// UpdateFolder calls collection.v1.CollectionService.UpdateFolder.
func (c *CollectionService) UpdateFolder(ctx context.Context, req *connect.Request[collectionv1.UpdateFolderRequest]) (*connect.Response[collectionv1.UpdateFolderResponse], error) {
	ulidID, err := ulid.Parse(req.Msg.Folder.Meta.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	collectionId, err := ulid.Parse(req.Msg.Folder.CollectionId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	var parentUlidID *ulid.ULID
	if req.Msg.Folder.ParentId == "" {
		parentUlidID = nil
	} else {
		tempParentUlidID, err := ulid.Parse(req.Msg.Folder.ParentId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		parentUlidID = &tempParentUlidID
	}

	folder := mitemfolder.ItemFolder{
		ID:           ulidID,
		CollectionID: collectionId,
		ParentID:     parentUlidID,
		Name:         req.Msg.Folder.Meta.Name,
	}

	sitemfolder.UpdateItemFolder(&folder)

	// sitemfolder.UpdateItemFolder(folder * mitemfolder.ItemFolder)
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

// UpdateApiCall calls collection.v1.CollectionService.UpdateApiCall.
func (c *CollectionService) UpdateApiCall(ctx context.Context, req *connect.Request[collectionv1.UpdateApiCallRequest]) (*connect.Response[collectionv1.UpdateApiCallResponse], error) {
	ulidID, err := ulid.Parse(req.Msg.ApiCall.Meta.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	ulidCollectionID, err := ulid.Parse(req.Msg.ApiCall.CollectionId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	itemApi := &mitemapi.ItemApi{
		ID:           ulidID,
		CollectionID: ulidCollectionID,
		Name:         req.Msg.ApiCall.Meta.Name,
		Url:          req.Msg.ApiCall.Data.Url,
		Method:       req.Msg.ApiCall.Data.Method,
		Headers:      mitemapi.Headers{HeaderMap: req.Msg.ApiCall.Data.Headers},
		QueryParams:  mitemapi.QueryParams{QueryMap: req.Msg.ApiCall.Data.QueryParams},
		Body:         req.Msg.ApiCall.Data.Body,
	}

	err = sitemapi.UpdateItemApi(itemApi)
	if err != nil {
		return nil, err
	}

	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

// DeleteFolder calls collection.v1.CollectionService.DeleteFolder.
func (c *CollectionService) DeleteFolder(ctx context.Context, req *connect.Request[collectionv1.DeleteFolderRequest]) (*connect.Response[collectionv1.DeleteFolderResponse], error) {
	ulidID, err := ulid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	err = sitemfolder.DeleteItemFolder(ulidID)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&collectionv1.DeleteFolderResponse{}), nil
}

// DeleteApiCall calls collection.v1.CollectionService.DeleteApiCall.
func (c *CollectionService) DeleteApiCall(ctx context.Context, req *connect.Request[collectionv1.DeleteApiCallRequest]) (*connect.Response[collectionv1.DeleteApiCallResponse], error) {
	ulidID, err := ulid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	err = sitemapi.DeleteItemApi(ulidID)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&collectionv1.DeleteApiCallResponse{}), nil
}

// RunApiCall calls collection.v1.CollectionService.RunApiCall.
func (c *CollectionService) RunApiCall(ctx context.Context, req *connect.Request[collectionv1.RunApiCallRequest]) (*connect.Response[collectionv1.RunApiCallResponse], error) {
	// TODO: implement
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func CreateService(db *sql.DB) (*api.Service, error) {
	server := &CollectionService{
		DB: db,
	}
	path, handler := collectionv1connect.NewCollectionServiceHandler(server)
	return &api.Service{Path: path, Handler: handler}, nil
}
