package nrequest

import (
	"context"
	"encoding/json"
	"fmt"
	"the-dev-tools/backend/pkg/flow/edge"
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/http/request"
	"the-dev-tools/backend/pkg/httpclient"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mbodyform"
	"the-dev-tools/backend/pkg/model/mbodyraw"
	"the-dev-tools/backend/pkg/model/mbodyurl"
	"the-dev-tools/backend/pkg/model/mexampleheader"
	"the-dev-tools/backend/pkg/model/mexamplequery"
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

	RawBody  mbodyraw.ExampleBodyRaw
	FormBody []mbodyform.BodyForm
	UrlBody  []mbodyurl.BodyURLEncoded

	HttpClient httpclient.HttpClient
}

func New(id idwrap.IDWrap, api mitemapi.ItemApi, example mitemapiexample.ItemApiExample,
	Queries []mexamplequery.Query, Headers []mexampleheader.Header,
	rawBody mbodyraw.ExampleBodyRaw, formBody []mbodyform.BodyForm, urlBody []mbodyurl.BodyURLEncoded, Httpclient httpclient.HttpClient,
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

		HttpClient: Httpclient,
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

	err = node.AddNodeVar(req, respMap, nr.GetID(), NodeRequestKey)
	if err != nil {
		result.Err = err
		return result
	}

	return result
}

func (nr *NodeRequest) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	nextID := edge.GetNextNodeID(req.EdgeSourceMap, nr.GetID(), edge.HandleUnspecified)
	result := node.FlowNodeResult{
		NextNodeID: nextID,
		Err:        nil,
	}

	resp, err := request.PrepareRequest(nr.Api, nr.Example,
		nr.Queries, nr.Headers, nr.RawBody, nr.FormBody, nr.UrlBody, nil, nr.HttpClient)
	if err != nil {
		fmt.Println("Error: ", err)
		result.Err = err
		resultChan <- result
		return
	}

	respMap := map[string]interface{}{}
	// TODO: change map conversion non json
	varResp := httpclient.ConvertResponseToVar(resp.HttpResp)
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
