package resultapi

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/service/scollection"
	"dev-tools-backend/pkg/service/sitemapi"
	"dev-tools-backend/pkg/service/sresultapi"
	"dev-tools-backend/pkg/service/sworkspace"
	responsev1 "dev-tools-spec/dist/buf/go/collection/item/response/v1"
	"dev-tools-spec/dist/buf/go/collection/item/response/v1/responsev1connect"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ResultService struct {
	DB  *sql.DB
	cs  scollection.CollectionService
	ias sitemapi.ItemApiService
	ws  sworkspace.WorkspaceService
	ras sresultapi.ResultApiService
}

func New(db *sql.DB, cs scollection.CollectionService, ias sitemapi.ItemApiService, ws sworkspace.WorkspaceService, ras sresultapi.ResultApiService) ResultService {
	return ResultService{
		DB:  db,
		cs:  cs,
		ias: ias,
		ws:  ws,
		ras: ras,
	}
}

func CreateService(srv ResultService, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := responsev1connect.NewResponseServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *ResultService) ResponseGet(ctx context.Context, req *connect.Request[responsev1.ResponseGetRequest]) (*connect.Response[responsev1.ResponseGetResponse], error) {
	ulidID, err := idwrap.NewFromBytes(req.Msg.ResponseId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

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

	result, err := c.ras.GetResultApi(ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	headers := make(map[string]string, len(result.HttpResp.Header))
	for k, v := range result.HttpResp.Header {
		headers[k] = strings.Join(v, ",")
	}
	respRPC := &responsev1.ResponseGetResponse{
		ResponseId: ulidID.Bytes(),
		Status:     int32(result.HttpResp.StatusCode),
		Body:       result.HttpResp.Body,
		Time:       timestamppb.New(result.Time),
		Duration:   int32(result.Duration.Milliseconds()),
	}

	return connect.NewResponse(respRPC), nil
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
