package ritemapi

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/backend/internal/api"
	"the-dev-tools/backend/internal/api/collection"
	"the-dev-tools/backend/internal/api/ritemfolder"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mexampleresp"
	"the-dev-tools/backend/pkg/model/mitemapiexample"
	"the-dev-tools/backend/pkg/permcheck"
	"the-dev-tools/backend/pkg/service/scollection"
	"the-dev-tools/backend/pkg/service/sexampleresp"
	"the-dev-tools/backend/pkg/service/sitemapi"
	"the-dev-tools/backend/pkg/service/sitemapiexample"
	"the-dev-tools/backend/pkg/service/sitemfolder"
	"the-dev-tools/backend/pkg/service/suser"
	"the-dev-tools/backend/pkg/translate/titemapi"
	changev1 "the-dev-tools/spec/dist/buf/go/change/v1"
	endpointv1 "the-dev-tools/spec/dist/buf/go/collection/item/endpoint/v1"
	"the-dev-tools/spec/dist/buf/go/collection/item/endpoint/v1/endpointv1connect"
	itemv1 "the-dev-tools/spec/dist/buf/go/collection/item/v1"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/anypb"
)

type ItemApiRPC struct {
	DB   *sql.DB
	ias  *sitemapi.ItemApiService
	ifs  *sitemfolder.ItemFolderService
	cs   *scollection.CollectionService
	us   *suser.UserService
	iaes *sitemapiexample.ItemApiExampleService

	ers *sexampleresp.ExampleRespService
}

func New(db *sql.DB, ias sitemapi.ItemApiService, cs scollection.CollectionService,
	ifs sitemfolder.ItemFolderService, us suser.UserService,
	iaes sitemapiexample.ItemApiExampleService,
	ers sexampleresp.ExampleRespService,
) ItemApiRPC {
	return ItemApiRPC{
		DB:   db,
		ias:  &ias,
		ifs:  &ifs,
		cs:   &cs,
		us:   &us,
		iaes: &iaes,

		ers: &ers,
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

	ID := idwrap.NewNow()
	itemApiReq.ID = ID

	err = c.ias.CreateItemApi(ctx, itemApiReq)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	name := "Default"
	example := &mitemapiexample.ItemApiExample{
		ID:           idwrap.NewNow(),
		ItemApiID:    itemApiReq.ID,
		CollectionID: itemApiReq.CollectionID,
		IsDefault:    true,
		Name:         name,
		BodyType:     mitemapiexample.BodyTypeNone,
	}
	err = c.iaes.CreateApiExample(ctx, example)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	EndpointChange := itemv1.CollectionItem{
		Kind: itemv1.ItemKind_ITEM_KIND_FOLDER,
		Endpoint: &endpointv1.EndpointListItem{
			EndpointId:     ID.Bytes(),
			ParentFolderId: req.Msg.ParentFolderId,
			Name:           name,
		},
	}

	ChangeAny, err := anypb.New(&EndpointChange)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	a := &itemv1.CollectionItemListResponse{}

	changeAny, err := anypb.New(a)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	changeKind := changev1.ListChangeKind_LIST_CHANGE_KIND_APPEND

	listChanges := []*changev1.ListChange{
		{
			Kind:   changeKind,
			Parent: changeAny,
		},
	}

	kind := changev1.ChangeKind_CHANGE_KIND_UNSPECIFIED
	change := &changev1.Change{
		Kind: &kind,
		List: listChanges,
		Data: ChangeAny,
	}

	changes := []*changev1.Change{
		change,
	}

	respRaw := &endpointv1.EndpointCreateResponse{
		EndpointId: itemApiReq.ID.Bytes(),
		ExampleId:  example.ID.Bytes(),
		Changes:    changes,
	}

	return connect.NewResponse(respRaw), nil
}

func (c *ItemApiRPC) EndpointDuplicate(ctx context.Context, req *connect.Request[endpointv1.EndpointDuplicateRequest]) (*connect.Response[endpointv1.EndpointDuplicateResponse], error) {
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
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to get api"))
	}

	examples, err := c.iaes.GetApiExamples(ctx, api.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to get examples"))
	}

	defaultExample, err := c.iaes.GetDefaultApiExample(ctx, api.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to get default example"))
	}

	examples = append(examples, *defaultExample)

	api.ID = idwrap.NewNow()
	api.Name = api.Name + " Copy"

	var exampleResps []mexampleresp.ExampleResp
	for i, v := range examples {
		resp, err := c.ers.GetExampleRespByExampleID(ctx, v.ID)
		if err != nil {
			if err != sexampleresp.ErrNoRespFound {
				return nil, connect.NewError(connect.CodeInternal, errors.New("failed to get example response"))
			}
		}
		examples[i].ID = idwrap.NewNow()
		examples[i].ItemApiID = api.ID
		if resp != nil {
			resp.ID = idwrap.NewNow()
			resp.ExampleID = examples[i].ID
			exampleResps = append(exampleResps, *resp)
		}
	}

	tx, err := c.DB.Begin()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	iasTX, err := sitemapi.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	iaesTX, err := sitemapiexample.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	ersTX, err := sexampleresp.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = iasTX.CreateItemApi(ctx, api)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = iaesTX.CreateApiExampleBulk(ctx, examples)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, resp := range exampleResps {
		err = ersTX.CreateExampleResp(ctx, resp)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &endpointv1.EndpointDuplicateResponse{}
	return connect.NewResponse(resp), nil
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
