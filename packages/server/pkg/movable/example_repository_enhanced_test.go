package movable

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"
	
	"the-dev-tools/server/pkg/idwrap"
)

// =============================================================================
// TEST FIXTURES AND HELPERS
// =============================================================================

// mockExampleRepository creates a mock enhanced example repository for testing
func mockExampleRepository() *EnhancedExampleRepository {
	config := DefaultExampleRepositoryConfig()
	
	// Create a simple enhanced repository with mock database
	baseRepo := &SimpleEnhancedRepository{
		SimpleRepository: &SimpleRepository{
			db: nil, // Mock database
		},
		config:             config.EnhancedRepositoryConfig,
		deltaRelations:     make(map[string]*DeltaRelation),
		performanceMetrics: &PerformanceMetrics{},
	}
	
	return &EnhancedExampleRepository{
		SimpleEnhancedRepository: baseRepo,
		exampleConfig:           config,
		deltaManager:            NewDefaultExampleDeltaManager(),
		exampleDeltas:          make(map[string]*ExampleDeltaRelation),
		endpointContexts:       make(map[string]*ExampleEndpointContext),
	}
}

// createExampleTestIDs creates test IDs for use in example tests
func createExampleTestIDs() (origin, delta, endpoint idwrap.IDWrap) {
	origin = idwrap.NewTextMust("origin-example-123")
	delta = idwrap.NewTextMust("delta-example-456") 
	endpoint = idwrap.NewTextMust("endpoint-789")
	return
}

// =============================================================================
// CONSTRUCTOR TESTS
// =============================================================================

func TestNewEnhancedExampleRepository(t *testing.T) {
	config := DefaultExampleRepositoryConfig()
	repo := NewEnhancedExampleRepository(nil, config)
	
	if repo == nil {
		t.Fatal("NewEnhancedExampleRepository returned nil")
	}
	
	if repo.exampleConfig == nil {
		t.Error("Example config not set")
	}
	
	if repo.deltaManager == nil {
		t.Error("Delta manager not set")
	}
	
	if repo.exampleDeltas == nil {
		t.Error("Example deltas map not initialized")
	}
	
	if repo.endpointContexts == nil {
		t.Error("Endpoint contexts map not initialized")
	}
}

func TestDefaultExampleRepositoryConfig(t *testing.T) {
	config := DefaultExampleRepositoryConfig()
	
	if config == nil {
		t.Fatal("DefaultExampleRepositoryConfig returned nil")
	}
	
	if config.EnhancedRepositoryConfig == nil {
		t.Error("Enhanced repository config not set")
	}
	
	if !config.HiddenDeltaSupport {
		t.Error("Hidden delta support should be enabled by default")
	}
	
	if config.InheritancePriority != 100 {
		t.Errorf("Expected inheritance priority 100, got %d", config.InheritancePriority)
	}
	
	if !config.SyncOnOriginChange {
		t.Error("Sync on origin change should be enabled by default")
	}
	
	if config.MaxExamplesPerEndpoint != 1000 {
		t.Errorf("Expected max examples per endpoint 1000, got %d", config.MaxExamplesPerEndpoint)
	}
}

// =============================================================================
// DELTA CREATION TESTS
// =============================================================================

func TestCreateDeltaExample(t *testing.T) {
	repo := mockExampleRepository()
	ctx := context.Background()
	originID, deltaID, endpointID := createExampleTestIDs()
	
	// Mock the base repository methods to avoid database calls
	repo.SimpleRepository = &MockSimpleRepository{}
	
	err := repo.CreateDeltaExample(ctx, nil, originID, deltaID, endpointID, OverrideLevelHeaders)
	if err != nil {
		t.Fatalf("CreateDeltaExample failed: %v", err)
	}
	
	// Verify delta relation was created
	relation := repo.getExampleDeltaRelation(deltaID)
	if relation == nil {
		t.Error("Delta relation not found after creation")
	} else {
		if relation.EndpointID.Compare(endpointID) != 0 {
			t.Error("Endpoint ID mismatch in delta relation")
		}
		
		if !relation.IsHidden {
			t.Error("Delta example should be hidden by default")
		}
		
		if relation.OverrideLevel != OverrideLevelHeaders {
			t.Errorf("Expected override level %v, got %v", OverrideLevelHeaders, relation.OverrideLevel)
		}
	}
	
	// Verify endpoint context was updated
	context, err := repo.GetEndpointContext(endpointID)
	if err != nil {
		t.Errorf("Failed to get endpoint context: %v", err)
	} else if context.DeltaCount != 1 {
		t.Errorf("Expected delta count 1, got %d", context.DeltaCount)
	}
}

func TestCreateDeltaExampleInvalidEndpoint(t *testing.T) {
	repo := mockExampleRepository()
	ctx := context.Background()
	originID, deltaID, _ := createExampleTestIDs()
	emptyEndpointID := idwrap.IDWrap{}
	
	err := repo.CreateDeltaExample(ctx, nil, originID, deltaID, emptyEndpointID, OverrideLevelHeaders)
	if err == nil {
		t.Error("Expected error for invalid endpoint ID")
	}
}

// =============================================================================
// DELTA RESOLUTION TESTS
// =============================================================================

func TestResolveDeltaExample(t *testing.T) {
	repo := mockExampleRepository()
	ctx := context.Background()
	originID, deltaID, endpointID := createExampleTestIDs()
	
	// Create a delta example first
	repo.SimpleRepository = &MockSimpleRepository{}
	err := repo.CreateDeltaExample(ctx, nil, originID, deltaID, endpointID, OverrideLevelHeaders)
	if err != nil {
		t.Fatalf("Failed to create delta example: %v", err)
	}
	
	// Create context metadata
	contextMeta := &ContextMetadata{
		Type:    ContextEndpoint,
		ScopeID: endpointID.Bytes(),
	}
	
	// Resolve the delta example
	resolved, err := repo.ResolveDeltaExample(ctx, originID, contextMeta)
	if err != nil {
		t.Fatalf("ResolveDeltaExample failed: %v", err)
	}
	
	if resolved == nil {
		t.Fatal("Resolved example is nil")
	}
	
	if resolved.EndpointID.Compare(endpointID) != 0 {
		t.Error("Endpoint ID mismatch in resolved example")
	}
	
	if !resolved.IsHidden {
		t.Error("Resolved delta example should be hidden")
	}
	
	// Verify field inheritance based on override level
	expectedOverridden := []string{"headers"}
	expectedInherited := []string{"body", "url", "method"}
	
	if !slicesEqual(resolved.OverriddenFields, expectedOverridden) {
		t.Errorf("Expected overridden fields %v, got %v", expectedOverridden, resolved.OverriddenFields)
	}
	
	if !slicesEqual(resolved.InheritedFields, expectedInherited) {
		t.Errorf("Expected inherited fields %v, got %v", expectedInherited, resolved.InheritedFields)
	}
}

func TestResolveDeltaExampleOriginOnly(t *testing.T) {
	repo := mockExampleRepository()
	ctx := context.Background()
	originID, _, endpointID := createExampleTestIDs()
	
	contextMeta := &ContextMetadata{
		Type:    ContextEndpoint,
		ScopeID: endpointID.Bytes(),
	}
	
	// Resolve example with no deltas
	resolved, err := repo.ResolveDeltaExample(ctx, originID, contextMeta)
	if err != nil {
		t.Fatalf("ResolveDeltaExample failed: %v", err)
	}
	
	if resolved == nil {
		t.Fatal("Resolved example is nil")
	}
	
	if resolved.IsHidden {
		t.Error("Origin example should not be hidden")
	}
	
	if resolved.ResolutionType != ResolutionOrigin {
		t.Errorf("Expected resolution type %v, got %v", ResolutionOrigin, resolved.ResolutionType)
	}
}

// =============================================================================
// SYNC AND PRUNING TESTS
// =============================================================================

func TestSyncDeltaExamples(t *testing.T) {
	repo := mockExampleRepository()
	ctx := context.Background()
	originID, deltaID, endpointID := createExampleTestIDs()
	
	// Create a delta example first
	repo.SimpleRepository = &MockSimpleRepository{}
	err := repo.CreateDeltaExample(ctx, nil, originID, deltaID, endpointID, OverrideLevelHeaders)
	if err != nil {
		t.Fatalf("Failed to create delta example: %v", err)
	}
	
	// Get initial sync time
	relation := repo.getExampleDeltaRelation(deltaID)
	initialSyncTime := relation.LastSyncTime
	
	// Small delay to ensure time difference
	time.Sleep(1 * time.Millisecond)
	
	// Sync deltas
	err = repo.SyncDeltaExamples(ctx, nil, originID)
	if err != nil {
		t.Fatalf("SyncDeltaExamples failed: %v", err)
	}
	
	// Verify sync time was updated
	relation = repo.getExampleDeltaRelation(deltaID)
	if relation.LastSyncTime.Equal(initialSyncTime) {
		t.Error("Sync time was not updated")
	}
}

func TestPruneStaleDeltaExamples(t *testing.T) {
	repo := mockExampleRepository()
	ctx := context.Background()
	originID, deltaID, endpointID := createExampleTestIDs()
	
	// Create a delta example
	repo.SimpleRepository = &MockSimpleRepository{}
	err := repo.CreateDeltaExample(ctx, nil, originID, deltaID, endpointID, OverrideLevelHeaders)
	if err != nil {
		t.Fatalf("Failed to create delta example: %v", err)
	}
	
	// Make the delta stale by setting old sync time
	relation := repo.getExampleDeltaRelation(deltaID)
	relation.LastSyncTime = time.Now().Add(-48 * time.Hour) // 48 hours ago
	
	// Prune stale deltas
	err = repo.PruneStaleDeltaExamples(ctx, nil, endpointID)
	if err != nil {
		t.Fatalf("PruneStaleDeltaExamples failed: %v", err)
	}
	
	// Verify delta was marked inactive
	relation = repo.getExampleDeltaRelation(deltaID)
	if relation != nil {
		t.Error("Stale delta should have been pruned")
	}
}

// =============================================================================
// CONTEXT HANDLING TESTS
// =============================================================================

func TestGetEndpointContext(t *testing.T) {
	repo := mockExampleRepository()
	endpointID := idwrap.NewTextMust("test-endpoint")
	
	// Get context for non-existent endpoint (should return default)
	context, err := repo.GetEndpointContext(endpointID)
	if err != nil {
		t.Fatalf("GetEndpointContext failed: %v", err)
	}
	
	if context == nil {
		t.Fatal("Endpoint context is nil")
	}
	
	if context.EndpointID.Compare(endpointID) != 0 {
		t.Error("Endpoint ID mismatch in context")
	}
	
	if context.ExampleCount != 0 || context.DeltaCount != 0 {
		t.Error("New endpoint context should have zero counts")
	}
}

func TestUpdateExamplePositionInEndpoint(t *testing.T) {
	repo := mockExampleRepository()
	ctx := context.Background()
	exampleID := idwrap.NewTextMust("example-123")
	endpointID := idwrap.NewTextMust("endpoint-456")
	
	// Mock the base repository to avoid database calls
	repo.SimpleRepository = &MockSimpleRepository{}
	
	err := repo.UpdateExamplePositionInEndpoint(ctx, nil, exampleID, endpointID, 5)
	if err != nil {
		t.Fatalf("UpdateExamplePositionInEndpoint failed: %v", err)
	}
	
	// The test passes if no error occurred since we're mocking the underlying calls
}

// =============================================================================
// BATCH OPERATIONS TESTS
// =============================================================================

func TestBatchCreateDeltaExamples(t *testing.T) {
	repo := mockExampleRepository()
	ctx := context.Background()
	repo.SimpleRepository = &MockSimpleRepository{}
	
	operations := []DeltaExampleOperation{
		{
			OriginID:      idwrap.NewTextMust("origin-1"),
			DeltaID:       idwrap.NewTextMust("delta-1"),
			EndpointID:    idwrap.NewTextMust("endpoint-1"),
			OverrideLevel: OverrideLevelHeaders,
		},
		{
			OriginID:      idwrap.NewTextMust("origin-2"),
			DeltaID:       idwrap.NewTextMust("delta-2"),
			EndpointID:    idwrap.NewTextMust("endpoint-1"),
			OverrideLevel: OverrideLevelBody,
		},
	}
	
	err := repo.BatchCreateDeltaExamples(ctx, nil, operations)
	if err != nil {
		t.Fatalf("BatchCreateDeltaExamples failed: %v", err)
	}
	
	// Verify both deltas were created
	for _, op := range operations {
		relation := repo.getExampleDeltaRelation(op.DeltaID)
		if relation == nil {
			t.Errorf("Delta relation not found for %s", op.DeltaID.String())
		} else if relation.OverrideLevel != op.OverrideLevel {
			t.Errorf("Override level mismatch for %s", op.DeltaID.String())
		}
	}
}

func TestBatchCreateDeltaExamplesExceedsBatchSize(t *testing.T) {
	repo := mockExampleRepository()
	ctx := context.Background()
	
	// Set small batch size
	repo.exampleConfig.SyncBatchSize = 1
	
	operations := []DeltaExampleOperation{
		{
			OriginID:   idwrap.NewTextMust("origin-1"),
			DeltaID:    idwrap.NewTextMust("delta-1"),
			EndpointID: idwrap.NewTextMust("endpoint-1"),
		},
		{
			OriginID:   idwrap.NewTextMust("origin-2"),
			DeltaID:    idwrap.NewTextMust("delta-2"),
			EndpointID: idwrap.NewTextMust("endpoint-1"),
		},
	}
	
	err := repo.BatchCreateDeltaExamples(ctx, nil, operations)
	if err == nil {
		t.Error("Expected error when batch size is exceeded")
	}
}

func TestBatchResolveExamples(t *testing.T) {
	repo := mockExampleRepository()
	ctx := context.Background()
	endpointID := idwrap.NewTextMust("endpoint-1")
	
	originIDs := []idwrap.IDWrap{
		idwrap.NewTextMust("origin-1"),
		idwrap.NewTextMust("origin-2"),
	}
	
	results, err := repo.BatchResolveExamples(ctx, originIDs, endpointID)
	if err != nil {
		t.Fatalf("BatchResolveExamples failed: %v", err)
	}
	
	if len(results) != len(originIDs) {
		t.Errorf("Expected %d results, got %d", len(originIDs), len(results))
	}
	
	// Verify each origin ID has a result
	for _, originID := range originIDs {
		if _, exists := results[originID]; !exists {
			t.Errorf("Missing result for origin ID %s", originID.String())
		}
	}
}

// =============================================================================
// OVERRIDE LEVEL TESTS
// =============================================================================

func TestOverrideLevelFieldMapping(t *testing.T) {
	tests := []struct {
		level                ExampleOverrideLevel
		expectedOverridden   []string
		expectedInherited    []string
	}{
		{
			level:              OverrideLevelNone,
			expectedOverridden: []string{},
			expectedInherited:  []string{"headers", "body", "url", "method"},
		},
		{
			level:              OverrideLevelHeaders,
			expectedOverridden: []string{"headers"},
			expectedInherited:  []string{"body", "url", "method"},
		},
		{
			level:              OverrideLevelBody,
			expectedOverridden: []string{"body"},
			expectedInherited:  []string{"headers", "url", "method"},
		},
		{
			level:              OverrideLevelComplete,
			expectedOverridden: []string{"headers", "body", "url", "method"},
			expectedInherited:  []string{},
		},
	}
	
	repo := mockExampleRepository()
	ctx := context.Background()
	
	for i, test := range tests {
		originID := idwrap.NewTextMust(fmt.Sprintf("origin-%d", i))
		deltaID := idwrap.NewTextMust(fmt.Sprintf("delta-%d", i))
		endpointID := idwrap.NewTextMust("endpoint-test")
		
		// Create delta with specific override level
		repo.SimpleRepository = &MockSimpleRepository{}
		err := repo.CreateDeltaExample(ctx, nil, originID, deltaID, endpointID, test.level)
		if err != nil {
			t.Fatalf("Failed to create delta example: %v", err)
		}
		
		// Resolve and check field mappings
		contextMeta := &ContextMetadata{
			Type:    ContextEndpoint,
			ScopeID: endpointID.Bytes(),
		}
		
		resolved, err := repo.ResolveDeltaExample(ctx, originID, contextMeta)
		if err != nil {
			t.Fatalf("Failed to resolve delta example: %v", err)
		}
		
		if !slicesEqual(resolved.OverriddenFields, test.expectedOverridden) {
			t.Errorf("Level %v: expected overridden fields %v, got %v", 
				test.level, test.expectedOverridden, resolved.OverriddenFields)
		}
		
		if !slicesEqual(resolved.InheritedFields, test.expectedInherited) {
			t.Errorf("Level %v: expected inherited fields %v, got %v", 
				test.level, test.expectedInherited, resolved.InheritedFields)
		}
	}
}

// =============================================================================
// CONTEXT METADATA EXTENSION TESTS
// =============================================================================

func TestContextMetadataGetEndpointID(t *testing.T) {
	endpointID := idwrap.NewTextMust("test-endpoint")
	
	// Test valid endpoint context
	contextMeta := &ContextMetadata{
		Type:    ContextEndpoint,
		ScopeID: endpointID.Bytes(),
	}
	
	extractedID := contextMeta.GetEndpointID()
	if extractedID.Compare(endpointID) != 0 {
		t.Error("GetEndpointID failed to extract correct endpoint ID")
	}
	
	// Test non-endpoint context
	contextMeta.Type = ContextCollection
	extractedID = contextMeta.GetEndpointID()
	emptyID := idwrap.IDWrap{}
	if extractedID.Compare(emptyID) != 0 {
		t.Error("GetEndpointID should return empty ID for non-endpoint context")
	}
	
	// Test nil context
	var nilContext *ContextMetadata = nil
	extractedID = nilContext.GetEndpointID()
	if extractedID.Compare(emptyID) != 0 {
		t.Error("GetEndpointID should return empty ID for nil context")
	}
}

// =============================================================================
// HELPER FUNCTIONS AND MOCKS
// =============================================================================

// MockSimpleRepository provides a mock implementation for testing
type MockSimpleRepository struct {
	*SimpleRepository
}

// UpdatePositionWithContext mocks the base repository method
func (m *MockSimpleRepository) UpdatePositionWithContext(ctx context.Context, tx *sql.Tx,
	itemID idwrap.IDWrap, listType ListType, position int, contextMeta *ContextMetadata) error {
	return nil
}

// CreateDelta mocks the base repository method
func (m *MockSimpleRepository) CreateDelta(ctx context.Context, tx *sql.Tx,
	originID idwrap.IDWrap, deltaID idwrap.IDWrap, relation *DeltaRelation) error {
	return nil
}

// ResolveDelta mocks the base repository method
func (m *MockSimpleRepository) ResolveDelta(ctx context.Context, originID idwrap.IDWrap,
	contextMeta *ContextMetadata) (*ResolvedItem, error) {
	return &ResolvedItem{
		ID:             originID,
		EffectiveID:    originID,
		OriginID:       &originID,
		ResolutionType: ResolutionOrigin,
		ResolutionTime: time.Now(),
	}, nil
}

// GetDeltasByOrigin mocks the base repository method
func (m *MockSimpleRepository) GetDeltasByOrigin(ctx context.Context, originID idwrap.IDWrap) ([]DeltaRelation, error) {
	return []DeltaRelation{}, nil
}

// GetDeltasByScope mocks the base repository method
func (m *MockSimpleRepository) GetDeltasByScope(ctx context.Context, scopeID idwrap.IDWrap,
	contextType MovableContext) ([]DeltaRelation, error) {
	return []DeltaRelation{}, nil
}

// slicesEqual compares two string slices for equality
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

