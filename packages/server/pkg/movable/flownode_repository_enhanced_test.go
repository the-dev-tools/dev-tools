package movable

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"the-dev-tools/server/pkg/idwrap"
)

// =============================================================================
// TEST SETUP AND UTILITIES
// =============================================================================

// testFlowNodeRepository creates a test instance of EnhancedFlowNodeRepository
func testFlowNodeRepository(t *testing.T) *EnhancedFlowNodeRepositoryImpl {
	// Create in-memory database for testing
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Create enhanced repository configuration
	baseConfig := &MoveConfig{
		EnableSafetyChecks: true,
		BatchSize:          50,
	}

	enhancedConfig := &EnhancedRepositoryConfig{
		MoveConfig:            baseConfig,
		ScopeResolver:         nil, // Use mock if needed
		ContextCache:          NewInMemoryContextCache(5 * time.Minute),
		EnableContextAware:    true,
		EnableDeltaSupport:    true,
		EnableScopeValidation: false, // Disable for testing
	}

	// Create flow node configuration
	flowConfig := &FlowNodeConfig{
		EnableDeltaSupport:          true,
		EnableBoundaryValidation:    false, // Disable for testing
		EnablePositionCompaction:    true,
		BatchSize:                   50,
		MaxNodesPerFlow:             1000,
		PositionCompactionThreshold: 10000.0,
		DefaultSpatialGrid:          50.0,
		MinSpatialDistance:          20.0,
		MaxSpatialCoordinate:        10000.0,
	}

	repo := NewEnhancedFlowNodeRepository(db, enhancedConfig, flowConfig)
	return repo
}

// createTestFlowContext creates a test flow boundary context
func createTestFlowContext(t *testing.T) *FlowBoundaryContext {
	return &FlowBoundaryContext{
		FlowID:        idwrap.NewTest(t, "flow_1"),
		WorkspaceID:   idwrap.NewTest(t, "workspace_1"),
		CollectionID:  nil,
		UserID:        idwrap.NewTest(t, "user_1"),
		EnforceStrict: false,
	}
}

// createTestFlowNodeItem creates a test flow node item
func createTestFlowNodeItem(t *testing.T, nodeID string, flowID idwrap.IDWrap,
	position FlowNodePosition, orderType FlowNodeOrderType) FlowNodeItem {

	return FlowNodeItem{
		ID:         idwrap.NewTest(t, nodeID),
		FlowID:     flowID,
		ParentID:   nil,
		Position:   position,
		Sequential: 0,
		NodeKind:   2, // REQUEST node
		ListType:   FlowListTypeNodes,
		OrderType:  orderType,
	}
}

// =============================================================================
// FLOW-SPECIFIC OPERATIONS TESTS
// =============================================================================

// TestGetNodesInFlow tests retrieving all nodes in a flow with proper ordering
func TestGetNodesInFlow(t *testing.T) {
	repo := testFlowNodeRepository(t)
	ctx := context.Background()
	flowID := idwrap.NewTest(t, "test_flow")

	tests := []struct {
		name        string
		flowID      idwrap.IDWrap
		orderType   FlowNodeOrderType
		expectError bool
	}{
		{
			name:        "Valid flow ID with spatial ordering",
			flowID:      flowID,
			orderType:   FlowNodeOrderSpatial,
			expectError: false,
		},
		{
			name:        "Valid flow ID with sequential ordering",
			flowID:      flowID,
			orderType:   FlowNodeOrderSequential,
			expectError: false,
		},
		{
			name:        "Empty flow ID",
			flowID:      idwrap.IDWrap{},
			orderType:   FlowNodeOrderSpatial,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes, err := repo.GetNodesInFlow(ctx, tt.flowID, tt.orderType)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for test '%s', but got none", tt.name)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for test '%s': %v", tt.name, err)
				return
			}

			if nodes == nil {
				t.Errorf("Expected non-nil nodes slice for test '%s'", tt.name)
			}

			// All returned nodes should belong to the specified flow
			for _, node := range nodes {
				if node.FlowID.Compare(tt.flowID) != 0 {
					t.Errorf("Node %s belongs to wrong flow: expected %s, got %s",
						node.ID.String(), tt.flowID.String(), node.FlowID.String())
				}
			}
		})
	}
}

// TestUpdateNodePosition tests updating a node's spatial position within a flow
func TestUpdateNodePosition(t *testing.T) {
	repo := testFlowNodeRepository(t)
	ctx := context.Background()
	nodeID := idwrap.NewTest(t, "test_node")
	flowContext := createTestFlowContext(t)

	tests := []struct {
		name        string
		nodeID      idwrap.IDWrap
		position    FlowNodePosition
		context     *FlowBoundaryContext
		expectError bool
	}{
		{
			name:   "Valid position update",
			nodeID: nodeID,
			position: FlowNodePosition{
				X: 100.0,
				Y: 200.0,
			},
			context:     flowContext,
			expectError: false,
		},
		{
			name:   "Invalid negative coordinates",
			nodeID: nodeID,
			position: FlowNodePosition{
				X: -100.0,
				Y: 200.0,
			},
			context:     flowContext,
			expectError: true,
		},
		{
			name:        "Empty node ID",
			nodeID:      idwrap.IDWrap{},
			position:    FlowNodePosition{X: 100.0, Y: 200.0},
			context:     flowContext,
			expectError: true,
		},
		{
			name:        "Nil flow context",
			nodeID:      nodeID,
			position:    FlowNodePosition{X: 100.0, Y: 200.0},
			context:     nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.UpdateNodePosition(ctx, nil, tt.nodeID, tt.position, tt.context)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for test '%s', but got none", tt.name)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for test '%s': %v", tt.name, err)
				}
			}
		})
	}
}

// TestBatchUpdateNodePositions tests efficiently updating multiple node positions
func TestBatchUpdateNodePositions(t *testing.T) {
	repo := testFlowNodeRepository(t)
	ctx := context.Background()
	flowContext := createTestFlowContext(t)

	// Create test updates
	validUpdates := []FlowNodePositionUpdate{
		{
			NodeID:    idwrap.NewTest(t, "node_1"),
			Position:  FlowNodePosition{X: 100.0, Y: 100.0},
			OrderType: FlowNodeOrderSpatial,
		},
		{
			NodeID:     idwrap.NewTest(t, "node_2"),
			Position:   FlowNodePosition{X: 200.0, Y: 200.0},
			Sequential: 1,
			OrderType:  FlowNodeOrderSequential,
		},
	}

	// Create oversized batch
	oversizedUpdates := make([]FlowNodePositionUpdate, repo.flowConfig.BatchSize+1)
	for i := range oversizedUpdates {
		oversizedUpdates[i] = FlowNodePositionUpdate{
			NodeID:    idwrap.NewTest(t, "node_"+string(rune(i))),
			Position:  FlowNodePosition{X: 100.0, Y: 100.0},
			OrderType: FlowNodeOrderSpatial,
		}
	}

	tests := []struct {
		name        string
		updates     []FlowNodePositionUpdate
		context     *FlowBoundaryContext
		expectError bool
	}{
		{
			name:        "Valid batch update",
			updates:     validUpdates,
			context:     flowContext,
			expectError: false,
		},
		{
			name:        "Empty updates",
			updates:     []FlowNodePositionUpdate{},
			context:     flowContext,
			expectError: false, // Should be no-op
		},
		{
			name:        "Oversized batch",
			updates:     oversizedUpdates,
			context:     flowContext,
			expectError: true,
		},
		{
			name:        "Nil flow context",
			updates:     validUpdates,
			context:     nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.BatchUpdateNodePositions(ctx, nil, tt.updates, tt.context)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for test '%s', but got none", tt.name)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for test '%s': %v", tt.name, err)
				}
			}
		})
	}
}

// TestValidateFlowBoundary tests flow scope boundary validation
func TestValidateFlowBoundary(t *testing.T) {
	repo := testFlowNodeRepository(t)
	ctx := context.Background()
	nodeID := idwrap.NewTest(t, "test_node")
	flowID := idwrap.NewTest(t, "test_flow")

	// Enable boundary validation for this test
	repo.flowConfig.EnableBoundaryValidation = true

	tests := []struct {
		name           string
		nodeID         idwrap.IDWrap
		expectedFlowID idwrap.IDWrap
		expectError    bool
	}{
		{
			name:           "Valid boundary check (disabled validation)",
			nodeID:         nodeID,
			expectedFlowID: flowID,
			expectError:    false,
		},
		{
			name:           "Empty node ID",
			nodeID:         idwrap.IDWrap{},
			expectedFlowID: flowID,
			expectError:    false, // Will be handled by scope resolver
		},
		{
			name:           "Empty flow ID",
			nodeID:         nodeID,
			expectedFlowID: idwrap.IDWrap{},
			expectError:    false, // Will be handled by scope resolver
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.ValidateFlowBoundary(ctx, tt.nodeID, tt.expectedFlowID)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for test '%s', but got none", tt.name)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for test '%s': %v", tt.name, err)
				}
			}
		})
	}

	// Reset boundary validation
	repo.flowConfig.EnableBoundaryValidation = false
}

// =============================================================================
// REQUEST NODE DELTA OPERATIONS TESTS
// =============================================================================

// TestHandleRequestNode tests managing REQUEST node with delta references
func TestHandleRequestNode(t *testing.T) {
	repo := testFlowNodeRepository(t)
	ctx := context.Background()
	nodeID := idwrap.NewTest(t, "request_node")
	flowID := idwrap.NewTest(t, "test_flow")

	validDeltaRef := &FlowNodeDeltaReference{
		NodeID:          nodeID,
		EndpointID:      idwrap.NewTestPtr(t, "endpoint_1"),
		ExampleID:       idwrap.NewTestPtr(t, "example_1"),
		DeltaEndpointID: idwrap.NewTestPtr(t, "delta_endpoint_1"),
		DeltaExampleID:  idwrap.NewTestPtr(t, "delta_example_1"),
		Context:         ContextFlow,
		FlowID:          flowID,
		IsActive:        true,
	}

	tests := []struct {
		name        string
		nodeID      idwrap.IDWrap
		deltaRef    *FlowNodeDeltaReference
		expectError bool
	}{
		{
			name:        "Valid REQUEST node with delta reference",
			nodeID:      nodeID,
			deltaRef:    validDeltaRef,
			expectError: false,
		},
		{
			name:        "Nil delta reference",
			nodeID:      nodeID,
			deltaRef:    nil,
			expectError: true,
		},
		{
			name:        "Empty node ID",
			nodeID:      idwrap.IDWrap{},
			deltaRef:    validDeltaRef,
			expectError: false, // Node ID will be set from deltaRef
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.HandleRequestNode(ctx, nil, tt.nodeID, tt.deltaRef)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for test '%s', but got none", tt.name)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for test '%s': %v", tt.name, err)
				}

				// Verify delta reference was stored
				if tt.deltaRef != nil {
					storedDeltas, err := repo.GetRequestNodeDeltas(ctx, flowID)
					if err != nil {
						t.Errorf("Failed to retrieve stored deltas: %v", err)
					} else if len(storedDeltas) == 0 {
						t.Error("Delta reference was not stored")
					}
				}
			}
		})
	}
}

// TestResolveRequestNodeDeltas tests resolving effective endpoints/examples for REQUEST nodes
func TestResolveRequestNodeDeltas(t *testing.T) {
	repo := testFlowNodeRepository(t)
	ctx := context.Background()
	nodeID := idwrap.NewTest(t, "request_node")
	flowContext := createTestFlowContext(t)

	// Set up a REQUEST node with deltas
	deltaRef := &FlowNodeDeltaReference{
		NodeID:          nodeID,
		EndpointID:      idwrap.NewTestPtr(t, "endpoint_1"),
		ExampleID:       idwrap.NewTestPtr(t, "example_1"),
		DeltaEndpointID: idwrap.NewTestPtr(t, "delta_endpoint_1"),
		DeltaExampleID:  idwrap.NewTestPtr(t, "delta_example_1"),
		Context:         ContextFlow,
		FlowID:          flowContext.FlowID,
		IsActive:        true,
	}

	// Store the delta reference
	err := repo.HandleRequestNode(ctx, nil, nodeID, deltaRef)
	if err != nil {
		t.Fatalf("Failed to set up REQUEST node: %v", err)
	}

	tests := []struct {
		name        string
		nodeID      idwrap.IDWrap
		context     *FlowBoundaryContext
		expectError bool
		expectNil   bool
	}{
		{
			name:        "Valid REQUEST node with deltas",
			nodeID:      nodeID,
			context:     flowContext,
			expectError: false,
			expectNil:   false,
		},
		{
			name:        "Non-existent node",
			nodeID:      idwrap.NewTest(t, "non_existent"),
			context:     flowContext,
			expectError: false,
			expectNil:   true, // Should return nil for non-existent deltas
		},
		{
			name:        "Empty node ID",
			nodeID:      idwrap.IDWrap{},
			context:     flowContext,
			expectError: false,
			expectNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, err := repo.ResolveRequestNodeDeltas(ctx, tt.nodeID, tt.context)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for test '%s', but got none", tt.name)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for test '%s': %v", tt.name, err)
				return
			}

			if tt.expectNil {
				if resolved != nil {
					t.Errorf("Expected nil resolved node for test '%s', but got %v", tt.name, resolved)
				}
			} else {
				if resolved == nil {
					t.Errorf("Expected non-nil resolved node for test '%s'", tt.name)
					return
				}

				// Verify resolution details
				if resolved.NodeID.Compare(tt.nodeID) != 0 {
					t.Errorf("Resolved node ID mismatch: expected %s, got %s",
						tt.nodeID.String(), resolved.NodeID.String())
				}

				if !resolved.HasDeltas {
					t.Error("Expected HasDeltas to be true")
				}

				// Verify effective IDs are delta IDs when deltas exist
				if resolved.EffectiveEndpointID == nil || resolved.EffectiveExampleID == nil {
					t.Error("Expected non-nil effective IDs")
				}
			}
		})
	}
}

// TestGetRequestNodeDeltas tests retrieving all delta references for REQUEST nodes in a flow
func TestGetRequestNodeDeltas(t *testing.T) {
	repo := testFlowNodeRepository(t)
	ctx := context.Background()
	flowID := idwrap.NewTest(t, "test_flow")
	otherFlowID := idwrap.NewTest(t, "other_flow")

	// Set up multiple REQUEST nodes with deltas
	node1ID := idwrap.NewTest(t, "request_node_1")
	node2ID := idwrap.NewTest(t, "request_node_2")
	node3ID := idwrap.NewTest(t, "request_node_3")

	deltaRef1 := &FlowNodeDeltaReference{
		NodeID:     node1ID,
		EndpointID: idwrap.NewTestPtr(t, "endpoint_1"),
		FlowID:     flowID,
		IsActive:   true,
	}

	deltaRef2 := &FlowNodeDeltaReference{
		NodeID:     node2ID,
		EndpointID: idwrap.NewTestPtr(t, "endpoint_2"),
		FlowID:     flowID,
		IsActive:   true,
	}

	deltaRef3 := &FlowNodeDeltaReference{
		NodeID:     node3ID,
		EndpointID: idwrap.NewTestPtr(t, "endpoint_3"),
		FlowID:     otherFlowID, // Different flow
		IsActive:   true,
	}

	// Store delta references
	_ = repo.HandleRequestNode(ctx, nil, node1ID, deltaRef1)
	_ = repo.HandleRequestNode(ctx, nil, node2ID, deltaRef2)
	_ = repo.HandleRequestNode(ctx, nil, node3ID, deltaRef3)

	tests := []struct {
		name          string
		flowID        idwrap.IDWrap
		expectedCount int
		expectError   bool
	}{
		{
			name:          "Flow with multiple deltas",
			flowID:        flowID,
			expectedCount: 2,
			expectError:   false,
		},
		{
			name:          "Flow with one delta",
			flowID:        otherFlowID,
			expectedCount: 1,
			expectError:   false,
		},
		{
			name:          "Flow with no deltas",
			flowID:        idwrap.NewTest(t, "empty_flow"),
			expectedCount: 0,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deltas, err := repo.GetRequestNodeDeltas(ctx, tt.flowID)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for test '%s', but got none", tt.name)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for test '%s': %v", tt.name, err)
				return
			}

			if len(deltas) != tt.expectedCount {
				t.Errorf("Expected %d deltas for test '%s', got %d",
					tt.expectedCount, tt.name, len(deltas))
			}

			// Verify all deltas belong to the correct flow
			for _, delta := range deltas {
				if delta.FlowID.Compare(tt.flowID) != 0 {
					t.Errorf("Delta belongs to wrong flow: expected %s, got %s",
						tt.flowID.String(), delta.FlowID.String())
				}
			}
		})
	}
}

// TestUpdateRequestNodeDeltas tests updating delta references for a REQUEST node
func TestUpdateRequestNodeDeltas(t *testing.T) {
	repo := testFlowNodeRepository(t)
	ctx := context.Background()
	nodeID := idwrap.NewTest(t, "request_node")
	flowID := idwrap.NewTest(t, "test_flow")

	originalDeltaRef := &FlowNodeDeltaReference{
		NodeID:     nodeID,
		EndpointID: idwrap.NewTestPtr(t, "endpoint_1"),
		FlowID:     flowID,
		IsActive:   true,
	}

	updatedDeltaRef := &FlowNodeDeltaReference{
		NodeID:          nodeID,
		EndpointID:      idwrap.NewTestPtr(t, "endpoint_2"),
		DeltaEndpointID: idwrap.NewTestPtr(t, "delta_endpoint_2"),
		FlowID:          flowID,
		IsActive:        true,
	}

	// Set up initial delta reference
	err := repo.HandleRequestNode(ctx, nil, nodeID, originalDeltaRef)
	if err != nil {
		t.Fatalf("Failed to set up initial delta reference: %v", err)
	}

	tests := []struct {
		name        string
		nodeID      idwrap.IDWrap
		deltaRef    *FlowNodeDeltaReference
		expectError bool
		expectNil   bool
	}{
		{
			name:        "Update existing delta reference",
			nodeID:      nodeID,
			deltaRef:    updatedDeltaRef,
			expectError: false,
		},
		{
			name:        "Remove delta reference",
			nodeID:      nodeID,
			deltaRef:    nil,
			expectError: false,
		},
		{
			name:        "Empty node ID",
			nodeID:      idwrap.IDWrap{},
			deltaRef:    updatedDeltaRef,
			expectError: false, // Node ID will be set from deltaRef
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.UpdateRequestNodeDeltas(ctx, nil, tt.nodeID, tt.deltaRef)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for test '%s', but got none", tt.name)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for test '%s': %v", tt.name, err)
				}

				// Verify the update
				deltas, err := repo.GetRequestNodeDeltas(ctx, flowID)
				if err != nil {
					t.Errorf("Failed to retrieve deltas after update: %v", err)
				} else {
					if tt.deltaRef == nil {
						// Should be removed
						found := false
						for _, delta := range deltas {
							if delta.NodeID.Compare(tt.nodeID) == 0 {
								found = true
								break
							}
						}
						if found {
							t.Error("Delta reference was not removed")
						}
					} else {
						// Should be updated
						found := false
						for _, delta := range deltas {
							if delta.NodeID.Compare(tt.nodeID) == 0 {
								if delta.EndpointID == nil ||
									delta.EndpointID.Compare(*tt.deltaRef.EndpointID) != 0 {
									t.Error("Delta reference was not updated correctly")
								}
								found = true
								break
							}
						}
						if !found {
							t.Error("Updated delta reference not found")
						}
					}
				}
			}
		})
	}
}

// =============================================================================
// FLOW VARIABLE OPERATIONS TESTS
// =============================================================================

// TestGetFlowVariablesOrder tests retrieving variables in sequential order
func TestGetFlowVariablesOrder(t *testing.T) {
	repo := testFlowNodeRepository(t)
	ctx := context.Background()
	flowID := idwrap.NewTest(t, "test_flow")

	tests := []struct {
		name        string
		flowID      idwrap.IDWrap
		expectError bool
	}{
		{
			name:        "Valid flow ID",
			flowID:      flowID,
			expectError: false,
		},
		{
			name:        "Empty flow ID",
			flowID:      idwrap.IDWrap{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			variables, err := repo.GetFlowVariablesOrder(ctx, tt.flowID)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for test '%s', but got none", tt.name)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for test '%s': %v", tt.name, err)
				}

				if variables == nil {
					t.Errorf("Expected non-nil variables slice for test '%s'", tt.name)
				}

				// Verify sequential ordering
				for i := 1; i < len(variables); i++ {
					if variables[i-1].Sequential >= variables[i].Sequential {
						t.Error("Variables are not in sequential order")
						break
					}
				}
			}
		})
	}
}

// TestReorderFlowVariable tests moving a flow variable in sequential ordering
func TestReorderFlowVariable(t *testing.T) {
	repo := testFlowNodeRepository(t)
	ctx := context.Background()
	variableID := idwrap.NewTest(t, "test_variable")
	flowContext := createTestFlowContext(t)

	tests := []struct {
		name        string
		variableID  idwrap.IDWrap
		newPosition int
		context     *FlowBoundaryContext
		expectError bool
	}{
		{
			name:        "Valid reorder",
			variableID:  variableID,
			newPosition: 5,
			context:     flowContext,
			expectError: false,
		},
		{
			name:        "Negative position",
			variableID:  variableID,
			newPosition: -1,
			context:     flowContext,
			expectError: true,
		},
		{
			name:        "Empty variable ID",
			variableID:  idwrap.IDWrap{},
			newPosition: 5,
			context:     flowContext,
			expectError: true,
		},
		{
			name:        "Nil context",
			variableID:  variableID,
			newPosition: 5,
			context:     nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.ReorderFlowVariable(ctx, nil, tt.variableID, tt.newPosition, tt.context)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for test '%s', but got none", tt.name)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for test '%s': %v", tt.name, err)
				}
			}
		})
	}
}

// =============================================================================
// BATCH OPERATIONS TESTS
// =============================================================================

// TestBatchCreateFlowNodes tests efficiently creating multiple flow nodes
func TestBatchCreateFlowNodes(t *testing.T) {
	repo := testFlowNodeRepository(t)
	ctx := context.Background()
	flowID := idwrap.NewTest(t, "test_flow")

	validNodes := []FlowNodeItem{
		createTestFlowNodeItem(t, "node_1", flowID, FlowNodePosition{X: 100, Y: 100}, FlowNodeOrderSpatial),
		createTestFlowNodeItem(t, "node_2", flowID, FlowNodePosition{X: 200, Y: 200}, FlowNodeOrderSpatial),
	}

	// Create oversized batch
	oversizedNodes := make([]FlowNodeItem, repo.flowConfig.BatchSize+1)
	for i := range oversizedNodes {
		oversizedNodes[i] = createTestFlowNodeItem(t, "node_"+string(rune(i)), flowID,
			FlowNodePosition{X: 100, Y: 100}, FlowNodeOrderSpatial)
	}

	tests := []struct {
		name        string
		nodes       []FlowNodeItem
		expectError bool
	}{
		{
			name:        "Valid batch create",
			nodes:       validNodes,
			expectError: false,
		},
		{
			name:        "Empty nodes",
			nodes:       []FlowNodeItem{},
			expectError: false, // Should be no-op
		},
		{
			name:        "Oversized batch",
			nodes:       oversizedNodes,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.BatchCreateFlowNodes(ctx, nil, tt.nodes)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for test '%s', but got none", tt.name)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for test '%s': %v", tt.name, err)
				}
			}
		})
	}
}

// TestBatchResolveFlowNodes tests efficiently resolving multiple nodes with deltas
func TestBatchResolveFlowNodes(t *testing.T) {
	repo := testFlowNodeRepository(t)
	ctx := context.Background()
	flowContext := createTestFlowContext(t)

	nodeIDs := []idwrap.IDWrap{
		idwrap.NewTest(t, "node_1"),
		idwrap.NewTest(t, "node_2"),
		idwrap.NewTest(t, "node_3"),
	}

	tests := []struct {
		name        string
		nodeIDs     []idwrap.IDWrap
		context     *FlowBoundaryContext
		expectError bool
	}{
		{
			name:        "Valid batch resolve",
			nodeIDs:     nodeIDs,
			context:     flowContext,
			expectError: false,
		},
		{
			name:        "Empty node IDs",
			nodeIDs:     []idwrap.IDWrap{},
			context:     flowContext,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := repo.BatchResolveFlowNodes(ctx, tt.nodeIDs, tt.context)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for test '%s', but got none", tt.name)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for test '%s': %v", tt.name, err)
				}

				if results == nil {
					t.Errorf("Expected non-nil results for test '%s'", tt.name)
				}

				if len(results) != len(tt.nodeIDs) {
					t.Errorf("Expected %d results for test '%s', got %d",
						len(tt.nodeIDs), tt.name, len(results))
				}
			}
		})
	}
}

// TestCompactFlowPositions tests rebalancing all positions within a flow
func TestCompactFlowPositions(t *testing.T) {
	repo := testFlowNodeRepository(t)
	ctx := context.Background()
	flowID := idwrap.NewTest(t, "test_flow")

	tests := []struct {
		name        string
		flowID      idwrap.IDWrap
		expectError bool
	}{
		{
			name:        "Valid flow compaction",
			flowID:      flowID,
			expectError: false,
		},
		{
			name:        "Empty flow ID",
			flowID:      idwrap.IDWrap{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.CompactFlowPositions(ctx, nil, tt.flowID)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for test '%s', but got none", tt.name)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for test '%s': %v", tt.name, err)
				}
			}
		})
	}
}

// =============================================================================
// HELPER FUNCTION TESTS
// =============================================================================

// TestSpatialPositionEncodingDecoding tests encoding and decoding spatial positions
func TestSpatialPositionEncodingDecoding(t *testing.T) {
	repo := testFlowNodeRepository(t)

	testPositions := []FlowNodePosition{
		{X: 0, Y: 0},
		{X: 100, Y: 200},
		{X: 1000, Y: 5000},
		{X: 50.5, Y: 75.5},
	}

	for i, pos := range testPositions {
		t.Run(fmt.Sprintf("Position %d", i), func(t *testing.T) {
			encoded := repo.encodeSpatialPosition(pos)
			decoded := repo.decodeSpatialPosition(encoded)

			// Allow for small floating-point precision differences
			if abs(decoded.X-pos.X) > 1.0 || abs(decoded.Y-pos.Y) > 1.0 {
				t.Errorf("Position encoding/decoding failed: original %v, decoded %v",
					pos, decoded)
			}
		})
	}
}

// abs returns the absolute value of a float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// TestValidateSpatialPosition tests spatial coordinate validation
func TestValidateSpatialPosition(t *testing.T) {
	repo := testFlowNodeRepository(t)

	tests := []struct {
		name        string
		position    FlowNodePosition
		expectError bool
	}{
		{
			name:        "Valid position",
			position:    FlowNodePosition{X: 100, Y: 200},
			expectError: false,
		},
		{
			name:        "Zero position",
			position:    FlowNodePosition{X: 0, Y: 0},
			expectError: false,
		},
		{
			name:        "Negative X coordinate",
			position:    FlowNodePosition{X: -100, Y: 200},
			expectError: true,
		},
		{
			name:        "Negative Y coordinate",
			position:    FlowNodePosition{X: 100, Y: -200},
			expectError: true,
		},
		{
			name:        "Exceeds maximum X",
			position:    FlowNodePosition{X: 20000, Y: 200},
			expectError: true,
		},
		{
			name:        "Exceeds maximum Y",
			position:    FlowNodePosition{X: 100, Y: 20000},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.validateSpatialPosition(tt.position)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for test '%s', but got none", tt.name)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for test '%s': %v", tt.name, err)
				}
			}
		})
	}
}
