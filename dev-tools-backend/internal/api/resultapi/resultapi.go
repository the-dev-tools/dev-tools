package resultapi

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/internal/api/ritemapiexample"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/permcheck"
	"dev-tools-backend/pkg/service/scollection"
	"dev-tools-backend/pkg/service/sexampleresp"
	"dev-tools-backend/pkg/service/sitemapi"
	"dev-tools-backend/pkg/service/sitemapiexample"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/service/sworkspace"
	"dev-tools-backend/pkg/translate/texampleresp"
	responsev1 "dev-tools-spec/dist/buf/go/collection/item/response/v1"
	"dev-tools-spec/dist/buf/go/collection/item/response/v1/responsev1connect"
	"errors"

	"connectrpc.com/connect"
)

type ResultService struct {
	DB   *sql.DB
	us   suser.UserService
	cs   scollection.CollectionService
	ias  sitemapi.ItemApiService
	iaes sitemapiexample.ItemApiExampleService
	ws   sworkspace.WorkspaceService
	ers  sexampleresp.ExampleRespService
}

func New(db *sql.DB, us suser.UserService, cs scollection.CollectionService, ias sitemapi.ItemApiService, iaes sitemapiexample.ItemApiExampleService, ws sworkspace.WorkspaceService, ers sexampleresp.ExampleRespService) ResultService {
	return ResultService{
		DB:   db,
		us:   us,
		cs:   cs,
		ias:  ias,
		iaes: iaes,
		ws:   ws,
		ers:  ers,
	}
}

func CreateService(srv ResultService, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := responsev1connect.NewResponseServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *ResultService) ResponseGet(ctx context.Context, req *connect.Request[responsev1.ResponseGetRequest]) (*connect.Response[responsev1.ResponseGetResponse], error) {
	ResponseID, err := idwrap.NewFromBytes(req.Msg.ResponseId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	result, err := c.ers.GetExampleResp(ctx, ResponseID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	exampleID := result.ExampleID
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exampleID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	rpcResp, err := texampleresp.SeralizeModelToRPC(*result)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	resp := &responsev1.ResponseGetResponse{
		ResponseId: rpcResp.ResponseId,
		Status:     rpcResp.Status,
		Body:       rpcResp.Body,
		Time:       rpcResp.Time,
		Duration:   rpcResp.Duration,
	}

	return connect.NewResponse(resp), nil
}

func (c *ResultService) ResponseHeaderList(ctx context.Context, req *connect.Request[responsev1.ResponseHeaderListRequest]) (*connect.Response[responsev1.ResponseHeaderListResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (c *ResultService) ResponseAssertList(ctx context.Context, req *connect.Request[responsev1.ResponseAssertListRequest]) (*connect.Response[responsev1.ResponseAssertListResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

/*
func (c *ResultService) GetResults(ctx context.Context, req *connect.Request[apiresultv1.GetResultsRequest]) (*connect.Response[apiresultv1.GetResultsResponse], error) {
	ulidID, err := idwrap.NewWithParse(req.Msg.GetTriggerBy())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	triggerType := mresultapi.TriggerType(req.Msg.GetTriggerType())
	workspaceID, err := c.ras.GetWorkspaceID(ctx, ulidID, c.cs, c.ias)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	userUlid, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	_, err = c.ws.GetByIDandUserID(ctx, workspaceID, userUlid)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("workspace not found"))
	}

	results, err := c.ras.GetResultsApiWithTriggerBy(ctx, ulidID, triggerType)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resultsProto := make([]*apiresultv1.Result, len(results))
	for i, result := range results {
		resultsProto[i] = convertResultToProto(&result)
	}
	return connect.NewResponse(&apiresultv1.GetResultsResponse{Results: resultsProto}), nil
}

func convertResultToProto(result *mresultapi.MResultAPI) *apiresultv1.Result {
	headers := make(map[string]string, len(result.HttpResp.Header))
	for k, v := range result.HttpResp.Header {
		headers[k] = strings.Join(v, ",")
	}
	return &apiresultv1.Result{
		Id:          result.ID.String(),
		TriggerType: apiresultv1.TriggerType(int32(result.TriggerType)),
		TriggerBy:   result.TriggerBy.String(),
		Name:        result.Name,
		Time:        result.Time.Unix(),
		Duration:    result.Duration.Milliseconds(),
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
	}
}
*/

func (c *ResultService) CheckOwnerWorkspace(ctx context.Context, workspaceID idwrap.IDWrap) (bool, error) {
	userUlid, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return false, connect.NewError(connect.CodeInternal, err)
	}
	_, err = c.ws.GetByIDandUserID(ctx, workspaceID, userUlid)
	if err != nil {
		if err == sql.ErrNoRows {
			// INFO: this mean that workspace not belong to user
			// So for avoid information leakage, we should return not found
			return false, connect.NewError(connect.CodeNotFound, errors.New("workspace not found"))
		}
	}
	return true, nil
}

func (c *ResultService) CheckOwnerCollection(ctx context.Context, collectionID idwrap.IDWrap) (bool, error) {
	workspaceID, err := c.cs.GetOwner(ctx, collectionID)
	if err != nil {
		return false, connect.NewError(connect.CodeInternal, err)
	}

	return c.CheckOwnerWorkspace(ctx, workspaceID)
}
