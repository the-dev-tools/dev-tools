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

    // Note: Keep FolderID as legacy folder ID for item_api insertion.
    // The collection_items parent linkage will be resolved inside CreateEndpointTX.

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

	// Try to get default example, but don't fail if it doesn't exist
	// (for backwards compatibility with endpoints created before default examples were mandatory)
	defaultExample, err := c.iaes.GetDefaultApiExample(ctx, api.ID)
	if err == nil && defaultExample != nil {
		examples = append(examples, *defaultExample)
	}
	
	// If no examples at all (not even default), create a default example for the duplicate
	if len(examples) == 0 {
		newDefaultExample := mitemapiexample.ItemApiExample{
			ID:           idwrap.NewNow(),
			ItemApiID:    idwrap.NewNow(), // Will be updated to the new API ID below
			CollectionID: api.CollectionID,
			IsDefault:    true,
			Name:         "Default",
			BodyType:     mitemapiexample.BodyTypeNone,
		}
		examples = append(examples, newDefaultExample)
	}

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

	iaesTX, err := sitemapiexample.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	ersTX, err := sexampleresp.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Use CollectionItemService to create endpoint with unified ordering
	// This ensures the duplicated endpoint appears in CollectionItemList
	err = c.cis.CreateEndpointTX(ctx, tx, api)
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

	// Find the default example ID from the duplicated examples
	var defaultExampleID []byte
	for _, ex := range examples {
		if ex.IsDefault {
			defaultExampleID = ex.ID.Bytes()
			break
		}
	}
	// If no default example, use the first example
	if defaultExampleID == nil && len(examples) > 0 {
		defaultExampleID = examples[0].ID.Bytes()
	}

	resp := &endpointv1.EndpointDuplicateResponse{
		EndpointId: api.ID.Bytes(),
		ExampleId:  defaultExampleID,
	}
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

    // Read-only phase (outside transaction): validate and prefetch mapping
    if _, err := c.ias.GetItemApi(ctx, id); err != nil {
        return nil, connect.NewError(connect.CodeInternal, err)
    }
    // Prefetch collection_items mapping BEFORE opening a write transaction to avoid read-while-write locks in SQLite
    mappedItemID, mapErr := c.cis.GetCollectionItemIDByLegacyID(ctx, id)

	tx, err := c.DB.Begin()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

    txias, err := sitemapi.NewTX(ctx, tx)
    if err != nil {
        return nil, connect.NewError(connect.CodeInternal, err)
    }
    // Bind collection item service to the same transaction for all reads/writes
    txcis := c.cis.TX(tx)

    // Unlink from collection_items chain via unified safe delete and then delete the endpoint row
    // If mapping existed, perform unlink inside TX using the tx-bound service (no additional reads)
    if mapErr == nil {
        if derr := txcis.DeleteCollectionItem(ctx, tx, mappedItemID); derr != nil {
            return nil, connect.NewError(connect.CodeInternal, derr)
        }
    }

    // 3) Delete the endpoint itself
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
