package ritemapiexample

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/collection"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/internal/api/middleware/mwcompress"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mbodyform"
	"dev-tools-backend/pkg/model/mitemapiexample"
	"dev-tools-backend/pkg/model/result/mresultapi"
	"dev-tools-backend/pkg/service/sbodyform"
	"dev-tools-backend/pkg/service/scollection"
	"dev-tools-backend/pkg/service/sexampleheader"
	"dev-tools-backend/pkg/service/sexamplequery"
	"dev-tools-backend/pkg/service/sitemapi"
	"dev-tools-backend/pkg/service/sitemapiexample"
	"dev-tools-backend/pkg/service/sresultapi"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/translate/tbodyform"
	"dev-tools-backend/pkg/translate/tgeneric"
	"dev-tools-backend/pkg/translate/theader"
	"dev-tools-backend/pkg/translate/tquery"
	"dev-tools-nodes/pkg/model/mnode"
	"dev-tools-nodes/pkg/model/mnodedata"
	"dev-tools-nodes/pkg/model/mnodemaster"
	"dev-tools-nodes/pkg/nodes/nodeapi"
	apiresultv1 "dev-tools-services/gen/apiresult/v1"
	itemapiexamplev1 "dev-tools-services/gen/itemapiexample/v1"
	"dev-tools-services/gen/itemapiexample/v1/itemapiexamplev1connect"
	"errors"
	"io"
	"net/http"
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
	hs  *sexampleheader.HeaderService
	qs  *sexamplequery.ExampleQueryService
	bfs *sbodyform.BodyFormService
}

func CreateService(ctx context.Context, db *sql.DB, secret []byte) (*api.Service, error) {
	iaes, err := sitemapiexample.New(ctx, db)
	if err != nil {
		return nil, err
	}

	ias, err := sitemapi.New(ctx, db)
	if err != nil {
		return nil, err
	}

	ras, err := sresultapi.New(ctx, db)
	if err != nil {
		return nil, err
	}

	cs, err := scollection.New(ctx, db)
	if err != nil {
		return nil, err
	}

	us, err := suser.New(ctx, db)
	if err != nil {
		return nil, err
	}

	hs, err := sexampleheader.New(ctx, db)
	if err != nil {
		return nil, err
	}

	qs, err := sexamplequery.New(ctx, db)
	if err != nil {
		return nil, err
	}

	bfs, err := sbodyform.New(ctx, db)
	if err != nil {
		return nil, err
	}

	var options []connect.HandlerOption
	options = append(options, connect.WithCompression("zstd", mwcompress.NewDecompress, mwcompress.NewCompress))
	options = append(options, connect.WithCompression("gzip", nil, nil))
	options = append(options, connect.WithInterceptors(mwauth.NewAuthInterceptor(secret)))
	server := &ItemAPIExampleRPC{
		DB:   db,
		iaes: iaes,
		ias:  ias,
		ras:  ras,
		cs:   cs,
		us:   us,
		hs:   hs,
		qs:   qs,
		bfs:  bfs,
	}

	path, handler := itemapiexamplev1connect.NewItemApiExampleServiceHandler(server, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

// TODO: check permissions
func (c *ItemAPIExampleRPC) GetExamples(ctx context.Context, req *connect.Request[itemapiexamplev1.GetExamplesRequest]) (*connect.Response[itemapiexamplev1.GetExamplesResponse], error) {
	apiUlid, err := idwrap.NewWithParse(req.Msg.GetItemApiId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid item api id"))
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
			Body: &itemapiexamplev1.Body{
				Value: &itemapiexamplev1.Body_Raw{
					Raw: &itemapiexamplev1.BodyRawData{
						BodyBytes: example.Body,
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

	isMember, err := c.CheckOwnerExample(ctx, exampleIdWrap)
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

	header, err := c.hs.GetHeaderByExampleID(ctx, exampleIdWrap)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	query, err := c.qs.GetExampleQueriesByExampleID(ctx, exampleIdWrap)
	if err != nil && err != sexamplequery.ErrNoQueryFound {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rpcHeaders := tgeneric.MassConvert(header, theader.SerializeHeaderModelToRPC)
	rpcQueries := tgeneric.MassConvert(query, tquery.SerializeQueryModelToRPC)

	exampleRPC := &itemapiexamplev1.ApiExample{
		Meta: &itemapiexamplev1.ApiExampleMeta{
			Id:   example.ID.String(),
			Name: example.Name,
		},
		Header: rpcHeaders,
		Query:  rpcQueries,
		Body: &itemapiexamplev1.Body{
			Value: &itemapiexamplev1.Body_Raw{
				Raw: &itemapiexamplev1.BodyRawData{
					BodyBytes: example.Body,
				},
			},
		},
		Updated: timestamppb.New(example.Updated),
	}
	return connect.NewResponse(&itemapiexamplev1.GetExampleResponse{
		Example: exampleRPC,
	}), nil
}

// TODO: check permissions
func (c *ItemAPIExampleRPC) CreateExample(ctx context.Context, req *connect.Request[itemapiexamplev1.CreateExampleRequest]) (*connect.Response[itemapiexamplev1.CreateExampleResponse], error) {
	apiIDWrap, err := idwrap.NewWithParse(req.Msg.GetItemApiId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid item api id"))
	}

	itemApi, err := c.ias.GetItemApi(ctx, apiIDWrap)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	ExampleIDWrapNew := idwrap.NewNow()
	exampleRPC := req.Msg.Example
	metaRPC := exampleRPC.GetMeta()
	ex := &mitemapiexample.ItemApiExample{
		ID:           ExampleIDWrapNew,
		ItemApiID:    apiIDWrap,
		CollectionID: itemApi.CollectionID,
		Name:         metaRPC.GetName(),
		// TODO: add the headers and query
		// TODO: add body parse
		Body: []byte{},
	}
	err = c.iaes.CreateApiExample(ctx, ex)
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

	isMember, err := c.CheckOwnerExample(ctx, exampleIDWrap)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isMember {
		// return not found
		return nil, connect.NewError(connect.CodeNotFound, errors.New("not found example"))
	}

	exRPC := req.Msg
	ex := &mitemapiexample.ItemApiExample{
		ID:   exampleIDWrap,
		Name: exRPC.GetName(),
		Body: exRPC.GetBody(),
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

	isMember, err := c.CheckOwnerExample(ctx, exampleUlid)
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
	isMember, err := c.CheckOwnerExample(ctx, exampleUlid)
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

	queries, err := c.qs.GetExampleQueriesByExampleID(ctx, exampleUlid)
	if err != nil && err != sexamplequery.ErrNoQueryFound {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	reqHeaders, err := c.hs.GetHeaderByExampleID(ctx, exampleUlid)
	if err != nil && err != sexampleheader.ErrNoHeaderFound {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	apiCallNodeData := mnodedata.NodeApiRestData{
		Url:     itemApiCall.Url,
		Method:  itemApiCall.Method,
		Body:    example.Body,
		Headers: reqHeaders,
		Query:   queries,
	}

	node := mnode.Node{
		ID:   exampleUlid.String(),
		Type: mnodemaster.ApiCallRest,
		Data: &apiCallNodeData,
	}

	runApiVars := make(map[string]interface{}, 0)

	nm := &mnodemaster.NodeMaster{
		CurrentNode: &node,
		HttpClient:  http.DefaultClient,
		Vars:        runApiVars,
	}

	now := time.Now()
	// TODO: add content encoding like gzip, zstd
	err = nodeapi.SendRestApiRequest(nm)
	lapse := time.Since(now)
	if err != nil {
		return nil, connect.NewError(connect.CodeAborted, err)
	}

	httpResp, err := nodeapi.GetHttpVarResponse(nm)
	if err != nil {
		return nil, connect.NewError(connect.CodeAborted, err)
	}

	bodyData, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	result := &mresultapi.MResultAPI{
		ID:          idwrap.NewNow(),
		TriggerType: mresultapi.TRIGGER_TYPE_COLLECTION,
		TriggerBy:   exampleUlid,
		Name:        itemApiCall.Name,
		Time:        time.Now(),
		Duration:    time.Duration(lapse),
		HttpResp: mresultapi.HttpResp{
			StatusCode: httpResp.StatusCode,
			Proto:      httpResp.Proto,
			ProtoMajor: httpResp.ProtoMajor,
			ProtoMinor: httpResp.ProtoMinor,
			Header:     httpResp.Header,
			Body:       bodyData,
		},
	}

	err = c.ras.CreateResultApi(ctx, result)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// TODO: can be more efficient with init size
	respHeaders := make(map[string]string, 0)
	for key, values := range httpResp.Header {
		respHeaders[key] = strings.Join(values, ",")
	}

	return connect.NewResponse(&itemapiexamplev1.RunExampleResponse{
		Result: &apiresultv1.Result{
			Id:       result.ID.String(),
			Name:     result.Name,
			Time:     result.Time.Unix(),
			Duration: result.Duration.Milliseconds(),
			Response: &apiresultv1.Result_HttpResponse{
				HttpResponse: &apiresultv1.HttpResponse{
					StatusCode: int32(result.HttpResp.StatusCode),
					Proto:      result.HttpResp.Proto,
					ProtoMajor: int32(result.HttpResp.ProtoMajor),
					ProtoMinor: int32(result.HttpResp.ProtoMinor),
					Header:     respHeaders,
					Body:       result.HttpResp.Body,
				},
			},
		},
	}), nil
}

func (c *ItemAPIExampleRPC) CheckOwnerExample(ctx context.Context, exampleUlid idwrap.IDWrap) (bool, error) {
	example, err := c.iaes.GetApiExample(ctx, exampleUlid)
	if err != nil {
		return false, err
	}
	return collection.CheckOwnerCollection(ctx, *c.cs, *c.us, example.CollectionID)
}

func (c *ItemAPIExampleRPC) CheckOwnerHeader(ctx context.Context, headerUlid idwrap.IDWrap) (bool, error) {
	header, err := c.hs.GetHeaderByID(ctx, headerUlid)
	if err != nil {
		return false, err
	}
	return c.CheckOwnerExample(ctx, header.ExampleID)
}

func (c *ItemAPIExampleRPC) CheckOwnerQuery(ctx context.Context, queryUlid idwrap.IDWrap) (bool, error) {
	query, err := c.qs.GetExampleQuery(ctx, queryUlid)
	if err != nil {
		return false, err
	}
	return c.CheckOwnerExample(ctx, query.ExampleID)
}

func (c *ItemAPIExampleRPC) CheckOwnerBodyForm(ctx context.Context, bodyFormUlid idwrap.IDWrap) (bool, error) {
	bodyForm, err := c.bfs.GetBodyForm(ctx, bodyFormUlid)
	if err != nil {
		return false, err
	}
	return c.CheckOwnerExample(ctx, bodyForm.ExampleID)
}

// Headers
// TODO: refactor to use the ulidwrap
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

	ok, err := c.CheckOwnerExample(ctx, headerModel.ExampleID)
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

// TODO: refactor to use the ulidwrap
func (c *ItemAPIExampleRPC) UpdateHeader(ctx context.Context, req *connect.Request[itemapiexamplev1.UpdateHeaderRequest]) (*connect.Response[itemapiexamplev1.UpdateHeaderResponse], error) {
	HeaderModel, err := theader.SerlializeHeaderRPCtoModel(req.Msg.GetHeader())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	ok, err := c.CheckOwnerHeader(ctx, HeaderModel.ID)
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

// TODO: refactor to use the ulidwrap
func (c *ItemAPIExampleRPC) DeleteHeader(ctx context.Context, req *connect.Request[itemapiexamplev1.DeleteHeaderRequest]) (*connect.Response[itemapiexamplev1.DeleteHeaderResponse], error) {
	ulidWrap, err := idwrap.NewWithParse(req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	ok, err := c.CheckOwnerHeader(ctx, ulidWrap)
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
	ok, err := c.CheckOwnerExample(ctx, queryData.ExampleID)
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
	ok, err := c.CheckOwnerQuery(ctx, queryData.ID)
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
	ok, err := c.CheckOwnerQuery(ctx, ulidWrap)
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

// BodyForm
func (c *ItemAPIExampleRPC) CreateBodyForm(ctx context.Context, req *connect.Request[itemapiexamplev1.CreateBodyFormRequest]) (*connect.Response[itemapiexamplev1.CreateBodyFormResponse], error) {
	bodyData := req.Msg.GetBodyForm()
	if bodyData == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("body form is nil"))
	}

	exampleID, err := idwrap.NewWithParse(bodyData.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	ok, err := c.CheckOwnerExample(ctx, exampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no example found"))
	}

	bodyForm := mbodyform.BodyForm{
		ID:          idwrap.NewNow(),
		ExampleID:   exampleID,
		BodyKey:     bodyData.GetKey(),
		Description: bodyData.GetDescription(),
		Enable:      bodyData.GetEnabled(),
		Value:       bodyData.GetValue(),
	}

	err = c.bfs.CreateBodyForm(ctx, &bodyForm)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&itemapiexamplev1.CreateBodyFormResponse{Id: bodyForm.ID.String()}), nil
}

func (c *ItemAPIExampleRPC) UpdateBodyForm(ctx context.Context, req *connect.Request[itemapiexamplev1.UpdateBodyFormRequest]) (*connect.Response[itemapiexamplev1.UpdateBodyFormResponse], error) {
	bodyData := req.Msg.GetBodyForm()
	if bodyData == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("body form is nil"))
	}
	BodyID, err := idwrap.NewWithParse(bodyData.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	ok, err := c.CheckOwnerBodyForm(ctx, BodyID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no body form found"))
	}

	bodyModel, err := tbodyform.SerializeFormRPCtoModel(bodyData)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	err = c.bfs.UpdateBodyForm(ctx, bodyModel)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&itemapiexamplev1.UpdateBodyFormResponse{}), nil
}

func (c *ItemAPIExampleRPC) DeleteBodyForm(ctx context.Context, req *connect.Request[itemapiexamplev1.DeleteBodyFormRequest]) (*connect.Response[itemapiexamplev1.DeleteBodyFormResponse], error) {
	ID, err := idwrap.NewWithParse(req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	ok, err := c.CheckOwnerBodyForm(ctx, ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no body form found"))
	}

	err = c.bfs.DeleteBodyForm(ctx, ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}
