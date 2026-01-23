//nolint:revive // exported
package nrequest

import (
	"context"
	"fmt"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/request"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/response"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/httpclient"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/varsystem"
	"log/slog"
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

	// Synchronization
	Done chan struct{}
}

const (
	OUTPUT_RESPONSE_NAME = "response"
	OUTPUT_REQUEST_NAME  = "request"
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
	httpClient httpclient.HttpClient, nodeRequestSideRespChan chan NodeRequestSideResp, logger *slog.Logger,
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

		HttpClient:              httpClient,
		NodeRequestSideRespChan: nodeRequestSideRespChan,
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
	nextID := mflow.GetNextNodeID(req.EdgeSourceMap, nr.GetID(), mflow.HandleUnspecified)
	result := node.FlowNodeResult{
		NextNodeID: nextID,
		Err:        nil,
	}

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

	// Debug: Log that AuxiliaryID is being set in RunSync
	if nr.logger != nil {
		nr.logger.Debug("HTTP node RunSync setting AuxiliaryID",
			"node_id", nr.FlownNodeID.String(),
			"node_name", nr.Name,
			"auxiliary_id", respCreate.HTTPResponse.ID.String(),
		)
	}

	done := make(chan struct{})

	// Check if any assertions failed
	for _, assertRes := range respCreate.ResponseAsserts {
		if !assertRes.Success {
			result.Err = fmt.Errorf("assertion failed: %s", assertRes.Value)

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
				Done:        done,
			}
			select {
			case <-done:
			case <-ctx.Done():
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
		Done:        done,
	}
	select {
	case <-done:
	case <-ctx.Done():
	}

	return result
}

// GetRequiredVariables implements node.VariableIntrospector.
// It extracts all variable references from URL, headers, query params, and body.
func (nr *NodeRequest) GetRequiredVariables() []string {
	var sources []string

	// URL
	sources = append(sources, nr.HttpReq.Url)

	// Headers
	for _, h := range nr.Headers {
		if h.Enabled {
			sources = append(sources, h.Key, h.Value)
		}
	}

	// Query params
	for _, p := range nr.Params {
		if p.Enabled {
			sources = append(sources, p.Key, p.Value)
		}
	}

	// Raw body
	if nr.RawBody != nil && len(nr.RawBody.RawData) > 0 {
		sources = append(sources, string(nr.RawBody.RawData))
	}

	// Form body
	for _, f := range nr.FormBody {
		if f.Enabled {
			sources = append(sources, f.Key, f.Value)
		}
	}

	// URL encoded body
	for _, u := range nr.UrlBody {
		if u.Enabled {
			sources = append(sources, u.Key, u.Value)
		}
	}

	return varsystem.ExtractVarKeysFromMultiple(sources...)
}

// GetOutputVariables implements node.VariableIntrospector.
// Returns the output paths this HTTP node produces.
func (nr *NodeRequest) GetOutputVariables() []string {
	return []string{
		"response.status",
		"response.body",
		"response.headers",
		"response.duration",
		"request.method",
		"request.url",
		"request.headers",
		"request.queries",
		"request.body",
	}
}

func (nr *NodeRequest) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	nextID := mflow.GetNextNodeID(req.EdgeSourceMap, nr.GetID(), mflow.HandleUnspecified)
	result := node.FlowNodeResult{
		NextNodeID: nextID,
		Err:        nil,
	}

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

	done := make(chan struct{})

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
				Done:        done,
			}
			select {
			case <-done:
			case <-ctx.Done():
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
		Done:        done,
	}
	select {
	case <-done:
	case <-ctx.Done():
	}

	if ctx.Err() != nil {
		return
	}

	resultChan <- result
}
