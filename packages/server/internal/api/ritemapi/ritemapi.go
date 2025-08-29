package ritemapi

import (
	"context"
	"database/sql"
	"errors"
	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/rcollection"
	"the-dev-tools/server/internal/api/ritemfolder"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/scollectionitem"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/translate/titemapi"
	endpointv1 "the-dev-tools/spec/dist/buf/go/collection/item/endpoint/v1"
	"the-dev-tools/spec/dist/buf/go/collection/item/endpoint/v1/endpointv1connect"

	"connectrpc.com/connect"
)

type ItemApiRPC struct {
	DB   *sql.DB
	ias  *sitemapi.ItemApiService
	ifs  *sitemfolder.ItemFolderService
	cs   *scollection.CollectionService
	us   *suser.UserService
	iaes *sitemapiexample.ItemApiExampleService
	ers *sexampleresp.ExampleRespService
	cis  *scollectionitem.CollectionItemService
}

func New(db *sql.DB, ias sitemapi.ItemApiService, cs scollection.CollectionService,
	ifs sitemfolder.ItemFolderService, us suser.UserService,
	iaes sitemapiexample.ItemApiExampleService,
	ers sexampleresp.ExampleRespService,
	cis *scollectionitem.CollectionItemService,
) ItemApiRPC {
	return ItemApiRPC{
		DB:   db,
		ias:  &ias,
		ifs:  &ifs,
		cs:   &cs,
		us:   &us,
		iaes: &iaes,
		ers: &ers,
		cis:  cis,
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
		Hidden:         msg.Hidden,
	}

	itemApiReq, err := titemapi.SeralizeRPCToModelWithoutID(endpointReq, collectionID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(rcollection.CheckOwnerCollection(ctx, *c.cs, *c.us, itemApiReq.CollectionID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	if itemApiReq.FolderID != nil {
		rpcErr := permcheck.CheckPerm(ritemfolder.CheckOwnerFolder(ctx, *c.ifs, *c.cs, *c.us, *itemApiReq.FolderID))
		if rpcErr != nil {
			return nil, rpcErr
		}
	}

	ID := idwrap.NewNow()
	itemApiReq.ID = ID

	exampleNanem := "Default"
	// Always create default examples, even for hidden endpoints (needed for CollectionItemList)
	isDefault := true
	example := &mitemapiexample.ItemApiExample{
		ID:           idwrap.NewNow(),
		ItemApiID:    itemApiReq.ID,
		CollectionID: itemApiReq.CollectionID,
		IsDefault:    isDefault,
		Name:         exampleNanem,
		BodyType:     mitemapiexample.BodyTypeNone,
	}

	rawBody := mbodyraw.ExampleBodyRaw{
		ID:        idwrap.NewNow(),
		ExampleID: example.ID,
	}

	// Convert legacy folder ID to collection_items folder ID if needed
	if itemApiReq.FolderID != nil {
		collectionItemsFolderID, err := c.cis.GetCollectionItemIDByLegacyID(ctx, *itemApiReq.FolderID)
		if err != nil {
			if err == scollectionitem.ErrCollectionItemNotFound {
				return nil, connect.NewError(connect.CodeNotFound, errors.New("parent folder not found"))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		itemApiReq.FolderID = &collectionItemsFolderID
	}

	tx, err := c.DB.Begin()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// Use CollectionItemService to create endpoint with unified ordering
	err = c.cis.CreateEndpointTX(ctx, tx, itemApiReq)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txIaes, err := sitemapiexample.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txRawBodyService, err := sbodyraw.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = txIaes.CreateApiExample(ctx, example)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = txRawBodyService.CreateBodyRaw(ctx, rawBody)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	respRaw := &endpointv1.EndpointCreateResponse{
		EndpointId: itemApiReq.ID.Bytes(),
		ExampleId:  example.ID.Bytes(),
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

	// Apply overrides from request if provided
	if req.Msg.Hidden != nil {
		api.Hidden = *req.Msg.Hidden
	}

	var exampleResps []mexampleresp.ExampleResp
	for i, v := range examples {
		resp, err := c.ers.GetExampleRespByExampleIDLatest(ctx, v.ID)
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
	defer devtoolsdb.TxnRollback(tx)

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

	if apiCall.FolderID != nil {
		checkfolder, err := c.ifs.GetFolder(ctx, *apiCall.FolderID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if checkfolder.CollectionID.Compare(apiCall.CollectionID) != 0 {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
		}
	}

	endpoint, err := c.ias.GetItemApi(ctx, apiCall.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	/*
		examples, err := c.iaes.GetApiExamplesWithDefaults(ctx, endpoint.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

			var changes []*changev1.Change
			if apiCall.Name != "" {
				endpoint.Name = apiCall.Name

				HistoryChangesService := "collection.item.example.v1"
				HistroyChangesMethod := "ExampleGet"
				for _, example := range examples {
					exampleVersionChangeKind := changev1.ChangeKind_CHANGE_KIND_INVALIDATE
					listRequest, err := anypb.New(&examplev1.ExampleGetRequest{
						ExampleId: example.ID.Bytes(),
					})
					if err != nil {
						return nil, connect.NewError(connect.CodeInternal, err)
					}
					changes = append(changes, &changev1.Change{
						Kind:    &exampleVersionChangeKind,
						Data:    listRequest,
						Service: &HistoryChangesService,
						Method:  &HistroyChangesMethod,
					})

				}
			}
	*/

	if apiCall.Method != "" {
		endpoint.Method = apiCall.Method
	}
	if apiCall.Url != "" {
		endpoint.Url = apiCall.Url
	}
	if apiCall.FolderID != nil {
		endpoint.FolderID = apiCall.FolderID
	}
	if apiCall.Name != "" {
		endpoint.Name = apiCall.Name
	}

	err = c.ias.UpdateItemApi(ctx, endpoint)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &endpointv1.EndpointUpdateResponse{}

	return connect.NewResponse(resp), nil
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

	endpoint, err := c.ias.GetItemApi(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	prev, next := endpoint.Prev, endpoint.Next
	var prevEndPointPtr, nextEndPointPtr *mitemapi.ItemApi
	if prev != nil {
		prevEndPointPtr, err = c.ias.GetItemApi(ctx, *prev)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("failed to get prev"))
		}
	}

	if next != nil {
		nextEndPointPtr, err = c.ias.GetItemApi(ctx, *next)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("failed to get next"))
		}
	}

	tx, err := c.DB.Begin()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	txias, err := sitemapi.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Before deletion, update the links
	if prevEndPointPtr != nil {
		prevEndPointPtr.Next = endpoint.Next // Point prev's next to current's next
		err = txias.UpdateItemApiOrder(ctx, prevEndPointPtr)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if nextEndPointPtr != nil {
		nextEndPointPtr.Prev = endpoint.Prev // Point next's prev to current's prev
		err = txias.UpdateItemApiOrder(ctx, nextEndPointPtr)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	err = txias.DeleteItemApi(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = tx.Commit()
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
	isOwner, err := rcollection.CheckOwnerCollection(ctx, cs, us, api.CollectionID)
	if err != nil {
		return false, err
	}
	return isOwner, nil
}
