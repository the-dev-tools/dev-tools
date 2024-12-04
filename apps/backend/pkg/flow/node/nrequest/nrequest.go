package nrequest

import (
	"context"
	"encoding/json"
	"fmt"
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mexampleheader"
	"the-dev-tools/backend/pkg/model/mexamplequery"
	"the-dev-tools/backend/pkg/model/mitemapi"
	"the-dev-tools/backend/pkg/model/mitemapiexample"
	"the-dev-tools/nodes/pkg/httpclient"
)

const NodeOutputKey = "request"

type NodeRequest struct {
	FlownNodeID idwrap.IDWrap
	Next        *idwrap.IDWrap
	Api         mitemapi.ItemApi
	Example     mitemapiexample.ItemApiExample
	Queries     []mexamplequery.Query
	Headers     []mexampleheader.Header
	Body        []byte
	HttpClient  httpclient.HttpClient
}

func New(id idwrap.IDWrap, next *idwrap.IDWrap, api mitemapi.ItemApi, example mitemapiexample.ItemApiExample,
	Queries []mexamplequery.Query, Headers []mexampleheader.Header, body []byte, Httpclient httpclient.HttpClient,
) *NodeRequest {
	return &NodeRequest{
		FlownNodeID: id,
		Next:        next,
		Api:         api,
		Example:     example,
		HttpClient:  Httpclient,
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
		Method: nr.Api.Method,
		URL:    nr.Api.Url,
		Body:   nr.Body,
	}
	result := node.FlowNodeResult{
		NextNodeID: nr.Next,
		Err:        nil,
	}

	resp, err := httpclient.SendRequestAndConvert(cl, httpReq, nr.Example.ID)
	if err != nil {
		result.Err = err
		return result
	}

	respMap := map[string]interface{}{}
	// TODO: change map conversion non json
	marshaledResp, err := json.Marshal(resp)
	if err != nil {
		result.Err = err
		return result
	}
	err = json.Unmarshal(marshaledResp, &respMap)
	req.VarMap[NodeOutputKey] = respMap

	return result
}

func (nr *NodeRequest) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	cl := nr.HttpClient
	httpReq := httpclient.Request{
		Method: nr.Api.Method,
		URL:    nr.Api.Url,
		Body:   nr.Body,
	}
	result := node.FlowNodeResult{
		NextNodeID: nr.Next,
		Err:        nil,
	}

	resp, err := httpclient.SendRequestAndConvert(cl, httpReq, nr.Example.ID)
	if err != nil {
		result.Err = err
		resultChan <- result
	}
	fmt.Println(resp.Body)

	respMap := map[string]interface{}{}
	// TODO: change map conversion non json
	marshaledResp, err := json.Marshal(resp)
	if err != nil {
		result.Err = err
		resultChan <- result
	}
	err = json.Unmarshal(marshaledResp, &respMap)
	req.VarMap[NodeOutputKey] = respMap

	// TODO: add some functionality here
	resultChan <- result
}
