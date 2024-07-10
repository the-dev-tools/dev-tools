package api_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/DevToolsGit/devtools-nodes/pkg/httpclient/httpmockclient"
	"github.com/DevToolsGit/devtools-nodes/pkg/model/medge"
	"github.com/DevToolsGit/devtools-nodes/pkg/model/mnode"
	"github.com/DevToolsGit/devtools-nodes/pkg/model/mnodemaster"
	"github.com/DevToolsGit/devtools-nodes/pkg/nodes/api"
	"github.com/DevToolsGit/devtools-nodes/pkg/resolver"
)

func TestSendRestApiRequest(t *testing.T) {
	apiCallData := &api.RestApiData{
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
	err := api.SendRestApiRequest(nm)
	if err != nil {
		t.Errorf("Error: %v", err)
	}

	if nm.Vars[api.VarResponseKey] == nil {
		t.Errorf("Expected response to be set in vars")
	}

	if nm.Vars[api.VarResponseKey] != &mockResponse {
		t.Errorf("Expected response to be set in vars")
	}

	if nm.NextNodeID != "" {
		t.Errorf("Expected NextNodeID to be empty")
	}
}

func TestSendRestApiRequestNextNode(t *testing.T) {
	apiCallData := &api.RestApiData{
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
	err := api.SendRestApiRequest(nm)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if nm.Vars[api.VarResponseKey] == nil {
		t.Errorf("Expected response to be set in vars")
	}
	if nm.Vars[api.VarResponseKey] != &mockResponse {
		t.Errorf("Expected response to be set in vars")
	}
	if nm.NextNodeID != nextNodeID {
		t.Errorf("Expected NextNodeID to be %s but find %s", nextNodeID, nm.NextNodeID)
	}
}
