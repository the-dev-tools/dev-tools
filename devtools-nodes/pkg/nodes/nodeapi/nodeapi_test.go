package nodeapi_test

import (
	"bytes"
	"devtools-nodes/pkg/httpclient/httpmockclient"
	"devtools-nodes/pkg/model/medge"
	"devtools-nodes/pkg/model/mnode"
	"devtools-nodes/pkg/model/mnodedata"
	"devtools-nodes/pkg/model/mnodemaster"
	"devtools-nodes/pkg/nodes/nodeapi"
	"devtools-nodes/pkg/resolver"
	"io"
	"net/http"
	"testing"
)

func TestSendRestApiRequest(t *testing.T) {
	apiCallData := &mnodedata.NodeApiRestData{
		Url:    "http://localhost:8080",
		Method: "GET",
		Body:   []byte("SomeBody"),
		Headers: map[string]string{
			"Content-Type": "application/json",
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

	for key, value := range apiCallData.Headers {
		mockResponse.Header.Add(key, value)
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
		Headers: map[string]string{
			"Content-Type": "application/json",
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
	for key, value := range apiCallData.Headers {
		mockResponse.Header.Add(key, value)
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
