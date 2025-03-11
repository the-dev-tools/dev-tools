package nrequest

import (
	"context"
	"encoding/json"
	"the-dev-tools/backend/pkg/flow/edge"
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/http/request"
	"the-dev-tools/backend/pkg/http/response"
	"the-dev-tools/backend/pkg/httpclient"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/massert"
	"the-dev-tools/backend/pkg/model/mbodyform"
	"the-dev-tools/backend/pkg/model/mbodyraw"
	"the-dev-tools/backend/pkg/model/mbodyurl"
	"the-dev-tools/backend/pkg/model/mexampleheader"
	"the-dev-tools/backend/pkg/model/mexamplequery"
	"the-dev-tools/backend/pkg/model/mexampleresp"
	"the-dev-tools/backend/pkg/model/mexamplerespheader"
	"the-dev-tools/backend/pkg/model/mitemapi"
	"the-dev-tools/backend/pkg/model/mitemapiexample"
)

const (
	NodeOutputKey  = "header"
	NodeRequestKey = "response"
)

type NodeRequest struct {
	FlownNodeID idwrap.IDWrap
	Api         mitemapi.ItemApi
	Example     mitemapiexample.ItemApiExample
	Queries     []mexamplequery.Query
	Headers     []mexampleheader.Header

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

func New(id idwrap.IDWrap, api mitemapi.ItemApi, example mitemapiexample.ItemApiExample,
	Queries []mexamplequery.Query, Headers []mexampleheader.Header,
	rawBody mbodyraw.ExampleBodyRaw, formBody []mbodyform.BodyForm, urlBody []mbodyurl.BodyURLEncoded,
	ExampleResp mexampleresp.ExampleResp, ExampleRespHeader []mexamplerespheader.ExampleRespHeader, asserts []massert.Assert,
	Httpclient httpclient.HttpClient, NodeRequestSideRespChan chan NodeRequestSideResp,
) *NodeRequest {
	return &NodeRequest{
		FlownNodeID: id,
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

func (nr *NodeRequest) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	nextID := edge.GetNextNodeID(req.EdgeSourceMap, nr.GetID(), edge.HandleUnspecified)
	result := node.FlowNodeResult{
		NextNodeID: nextID,
		Err:        nil,
	}

	resp, err := request.PrepareRequest(nr.Api, nr.Example,
		nr.Queries, nr.Headers, nr.RawBody, nr.FormBody, nr.UrlBody, nil, nr.HttpClient)
	if err != nil {
		result.Err = err
		return result
	}

	varResp := httpclient.ConvertResponseToVar(resp.HttpResp)
	respMap := map[string]interface{}{}
	marshaledResp, err := json.Marshal(varResp)
	if err != nil {
		result.Err = err
		return result
	}
	err = json.Unmarshal(marshaledResp, &respMap)
	if err != nil {
		result.Err = err
		return result
	}

	err = node.WriteNodeVar(req, nr.GetID(), NodeRequestKey, respMap)
	if err != nil {
		result.Err = err
		return result
	}

	respCreate, err := response.ResponseCreate(ctx, *resp, nr.ExampleResp, nr.ExampleRespHeader, nr.ExampleAsserts)
	if err != nil {
		result.Err = err
		return result
	}

	nr.NodeRequestSideRespChan <- NodeRequestSideResp{
		Example: nr.Example,
		Queries: nr.Queries,
		Headers: nr.Headers,

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

	// TODO: varMap is null create varMap
	resp, err := request.PrepareRequest(nr.Api, nr.Example,
		nr.Queries, nr.Headers, nr.RawBody, nr.FormBody, nr.UrlBody, nil, nr.HttpClient)
	if err != nil {
		result.Err = err
		resultChan <- result
		return
	}

	varResp := httpclient.ConvertResponseToVar(resp.HttpResp)
	respMap := map[string]any{}
	// TODO: change map conversion non json
	marshaledResp, err := json.Marshal(varResp)
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

	err = node.WriteNodeVar(req, nr.GetID(), NodeRequestKey, respMap)
	if err != nil {
		result.Err = err
		resultChan <- result
		return
	}

	respCreate, err := response.ResponseCreate(ctx, *resp, nr.ExampleResp, nr.ExampleRespHeader, nr.ExampleAsserts)
	if err != nil {
		result.Err = err
		resultChan <- result
		return
	}

	nr.NodeRequestSideRespChan <- NodeRequestSideResp{
		Example: nr.Example,
		Queries: nr.Queries,
		Headers: nr.Headers,

		RawBody:  nr.RawBody,
		FormBody: nr.FormBody,
		UrlBody:  nr.UrlBody,

		Resp: *respCreate,
	}

	// TODO: add some functionality here
	resultChan <- result
}
