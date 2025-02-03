package nrequest

import (
	"context"
	"encoding/json"
	"the-dev-tools/backend/pkg/flow/edge"
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/httpclient"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mexampleheader"
	"the-dev-tools/backend/pkg/model/mexamplequery"
	"the-dev-tools/backend/pkg/model/mitemapi"
	"the-dev-tools/backend/pkg/model/mitemapiexample"
)

const (
	NodeOutputKey  = "header"
	NodeRequestKey = "request"
)

type NodeRequest struct {
	FlownNodeID idwrap.IDWrap
	Api         mitemapi.ItemApi
	Example     mitemapiexample.ItemApiExample
	Queries     []mexamplequery.Query
	Headers     []mexampleheader.Header
	Body        []byte
	HttpClient  httpclient.HttpClient
}

func New(id idwrap.IDWrap, api mitemapi.ItemApi, example mitemapiexample.ItemApiExample,
	Queries []mexamplequery.Query, Headers []mexampleheader.Header, body []byte, Httpclient httpclient.HttpClient,
) *NodeRequest {
	return &NodeRequest{
		FlownNodeID: id,
		Api:         api,
		Example:     example,
		HttpClient:  Httpclient,
		Headers:     Headers,
		Queries:     Queries,
		Body:        body,
	}
}

func (nr *NodeRequest) GetID() idwrap.IDWrap {
	return nr.FlownNodeID
}

func (nr *NodeRequest) SetID(id idwrap.IDWrap) {
	nr.FlownNodeID = id
}

func (nr *NodeRequest) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	cl := nr.HttpClient
	httpReq := httpclient.Request{
		Method:  nr.Api.Method,
		URL:     nr.Api.Url,
		Queries: nr.Queries,
		Headers: nr.Headers,
		Body:    nr.Body,
	}

	nextID := edge.GetNextNodeID(req.EdgeSourceMap, nr.GetID(), edge.HandleUnspecified)
	result := node.FlowNodeResult{
		NextNodeID: nextID,
		Err:        nil,
	}

	resp, err := httpclient.SendRequestAndConvert(cl, httpReq, nr.Example.ID)
	if err != nil {
		result.Err = err
		return result
	}
	varResp := httpclient.ConvertResponseToVar(resp)
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

	err = node.AddNodeVar(req, respMap, nr.GetID(), NodeRequestKey)
	if err != nil {
		result.Err = err
		return result
	}

	return result
}

func (nr *NodeRequest) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	cl := nr.HttpClient
	httpReq := httpclient.Request{
		Method:  nr.Api.Method,
		URL:     nr.Api.Url,
		Queries: nr.Queries,
		Headers: nr.Headers,
		Body:    nr.Body,
	}
	nextID := edge.GetNextNodeID(req.EdgeSourceMap, nr.GetID(), edge.HandleUnspecified)
	result := node.FlowNodeResult{
		NextNodeID: nextID,
		Err:        nil,
	}

	resp, err := httpclient.SendRequestAndConvert(cl, httpReq, nr.Example.ID)
	if err != nil {
		result.Err = err
		resultChan <- result
		return
	}

	respMap := map[string]interface{}{}
	// TODO: change map conversion non json
	varResp := httpclient.ConvertResponseToVar(resp)
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

	err = node.AddNodeVar(req, respMap, nr.GetID(), NodeRequestKey)
	if err != nil {
		result.Err = err
		resultChan <- result
		return
	}

	// TODO: add some functionality here
	resultChan <- result
}
