package ritemapiexample

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"mime/multipart"
	"net/url"
	"strings"
	"the-dev-tools/backend/internal/api"
	"the-dev-tools/backend/internal/api/collection"
	"the-dev-tools/backend/internal/api/ritemapi"
	"the-dev-tools/backend/pkg/assertsys"
	"the-dev-tools/backend/pkg/compress"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/massert"
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
	"the-dev-tools/backend/pkg/translate/texample"
	"the-dev-tools/backend/pkg/varsystem"
	"the-dev-tools/backend/pkg/zstdcompress"
	"the-dev-tools/nodes/pkg/httpclient"
	bodyv1 "the-dev-tools/spec/dist/buf/go/collection/item/body/v1"
	examplev1 "the-dev-tools/spec/dist/buf/go/collection/item/example/v1"
	"the-dev-tools/spec/dist/buf/go/collection/item/example/v1/examplev1connect"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/anypb"
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
	return connect.NewResponse(&examplev1.ExampleListResponse{Items: respsRpc}), nil
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
	ExampleIDWrapNew := idwrap.NewNow()
	ex := &mitemapiexample.ItemApiExample{
		ID:           ExampleIDWrapNew,
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
		ExampleID:     ExampleIDWrapNew,
		VisualizeMode: mbodyraw.VisualizeModeBinary,
		CompressType:  mbodyraw.CompressTypeNone,
		Data:          []byte{},
	}

	err = c.brs.CreateBodyRaw(ctx, bodyRaw)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&examplev1.ExampleCreateResponse{
		ExampleId: ExampleIDWrapNew.Bytes(),
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

	var changes []*anypb.Any

	changeAny, err := anypb.New(a)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	changes = append(changes, changeAny)

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
	rpcErr := permcheck.CheckPerm(CheckOwnerExample(ctx, *c.iaes, *c.cs, *c.us, exampleUlid))
	if rpcErr != nil {
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

	// Get workspace of of the current example

	collection, err := c.cs.GetCollection(ctx, example.CollectionID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	workspaceID := collection.OwnerID

	env, err := c.es.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var selectedEnv *menv.Env
	var globalEnv *menv.Env
	if len(env) != 0 {
		for _, e := range env {
			if e.Type == menv.EnvGlobal {
				globalEnv = &e
				continue
			}
			if e.Active {
				selectedEnv = &e
				continue
			}
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
		// TODO implement var system
		for i, query := range reqQueries {
			if varsystem.CheckIsVar(query.Value) {
				key := varsystem.GetVarKeyFromRaw(query.Value)
				val, ok := varMap.Get(key)
				if ok {
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
				case "deflate":
					return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("deflate not supported"))
				case "br":
					return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("br not supported"))
				case "identity":
					return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("identity not supported"))
				default:
					return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid compression type %s", header.Value))
				}
			}

			if varsystem.CheckIsVar(header.Value) {
				key := varsystem.GetVarKeyFromRaw(header.Value)
				val, ok := varMap.Get(key)
				if !ok {
					return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%s named error not found", key))
				}
				reqHeaders[i].Value = val.Value
			}
		}
	}

	bodyBytes := &bytes.Buffer{}
	switch example.BodyType {
	case mitemapiexample.BodyTypeNone:
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
			err = writer.WriteField(v.BodyKey, v.Value)
			if err != nil {
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
		// TODO: refactor url encode
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

			err = c.ers.CreateExampleResp(ctx, exampleRespTemp)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		} else {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if len(bodyData) > 1024 {
		// TODO: check later if it is better then %10 if it is not better change it
		bodyDataTemp := zstdcompress.Compress(bodyData)
		if len(bodyDataTemp) < 1024 {
			exampleResp.BodyCompressType = mexampleresp.BodyCompressTypeZstd
			bodyData = bodyDataTemp
		}
	}

	exampleResp.Body = bodyData
	exampleResp.Duration = int32(lapse.Milliseconds())
	exampleResp.Status = uint16(respHttp.StatusCode)

	err = c.ers.UpdateExampleResp(ctx, *exampleResp)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	dbHeaders, err := c.erhs.GetHeaderByRespID(ctx, exampleResp.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// TODO: make it more efficient
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
			}
		}
		if !found {
			taskDeleteHeaders = append(taskDeleteHeaders, dbHeader.ID)
		}
	}
	fullHeaders := append(taskCreateHeaders, taskUpdateHeaders...)
	if len(fullHeaders) > 0 {
		tx, err := c.DB.Begin()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		erhsTx, err := sexamplerespheader.NewTX(ctx, tx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if len(taskCreateHeaders) > 0 {
			err = erhsTx.CreateExampleRespHeaderBulk(ctx, taskCreateHeaders)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
		if len(taskUpdateHeaders) > 0 {
			err = erhsTx.UpdateExampleRespHeaderBulk(ctx, taskUpdateHeaders)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
		if len(taskDeleteHeaders) > 0 {
			err = erhsTx.DeleteExampleRespHeaderBulk(ctx, taskDeleteHeaders)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
		err = tx.Commit()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	assertions, err := c.as.GetAssertByExampleID(ctx, example.ID)
	if err != nil {
		if err != sassert.ErrNoAssertFound {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		assertions = []massert.Assert{}
	}
	for _, assertion := range assertions {
		if assertion.Enable {
			assertionResult, err := assertsys.New().Eval(respHttp, assertion.Type, assertion.Path, assertion.Value)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			res := massertres.AssertResult{
				ID:         idwrap.NewNow(),
				ResponseID: exampleResp.ID,
				AssertID:   assertion.ID,
				Result:     assertionResult,
			}
			err = c.ars.CreateAssertResult(ctx, res)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

		}
	}

	rpcResponse := connect.NewResponse(&examplev1.ExampleRunResponse{
		ResponseId: exampleResp.ID.Bytes(),
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
	return collection.CheckOwnerCollection(ctx, cs, us, example.CollectionID)
}
