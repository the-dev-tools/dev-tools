package ritemapiexample

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/collection"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/pkg/model/mitemapiexample"
	"dev-tools-backend/pkg/model/result/mresultapi"
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
	"dev-tools-backend/pkg/ulidwrap"
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
	"github.com/oklog/ulid/v2"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ItemAPIExampleRPC struct {
	DB   *sql.DB
	iaes *sitemapiexample.ItemApiExampleService
	ias  *sitemapi.ItemApiService
	ras  *sresultapi.ResultApiService
	cs   *scollection.CollectionService
	us   *suser.UserService
	hs   *sexampleheader.HeaderService
	qs   *sexamplequery.ExampleQueryService
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

	authInterceptor := mwauth.NewAuthInterceptor(secret)
	interceptors := connect.WithInterceptors(authInterceptor)
	server := &ItemAPIExampleRPC{
		DB:   db,
		iaes: iaes,
		ias:  ias,
		ras:  ras,
		cs:   cs,
		us:   us,
		hs:   hs,
		qs:   qs,
	}

	path, handler := itemapiexamplev1connect.NewItemApiExampleServiceHandler(server, interceptors)
	return &api.Service{Path: path, Handler: handler}, nil
}

// TODO: check permissions
func (c *ItemAPIExampleRPC) GetExamples(ctx context.Context, req *connect.Request[itemapiexamplev1.GetExamplesRequest]) (*connect.Response[itemapiexamplev1.GetExamplesResponse], error) {
	apiUlid, err := ulid.Parse(req.Msg.GetItemApiId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid item api id"))
	}

	examples, err := c.iaes.GetApiExamples(ctx, apiUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rpcExamples := make([]*itemapiexamplev1.ApiExample, len(examples))
	for i, example := range examples {
		exampleUlidWrap := ulidwrap.New(example.ID)

		header, err := c.hs.GetHeaderByExampleID(ctx, example.ID)
		if err != nil && err != sexampleheader.ErrNoHeaderFound {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		query, err := c.qs.GetExampleQueriesByExampleID(ctx, exampleUlidWrap)
		if err != nil && err != sexamplequery.ErrNoQueryFound {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		rpcHeaders := tgeneric.MassConvert(header, theader.SerializeHeaderModelToRPC)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		rpcQueries := tgeneric.MassConvert(query, tquery.SerializeQueryModelToRPC)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

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
	exampleUlid, err := ulid.Parse(req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid item api id"))
	}

	exampleUlidWrap := ulidwrap.New(exampleUlid)

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

	header, err := c.hs.GetHeaderByExampleID(ctx, exampleUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	query, err := c.qs.GetExampleQueriesByExampleID(ctx, exampleUlidWrap)
	if err != nil && err != sexamplequery.ErrNoQueryFound {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rpcHeaders := tgeneric.MassConvert(header, theader.SerializeHeaderModelToRPC)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rpcQueries := tgeneric.MassConvert(query, tquery.SerializeQueryModelToRPC)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

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
	apiUlid, err := ulid.Parse(req.Msg.GetItemApiId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid item api id"))
	}

	itemApi, err := c.ias.GetItemApi(ctx, apiUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	exampleRPC := req.Msg.Example
	metaRPC := exampleRPC.GetMeta()
	ex := &mitemapiexample.ItemApiExample{
		ID:           ulid.Make(),
		ItemApiID:    apiUlid,
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

	return connect.NewResponse(&itemapiexamplev1.CreateExampleResponse{}), nil
}

func (c *ItemAPIExampleRPC) UpdateExample(ctx context.Context, req *connect.Request[itemapiexamplev1.UpdateExampleRequest]) (*connect.Response[itemapiexamplev1.UpdateExampleResponse], error) {
	exampleUlid, err := ulid.Parse(req.Msg.GetId())
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

	exRPC := req.Msg
	ex := &mitemapiexample.ItemApiExample{
		ID:   exampleUlid,
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
	exampleUlid, err := ulid.Parse(req.Msg.GetId())
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
	exampleUlid, err := ulid.Parse(req.Msg.GetId())
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

	apiCallNodeData := mnodedata.NodeApiRestData{
		Url:    itemApiCall.Url,
		Method: itemApiCall.Method,
		Body:   example.Body,
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
	err = nodeapi.SendRestApiRequest(nm)
	if err != nil {
		return nil, connect.NewError(connect.CodeAborted, err)
	}
	lapse := time.Since(now)

	httpResp, err := nodeapi.GetHttpVarResponse(nm)
	if err != nil {
		return nil, connect.NewError(connect.CodeAborted, err)
	}

	bodyData, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	result := &mresultapi.MResultAPI{
		ID:          ulid.Make(),
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
	headers := make(map[string]string, 0)
	for key, values := range httpResp.Header {
		headers[key] = strings.Join(values, ",")
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
					Header:     headers,
					Body:       result.HttpResp.Body,
				},
			},
		},
	}), nil
}

func (c *ItemAPIExampleRPC) CheckOwnerExample(ctx context.Context, exampleUlid ulid.ULID) (bool, error) {
	example, err := c.iaes.GetApiExample(ctx, exampleUlid)
	if err != nil {
		return false, err
	}
	return collection.CheckOwnerCollection(ctx, *c.cs, *c.us, example.CollectionID)
}

// Headers
func (c *ItemAPIExampleRPC) CreateHeader(ctx context.Context, req *connect.Request[itemapiexamplev1.CreateHeaderRequest]) (*connect.Response[itemapiexamplev1.CreateHeaderResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (c *ItemAPIExampleRPC) UpdateHeader(ctx context.Context, req *connect.Request[itemapiexamplev1.UpdateHeaderRequest]) (*connect.Response[itemapiexamplev1.UpdateHeaderResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

// DeleteHeader calls itemapiexample.v1.ItemApiExampleService.DeleteHeader.
func (c *ItemAPIExampleRPC) DeleteHeader(ctx context.Context, req *connect.Request[itemapiexamplev1.DeleteHeaderRequest]) (*connect.Response[itemapiexamplev1.DeleteHeaderResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

// CreateQuery calls itemapiexample.v1.ItemApiExampleService.CreateQuery.
func (c *ItemAPIExampleRPC) CreateQuery(ctx context.Context, req *connect.Request[itemapiexamplev1.CreateQueryRequest]) (*connect.Response[itemapiexamplev1.CreateQueryResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

// UpdateQuery calls itemapiexample.v1.ItemApiExampleService.UpdateQuery.
func (c *ItemAPIExampleRPC) UpdateQuery(ctx context.Context, req *connect.Request[itemapiexamplev1.UpdateQueryRequest]) (*connect.Response[itemapiexamplev1.UpdateQueryResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

// DeleteQuery calls itemapiexample.v1.ItemApiExampleService.DeleteQuery.
func (c *ItemAPIExampleRPC) DeleteQuery(ctx context.Context, req *connect.Request[itemapiexamplev1.DeleteQueryRequest]) (*connect.Response[itemapiexamplev1.DeleteQueryResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

// CreateBodyForm calls itemapiexample.v1.ItemApiExampleService.CreateBodyForm.
func (c *ItemAPIExampleRPC) CreateBodyForm(ctx context.Context, req *connect.Request[itemapiexamplev1.CreateBodyFormRequest]) (*connect.Response[itemapiexamplev1.CreateBodyFormResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

// UpdateBodyForm calls itemapiexample.v1.ItemApiExampleService.UpdateBodyForm.
func (c *ItemAPIExampleRPC) UpdateBodyForm(ctx context.Context, req *connect.Request[itemapiexamplev1.UpdateBodyFormRequest]) (*connect.Response[itemapiexamplev1.UpdateBodyFormResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

// DeleteBodyForm calls itemapiexample.v1.ItemApiExampleService.DeleteBodyForm.
func (c *ItemAPIExampleRPC) DeleteBodyForm(ctx context.Context, req *connect.Request[itemapiexamplev1.DeleteBodyFormRequest]) (*connect.Response[itemapiexamplev1.DeleteBodyFormResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}
