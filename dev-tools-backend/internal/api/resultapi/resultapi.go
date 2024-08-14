package resultapi

import (
	"context"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/pkg/service/scollection/sitemapi"
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

	resultOwner, err := sresultapi.GetOwnerID(ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	ownerID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if resultOwner.Compare(*ownerID) != 0 {
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

	resp := &apiresultv1.GetResponse{
		Result: &apiresultv1.Result{
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
		},
	}

	return connect.NewResponse(resp), nil
}

func (c *ResultService) GetResults(ctx context.Context, req *connect.Request[apiresultv1.GetResultsRequest]) (*connect.Response[apiresultv1.GetResultsResponse], error) {
	ulidID, err := ulid.Parse(req.Msg.TriggerBy)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	switch req.Msg.TriggerType {
	case apiresultv1.TriggerType_TRIGGER_TYPE_COLLECTION:
		ownerID, err := sitemapi.GetOwnerID(ulidID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		userID, err := mwauth.GetContextUserID(ctx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if ownerID.Compare(*userID) != 0 {
			return nil, connect.NewError(connect.CodePermissionDenied, nil)
		}
		break
	default:
		return nil, connect.NewError(connect.CodeUnimplemented, nil)
	}

	results, err := sresultapi.GetResultsApiWithTriggerBy(ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rawResults := make([]*apiresultv1.Result, len(results))
	for i, result := range results {
		headers := make(map[string]string, len(result.HttpResp.Header))
		for k, v := range result.HttpResp.Header {
			headers[k] = strings.Join(v, ",")
		}
		rawResults[i] = &apiresultv1.Result{
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
	return connect.NewResponse(&apiresultv1.GetResultsResponse{Results: rawResults}), nil
}

func CreateService() (*api.Service, error) {
	service := &ResultService{}
	path, handler := apiresultv1connect.NewApiResultServiceHandler(service)
	return &api.Service{Path: path, Handler: handler}, nil
}
