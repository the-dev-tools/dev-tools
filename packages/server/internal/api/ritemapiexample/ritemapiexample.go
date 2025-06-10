package ritemapiexample

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/rcollection"
	"the-dev-tools/server/internal/api/ritemapi"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/http/request"
	"the-dev-tools/server/pkg/http/response"
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logconsole"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/massertres"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mexamplerespheader"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/reference"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/sassertres"
	"the-dev-tools/server/pkg/service/sbodyform"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/sbodyurl"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sexamplerespheader"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/translate/tbreadcrumbs"
	"the-dev-tools/server/pkg/translate/texample"
	"the-dev-tools/server/pkg/translate/texampleresp"
	"the-dev-tools/server/pkg/translate/texampleversion"
	"the-dev-tools/server/pkg/translate/tgeneric"
	"the-dev-tools/server/pkg/varsystem"
	examplev1 "the-dev-tools/spec/dist/buf/go/collection/item/example/v1"
	"the-dev-tools/spec/dist/buf/go/collection/item/example/v1/examplev1connect"

	"connectrpc.com/connect"
)

type ItemAPIExampleRPC struct {
	DB   *sql.DB
	iaes *sitemapiexample.ItemApiExampleService
	ias  *sitemapi.ItemApiService
	ifs  *sitemfolder.ItemFolderService

	ws *sworkspace.WorkspaceService
	cs *scollection.CollectionService
	us *suser.UserService
	// sub
	hs   *sexampleheader.HeaderService
	qs   *sexamplequery.ExampleQueryService
	bfs  *sbodyform.BodyFormService
	bues *sbodyurl.BodyURLEncodedService
	brs  *sbodyraw.BodyRawService
	// resp sub
	erhs *sexamplerespheader.ExampleRespHeaderService
	ers  *sexampleresp.ExampleRespService
	// env
	es senv.EnvService
	vs svar.VarService

	// assert
	as  *sassert.AssertService
	ars *sassertres.AssertResultService

	logChanMap logconsole.LogChanMap
}

func New(db *sql.DB, iaes sitemapiexample.ItemApiExampleService, ias sitemapi.ItemApiService, ifs sitemfolder.ItemFolderService,
	ws sworkspace.WorkspaceService, cs scollection.CollectionService, us suser.UserService, hs sexampleheader.HeaderService, qs sexamplequery.ExampleQueryService,
	bfs sbodyform.BodyFormService, beus sbodyurl.BodyURLEncodedService, brs sbodyraw.BodyRawService, erhs sexamplerespheader.ExampleRespHeaderService,
	ers sexampleresp.ExampleRespService, es senv.EnvService, vs svar.VarService, as sassert.AssertService, ars sassertres.AssertResultService,
	logChanMap logconsole.LogChanMap,
) ItemAPIExampleRPC {
	return ItemAPIExampleRPC{
		DB:         db,
		iaes:       &iaes,
		ias:        &ias,
		ifs:        &ifs,
		ws:         &ws,
		cs:         &cs,
		us:         &us,
		hs:         &hs,
		qs:         &qs,
		bfs:        &bfs,
		bues:       &beus,
		brs:        &brs,
		erhs:       &erhs,
		ers:        &ers,
		es:         es,
		vs:         vs,
		as:         &as,
		ars:        &ars,
		logChanMap: logChanMap,
	}
}

func CreateService(srv ItemAPIExampleRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := examplev1connect.NewExampleServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *ItemAPIExampleRPC) ExampleList(ctx context.Context, req *connect.Request[examplev1.ExampleListRequest]) (*connect.Response[examplev1.ExampleListResponse], error) {
	apiUlid, err := idwrap.NewFromBytes(req.Msg.GetEndpointId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid item api id"))
	}

	rpcErr := permcheck.CheckPerm(ritemapi.CheckOwnerApi(ctx, *c.ias, *c.cs, *c.us, apiUlid))
	if rpcErr != nil {
		return nil, rpcErr
	}

	examples, err := c.iaes.GetApiExamples(ctx, apiUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var respsRpc []*examplev1.ExampleListItem
	for _, example := range examples {
		exampleResp, err := c.ers.GetExampleRespByExampleIDLatest(ctx, example.ID)
		var exampleRespID *idwrap.IDWrap
		if err != nil {
			if err != sql.ErrNoRows {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		} else {
			exampleRespID = &exampleResp.ID
		}

		respsRpc = append(respsRpc, texample.SerializeModelToRPCItem(example, exampleRespID))

	}
	resp := &examplev1.ExampleListResponse{
		EndpointId: apiUlid.Bytes(),
		Items:      respsRpc,
	}

	return connect.NewResponse(resp), nil
}

func (c *ItemAPIExampleRPC) ExampleGet(ctx context.Context, req *connect.Request[examplev1.ExampleGetRequest]) (*connect.Response[examplev1.ExampleGetResponse], error) {
	exampleIdWrap, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid item api id"))
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerExample(ctx, *c.iaes, *c.cs, *c.us, exampleIdWrap))
	if rpcErr != nil {
		return nil, rpcErr
	}

	example, err := c.iaes.GetApiExample(ctx, exampleIdWrap)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	exampleBreadcrumbs, err := c.iaes.GetExampleAllParents(ctx, exampleIdWrap, *c.cs, *c.ifs, *c.ias)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// TODO: this can fail fix this
	var respIdPtr *idwrap.IDWrap
	exampleResp, err := c.ers.GetExampleRespByExampleIDLatest(ctx, exampleIdWrap)
	if err != nil && err != sexampleresp.ErrNoRespFound {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if exampleResp != nil {
		respIdPtr = &exampleResp.ID
	}

	rpcBreadcrumbs := tgeneric.MassConvert(exampleBreadcrumbs, tbreadcrumbs.SerializeModelToRPC)

	rpcExample := texample.SerializeModelToRPC(*example, respIdPtr, rpcBreadcrumbs)
	resp := &examplev1.ExampleGetResponse{
		ExampleId:      rpcExample.ExampleId,
		LastResponseId: rpcExample.LastResponseId,
		Name:           rpcExample.Name,
		Breadcrumbs:    rpcExample.Breadcrumbs,
		BodyKind:       rpcExample.BodyKind,
	}
	return connect.NewResponse(resp), nil
}

func (c *ItemAPIExampleRPC) ExampleCreate(ctx context.Context, req *connect.Request[examplev1.ExampleCreateRequest]) (*connect.Response[examplev1.ExampleCreateResponse], error) {
	apiIDWrap, err := idwrap.NewFromBytes(req.Msg.GetEndpointId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid item api id"))
	}
	ok, err := ritemapi.CheckOwnerApi(ctx, *c.ias, *c.cs, *c.us, apiIDWrap)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("not found api"))
	}

	itemApi, err := c.ias.GetItemApi(ctx, apiIDWrap)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// TODO: make this a transaction
	ExampleID := idwrap.NewNow()
	ex := &mitemapiexample.ItemApiExample{
		ID:           ExampleID,
		ItemApiID:    apiIDWrap,
		CollectionID: itemApi.CollectionID,
		Name:         req.Msg.Name,
		BodyType:     mitemapiexample.BodyTypeNone,
		IsDefault:    false,
	}
	err = c.iaes.CreateApiExample(ctx, ex)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	bodyRaw := mbodyraw.ExampleBodyRaw{
		ID:            idwrap.NewNow(),
		ExampleID:     ExampleID,
		VisualizeMode: mbodyraw.VisualizeModeBinary,
		CompressType:  compress.CompressTypeNone,
		Data:          []byte{},
	}

	err = c.brs.CreateBodyRaw(ctx, bodyRaw)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// TODO: refactor changes stuff
	/*
		folderChange := itemv1.CollectionItem{
			Kind: itemv1.ItemKind_ITEM_KIND_FOLDER,
			Example: &examplev1.ExampleListItem{
				ExampleId: ExampleID.Bytes(),
				Name:      req.Msg.Name,
			},
		}

			folderChangeAny, err := anypb.New(&folderChange)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			a := &examplev1.ExampleListResponse{
				EndpointId: ExampleID.Bytes(),
			}

			changeAny, err := anypb.New(a)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
	*/

	/*
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
			Data: folderChangeAny,
		}

		changes := []*changev1.Change{
			change,
		}
	*/

	return connect.NewResponse(&examplev1.ExampleCreateResponse{
		ExampleId: ExampleID.Bytes(),
	}), nil
}

func (c *ItemAPIExampleRPC) ExampleUpdate(ctx context.Context, req *connect.Request[examplev1.ExampleUpdateRequest]) (*connect.Response[examplev1.ExampleUpdateResponse], error) {

	exampleIDWrap, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid item api id"))
	}

	isMember, err := CheckOwnerExample(ctx, *c.iaes, *c.cs, *c.us, exampleIDWrap)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isMember {
		// return not found
		return nil, connect.NewError(connect.CodeNotFound, errors.New("not found example"))
	}

	dbExample, err := c.iaes.GetApiExample(ctx, exampleIDWrap)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// var changes []*changev1.Change

	var updateChange bool
	exRPC := req.Msg
	if exRPC.Name != nil {
		dbExample.Name = *exRPC.Name
		updateChange = true

		/*
			folderChange := itemv1.CollectionItem{
				Kind: itemv1.ItemKind_ITEM_KIND_FOLDER,
				Example: &examplev1.ExampleListItem{
					Name: dbExample.Name,
				},
			}

				folderChangeAny, err := anypb.New(&folderChange)
				if err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}

					kind := changev1.ChangeKind_CHANGE_KIND_UPDATE
					normalizationChange := &changev1.Change{
						Kind: &kind,
						Data: folderChangeAny,
					}
					changes = append(changes, normalizationChange)
		*/
	}
	if exRPC.BodyKind != nil {
		dbExample.BodyType = mitemapiexample.BodyType(*exRPC.BodyKind)
		updateChange = true
	}

	if !updateChange {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("all fields are null"))
	}

	err = c.iaes.UpdateItemApiExample(ctx, dbExample)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&examplev1.ExampleUpdateResponse{}), nil
}

func (c *ItemAPIExampleRPC) ExampleDelete(ctx context.Context, req *connect.Request[examplev1.ExampleDeleteRequest]) (*connect.Response[examplev1.ExampleDeleteResponse], error) {
	exampleUlid, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid item api id"))
	}

	isMember, err := CheckOwnerExample(ctx, *c.iaes, *c.cs, *c.us, exampleUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isMember {
		// return not found
		return nil, connect.NewError(connect.CodeNotFound, errors.New("not found example"))
	}

	example, err := c.iaes.GetApiExample(ctx, exampleUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	prevID, nextID := example.Prev, example.Next
	var prevExamplePtr, nextExamplePtr *mitemapiexample.ItemApiExample
	if prevID != nil {
		prevExamplePtr, err = c.iaes.GetApiExample(ctx, *prevID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	if nextID != nil {
		nextExamplePtr, err = c.iaes.GetApiExample(ctx, *nextID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	tx, err := c.DB.Begin()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	txIfs, err := sitemapiexample.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if prevExamplePtr != nil {
		err = txIfs.UpdateItemApiExampleOrder(ctx, prevExamplePtr)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	if nextExamplePtr != nil {
		err = txIfs.UpdateItemApiExampleOrder(ctx, nextExamplePtr)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	err = txIfs.DeleteApiExample(ctx, exampleUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	/*

		a := &examplev1.ExampleChange{
			ExampleId: exampleUlid.Bytes(),
		}

		changeAny, err := anypb.New(a)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		changeKind := changev1.ChangeKind_CHANGE_KIND_DELETE

		changes := []*changev1.Change{
			{
				Kind: &changeKind,
				Data: changeAny,
			},
		}
	*/

	resp := &examplev1.ExampleDeleteResponse{}

	return connect.NewResponse(resp), nil
}

func (c *ItemAPIExampleRPC) ExampleDuplicate(ctx context.Context, req *connect.Request[examplev1.ExampleDuplicateRequest]) (*connect.Response[examplev1.ExampleDuplicateResponse], error) {
	exampleUlid, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid item api id"))
	}

	isMember, err := CheckOwnerExample(ctx, *c.iaes, *c.cs, *c.us, exampleUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isMember {
		// return not found
		return nil, connect.NewError(connect.CodeNotFound, errors.New("not found example"))
	}

	example, err := c.iaes.GetApiExample(ctx, exampleUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res, err := PrepareCopyExample(ctx, example.ItemApiID, *example, *c.hs, *c.qs, *c.brs, *c.bfs, *c.bues, *c.ers, *c.erhs, *c.as, *c.ars)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	exampleIDBytes := res.Example.ID.Bytes()
	//	exampleName := res.Example.Name
	tx, err := c.DB.Begin()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = CreateCopyExample(ctx, tx, res)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	/*

		// TODO: refactor changes stuff
		folderChange := itemv1.CollectionItem{
			Kind: itemv1.ItemKind_ITEM_KIND_FOLDER,
			Example: &examplev1.ExampleListItem{
				ExampleId: exampleIDBytes,
				Name:      exampleName,
			},
		}

		folderChangeAny, err := anypb.New(&folderChange)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		a := &examplev1.ExampleListResponse{
			EndpointId: exampleIDBytes,
		}

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
			Data: folderChangeAny,
		}

		changes := []*changev1.Change{
			change,
		}
	*/

	resp := &examplev1.ExampleDuplicateResponse{
		ExampleId: exampleIDBytes,
	}

	return connect.NewResponse(resp), nil
}

type ExampleRunLog struct {
	Request  request.RequestResponseVar `json:"request"`
	Response httpclient.ResponseVar     `json:"response"`
}

func (c *ItemAPIExampleRPC) ExampleRun(ctx context.Context, req *connect.Request[examplev1.ExampleRunRequest]) (*connect.Response[examplev1.ExampleRunResponse], error) {
	exampleUlid, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if rpcErr := permcheck.CheckPerm(CheckOwnerExample(ctx, *c.iaes, *c.cs, *c.us, exampleUlid)); rpcErr != nil {
		return nil, rpcErr
	}

	example, err := c.iaes.GetApiExample(ctx, exampleUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	itemApiCall, err := c.ias.GetItemApi(ctx, example.ItemApiID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	reqQueries, err := c.qs.GetExampleQueriesByExampleID(ctx, exampleUlid)
	if err != nil && err != sexamplequery.ErrNoQueryFound {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	collection, err := c.cs.GetCollection(ctx, example.CollectionID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	workspaceID := collection.WorkspaceID

	workspace, err := c.ws.Get(ctx, workspaceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	selectedEnv, err := c.es.Get(ctx, workspace.ActiveEnv)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	globalEnv, err := c.es.Get(ctx, workspace.GlobalEnv)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var varMap *varsystem.VarMap
	if selectedEnv != nil {
		currentVars, err := c.vs.GetVariableByEnvID(ctx, selectedEnv.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		globalVars, err := c.vs.GetVariableByEnvID(ctx, globalEnv.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		tempVarMap := varsystem.NewVarMap(varsystem.MergeVars(globalVars, currentVars))
		varMap = &tempVarMap
	}

	if varsystem.CheckStringHasAnyVarKey(itemApiCall.Url) {
		itemApiCall.Url, err = varMap.ReplaceVars(itemApiCall.Url)
		if err != nil {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
	}

	reqHeaders, err := c.hs.GetHeaderByExampleID(ctx, exampleUlid)
	if err != nil && err != sexampleheader.ErrNoHeaderFound {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rawBody, err := c.brs.GetBodyRawByExampleID(ctx, exampleUlid)
	if err != nil {
		if err == sbodyraw.ErrNoBodyRawFound {

			tempBodyRaw := mbodyraw.ExampleBodyRaw{
				ID:            idwrap.NewNow(),
				ExampleID:     exampleUlid,
				VisualizeMode: mbodyraw.VisualizeModeBinary,
				CompressType:  compress.CompressTypeNone,
				Data:          []byte{},
			}

			err = c.brs.CreateBodyRaw(ctx, tempBodyRaw)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			rawBody = &tempBodyRaw
		} else {
			return nil, err
		}
	}

	formBody, err := c.bfs.GetBodyFormsByExampleID(ctx, exampleUlid)
	if err != nil {
		if err == sbodyform.ErrNoBodyFormFound {
			formBody = []mbodyform.BodyForm{}
		} else {
			return nil, err
		}
	}

	urlBody, err := c.bues.GetBodyURLEncodedByExampleID(ctx, exampleUlid)
	if err != nil {
		if err == sbodyurl.ErrNoBodyUrlEncodedFound {
			urlBody = []mbodyurl.BodyURLEncoded{}
		} else {
			return nil, err
		}
	}

	client := httpclient.New()
	preparedRequest, err := request.PrepareRequest(*itemApiCall, *example, reqQueries, reqHeaders, *rawBody, formBody, urlBody, *varMap)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare request: %w", err)
	}

	requestResp, err := request.SendRequest(preparedRequest, example.ID, client)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// TODO: simplify this package deps
	exampleRunLog := ExampleRunLog{
		Request:  request.ConvertRequestToVar(preparedRequest),
		Response: httpclient.ConvertResponseToVar(requestResp.HttpResp),
	}

	ref := reference.NewReferenceFromInterfaceWithKey(exampleRunLog, example.Name)
	refs := []reference.ReferenceTreeItem{ref}

	err = c.logChanMap.SendMsgToUserWithContext(ctx, idwrap.NewNow(), fmt.Sprintf("Request %s:%s", example.Name, example.ID.String()), logconsole.LogLevelUnspecified, refs)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	exampleResp := mexampleresp.ExampleResp{
		ID:        idwrap.NewNow(),
		ExampleID: exampleUlid,
	}

	if err := c.ers.CreateExampleResp(ctx, exampleResp); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	currentRespHeaders, err := c.erhs.GetHeaderByRespID(ctx, exampleResp.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	assertions, err := c.as.GetAssertByExampleID(ctx, example.ID)
	if err != nil && err != sassert.ErrNoAssertFound {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	responseOutput, err := response.ResponseCreate(ctx, *requestResp, exampleResp, currentRespHeaders, assertions)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	exampleResp = responseOutput.ExampleResp

	err = c.ers.UpdateExampleResp(ctx, responseOutput.ExampleResp)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// var assertResults []massertres.AssertResult
	// for _, assertion := range responseOutput.AssertCouples {
	// 	assertResults = append(assertResults, assertion.AssertRes)
	// }

	taskCreateHeaders := responseOutput.CreateHeaders
	taskUpdateHeaders := responseOutput.UpdateHeaders
	taskDeleteHeaders := responseOutput.DeleteHeaderIds

	fullHeaders := append(taskCreateHeaders, taskUpdateHeaders...)
	if len(fullHeaders) > 0 || len(taskDeleteHeaders) > 0 {
		tx, err := c.DB.Begin()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		erhsTx, err := sexamplerespheader.NewTX(ctx, tx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if len(taskCreateHeaders) > 0 {
			if err := erhsTx.CreateExampleRespHeaderBulk(ctx, taskCreateHeaders); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
		if len(taskUpdateHeaders) > 0 {
			if err := erhsTx.UpdateExampleRespHeaderBulk(ctx, taskUpdateHeaders); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
		if len(taskDeleteHeaders) > 0 {
			if err := erhsTx.DeleteExampleRespHeaderBulk(ctx, taskDeleteHeaders); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
		if err := tx.Commit(); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	for _, assertionCouple := range responseOutput.AssertCouples {
		if _, err := c.ars.GetAssertResult(ctx, assertionCouple.AssertRes.ID); err != nil {
			if err == sql.ErrNoRows {
				if err := c.ars.CreateAssertResult(ctx, assertionCouple.AssertRes); err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
			}
		} else {
			if err := c.ars.UpdateAssertResult(ctx, assertionCouple.AssertRes); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
	}

	// Copy Full Item Api/Endpoint
	// TODO: make this transaction
	endpoint, err := c.ias.GetItemApi(ctx, example.ItemApiID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	endpoint.VersionParentID = &endpoint.ID
	endpointNewID := idwrap.NewNow()
	endpoint.ID = endpointNewID

	err = c.ias.CreateItemApi(ctx, endpoint)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to copy endpoint"))
	}

	// exampleVersionID := example.ID
	example.VersionParentID = &example.ID

	res, err := PrepareCopyExample(ctx, endpointNewID, *example, *c.hs, *c.qs, *c.brs, *c.bfs, *c.bues, *c.ers, *c.erhs, *c.as, *c.ars)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to copy example: %w", err))
	}

	tx, err := c.DB.Begin()
	if err != nil {
		return nil, err
	}

	err = CreateCopyExample(ctx, tx, res)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to copy example: %w", err))
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	/*

		var changes []*changev1.Change
		if isExampleRespExists {
			exampleResp.Body = responseOutput.BodyRaw
			changes, err = HandleResponseUpdate(exampleResp, assertions, assertResults, requestResp.HttpResp.Headers)
		} else {
			changes, err = HandleResponseCreate(example.ID, exampleResp.ID)
		}
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		exampleVersionRequest, err := anypb.New(&examplev1.ExampleVersionsRequest{
			ExampleId: exampleVersionID.Bytes(),
		})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

			HistoryChangesService := "collection.item.example.v1.ExampleService"
			HistroyChangesMethod := "ExampleVersions"
			exampleVersionChangeKind := changev1.ChangeKind_CHANGE_KIND_INVALIDATE
			changes = append(changes, &changev1.Change{
				Kind:    &exampleVersionChangeKind,
				Data:    exampleVersionRequest,
				Service: &HistoryChangesService,
				Method:  &HistroyChangesMethod,
			})
	*/

	rpcResponseGet, err := texampleresp.SeralizeModelToRPCGetResponse(exampleResp)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&examplev1.ExampleRunResponse{
		Response: rpcResponseGet,
		Version:  texampleversion.ModelToRPC(*example, &res.Resp.ID),
	}), nil
}

type CopyExampleResult struct {
	Example        mitemapiexample.ItemApiExample
	Headers        []mexampleheader.Header
	Queries        []mexamplequery.Query
	BodyRaw        *mbodyraw.ExampleBodyRaw
	BodyForms      []mbodyform.BodyForm
	BodyURLEncoded []mbodyurl.BodyURLEncoded
	Assertions     []massert.Assert

	// Resp
	Resp        mexampleresp.ExampleResp
	RespHeaders []mexamplerespheader.ExampleRespHeader
	RespAsserts []massertres.AssertResult
}

func PrepareCopyExample(ctx context.Context, itemApi idwrap.IDWrap, example mitemapiexample.ItemApiExample,
	hs sexampleheader.HeaderService, qs sexamplequery.ExampleQueryService,
	brs sbodyraw.BodyRawService, bfs sbodyform.BodyFormService, bues sbodyurl.BodyURLEncodedService,
	ers sexampleresp.ExampleRespService, erhs sexamplerespheader.ExampleRespHeaderService,
	as sassert.AssertService, ars sassertres.AssertResultService,
) (CopyExampleResult, error) {
	result := CopyExampleResult{}
	example.IsDefault = false

	// Prepare new example
	exampleIDWrapNew := idwrap.NewNow()
	newExample := example
	newExample.Name = fmt.Sprintf("%s - Copy", example.Name)
	newExample.ID = exampleIDWrapNew
	newExample.ItemApiID = itemApi
	result.Example = newExample

	// Prepare headers copy
	headers, err := hs.GetHeaderByExampleID(ctx, example.ID)
	if err != nil && err != sexampleheader.ErrNoHeaderFound {
		return result, err
	}
	for _, header := range headers {
		newHeader := header
		newHeader.ID = idwrap.NewNow()
		newHeader.ExampleID = exampleIDWrapNew
		result.Headers = append(result.Headers, newHeader)
	}

	// Prepare queries copy
	queries, err := qs.GetExampleQueriesByExampleID(ctx, example.ID)
	if err != nil && err != sexamplequery.ErrNoQueryFound {
		return result, err
	}
	for _, query := range queries {
		newQuery := query
		newQuery.ID = idwrap.NewNow()
		newQuery.ExampleID = exampleIDWrapNew
		result.Queries = append(result.Queries, newQuery)
	}

	// Prepare body copy based on type
	switch example.BodyType {
	case mitemapiexample.BodyTypeRaw:
		bodyRaw, err := brs.GetBodyRawByExampleID(ctx, example.ID)
		if err != nil && err != sbodyraw.ErrNoBodyRawFound {
			return result, err
		}
		if bodyRaw != nil {
			newBodyRaw := *bodyRaw
			newBodyRaw.ID = idwrap.NewNow()
			newBodyRaw.ExampleID = exampleIDWrapNew
			result.BodyRaw = &newBodyRaw
		}

	case mitemapiexample.BodyTypeForm:
		forms, err := bfs.GetBodyFormsByExampleID(ctx, example.ID)
		if err != nil && err != sbodyform.ErrNoBodyFormFound {
			return result, err
		}
		for _, form := range forms {
			newForm := form
			newForm.ID = idwrap.NewNow()
			newForm.ExampleID = exampleIDWrapNew
			result.BodyForms = append(result.BodyForms, newForm)
		}

	case mitemapiexample.BodyTypeUrlencoded:
		urlEncoded, err := bues.GetBodyURLEncodedByExampleID(ctx, example.ID)
		if err != nil && err != sbodyurl.ErrNoBodyUrlEncodedFound {
			return result, err
		}
		for _, encoded := range urlEncoded {
			newEncoded := encoded
			newEncoded.ID = idwrap.NewNow()
			newEncoded.ExampleID = exampleIDWrapNew
			result.BodyURLEncoded = append(result.BodyURLEncoded, newEncoded)
		}
	}

	// Prepare assertions copy
	assertions, err := as.GetAssertByExampleID(ctx, example.ID)
	if err != nil && err != sassert.ErrNoAssertFound {
		return result, err
	}
	for i := range assertions {
		assertions[i].ID = idwrap.NewNow()
		assertions[i].ExampleID = exampleIDWrapNew
	}
	result.Assertions = assertions

	resp, err := ers.GetExampleRespByExampleIDLatest(ctx, example.ID)
	if err != nil && err != sexampleresp.ErrNoRespFound {
		return result, err
	}

	if resp != nil {
		resp.ExampleID = exampleIDWrapNew
		oldRespID := resp.ID
		resp.ID = idwrap.NewNow()
		result.Resp = *resp

		respHeaders, err := erhs.GetHeaderByRespID(ctx, oldRespID)
		if err != nil && err != sexamplerespheader.ErrNoRespHeaderFound {
			return result, err
		}
		for i := range respHeaders {
			respHeaders[i].ID = idwrap.NewNow()
			respHeaders[i].ExampleRespID = resp.ID
		}

		result.RespHeaders = respHeaders

		assertResp, err := ars.GetAssertResultsByResponseID(ctx, oldRespID)
		if err != nil {
			return result, err
		}

		for i := range assertResp {
			assertResp[i].ID = idwrap.NewNow()
			assertResp[i].ResponseID = resp.ID
		}

		result.RespAsserts = assertResp
	}

	return result, nil
}

// Changes
/*
func createChangeResponse(exampleResp *mexampleresp.ExampleResp) (*responsev1.ResponseChange, error) {
	changeStatus := int32(exampleResp.Status)
	size := int32(len(exampleResp.Body))

	return &responsev1.ResponseChange{
		ResponseId: exampleResp.ID.Bytes(),
		Status:     &changeStatus,
		Body:       exampleResp.Body,
		Time:       timestamppb.New(time.Now()),
		Duration:   &exampleResp.Duration,
		Size:       &size,
	}, nil
}
*/

// func createAssertResponse(exampleResp *mexampleresp.ExampleResp, assertions []massert.Assert, resultArr []massertres.AssertResult) (*responsev1.ResponseAssertListResponse, error) {
// 	response := &responsev1.ResponseAssertListResponse{
// 		ResponseId: exampleResp.ID.Bytes(),
// 		Items:      make([]*responsev1.ResponseAssertListItem, len(assertions)),
// 	}

// 	for i := range assertions {
// 		rpcAssert, err := tassert.SerializeAssertModelToRPC(assertions[i])
// 		if err != nil {
// 			return nil, err
// 		}
// 		response.Items[i] = &responsev1.ResponseAssertListItem{
// 			Assert: rpcAssert,
// 			Result: resultArr[i].Result,
// 		}
// 	}
// 	return response, nil
// }

func PrepareCopyExampleNoService(ctx context.Context, itemApi idwrap.IDWrap, example mitemapiexample.ItemApiExample,
	queries []mexamplequery.Query, headers []mexampleheader.Header, assertions []massert.Assert,
	bodyRaw *mbodyraw.ExampleBodyRaw, bodyForm []mbodyform.BodyForm, bodyUrl []mbodyurl.BodyURLEncoded,
	resp *mexampleresp.ExampleResp, respHeaders []mexamplerespheader.ExampleRespHeader, assertResp []massertres.AssertResult,
) (CopyExampleResult, error) {
	result := CopyExampleResult{}
	example.IsDefault = false

	// Prepare new example
	exampleIDWrapNew := idwrap.NewNow()
	newExample := example
	newExample.Name = fmt.Sprintf("%s - Copy", example.Name)
	newExample.ID = exampleIDWrapNew
	newExample.ItemApiID = itemApi
	result.Example = newExample

	// Prepare headers copy
	for _, header := range headers {
		newHeader := header
		newHeader.ID = idwrap.NewNow()
		newHeader.ExampleID = exampleIDWrapNew
		result.Headers = append(result.Headers, newHeader)
	}

	// Prepare queries copy
	for _, query := range queries {
		newQuery := query
		newQuery.ID = idwrap.NewNow()
		newQuery.ExampleID = exampleIDWrapNew
		result.Queries = append(result.Queries, newQuery)
	}

	// Prepare body copy based on type
	switch example.BodyType {
	case mitemapiexample.BodyTypeRaw:
		if bodyRaw != nil {
			newBodyRaw := *bodyRaw
			newBodyRaw.ID = idwrap.NewNow()
			newBodyRaw.ExampleID = exampleIDWrapNew
			result.BodyRaw = &newBodyRaw
		}

	case mitemapiexample.BodyTypeForm:
		for _, form := range bodyForm {
			newForm := form
			newForm.ID = idwrap.NewNow()
			newForm.ExampleID = exampleIDWrapNew
			result.BodyForms = append(result.BodyForms, newForm)
		}

	case mitemapiexample.BodyTypeUrlencoded:
		for _, encoded := range bodyUrl {
			newEncoded := encoded
			newEncoded.ID = idwrap.NewNow()
			newEncoded.ExampleID = exampleIDWrapNew
			result.BodyURLEncoded = append(result.BodyURLEncoded, newEncoded)
		}
	}

	// Prepare assertions copy
	for i := range assertions {
		assertions[i].ID = idwrap.NewNow()
		assertions[i].ExampleID = exampleIDWrapNew
	}
	result.Assertions = assertions

	if resp != nil {
		resp.ExampleID = exampleIDWrapNew
		resp.ID = idwrap.NewNow()
		result.Resp = *resp

		for i := range respHeaders {
			respHeaders[i].ID = idwrap.NewNow()
			respHeaders[i].ExampleRespID = resp.ID
		}

		result.RespHeaders = respHeaders

		for i := range assertResp {
			assertResp[i].ID = idwrap.NewNow()
			assertResp[i].ResponseID = resp.ID
		}

		result.RespAsserts = assertResp
	}

	return result, nil
}

/*
func createHeaderResponse(exampleResp *mexampleresp.ExampleResp, headers []mexamplerespheader.ExampleRespHeader) *responsev1.ResponseHeaderListResponse {
	response := &responsev1.ResponseHeaderListResponse{
		ResponseId: exampleResp.ID.Bytes(),
		Items:      make([]*responsev1.ResponseHeaderListItem, 0),
	}

	slices.SortStableFunc(headers, func(i, j mexamplerespheader.ExampleRespHeader) int {
		return strings.Compare(i.HeaderKey, j.HeaderKey)
	})

	for _, header := range headers {
		response.Items = append(response.Items, &responsev1.ResponseHeaderListItem{
			ResponseHeaderId: header.ID.Bytes(),
			Key:              header.HeaderKey,
			Value:            header.Value,
		})
	}
	return response
}
*/

/*
func createChange(data proto.Message, kind changev1.ChangeKind) (*changev1.Change, error) {
	anyData, err := anypb.New(data)
	if err != nil {
		return nil, err
	}
	return &changev1.Change{
		Kind: &kind,
		Data: anyData,
	}, nil
}
*/

/*
func HandleResponseUpdate(exampleResp *mexampleresp.ExampleResp, assertions []massert.Assert, resultArr []massertres.AssertResult, headers []mexamplerespheader.ExampleRespHeader) ([]*changev1.Change, error) {
	// Create and add change response
	changeResp, err := createChangeResponse(exampleResp)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	change, err := createChange(changeResp, changev1.ChangeKind_CHANGE_KIND_UPDATE)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Create and add assert response
	assertResp, err := createAssertResponse(exampleResp, assertions, resultArr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	assertChange, err := createChange(assertResp, changev1.ChangeKind_CHANGE_KIND_UPDATE)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Create and add header response
	headerResp := createHeaderResponse(exampleResp, headers)
	headerChange, err := createChange(headerResp, changev1.ChangeKind_CHANGE_KIND_UPDATE)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return []*changev1.Change{change, assertChange, headerChange}, nil
}

func HandleResponseCreate(exampleID, exampleRespID idwrap.IDWrap) ([]*changev1.Change, error) {
	exampleChange := &examplev1.ExampleChange{
		ExampleId:      exampleID.Bytes(),
		LastResponseId: exampleRespID.Bytes(),
	}

	createExampleChange, err := createChange(exampleChange, changev1.ChangeKind_CHANGE_KIND_UPDATE)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return []*changev1.Change{createExampleChange}, nil
}
*/

// TODO: make this transaction
func CreateCopyExample(ctx context.Context, tx *sql.Tx, result CopyExampleResult) error {
	// Create the main example
	txIaes, err := sitemapiexample.NewTX(ctx, tx)
	if err != nil {
		return fmt.Errorf("failed to create example: %w", err)
	}
	err = txIaes.CreateApiExample(ctx, &result.Example)
	if err != nil {
		return fmt.Errorf("failed to create example: %w", err)
	}

	// Create headers
	txehs, err := sexampleheader.NewTX(ctx, tx)
	if err != nil {
		return err
	}

	err = txehs.CreateBulkHeader(ctx, result.Headers)
	if err != nil {
		return fmt.Errorf("failed to create header: %w", err)
	}

	if len(result.Queries) > 0 {
		txQs, err := sexamplequery.NewTX(ctx, tx)
		if err != nil {
			return err
		}
		err = txQs.CreateBulkQuery(ctx, result.Queries)
		if err != nil {
			return err
		}
	}

	// Create body based on type
	switch result.Example.BodyType {
	case mitemapiexample.BodyTypeRaw:
		if result.BodyRaw != nil {
			txBrs, err := sbodyraw.NewTX(ctx, tx)
			if err != nil {
				return fmt.Errorf("failed to create body raw: %w", err)
			}
			err = txBrs.CreateBodyRaw(ctx, *result.BodyRaw)
			if err != nil {
				return fmt.Errorf("failed to create body raw: %w", err)
			}
		}
	case mitemapiexample.BodyTypeForm:
		txBfs, err := sbodyform.NewTX(ctx, tx)
		if err != nil {
			return fmt.Errorf("failed to create body form: %w", err)
		}
		err = txBfs.CreateBulkBodyForm(ctx, result.BodyForms)
		if err != nil {
			return fmt.Errorf("failed to create body form: %w", err)
		}

	case mitemapiexample.BodyTypeUrlencoded:
		txBues, err := sbodyurl.NewTX(ctx, tx)
		if err != nil {
			return fmt.Errorf("failed to create body url encoded: %w", err)
		}
		err = txBues.CreateBulkBodyURLEncoded(ctx, result.BodyURLEncoded)
		if err != nil {
			return fmt.Errorf("failed to create body url encoded: %w", err)
		}
	}

	// Create assertions
	txAs, err := sassert.NewTX(ctx, tx)
	if err != nil {
		return fmt.Errorf("failed to create assertion: %w", err)
	}
	err = txAs.CreateAssertBulk(ctx, result.Assertions)
	if err != nil {
		return fmt.Errorf("failed to create assertion: %w", err)
	}

	txErs, err := sexampleresp.NewTX(ctx, tx)
	if err != nil {
		return fmt.Errorf("failed to create example response: %w", err)
	}
	err = txErs.CreateExampleResp(ctx, result.Resp)
	if err != nil {
		return fmt.Errorf("failed to create example response: %w", err)
	}

	txErhs, err := sexamplerespheader.NewTX(ctx, tx)
	if err != nil {
		return fmt.Errorf("failed to create example response header: %w", err)
	}
	err = txErhs.CreateExampleRespHeaderBulk(ctx, result.RespHeaders)
	if err != nil {
		return fmt.Errorf("failed to create example response header: %w", err)
	}

	txArs, err := sassertres.NewTX(ctx, tx)
	if err != nil {
		return fmt.Errorf("failed to create assert result: %w", err)
	}
	err = txArs.CreateAssertResultBulk(ctx, result.RespAsserts)
	return err
}

func (c *ItemAPIExampleRPC) ExampleVersions(ctx context.Context, req *connect.Request[examplev1.ExampleVersionsRequest]) (*connect.Response[examplev1.ExampleVersionsResponse], error) {
	versionParentID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid item api id"))
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerExample(ctx, *c.iaes, *c.cs, *c.us, versionParentID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	exampleVersionItems, err := c.GetVersion(ctx, versionParentID)
	if err != nil {
		return nil, err
	}

	resp := &examplev1.ExampleVersionsResponse{
		ExampleId: req.Msg.GetExampleId(),
		Items:     exampleVersionItems,
	}

	return connect.NewResponse(resp), nil
}

func (c *ItemAPIExampleRPC) GetVersion(ctx context.Context, versionParentID idwrap.IDWrap) ([]*examplev1.ExampleVersionsItem, error) {
	examples, err := c.iaes.GetApiExampleByVersionParentID(ctx, versionParentID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// sort by created at
	sort.Slice(examples, func(i, j int) bool {
		return examples[i].ID.Compare(examples[j].ID) > 0
	})
	items := make([]*examplev1.ExampleVersionsItem, len(examples))

	for i, example := range examples {
		a := &examplev1.ExampleVersionsItem{}
		items[i] = a

		a.ExampleId = example.ID.Bytes()
		resp, err := c.ers.GetExampleRespByExampleIDLatest(ctx, example.ID)
		if err != nil {
			continue
		}
		a.LastResponseId = resp.ID.Bytes()
	}

	return items, nil
}

func CheckOwnerExample(ctx context.Context, iaes sitemapiexample.ItemApiExampleService, cs scollection.CollectionService, us suser.UserService, exampleUlid idwrap.IDWrap) (bool, error) {
	example, err := iaes.GetApiExample(ctx, exampleUlid)
	if err != nil {
		if err == sql.ErrNoRows {
			// INFO: this mean that workspace not belong to user
			// So for avoid information leakage, we should return not found
			err = connect.NewError(connect.CodeNotFound, errors.New("example not found"))
		}
		return false, err
	}
	return rcollection.CheckOwnerCollection(ctx, cs, us, example.CollectionID)
}
