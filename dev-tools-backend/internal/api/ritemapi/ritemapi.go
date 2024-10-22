package ritemapi

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/collection"
	"dev-tools-backend/internal/api/ritemfolder"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mitemapiexample"
	"dev-tools-backend/pkg/permcheck"
	"dev-tools-backend/pkg/service/scollection"
	"dev-tools-backend/pkg/service/sitemapi"
	"dev-tools-backend/pkg/service/sitemapiexample"
	"dev-tools-backend/pkg/service/sitemfolder"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/translate/titemapi"
	endpointv1 "dev-tools-spec/dist/buf/go/collection/item/endpoint/v1"
	"dev-tools-spec/dist/buf/go/collection/item/endpoint/v1/endpointv1connect"
	"errors"

	"connectrpc.com/connect"
)

type ItemApiRPC struct {
	DB   *sql.DB
	ias  *sitemapi.ItemApiService
	ifs  *sitemfolder.ItemFolderService
	cs   *scollection.CollectionService
	us   *suser.UserService
	iaes *sitemapiexample.ItemApiExampleService
}

func New(db *sql.DB, ias sitemapi.ItemApiService, cs scollection.CollectionService,
	ifs sitemfolder.ItemFolderService, us suser.UserService,
	iaes sitemapiexample.ItemApiExampleService,
) ItemApiRPC {
	return ItemApiRPC{
		DB:   db,
		ias:  &ias,
		ifs:  &ifs,
		cs:   &cs,
		us:   &us,
		iaes: &iaes,
	}
}

func CreateService(srv ItemApiRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := endpointv1connect.NewEndpointServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *ItemApiRPC) EndpointCreate(ctx context.Context, req *connect.Request[endpointv1.EndpointCreateRequest]) (*connect.Response[endpointv1.EndpointCreateResponse], error) {
	collectionID, err := idwrap.NewFromBytes(req.Msg.GetCollectionId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	msg := req.Msg
	endpointReq := &endpointv1.Endpoint{
		Name:           msg.GetName(),
		Method:         msg.GetMethod(),
		Url:            msg.GetUrl(),
		ParentFolderId: msg.GetParentFolderId(),
	}
	itemApiReq, err := titemapi.SeralizeRPCToModelWithoutID(endpointReq, collectionID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(collection.CheckOwnerCollection(ctx, *c.cs, *c.us, itemApiReq.CollectionID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	if itemApiReq.ParentID != nil {
		rpcErr := permcheck.CheckPerm(ritemfolder.CheckOwnerFolder(ctx, *c.ifs, *c.cs, *c.us, *itemApiReq.ParentID))
		if rpcErr != nil {
			return nil, rpcErr
		}
	}

	// TODO: add ordering it should append into end

	itemApiReq.ID = idwrap.NewNow()

	err = c.ias.CreateItemApi(ctx, itemApiReq)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	example := &mitemapiexample.ItemApiExample{
		ID:           idwrap.NewNow(),
		ItemApiID:    itemApiReq.ID,
		CollectionID: itemApiReq.CollectionID,
		IsDefault:    true,
		Name:         "Default",
		BodyType:     mitemapiexample.BodyTypeNone,
	}
	err = c.iaes.CreateApiExample(ctx, example)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	respRaw := &endpointv1.EndpointCreateResponse{
		EndpointId: itemApiReq.ID.Bytes(),
	}
	return connect.NewResponse(respRaw), nil
}

func (c *ItemApiRPC) EndpointDuplicate(ctx context.Context, req *connect.Request[endpointv1.EndpointDuplicateRequest]) (*connect.Response[endpointv1.EndpointDuplicateResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (c *ItemApiRPC) EndpointGet(ctx context.Context, req *connect.Request[endpointv1.EndpointGetRequest]) (*connect.Response[endpointv1.EndpointGetResponse], error) {
	apiUlid, err := idwrap.NewFromBytes(req.Msg.GetEndpointId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerApi(ctx, *c.ias, *c.cs, *c.us, apiUlid))
	if rpcErr != nil {
		return nil, rpcErr
	}

	api, err := c.ias.GetItemApi(ctx, apiUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	apiCall := titemapi.DeseralizeModelToRPC(api)
	resp := &endpointv1.EndpointGetResponse{
		EndpointId: apiCall.EndpointId,
		Name:       apiCall.Name,
		Method:     apiCall.Method,
		Url:        apiCall.Url,
	}
	return connect.NewResponse(resp), nil
}

func (c *ItemApiRPC) EndpointUpdate(ctx context.Context, req *connect.Request[endpointv1.EndpointUpdateRequest]) (*connect.Response[endpointv1.EndpointUpdateResponse], error) {
	endpointReq := &endpointv1.Endpoint{
		EndpointId:     req.Msg.GetEndpointId(),
		ParentFolderId: req.Msg.GetParentFolderId(),
		Name:           req.Msg.GetName(),
		Method:         req.Msg.GetMethod(),
		Url:            req.Msg.GetUrl(),
	}
	apiCall, err := titemapi.SeralizeRPCToModel(endpointReq, idwrap.IDWrap{})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerApi(ctx, *c.ias, *c.cs, *c.us, apiCall.ID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	if apiCall.ParentID != nil {
		checkfolder, err := c.ifs.GetFolder(ctx, *apiCall.ParentID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if checkfolder.CollectionID.Compare(apiCall.CollectionID) != 0 {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
		}
	}

	err = c.ias.UpdateItemApi(ctx, apiCall)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&endpointv1.EndpointUpdateResponse{}), nil
}

func (c *ItemApiRPC) EndpointDelete(ctx context.Context, req *connect.Request[endpointv1.EndpointDeleteRequest]) (*connect.Response[endpointv1.EndpointDeleteResponse], error) {
	id, err := idwrap.NewFromBytes(req.Msg.GetEndpointId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerApi(ctx, *c.ias, *c.cs, *c.us, id))
	if rpcErr != nil {
		return nil, rpcErr
	}

	err = c.ias.DeleteItemApi(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&endpointv1.EndpointDeleteResponse{}), nil
}

func CheckOwnerApi(ctx context.Context, ias sitemapi.ItemApiService, cs scollection.CollectionService, us suser.UserService, apiID idwrap.IDWrap) (bool, error) {
	api, err := ias.GetItemApi(ctx, apiID)
	if err != nil {
		return false, err
	}
	isOwner, err := collection.CheckOwnerCollection(ctx, cs, us, api.CollectionID)
	if err != nil {
		return false, err
	}
	return isOwner, nil
}
