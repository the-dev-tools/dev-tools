package rreference_test

import (
	"context"
	"encoding/json"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rreference"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node/nrequest"
	"the-dev-tools/server/pkg/http/request"
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnodeexecution"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sexamplerespheader"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodeexecution"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/testutil"
	referencev1 "the-dev-tools/spec/dist/buf/go/reference/v1"
)

func TestReferenceTree_WithRequestNodeExecution(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	// Initialize all services
	us := suser.New(queries)
	ws := sworkspace.New(queries)
	es := senv.New(queries)
	vs := svar.New(queries)
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	fs := sflow.New(queries)
	fns := snode.New(queries)
	frns := snoderequest.New(queries)
	flowVariableService := sflowvariable.New(queries)
	edgeService := sedge.New(queries)
	nodeExecutionService := snodeexecution.New(queries)

	// Create ReferenceServiceRPC
	referenceRPC := rreference.NewNodeServiceRPC(
		db, us, ws, es, vs, ers, erhs, fs, fns, frns,
		flowVariableService, edgeService, nodeExecutionService,
	)

	// Create test data
	workspaceID := idwrap.NewNow()
	userID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	requestNodeID := idwrap.NewNow()

	// Create workspace
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, idwrap.NewNow(), userID, idwrap.NewNow())

	// Create flow
	err := fs.CreateFlow(ctx, mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	})
	require.NoError(t, err)

	// Create REQUEST node
	err = fns.CreateNode(ctx, mnnode.MNode{
		ID:       requestNodeID,
		FlowID:   flowID,
		Name:     "TestRequest",
		NodeKind: mnnode.NODE_KIND_REQUEST,
	})
	require.NoError(t, err)

	// Create node execution data
	nodeOutput := nrequest.NodeRequestOutput{
		Request: request.RequestResponseVar{
			Headers: map[string]string{"Authorization": "Bearer token"},
			Queries: map[string]string{"page": "1", "limit": "10"},
			Body:    `{"test": "data"}`,
		},
		Response: httpclient.ResponseVar{
			StatusCode: 200,
			Body: map[string]interface{}{
				"users": []interface{}{
					map[string]interface{}{
						"id":    1,
						"name":  "John Doe",
						"email": "john@example.com",
					},
					map[string]interface{}{
						"id":    2,
						"name":  "Jane Doe",
						"email": "jane@example.com",
					},
				},
				"total": 2,
			},
			Headers:  map[string]string{"Content-Type": "application/json"},
			Duration: 150,
		},
	}

	// Marshal the output
	outputData, err := json.Marshal(nodeOutput)
	require.NoError(t, err)

	// Create node execution record
	err = nodeExecutionService.CreateNodeExecution(ctx, mnodeexecution.NodeExecution{
		ID:                     idwrap.NewNow(),
		NodeID:                 requestNodeID,
		State:                  2, // Success
		OutputData:             outputData,
		OutputDataCompressType: compress.CompressTypeNone,
	})
	require.NoError(t, err)

	// Create a second node that comes after the request node
	secondNodeID := idwrap.NewNow()
	err = fns.CreateNode(ctx, mnnode.MNode{
		ID:       secondNodeID,
		FlowID:   flowID,
		Name:     "ProcessData",
		NodeKind: mnnode.NODE_KIND_JS,
	})
	require.NoError(t, err)

	// Create edge from request to second node
	err = edgeService.CreateEdge(ctx, edge.Edge{
		ID:            idwrap.NewNow(),
		FlowID:        flowID,
		SourceID:      requestNodeID,
		TargetID:      secondNodeID,
		SourceHandler: edge.HandleUnspecified,
		Kind:          edge.EdgeKindUnspecified,
	})
	require.NoError(t, err)

	// Test ReferenceTree - we query from the second node to see nodes before it
	req := connect.NewRequest(&referencev1.ReferenceTreeRequest{
		NodeId: secondNodeID.Bytes(),
	})

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := referenceRPC.ReferenceTree(authedCtx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Debug: print detailed structure
	t.Logf("Got %d items in response", len(resp.Msg.Items))
	for i, item := range resp.Msg.Items {
		t.Logf("Item %d: Kind=%v", i, item.Kind)
		if item.Key != nil {
			t.Logf("  Key Kind: %v", item.Key.Kind)
			if item.Key.Group != nil {
				t.Logf("  Group: '%s'", *item.Key.Group)
			}
			if item.Key.Key != nil {
				t.Logf("  Key: '%s'", *item.Key.Key)
			}
		}
		// Print nested items
		if item.Map != nil {
			t.Logf("  Has %d map items", len(item.Map))
			for j, mapItem := range item.Map {
				if mapItem.Key != nil && mapItem.Key.Key != nil {
					t.Logf("  Map item %d: Key='%s', Kind=%v", j, *mapItem.Key.Key, mapItem.Kind)
				}
			}
		}
	}

	// Verify response contains request node data
	require.NotEmpty(t, resp.Msg.Items)

	// The response contains a map item with nested request/response
	// Based on the debug output, the structure has the node data directly
	rootItem := resp.Msg.Items[0]
	require.Equal(t, referencev1.ReferenceKind_REFERENCE_KIND_MAP, rootItem.Kind)
	require.NotNil(t, rootItem.Map)

	// The rootItem.Map should contain "request" and "response" entries
	// These come from the execution data we stored
	var hasRequest, hasResponse bool
	for _, item := range rootItem.Map {
		if item.Key != nil && item.Key.Key != nil {
			switch *item.Key.Key {
			case "request":
				hasRequest = true
				assert.Equal(t, referencev1.ReferenceKind_REFERENCE_KIND_MAP, item.Kind)
			case "response":
				hasResponse = true
				assert.Equal(t, referencev1.ReferenceKind_REFERENCE_KIND_MAP, item.Kind)
			}
		}
	}

	assert.True(t, hasRequest, "Should have request object")
	assert.True(t, hasResponse, "Should have response object")
}

func TestReferenceTree_WithoutExecution(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	// Initialize all services
	us := suser.New(queries)
	ws := sworkspace.New(queries)
	es := senv.New(queries)
	vs := svar.New(queries)
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	fs := sflow.New(queries)
	fns := snode.New(queries)
	frns := snoderequest.New(queries)
	flowVariableService := sflowvariable.New(queries)
	edgeService := sedge.New(queries)
	nodeExecutionService := snodeexecution.New(queries)

	// Create ReferenceServiceRPC
	referenceRPC := rreference.NewNodeServiceRPC(
		db, us, ws, es, vs, ers, erhs, fs, fns, frns,
		flowVariableService, edgeService, nodeExecutionService,
	)

	// Create test data
	workspaceID := idwrap.NewNow()
	userID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	requestNodeID := idwrap.NewNow()

	// Create workspace
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, idwrap.NewNow(), userID, idwrap.NewNow())

	// Create flow
	err := fs.CreateFlow(ctx, mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	})
	require.NoError(t, err)

	// Create REQUEST node (without execution data)
	err = fns.CreateNode(ctx, mnnode.MNode{
		ID:       requestNodeID,
		FlowID:   flowID,
		Name:     "TestRequest",
		NodeKind: mnnode.NODE_KIND_REQUEST,
	})
	require.NoError(t, err)

	// Create a second node that comes after the request node
	secondNodeID := idwrap.NewNow()
	err = fns.CreateNode(ctx, mnnode.MNode{
		ID:       secondNodeID,
		FlowID:   flowID,
		Name:     "ProcessData",
		NodeKind: mnnode.NODE_KIND_JS,
	})
	require.NoError(t, err)

	// Create edge from request to second node
	err = edgeService.CreateEdge(ctx, edge.Edge{
		ID:            idwrap.NewNow(),
		FlowID:        flowID,
		SourceID:      requestNodeID,
		TargetID:      secondNodeID,
		SourceHandler: edge.HandleUnspecified,
		Kind:          edge.EdgeKindUnspecified,
	})
	require.NoError(t, err)

	// Test ReferenceTree - we query from the second node to see nodes before it
	req := connect.NewRequest(&referencev1.ReferenceTreeRequest{
		NodeId: secondNodeID.Bytes(),
	})

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := referenceRPC.ReferenceTree(authedCtx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify response contains request node schema
	require.NotEmpty(t, resp.Msg.Items)

	// The response should have the node schema
	rootItem := resp.Msg.Items[0]
	require.Equal(t, referencev1.ReferenceKind_REFERENCE_KIND_MAP, rootItem.Kind)
	require.NotNil(t, rootItem.Map)
	require.NotNil(t, rootItem.Key)
	require.NotNil(t, rootItem.Key.Key)
	require.Equal(t, "TestRequest", *rootItem.Key.Key)

	// Verify the structure contains request and response schema
	var hasRequest, hasResponse bool
	for _, item := range rootItem.Map {
		if item.Key != nil && item.Key.Key != nil {
			switch *item.Key.Key {
			case "request":
				hasRequest = true
				assert.Equal(t, referencev1.ReferenceKind_REFERENCE_KIND_MAP, item.Kind)
			case "response":
				hasResponse = true
				assert.Equal(t, referencev1.ReferenceKind_REFERENCE_KIND_MAP, item.Kind)
			}
		}
	}

	assert.True(t, hasRequest, "Should have request schema")
	assert.True(t, hasResponse, "Should have response schema")
}

func TestReferenceCompletion_WithRequestNodeExecution(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	// Initialize all services
	us := suser.New(queries)
	ws := sworkspace.New(queries)
	es := senv.New(queries)
	vs := svar.New(queries)
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	fs := sflow.New(queries)
	fns := snode.New(queries)
	frns := snoderequest.New(queries)
	flowVariableService := sflowvariable.New(queries)
	edgeService := sedge.New(queries)
	nodeExecutionService := snodeexecution.New(queries)

	// Create ReferenceServiceRPC
	referenceRPC := rreference.NewNodeServiceRPC(
		db, us, ws, es, vs, ers, erhs, fs, fns, frns,
		flowVariableService, edgeService, nodeExecutionService,
	)

	// Create test data
	workspaceID := idwrap.NewNow()
	userID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	requestNodeID := idwrap.NewNow()
	currentNodeID := idwrap.NewNow()

	// Create workspace
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, idwrap.NewNow(), userID, idwrap.NewNow())

	// Create flow
	err := fs.CreateFlow(ctx, mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	})
	require.NoError(t, err)

	// Create REQUEST node
	err = fns.CreateNode(ctx, mnnode.MNode{
		ID:       requestNodeID,
		FlowID:   flowID,
		Name:     "GetUsers",
		NodeKind: mnnode.NODE_KIND_REQUEST,
	})
	require.NoError(t, err)

	// Create current node
	err = fns.CreateNode(ctx, mnnode.MNode{
		ID:       currentNodeID,
		FlowID:   flowID,
		Name:     "ProcessUsers",
		NodeKind: mnnode.NODE_KIND_JS,
	})
	require.NoError(t, err)

	// Create edge from request node to current node
	err = edgeService.CreateEdge(ctx, edge.Edge{
		ID:            idwrap.NewNow(),
		FlowID:        flowID,
		SourceID:      requestNodeID,
		TargetID:      currentNodeID,
		SourceHandler: edge.HandleUnspecified,
		Kind:          edge.EdgeKindUnspecified,
	})
	require.NoError(t, err)

	// Create node execution data
	nodeOutput := nrequest.NodeRequestOutput{
		Request: request.RequestResponseVar{
			Headers: map[string]string{"Authorization": "Bearer token"},
			Queries: map[string]string{"page": "1"},
			Body:    `{"filter": "active"}`,
		},
		Response: httpclient.ResponseVar{
			StatusCode: 200,
			Body: map[string]interface{}{
				"users": []interface{}{
					map[string]interface{}{
						"id":    1,
						"name":  "John Doe",
						"email": "john@example.com",
					},
				},
				"total": 1,
			},
			Headers:  map[string]string{"Content-Type": "application/json"},
			Duration: 100,
		},
	}

	// Marshal the output
	outputData, err := json.Marshal(nodeOutput)
	require.NoError(t, err)

	// Create node execution record
	err = nodeExecutionService.CreateNodeExecution(ctx, mnodeexecution.NodeExecution{
		ID:                     idwrap.NewNow(),
		NodeID:                 requestNodeID,
		State:                  2, // Success
		OutputData:             outputData,
		OutputDataCompressType: compress.CompressTypeNone,
	})
	require.NoError(t, err)

	// Test ReferenceCompletion
	req := connect.NewRequest(&referencev1.ReferenceCompletionRequest{
		NodeId: currentNodeID.Bytes(),
		Start:  "GetUsers.response.body.users",
	})

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := referenceRPC.ReferenceCompletion(authedCtx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify completions
	require.NotEmpty(t, resp.Msg.Items)

	// Debug: print what completions we got
	t.Logf("Got %d completion items for '%s'", len(resp.Msg.Items), req.Msg.Start)
	for i, item := range resp.Msg.Items {
		t.Logf("Item %d: EndToken='%s', Kind=%v", i, item.EndToken, item.Kind)
	}

	// Should have completions for users array
	found := false
	for _, item := range resp.Msg.Items {
		if item.EndToken == "GetUsers.response.body.users" {
			found = true
			// users is an array, so it should be KIND_VALUE (or KIND_MAP if it has nested items)
			// The completion system marks arrays as VALUE
			assert.Equal(t, referencev1.ReferenceKind_REFERENCE_KIND_VALUE, item.Kind)
			break
		}
	}
	assert.True(t, found, "Should find completion for GetUsers.response.body.users")
}

func TestReferenceValue_WithRequestNodeExecution(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	// Initialize all services
	us := suser.New(queries)
	ws := sworkspace.New(queries)
	es := senv.New(queries)
	vs := svar.New(queries)
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	fs := sflow.New(queries)
	fns := snode.New(queries)
	frns := snoderequest.New(queries)
	flowVariableService := sflowvariable.New(queries)
	edgeService := sedge.New(queries)
	nodeExecutionService := snodeexecution.New(queries)

	// Create ReferenceServiceRPC
	referenceRPC := rreference.NewNodeServiceRPC(
		db, us, ws, es, vs, ers, erhs, fs, fns, frns,
		flowVariableService, edgeService, nodeExecutionService,
	)

	// Create test data
	workspaceID := idwrap.NewNow()
	userID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	requestNodeID := idwrap.NewNow()
	currentNodeID := idwrap.NewNow()

	// Create workspace
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, idwrap.NewNow(), userID, idwrap.NewNow())

	// Create flow
	err := fs.CreateFlow(ctx, mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	})
	require.NoError(t, err)

	// Create flow variable
	err = flowVariableService.CreateFlowVariable(ctx, mflowvariable.FlowVariable{
		ID:      idwrap.NewNow(),
		FlowID:  flowID,
		Name:    "apiKey",
		Value:   "test-api-key",
		Enabled: true,
	})
	require.NoError(t, err)

	// Create REQUEST node
	err = fns.CreateNode(ctx, mnnode.MNode{
		ID:       requestNodeID,
		FlowID:   flowID,
		Name:     "GetData",
		NodeKind: mnnode.NODE_KIND_REQUEST,
	})
	require.NoError(t, err)

	// Create current node
	err = fns.CreateNode(ctx, mnnode.MNode{
		ID:       currentNodeID,
		FlowID:   flowID,
		Name:     "ProcessData",
		NodeKind: mnnode.NODE_KIND_JS,
	})
	require.NoError(t, err)

	// Create edge from request node to current node
	err = edgeService.CreateEdge(ctx, edge.Edge{
		ID:            idwrap.NewNow(),
		FlowID:        flowID,
		SourceID:      requestNodeID,
		TargetID:      currentNodeID,
		SourceHandler: edge.HandleUnspecified,
		Kind:          edge.EdgeKindUnspecified,
	})
	require.NoError(t, err)

	// Create node execution data
	nodeOutput := nrequest.NodeRequestOutput{
		Request: request.RequestResponseVar{
			Headers: map[string]string{"X-API-Key": "secret"},
			Queries: map[string]string{"format": "json"},
			Body:    `{"query": "test"}`,
		},
		Response: httpclient.ResponseVar{
			StatusCode: 200,
			Body:       map[string]interface{}{"result": "success", "count": 42},
			Headers:    map[string]string{"Content-Type": "application/json"},
			Duration:   75,
		},
	}

	// Marshal the output
	outputData, err := json.Marshal(nodeOutput)
	require.NoError(t, err)

	// Create node execution record
	err = nodeExecutionService.CreateNodeExecution(ctx, mnodeexecution.NodeExecution{
		ID:                     idwrap.NewNow(),
		NodeID:                 requestNodeID,
		State:                  2, // Success
		OutputData:             outputData,
		OutputDataCompressType: compress.CompressTypeNone,
	})
	require.NoError(t, err)

	// Test ReferenceValue - get response status
	req1 := connect.NewRequest(&referencev1.ReferenceValueRequest{
		NodeId: currentNodeID.Bytes(),
		Path:   "GetData.response.status",
	})

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp1, err := referenceRPC.ReferenceValue(authedCtx, req1)
	require.NoError(t, err)
	require.NotNil(t, resp1)
	assert.Equal(t, "200", resp1.Msg.Value)

	// Test ReferenceValue - get response body count
	req2 := connect.NewRequest(&referencev1.ReferenceValueRequest{
		NodeId: currentNodeID.Bytes(),
		Path:   "GetData.response.body.count",
	})

	resp2, err := referenceRPC.ReferenceValue(authedCtx, req2)
	require.NoError(t, err)
	require.NotNil(t, resp2)
	assert.Equal(t, "42", resp2.Msg.Value)

	// Test ReferenceValue - get flow variable
	req3 := connect.NewRequest(&referencev1.ReferenceValueRequest{
		NodeId: currentNodeID.Bytes(),
		Path:   "apiKey",
	})

	resp3, err := referenceRPC.ReferenceValue(authedCtx, req3)
	require.NoError(t, err)
	require.NotNil(t, resp3)
	assert.Equal(t, "test-api-key", resp3.Msg.Value)
}
