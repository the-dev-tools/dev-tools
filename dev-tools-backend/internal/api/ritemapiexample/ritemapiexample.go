package ritemapiexample

import (
	"bytes"
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/collection"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/internal/api/middleware/mwcompress"
	"dev-tools-backend/internal/api/ritemapi"
	"dev-tools-backend/pkg/compress"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mbodyraw"
	"dev-tools-backend/pkg/model/menv"
	"dev-tools-backend/pkg/model/mexampleresp"
	"dev-tools-backend/pkg/model/mexamplerespheader"
	"dev-tools-backend/pkg/model/mitemapiexample"
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
	"dev-tools-backend/pkg/translate/tassert"
	"dev-tools-backend/pkg/translate/tbodyraw"
	"dev-tools-backend/pkg/translate/texample"
	"dev-tools-backend/pkg/translate/texampleresp"
	"dev-tools-backend/pkg/translate/tgeneric"
	"dev-tools-backend/pkg/translate/theader"
	"dev-tools-backend/pkg/translate/tquery"
	"dev-tools-backend/pkg/varsystem"
	"dev-tools-backend/pkg/zstdcompress"
	"dev-tools-nodes/pkg/httpclient"
	bodyv1 "dev-tools-services/gen/body/v1"
	itemapiexamplev1 "dev-tools-services/gen/itemapiexample/v1"
	"dev-tools-services/gen/itemapiexample/v1/itemapiexamplev1connect"
	"errors"
	"fmt"
	"mime/multipart"
	"net/url"
	"strings"
	"time"

	"connectrpc.com/connect"
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
	as     *sassert.AssertService
	ars    *sassertres.AssertResultService
	secret []byte
}

func New(db *sql.DB, iaes sitemapiexample.ItemApiExampleService, ias sitemapi.ItemApiService, ras sresultapi.ResultApiService,
	cs scollection.CollectionService, us suser.UserService, hs sexampleheader.HeaderService, qs sexamplequery.ExampleQueryService,
	bfs sbodyform.BodyFormService, beus sbodyurl.BodyURLEncodedService, brs sbodyraw.BodyRawService, erhs sexamplerespheader.ExampleRespHeaderService,
	ers sexampleresp.ExampleRespService, es senv.EnvService, vs svar.VarService, as sassert.AssertService, ars sassertres.AssertResultService,
	secret []byte,
) *ItemAPIExampleRPC {
	return &ItemAPIExampleRPC{
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

func CreateService(ctx context.Context, srv ItemAPIExampleRPC) (*api.Service, error) {
	var options []connect.HandlerOption
	options = append(options, connect.WithCompression("zstd", mwcompress.NewDecompress, mwcompress.NewCompress))
	options = append(options, connect.WithCompression("gzip", nil, nil))
	options = append(options, connect.WithInterceptors(mwauth.NewAuthInterceptor(srv.secret)))

	path, handler := itemapiexamplev1connect.NewItemApiExampleServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *ItemAPIExampleRPC) GetExamples(ctx context.Context, req *connect.Request[itemapiexamplev1.GetExamplesRequest]) (*connect.Response[itemapiexamplev1.GetExamplesResponse], error) {
	apiUlid, err := idwrap.NewWithParse(req.Msg.GetItemApiId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid item api id"))
	}

	ok, err := ritemapi.CheckOwnerApi(ctx, *c.ias, *c.cs, *c.us, apiUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("not found api"))
	}

	examples, err := c.iaes.GetApiExamples(ctx, apiUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rpcExamples := make([]*itemapiexamplev1.ApiExample, len(examples))
	for i, example := range examples {
		header, err := c.hs.GetHeaderByExampleID(ctx, example.ID)
		if err != nil && err != sexampleheader.ErrNoHeaderFound {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		query, err := c.qs.GetExampleQueriesByExampleID(ctx, example.ID)
		if err != nil && err != sexamplequery.ErrNoQueryFound {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		rpcHeaders := tgeneric.MassConvert(header, theader.SerializeHeaderModelToRPC)
		rpcQueries := tgeneric.MassConvert(query, tquery.SerializeQueryModelToRPC)

		rpcExamples[i] = &itemapiexamplev1.ApiExample{
			Meta: &itemapiexamplev1.ApiExampleMeta{
				Id:   example.ID.String(),
				Name: example.Name,
			},
			Header: rpcHeaders,
			Query:  rpcQueries,
			Body: &bodyv1.Body{
				Value: &bodyv1.Body_Raw{
					Raw: &bodyv1.BodyRaw{
						BodyBytes: nil,
					},
				},
			},
			Updated: timestamppb.New(example.Updated),
		}
	}

	return connect.NewResponse(&itemapiexamplev1.GetExamplesResponse{
		Examples: rpcExamples,
	}), nil
}

func (c *ItemAPIExampleRPC) GetExample(ctx context.Context, req *connect.Request[itemapiexamplev1.GetExampleRequest]) (*connect.Response[itemapiexamplev1.GetExampleResponse], error) {
	exampleIdWrap, err := idwrap.NewWithParse(req.Msg.GetId())
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

	headers, err := c.hs.GetHeaderByExampleID(ctx, exampleIdWrap)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	queries, err := c.qs.GetExampleQueriesByExampleID(ctx, exampleIdWrap)
	if err != nil && err != sexamplequery.ErrNoQueryFound {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	asserts, err := c.as.GetAssertByExampleID(ctx, exampleIdWrap)
	if err != nil && err != sassert.ErrNoAssertFound {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	assertsRPC := tgeneric.MassConvert(asserts, tassert.SerializeAssertModelToRPC)

	body, err := tbodyraw.SerializeModelToRPC(ctx, *example, c.brs, c.bfs, c.bues)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var resp *itemapiexamplev1.ApiExampleResponse = nil

	exampleResp, err := c.ers.GetExampleRespByExampleID(ctx, exampleIdWrap)
	if err != nil && err != sexampleresp.ErrNoRespFound {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if exampleResp != nil {
		respHeaders, err := c.erhs.GetHeaderByRespID(ctx, exampleResp.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		resp, err = texampleresp.SeralizeModelToRPC(*exampleResp, respHeaders)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&itemapiexamplev1.GetExampleResponse{
		Example: texample.SerializeModelToRPC(*example, queries, headers, body, resp, assertsRPC),
	}), nil
}

func (c *ItemAPIExampleRPC) DupeExample(ctx context.Context, req *connect.Request[itemapiexamplev1.DupeExampleRequest]) (*connect.Response[itemapiexamplev1.DupeExampleResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("unimplemented"))
}

func (c *ItemAPIExampleRPC) CreateExample(ctx context.Context, req *connect.Request[itemapiexamplev1.CreateExampleRequest]) (*connect.Response[itemapiexamplev1.CreateExampleResponse], error) {
	apiIDWrap, err := idwrap.NewWithParse(req.Msg.GetItemApiId())
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
	exampleRPC := req.Msg.Example
	metaRPC := exampleRPC.GetMeta()
	ex := &mitemapiexample.ItemApiExample{
		ID:           ExampleIDWrapNew,
		ItemApiID:    apiIDWrap,
		CollectionID: itemApi.CollectionID,
		Name:         metaRPC.GetName(),
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

	return connect.NewResponse(&itemapiexamplev1.CreateExampleResponse{
		Id: ExampleIDWrapNew.String(),
	}), nil
}

func (c *ItemAPIExampleRPC) UpdateExample(ctx context.Context, req *connect.Request[itemapiexamplev1.UpdateExampleRequest]) (*connect.Response[itemapiexamplev1.UpdateExampleResponse], error) {
	exampleIDWrap, err := idwrap.NewWithParse(req.Msg.GetId())
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

	bodyType := mitemapiexample.BodyTypeUndefined
	switch req.Msg.BodyType.Value.(type) {
	case *bodyv1.Body_None:
		bodyType = mitemapiexample.BodyTypeNone
	case *bodyv1.Body_Raw:
		bodyType = mitemapiexample.BodyTypeRaw
	case *bodyv1.Body_Forms:
		bodyType = mitemapiexample.BodyTypeForm
	case *bodyv1.Body_UrlEncodeds:
		bodyType = mitemapiexample.BodyTypeUrlencoded
	}

	exRPC := req.Msg
	ex := &mitemapiexample.ItemApiExample{
		ID:       exampleIDWrap,
		Name:     exRPC.GetName(),
		BodyType: bodyType,
	}

	err = c.iaes.UpdateItemApiExample(ctx, ex)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&itemapiexamplev1.UpdateExampleResponse{}), nil
}

func (c *ItemAPIExampleRPC) DeleteExample(ctx context.Context, req *connect.Request[itemapiexamplev1.DeleteExampleRequest]) (*connect.Response[itemapiexamplev1.DeleteExampleResponse], error) {
	exampleUlid, err := idwrap.NewWithParse(req.Msg.GetId())
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

	return connect.NewResponse(&itemapiexamplev1.DeleteExampleResponse{}), nil
}

func (c *ItemAPIExampleRPC) RunExample(ctx context.Context, req *connect.Request[itemapiexamplev1.RunExampleRequest]) (*connect.Response[itemapiexamplev1.RunExampleResponse], error) {
	exampleUlid, err := idwrap.NewWithParse(req.Msg.GetId())
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

	currentHeaders, err := c.erhs.GetHeaderByRespID(ctx, exampleResp.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rpcExampleResp, err := texampleresp.SeralizeModelToRPC(*exampleResp, currentHeaders)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	rpcResponse := connect.NewResponse(&itemapiexamplev1.RunExampleResponse{
		Response: rpcExampleResp,
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

func CheckOwnerHeader(ctx context.Context, hs sexampleheader.HeaderService, iaes sitemapiexample.ItemApiExampleService, cs scollection.CollectionService, us suser.UserService, headerUlid idwrap.IDWrap) (bool, error) {
	header, err := hs.GetHeaderByID(ctx, headerUlid)
	if err != nil {
		return false, err
	}
	return CheckOwnerExample(ctx, iaes, cs, us, header.ExampleID)
}

func CheckOwnerQuery(ctx context.Context, qs sexamplequery.ExampleQueryService, iaes sitemapiexample.ItemApiExampleService, cs scollection.CollectionService, us suser.UserService, queryUlid idwrap.IDWrap) (bool, error) {
	query, err := qs.GetExampleQuery(ctx, queryUlid)
	if err != nil {
		return false, err
	}
	return CheckOwnerExample(ctx, iaes, cs, us, query.ExampleID)
}

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
