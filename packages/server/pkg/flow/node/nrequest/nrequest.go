package nrequest

import (
	"context"
	"encoding/json"
	"fmt"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/http/request"
	"the-dev-tools/server/pkg/http/response"
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mexamplerespheader"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/varsystem"
)

type NodeRequest struct {
	FlownNodeID idwrap.IDWrap
	Name        string

	Api     mitemapi.ItemApi
	Example mitemapiexample.ItemApiExample
	Queries []mexamplequery.Query
	Headers []mexampleheader.Header

	RawBody        mbodyraw.ExampleBodyRaw
	FormBody       []mbodyform.BodyForm
	UrlBody        []mbodyurl.BodyURLEncoded
	ExampleAsserts []massert.Assert

	ExampleResp       mexampleresp.ExampleResp
	ExampleRespHeader []mexamplerespheader.ExampleRespHeader

	HttpClient              httpclient.HttpClient
	NodeRequestSideRespChan chan NodeRequestSideResp
}

type NodeRequestSideResp struct {
	// Execution tracking
	ExecutionID idwrap.IDWrap // The specific execution ID for this request

	// Request
	Example mitemapiexample.ItemApiExample
	Queries []mexamplequery.Query
	Headers []mexampleheader.Header

	RawBody  mbodyraw.ExampleBodyRaw
	FormBody []mbodyform.BodyForm
	UrlBody  []mbodyurl.BodyURLEncoded

	// Resp
	Resp response.ResponseCreateOutput
}

const (
	OUTPUT_RESPONE_NAME = "response"
	OUTPUT_REQUEST_NAME = "request"
)

type NodeRequestOutput struct {
	Request  request.RequestResponseVar `json:"request"`
	Response httpclient.ResponseVar     `json:"response"`
}

func New(id idwrap.IDWrap, name string, api mitemapi.ItemApi, example mitemapiexample.ItemApiExample,
	Queries []mexamplequery.Query, Headers []mexampleheader.Header,
	rawBody mbodyraw.ExampleBodyRaw, formBody []mbodyform.BodyForm, urlBody []mbodyurl.BodyURLEncoded,
	ExampleResp mexampleresp.ExampleResp, ExampleRespHeader []mexamplerespheader.ExampleRespHeader, asserts []massert.Assert,
	Httpclient httpclient.HttpClient, NodeRequestSideRespChan chan NodeRequestSideResp,
) *NodeRequest {
	return &NodeRequest{
		FlownNodeID: id,
		Name:        name,
		Api:         api,
		Example:     example,

		Headers: Headers,
		Queries: Queries,

		RawBody:  rawBody,
		FormBody: formBody,
		UrlBody:  urlBody,

		ExampleResp:       ExampleResp,
		ExampleRespHeader: ExampleRespHeader,
		ExampleAsserts:    asserts,

		HttpClient:              Httpclient,
		NodeRequestSideRespChan: NodeRequestSideRespChan,
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

	nr.ExampleResp.ID = idwrap.NewNow()

	// TODO: varMap is null create varMap
	// Create a deep copy of VarMap to prevent concurrent access issues
	varMapCopy := node.DeepCopyVarMap(req)
	varMap := varsystem.NewVarMapFromAnyMap(varMapCopy)

	prepareResult, err := request.PrepareRequestWithTracking(nr.Api, nr.Example,
		nr.Queries, nr.Headers, nr.RawBody, nr.FormBody, nr.UrlBody, varMap)
	if err != nil {
		result.Err = err
		return result
	}

	prepareOutput := prepareResult.Request
	inputVars := prepareResult.ReadVars

	// Track variable reads if tracker is available
	if req.VariableTracker != nil {
		for varKey, varValue := range inputVars {
			req.VariableTracker.TrackRead(varKey, varValue)
		}
	}

	if ctx.Err() != nil {
		return result
	}

	resp, err := request.SendRequestWithContext(ctx, prepareOutput, nr.Example.ID, nr.HttpClient)
	if err != nil {
		result.Err = err
		return result
	}

	if ctx.Err() != nil {
		return result
	}

	output := NodeRequestOutput{
		Request:  request.ConvertRequestToVar(prepareOutput),
		Response: httpclient.ConvertResponseToVar(resp.HttpResp),
	}

	respMap := map[string]any{}
	// TODO: change map conversion non json
	marshaledResp, err := json.Marshal(output)
	if err != nil {
		result.Err = err
		return result
	}
	err = json.Unmarshal(marshaledResp, &respMap)
	if err != nil {
		result.Err = err
		return result
	}

	if req.VariableTracker != nil {
		err = node.WriteNodeVarBulkWithTracking(req, nr.Name, respMap, req.VariableTracker)
	} else {
		err = node.WriteNodeVarBulk(req, nr.Name, respMap)
	}
	if err != nil {
		result.Err = err
		return result
	}

	respCreate, err := response.ResponseCreate(ctx, *resp, nr.ExampleResp, nr.ExampleRespHeader, nr.ExampleAsserts, varMap)
	if err != nil {
		result.Err = err
		return result
	}

	if ctx.Err() != nil {
		return result
	}

	// Check if any assertions failed
	for _, assertCouple := range respCreate.AssertCouples {
		if !assertCouple.AssertRes.Result {
			result.Err = fmt.Errorf("assertion failed: %s", assertCouple.Assert.Condition.Comparisons.Expression)
			// Still send the response data even though we're failing
			nr.NodeRequestSideRespChan <- NodeRequestSideResp{
				ExecutionID: req.ExecutionID,
				Example:     nr.Example,
				Queries:     nr.Queries,
				Headers:     nr.Headers,

				RawBody:  nr.RawBody,
				FormBody: nr.FormBody,
				UrlBody:  nr.UrlBody,

				Resp: *respCreate,
			}
			return result
		}
	}

	nr.NodeRequestSideRespChan <- NodeRequestSideResp{
		ExecutionID: req.ExecutionID,
		Example:     nr.Example,
		Queries:     nr.Queries,
		Headers:     nr.Headers,

		RawBody:  nr.RawBody,
		FormBody: nr.FormBody,
		UrlBody:  nr.UrlBody,

		Resp: *respCreate,
	}

	return result
}

func (nr *NodeRequest) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	nextID := edge.GetNextNodeID(req.EdgeSourceMap, nr.GetID(), edge.HandleUnspecified)
	result := node.FlowNodeResult{
		NextNodeID: nextID,
		Err:        nil,
	}

	nr.ExampleResp.ID = idwrap.NewNow()

	// TODO: varMap is null create varMap
	// Create a deep copy of VarMap to prevent concurrent access issues
	varMapCopy := node.DeepCopyVarMap(req)
	varMap := varsystem.NewVarMapFromAnyMap(varMapCopy)

	prepareResult, err := request.PrepareRequestWithTracking(nr.Api, nr.Example,
		nr.Queries, nr.Headers, nr.RawBody, nr.FormBody, nr.UrlBody, varMap)
	if err != nil {
		result.Err = err
		resultChan <- result
		return
	}

	prepareOutput := prepareResult.Request
	inputVars := prepareResult.ReadVars

	// Track variable reads if tracker is available
	if req.VariableTracker != nil {
		for varKey, varValue := range inputVars {
			req.VariableTracker.TrackRead(varKey, varValue)
		}
	}

	if ctx.Err() != nil {
		return
	}

	resp, err := request.SendRequestWithContext(ctx, prepareOutput, nr.Example.ID, nr.HttpClient)
	if err != nil {
		result.Err = err
		resultChan <- result
		return
	}

	if ctx.Err() != nil {
		return
	}

	output := NodeRequestOutput{
		Request:  request.ConvertRequestToVar(prepareOutput),
		Response: httpclient.ConvertResponseToVar(resp.HttpResp),
	}

	respMap := map[string]any{}
	// TODO: change map conversion non json
	marshaledResp, err := json.Marshal(output)
	if err != nil {
		result.Err = err
		resultChan <- result
		return
	}
	err = json.Unmarshal(marshaledResp, &respMap)
	if err != nil {
		result.Err = err
		resultChan <- result
		return
	}

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

	respCreate, err := response.ResponseCreate(ctx, *resp, nr.ExampleResp, nr.ExampleRespHeader, nr.ExampleAsserts, varMap)
	if err != nil {
		result.Err = err
		resultChan <- result
		return
	}

	nr.ExampleResp.ID = idwrap.NewNow()

	// Check if any assertions failed
	for _, assertCouple := range respCreate.AssertCouples {
		if !assertCouple.AssertRes.Result {
			result.Err = fmt.Errorf("assertion failed: %s", assertCouple.Assert.Condition.Comparisons.Expression)
			// Still send the response data even though we're failing
			nr.NodeRequestSideRespChan <- NodeRequestSideResp{
				ExecutionID: req.ExecutionID,
				Example:     nr.Example,
				Queries:     nr.Queries,
				Headers:     nr.Headers,

				RawBody:  nr.RawBody,
				FormBody: nr.FormBody,
				UrlBody:  nr.UrlBody,

				Resp: *respCreate,
			}
			resultChan <- result
			return
		}
	}

	nr.NodeRequestSideRespChan <- NodeRequestSideResp{
		ExecutionID: req.ExecutionID,
		Example:     nr.Example,
		Queries:     nr.Queries,
		Headers:     nr.Headers,

		RawBody:  nr.RawBody,
		FormBody: nr.FormBody,
		UrlBody:  nr.UrlBody,

		Resp: *respCreate,
	}
	if ctx.Err() != nil {
		return
	}

	resultChan <- result
}
