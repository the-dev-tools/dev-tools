package rreference

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/testutil"
	referencev1 "the-dev-tools/spec/dist/buf/go/api/reference/v1"
)

type referenceTestServices struct {
	us   suser.UserService
	ws   sworkspace.WorkspaceService
	es   senv.EnvironmentService
	vs   senv.VariableService
	fs   sflow.FlowService
	fns  sflow.NodeService
	fvs  sflow.FlowVariableService
	esvc sflow.EdgeService
	nes  sflow.NodeExecutionService
	hrs  shttp.HttpResponseService
}

func setupTestService(t *testing.T) (*ReferenceServiceRPC, context.Context, idwrap.IDWrap, idwrap.IDWrap, referenceTestServices) {
	ctx := context.Background()
	baseDB := testutil.CreateBaseDB(ctx, t)
	t.Cleanup(func() { baseDB.Close() })

	queries := baseDB.Queries
	db := baseDB.DB
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Setup Services
	us := suser.New(queries)
	ws := sworkspace.NewWorkspaceService(queries)
	es := senv.NewEnvironmentService(queries, logger)
	vs := senv.NewVariableService(queries, logger)
	fs := sflow.NewFlowService(queries)
	fns := sflow.NewNodeService(queries)
	frns := sflow.NewNodeRequestService(queries)
	fvs := sflow.NewFlowVariableService(queries)
	edgeService := sflow.NewEdgeService(queries)
	nes := sflow.NewNodeExecutionService(queries)

	httpResponseService := shttp.NewHttpResponseService(queries)

	svc := NewReferenceServiceRPC(ReferenceServiceRPCDeps{
		DB: db,
		Readers: ReferenceServiceRPCReaders{
			User:          sworkspace.NewUserReader(db),
			Workspace:     ws.Reader(),
			Env:           es.Reader(),
			Variable:      vs.Reader(),
			Flow:          fs.Reader(),
			Node:          fns.Reader(),
			NodeRequest:   frns.Reader(),
			FlowVariable:  fvs.Reader(),
			FlowEdge:      edgeService.Reader(),
			NodeExecution: nes.Reader(),
			HttpResponse:  httpResponseService.Reader(),
		},
	})

	// Create User and Workspace using BaseTestServices helper
	userID := idwrap.NewNow()
	ctx = mwauth.CreateAuthedContext(ctx, userID)

	baseServices := baseDB.GetBaseServices()
	workspaceID, err := baseServices.CreateTempCollection(ctx, userID, "Test Workspace")
	require.NoError(t, err)

	return svc, ctx, userID, workspaceID, referenceTestServices{
		us:   us,
		ws:   ws,
		es:   es,
		vs:   vs,
		fs:   fs,
		fns:  fns,
		fvs:  fvs,
		esvc: edgeService,
		nes:  nes,
		hrs:  httpResponseService,
	}
}

func TestReferenceCompletion_Workspace(t *testing.T) {
	svc, ctx, _, workspaceID, ts := setupTestService(t)

	// Create Env
	envID := idwrap.NewNow()
	err := ts.es.Create(ctx, menv.Env{
		ID:          envID,
		WorkspaceID: workspaceID,
		Name:        "Test Env",
	})
	require.NoError(t, err)

	// Create Vars
	varID1 := idwrap.NewNow()
	err = ts.vs.Create(ctx, menv.Variable{
		ID:      varID1,
		EnvID:   envID,
		VarKey:  "env_var_1",
		Value:   "value_1",
		Enabled: true,
	})
	require.NoError(t, err)

	varID2 := idwrap.NewNow()
	err = ts.vs.Create(ctx, menv.Variable{
		ID:      varID2,
		EnvID:   envID,
		VarKey:  "env_var_2",
		Value:   "value_2",
		Enabled: false, // Disabled, should not appear
	})
	require.NoError(t, err)

	// Test Completion
	req := connect.NewRequest(&referencev1.ReferenceCompletionRequest{
		WorkspaceId: workspaceID.Bytes(),
		Start:       "env_var",
	})

	resp, err := svc.ReferenceCompletion(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify env_var_1 is present
	found := false
	for _, item := range resp.Msg.Items {
		if item.EndToken == "env_var_1" {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected env_var_1 in completions")
}

func TestReferenceCompletion_FlowNode(t *testing.T) {
	svc, ctx, _, workspaceID, ts := setupTestService(t)

	// Create Flow
	flowID := idwrap.NewNow()
	err := ts.fs.CreateFlow(ctx, mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	})
	require.NoError(t, err)

	// Create Source Node (REQUEST)
	sourceNodeID := idwrap.NewNow()
	err = ts.fns.CreateNode(ctx, mflow.Node{
		ID:       sourceNodeID,
		FlowID:   flowID,
		Name:     "SourceRequest",
		NodeKind: mflow.NODE_KIND_REQUEST,
	})
	require.NoError(t, err)

	// Create Target Node (REQUEST) - this is where we request completion from
	targetNodeID := idwrap.NewNow()
	err = ts.fns.CreateNode(ctx, mflow.Node{
		ID:       targetNodeID,
		FlowID:   flowID,
		Name:     "TargetRequest",
		NodeKind: mflow.NODE_KIND_REQUEST,
	})
	require.NoError(t, err)

	// Create Edge from Source to Target
	edgeID := idwrap.NewNow()
	err = ts.esvc.CreateEdge(ctx, mflow.Edge{
		ID:       edgeID,
		FlowID:   flowID,
		SourceID: sourceNodeID,
		TargetID: targetNodeID,
	})
	require.NoError(t, err)

	// Create Flow Variable
	flowVarID := idwrap.NewNow()
	err = ts.fvs.CreateFlowVariable(ctx, mflow.FlowVariable{
		ID:      flowVarID,
		FlowID:  flowID,
		Name:    "flow_var_1",
		Value:   "val1",
		Enabled: true,
	})
	require.NoError(t, err)

	// Test Completion
	req := connect.NewRequest(&referencev1.ReferenceCompletionRequest{
		FlowNodeId: targetNodeID.Bytes(),
		Start:      "", // Empty start to match everything
	})

	resp, err := svc.ReferenceCompletion(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify completions
	var foundFlowVar, foundSourceNode, foundSelfNode bool
	for _, item := range resp.Msg.Items {
		if item.EndToken == "flow_var_1" {
			foundFlowVar = true
		}
		if item.EndToken == "SourceRequest" {
			foundSourceNode = true
		}
		if item.EndToken == "request" { // Self-reference for TargetRequest
			foundSelfNode = true
		}
	}

	assert.True(t, foundFlowVar, "Expected flow_var_1")
	assert.True(t, foundSourceNode, "Expected SourceRequest")
	assert.True(t, foundSelfNode, "Expected self reference (request/response)")
}

func TestReferenceCompletion_Http(t *testing.T) {
	svc, ctx, _, _, ts := setupTestService(t)

	httpID := idwrap.NewNow()

	// Create Response using svc.httpResponseReader
	respID := idwrap.NewNow()
	err := ts.hrs.Create(ctx, mhttp.HTTPResponse{
		ID:        respID,
		HttpID:    httpID,
		Status:    201,
		Body:      []byte(`{"foo":"bar"}`),
		Time:      time.Now().Unix(),
		CreatedAt: time.Now().Unix(),
	})
	require.NoError(t, err)

	req := connect.NewRequest(&referencev1.ReferenceCompletionRequest{
		HttpId: httpID.Bytes(),
		Start:  "",
	})

	resp, err := svc.ReferenceCompletion(ctx, req)
	require.NoError(t, err)

	// Should see "response" and "request"
	var foundResponse bool
	for _, item := range resp.Msg.Items {
		if item.EndToken == "response" {
			foundResponse = true
			break
		}
	}
	assert.True(t, foundResponse, "Expected response in completions")
}

func TestReferenceValue_Simple(t *testing.T) {
	svc, ctx, _, workspaceID, ts := setupTestService(t)

	// Create Env Var
	envID := idwrap.NewNow()
	err := ts.es.Create(ctx, menv.Env{
		ID:          envID,
		WorkspaceID: workspaceID,
		Name:        "Test Env",
	})
	require.NoError(t, err)

	err = ts.vs.Create(ctx, menv.Variable{
		ID:      idwrap.NewNow(),
		EnvID:   envID,
		VarKey:  "my_var",
		Value:   "my_value",
		Enabled: true,
	})
	require.NoError(t, err)

	// Request Value
	req := connect.NewRequest(&referencev1.ReferenceValueRequest{
		WorkspaceId: workspaceID.Bytes(),
		Path:        "my_var",
	})

	resp, err := svc.ReferenceValue(ctx, req)
	require.NoError(t, err)

	// Value should be "my_value"
	assert.Equal(t, "my_value", resp.Msg.Value)
}

func TestReferenceValue_Nested(t *testing.T) {
	svc, ctx, _, workspaceID, ts := setupTestService(t)

	// Create Flow
	flowID := idwrap.NewNow()
	err := ts.fs.CreateFlow(ctx, mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	})
	require.NoError(t, err)

	// Create Node with Execution Data
	nodeID := idwrap.NewNow()
	err = ts.fns.CreateNode(ctx, mflow.Node{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     "MyNode",
		NodeKind: mflow.NODE_KIND_JS,
	})
	require.NoError(t, err)

	// Create Execution
	execID := idwrap.NewNow()
	outputData := map[string]interface{}{
		"MyNode": map[string]interface{}{
			"foo": "bar",
			"nested": map[string]interface{}{
				"baz": 123,
			},
		},
	}
	outputBytes, _ := json.Marshal(outputData)

	now := time.Now().Unix()
	exec := mflow.NodeExecution{
		ID:                     execID,
		NodeID:                 nodeID,
		Name:                   "Exec 1",
		State:                  mflow.NODE_STATE_SUCCESS,
		CompletedAt:            &now,
		OutputData:             outputBytes,
		OutputDataCompressType: compress.CompressTypeNone,
	}

	// Use CreateNodeExecution method
	err = ts.nes.CreateNodeExecution(ctx, exec)
	require.NoError(t, err)

	// We need another node to reference "MyNode"
	targetNodeID := idwrap.NewNow()
	err = ts.fns.CreateNode(ctx, mflow.Node{
		ID:       targetNodeID,
		FlowID:   flowID,
		Name:     "TargetNode",
		NodeKind: mflow.NODE_KIND_JS,
	})
	require.NoError(t, err)

	// Create Edge
	err = ts.esvc.CreateEdge(ctx, mflow.Edge{
		ID:       idwrap.NewNow(),
		FlowID:   flowID,
		SourceID: nodeID,
		TargetID: targetNodeID,
	})
	require.NoError(t, err)

	// Request Value: MyNode.nested.baz
	req := connect.NewRequest(&referencev1.ReferenceValueRequest{
		FlowNodeId: targetNodeID.Bytes(),
		Path:       "MyNode.nested.baz",
	})

	resp, err := svc.ReferenceValue(ctx, req)
	require.NoError(t, err)

	// Expect string representation of 123
	assert.Equal(t, "123", resp.Msg.Value)
}

func TestReferenceTree_Workspace(t *testing.T) {
	svc, ctx, _, workspaceID, ts := setupTestService(t)

	// Create Env
	envID := idwrap.NewNow()
	err := ts.es.Create(ctx, menv.Env{
		ID:          envID,
		WorkspaceID: workspaceID,
		Name:        "Test Env",
	})
	require.NoError(t, err)

	// Create Vars
	varID1 := idwrap.NewNow()
	err = ts.vs.Create(ctx, menv.Variable{
		ID:      varID1,
		EnvID:   envID,
		VarKey:  "env_var_1",
		Value:   "value_1",
		Enabled: true,
	})
	require.NoError(t, err)

	req := connect.NewRequest(&referencev1.ReferenceTreeRequest{
		WorkspaceId: workspaceID.Bytes(),
	})

	resp, err := svc.ReferenceTree(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Check if we have the "env" group
	var envGroup *referencev1.ReferenceTreeItem
	for _, item := range resp.Msg.Items {
		if item.Key.GetGroup() == "env" {
			envGroup = item
			break
		}
	}
	require.NotNil(t, envGroup)
	assert.Equal(t, referencev1.ReferenceKind_REFERENCE_KIND_MAP, envGroup.Kind)

	// Check if env_var_1 is inside
	found := false
	for _, item := range envGroup.Map {
		if item.Key.GetKey() == "env_var_1" {
			found = true
			assert.Contains(t, item.Variable, "Test Env")
			break
		}
	}
	assert.True(t, found)
}

func TestReferenceTree_Http(t *testing.T) {
	svc, ctx, _, _, ts := setupTestService(t)

	httpID := idwrap.NewNow()

	// Create Response
	respID := idwrap.NewNow()
	err := ts.hrs.Create(ctx, mhttp.HTTPResponse{
		ID:        respID,
		HttpID:    httpID,
		Status:    201,
		Body:      []byte(`{"foo":"bar"}`),
		Time:      time.Now().Unix(),
		CreatedAt: time.Now().Unix(),
	})
	require.NoError(t, err)

	req := connect.NewRequest(&referencev1.ReferenceTreeRequest{
		HttpId: httpID.Bytes(),
	})

	resp, err := svc.ReferenceTree(ctx, req)
	require.NoError(t, err)

	// Should contain response tree
	require.NotEmpty(t, resp.Msg.Items)

	// Look for response item
	var responseItem *referencev1.ReferenceTreeItem
	for _, item := range resp.Msg.Items {
		if item.Key.GetKey() == "response" {
			responseItem = item
			break
		}
	}
	require.NotNil(t, responseItem)

	// Check structure of response
	// response -> body -> foo
	var bodyItem *referencev1.ReferenceTreeItem
	for _, item := range responseItem.Map {
		if item.Key.GetKey() == "body" {
			bodyItem = item
			break
		}
	}
	require.NotNil(t, bodyItem)

	var fooItem *referencev1.ReferenceTreeItem
	for _, item := range bodyItem.Map {
		if item.Key.GetKey() == "foo" {
			fooItem = item
			break
		}
	}
	require.NotNil(t, fooItem)
	require.NotNil(t, fooItem.Value)
	assert.Equal(t, "bar", *fooItem.Value)
}

func TestReferenceTree_FlowNode(t *testing.T) {
	svc, ctx, _, workspaceID, ts := setupTestService(t)

	// Create Flow
	flowID := idwrap.NewNow()
	err := ts.fs.CreateFlow(ctx, mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	})
	require.NoError(t, err)

	// 1. Flow Variable
	err = ts.fvs.CreateFlowVariable(ctx, mflow.FlowVariable{
		ID:      idwrap.NewNow(),
		FlowID:  flowID,
		Name:    "flow_var",
		Value:   "flow_val",
		Enabled: true,
	})
	require.NoError(t, err)

	// 2. Previous Node with Execution Data
	node1ID := idwrap.NewNow()
	err = ts.fns.CreateNode(ctx, mflow.Node{
		ID:       node1ID,
		FlowID:   flowID,
		Name:     "NodeWithExec",
		NodeKind: mflow.NODE_KIND_JS,
	})
	require.NoError(t, err)

	// Create execution for Node 1
	execData := map[string]interface{}{
		"NodeWithExec": map[string]interface{}{
			"output": "exec_val",
		},
	}
	execBytes, _ := json.Marshal(execData)
	now := time.Now().Unix()
	err = ts.nes.CreateNodeExecution(ctx, mflow.NodeExecution{
		ID:                     idwrap.NewNow(),
		NodeID:                 node1ID,
		Name:                   "Exec1",
		State:                  mflow.NODE_STATE_SUCCESS,
		CompletedAt:            &now,
		OutputData:             execBytes,
		OutputDataCompressType: compress.CompressTypeNone,
	})
	require.NoError(t, err)

	// 3. Previous Node without Execution Data (Schema Fallback) - using REQUEST node
	node2ID := idwrap.NewNow()
	err = ts.fns.CreateNode(ctx, mflow.Node{
		ID:       node2ID,
		FlowID:   flowID,
		Name:     "NodeRequest",
		NodeKind: mflow.NODE_KIND_REQUEST,
	})
	require.NoError(t, err)

	// 4. For Loop Node (Schema Fallback)
	node3ID := idwrap.NewNow()
	err = ts.fns.CreateNode(ctx, mflow.Node{
		ID:       node3ID,
		FlowID:   flowID,
		Name:     "LoopNode",
		NodeKind: mflow.NODE_KIND_FOR,
	})
	require.NoError(t, err)

	// 5. Target Node
	targetNodeID := idwrap.NewNow()
	err = ts.fns.CreateNode(ctx, mflow.Node{
		ID:       targetNodeID,
		FlowID:   flowID,
		Name:     "Target",
		NodeKind: mflow.NODE_KIND_JS,
	})
	require.NoError(t, err)

	// Connect edges
	// NodeWithExec -> Target
	err = ts.esvc.CreateEdge(ctx, mflow.Edge{
		ID:       idwrap.NewNow(),
		FlowID:   flowID,
		SourceID: node1ID,
		TargetID: targetNodeID,
	})
	require.NoError(t, err)

	// NodeRequest -> Target
	err = ts.esvc.CreateEdge(ctx, mflow.Edge{
		ID:       idwrap.NewNow(),
		FlowID:   flowID,
		SourceID: node2ID,
		TargetID: targetNodeID,
	})
	require.NoError(t, err)

	// LoopNode -> Target
	err = ts.esvc.CreateEdge(ctx, mflow.Edge{
		ID:       idwrap.NewNow(),
		FlowID:   flowID,
		SourceID: node3ID,
		TargetID: targetNodeID,
	})
	require.NoError(t, err)

	// Request Tree
	req := connect.NewRequest(&referencev1.ReferenceTreeRequest{
		FlowNodeId: targetNodeID.Bytes(),
	})

	resp, err := svc.ReferenceTree(ctx, req)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.Items)

	// Verify Flow Variable
	var flowVarItem *referencev1.ReferenceTreeItem
	for _, item := range resp.Msg.Items {
		if item.Key.GetKey() == "flow_var" {
			flowVarItem = item
			break
		}
	}
	require.NotNil(t, flowVarItem)
	require.NotNil(t, flowVarItem.Value)
	assert.Equal(t, "flow_val", *flowVarItem.Value)

	// Verify NodeWithExec
	var node1Item *referencev1.ReferenceTreeItem
	for _, item := range resp.Msg.Items {
		if item.Key.GetKey() == "NodeWithExec" {
			node1Item = item
			break
		}
	}
	require.NotNil(t, node1Item)
	// Check structure: NodeWithExec -> output
	var outputItem *referencev1.ReferenceTreeItem
	for _, item := range node1Item.Map {
		if item.Key.GetKey() == "output" {
			outputItem = item
			break
		}
	}
	require.NotNil(t, outputItem)
	require.NotNil(t, outputItem.Value)
	assert.Equal(t, "exec_val", *outputItem.Value)

	// Verify NodeRequest (Schema)
	var node2Item *referencev1.ReferenceTreeItem
	for _, item := range resp.Msg.Items {
		if item.Key.GetKey() == "NodeRequest" {
			node2Item = item
			break
		}
	}
	require.NotNil(t, node2Item)
	// Check structure: NodeRequest -> request/response
	var requestItem *referencev1.ReferenceTreeItem
	for _, item := range node2Item.Map {
		if item.Key.GetKey() == "request" {
			requestItem = item
			break
		}
	}
	require.NotNil(t, requestItem)

	// Verify LoopNode (Schema)
	var node3Item *referencev1.ReferenceTreeItem
	for _, item := range resp.Msg.Items {
		if item.Key.GetKey() == "LoopNode" {
			node3Item = item
			break
		}
	}
	require.NotNil(t, node3Item)
	// Check structure: LoopNode -> index
	var indexItem *referencev1.ReferenceTreeItem
	for _, item := range node3Item.Map {
		if item.Key.GetKey() == "index" {
			indexItem = item
			break
		}
	}
	require.NotNil(t, indexItem)
	// The default value for index is 0 (int), which is likely marshaled to float64 or int in the tree
	// We just check it exists and has a value
	require.NotNil(t, indexItem.Value)
}
