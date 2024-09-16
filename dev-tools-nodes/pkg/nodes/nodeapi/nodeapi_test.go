package nodeapi_test

import (
	"bytes"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mexampleheader"
	"dev-tools-backend/pkg/model/mexamplequery"
	"dev-tools-nodes/pkg/httpclient/httpmockclient"
	"dev-tools-nodes/pkg/model/medge"
	"dev-tools-nodes/pkg/model/mnode"
	"dev-tools-nodes/pkg/model/mnodedata"
	"dev-tools-nodes/pkg/model/mnodemaster"
	"dev-tools-nodes/pkg/nodes/nodeapi"
	"dev-tools-nodes/pkg/resolver"
	"io"
	"net/http"
	"testing"
)

func TestSendRestApiRequest(t *testing.T) {
	apiCallData := &mnodedata.NodeApiRestData{
		Url:    "http://localhost:8080",
		Method: "GET",
		Body:   []byte("SomeBody"),
		Headers: []mexampleheader.Header{
			{ID: idwrap.NewNow(), HeaderKey: "Content-Type", Value: "application/json"},
		},
		Query: []mexamplequery.Query{
			{ID: idwrap.NewNow(), QueryKey: "key", Value: "value"},
		},
	}

	node := &mnode.Node{
		ID:   "test",
		Type: resolver.ApiCallRest,
		Data: apiCallData,
	}

	bodyReader := bytes.NewReader(apiCallData.Body)
	bodyReaderCloser := io.NopCloser(bodyReader)

	mockResponse := http.Response{
		Status:     "Hello World",
		StatusCode: 200,
		Body:       bodyReaderCloser,
		Header:     make(http.Header),
	}

	for _, v := range apiCallData.Headers {
		mockResponse.Header.Add(v.HeaderKey, v.Value)
	}

	mockHttpClient := &httpmockclient.MockHttpClient{
		ReturnResponse: &mockResponse,
	}

	nm := &mnodemaster.NodeMaster{
		CurrentNode: node,
		HttpClient:  mockHttpClient,
		Vars:        make(map[string]interface{}),
	}
	err := nodeapi.SendRestApiRequest(nm)
	if err != nil {
		t.Errorf("Error: %v", err)
	}

	if nm.Vars[nodeapi.VarResponseKey] == nil {
		t.Errorf("Expected response to be set in vars")
	}

	if nm.Vars[nodeapi.VarResponseKey] != &mockResponse {
		t.Errorf("Expected response to be set in vars")
	}

	if nm.NextNodeID != "" {
		t.Errorf("Expected NextNodeID to be empty")
	}
}

func TestSendRestApiRequestNextNode(t *testing.T) {
	apiCallData := &mnodedata.NodeApiRestData{
		Url:    "http://localhost:8080",
		Method: "GET",
		Body:   []byte("SomeBody"),
		Headers: []mexampleheader.Header{
			{ID: idwrap.NewNow(), HeaderKey: "Content-Type", Value: "application/json"},
		},
		Query: []mexamplequery.Query{
			{ID: idwrap.NewNow(), QueryKey: "key", Value: "value"},
		},
	}
	nextNodeID := "nextNode"
	node := &mnode.Node{
		ID:   "test",
		Type: resolver.ApiCallRest,
		Data: apiCallData,
		Edges: medge.Edges{
			OutNodes: map[string]string{medge.DefaultSuccessEdge: nextNodeID},
		},
	}
	bodyReader := bytes.NewReader(apiCallData.Body)
	bodyReaderCloser := io.NopCloser(bodyReader)
	mockResponse := http.Response{
		Status:     "Hello World",
		StatusCode: 200,
		Body:       bodyReaderCloser,
		Header:     make(http.Header),
	}
	for _, v := range apiCallData.Headers {
		mockResponse.Header.Add(v.HeaderKey, v.Value)
	}
	mockHttpClient := &httpmockclient.MockHttpClient{
		ReturnResponse: &mockResponse,
	}
	nm := &mnodemaster.NodeMaster{
		CurrentNode: node,
		HttpClient:  mockHttpClient,
		Vars:        make(map[string]interface{}),
	}
	err := nodeapi.SendRestApiRequest(nm)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if nm.Vars[nodeapi.VarResponseKey] == nil {
		t.Errorf("Expected response to be set in vars")
	}
	if nm.Vars[nodeapi.VarResponseKey] != &mockResponse {
		t.Errorf("Expected response to be set in vars")
	}
	if nm.NextNodeID != nextNodeID {
		t.Errorf("Expected NextNodeID to be %s but find %s", nextNodeID, nm.NextNodeID)
	}
}
