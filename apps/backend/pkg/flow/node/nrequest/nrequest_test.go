package nrequest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"the-dev-tools/backend/pkg/flow/edge"
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/flow/node/nrequest"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mexampleheader"
	"the-dev-tools/backend/pkg/model/mexamplequery"
	"the-dev-tools/backend/pkg/model/mitemapi"
	"the-dev-tools/backend/pkg/model/mitemapiexample"
	"the-dev-tools/backend/pkg/testutil"
	"the-dev-tools/nodes/pkg/httpclient"
	"the-dev-tools/nodes/pkg/httpclient/httpmockclient"
)

func TestNodeRequest_Run(t *testing.T) {
	id := idwrap.NewNow()
	next := idwrap.NewNow()

	api := mitemapi.ItemApi{
		Method: "GET",
		Url:    "http://example.com",
	}

	example := mitemapiexample.ItemApiExample{
		ID:   idwrap.NewNow(),
		Name: "example",
	}

	queries := []mexamplequery.Query{}
	headers := []mexampleheader.Header{}

	t.Run("RunSync", func(t *testing.T) {
		expectedBody := []byte("Hello, World!")
		buf := bytes.NewBuffer(expectedBody)
		readCloser := io.NopCloser(buf)

		mockResp := &http.Response{
			StatusCode: 200,
			Body:       readCloser,
		}
		mockHttpClient := httpmockclient.NewMockHttpClient(mockResp)
		requestBody := []byte("Request body")

		requestNode := nrequest.New(id, api, example, queries, headers, requestBody, mockHttpClient)

		edge1 := edge.NewEdge(idwrap.NewNow(), id, next, edge.HandleUnspecified)
		edges := []edge.Edge{edge1}
		edgesMap := edge.NewEdgesMap(edges)

		req := &node.FlowNodeRequest{
			VarMap:        map[string]interface{}{},
			EdgeSourceMap: edgesMap,
		}
		ctx := context.TODO()
		resault := requestNode.RunSync(ctx, req)
		testutil.Assert(t, next, *resault.NextNodeID)
		testutil.Assert(t, nil, resault.Err)
		testutil.Assert(t, id, requestNode.GetID())
		testutil.AssertNot(t, req, nil)
		if req.VarMap == nil {
			t.Errorf("Expected req.VarMap to be not nil, but got %v", req.VarMap)
		}
		testutil.AssertNot(t, req.VarMap[nrequest.NodeOutputKey], nil)
		var httpResp httpclient.Response
		RawOutput, ok := req.VarMap[nrequest.NodeOutputKey]
		testutil.Assert(t, true, ok)

		CastedOutput := RawOutput.(map[string]interface{})
		jsonOutput, err := json.Marshal(CastedOutput)
		testutil.Assert(t, nil, err)
		err = json.Unmarshal(jsonOutput, &httpResp)

		testutil.Assert(t, nil, err)
		testutil.Assert(t, 200, httpResp.StatusCode)
		if !bytes.Equal(httpResp.Body, expectedBody) {
			t.Errorf("Expected castedOutput.Body to be not nil, but got %v", httpResp.Body)
		}
	})

	t.Run("RunAsync", func(t *testing.T) {
		expectedBody := []byte("Hello, World!")
		buf := bytes.NewBuffer(expectedBody)
		readCloser := io.NopCloser(buf)

		mockResp := &http.Response{
			StatusCode: 200,
			Body:       readCloser,
		}
		mockHttpClient := httpmockclient.NewMockHttpClient(mockResp)
		requestBody := []byte("Request body")

		requestNode := nrequest.New(id, api, example, queries, headers, requestBody, mockHttpClient)
		edge1 := edge.NewEdge(idwrap.NewNow(), id, next, edge.HandleUnspecified)
		edges := []edge.Edge{edge1}
		edgesMap := edge.NewEdgesMap(edges)

		req := &node.FlowNodeRequest{
			VarMap:        map[string]interface{}{},
			EdgeSourceMap: edgesMap,
		}
		ctx := context.TODO()
		resChan := make(chan node.FlowNodeResult, 1)
		go requestNode.RunAsync(ctx, req, resChan)
		resault := <-resChan
		testutil.Assert(t, next, *resault.NextNodeID)
		testutil.Assert(t, nil, resault.Err)
		testutil.Assert(t, id, requestNode.GetID())
		testutil.AssertNot(t, nil, req)
		if req.VarMap == nil {
			t.Errorf("Expected req.VarMap to be not nil, but got %v", req.VarMap)
		}
		testutil.AssertNot(t, req.VarMap[nrequest.NodeOutputKey], nil)
		var httpResp httpclient.Response
		RawOutput, ok := req.VarMap[nrequest.NodeOutputKey]
		testutil.Assert(t, true, ok)
		CastedOutput := RawOutput.(map[string]interface{})
		jsonOutput, err := json.Marshal(CastedOutput)
		testutil.Assert(t, nil, err)
		err = json.Unmarshal(jsonOutput, &httpResp)
		testutil.Assert(t, nil, err)
		testutil.Assert(t, 200, httpResp.StatusCode)
		if !bytes.Equal(httpResp.Body, expectedBody) {
			t.Errorf("Expected castedOutput.Body to be not nil, but got %v", httpResp.Body)
		}
	})
}

func TestNodeRequest_SetID(t *testing.T) {
	id := idwrap.NewNow()
	api := mitemapi.ItemApi{
		Method: "GET",
		Url:    "http://example.com",
	}
	example := mitemapiexample.ItemApiExample{
		ID: idwrap.NewNow(),
	}
	queries := []mexamplequery.Query{}
	headers := []mexampleheader.Header{}
	mockResp := &http.Response{
		StatusCode: 200,
		Body:       nil,
	}
	mockHttpClient := httpmockclient.NewMockHttpClient(mockResp)
	requestBody := []byte("Request body")
	requestNode := nrequest.New(id, api, example, queries, headers, requestBody, mockHttpClient)
	newID := idwrap.NewNow()
	requestNode.SetID(newID)
	if requestNode.GetID() != newID {
		t.Errorf("Expected ID to be %v, but got %v", newID, requestNode.GetID())
	}
}
