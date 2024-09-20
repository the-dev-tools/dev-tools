package ritemapiexample

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/collection"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/internal/api/middleware/mwcompress"
	"dev-tools-backend/internal/api/ritemapi"
	"dev-tools-backend/pkg/idwrap"
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
	"dev-tools-backend/pkg/translate/tgeneric"
	"dev-tools-backend/pkg/translate/theader"
	"dev-tools-backend/pkg/translate/tquery"
	"dev-tools-nodes/pkg/model/mnode"
	"dev-tools-nodes/pkg/model/mnodedata"
	"dev-tools-nodes/pkg/model/mnodemaster"
	"dev-tools-nodes/pkg/nodes/nodeapi"
	apiresultv1 "dev-tools-services/gen/apiresult/v1"
	bodyv1 "dev-tools-services/gen/body/v1"
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
		Body: &bodyv1.Body{
			Value: &bodyv1.Body_Raw{
				Raw: &bodyv1.BodyRaw{
					BodyBytes: nil,
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
		Body:    nil,
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
