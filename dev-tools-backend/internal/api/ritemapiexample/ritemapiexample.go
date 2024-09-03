package ritemapiexample

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/pkg/model/mitemapiexample"
	"dev-tools-backend/pkg/service/sitemapi"
	"dev-tools-backend/pkg/service/sitemapiexample"
	itemapiexamplev1 "dev-tools-services/gen/itemapiexample/v1"
	"dev-tools-services/gen/itemapiexample/v1/itemapiexamplev1connect"
	"errors"

	"connectrpc.com/connect"
	"github.com/oklog/ulid/v2"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ItemAPIExampleRPC struct {
	DB   *sql.DB
	iaes *sitemapiexample.ItemApiExampleService
	ias  *sitemapi.ItemApiService
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

	authInterceptor := mwauth.NewAuthInterceptor(secret)
	interceptors := connect.WithInterceptors(authInterceptor)
	server := &ItemAPIExampleRPC{
		DB:   db,
		iaes: iaes,
		ias:  ias,
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
		rpcExamples[i] = &itemapiexamplev1.ApiExample{
			Meta: &itemapiexamplev1.ApiExampleMeta{
				Id:   example.ID.String(),
				Name: example.Name,
			},
			Headers: example.Headers.HeaderMap,
			Query:   example.Query.QueryMap,
			Body:    example.Body,
			Created: timestamppb.New(example.GetCreatedTime()),
			Updated: timestamppb.New(example.Updated),
		}
	}

	return connect.NewResponse(&itemapiexamplev1.GetExamplesResponse{
		Examples: rpcExamples,
	}), nil
}

func (c *ItemAPIExampleRPC) GetExample(ctx context.Context, req *connect.Request[itemapiexamplev1.GetExampleRequest]) (*connect.Response[itemapiexamplev1.GetExampleResponse], error) {
	exampleId, err := ulid.Parse(req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid item api id"))
	}

	example, err := c.iaes.GetApiExample(ctx, exampleId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	exampleRPC := &itemapiexamplev1.ApiExample{
		Meta: &itemapiexamplev1.ApiExampleMeta{
			Id:   example.ID.String(),
			Name: example.Name,
		},
		Headers: example.Headers.HeaderMap,
		Query:   example.Query.QueryMap,
		Body:    example.Body,
		Created: timestamppb.New(example.GetCreatedTime()),
		Updated: timestamppb.New(example.Updated),
	}
	return connect.NewResponse(&itemapiexamplev1.GetExampleResponse{
		Example: exampleRPC,
	}), nil
}

func (c *ItemAPIExampleRPC) CreateExample(ctx context.Context, req *connect.Request[itemapiexamplev1.CreateExampleRequest]) (*connect.Response[itemapiexamplev1.CreateExampleResponse], error) {
	apiUlid, err := ulid.Parse(req.Msg.GetItemApiId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid item api id"))
	}

	itemApi, err := c.ias.GetItemApi(ctx, apiUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	exRPC := req.Msg
	ex := &mitemapiexample.ItemApiExample{
		ID:           ulid.Make(),
		ItemApiID:    apiUlid,
		CollectionID: itemApi.CollectionID,
		Name:         exRPC.GetName(),
		Headers:      mitemapiexample.Headers{HeaderMap: exRPC.GetHeaders()},
		Query:        mitemapiexample.Query{QueryMap: exRPC.GetQuery()},
		Body:         exRPC.GetBody(),
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

	exRPC := req.Msg
	ex := &mitemapiexample.ItemApiExample{
		ID:      exampleUlid,
		Name:    exRPC.GetName(),
		Headers: *mitemapiexample.NewHeaders(exRPC.GetHeaders()),
		Query:   *mitemapiexample.NewQuery(exRPC.GetQuery()),
		Cookies: *mitemapiexample.NewCookies(exRPC.GetCookies()),
		Body:    exRPC.GetBody(),
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

	err = c.iaes.DeleteApiExample(ctx, exampleUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&itemapiexamplev1.DeleteExampleResponse{}), nil
}

func (c *ItemAPIExampleRPC) RunExample(ctx context.Context, req *connect.Request[itemapiexamplev1.RunExampleRequest]) (*connect.Response[itemapiexamplev1.RunExampleResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}
