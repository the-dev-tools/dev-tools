package ritemapiexample

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"mime/multipart"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"the-dev-tools/backend/internal/api"
	"the-dev-tools/backend/internal/api/rcollection"
	"the-dev-tools/backend/internal/api/ritemapi"
	"the-dev-tools/backend/pkg/assertv2"
	"the-dev-tools/backend/pkg/assertv2/leafs/leafjson"
	"the-dev-tools/backend/pkg/compress"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/massertres"
	"the-dev-tools/backend/pkg/model/mbodyraw"
	"the-dev-tools/backend/pkg/model/menv"
	"the-dev-tools/backend/pkg/model/mexampleresp"
	"the-dev-tools/backend/pkg/model/mexamplerespheader"
	"the-dev-tools/backend/pkg/model/mitemapiexample"
	"the-dev-tools/backend/pkg/permcheck"
	"the-dev-tools/backend/pkg/service/sassert"
	"the-dev-tools/backend/pkg/service/sassertres"
	"the-dev-tools/backend/pkg/service/sbodyform"
	"the-dev-tools/backend/pkg/service/sbodyraw"
	"the-dev-tools/backend/pkg/service/sbodyurl"
	"the-dev-tools/backend/pkg/service/scollection"
	"the-dev-tools/backend/pkg/service/senv"
	"the-dev-tools/backend/pkg/service/sexampleheader"
	"the-dev-tools/backend/pkg/service/sexamplequery"
	"the-dev-tools/backend/pkg/service/sexampleresp"
	"the-dev-tools/backend/pkg/service/sexamplerespheader"
	"the-dev-tools/backend/pkg/service/sitemapi"
	"the-dev-tools/backend/pkg/service/sitemapiexample"
	"the-dev-tools/backend/pkg/service/sresultapi"
	"the-dev-tools/backend/pkg/service/suser"
	"the-dev-tools/backend/pkg/service/svar"
	"the-dev-tools/backend/pkg/translate/tassert"
	"the-dev-tools/backend/pkg/translate/texample"
	"the-dev-tools/backend/pkg/varsystem"
	"the-dev-tools/backend/pkg/zstdcompress"
	"the-dev-tools/nodes/pkg/httpclient"
	changev1 "the-dev-tools/spec/dist/buf/go/change/v1"
	bodyv1 "the-dev-tools/spec/dist/buf/go/collection/item/body/v1"
	examplev1 "the-dev-tools/spec/dist/buf/go/collection/item/example/v1"
	"the-dev-tools/spec/dist/buf/go/collection/item/example/v1/examplev1connect"
	responsev1 "the-dev-tools/spec/dist/buf/go/collection/item/response/v1"
	itemv1 "the-dev-tools/spec/dist/buf/go/collection/item/v1"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ItemAPIExampleRPC struct {
	DB   *sql.DB
	iaes *sitemapiexample.ItemApiExampleService
	ias  *sitemapi.ItemApiService
	ras  *sresultapi.ResultApiService
	cs   *scollection.CollectionService
	us   *suser.UserService
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
}

func New(db *sql.DB, iaes sitemapiexample.ItemApiExampleService, ias sitemapi.ItemApiService, ras sresultapi.ResultApiService,
	cs scollection.CollectionService, us suser.UserService, hs sexampleheader.HeaderService, qs sexamplequery.ExampleQueryService,
	bfs sbodyform.BodyFormService, beus sbodyurl.BodyURLEncodedService, brs sbodyraw.BodyRawService, erhs sexamplerespheader.ExampleRespHeaderService,
	ers sexampleresp.ExampleRespService, es senv.EnvService, vs svar.VarService, as sassert.AssertService, ars sassertres.AssertResultService,
) ItemAPIExampleRPC {
	return ItemAPIExampleRPC{
		DB:   db,
		iaes: &iaes,
		ias:  &ias,
		ras:  &ras,
		cs:   &cs,
		us:   &us,
		hs:   &hs,
		qs:   &qs,
		bfs:  &bfs,
		bues: &beus,
		brs:  &brs,
		erhs: &erhs,
		ers:  &ers,
		es:   es,
		vs:   vs,
		as:   &as,
		ars:  &ars,
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
		exampleResp, err := c.ers.GetExampleRespByExampleID(ctx, example.ID)
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

	isMember, err := CheckOwnerExample(ctx, *c.iaes, *c.cs, *c.us, exampleIdWrap)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isMember {
		// return not found
		return nil, connect.NewError(connect.CodeNotFound, errors.New("not found example"))
	}

	example, err := c.iaes.GetApiExample(ctx, exampleIdWrap)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// TODO: this can fail fix this
	var parentExampleIdWrap []byte = nil
	exampleResp, err := c.ers.GetExampleRespByExampleID(ctx, exampleIdWrap)
	if err != nil && err != sexampleresp.ErrNoRespFound {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err == nil && exampleResp != nil {
		parentExampleIdWrap = exampleResp.ID.Bytes()
	}

	resp := &examplev1.ExampleGetResponse{
		ExampleId:      example.ID.Bytes(),
		LastResponseId: parentExampleIdWrap,
		Name:           example.Name,
		BodyKind:       bodyv1.BodyKind(example.BodyType),
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
		CompressType:  mbodyraw.CompressTypeNone,
		Data:          []byte{},
	}

	err = c.brs.CreateBodyRaw(ctx, bodyRaw)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// TODO: refactor changes stuff
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

	return connect.NewResponse(&examplev1.ExampleCreateResponse{
		ExampleId: ExampleID.Bytes(),
		Changes:   changes,
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

	exRPC := req.Msg
	ex := &mitemapiexample.ItemApiExample{
		ID:       exampleIDWrap,
		Name:     exRPC.GetName(),
		BodyType: mitemapiexample.BodyType(exRPC.GetBodyKind()),
	}

	err = c.iaes.UpdateItemApiExample(ctx, ex)
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

	err = c.iaes.DeleteApiExample(ctx, exampleUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

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

	resp := &examplev1.ExampleDeleteResponse{
		Changes: changes,
	}

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

	exampleIDWrapNew := idwrap.NewNow()
	example.Name = fmt.Sprintf("%s - Copy", example.Name)
	example.ID = exampleIDWrapNew
	err = c.iaes.CreateApiExample(ctx, example)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&examplev1.ExampleDuplicateResponse{}), nil
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
	workspaceID := collection.OwnerID

	env, err := c.es.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var selectedEnv, globalEnv *menv.Env
	for _, e := range env {
		if e.Type == menv.EnvGlobal {
			globalEnv = &e
		} else if e.Active {
			selectedEnv = &e
		}
	}
	if selectedEnv == nil {
		selectedEnv = globalEnv
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

	compressType := compress.CompressTypeNone
	if varMap != nil {
		for i, query := range reqQueries {
			if varsystem.CheckIsVar(query.Value) {
				key := varsystem.GetVarKeyFromRaw(query.Value)
				if val, ok := varMap.Get(key); ok {
					reqQueries[i].Value = val.Value
				} else {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%s named error not found", key))
				}
			}
		}

		for i, header := range reqHeaders {
			if header.HeaderKey == "Content-Encoding" {
				switch strings.ToLower(header.Value) {
				case "gzip":
					compressType = compress.CompressTypeGzip
				case "zstd":
					compressType = compress.CompressTypeZstd
				case "deflate", "br", "identity":
					return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s not supported", header.Value))
				default:
					return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid compression type %s", header.Value))
				}
			}

			if varsystem.CheckIsVar(header.Value) {
				key := varsystem.GetVarKeyFromRaw(header.Value)
				if val, ok := varMap.Get(key); !ok {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%s named error not found", key))
				} else {
					reqHeaders[i].Value = val.Value
				}
			}
		}
	}

	bodyBytes := &bytes.Buffer{}
	switch example.BodyType {
	case mitemapiexample.BodyTypeRaw:
		bodyData, err := c.brs.GetBodyRawByExampleID(ctx, exampleUlid)
		if err != nil {
			return nil, err
		}
		bodyBytes.Write(bodyData.Data)
	case mitemapiexample.BodyTypeForm:
		forms, err := c.bfs.GetBodyFormsByExampleID(ctx, exampleUlid)
		if err != nil {
			return nil, err
		}
		writer := multipart.NewWriter(bodyBytes)
		for _, v := range forms {
			if err := writer.WriteField(v.BodyKey, v.Value); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
	case mitemapiexample.BodyTypeUrlencoded:
		urls, err := c.bues.GetBodyURLEncodedByExampleID(ctx, exampleUlid)
		if err != nil {
			return nil, err
		}
		urlVal := url.Values{}
		for _, url := range urls {
			urlVal.Add(url.BodyKey, url.Value)
		}
		itemApiCall.Url += urlVal.Encode()
	}

	if compressType != compress.CompressTypeNone {
		compressedData, err := compress.Compress(bodyBytes.Bytes(), compressType)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		bodyBytes = bytes.NewBuffer(compressedData)
	}

	httpReq := httpclient.Request{
		Method:  itemApiCall.Method,
		URL:     itemApiCall.Url,
		Headers: reqHeaders,
		Queries: reqQueries,
		Body:    bodyBytes.Bytes(),
	}

	var changes changev1.Changes

	now := time.Now()
	respHttp, err := httpclient.SendRequestAndConvert(httpclient.New(), httpReq, exampleUlid)
	lapse := time.Since(now)
	if err != nil {
		return nil, connect.NewError(connect.CodeAborted, err)
	}

	bodyData := respHttp.Body

	exampleResp, err := c.ers.GetExampleRespByExampleID(ctx, exampleUlid)
	if err != nil {
		if err == sexampleresp.ErrNoRespFound {
			exampleRespTemp := mexampleresp.ExampleResp{
				ID:        idwrap.NewNow(),
				ExampleID: exampleUlid,
			}
			exampleResp = &exampleRespTemp

			if err := c.ers.CreateExampleResp(ctx, exampleRespTemp); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		} else {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if len(bodyData) > 1024 {
		bodyDataTemp := zstdcompress.Compress(bodyData)
		if len(bodyDataTemp) < len(bodyData) {
			exampleResp.BodyCompressType = mexampleresp.BodyCompressTypeZstd
			bodyData = bodyDataTemp
		}
	}

	exampleResp.Body = bodyData
	exampleResp.Duration = int32(lapse.Milliseconds())
	exampleResp.Status = uint16(respHttp.StatusCode)

	if err := c.ers.UpdateExampleResp(ctx, *exampleResp); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	dbHeaders, err := c.erhs.GetHeaderByRespID(ctx, exampleResp.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	taskCreateHeaders := make([]mexamplerespheader.ExampleRespHeader, 0)
	taskUpdateHeaders := make([]mexamplerespheader.ExampleRespHeader, 0)
	taskDeleteHeaders := make([]idwrap.IDWrap, 0)
	for _, respHeader := range respHttp.Headers {
		found := false
		for _, dbHeader := range dbHeaders {
			if dbHeader.HeaderKey == respHeader.HeaderKey {
				found = true
				if dbHeader.Value != respHeader.Value {
					dbHeader.Value = respHeader.Value
					taskUpdateHeaders = append(taskUpdateHeaders, dbHeader)
				}
			}
		}
		if !found {
			taskCreateHeaders = append(taskCreateHeaders, mexamplerespheader.ExampleRespHeader{
				ID:            idwrap.NewNow(),
				ExampleRespID: exampleResp.ID,
				HeaderKey:     respHeader.HeaderKey,
				Value:         respHeader.Value,
			})
		}
	}

	for _, dbHeader := range dbHeaders {
		found := false
		for _, respHeader := range respHttp.Headers {
			if dbHeader.HeaderKey == respHeader.HeaderKey {
				found = true
				break
			}
		}
		if !found {
			taskDeleteHeaders = append(taskDeleteHeaders, dbHeader.ID)
		}
	}
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

	assertions, err := c.as.GetAssertByExampleID(ctx, example.ID)
	if err != nil && err != sassert.ErrNoAssertFound {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var resultArr []massertres.AssertResult
	for _, assertion := range assertions {
		if assertion.Enable {
			tempStruct := struct {
				Response httpclient.ResponseVar `json:"response"`
			}{
				Response: httpclient.ConvertResponseToVar(respHttp),
			}

			rootLeaf, err := leafjson.NewWithStruct(tempStruct)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			root := assertv2.NewAssertRoot(rootLeaf)
			assertSys := assertv2.NewAssertSystem(root)
			val := assertion.Value
			var value interface{} = val

			if strings.Contains(val, ".") {
				if feetFloat, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
					value = feetFloat
				}
			} else {
				if feetInt, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
					value = feetInt
				}
			}

			ok, err := assertSys.AssertSimple(ctx, assertv2.AssertType(assertion.Type), assertion.Path, value)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			res := massertres.AssertResult{
				ID:         assertion.ID,
				ResponseID: exampleResp.ID,
				AssertID:   assertion.ID,
				Result:     ok,
			}

			resultArr = append(resultArr, res)

			if _, err := c.ars.GetAssertResult(ctx, res.ID); err != nil {
				if err == sql.ErrNoRows {
					if err := c.ars.CreateAssertResult(ctx, res); err != nil {
						return nil, connect.NewError(connect.CodeInternal, err)
					}
				}
			} else {
				if err := c.ars.UpdateAssertResult(ctx, res); err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
			}
		}
	}

	changeStatus := int32(exampleResp.Status)
	changeResp := responsev1.ResponseChange{
		ResponseId: exampleResp.ID.Bytes(),
		Status:     &changeStatus,
		Body:       exampleResp.Body,
		Time:       timestamppb.New(time.Now()),
		Duration:   &exampleResp.Duration,
	}

	kind := changev1.ChangeKind_CHANGE_KIND_UPDATE
	anyData, err := anypb.New(&changeResp)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	changeRoot := changev1.Change{
		Kind: &kind,
		Data: anyData,
	}
	changes.Changes = append(changes.Changes, &changeRoot)

	responseAssertChangeNormal := responsev1.ResponseAssertListResponse{
		ResponseId: exampleResp.ID.Bytes(),
		Items:      make([]*responsev1.ResponseAssertListItem, 0),
	}
	for i, result := range assertions {
		rpcAssert, err := tassert.SerializeAssertModelToRPC(result)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		responseAssertChangeNormal.Items = append(responseAssertChangeNormal.Items, &responsev1.ResponseAssertListItem{
			Assert: rpcAssert,
			Result: resultArr[i].Result,
		})
	}

	assertRespAny, err := anypb.New(&responseAssertChangeNormal)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	assertRespChange := changev1.Change{
		Kind: &kind,
		Data: assertRespAny,
	}
	changes.Changes = append(changes.Changes, &assertRespChange)

	responseHeaderChangeNormal := responsev1.ResponseHeaderListResponse{
		ResponseId: exampleResp.ID.Bytes(),
		Items:      make([]*responsev1.ResponseHeaderListItem, 0),
	}

	slices.SortStableFunc(respHttp.Headers, func(i, j mexamplerespheader.ExampleRespHeader) int {
		return strings.Compare(i.HeaderKey, j.HeaderKey)
	})

	// TODO: this should be just changes later
	for _, header := range respHttp.Headers {
		rpcHeader := &responsev1.ResponseHeaderListItem{
			ResponseHeaderId: header.ID.Bytes(),
			Key:              header.HeaderKey,
			Value:            header.Value,
		}
		responseHeaderChangeNormal.Items = append(responseHeaderChangeNormal.Items, rpcHeader)
	}

	headerRespAny, err := anypb.New(&responseHeaderChangeNormal)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	headerRespChange := changev1.Change{
		Kind: &kind,
		Data: headerRespAny,
	}

	changes.Changes = append(changes.Changes, &headerRespChange)

	rpcResponse := connect.NewResponse(&examplev1.ExampleRunResponse{
		ResponseId: exampleResp.ID.Bytes(),
		Status:     int32(exampleResp.Status),
		Body:       exampleResp.Body,
		Time:       timestamppb.New(time.Now()),
		Duration:   exampleResp.Duration,
		Changes:    changes.Changes,
	})
	rpcResponse.Header().Set("Cache-Control", "max-age=0")

	return rpcResponse, nil
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
