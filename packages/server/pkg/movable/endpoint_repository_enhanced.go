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
// ENHANCED ENDPOINT REPOSITORY TYPES
// =============================================================================

// EndpointContext defines the different contexts for endpoint operations
type EndpointContext int

const (
	EndpointContextCollection EndpointContext = iota // Collection-based endpoint (standard Postman-style)
	EndpointContextFlow                              // Flow-based endpoint (delta from collection)
	EndpointContextRequest                           // Request node-specific endpoint (flow execution)
)

// String returns the string representation of the endpoint context
func (e EndpointContext) String() string {
	switch e {
	case EndpointContextCollection:
		return "collection"
	case EndpointContextFlow:
		return "flow"
	case EndpointContextRequest:
		return "request"
	default:
		return "unknown"
	}
}

// EndpointMetadata provides context-specific endpoint information
type EndpointMetadata struct {
	Context      EndpointContext
	ScopeID      idwrap.IDWrap // Collection ID, Flow ID, or Request Node ID
	IsHidden     bool          // For flow context: hidden from collection view
	OriginID     *idwrap.IDWrap // Original endpoint ID if this is a delta
	Priority     int           // Resolution priority for conflicts
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ExampleCount int // Cache of example count for performance
}

// EndpointDelta represents a delta relationship specific to endpoints
type EndpointDelta struct {
	DeltaID      idwrap.IDWrap
	OriginID     idwrap.IDWrap
	Context      EndpointContext
	ScopeID      idwrap.IDWrap
	IsActive     bool
	HTTPMethod   *string    // Override HTTP method if specified
	URL          *string    // Override URL if specified
	Description  *string    // Override description if specified
	Priority     int        // Higher priority = higher precedence
	LastSyncAt   *time.Time // Last synchronization with origin
}

// EndpointResolvedItem represents an endpoint after delta resolution
type EndpointResolvedItem struct {
	ID             idwrap.IDWrap
	EffectiveID    idwrap.IDWrap // Delta ID if exists, Origin ID otherwise
	OriginID       *idwrap.IDWrap
	DeltaID        *idwrap.IDWrap
	ParentID       *idwrap.IDWrap
	Position       int
	Context        EndpointContext
	HTTPMethod     string
	URL            string
	Description    string
	AppliedDeltas  []EndpointDelta
	ExampleCount   int
	ResolutionTime time.Time
}

// =============================================================================
// ENHANCED ENDPOINT REPOSITORY IMPLEMENTATION
// =============================================================================

// EnhancedEndpointRepository provides context-aware endpoint operations with delta support
type EnhancedEndpointRepository struct {
	// Embed EnhancedMovableRepository for base functionality
	*SimpleEnhancedRepository
	
	// Endpoint-specific components
	deltaManager      *DeltaAwareManager
	endpointCache     *EndpointContextCache
	scopeValidator    EndpointScopeValidator
	
	// Delta tracking
	endpointDeltas    map[string]*EndpointDelta
	deltasMutex       sync.RWMutex
	
	// Performance metrics
	metrics           *EndpointMetrics
	
	// Configuration
	config            *EnhancedEndpointConfig
}

// EndpointMetrics tracks endpoint repository performance
type EndpointMetrics struct {
	TotalOperations       int64
	CacheHits             int64
	CacheMisses           int64
	DeltaResolutions      int64
	ContextSwitches       int64
	ExampleSyncs          int64
	AverageLatency        time.Duration
	LastMaintenanceTime   time.Time
	mutex                 sync.RWMutex
}

// EndpointContextCache provides fast access to endpoint context information
type EndpointContextCache struct {
	mu           sync.RWMutex
	cache        map[string]*EndpointMetadata
	scopeIndex   map[string][]string // ScopeID -> EndpointIDs
	ttl          time.Duration
	maxSize      int
	accessOrder  []string // LRU tracking
}

// EndpointScopeValidator validates endpoint operations within scope boundaries
type EndpointScopeValidator interface {
	// ValidateEndpointScope ensures endpoint belongs to expected scope
	ValidateEndpointScope(ctx context.Context, endpointID idwrap.IDWrap, 
		expectedContext EndpointContext, expectedScope idwrap.IDWrap) error
		
	// ValidateExampleScope ensures example belongs to endpoint scope
	ValidateExampleScope(ctx context.Context, exampleID idwrap.IDWrap, 
		endpointID idwrap.IDWrap) error
		
	// ValidateCrossContextMove validates moving endpoints between contexts
	ValidateCrossContextMove(ctx context.Context, endpointID idwrap.IDWrap, 
		fromContext, toContext EndpointContext, targetScopeID idwrap.IDWrap) error
		
	// GetEndpointContext resolves the current context for an endpoint
	GetEndpointContext(ctx context.Context, endpointID idwrap.IDWrap) (EndpointContext, idwrap.IDWrap, error)
}

// EnhancedEndpointConfig extends EnhancedRepositoryConfig with endpoint-specific settings
type EnhancedEndpointConfig struct {
	*EnhancedRepositoryConfig
	
	// Endpoint-specific settings
	EndpointCacheTTL     time.Duration
	EndpointCacheMaxSize int
	EnableExampleCaching bool
	ExampleCacheTTL      time.Duration
	
	// Delta management
	EnableDeltaEndpoints bool
	MaxDeltaDepth        int
	DeltaSyncInterval    time.Duration
	
	// Cross-context operations
	AllowCrossContextMoves bool
	RequireExplicitConsent bool
	
	// Performance tuning
	EndpointBatchSize      int
	ExampleBatchSize       int
	EnableAsyncOperations  bool
}

// =============================================================================
// CONSTRUCTOR AND INITIALIZATION
// =============================================================================

// NewEnhancedEndpointRepository creates a new enhanced endpoint repository
func NewEnhancedEndpointRepository(db *sql.DB, config *EnhancedEndpointConfig) *EnhancedEndpointRepository {
	// Create base enhanced repository
	baseRepo := NewSimpleEnhancedRepository(db, config.EnhancedRepositoryConfig)
	
	// Initialize endpoint-specific cache
	cache := &EndpointContextCache{
		cache:       make(map[string]*EndpointMetadata),
		scopeIndex:  make(map[string][]string),
		ttl:         config.EndpointCacheTTL,
		maxSize:     config.EndpointCacheMaxSize,
		accessOrder: make([]string, 0, config.EndpointCacheMaxSize),
	}
	
	// Initialize delta manager if enabled
	var deltaManager *DeltaAwareManager
	if config.EnableDeltaEndpoints {
		deltaConfig := DefaultDeltaManagerConfig()
		deltaManager = NewDeltaAwareManager(context.Background(), baseRepo, deltaConfig)
	}
	
	return &EnhancedEndpointRepository{
		SimpleEnhancedRepository: baseRepo,
		deltaManager:             deltaManager,
		endpointCache:            cache,
		endpointDeltas:          make(map[string]*EndpointDelta),
		metrics:                 &EndpointMetrics{LastMaintenanceTime: time.Now()},
		config:                  config,
	}
}

// WithScopeValidator sets a custom scope validator
func (r *EnhancedEndpointRepository) WithScopeValidator(validator EndpointScopeValidator) *EnhancedEndpointRepository {
	r.scopeValidator = validator
	return r
}

// TX returns a transaction-aware repository instance
func (r *EnhancedEndpointRepository) TX(tx *sql.Tx) *EnhancedEndpointRepository {
	baseTx := r.SimpleEnhancedRepository.TX(tx).(*SimpleEnhancedRepository)
	
	return &EnhancedEndpointRepository{
		SimpleEnhancedRepository: baseTx,
		deltaManager:             r.deltaManager,
		endpointCache:            r.endpointCache, // Share cache across transactions
		scopeValidator:           r.scopeValidator,
		endpointDeltas:          r.endpointDeltas, // Share deltas for consistency
		metrics:                 r.metrics,        // Share metrics
		config:                  r.config,
	}
}

// =============================================================================
// ENDPOINT-SPECIFIC CONTEXT OPERATIONS
// =============================================================================

// UpdateEndpointPosition updates an endpoint position with context awareness
func (r *EnhancedEndpointRepository) UpdateEndpointPosition(ctx context.Context, tx *sql.Tx,
	endpointID idwrap.IDWrap, listType ListType, position int, 
	endpointContext EndpointContext, scopeID idwrap.IDWrap) error {
	
	startTime := time.Now()
	defer r.trackMetric("UpdateEndpointPosition", startTime)
	
	// Validate context scope if validator is available
	if r.scopeValidator != nil {
		if err := r.scopeValidator.ValidateEndpointScope(ctx, endpointID, endpointContext, scopeID); err != nil {
			return fmt.Errorf("scope validation failed: %w", err)
		}
	}
	
	// Create context metadata
	contextMeta := &ContextMetadata{
		Type:      MovableContext(endpointContext),
		ScopeID:   scopeID.Bytes(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Check if this is a delta endpoint
	if delta := r.getEndpointDelta(endpointID); delta != nil {
		contextMeta.IsHidden = endpointContext == EndpointContextFlow
		originIDBytes := delta.OriginID.Bytes()
		contextMeta.OriginID = &originIDBytes
		contextMeta.Priority = delta.Priority
	}
	
	// Update cache
	r.updateEndpointCache(endpointID, &EndpointMetadata{
		Context:   endpointContext,
		ScopeID:   scopeID,
		IsHidden:  contextMeta.IsHidden,
		OriginID:  nil, // Set from delta if applicable
		Priority:  contextMeta.Priority,
		CreatedAt: contextMeta.CreatedAt,
		UpdatedAt: contextMeta.UpdatedAt,
	})
	
	// Delegate to base repository with context
	return r.SimpleEnhancedRepository.UpdatePositionWithContext(ctx, tx, endpointID, listType, position, contextMeta)
}

// GetEndpointsByCollection retrieves endpoints in a collection context
func (r *EnhancedEndpointRepository) GetEndpointsByCollection(ctx context.Context, 
	collectionID idwrap.IDWrap, includeHidden bool) ([]EndpointResolvedItem, error) {
	
	startTime := time.Now()
	defer r.trackMetric("GetEndpointsByCollection", startTime)
	
	// Create context metadata for collection
	contextMeta := &ContextMetadata{
		Type:    ContextCollection,
		ScopeID: collectionID.Bytes(),
	}
	
	// Get contextual items
	items, err := r.GetItemsByParentWithContext(ctx, collectionID, CollectionListTypeEndpoints, contextMeta)
	if err != nil {
		return nil, fmt.Errorf("failed to get contextual items: %w", err)
	}
	
	// Convert to resolved endpoint items
	endpoints := make([]EndpointResolvedItem, 0, len(items))
	for _, item := range items {
		// Skip hidden items unless explicitly requested
		if item.Context != nil && item.Context.IsHidden && !includeHidden {
			continue
		}
		
		resolved, err := r.resolveEndpoint(ctx, item.ID, EndpointContextCollection, collectionID)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve endpoint %s: %w", item.ID.String(), err)
		}
		
		endpoints = append(endpoints, *resolved)
	}
	
	return endpoints, nil
}

// GetEndpointsByFlow retrieves endpoints in a flow context, including deltas
func (r *EnhancedEndpointRepository) GetEndpointsByFlow(ctx context.Context, 
	flowID idwrap.IDWrap, includeOrigins bool) ([]EndpointResolvedItem, error) {
	
	startTime := time.Now()
	defer r.trackMetric("GetEndpointsByFlow", startTime)
	
	// Get flow-scoped deltas
	deltas := r.getDeltasByScope(flowID, EndpointContextFlow)
	
	endpoints := make([]EndpointResolvedItem, 0, len(deltas))
	for _, delta := range deltas {
		if !delta.IsActive {
			continue
		}
		
		resolved, err := r.resolveEndpoint(ctx, delta.DeltaID, EndpointContextFlow, flowID)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve flow endpoint %s: %w", delta.DeltaID.String(), err)
		}
		
		endpoints = append(endpoints, *resolved)
	}
	
	// Optionally include origin endpoints that don't have flow deltas
	if includeOrigins {
		// This would require additional logic to find collection endpoints
		// and check if they have flow deltas
	}
	
	return endpoints, nil
}

// =============================================================================
// DELTA ENDPOINT OPERATIONS
// =============================================================================

// CreateEndpointDelta creates a delta endpoint from an origin endpoint
func (r *EnhancedEndpointRepository) CreateEndpointDelta(ctx context.Context, tx *sql.Tx,
	originID idwrap.IDWrap, deltaID idwrap.IDWrap, flowID idwrap.IDWrap,
	overrides *EndpointDeltaOverrides) error {
	
	startTime := time.Now()
	defer r.trackMetric("CreateEndpointDelta", startTime)
	
	// Validate scope
	if r.scopeValidator != nil {
		if err := r.scopeValidator.ValidateEndpointScope(ctx, originID, EndpointContextCollection, idwrap.IDWrap{}); err != nil {
			return fmt.Errorf("origin scope validation failed: %w", err)
		}
		
		if err := r.scopeValidator.ValidateEndpointScope(ctx, deltaID, EndpointContextFlow, flowID); err != nil {
			return fmt.Errorf("delta scope validation failed: %w", err)
		}
	}
	
	// Create endpoint delta
	delta := &EndpointDelta{
		DeltaID:    deltaID,
		OriginID:   originID,
		Context:    EndpointContextFlow,
		ScopeID:    flowID,
		IsActive:   true,
		Priority:   1, // Default priority
	}
	
	// Apply overrides if provided
	if overrides != nil {
		delta.HTTPMethod = overrides.HTTPMethod
		delta.URL = overrides.URL
		delta.Description = overrides.Description
		delta.Priority = overrides.Priority
	}
	
	// Store delta relationship
	r.deltasMutex.Lock()
	r.endpointDeltas[deltaID.String()] = delta
	r.deltasMutex.Unlock()
	
	// Create general delta relation for base repository
	relation := &DeltaRelation{
		DeltaID:      deltaID,
		OriginID:     originID,
		Context:      ContextFlow,
		ScopeID:      flowID,
		RelationType: DeltaRelationOverride,
		Priority:     delta.Priority,
		IsActive:     true,
	}
	
	// Delegate to base repository
	if err := r.CreateDelta(ctx, tx, originID, deltaID, relation); err != nil {
		// Rollback delta creation
		r.deltasMutex.Lock()
		delete(r.endpointDeltas, deltaID.String())
		r.deltasMutex.Unlock()
		return fmt.Errorf("failed to create base delta: %w", err)
	}
	
	// Update cache
	r.updateEndpointCache(deltaID, &EndpointMetadata{
		Context:   EndpointContextFlow,
		ScopeID:   flowID,
		IsHidden:  true, // Flow endpoints are hidden from collection view
		OriginID:  &originID,
		Priority:  delta.Priority,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
	
	return nil
}

// ResolveEndpoint resolves an endpoint considering all applicable deltas
func (r *EnhancedEndpointRepository) ResolveEndpoint(ctx context.Context, 
	endpointID idwrap.IDWrap, context EndpointContext, scopeID idwrap.IDWrap) (*EndpointResolvedItem, error) {
	
	startTime := time.Now()
	defer r.trackMetric("ResolveEndpoint", startTime)
	
	return r.resolveEndpoint(ctx, endpointID, context, scopeID)
}

// =============================================================================
// EXAMPLE SUB-CONTEXT MANAGEMENT
// =============================================================================

// UpdateExamplePosition updates an example position within an endpoint context
func (r *EnhancedEndpointRepository) UpdateExamplePosition(ctx context.Context, tx *sql.Tx,
	exampleID idwrap.IDWrap, endpointID idwrap.IDWrap, position int) error {
	
	startTime := time.Now()
	defer r.trackMetric("UpdateExamplePosition", startTime)
	
	// Validate example-endpoint relationship
	if r.scopeValidator != nil {
		if err := r.scopeValidator.ValidateExampleScope(ctx, exampleID, endpointID); err != nil {
			return fmt.Errorf("example scope validation failed: %w", err)
		}
	}
	
	// Get endpoint context from cache or resolve
	endpointMeta, err := r.getEndpointMetadata(ctx, endpointID)
	if err != nil {
		return fmt.Errorf("failed to get endpoint metadata: %w", err)
	}
	
	// Create context metadata for example
	contextMeta := &ContextMetadata{
		Type:      MovableContext(endpointMeta.Context),
		ScopeID:   endpointMeta.ScopeID.Bytes(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Delegate to base repository
	return r.UpdatePositionWithContext(ctx, tx, exampleID, CollectionListTypeExamples, position, contextMeta)
}

// GetExamplesByEndpoint retrieves examples for an endpoint in its current context
func (r *EnhancedEndpointRepository) GetExamplesByEndpoint(ctx context.Context, 
	endpointID idwrap.IDWrap) ([]ContextualMovableItem, error) {
	
	startTime := time.Now()
	defer r.trackMetric("GetExamplesByEndpoint", startTime)
	
	// Get endpoint context
	endpointMeta, err := r.getEndpointMetadata(ctx, endpointID)
	if err != nil {
		return nil, fmt.Errorf("failed to get endpoint metadata: %w", err)
	}
	
	// Create context metadata
	contextMeta := &ContextMetadata{
		Type:    MovableContext(endpointMeta.Context),
		ScopeID: endpointMeta.ScopeID.Bytes(),
	}
	
	// Get contextual items
	return r.GetItemsByParentWithContext(ctx, endpointID, CollectionListTypeExamples, contextMeta)
}

// =============================================================================
// CROSS-CONTEXT MOVE VALIDATION
// =============================================================================

// ValidateCrossContextEndpointMove validates moving an endpoint between contexts
func (r *EnhancedEndpointRepository) ValidateCrossContextEndpointMove(ctx context.Context,
	endpointID idwrap.IDWrap, fromContext, toContext EndpointContext, 
	targetScopeID idwrap.IDWrap) error {
	
	startTime := time.Now()
	defer r.trackMetric("ValidateCrossContextMove", startTime)
	
	// Check if cross-context moves are allowed
	if !r.config.AllowCrossContextMoves {
		return fmt.Errorf("cross-context moves are disabled")
	}
	
	// Use scope validator if available
	if r.scopeValidator != nil {
		return r.scopeValidator.ValidateCrossContextMove(ctx, endpointID, fromContext, toContext, targetScopeID)
	}
	
	// Basic validation rules
	switch {
	case fromContext == EndpointContextCollection && toContext == EndpointContextFlow:
		// Collection -> Flow: Create delta, keep original
		return nil
		
	case fromContext == EndpointContextFlow && toContext == EndpointContextCollection:
		// Flow -> Collection: Not allowed (would break delta relationship)
		return fmt.Errorf("cannot move flow endpoint back to collection context")
		
	case fromContext == EndpointContextRequest:
		// Request endpoints cannot be moved
		return fmt.Errorf("request endpoints cannot be moved between contexts")
		
	default:
		return fmt.Errorf("unsupported context transition: %s -> %s", fromContext.String(), toContext.String())
	}
}

// =============================================================================
// PERFORMANCE OPERATIONS
// =============================================================================

// BatchCreateEndpointDeltas creates multiple endpoint deltas efficiently
func (r *EnhancedEndpointRepository) BatchCreateEndpointDeltas(ctx context.Context, tx *sql.Tx,
	operations []EndpointDeltaOperation) (*BatchEndpointResult, error) {
	
	startTime := time.Now()
	defer r.trackMetric("BatchCreateEndpointDeltas", startTime)
	
	if len(operations) > r.config.EndpointBatchSize {
		return nil, fmt.Errorf("batch size %d exceeds maximum %d", len(operations), r.config.EndpointBatchSize)
	}
	
	result := &BatchEndpointResult{
		TotalRequested: len(operations),
		StartTime:      startTime,
	}
	
	// Process in chunks for better memory usage
	chunkSize := min(r.config.EndpointBatchSize, 20)
	for i := 0; i < len(operations); i += chunkSize {
		end := min(i+chunkSize, len(operations))
		chunk := operations[i:end]
		
		chunkResult, err := r.processDeltaBatchChunk(ctx, tx, chunk)
		if err != nil {
			result.FailedCount += len(chunk)
			result.Errors = append(result.Errors, fmt.Errorf("chunk %d-%d failed: %w", i, end, err))
			continue
		}
		
		result.SuccessCount += chunkResult.SuccessCount
		result.FailedCount += chunkResult.FailedCount
		result.CreatedDeltas = append(result.CreatedDeltas, chunkResult.CreatedDeltas...)
	}
	
	result.Duration = time.Since(startTime)
	return result, nil
}

// CompactEndpointPositions rebalances endpoint positions within a scope
func (r *EnhancedEndpointRepository) CompactEndpointPositions(ctx context.Context, tx *sql.Tx,
	scopeID idwrap.IDWrap, context EndpointContext) error {
	
	startTime := time.Now()
	defer r.trackMetric("CompactEndpointPositions", startTime)
	
	// Get all endpoints in scope
	var endpoints []EndpointResolvedItem
	var err error
	
	switch context {
	case EndpointContextCollection:
		endpoints, err = r.GetEndpointsByCollection(ctx, scopeID, true)
	case EndpointContextFlow:
		endpoints, err = r.GetEndpointsByFlow(ctx, scopeID, false)
	default:
		return fmt.Errorf("unsupported context for compaction: %s", context.String())
	}
	
	if err != nil {
		return fmt.Errorf("failed to get endpoints for compaction: %w", err)
	}
	
	if len(endpoints) == 0 {
		return nil // Nothing to compact
	}
	
	// Create position updates with sequential positions
	updates := make([]ContextualPositionUpdate, len(endpoints))
	for i, endpoint := range endpoints {
		contextMeta := &ContextMetadata{
			Type:    MovableContext(context),
			ScopeID: scopeID.Bytes(),
		}
		
		updates[i] = ContextualPositionUpdate{
			ItemID:   endpoint.ID,
			ListType: CollectionListTypeEndpoints,
			Position: i,
			Context:  contextMeta,
			IsDelta:  endpoint.DeltaID != nil,
		}
		
		if endpoint.OriginID != nil {
			updates[i].OriginID = endpoint.OriginID
		}
	}
	
	// Apply batch updates
	return r.UpdatePositionsWithContext(ctx, tx, updates)
}

// =============================================================================
// HELPER METHODS AND UTILITIES
// =============================================================================

// resolveEndpoint internal method for resolving endpoint with context
func (r *EnhancedEndpointRepository) resolveEndpoint(ctx context.Context,
	endpointID idwrap.IDWrap, context EndpointContext, scopeID idwrap.IDWrap) (*EndpointResolvedItem, error) {
	
	// Check cache first
	if cached, found := r.getFromCache(endpointID); found {
		return r.buildResolvedItem(endpointID, cached), nil
	}
	
	// Get endpoint delta if exists
	delta := r.getEndpointDelta(endpointID)
	
	// Build resolved item
	resolved := &EndpointResolvedItem{
		ID:             endpointID,
		EffectiveID:    endpointID,
		Context:        context,
		ResolutionTime: time.Now(),
	}
	
	if delta != nil {
		resolved.OriginID = &delta.OriginID
		resolved.DeltaID = &delta.DeltaID
		resolved.AppliedDeltas = []EndpointDelta{*delta}
		
		// Apply delta overrides
		if delta.HTTPMethod != nil {
			resolved.HTTPMethod = *delta.HTTPMethod
		}
		if delta.URL != nil {
			resolved.URL = *delta.URL
		}
		if delta.Description != nil {
			resolved.Description = *delta.Description
		}
	}
	
	return resolved, nil
}

// getEndpointDelta retrieves endpoint delta by ID
func (r *EnhancedEndpointRepository) getEndpointDelta(endpointID idwrap.IDWrap) *EndpointDelta {
	r.deltasMutex.RLock()
	defer r.deltasMutex.RUnlock()
	
	delta, exists := r.endpointDeltas[endpointID.String()]
	if exists && delta.IsActive {
		return delta
	}
	return nil
}

// getDeltasByScope retrieves all deltas in a specific scope
func (r *EnhancedEndpointRepository) getDeltasByScope(scopeID idwrap.IDWrap, 
	context EndpointContext) []EndpointDelta {
	
	r.deltasMutex.RLock()
	defer r.deltasMutex.RUnlock()
	
	deltas := make([]EndpointDelta, 0)
	scopeIDStr := scopeID.String()
	
	for _, delta := range r.endpointDeltas {
		if delta.ScopeID.String() == scopeIDStr && 
		   delta.Context == context && 
		   delta.IsActive {
			deltas = append(deltas, *delta)
		}
	}
	
	return deltas
}

// getEndpointMetadata retrieves or resolves endpoint metadata
func (r *EnhancedEndpointRepository) getEndpointMetadata(ctx context.Context,
	endpointID idwrap.IDWrap) (*EndpointMetadata, error) {
	
	// Try cache first
	if cached, found := r.getFromCache(endpointID); found {
		return cached, nil
	}
	
	// Resolve using scope validator if available
	if r.scopeValidator != nil {
		context, scopeID, err := r.scopeValidator.GetEndpointContext(ctx, endpointID)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve endpoint context: %w", err)
		}
		
		metadata := &EndpointMetadata{
			Context:   context,
			ScopeID:   scopeID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		
		// Cache the result
		r.updateEndpointCache(endpointID, metadata)
		return metadata, nil
	}
	
	// Default fallback
	return &EndpointMetadata{
		Context:   EndpointContextCollection,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

// updateEndpointCache updates the endpoint metadata cache
func (r *EnhancedEndpointRepository) updateEndpointCache(endpointID idwrap.IDWrap, 
	metadata *EndpointMetadata) {
	
	r.endpointCache.mu.Lock()
	defer r.endpointCache.mu.Unlock()
	
	key := endpointID.String()
	
	// Update main cache
	r.endpointCache.cache[key] = metadata
	
	// Update scope index
	scopeKey := metadata.ScopeID.String()
	if _, exists := r.endpointCache.scopeIndex[scopeKey]; !exists {
		r.endpointCache.scopeIndex[scopeKey] = make([]string, 0)
	}
	
	// Add to scope index if not already present
	found := false
	for _, id := range r.endpointCache.scopeIndex[scopeKey] {
		if id == key {
			found = true
			break
		}
	}
	if !found {
		r.endpointCache.scopeIndex[scopeKey] = append(r.endpointCache.scopeIndex[scopeKey], key)
	}
	
	// Update LRU order
	r.updateAccessOrder(key)
	
	// Enforce cache size limits
	if len(r.endpointCache.cache) > r.endpointCache.maxSize {
		r.evictOldestEntry()
	}
}

// getFromCache retrieves endpoint metadata from cache
func (r *EnhancedEndpointRepository) getFromCache(endpointID idwrap.IDWrap) (*EndpointMetadata, bool) {
	r.endpointCache.mu.RLock()
	defer r.endpointCache.mu.RUnlock()
	
	key := endpointID.String()
	metadata, exists := r.endpointCache.cache[key]
	if !exists {
		return nil, false
	}
	
	// Check TTL
	if r.endpointCache.ttl > 0 && time.Since(metadata.UpdatedAt) > r.endpointCache.ttl {
		return nil, false
	}
	
	// Update access order (deferred to avoid lock issues)
	go r.updateAccessOrderAsync(key)
	
	return metadata, true
}

// updateAccessOrder updates LRU access order
func (r *EnhancedEndpointRepository) updateAccessOrder(key string) {
	// Remove existing entry
	for i, id := range r.endpointCache.accessOrder {
		if id == key {
			r.endpointCache.accessOrder = append(
				r.endpointCache.accessOrder[:i],
				r.endpointCache.accessOrder[i+1:]...)
			break
		}
	}
	
	// Add to end (most recent)
	r.endpointCache.accessOrder = append(r.endpointCache.accessOrder, key)
}

// updateAccessOrderAsync updates access order asynchronously
func (r *EnhancedEndpointRepository) updateAccessOrderAsync(key string) {
	r.endpointCache.mu.Lock()
	defer r.endpointCache.mu.Unlock()
	r.updateAccessOrder(key)
}

// evictOldestEntry removes the least recently used cache entry
func (r *EnhancedEndpointRepository) evictOldestEntry() {
	if len(r.endpointCache.accessOrder) > 0 {
		oldest := r.endpointCache.accessOrder[0]
		r.endpointCache.accessOrder = r.endpointCache.accessOrder[1:]
		
		// Remove from main cache
		if metadata, exists := r.endpointCache.cache[oldest]; exists {
			delete(r.endpointCache.cache, oldest)
			
			// Remove from scope index
			scopeKey := metadata.ScopeID.String()
			if scopeList, exists := r.endpointCache.scopeIndex[scopeKey]; exists {
				for i, id := range scopeList {
					if id == oldest {
						r.endpointCache.scopeIndex[scopeKey] = append(
							scopeList[:i], scopeList[i+1:]...)
						break
					}
				}
			}
		}
	}
}

// buildResolvedItem builds a resolved item from cached metadata
func (r *EnhancedEndpointRepository) buildResolvedItem(endpointID idwrap.IDWrap,
	metadata *EndpointMetadata) *EndpointResolvedItem {
	
	resolved := &EndpointResolvedItem{
		ID:             endpointID,
		EffectiveID:    endpointID,
		Context:        metadata.Context,
		ExampleCount:   metadata.ExampleCount,
		ResolutionTime: time.Now(),
	}
	
	if metadata.OriginID != nil {
		resolved.OriginID = metadata.OriginID
		resolved.DeltaID = &endpointID
	}
	
	return resolved
}

// processDeltaBatchChunk processes a chunk of delta operations
func (r *EnhancedEndpointRepository) processDeltaBatchChunk(ctx context.Context, tx *sql.Tx,
	operations []EndpointDeltaOperation) (*BatchEndpointResult, error) {
	
	result := &BatchEndpointResult{
		TotalRequested: len(operations),
		StartTime:      time.Now(),
	}
	
	for _, op := range operations {
		if err := r.CreateEndpointDelta(ctx, tx, op.OriginID, op.DeltaID, op.ScopeID, op.Overrides); err != nil {
			result.FailedCount++
			result.Errors = append(result.Errors, fmt.Errorf("failed to create delta %s: %w", op.DeltaID.String(), err))
			continue
		}
		
		result.SuccessCount++
		result.CreatedDeltas = append(result.CreatedDeltas, EndpointDelta{
			DeltaID:  op.DeltaID,
			OriginID: op.OriginID,
			Context:  EndpointContextFlow,
			ScopeID:  op.ScopeID,
			IsActive: true,
		})
	}
	
	result.Duration = time.Since(result.StartTime)
	return result, nil
}

// trackMetric tracks performance metrics
func (r *EnhancedEndpointRepository) trackMetric(operation string, startTime time.Time) {
	duration := time.Since(startTime)
	
	r.metrics.mutex.Lock()
	defer r.metrics.mutex.Unlock()
	
	r.metrics.TotalOperations++
	
	// Update average latency (simple moving average)
	if r.metrics.AverageLatency == 0 {
		r.metrics.AverageLatency = duration
	} else {
		// Weighted average: 80% previous, 20% current
		r.metrics.AverageLatency = time.Duration(
			0.8*float64(r.metrics.AverageLatency) + 0.2*float64(duration))
	}
}

// min helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// =============================================================================
// SUPPORTING TYPES AND INTERFACES
// =============================================================================

// EndpointDeltaOverrides specifies what to override in a delta endpoint
type EndpointDeltaOverrides struct {
	HTTPMethod  *string
	URL         *string
	Description *string
	Priority    int
}

// EndpointDeltaOperation represents a batch delta creation operation
type EndpointDeltaOperation struct {
	OriginID  idwrap.IDWrap
	DeltaID   idwrap.IDWrap
	ScopeID   idwrap.IDWrap
	Overrides *EndpointDeltaOverrides
}

// BatchEndpointResult contains results of batch endpoint operations
type BatchEndpointResult struct {
	TotalRequested int
	SuccessCount   int
	FailedCount    int
	CreatedDeltas  []EndpointDelta
	Errors         []error
	StartTime      time.Time
	Duration       time.Duration
}

// EndpointItem basic implementation for compatibility with existing Item interface
type EndpointItem struct {
	Position int
	ParentID idwrap.IDWrap
}

func (s EndpointItem) GetPosition() int              { return s.Position }
func (s EndpointItem) GetParentID() idwrap.IDWrap   { return s.ParentID }