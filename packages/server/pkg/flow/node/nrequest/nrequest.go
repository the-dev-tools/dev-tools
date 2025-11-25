package nrequest

import (
	"context"
	"fmt"
	"log/slog"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/http/request"
	"the-dev-tools/server/pkg/http/response"
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/varsystem"
)

type NodeRequest struct {
	FlownNodeID idwrap.IDWrap
	Name        string

	HttpReq mhttp.HTTP
	Headers []mhttp.HTTPHeader
	Params  []mhttp.HTTPSearchParam

	RawBody  *mhttp.HTTPBodyRaw
	FormBody []mhttp.HTTPBodyForm
	UrlBody  []mhttp.HTTPBodyUrlencoded
	Asserts  []mhttp.HTTPAssert

	HttpClient              httpclient.HttpClient
	NodeRequestSideRespChan chan NodeRequestSideResp
	logger                  *slog.Logger
}

type NodeRequestSideResp struct {
	// Execution tracking
	ExecutionID idwrap.IDWrap // The specific execution ID for this request

	// Request
	HttpReq mhttp.HTTP
	Headers []mhttp.HTTPHeader
	Params  []mhttp.HTTPSearchParam

	RawBody  *mhttp.HTTPBodyRaw
	FormBody []mhttp.HTTPBodyForm
	UrlBody  []mhttp.HTTPBodyUrlencoded

	// Resp
	Resp response.ResponseCreateHTTPOutput
}

const (
	OUTPUT_RESPONSE_NAME = "response"
	OUTPUT_REQUEST_NAME = "request"
)

type NodeRequestOutput struct {
	Request  request.RequestResponseVar `json:"request"`
	Response httpclient.ResponseVar     `json:"response"`
}

func buildNodeRequestOutputMap(output NodeRequestOutput) map[string]any {
	result := make(map[string]any, 2)
	requestMap := map[string]any{
		"method":  output.Request.Method,
		"url":     output.Request.URL,
		"headers": cloneStringMapToAny(output.Request.Headers),
		"queries": cloneStringMapToAny(output.Request.Queries),
		"body":    output.Request.Body,
	}

	responseMap := map[string]any{
		"status":   float64(output.Response.StatusCode),
		"body":     node.DeepCopyValue(output.Response.Body),
		"headers":  cloneStringMapToAny(output.Response.Headers),
		"duration": float64(output.Response.Duration),
	}

	result[OUTPUT_REQUEST_NAME] = requestMap
	result[OUTPUT_RESPONSE_NAME] = responseMap
	return result
}

func cloneStringMapToAny(src map[string]string) map[string]any {
	if len(src) == 0 {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func New(id idwrap.IDWrap, name string,
	httpReq mhttp.HTTP,
	headers []mhttp.HTTPHeader,
	params []mhttp.HTTPSearchParam,
	rawBody *mhttp.HTTPBodyRaw,
	formBody []mhttp.HTTPBodyForm,
	urlBody []mhttp.HTTPBodyUrlencoded,
	asserts []mhttp.HTTPAssert,
	Httpclient httpclient.HttpClient, NodeRequestSideRespChan chan NodeRequestSideResp, logger *slog.Logger,
) *NodeRequest {
	return &NodeRequest{
		FlownNodeID: id,
		Name:        name,

		HttpReq: httpReq,
		Headers: headers,
		Params:  params,

		RawBody:  rawBody,
		FormBody: formBody,
		UrlBody:  urlBody,
		Asserts:  asserts,

		HttpClient:              Httpclient,
		NodeRequestSideRespChan: NodeRequestSideRespChan,
		logger:                  logger,
	}
}

func (nr *NodeRequest) GetID() idwrap.IDWrap {
	return nr.FlownNodeID
}

func (nr *NodeRequest) SetID(id idwrap.IDWrap) {
	nr.FlownNodeID = id
}

func (nr *NodeRequest) GetName() string {
	return nr.Name
}

func (nr *NodeRequest) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	nextID := edge.GetNextNodeID(req.EdgeSourceMap, nr.GetID(), edge.HandleUnspecified)
	result := node.FlowNodeResult{
		NextNodeID: nextID,
		Err:        nil,
	}

	// TODO: varMap is null create varMap
	// Create a deep copy of VarMap to prevent concurrent access issues
	varMapCopy := node.DeepCopyVarMap(req)
	varMap := varsystem.NewVarMapFromAnyMap(varMapCopy)

	prepareResult, err := request.PrepareHTTPRequestWithTracking(nr.HttpReq, nr.Headers,
		nr.Params, nr.RawBody, nr.FormBody, nr.UrlBody, varMap)
	if err != nil {
		result.Err = err
		return result
	}

	prepareOutput := prepareResult.Request
	inputVars := prepareResult.ReadVars

	request.LogPreparedRequest(ctx, nr.logger, req.ExecutionID, nr.FlownNodeID, nr.Name, prepareOutput)

	// Track variable reads if tracker is available
	if req.VariableTracker != nil {
		for varKey, varValue := range inputVars {
			req.VariableTracker.TrackRead(varKey, varValue)
		}
	}

	if ctx.Err() != nil {
		return result
	}

	// Use httpReq.ID as exampleID? Or pass ID from definition?
	// SendRequest expects exampleID for logging/metrics?
	// It's used in `httpclient.SendRequestAndConvert`.
	// I'll pass nr.HttpReq.ID.
	resp, err := request.SendRequestWithContext(ctx, prepareOutput, nr.HttpReq.ID, nr.HttpClient)
	if err != nil {
		result.Err = err
		return result
	}

	if ctx.Err() != nil {
		return result
	}

	// Build output using measured duration
	respVar := httpclient.ConvertResponseToVar(resp.HttpResp)
	respVar.Duration = int32(resp.LapTime.Milliseconds()) // nolint:gosec // G115
	output := NodeRequestOutput{
		Request:  request.ConvertRequestToVar(prepareOutput),
		Response: respVar,
	}

	respMap := buildNodeRequestOutputMap(output)

	if req.VariableTracker != nil {
		err = node.WriteNodeVarBulkWithTracking(req, nr.Name, respMap, req.VariableTracker)
	} else {
		err = node.WriteNodeVarBulk(req, nr.Name, respMap)
	}
	if err != nil {
		result.Err = err
		return result
	}

	respCreate, err := response.ResponseCreateHTTP(ctx, *resp, nr.HttpReq.ID, nr.Asserts, varMap, varMapCopy)
	if err != nil {
		result.Err = err
		return result
	}

	    if ctx.Err() != nil {
	        return result
	    }
	
	    	result.AuxiliaryID = &respCreate.HTTPResponse.ID
	    
	    	// Check if any assertions failed
	    	for _, assertRes := range respCreate.ResponseAsserts {
	    		if !assertRes.Success {			result.Err = fmt.Errorf("assertion failed: %s", assertRes.Value)

			// Still send the response data even though we're failing
			nr.NodeRequestSideRespChan <- NodeRequestSideResp{
				ExecutionID: req.ExecutionID,
				HttpReq:     nr.HttpReq,
				Headers:     nr.Headers,
				Params:      nr.Params,
				RawBody:     nr.RawBody,
				FormBody:    nr.FormBody,
				UrlBody:     nr.UrlBody,
				Resp:        *respCreate,
			}
			return result
		}
	}

	nr.NodeRequestSideRespChan <- NodeRequestSideResp{
		ExecutionID: req.ExecutionID,
		HttpReq:     nr.HttpReq,
		Headers:     nr.Headers,
		Params:      nr.Params,
		RawBody:     nr.RawBody,
		FormBody:    nr.FormBody,
		UrlBody:     nr.UrlBody,
		Resp:        *respCreate,
	}

	return result
}

func (nr *NodeRequest) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	nextID := edge.GetNextNodeID(req.EdgeSourceMap, nr.GetID(), edge.HandleUnspecified)
	result := node.FlowNodeResult{
		NextNodeID: nextID,
		Err:        nil,
	}

	// TODO: varMap is null create varMap
	// Create a deep copy of VarMap to prevent concurrent access issues
	varMapCopy := node.DeepCopyVarMap(req)
	varMap := varsystem.NewVarMapFromAnyMap(varMapCopy)

	prepareResult, err := request.PrepareHTTPRequestWithTracking(nr.HttpReq, nr.Headers,
		nr.Params, nr.RawBody, nr.FormBody, nr.UrlBody, varMap)
	if err != nil {
		result.Err = err
		resultChan <- result
		return
	}

	prepareOutput := prepareResult.Request
	inputVars := prepareResult.ReadVars

	request.LogPreparedRequest(ctx, nr.logger, req.ExecutionID, nr.FlownNodeID, nr.Name, prepareOutput)

	// Track variable reads if tracker is available
	if req.VariableTracker != nil {
		for varKey, varValue := range inputVars {
			req.VariableTracker.TrackRead(varKey, varValue)
		}
	}

	if ctx.Err() != nil {
		return
	}

	resp, err := request.SendRequestWithContext(ctx, prepareOutput, nr.HttpReq.ID, nr.HttpClient)
	if err != nil {
		result.Err = err
		resultChan <- result
		return
	}

	if ctx.Err() != nil {
		return
	}

	// Build output using measured duration
	respVar := httpclient.ConvertResponseToVar(resp.HttpResp)
	respVar.Duration = int32(resp.LapTime.Milliseconds()) // nolint:gosec // G115
	output := NodeRequestOutput{
		Request:  request.ConvertRequestToVar(prepareOutput),
		Response: respVar,
	}

	respMap := buildNodeRequestOutputMap(output)

	if req.VariableTracker != nil {
		err = node.WriteNodeVarBulkWithTracking(req, nr.Name, respMap, req.VariableTracker)
	} else {
		err = node.WriteNodeVarBulk(req, nr.Name, respMap)
	}
	if err != nil {
		result.Err = err
		resultChan <- result
		return
	}

	respCreate, err := response.ResponseCreateHTTP(ctx, *resp, nr.HttpReq.ID, nr.Asserts, varMap, varMapCopy)
	if err != nil {
		result.Err = err
		resultChan <- result
		return
	}

	result.AuxiliaryID = &respCreate.HTTPResponse.ID

	// Check if any assertions failed
	for _, assertRes := range respCreate.ResponseAsserts {
		if !assertRes.Success {
			result.Err = fmt.Errorf("assertion failed: %s", assertRes.Value)
			
			nr.NodeRequestSideRespChan <- NodeRequestSideResp{
				ExecutionID: req.ExecutionID,
				HttpReq:     nr.HttpReq,
				Headers:     nr.Headers,
				Params:      nr.Params,
				RawBody:     nr.RawBody,
				FormBody:    nr.FormBody,
				UrlBody:     nr.UrlBody,
				Resp:        *respCreate,
			}
			resultChan <- result
			return
		}
	}

	nr.NodeRequestSideRespChan <- NodeRequestSideResp{
		ExecutionID: req.ExecutionID,
		HttpReq:     nr.HttpReq,
		Headers:     nr.Headers,
		Params:      nr.Params,
		RawBody:     nr.RawBody,
		FormBody:    nr.FormBody,
		UrlBody:     nr.UrlBody,
		Resp:        *respCreate,
	}
	if ctx.Err() != nil {
		return
	}

	resultChan <- result
}