package ritemapiexample

import (
	"bytes"
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/collection"
	"dev-tools-backend/internal/api/ritemapi"
	"dev-tools-backend/pkg/assertsys"
	"dev-tools-backend/pkg/compress"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/massertres"
	"dev-tools-backend/pkg/model/mbodyraw"
	"dev-tools-backend/pkg/model/menv"
	"dev-tools-backend/pkg/model/mexampleresp"
	"dev-tools-backend/pkg/model/mexamplerespheader"
	"dev-tools-backend/pkg/model/mitemapiexample"
	"dev-tools-backend/pkg/permcheck"
	"dev-tools-backend/pkg/service/sassert"
	"dev-tools-backend/pkg/service/sassertres"
	"dev-tools-backend/pkg/service/sbodyform"
	"dev-tools-backend/pkg/service/sbodyraw"
	"dev-tools-backend/pkg/service/sbodyurl"
	"dev-tools-backend/pkg/service/scollection"
	"dev-tools-backend/pkg/service/senv"
	"dev-tools-backend/pkg/service/sexampleheader"
	"dev-tools-backend/pkg/service/sexamplequery"
	"dev-tools-backend/pkg/service/sexampleresp"
	"dev-tools-backend/pkg/service/sexamplerespheader"
	"dev-tools-backend/pkg/service/sitemapi"
	"dev-tools-backend/pkg/service/sitemapiexample"
	"dev-tools-backend/pkg/service/sresultapi"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/service/svar"
	"dev-tools-backend/pkg/translate/texample"
	"dev-tools-backend/pkg/varsystem"
	"dev-tools-backend/pkg/zstdcompress"
	"dev-tools-nodes/pkg/httpclient"
	bodyv1 "dev-tools-spec/dist/buf/go/collection/item/body/v1"
	examplev1 "dev-tools-spec/dist/buf/go/collection/item/example/v1"
	"dev-tools-spec/dist/buf/go/collection/item/example/v1/examplev1connect"
	"errors"
	"fmt"
	"mime/multipart"
	"net/url"
	"strings"
	"time"

	"connectrpc.com/connect"
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

	return connect.NewResponse(&examplev1.ExampleDeleteResponse{}), nil
}

func (c *ItemAPIExampleRPC) ExampleDuplicate(ctx context.Context, req *connect.Request[examplev1.ExampleDuplicateRequest]) (*connect.Response[examplev1.ExampleDuplicateResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (c *ItemAPIExampleRPC) ExampleRun(ctx context.Context, req *connect.Request[examplev1.ExampleRunRequest]) (*connect.Response[examplev1.ExampleRunResponse], error) {
	exampleUlid, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
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

	assertions, err := c.as.GetAssertByExampleID(ctx, example.ItemApiID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, assertion := range assertions {
		if assertion.Enable {
			assertionResult, err := assertsys.New().Eval(respHttp, assertion.Type, assertion.Path, assertion.Value)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			res := massertres.AssertResult{
				ID:       idwrap.NewNow(),
				AssertID: assertion.ID,
				Result:   assertionResult,
			}
			err = c.ars.CreateAssertResult(ctx, res)
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
		return false, err
	}
	return collection.CheckOwnerCollection(ctx, cs, us, example.CollectionID)
}

/*
// Asserts
func (c ItemAPIExampleRPC) CreateAssert(ctx context.Context, req *connect.Request[itemapiexamplev1.CreateAssertRequest]) (*connect.Response[itemapiexamplev1.CreateAssertResponse], error) {
	assert, err := tassert.SerializeAssertRPCToModelWithoutID(req.Msg.GetAssert())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	assert.ID = idwrap.NewNow()
	err = c.as.CreateAssert(ctx, assert)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&itemapiexamplev1.CreateAssertResponse{Id: assert.ID.String()}), nil
}

func (c ItemAPIExampleRPC) UpdateAssert(ctx context.Context, req *connect.Request[itemapiexamplev1.UpdateAssertRequest]) (*connect.Response[itemapiexamplev1.UpdateAssertResponse], error) {
	assert, err := tassert.SerializeAssertRPCToModel(req.Msg.GetAssert())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	err = c.as.UpdateAssert(ctx, assert)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&itemapiexamplev1.UpdateAssertResponse{}), nil
}

func (c ItemAPIExampleRPC) DeleteAssert(ctx context.Context, req *connect.Request[itemapiexamplev1.DeleteAssertRequest]) (*connect.Response[itemapiexamplev1.DeleteAssertResponse], error) {
	id, err := idwrap.NewWithParse(req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	err = c.as.DeleteAssert(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&itemapiexamplev1.DeleteAssertResponse{}), nil
}

// Headers
func (c *ItemAPIExampleRPC) CreateHeader(ctx context.Context, req *connect.Request[itemapiexamplev1.CreateHeaderRequest]) (*connect.Response[itemapiexamplev1.CreateHeaderResponse], error) {
	headerData := req.Msg.GetHeader()
	if headerData == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("header is nil"))
	}

	headerModel, err := theader.SerlializeHeaderRPCtoModelNoID(req.Msg.GetHeader())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	newIDWrap := idwrap.NewNow()
	headerModel.ID = newIDWrap

	ok, err := CheckOwnerExample(ctx, *c.iaes, *c.cs, *c.us, headerModel.ExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no example found"))
	}

	err = c.hs.CreateHeader(ctx, headerModel)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&itemapiexamplev1.CreateHeaderResponse{Id: newIDWrap.String()}), nil
}

func (c *ItemAPIExampleRPC) UpdateHeader(ctx context.Context, req *connect.Request[itemapiexamplev1.UpdateHeaderRequest]) (*connect.Response[itemapiexamplev1.UpdateHeaderResponse], error) {
	HeaderModel, err := theader.SerlializeHeaderRPCtoModel(req.Msg.GetHeader())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	ok, err := CheckOwnerHeader(ctx, *c.hs, *c.iaes, *c.cs, *c.us, HeaderModel.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no header found"))
	}
	err = c.hs.UpdateHeader(ctx, HeaderModel)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&itemapiexamplev1.UpdateHeaderResponse{}), nil
}

func (c *ItemAPIExampleRPC) DeleteHeader(ctx context.Context, req *connect.Request[itemapiexamplev1.DeleteHeaderRequest]) (*connect.Response[itemapiexamplev1.DeleteHeaderResponse], error) {
	ulidWrap, err := idwrap.NewWithParse(req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	ok, err := CheckOwnerHeader(ctx, *c.hs, *c.iaes, *c.cs, *c.us, ulidWrap)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no header found"))
	}
	err = c.hs.DeleteHeader(ctx, ulidWrap)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&itemapiexamplev1.DeleteHeaderResponse{}), nil
}

// Query
func (c *ItemAPIExampleRPC) CreateQuery(ctx context.Context, req *connect.Request[itemapiexamplev1.CreateQueryRequest]) (*connect.Response[itemapiexamplev1.CreateQueryResponse], error) {
	queryData, err := tquery.SerlializeQueryRPCtoModelNoID(req.Msg.GetQuery())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	idWrap := idwrap.NewNow()
	queryData.ID = idWrap
	ok, err := CheckOwnerExample(ctx, *c.iaes, *c.cs, *c.us, queryData.ExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no example found"))
	}
	err = c.qs.CreateExampleQuery(ctx, queryData)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&itemapiexamplev1.CreateQueryResponse{Id: idWrap.String()}), nil
}

func (c *ItemAPIExampleRPC) UpdateQuery(ctx context.Context, req *connect.Request[itemapiexamplev1.UpdateQueryRequest]) (*connect.Response[itemapiexamplev1.UpdateQueryResponse], error) {
	queryData, err := tquery.SerlializeQueryRPCtoModel(req.Msg.GetQuery())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	ok, err := CheckOwnerQuery(ctx, *c.qs, *c.iaes, *c.cs, *c.us, queryData.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no query found"))
	}
	err = c.qs.UpdateExampleQuery(ctx, queryData)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&itemapiexamplev1.UpdateQueryResponse{}), nil
}

func (c *ItemAPIExampleRPC) DeleteQuery(ctx context.Context, req *connect.Request[itemapiexamplev1.DeleteQueryRequest]) (*connect.Response[itemapiexamplev1.DeleteQueryResponse], error) {
	ulidWrap, err := idwrap.NewWithParse(req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	ok, err := CheckOwnerQuery(ctx, *c.qs, *c.iaes, *c.cs, *c.us, ulidWrap)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no query found"))
	}
	err = c.qs.DeleteExampleQuery(ctx, ulidWrap)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&itemapiexamplev1.DeleteQueryResponse{}), nil
}
*/
