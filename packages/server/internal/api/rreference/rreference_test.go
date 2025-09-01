package rreference_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())
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
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())
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
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())
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
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())
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

func TestReferenceCompletion_ForLoopSelfReference(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	// Initialize all services
	us := suser.New(queries)
	ws := sworkspace.New(queries)
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())
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
	forNodeID := idwrap.NewNow()

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

	// Create FOR node
	err = fns.CreateNode(ctx, mnnode.MNode{
		ID:       forNodeID,
		FlowID:   flowID,
		Name:     "for_loop_1",
		NodeKind: mnnode.NODE_KIND_FOR,
	})
	require.NoError(t, err)

	// Test autocomplete for FOR node self-reference
	req := connect.NewRequest(&referencev1.ReferenceCompletionRequest{
		NodeId: forNodeID.Bytes(),
		Start:  "for_loop_1.",
	})

	response, err := referenceRPC.ReferenceCompletion(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, response)

	// Should find index completion
	found := false
	for _, item := range response.Msg.Items {
		if item.EndToken == "for_loop_1.index" {
			found = true
			assert.Equal(t, referencev1.ReferenceKind_REFERENCE_KIND_VALUE, item.Kind)
			break
		}
	}
	assert.True(t, found, "Should find 'for_loop_1.index' completion for FOR node self-reference")

	t.Logf("Found %d completions for FOR loop self-reference", len(response.Msg.Items))
	for _, item := range response.Msg.Items {
		t.Logf("  - %s (kind: %s)", item.EndToken, item.Kind.String())
	}
}

func TestReferenceCompletion_ForEachLoopSelfReference(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	// Initialize all services
	us := suser.New(queries)
	ws := sworkspace.New(queries)
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())
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
	forEachNodeID := idwrap.NewNow()

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

	// Create FOREACH node
	err = fns.CreateNode(ctx, mnnode.MNode{
		ID:       forEachNodeID,
		FlowID:   flowID,
		Name:     "foreach_items",
		NodeKind: mnnode.NODE_KIND_FOR_EACH,
	})
	require.NoError(t, err)

	// Test autocomplete for FOREACH node self-reference
	req := connect.NewRequest(&referencev1.ReferenceCompletionRequest{
		NodeId: forEachNodeID.Bytes(),
		Start:  "foreach_items.",
	})

	response, err := referenceRPC.ReferenceCompletion(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, response)

	// Should find item and key completions
	foundItem := false
	foundKey := false
	for _, item := range response.Msg.Items {
		if item.EndToken == "foreach_items.item" {
			foundItem = true
			assert.Equal(t, referencev1.ReferenceKind_REFERENCE_KIND_VALUE, item.Kind)
		}
		if item.EndToken == "foreach_items.key" {
			foundKey = true
			assert.Equal(t, referencev1.ReferenceKind_REFERENCE_KIND_VALUE, item.Kind)
		}
	}
	assert.True(t, foundItem, "Should find 'foreach_items.item' completion for FOREACH node self-reference")
	assert.True(t, foundKey, "Should find 'foreach_items.key' completion for FOREACH node self-reference")

	t.Logf("Found %d completions for FOREACH loop self-reference", len(response.Msg.Items))
	for _, item := range response.Msg.Items {
		t.Logf("  - %s (kind: %s)", item.EndToken, item.Kind.String())
	}
}

func TestReferenceCompletion_OnlyBeforeNodes(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	// Initialize all services
	us := suser.New(queries)
	ws := sworkspace.New(queries)
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())
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
	beforeNodeID := idwrap.NewNow()
	currentNodeID := idwrap.NewNow()
	afterNodeID := idwrap.NewNow()

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

	// Create nodes in sequence: beforeNode -> currentNode -> afterNode
	err = fns.CreateNode(ctx, mnnode.MNode{
		ID:       beforeNodeID,
		FlowID:   flowID,
		Name:     "before_node",
		NodeKind: mnnode.NODE_KIND_REQUEST,
	})
	require.NoError(t, err)

	err = fns.CreateNode(ctx, mnnode.MNode{
		ID:       currentNodeID,
		FlowID:   flowID,
		Name:     "current_node",
		NodeKind: mnnode.NODE_KIND_JS,
	})
	require.NoError(t, err)

	err = fns.CreateNode(ctx, mnnode.MNode{
		ID:       afterNodeID,
		FlowID:   flowID,
		Name:     "after_node",
		NodeKind: mnnode.NODE_KIND_REQUEST,
	})
	require.NoError(t, err)

	// Create edges: beforeNode -> currentNode -> afterNode
	err = edgeService.CreateEdge(ctx, edge.Edge{
		ID:            idwrap.NewNow(),
		FlowID:        flowID,
		SourceID:      beforeNodeID,
		TargetID:      currentNodeID,
		SourceHandler: edge.HandleUnspecified,
		Kind:          edge.EdgeKindUnspecified,
	})
	require.NoError(t, err)

	err = edgeService.CreateEdge(ctx, edge.Edge{
		ID:            idwrap.NewNow(),
		FlowID:        flowID,
		SourceID:      currentNodeID,
		TargetID:      afterNodeID,
		SourceHandler: edge.HandleUnspecified,
		Kind:          edge.EdgeKindUnspecified,
	})
	require.NoError(t, err)

	// Test autocomplete from current_node context
	req := connect.NewRequest(&referencev1.ReferenceCompletionRequest{
		NodeId: currentNodeID.Bytes(),
		Start:  "",
	})

	response, err := referenceRPC.ReferenceCompletion(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, response)

	// Should find before_node but NOT after_node
	foundBeforeNode := false
	foundAfterNode := false
	for _, item := range response.Msg.Items {
		if strings.HasPrefix(item.EndToken, "before_node") {
			foundBeforeNode = true
		}
		if strings.HasPrefix(item.EndToken, "after_node") {
			foundAfterNode = true
		}
	}

	assert.True(t, foundBeforeNode, "Should find 'before_node' in autocomplete as it comes before current node")
	assert.False(t, foundAfterNode, "Should NOT find 'after_node' in autocomplete as it comes after current node")

	t.Logf("Found %d completions for context-aware node filtering", len(response.Msg.Items))
	for _, item := range response.Msg.Items {
		t.Logf("  - %s (kind: %s)", item.EndToken, item.Kind.String())
	}
}

// TestReferenceCompletion_RequestNodeSelfReference tests that REQUEST nodes can reference their own response without prefix
func TestReferenceCompletion_RequestNodeSelfReference(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	// Initialize all services
	us := suser.New(queries)
	ws := sworkspace.New(queries)
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())
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

	// Create workspace using base services
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
		Name:     "GetAPIData",
		NodeKind: mnnode.NODE_KIND_REQUEST,
	})
	require.NoError(t, err)

	// Create node execution with data
	executionData := map[string]interface{}{
		"GetAPIData": map[string]interface{}{
			"response": map[string]interface{}{
				"status": 200,
				"body": map[string]interface{}{
					"data": map[string]interface{}{
						"id":   "123",
						"name": "Test Item",
						"tags": []string{"api", "test"},
					},
				},
				"headers": map[string]string{
					"content-type": "application/json",
				},
			},
			"request": map[string]interface{}{
				"body": "{\"query\":\"test\"}",
			},
		},
	}

	executionDataBytes, err := json.Marshal(executionData)
	require.NoError(t, err)

	// Create node execution record
	err = nodeExecutionService.CreateNodeExecution(ctx, mnodeexecution.NodeExecution{
		ID:                     idwrap.NewNow(),
		NodeID:                 requestNodeID,
		Name:                   "Main Execution",
		State:                  2, // Success
		OutputData:             executionDataBytes,
		OutputDataCompressType: compress.CompressTypeNone,
	})
	require.NoError(t, err)

	// Test completion for self-reference (should work without node name prefix)
	req := connect.NewRequest(&referencev1.ReferenceCompletionRequest{
		NodeId: requestNodeID.Bytes(),
		Start:  "response",
	})

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := referenceRPC.ReferenceCompletion(authedCtx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Should find completions for response
	require.NotEmpty(t, resp.Msg.Items, "Should have completions for 'response'")

	// Check that we found the response object
	foundResponse := false
	for _, item := range resp.Msg.Items {
		if item.EndToken == "response" {
			foundResponse = true
		}
	}

	assert.True(t, foundResponse, "Should find 'response' in completions")

	t.Logf("Found %d completions for REQUEST node self-reference 'response'", len(resp.Msg.Items))
	for _, item := range resp.Msg.Items {
		t.Logf("  - %s (kind: %s)", item.EndToken, item.Kind.String())
	}

	// Now test deeper path
	req2 := connect.NewRequest(&referencev1.ReferenceCompletionRequest{
		NodeId: requestNodeID.Bytes(),
		Start:  "response.body.data.",
	})

	resp2, err := referenceRPC.ReferenceCompletion(authedCtx, req2)
	require.NoError(t, err)
	require.NotNil(t, resp2)

	t.Logf("Found %d completions for 'response.body.data.'", len(resp2.Msg.Items))
	for _, item := range resp2.Msg.Items {
		t.Logf("  - %s (kind: %s)", item.EndToken, item.Kind.String())
	}

	// Test value lookup for self-reference
	valueReq := connect.NewRequest(&referencev1.ReferenceValueRequest{
		NodeId: requestNodeID.Bytes(),
		Path:   "response.status",
	})

	valueResp, err := referenceRPC.ReferenceValue(authedCtx, valueReq)
	require.NoError(t, err)
	require.NotNil(t, valueResp)
	assert.Equal(t, "200", valueResp.Msg.Value, "Should get value '200' for 'response.status'")
}

// TestReferenceValue_WithForEachIterationExecutions tests that the reference system handles
// FOR_EACH nodes with multiple iteration executions correctly
func TestReferenceValue_WithForEachIterationExecutions(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	// Initialize all services
	us := suser.New(queries)
	ws := sworkspace.New(queries)
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())
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
	forEachNodeID := idwrap.NewNow()
	nextNodeID := idwrap.NewNow()

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

	// Create FOR_EACH node
	err = fns.CreateNode(ctx, mnnode.MNode{
		ID:       forEachNodeID,
		FlowID:   flowID,
		Name:     "ForEachNode",
		NodeKind: mnnode.NODE_KIND_FOR_EACH,
	})
	require.NoError(t, err)

	// Create next node
	err = fns.CreateNode(ctx, mnnode.MNode{
		ID:       nextNodeID,
		FlowID:   flowID,
		Name:     "NextNode",
		NodeKind: mnnode.NODE_KIND_JS,
	})
	require.NoError(t, err)

	// Create edge from forEach to next node
	err = edgeService.CreateEdge(ctx, edge.Edge{
		ID:            idwrap.NewNow(),
		FlowID:        flowID,
		SourceID:      forEachNodeID,
		TargetID:      nextNodeID,
		SourceHandler: edge.HandleUnspecified,
		Kind:          edge.EdgeKindUnspecified,
	})
	require.NoError(t, err)

	// Create multiple executions for the FOR_EACH node:
	// 1. Iteration executions (should be ignored)
	// 2. Main execution with the actual written variables (should be used)

	// Create iteration executions first
	for i := 0; i < 3; i++ {
		iterationData := map[string]interface{}{
			"iteration": i,
			"data":      fmt.Sprintf("iteration_%d_data", i),
		}
		iterationDataBytes, err := json.Marshal(iterationData)
		require.NoError(t, err)

		err = nodeExecutionService.CreateNodeExecution(ctx, mnodeexecution.NodeExecution{
			ID:                     idwrap.NewNow(),
			NodeID:                 forEachNodeID,
			Name:                   fmt.Sprintf("Iteration %d", i),
			State:                  2, // Success
			OutputData:             iterationDataBytes,
			OutputDataCompressType: compress.CompressTypeNone,
		})
		require.NoError(t, err)
	}

	// Create Error Summary execution
	errorSummaryData := map[string]interface{}{
		"errors": []string{"error1", "error2"},
	}
	errorSummaryDataBytes, err := json.Marshal(errorSummaryData)
	require.NoError(t, err)

	err = nodeExecutionService.CreateNodeExecution(ctx, mnodeexecution.NodeExecution{
		ID:                     idwrap.NewNow(),
		NodeID:                 forEachNodeID,
		Name:                   "Error Summary",
		State:                  2, // Success
		OutputData:             errorSummaryDataBytes,
		OutputDataCompressType: compress.CompressTypeNone,
	})
	require.NoError(t, err)

	// Create the main execution with the actual written variables
	// This is what should be returned by the reference system
	mainExecutionData := map[string]interface{}{
		"ForEachNode": map[string]interface{}{
			"processedItems": []string{"item1", "item2", "item3"},
			"totalCount":     3,
			"status":         "completed",
		},
	}
	mainExecutionDataBytes, err := json.Marshal(mainExecutionData)
	require.NoError(t, err)

	err = nodeExecutionService.CreateNodeExecution(ctx, mnodeexecution.NodeExecution{
		ID:                     idwrap.NewNow(),
		NodeID:                 forEachNodeID,
		Name:                   "Main Execution", // Not an iteration name
		State:                  2,                // Success
		OutputData:             mainExecutionDataBytes,
		OutputDataCompressType: compress.CompressTypeNone,
	})
	require.NoError(t, err)

	// Test ReferenceValue - should get the main execution data, not iterations
	req := connect.NewRequest(&referencev1.ReferenceValueRequest{
		NodeId: nextNodeID.Bytes(),
		Path:   "ForEachNode.totalCount",
	})

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := referenceRPC.ReferenceValue(authedCtx, req)
	
	// Log all executions to debug
	allExecutions, _ := nodeExecutionService.GetNodeExecutionsByNodeID(ctx, forEachNodeID)
	t.Logf("Total executions for node: %d", len(allExecutions))
	for i, exec := range allExecutions {
		t.Logf("Execution %d: Name='%s', ID=%s", i, exec.Name, exec.ID)
	}

	require.NoError(t, err)
	require.NotNil(t, resp)
	
	// Since we're now taking the first execution (which will be the last created = Main Execution)
	// This should work correctly
	assert.Equal(t, "3", resp.Msg.Value, "Should get value '3' for 'ForEachNode.totalCount' from main execution")
}

// TestReferenceCompletion_WithForEachIterationExecutions tests completion with iteration executions
func TestReferenceCompletion_WithForEachIterationExecutions(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	// Initialize all services
	us := suser.New(queries)
	ws := sworkspace.New(queries)
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())
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
	forEachNodeID := idwrap.NewNow()
	nextNodeID := idwrap.NewNow()

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

	// Create FOR_EACH node
	err = fns.CreateNode(ctx, mnnode.MNode{
		ID:       forEachNodeID,
		FlowID:   flowID,
		Name:     "LoopItems",
		NodeKind: mnnode.NODE_KIND_FOR_EACH,
	})
	require.NoError(t, err)

	// Create next node
	err = fns.CreateNode(ctx, mnnode.MNode{
		ID:       nextNodeID,
		FlowID:   flowID,
		Name:     "ProcessResults",
		NodeKind: mnnode.NODE_KIND_JS,
	})
	require.NoError(t, err)

	// Create edge
	err = edgeService.CreateEdge(ctx, edge.Edge{
		ID:            idwrap.NewNow(),
		FlowID:        flowID,
		SourceID:      forEachNodeID,
		TargetID:      nextNodeID,
		SourceHandler: edge.HandleUnspecified,
		Kind:          edge.EdgeKindUnspecified,
	})
	require.NoError(t, err)

	// Add iteration executions
	for i := 0; i < 2; i++ {
		iterData, _ := json.Marshal(map[string]interface{}{"iter": i})
		err = nodeExecutionService.CreateNodeExecution(ctx, mnodeexecution.NodeExecution{
			ID:                     idwrap.NewNow(),
			NodeID:                 forEachNodeID,
			Name:                   fmt.Sprintf("Iteration %d", i),
			State:                  2,
			OutputData:             iterData,
			OutputDataCompressType: compress.CompressTypeNone,
		})
		require.NoError(t, err)
	}

	// Add main execution with proper structure
	mainData := map[string]interface{}{
		"LoopItems": map[string]interface{}{
			"results": []map[string]interface{}{
				{"id": 1, "name": "Item1"},
				{"id": 2, "name": "Item2"},
			},
			"summary": "Processed 2 items",
		},
	}
	mainDataBytes, err := json.Marshal(mainData)
	require.NoError(t, err)

	err = nodeExecutionService.CreateNodeExecution(ctx, mnodeexecution.NodeExecution{
		ID:                     idwrap.NewNow(),
		NodeID:                 forEachNodeID,
		Name:                   "FinalExecution",
		State:                  2,
		OutputData:             mainDataBytes,
		OutputDataCompressType: compress.CompressTypeNone,
	})
	require.NoError(t, err)

	// Test completion
	req := connect.NewRequest(&referencev1.ReferenceCompletionRequest{
		NodeId: nextNodeID.Bytes(),
		Start:  "LoopItems.results",
	})

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := referenceRPC.ReferenceCompletion(authedCtx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Should find the completion
	found := false
	for _, item := range resp.Msg.Items {
		if item.EndToken == "LoopItems.results" {
			found = true
			break
		}
	}

	t.Logf("Found %d completions for 'LoopItems.results'", len(resp.Msg.Items))
	for _, item := range resp.Msg.Items {
		t.Logf("  - %s (kind: %s)", item.EndToken, item.Kind.String())
	}

	assert.True(t, found, "Should find 'LoopItems.results' in completions from main execution")
}

// TestReferenceValue_ExecutionOrderMatters tests that we always get the latest execution
// regardless of whether it's an iteration or main execution
func TestReferenceValue_ExecutionOrderMatters(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	// Initialize all services
	us := suser.New(queries)
	ws := sworkspace.New(queries)
	es := senv.New(queries, base.Logger())
	vs := svar.New(queries, base.Logger())
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
	jsNodeID := idwrap.NewNow()
	nextNodeID := idwrap.NewNow()

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

	// Create JS node (not a FOR_EACH or FOR node, so no iteration executions expected)
	err = fns.CreateNode(ctx, mnnode.MNode{
		ID:       jsNodeID,
		FlowID:   flowID,
		Name:     "JSNode",
		NodeKind: mnnode.NODE_KIND_JS,
	})
	require.NoError(t, err)

	// Create next node
	err = fns.CreateNode(ctx, mnnode.MNode{
		ID:       nextNodeID,
		FlowID:   flowID,
		Name:     "NextNode",
		NodeKind: mnnode.NODE_KIND_JS,
	})
	require.NoError(t, err)

	// Create edge
	err = edgeService.CreateEdge(ctx, edge.Edge{
		ID:            idwrap.NewNow(),
		FlowID:        flowID,
		SourceID:      jsNodeID,
		TargetID:      nextNodeID,
		SourceHandler: edge.HandleUnspecified,
		Kind:          edge.EdgeKindUnspecified,
	})
	require.NoError(t, err)

	// Test scenario 1: Only one execution (should use it)
	executionData1 := map[string]interface{}{
		"JSNode": map[string]interface{}{
			"result": "first",
		},
	}
	data1, _ := json.Marshal(executionData1)
	err = nodeExecutionService.CreateNodeExecution(ctx, mnodeexecution.NodeExecution{
		ID:                     idwrap.NewNow(),
		NodeID:                 jsNodeID,
		Name:                   "First Run",
		State:                  2,
		OutputData:             data1,
		OutputDataCompressType: compress.CompressTypeNone,
	})
	require.NoError(t, err)

	req := connect.NewRequest(&referencev1.ReferenceValueRequest{
		NodeId: nextNodeID.Bytes(),
		Path:   "JSNode.result",
	})

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := referenceRPC.ReferenceValue(authedCtx, req)
	require.NoError(t, err)
	assert.Equal(t, "first", resp.Msg.Value, "Should get first execution when it's the only one")

	// Test scenario 2: Add a second execution (should use the latest)
	executionData2 := map[string]interface{}{
		"JSNode": map[string]interface{}{
			"result": "second",
		},
	}
	data2, _ := json.Marshal(executionData2)
	err = nodeExecutionService.CreateNodeExecution(ctx, mnodeexecution.NodeExecution{
		ID:                     idwrap.NewNow(),
		NodeID:                 jsNodeID,
		Name:                   "Second Run",
		State:                  2,
		OutputData:             data2,
		OutputDataCompressType: compress.CompressTypeNone,
	})
	require.NoError(t, err)

	resp, err = referenceRPC.ReferenceValue(authedCtx, req)
	require.NoError(t, err)
	assert.Equal(t, "second", resp.Msg.Value, "Should get latest execution")

	// Test scenario 3: Add a third execution with different structure
	executionData3 := map[string]interface{}{
		"JSNode": map[string]interface{}{
			"result": "third",
			"extra":  "data",
		},
	}
	data3, _ := json.Marshal(executionData3)
	err = nodeExecutionService.CreateNodeExecution(ctx, mnodeexecution.NodeExecution{
		ID:                     idwrap.NewNow(),
		NodeID:                 jsNodeID,
		Name:                   "Third Run",
		State:                  2,
		OutputData:             data3,
		OutputDataCompressType: compress.CompressTypeNone,
	})
	require.NoError(t, err)

	resp, err = referenceRPC.ReferenceValue(authedCtx, req)
	require.NoError(t, err)
	assert.Equal(t, "third", resp.Msg.Value, "Should always get the most recent execution")

	// Also verify we can access the new field
	req2 := connect.NewRequest(&referencev1.ReferenceValueRequest{
		NodeId: nextNodeID.Bytes(),
		Path:   "JSNode.extra",
	})
	resp2, err := referenceRPC.ReferenceValue(authedCtx, req2)
	require.NoError(t, err)
	assert.Equal(t, "data", resp2.Msg.Value, "Should access new fields from latest execution")
}
