package movable

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
	"the-dev-tools/server/pkg/idwrap"
)

// =============================================================================
// EXAMPLE-SPECIFIC ENHANCED REPOSITORY
// =============================================================================

// EnhancedExampleRepository provides delta-aware operations for examples
// Examples are scoped by endpoint context and support hidden delta examples
type EnhancedExampleRepository struct {
	// Embed the enhanced repository for core functionality
	*SimpleEnhancedRepository
	
	// Example-specific configuration
	exampleConfig  *ExampleRepositoryConfig
	deltaManager   ExampleDeltaManager
	
	// Example relationship tracking
	exampleDeltas     map[string]*ExampleDeltaRelation
	deltasMutex       sync.RWMutex
	
	// Context tracking for endpoint scope
	endpointContexts  map[string]*ExampleEndpointContext
	contextMutex      sync.RWMutex
}

// ExampleRepositoryConfig defines example-specific configuration
type ExampleRepositoryConfig struct {
	*EnhancedRepositoryConfig
	
	// Example-specific settings
	EndpointScopeResolver EndpointScopeResolver
	DeltaManager          ExampleDeltaManager
	HiddenDeltaSupport    bool
	InheritancePriority   int
	SyncOnOriginChange    bool
	
	// Performance settings
	MaxExamplesPerEndpoint int
	DeltaCacheSize         int
	SyncBatchSize          int
}

// ExampleDeltaRelation extends DeltaRelation with example-specific metadata
type ExampleDeltaRelation struct {
	*DeltaRelation
	
	// Example-specific fields
	EndpointID     idwrap.IDWrap
	IsHidden       bool
	InheritsFrom   *idwrap.IDWrap
	OverrideLevel  ExampleOverrideLevel
	LastSyncTime   time.Time
}

// ExampleOverrideLevel defines the extent of example overrides
type ExampleOverrideLevel int

const (
	OverrideLevelNone ExampleOverrideLevel = iota
	OverrideLevelHeaders
	OverrideLevelBody
	OverrideLevelComplete
)

// ExampleEndpointContext provides endpoint-specific context information for examples
type ExampleEndpointContext struct {
	EndpointID    idwrap.IDWrap
	CollectionID  idwrap.IDWrap
	ExampleCount  int
	DeltaCount    int
	LastUpdated   time.Time
}

// EndpointScopeResolver determines endpoint context for examples
type EndpointScopeResolver interface {
	// ResolveEndpointID determines the endpoint for a given example
	ResolveEndpointID(ctx context.Context, exampleID idwrap.IDWrap) (idwrap.IDWrap, error)
	
	// ValidateEndpointScope ensures example belongs to expected endpoint
	ValidateEndpointScope(ctx context.Context, exampleID, endpointID idwrap.IDWrap) error
	
	// GetEndpointExamples returns all examples for an endpoint
	GetEndpointExamples(ctx context.Context, endpointID idwrap.IDWrap) ([]idwrap.IDWrap, error)
}

// ExampleDeltaManager handles delta relationships and inheritance for examples
type ExampleDeltaManager interface {
	// CreateDeltaRelation creates a new delta relationship
	CreateDeltaRelation(ctx context.Context, relation *ExampleDeltaRelation) error
	
	// ResolveDeltaInheritance resolves example with delta inheritance
	ResolveDeltaInheritance(ctx context.Context, originID idwrap.IDWrap, context *ContextMetadata) (*ResolvedExample, error)
	
	// SyncDeltaWithOrigin syncs delta when origin changes
	SyncDeltaWithOrigin(ctx context.Context, originID idwrap.IDWrap) error
	
	// PruneStaleDeltaExamples removes obsolete deltas
	PruneStaleDeltaExamples(ctx context.Context, endpointID idwrap.IDWrap) error
}

// ResolvedExample represents an example after delta resolution
type ResolvedExample struct {
	*ResolvedItem
	
	// Example-specific resolved data
	EndpointID      idwrap.IDWrap
	EffectiveExample interface{} // The resolved example entity
	InheritedFields  []string   // Fields inherited from origin
	OverriddenFields []string   // Fields overridden by delta
	IsHidden        bool
}

// =============================================================================
// CONSTRUCTOR AND FACTORY METHODS
// =============================================================================

// NewEnhancedExampleRepository creates a new enhanced example repository
func NewEnhancedExampleRepository(db *sql.DB, config *ExampleRepositoryConfig) *EnhancedExampleRepository {
	// Create base enhanced repository
	baseRepo := NewSimpleEnhancedRepository(db, config.EnhancedRepositoryConfig)
	
	// Create delta manager if not provided
	deltaManager := config.DeltaManager
	if deltaManager == nil {
		deltaManager = NewDefaultExampleDeltaManager()
	}
	
	return &EnhancedExampleRepository{
		SimpleEnhancedRepository: baseRepo,
		exampleConfig:           config,
		deltaManager:            deltaManager,
		exampleDeltas:          make(map[string]*ExampleDeltaRelation),
		endpointContexts:       make(map[string]*ExampleEndpointContext),
	}
}

// DefaultExampleRepositoryConfig returns sensible defaults for example repository
func DefaultExampleRepositoryConfig() *ExampleRepositoryConfig {
	return &ExampleRepositoryConfig{
		EnhancedRepositoryConfig: &EnhancedRepositoryConfig{
			MoveConfig: &MoveConfig{
				ParentScope: ParentScopeConfig{
					Pattern: DirectFKPattern,
				},
			},
			EnableContextAware: true,
			EnableDeltaSupport: true,
			EnableScopeValidation: true,
			BatchSize: 50,
			CacheTTL: 5 * time.Minute,
		},
		HiddenDeltaSupport:     true,
		InheritancePriority:    100,
		SyncOnOriginChange:     true,
		MaxExamplesPerEndpoint: 1000,
		DeltaCacheSize:         500,
		SyncBatchSize:          25,
	}
}

// =============================================================================
// DELTA-SPECIFIC OPERATIONS FOR EXAMPLES
// =============================================================================

// CreateDeltaExample creates a hidden delta example with proper inheritance
func (r *EnhancedExampleRepository) CreateDeltaExample(ctx context.Context, tx *sql.Tx,
	originID, deltaID, endpointID idwrap.IDWrap, overrideLevel ExampleOverrideLevel) error {
	
	startTime := time.Now()
	defer r.trackPerformance("CreateDeltaExample", startTime)
	
	// Validate endpoint scope
	if r.exampleConfig.EndpointScopeResolver != nil {
		if err := r.exampleConfig.EndpointScopeResolver.ValidateEndpointScope(ctx, deltaID, endpointID); err != nil {
			return fmt.Errorf("endpoint scope validation failed: %w", err)
		}
	}
	
	// Create example-specific delta relation
	relation := &ExampleDeltaRelation{
		DeltaRelation: &DeltaRelation{
			DeltaID:      deltaID,
			OriginID:     originID,
			Context:      ContextEndpoint,
			ScopeID:      endpointID,
			RelationType: DeltaRelationOverride,
			Priority:     r.exampleConfig.InheritancePriority,
			IsActive:     true,
		},
		EndpointID:     endpointID,
		IsHidden:       true, // Delta examples are hidden by default
		OverrideLevel:  overrideLevel,
		LastSyncTime:   time.Now(),
	}
	
	// Store delta relationship
	r.deltasMutex.Lock()
	r.exampleDeltas[deltaID.String()] = relation
	r.deltasMutex.Unlock()
	
	// Create base delta relationship in parent repository
	if err := r.CreateDelta(ctx, tx, originID, deltaID, relation.DeltaRelation); err != nil {
		return fmt.Errorf("failed to create base delta: %w", err)
	}
	
	// Update endpoint context
	if err := r.updateEndpointContext(endpointID, 0, 1); err != nil {
		return fmt.Errorf("failed to update endpoint context: %w", err)
	}
	
	// Use delta manager if available
	if r.deltaManager != nil {
		if err := r.deltaManager.CreateDeltaRelation(ctx, relation); err != nil {
			return fmt.Errorf("delta manager create failed: %w", err)
		}
	}
	
	return nil
}

// ResolveDeltaExample resolves an example with delta inheritance
func (r *EnhancedExampleRepository) ResolveDeltaExample(ctx context.Context, originID idwrap.IDWrap, 
	contextMeta *ContextMetadata) (*ResolvedExample, error) {
	
	startTime := time.Now()
	defer r.trackPerformance("ResolveDeltaExample", startTime)
	
	// Use delta manager if available
	if r.deltaManager != nil {
		return r.deltaManager.ResolveDeltaInheritance(ctx, originID, contextMeta)
	}
	
	// Fallback to base resolution
	resolved, err := r.ResolveDelta(ctx, originID, contextMeta)
	if err != nil {
		return nil, fmt.Errorf("base delta resolution failed: %w", err)
	}
	
	// Find example-specific delta
	exampleDelta := r.getExampleDeltaRelation(resolved.EffectiveID)
	
	// Create resolved example
	resolvedExample := &ResolvedExample{
		ResolvedItem:     resolved,
		EndpointID:       contextMeta.GetEndpointID(),
		InheritedFields:  []string{},
		OverriddenFields: []string{},
		IsHidden:        exampleDelta != nil && exampleDelta.IsHidden,
	}
	
	// Determine inherited vs overridden fields based on override level
	if exampleDelta != nil {
		switch exampleDelta.OverrideLevel {
		case OverrideLevelHeaders:
			resolvedExample.OverriddenFields = []string{"headers"}
			resolvedExample.InheritedFields = []string{"body", "url", "method"}
		case OverrideLevelBody:
			resolvedExample.OverriddenFields = []string{"body"}
			resolvedExample.InheritedFields = []string{"headers", "url", "method"}
		case OverrideLevelComplete:
			resolvedExample.OverriddenFields = []string{"headers", "body", "url", "method"}
			resolvedExample.InheritedFields = []string{}
		default:
			resolvedExample.InheritedFields = []string{"headers", "body", "url", "method"}
		}
	}
	
	return resolvedExample, nil
}

// SyncDeltaExamples synchronizes deltas when origin examples change
func (r *EnhancedExampleRepository) SyncDeltaExamples(ctx context.Context, tx *sql.Tx, 
	originID idwrap.IDWrap) error {
	
	startTime := time.Now()
	defer r.trackPerformance("SyncDeltaExamples", startTime)
	
	if !r.exampleConfig.SyncOnOriginChange {
		return nil // Sync disabled
	}
	
	// Use delta manager if available
	if r.deltaManager != nil {
		return r.deltaManager.SyncDeltaWithOrigin(ctx, originID)
	}
	
	// Fallback implementation
	// Find all deltas for this origin
	deltas, err := r.GetDeltasByOrigin(ctx, originID)
	if err != nil {
		return fmt.Errorf("failed to get deltas for origin %s: %w", originID.String(), err)
	}
	
	// Process each delta for synchronization
	for _, delta := range deltas {
		exampleDelta := r.getExampleDeltaRelation(delta.DeltaID)
		if exampleDelta == nil {
			continue
		}
		
		// Update last sync time
		exampleDelta.LastSyncTime = time.Now()
		
		// Additional sync logic would go here based on override level
		// For now, we just update the timestamp
	}
	
	return nil
}

// PruneStaleDeltaExamples removes obsolete delta examples
func (r *EnhancedExampleRepository) PruneStaleDeltaExamples(ctx context.Context, tx *sql.Tx,
	endpointID idwrap.IDWrap) error {
	
	startTime := time.Now()
	defer r.trackPerformance("PruneStaleDeltaExamples", startTime)
	
	// Use delta manager if available
	if r.deltaManager != nil {
		return r.deltaManager.PruneStaleDeltaExamples(ctx, endpointID)
	}
	
	// Get all deltas in the endpoint scope
	deltas, err := r.GetDeltasByScope(ctx, endpointID, ContextEndpoint)
	if err != nil {
		return fmt.Errorf("failed to get deltas for endpoint %s: %w", endpointID.String(), err)
	}
	
	prunedCount := 0
	staleThreshold := time.Now().Add(-24 * time.Hour) // 24 hours ago
	
	r.deltasMutex.Lock()
	for _, delta := range deltas {
		exampleDelta := r.getExampleDeltaRelationUnsafe(delta.DeltaID)
		if exampleDelta != nil && exampleDelta.LastSyncTime.Before(staleThreshold) {
			// Mark as inactive rather than deleting
			exampleDelta.IsActive = false
			delete(r.exampleDeltas, delta.DeltaID.String())
			prunedCount++
		}
	}
	r.deltasMutex.Unlock()
	
	// Update endpoint context
	if prunedCount > 0 {
		if err := r.updateEndpointContext(endpointID, 0, -prunedCount); err != nil {
			return fmt.Errorf("failed to update endpoint context: %w", err)
		}
	}
	
	return nil
}

// =============================================================================
// CONTEXT HANDLING FOR ENDPOINT SCOPE
// =============================================================================

// GetExamplesByEndpoint returns all examples for an endpoint with delta resolution
func (r *EnhancedExampleRepository) GetExamplesByEndpoint(ctx context.Context, endpointID idwrap.IDWrap,
	includeHidden bool) ([]ResolvedExample, error) {
	
	startTime := time.Now()
	defer r.trackPerformance("GetExamplesByEndpoint", startTime)
	
	// Create context metadata for endpoint scope
	contextMeta := &ContextMetadata{
		Type:    ContextEndpoint,
		ScopeID: endpointID.Bytes(),
	}
	
	// Get base items using context
	contextualItems, err := r.GetItemsByParentWithContext(ctx, endpointID, 
		CollectionListTypeExamples, contextMeta)
	if err != nil {
		return nil, fmt.Errorf("failed to get contextual items: %w", err)
	}
	
	resolved := make([]ResolvedExample, 0, len(contextualItems))
	
	for _, item := range contextualItems {
		// Skip hidden items unless explicitly requested
		if item.Context != nil && item.Context.IsHidden && !includeHidden {
			continue
		}
		
		// Resolve each example with deltas
		resolvedExample, err := r.ResolveDeltaExample(ctx, item.ID, contextMeta)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve example %s: %w", item.ID.String(), err)
		}
		
		resolved = append(resolved, *resolvedExample)
	}
	
	return resolved, nil
}

// UpdateExamplePositionInEndpoint updates example position within endpoint scope
func (r *EnhancedExampleRepository) UpdateExamplePositionInEndpoint(ctx context.Context, tx *sql.Tx,
	exampleID idwrap.IDWrap, endpointID idwrap.IDWrap, position int) error {
	
	startTime := time.Now()
	defer r.trackPerformance("UpdateExamplePositionInEndpoint", startTime)
	
	// Create context metadata
	contextMeta := &ContextMetadata{
		Type:    ContextEndpoint,
		ScopeID: endpointID.Bytes(),
	}
	
	// Check if this is a delta example
	exampleDelta := r.getExampleDeltaRelation(exampleID)
	if exampleDelta != nil {
		contextMeta.IsHidden = exampleDelta.IsHidden
		originIDBytes := exampleDelta.OriginID.Bytes()
		contextMeta.OriginID = &originIDBytes
	}
	
	// Use context-aware position update
	return r.UpdatePositionWithContext(ctx, tx, exampleID, CollectionListTypeExamples, position, contextMeta)
}

// GetEndpointContext returns context information for an endpoint
func (r *EnhancedExampleRepository) GetEndpointContext(endpointID idwrap.IDWrap) (*ExampleEndpointContext, error) {
	r.contextMutex.RLock()
	defer r.contextMutex.RUnlock()
	
	context, exists := r.endpointContexts[endpointID.String()]
	if !exists {
		// Create default context
		return &ExampleEndpointContext{
			EndpointID:   endpointID,
			ExampleCount: 0,
			DeltaCount:   0,
			LastUpdated:  time.Now(),
		}, nil
	}
	
	return context, nil
}

// =============================================================================
// BATCH OPERATIONS FOR PERFORMANCE
// =============================================================================

// BatchCreateDeltaExamples efficiently creates multiple delta examples
func (r *EnhancedExampleRepository) BatchCreateDeltaExamples(ctx context.Context, tx *sql.Tx,
	operations []DeltaExampleOperation) error {
	
	startTime := time.Now()
	defer r.trackPerformance("BatchCreateDeltaExamples", startTime)
	
	if len(operations) == 0 {
		return nil
	}
	
	// Validate batch size
	batchSize := r.exampleConfig.SyncBatchSize
	if batchSize == 0 {
		batchSize = 25
	}
	
	if len(operations) > batchSize {
		return fmt.Errorf("batch size %d exceeds maximum %d", len(operations), batchSize)
	}
	
	// Process each operation
	for _, op := range operations {
		if err := r.CreateDeltaExample(ctx, tx, op.OriginID, op.DeltaID, 
			op.EndpointID, op.OverrideLevel); err != nil {
			return fmt.Errorf("failed to create delta example %s: %w", op.DeltaID.String(), err)
		}
	}
	
	return nil
}

// BatchResolveExamples efficiently resolves multiple examples
func (r *EnhancedExampleRepository) BatchResolveExamples(ctx context.Context, originIDs []idwrap.IDWrap,
	endpointID idwrap.IDWrap) (map[idwrap.IDWrap]*ResolvedExample, error) {
	
	startTime := time.Now()
	defer r.trackPerformance("BatchResolveExamples", startTime)
	
	contextMeta := &ContextMetadata{
		Type:    ContextEndpoint,
		ScopeID: endpointID.Bytes(),
	}
	
	results := make(map[idwrap.IDWrap]*ResolvedExample, len(originIDs))
	
	for _, originID := range originIDs {
		resolved, err := r.ResolveDeltaExample(ctx, originID, contextMeta)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve example %s: %w", originID.String(), err)
		}
		results[originID] = resolved
	}
	
	return results, nil
}

// DeltaExampleOperation represents a delta example creation operation
type DeltaExampleOperation struct {
	OriginID      idwrap.IDWrap
	DeltaID       idwrap.IDWrap
	EndpointID    idwrap.IDWrap
	OverrideLevel ExampleOverrideLevel
}

// =============================================================================
// HELPER METHODS AND UTILITIES
// =============================================================================

// getExampleDeltaRelation retrieves example-specific delta relation (thread-safe)
func (r *EnhancedExampleRepository) getExampleDeltaRelation(deltaID idwrap.IDWrap) *ExampleDeltaRelation {
	r.deltasMutex.RLock()
	defer r.deltasMutex.RUnlock()
	
	return r.getExampleDeltaRelationUnsafe(deltaID)
}

// getExampleDeltaRelationUnsafe retrieves example-specific delta relation (not thread-safe)
func (r *EnhancedExampleRepository) getExampleDeltaRelationUnsafe(deltaID idwrap.IDWrap) *ExampleDeltaRelation {
	relation, exists := r.exampleDeltas[deltaID.String()]
	if exists && relation.IsActive {
		return relation
	}
	return nil
}

// updateEndpointContext updates endpoint context counters
func (r *EnhancedExampleRepository) updateEndpointContext(endpointID idwrap.IDWrap, exampleDelta, deltaDelta int) error {
	r.contextMutex.Lock()
	defer r.contextMutex.Unlock()
	
	contextKey := endpointID.String()
	context, exists := r.endpointContexts[contextKey]
	
	if !exists {
		context = &ExampleEndpointContext{
			EndpointID:   endpointID,
			ExampleCount: 0,
			DeltaCount:   0,
			LastUpdated:  time.Now(),
		}
		r.endpointContexts[contextKey] = context
	}
	
	context.ExampleCount += exampleDelta
	context.DeltaCount += deltaDelta
	context.LastUpdated = time.Now()
	
	// Validate counts don't go negative
	if context.ExampleCount < 0 {
		context.ExampleCount = 0
	}
	if context.DeltaCount < 0 {
		context.DeltaCount = 0
	}
	
	return nil
}

// =============================================================================
// CONTEXT METADATA EXTENSIONS
// =============================================================================

// GetEndpointID extracts endpoint ID from context metadata
func (c *ContextMetadata) GetEndpointID() idwrap.IDWrap {
	if c == nil || c.Type != ContextEndpoint {
		return idwrap.IDWrap{}
	}
	
	endpointID, err := idwrap.NewFromBytes(c.ScopeID)
	if err != nil {
		return idwrap.IDWrap{}
	}
	
	return endpointID
}

// =============================================================================
// DEFAULT IMPLEMENTATIONS
// =============================================================================

// DefaultExampleDeltaManager provides a simple implementation of ExampleDeltaManager
type DefaultExampleDeltaManager struct {
	// Placeholder implementation
}

// NewDefaultExampleDeltaManager creates a default delta manager
func NewDefaultExampleDeltaManager() ExampleDeltaManager {
	return &DefaultExampleDeltaManager{}
}

// CreateDeltaRelation implements ExampleDeltaManager interface
func (m *DefaultExampleDeltaManager) CreateDeltaRelation(ctx context.Context, relation *ExampleDeltaRelation) error {
	// Default implementation is a no-op
	return nil
}

// ResolveDeltaInheritance implements ExampleDeltaManager interface
func (m *DefaultExampleDeltaManager) ResolveDeltaInheritance(ctx context.Context, originID idwrap.IDWrap, 
	context *ContextMetadata) (*ResolvedExample, error) {
	
	// Default implementation returns unresolved
	return &ResolvedExample{
		ResolvedItem: &ResolvedItem{
			ID:             originID,
			EffectiveID:    originID,
			OriginID:       &originID,
			ResolutionType: ResolutionOrigin,
			ResolutionTime: time.Now(),
		},
		EndpointID: context.GetEndpointID(),
	}, nil
}

// SyncDeltaWithOrigin implements ExampleDeltaManager interface
func (m *DefaultExampleDeltaManager) SyncDeltaWithOrigin(ctx context.Context, originID idwrap.IDWrap) error {
	// Default implementation is a no-op
	return nil
}

// PruneStaleDeltaExamples implements ExampleDeltaManager interface
func (m *DefaultExampleDeltaManager) PruneStaleDeltaExamples(ctx context.Context, endpointID idwrap.IDWrap) error {
	// Default implementation is a no-op
	return nil
}

