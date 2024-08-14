package resultapi

import (
	"context"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/pkg/model/result/mresultapi"
	"dev-tools-backend/pkg/service/sresultapi"
	apiresultv1 "devtools-services/gen/apiresult/v1"
	"devtools-services/gen/apiresult/v1/apiresultv1connect"
	"strings"

	"connectrpc.com/connect"
	"github.com/oklog/ulid/v2"
)

type ResultService struct{}

func (c *ResultService) Get(ctx context.Context, req *connect.Request[apiresultv1.GetRequest]) (*connect.Response[apiresultv1.GetResponse], error) {
	ulidID, err := ulid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	ownerID, err := sresultapi.GetOwnerID(ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	orgID, err := mwauth.GetContextUserOrgID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if ownerID.Compare(*orgID) != 0 {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}

	result, err := sresultapi.GetResultApi(ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	headers := make(map[string]string, len(result.HttpResp.Header))
	for k, v := range result.HttpResp.Header {
		headers[k] = strings.Join(v, ",")
	}
	protoResult := convertResultToProto(result)

	return connect.NewResponse(&apiresultv1.GetResponse{Result: protoResult}), nil
}

func (c *ResultService) GetResults(ctx context.Context, req *connect.Request[apiresultv1.GetResultsRequest]) (*connect.Response[apiresultv1.GetResultsResponse], error) {
	ulidID, err := ulid.Parse(req.Msg.TriggerBy)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	triggerType := mresultapi.TriggerType(req.Msg.TriggerType)
	ownerID, err := sresultapi.GetOwnerID(ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	orgID, err := mwauth.GetContextUserOrgID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if ownerID.Compare(*orgID) != 0 {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}

	results, err := sresultapi.GetResultsApiWithTriggerBy(ulidID, triggerType)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resultsProto := make([]*apiresultv1.Result, len(results))
	for i, result := range results {
		resultsProto[i] = convertResultToProto(result)
	}
	return connect.NewResponse(&apiresultv1.GetResultsResponse{Results: resultsProto}), nil
}

func CreateService() (*api.Service, error) {
	service := &ResultService{}
	path, handler := apiresultv1connect.NewApiResultServiceHandler(service)
	return &api.Service{Path: path, Handler: handler}, nil
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
