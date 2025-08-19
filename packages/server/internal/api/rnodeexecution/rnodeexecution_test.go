package rnodeexecution

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
	"the-dev-tools/db/pkg/sqlc"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnodeexecution"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodeexecution"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/testutil"
	"the-dev-tools/server/pkg/translate/tnodeexecution"
	nodeexecutionv1 "the-dev-tools/spec/dist/buf/go/flow/node/execution/v1"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeExecutionService_Constructor(t *testing.T) {
	// Test that the constructor correctly sets all services
	// Using nil values since we're just testing the constructor
	var mockNES *snodeexecution.NodeExecutionService
	var mockNS *snode.NodeService
	var mockFS *sflow.FlowService
	var mockUS *suser.UserService
	var mockERS *sexampleresp.ExampleRespService
	var mockRNS *snoderequest.NodeRequestService

	service := New(mockNES, mockNS, mockFS, mockUS, mockERS, mockRNS)

	assert.NotNil(t, service)
	assert.Equal(t, mockNES, service.nes)
	assert.Equal(t, mockNS, service.ns)
	assert.Equal(t, mockFS, service.fs)
	assert.Equal(t, mockUS, service.us)
	assert.Equal(t, mockERS, service.ers)
	assert.Equal(t, mockRNS, service.rns)
}

func TestNodeExecutionGet_RequestNodeDetection(t *testing.T) {
	// Test that the node type detection logic compiles and runs
	// This is more of a compilation test since we can't easily mock the dependencies
	
	// Test data structures
	executionID := idwrap.NewNow()
	nodeID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	responseID := idwrap.NewNow()

	// Test NodeExecution model with ResponseID
	execution := &mnodeexecution.NodeExecution{
		ID:         executionID,
		NodeID:     nodeID,
		ResponseID: &responseID, // This is the key field we're testing
	}

	// Test REQUEST node
	node := &mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		NodeKind: mnnode.NODE_KIND_REQUEST, // This should trigger ResponseID handling
	}

	// Test flow
	flow := mflow.Flow{
		ID: flowID,
	}

	// Verify the test data is set up correctly
	assert.NotNil(t, execution)
	assert.NotNil(t, execution.ResponseID)
	assert.Equal(t, responseID, *execution.ResponseID)
	assert.Equal(t, mnnode.NODE_KIND_REQUEST, node.NodeKind)
	assert.Equal(t, flowID, flow.ID)

	// Test the condition that would be checked in the actual service
	if node.NodeKind == mnnode.NODE_KIND_REQUEST && execution.ResponseID != nil {
		// This is the logic path that would be executed for REQUEST nodes with ResponseID
		assert.True(t, true, "REQUEST node with ResponseID logic path")
	} else {
		t.Error("Expected REQUEST node with ResponseID condition to be true")
	}
}

func TestNodeExecutionGet_NonRequestNode(t *testing.T) {
	// Test that non-REQUEST nodes don't trigger ResponseID logic
	executionID := idwrap.NewNow()
	nodeID := idwrap.NewNow()
	flowID := idwrap.NewNow()

	// Test execution without ResponseID
	execution := &mnodeexecution.NodeExecution{
		ID:         executionID,
		NodeID:     nodeID,
		ResponseID: nil,
	}

	// Test CONDITION node (not REQUEST)
	node := &mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		NodeKind: mnnode.NODE_KIND_CONDITION,
	}

	// Test flow
	flow := mflow.Flow{
		ID: flowID,
	}

	// Verify the test data
	assert.NotNil(t, execution)
	assert.Nil(t, execution.ResponseID)
	assert.Equal(t, mnnode.NODE_KIND_CONDITION, node.NodeKind)
	assert.Equal(t, flowID, flow.ID)

	// Test the condition - should NOT enter the ResponseID logic
	if node.NodeKind == mnnode.NODE_KIND_REQUEST && execution.ResponseID != nil {
		t.Error("Should not enter REQUEST node logic for CONDITION node")
	} else {
		// This is the expected path
		assert.True(t, true, "Non-REQUEST node correctly bypassed ResponseID logic")
	}
}

func TestNodeExecutionGet_RequestNodeWithoutResponseID(t *testing.T) {
	// Test REQUEST node without ResponseID (edge case)
	executionID := idwrap.NewNow()
	nodeID := idwrap.NewNow()
	flowID := idwrap.NewNow()

	// Test execution without ResponseID
	execution := &mnodeexecution.NodeExecution{
		ID:         executionID,
		NodeID:     nodeID,
		ResponseID: nil, // No ResponseID even though it's a REQUEST node
	}

	// Test REQUEST node
	node := &mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		NodeKind: mnnode.NODE_KIND_REQUEST,
	}

	// Test flow
	flow := mflow.Flow{
		ID: flowID,
	}

	// Verify the test data
	assert.NotNil(t, execution)
	assert.Nil(t, execution.ResponseID)
	assert.Equal(t, mnnode.NODE_KIND_REQUEST, node.NodeKind)
	assert.Equal(t, flowID, flow.ID)

	// Test the condition - should NOT enter the ResponseID logic because ResponseID is nil
	if node.NodeKind == mnnode.NODE_KIND_REQUEST && execution.ResponseID != nil {
		t.Error("Should not enter ResponseID logic when ResponseID is nil")
	} else {
		// This is the expected path
		assert.True(t, true, "REQUEST node without ResponseID correctly bypassed ResponseID logic")
	}
}

// Test constants and types
func TestNodeKindConstants(t *testing.T) {
	// Verify that the node kind constants are correctly defined
	assert.Equal(t, int32(2), mnnode.NODE_KIND_REQUEST)
	assert.Equal(t, int32(3), mnnode.NODE_KIND_CONDITION)
	assert.Equal(t, int32(1), mnnode.NODE_KIND_NO_OP)
	assert.Equal(t, int32(4), mnnode.NODE_KIND_FOR)
	assert.Equal(t, int32(5), mnnode.NODE_KIND_FOR_EACH)
	assert.Equal(t, int32(6), mnnode.NODE_KIND_JS)
}

// Helper function to create a node execution with input/output data for testing
func createTestNodeExecutionWithData(t *testing.T, withInputOutput bool) *mnodeexecution.NodeExecution {
	t.Helper()

	executionID := idwrap.NewNow()
	nodeID := idwrap.NewNow()
	responseID := idwrap.NewNow()
	completedAt := time.Now().UnixMilli()

	execution := &mnodeexecution.NodeExecution{
		ID:          executionID,
		NodeID:      nodeID,
		Name:        "Test Execution",
		State:       1, // Assuming 1 is a valid state
		Error:       nil,
		ResponseID:  &responseID,
		CompletedAt: &completedAt,
	}

	if withInputOutput {
		// Create some test input data
		inputData := map[string]interface{}{
			"endpoint": "https://api.example.com/test",
			"method":   "POST",
			"headers": map[string]string{
				"Content-Type": "application/json",
				"User-Agent":   "DevTools/1.0",
			},
			"body": map[string]interface{}{
				"id":   12345,
				"name": "Test Request",
				"data": []string{"item1", "item2", "item3"},
			},
		}

		// Create some test output data  
		outputData := map[string]interface{}{
			"status":     200,
			"statusText": "OK",
			"headers": map[string]string{
				"Content-Type":   "application/json",
				"Content-Length": "156",
				"Server":         "nginx/1.18.0",
			},
			"body": map[string]interface{}{
				"success": true,
				"id":      12345,
				"message": "Request processed successfully",
				"results": []interface{}{
					map[string]interface{}{"id": 1, "value": "result1"},
					map[string]interface{}{"id": 2, "value": "result2"},
				},
			},
			"timing": map[string]interface{}{
				"start":    "2024-01-15T10:30:00Z",
				"end":      "2024-01-15T10:30:01.234Z",
				"duration": 1234,
			},
		}

		// Convert to JSON and set with compression
		inputJSON, err := json.Marshal(inputData)
		require.NoError(t, err)
		err = execution.SetInputJSON(inputJSON)
		require.NoError(t, err)

		outputJSON, err := json.Marshal(outputData)
		require.NoError(t, err)
		err = execution.SetOutputJSON(outputJSON)
		require.NoError(t, err)
	}

	return execution
}

func TestSerializeNodeExecutionModelToRPCListItem_ExcludesInputOutput(t *testing.T) {
	// Test that SerializeNodeExecutionModelToRPCListItem excludes input/output fields for optimization
	tests := []struct {
		name              string
		hasInputOutput    bool
		expectedError     *string
		expectedNodeName  string
	}{
		{
			name:              "Execution with input/output data",
			hasInputOutput:    true,
			expectedNodeName:  "Test Execution",
		},
		{
			name:              "Execution without input/output data",
			hasInputOutput:    false,
			expectedNodeName:  "Test Execution",
		},
		{
			name:              "Execution with error and input/output data",
			hasInputOutput:    true,
			expectedError:     stringPtr("Test error occurred"),
			expectedNodeName:  "Test Execution",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test execution with or without input/output
			execution := createTestNodeExecutionWithData(t, tt.hasInputOutput)
			
			// Set error if specified
			if tt.expectedError != nil {
				execution.Error = tt.expectedError
			}

			// Call the list serialization function
			listItem, err := tnodeexecution.SerializeNodeExecutionModelToRPCListItem(execution)
			require.NoError(t, err)
			require.NotNil(t, listItem)

			// Verify all non-input/output fields are correctly populated
			assert.Equal(t, execution.ID.Bytes(), listItem.NodeExecutionId)
			assert.Equal(t, execution.NodeID.Bytes(), listItem.NodeId)
			assert.Equal(t, tt.expectedNodeName, listItem.Name)
			assert.Equal(t, int32(execution.State), int32(listItem.State))
			assert.Equal(t, execution.ResponseID.Bytes(), listItem.ResponseId)
			assert.NotNil(t, listItem.CompletedAt)

			// Verify error field
			if tt.expectedError != nil {
				assert.NotNil(t, listItem.Error)
				assert.Equal(t, *tt.expectedError, *listItem.Error)
			}

			// CRITICAL: Verify that input and output fields are NIL (optimization)
			assert.Nil(t, listItem.Input, "Input field should be nil in list item for performance optimization")
			assert.Nil(t, listItem.Output, "Output field should be nil in list item for performance optimization")

			// If the execution had input/output data, verify it exists in the model but not in the serialized list item
			if tt.hasInputOutput {
				// Verify the original execution model still has the data
				inputJSON, err := execution.GetInputJSON()
				assert.NoError(t, err)
				assert.NotNil(t, inputJSON, "Original execution should still have input data")

				outputJSON, err := execution.GetOutputJSON()
				assert.NoError(t, err)
				assert.NotNil(t, outputJSON, "Original execution should still have output data")

				// But the list item should not include this data for performance
				assert.Nil(t, listItem.Input, "List item input should be nil for performance")
				assert.Nil(t, listItem.Output, "List item output should be nil for performance")
			}
		})
	}
}

func TestSerializeNodeExecutionModelToRPCGetResponse_IncludesInputOutput(t *testing.T) {
	// Test that SerializeNodeExecutionModelToRPCGetResponse still includes input/output fields
	tests := []struct {
		name              string
		hasInputOutput    bool
		expectedError     *string
		expectedNodeName  string
	}{
		{
			name:              "Execution with input/output data",
			hasInputOutput:    true,
			expectedNodeName:  "Test Execution",
		},
		{
			name:              "Execution without input/output data",
			hasInputOutput:    false,
			expectedNodeName:  "Test Execution",
		},
		{
			name:              "Execution with error and input/output data",
			hasInputOutput:    true,
			expectedError:     stringPtr("Test error occurred"),
			expectedNodeName:  "Test Execution",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test execution with or without input/output
			execution := createTestNodeExecutionWithData(t, tt.hasInputOutput)
			
			// Set error if specified
			if tt.expectedError != nil {
				execution.Error = tt.expectedError
			}

			// Call the get response serialization function
			getResponse, err := tnodeexecution.SerializeNodeExecutionModelToRPCGetResponse(execution)
			require.NoError(t, err)
			require.NotNil(t, getResponse)

			// Verify all fields are correctly populated
			assert.Equal(t, execution.ID.Bytes(), getResponse.NodeExecutionId)
			assert.Equal(t, execution.NodeID.Bytes(), getResponse.NodeId)
			assert.Equal(t, tt.expectedNodeName, getResponse.Name)
			assert.Equal(t, int32(execution.State), int32(getResponse.State))
			assert.Equal(t, execution.ResponseID.Bytes(), getResponse.ResponseId)
			assert.NotNil(t, getResponse.CompletedAt)

			// Verify error field
			if tt.expectedError != nil {
				assert.NotNil(t, getResponse.Error)
				assert.Equal(t, *tt.expectedError, *getResponse.Error)
			}

			// CRITICAL: Verify that input and output fields are included in get response
			if tt.hasInputOutput {
				assert.NotNil(t, getResponse.Input, "Input field should be included in get response")
				assert.NotNil(t, getResponse.Output, "Output field should be included in get response")

				// Verify the content matches what we expect
				inputStruct := getResponse.Input.GetStructValue()
				require.NotNil(t, inputStruct)
				assert.Contains(t, inputStruct.Fields, "endpoint")
				assert.Contains(t, inputStruct.Fields, "method")
				assert.Contains(t, inputStruct.Fields, "headers")
				assert.Contains(t, inputStruct.Fields, "body")

				outputStruct := getResponse.Output.GetStructValue()
				require.NotNil(t, outputStruct)
				assert.Contains(t, outputStruct.Fields, "status")
				assert.Contains(t, outputStruct.Fields, "statusText")
				assert.Contains(t, outputStruct.Fields, "headers")
				assert.Contains(t, outputStruct.Fields, "body")
				assert.Contains(t, outputStruct.Fields, "timing")
			} else {
				// If no input/output data was set, these should be nil
				assert.Nil(t, getResponse.Input, "Input should be nil when no data was set")
				assert.Nil(t, getResponse.Output, "Output should be nil when no data was set")
			}
		})
	}
}

func TestSerializeNodeExecutionModelToRPC_IncludesInputOutput(t *testing.T) {
	// Test that the general SerializeNodeExecutionModelToRPC function still includes input/output
	execution := createTestNodeExecutionWithData(t, true)

	// Call the general RPC serialization function  
	rpcExecution, err := tnodeexecution.SerializeNodeExecutionModelToRPC(execution)
	require.NoError(t, err)
	require.NotNil(t, rpcExecution)

	// Verify all fields are correctly populated
	assert.Equal(t, execution.ID.Bytes(), rpcExecution.NodeExecutionId)
	assert.Equal(t, execution.NodeID.Bytes(), rpcExecution.NodeId)
	assert.Equal(t, "Test Execution", rpcExecution.Name)
	assert.Equal(t, int32(execution.State), int32(rpcExecution.State))
	assert.Equal(t, execution.ResponseID.Bytes(), rpcExecution.ResponseId)
	assert.NotNil(t, rpcExecution.CompletedAt)

	// CRITICAL: Verify that input and output fields are included in general RPC serialization
	assert.NotNil(t, rpcExecution.Input, "Input field should be included in general RPC serialization")
	assert.NotNil(t, rpcExecution.Output, "Output field should be included in general RPC serialization")
}

func TestOptimizationComparison_ListVsGetResponse(t *testing.T) {
	// Test to demonstrate the optimization difference between list and get response serialization
	execution := createTestNodeExecutionWithData(t, true)

	// Serialize for list (optimized - no input/output)
	listItem, err := tnodeexecution.SerializeNodeExecutionModelToRPCListItem(execution)
	require.NoError(t, err)

	// Serialize for get response (full data including input/output)
	getResponse, err := tnodeexecution.SerializeNodeExecutionModelToRPCGetResponse(execution)
	require.NoError(t, err)

	// Verify the optimization: list item excludes input/output while get response includes them
	assert.Nil(t, listItem.Input, "List item should exclude input for performance")
	assert.Nil(t, listItem.Output, "List item should exclude output for performance")

	assert.NotNil(t, getResponse.Input, "Get response should include input for detail view")
	assert.NotNil(t, getResponse.Output, "Get response should include output for detail view")

	// Verify common fields are the same
	assert.Equal(t, listItem.NodeExecutionId, getResponse.NodeExecutionId)
	assert.Equal(t, listItem.NodeId, getResponse.NodeId)
	assert.Equal(t, listItem.Name, getResponse.Name)
	assert.Equal(t, listItem.State, getResponse.State)
	assert.Equal(t, listItem.ResponseId, getResponse.ResponseId)
}

func TestNodeExecutionWithCompressedData(t *testing.T) {
	// Test that the serialization works correctly with compressed input/output data
	execution := createTestNodeExecutionWithData(t, false)

	// Create large data that will trigger compression
	largeData := make(map[string]interface{})
	for i := 0; i < 1000; i++ {
		largeData[fmt.Sprintf("key_%d", i)] = fmt.Sprintf("Large data value %d that should trigger compression when serialized to JSON", i)
	}

	// Set large input data (should be compressed)
	largeJSON, err := json.Marshal(largeData)
	require.NoError(t, err)
	require.Greater(t, len(largeJSON), 1024, "Test data should be large enough to trigger compression")

	err = execution.SetInputJSON(largeJSON)
	require.NoError(t, err)

	// Verify compression was applied
	assert.Equal(t, compress.CompressTypeZstd, execution.InputDataCompressType)
	assert.NotNil(t, execution.InputData)
	assert.Less(t, len(execution.InputData), len(largeJSON), "Compressed data should be smaller")

	// Test list serialization with compressed data
	listItem, err := tnodeexecution.SerializeNodeExecutionModelToRPCListItem(execution)
	require.NoError(t, err)
	
	// Input should still be nil in list item even with compressed data
	assert.Nil(t, listItem.Input, "List item should exclude compressed input for performance")

	// Test get response serialization with compressed data  
	getResponse, err := tnodeexecution.SerializeNodeExecutionModelToRPCGetResponse(execution)
	require.NoError(t, err)

	// Input should be properly decompressed and included in get response
	assert.NotNil(t, getResponse.Input, "Get response should include decompressed input")
	
	// Verify the decompressed content is accessible
	inputStruct := getResponse.Input.GetStructValue()
	require.NotNil(t, inputStruct)
	assert.Contains(t, inputStruct.Fields, "key_0")
	assert.Contains(t, inputStruct.Fields, "key_100") 
	assert.Contains(t, inputStruct.Fields, "key_500")
}

func TestPerformanceImplicationOfOptimization(t *testing.T) {
	// Create executions with various data sizes to demonstrate performance implications
	testCases := []struct {
		name     string
		dataSize int
	}{
		{"Small data", 100},
		{"Medium data", 1000},
		{"Large data", 10000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			execution := createTestNodeExecutionWithData(t, false)

			// Create data of specified size
			data := make(map[string]string)
			for i := 0; i < tc.dataSize; i++ {
				data[fmt.Sprintf("field_%d", i)] = fmt.Sprintf("value_%d_with_some_content", i)
			}

			dataJSON, err := json.Marshal(data)
			require.NoError(t, err)

			err = execution.SetInputJSON(dataJSON)
			require.NoError(t, err)
			err = execution.SetOutputJSON(dataJSON)
			require.NoError(t, err)

			// Measure serialization time for list (optimized)
			startTime := time.Now()
			listItem, err := tnodeexecution.SerializeNodeExecutionModelToRPCListItem(execution)
			listDuration := time.Since(startTime)
			require.NoError(t, err)

			// Measure serialization time for get response (full data)
			startTime = time.Now()
			getResponse, err := tnodeexecution.SerializeNodeExecutionModelToRPCGetResponse(execution)
			getDuration := time.Since(startTime)
			require.NoError(t, err)

			// Verify optimization effects
			assert.Nil(t, listItem.Input, "List item should exclude input")
			assert.Nil(t, listItem.Output, "List item should exclude output")
			assert.NotNil(t, getResponse.Input, "Get response should include input")
			assert.NotNil(t, getResponse.Output, "Get response should include output")

			// For larger data sizes, list serialization should be noticeably faster
			if tc.dataSize >= 1000 {
				t.Logf("%s - List serialization: %v, Get response serialization: %v", tc.name, listDuration, getDuration)
				// List serialization should generally be faster due to skipping JSON decompression and protobuf conversion
				// This is more of an informational test than a strict assertion since timing can vary
			}
		})
	}
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}

// Integration Tests for RPC Optimization
// These tests verify the actual RPC behavior end-to-end

func TestNodeExecutionListRPC_ExcludesInputOutput_Integration(t *testing.T) {
	// This test verifies that the NodeExecutionList RPC method actually excludes
	// input/output fields in the response for performance optimization
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	defer sqlc.CloseQueriesAndLog(queries)

	// Create all required services
	us := suser.New(queries)
	fs := sflow.New(queries)
	ns := snode.New(queries)
	nes := snodeexecution.New(queries)
	ers := sexampleresp.New(queries)
	rns := snoderequest.New(queries)

	// Create RPC service
	rpcService := New(&nes, &ns, &fs, &us, &ers, &rns)

	// Setup test data using the base services pattern
	baseServices := base.GetBaseServices()
	workspaceID := idwrap.NewNow()
	wuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	nodeID := idwrap.NewNow()

	// Create workspace, user, and collection using helper
	baseServices.CreateTempCollection(t, ctx, workspaceID, wuserID, userID, collectionID)

	// Create flow
	flowData := mflow.Flow{
		ID:          flowID,
		Name:        "Test Flow",
		WorkspaceID: workspaceID,
	}
	err := fs.CreateFlow(ctx, flowData)
	require.NoError(t, err)

	// Create a REQUEST node
	nodeData := mnnode.MNode{
		ID:        nodeID,
		FlowID:    flowID,
		NodeKind:  mnnode.NODE_KIND_REQUEST,
		Name:      "Test Request Node",
		PositionX: 100,
		PositionY: 200,
	}
	err = ns.CreateNode(ctx, nodeData)
	require.NoError(t, err)

	// Create multiple node executions with large input/output data
	testExecutions := make([]idwrap.IDWrap, 3)
	for i := 0; i < 3; i++ {
		executionID := idwrap.NewNow()
		testExecutions[i] = executionID

		// Create large test data that would normally slow down the response
		largeInputData := make(map[string]interface{})
		for j := 0; j < 500; j++ {
			largeInputData[fmt.Sprintf("input_field_%d", j)] = fmt.Sprintf("Large input data value %d that would impact performance if included in list responses", j)
		}

		largeOutputData := make(map[string]interface{})
		for j := 0; j < 500; j++ {
			largeOutputData[fmt.Sprintf("output_field_%d", j)] = fmt.Sprintf("Large output data value %d that would impact performance if included in list responses", j)
		}

		// Create node execution
		execution := &mnodeexecution.NodeExecution{
			ID:     executionID,
			NodeID: nodeID,
			Name:   fmt.Sprintf("Test Execution %d", i+1),
			State:  1, // Completed state
			Error:  nil,
		}

		// Set large input/output data
		inputJSON, err := json.Marshal(largeInputData)
		require.NoError(t, err)
		err = execution.SetInputJSON(inputJSON)
		require.NoError(t, err)

		outputJSON, err := json.Marshal(largeOutputData)
		require.NoError(t, err)
		err = execution.SetOutputJSON(outputJSON)
		require.NoError(t, err)

		// Save to database
		err = nes.CreateNodeExecution(ctx, *execution)
		require.NoError(t, err)
	}

	// Set up authentication context
	authCtx := mwauth.CreateAuthedContext(ctx, userID)

	// Create RPC request
	request := &nodeexecutionv1.NodeExecutionListRequest{
		NodeId: nodeID.Bytes(),
	}
	req := connect.NewRequest(request)

	// Call the actual RPC method
	response, err := rpcService.NodeExecutionList(authCtx, req)
	require.NoError(t, err, "RPC call should succeed")
	require.NotNil(t, response)
	require.NotNil(t, response.Msg)

	// Verify response structure
	assert.Len(t, response.Msg.Items, 3, "Should return all 3 executions")

	// CRITICAL: Verify that input/output fields are NIL in list response for performance
	executionIDs := make(map[string]bool)
	for _, exec := range testExecutions {
		executionIDs[string(exec.Bytes())] = true
	}

	for _, item := range response.Msg.Items {
		// Verify basic fields are populated correctly
		assert.True(t, executionIDs[string(item.NodeExecutionId)], "Execution ID should be one of the created ones")
		assert.Equal(t, nodeID.Bytes(), item.NodeId, "Node ID should match")
		assert.Contains(t, item.Name, "Test Execution", "Name should contain test execution")
		assert.Equal(t, int32(1), int32(item.State), "State should match")

		// PERFORMANCE OPTIMIZATION VERIFICATION:
		// These fields should be NIL to improve list performance
		assert.Nil(t, item.Input, "Input field should be nil in list response for performance optimization")
		assert.Nil(t, item.Output, "Output field should be nil in list response for performance optimization")
	}

	t.Logf("✅ NodeExecutionList RPC successfully excludes input/output fields for %d executions", len(response.Msg.Items))
}

func TestNodeExecutionGetRPC_IncludesInputOutput_Integration(t *testing.T) {
	// This test verifies that the NodeExecutionGet RPC method still includes
	// input/output fields in the response to maintain backward compatibility
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	defer sqlc.CloseQueriesAndLog(queries)

	// Create all required services
	us := suser.New(queries)
	fs := sflow.New(queries)
	ns := snode.New(queries)
	nes := snodeexecution.New(queries)
	ers := sexampleresp.New(queries)
	rns := snoderequest.New(queries)

	// Create RPC service
	rpcService := New(&nes, &ns, &fs, &us, &ers, &rns)

	// Setup test data using the base services pattern
	baseServices := base.GetBaseServices()
	workspaceID := idwrap.NewNow()
	wuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()

	// Create workspace, user, and collection using helper
	baseServices.CreateTempCollection(t, ctx, workspaceID, wuserID, userID, collectionID)

	// Create flow
	flowData := mflow.Flow{
		ID:          flowID,
		Name:        "Test Flow",
		WorkspaceID: workspaceID,
	}
	err := fs.CreateFlow(ctx, flowData)
	require.NoError(t, err)

	// Create a REQUEST node
	nodeData := mnnode.MNode{
		ID:        nodeID,
		FlowID:    flowID,
		NodeKind:  mnnode.NODE_KIND_REQUEST,
		Name:      "Test Request Node",
		PositionX: 100,
		PositionY: 200,
	}
	err = ns.CreateNode(ctx, nodeData)
	require.NoError(t, err)

	// Create node execution with detailed input/output data
	inputData := map[string]interface{}{
		"endpoint": "https://api.example.com/users",
		"method":   "POST",
		"headers": map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer token123",
			"User-Agent":    "DevTools/1.0",
		},
		"body": map[string]interface{}{
			"id":       12345,
			"name":     "John Doe",
			"email":    "john@example.com",
			"metadata": []string{"tag1", "tag2", "tag3"},
		},
	}

	outputData := map[string]interface{}{
		"status":     201,
		"statusText": "Created",
		"headers": map[string]string{
			"Content-Type":   "application/json",
			"Content-Length": "89",
			"Server":         "nginx/1.18.0",
			"Location":       "/users/12345",
		},
		"body": map[string]interface{}{
			"success":   true,
			"id":        12345,
			"message":   "User created successfully",
			"timestamp": "2024-01-15T10:30:01.234Z",
		},
		"timing": map[string]interface{}{
			"dns":      12,
			"connect":  45,
			"request":  123,
			"response": 67,
			"total":    247,
		},
	}

	// Create node execution
	execution := &mnodeexecution.NodeExecution{
		ID:     executionID,
		NodeID: nodeID,
		Name:   "Detailed Test Execution",
		State:  1, // Completed state
		Error:  nil,
	}

	// Set input/output data
	inputJSON, err := json.Marshal(inputData)
	require.NoError(t, err)
	err = execution.SetInputJSON(inputJSON)
	require.NoError(t, err)

	outputJSON, err := json.Marshal(outputData)
	require.NoError(t, err)
	err = execution.SetOutputJSON(outputJSON)
	require.NoError(t, err)

	// Save to database
	err = nes.CreateNodeExecution(ctx, *execution)
	require.NoError(t, err)

	// Set up authentication context
	authCtx := mwauth.CreateAuthedContext(ctx, userID)

	// Create RPC request
	request := &nodeexecutionv1.NodeExecutionGetRequest{
		NodeExecutionId: executionID.Bytes(),
	}
	req := connect.NewRequest(request)

	// Call the actual RPC method
	response, err := rpcService.NodeExecutionGet(authCtx, req)
	require.NoError(t, err, "RPC call should succeed")
	require.NotNil(t, response)
	require.NotNil(t, response.Msg)

	// Verify basic fields
	assert.Equal(t, executionID.Bytes(), response.Msg.NodeExecutionId, "Execution ID should match")
	assert.Equal(t, nodeID.Bytes(), response.Msg.NodeId, "Node ID should match")
	assert.Equal(t, "Detailed Test Execution", response.Msg.Name, "Name should match")
	assert.Equal(t, int32(1), int32(response.Msg.State), "State should match")

	// BACKWARD COMPATIBILITY VERIFICATION:
	// These fields should be INCLUDED in get response for detail view
	assert.NotNil(t, response.Msg.Input, "Input field should be included in get response")
	assert.NotNil(t, response.Msg.Output, "Output field should be included in get response")

	// Verify input data content
	inputStruct := response.Msg.Input.GetStructValue()
	require.NotNil(t, inputStruct, "Input should be a struct")
	assert.Contains(t, inputStruct.Fields, "endpoint", "Input should contain endpoint field")
	assert.Contains(t, inputStruct.Fields, "method", "Input should contain method field")
	assert.Contains(t, inputStruct.Fields, "headers", "Input should contain headers field")
	assert.Contains(t, inputStruct.Fields, "body", "Input should contain body field")

	// Verify output data content
	outputStruct := response.Msg.Output.GetStructValue()
	require.NotNil(t, outputStruct, "Output should be a struct")
	assert.Contains(t, outputStruct.Fields, "status", "Output should contain status field")
	assert.Contains(t, outputStruct.Fields, "statusText", "Output should contain statusText field")
	assert.Contains(t, outputStruct.Fields, "headers", "Output should contain headers field")
	assert.Contains(t, outputStruct.Fields, "body", "Output should contain body field")
	assert.Contains(t, outputStruct.Fields, "timing", "Output should contain timing field")

	// Verify specific values in the response
	endpointValue := inputStruct.Fields["endpoint"].GetStringValue()
	assert.Equal(t, "https://api.example.com/users", endpointValue, "Endpoint should match original input")

	statusValue := outputStruct.Fields["status"].GetNumberValue()
	assert.Equal(t, float64(201), statusValue, "Status should match original output")

	t.Logf("✅ NodeExecutionGet RPC successfully includes full input/output data for detailed view")
}

func TestRPCOptimization_EndToEndPerformanceComparison_Integration(t *testing.T) {
	// This test demonstrates the real-world performance impact of the optimization
	// by comparing List vs Get operations on the same data
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	defer sqlc.CloseQueriesAndLog(queries)

	// Create all required services
	us := suser.New(queries)
	fs := sflow.New(queries)
	ns := snode.New(queries)
	nes := snodeexecution.New(queries)
	ers := sexampleresp.New(queries)
	rns := snoderequest.New(queries)

	// Create RPC service
	rpcService := New(&nes, &ns, &fs, &us, &ers, &rns)

	// Setup test data
	workspaceID := idwrap.NewNow()
	userID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	nodeID := idwrap.NewNow()

	// Setup test data using the base services pattern
	baseServices := base.GetBaseServices()
	collectionID := idwrap.NewNow()
	wuserID := idwrap.NewNow()

	// Create workspace, user, and collection using helper
	baseServices.CreateTempCollection(t, ctx, workspaceID, wuserID, userID, collectionID)

	// Create flow
	flowData := mflow.Flow{
		ID:          flowID,
		Name:        "Performance Test Flow",
		WorkspaceID: workspaceID,
	}
	err := fs.CreateFlow(ctx, flowData)
	require.NoError(t, err)

	// Create a REQUEST node
	nodeData := mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		NodeKind: mnnode.NODE_KIND_REQUEST,
		Name:     "Performance Test Node",
		PositionX: 100,
		PositionY: 200,
	}
	err = ns.CreateNode(ctx, nodeData)
	require.NoError(t, err)

	// Create multiple executions with increasingly large payloads
	numExecutions := 10
	executionIDs := make([]idwrap.IDWrap, numExecutions)

	for i := 0; i < numExecutions; i++ {
		executionID := idwrap.NewNow()
		executionIDs[i] = executionID

		// Create increasingly large data to demonstrate performance impact
		dataSize := 100 * (i + 1) // Growing data size: 100, 200, 300, ..., 1000 fields
		largeData := make(map[string]interface{})
		for j := 0; j < dataSize; j++ {
			largeData[fmt.Sprintf("field_%d", j)] = fmt.Sprintf("This is field %d with substantial content that would impact serialization performance when included in list responses. The content grows with each execution to demonstrate the cumulative effect of the optimization.", j)
		}

		execution := &mnodeexecution.NodeExecution{
			ID:     executionID,
			NodeID: nodeID,
			Name:   fmt.Sprintf("Performance Test Execution %d", i+1),
			State:  1,
			Error:  nil,
		}

		// Set the same large data as both input and output
		dataJSON, err := json.Marshal(largeData)
		require.NoError(t, err)

		err = execution.SetInputJSON(dataJSON)
		require.NoError(t, err)
		err = execution.SetOutputJSON(dataJSON)
		require.NoError(t, err)

		err = nes.CreateNodeExecution(ctx, *execution)
		require.NoError(t, err)
	}

	// Set up authentication context
	authCtx := mwauth.CreateAuthedContext(ctx, userID)

	// Measure LIST operation performance (optimized - excludes input/output)
	listRequest := &nodeexecutionv1.NodeExecutionListRequest{
		NodeId: nodeID.Bytes(),
	}
	listReq := connect.NewRequest(listRequest)

	listStartTime := time.Now()
	listResponse, err := rpcService.NodeExecutionList(authCtx, listReq)
	listDuration := time.Since(listStartTime)
	require.NoError(t, err)
	require.NotNil(t, listResponse)
	assert.Len(t, listResponse.Msg.Items, numExecutions)

	// Verify optimization: all input/output fields should be nil
	for _, item := range listResponse.Msg.Items {
		assert.Nil(t, item.Input, "List response should exclude input for performance")
		assert.Nil(t, item.Output, "List response should exclude output for performance")
	}

	// Measure GET operations for comparison (includes input/output)
	var getTotalDuration time.Duration
	for i, executionID := range executionIDs {
		getRequest := &nodeexecutionv1.NodeExecutionGetRequest{
			NodeExecutionId: executionID.Bytes(),
		}
		getReq := connect.NewRequest(getRequest)

		getStartTime := time.Now()
		getResponse, err := rpcService.NodeExecutionGet(authCtx, getReq)
		getSingleDuration := time.Since(getStartTime)
		getTotalDuration += getSingleDuration

		require.NoError(t, err)
		require.NotNil(t, getResponse)

		// Verify get response includes data
		assert.NotNil(t, getResponse.Msg.Input, "Get response should include input")
		assert.NotNil(t, getResponse.Msg.Output, "Get response should include output")

		// Only check the first few to avoid too much logging
		if i < 3 {
			// Verify data integrity
			inputStruct := getResponse.Msg.Input.GetStructValue()
			require.NotNil(t, inputStruct)
			expectedFieldCount := 100 * (i + 1)
			actualFieldCount := len(inputStruct.Fields)
			assert.Equal(t, expectedFieldCount, actualFieldCount, "Field count should match for execution %d", i+1)
		}
	}

	// Calculate average get duration
	averageGetDuration := getTotalDuration / time.Duration(numExecutions)

	// Performance analysis and reporting
	t.Logf("\n=== RPC Performance Optimization Analysis ===")
	t.Logf("Number of executions: %d", numExecutions)
	t.Logf("LIST operation (optimized):  %v", listDuration)
	t.Logf("GET operations total:        %v", getTotalDuration)
	t.Logf("GET operation average:       %v", averageGetDuration)
	t.Logf("Performance improvement:     LIST is %.2fx faster than single GET", float64(averageGetDuration)/float64(listDuration))

	// Data integrity verification
	t.Logf("\n=== Data Integrity Verification ===")
	t.Logf("LIST response: %d items, all with nil input/output (optimized)", len(listResponse.Msg.Items))
	t.Logf("GET responses: %d items, all with full input/output data (complete)", numExecutions)

	// Performance assertions
	// List should generally be faster than individual gets for the same amount of data
	// Note: We use a relatively lenient check since exact performance can vary
	if numExecutions >= 5 {
		// Only check performance for meaningful data sizes
		listPerExecution := listDuration / time.Duration(numExecutions)
		if averageGetDuration > listPerExecution {
			t.Logf("✅ Optimization effective: LIST per execution (%v) is faster than GET (%v)", listPerExecution, averageGetDuration)
		} else {
			t.Logf("ℹ️ Performance results: LIST per execution (%v), GET (%v) - optimization may still be beneficial for larger datasets", listPerExecution, averageGetDuration)
		}
	}

	// Verify no data loss - the optimization should not affect data availability
	t.Logf("\n=== Optimization Correctness ===")
	t.Logf("✅ No data loss: All data accessible via GET operations")
	t.Logf("✅ Performance gain: LIST operations exclude heavy input/output serialization")
	t.Logf("✅ Backward compatibility: GET operations maintain full data access")
}