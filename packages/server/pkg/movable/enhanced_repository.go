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
// ENHANCED REPOSITORY SPECIFIC TYPES
// =============================================================================

// =============================================================================
// SIMPLE ENHANCED REPOSITORY IMPLEMENTATION
// =============================================================================

// SimpleEnhancedRepository implements EnhancedMovableRepository by embedding SimpleRepository
// and adding context-aware operations for delta support
type SimpleEnhancedRepository struct {
	// Embed the base SimpleRepository for backward compatibility
	*SimpleRepository
	
	// Enhanced functionality
	config        *EnhancedRepositoryConfig
	contextCache  ContextCache
	scopeResolver ScopeResolver
	
	// Delta relationship tracking
	deltaRelations map[string]*DeltaRelation
	deltasMutex    sync.RWMutex
	
	// Performance tracking
	performanceMetrics *PerformanceMetrics
}

// PerformanceMetrics tracks repository performance
type PerformanceMetrics struct {
	TotalOperations    int64
	AverageLatency     time.Duration
	CacheHitRate       float64
	DeltaResolutions   int64
	LastCompactionTime time.Time
	mutex             sync.RWMutex
}

// NewSimpleEnhancedRepository creates a new enhanced repository instance
func NewSimpleEnhancedRepository(db *sql.DB, config *EnhancedRepositoryConfig) *SimpleEnhancedRepository {
	// Create base repository with enhanced configuration's base config
	baseRepo := NewSimpleRepository(db, config.MoveConfig)
	
	// Initialize context cache if not provided
	contextCache := config.ContextCache
	if contextCache == nil {
		contextCache = NewInMemoryContextCache(config.CacheTTL)
	}
	
	return &SimpleEnhancedRepository{
		SimpleRepository:   baseRepo,
		config:            config,
		contextCache:      contextCache,
		scopeResolver:     config.ScopeResolver,
		deltaRelations:    make(map[string]*DeltaRelation),
		performanceMetrics: &PerformanceMetrics{
			LastCompactionTime: time.Now(),
		},
	}
}

// TX returns an enhanced repository instance with transaction support
func (r *SimpleEnhancedRepository) TX(tx *sql.Tx) EnhancedMovableRepository {
	// Get transaction-aware base repository
	baseTxRepo := r.SimpleRepository.TX(tx).(*SimpleRepository)
	
	return &SimpleEnhancedRepository{
		SimpleRepository:   baseTxRepo,
		config:            r.config,
		contextCache:      r.contextCache,
		scopeResolver:     r.scopeResolver,
		deltaRelations:    r.deltaRelations, // Share delta relations for consistency
		performanceMetrics: r.performanceMetrics, // Share metrics
	}
}

// WithContext returns a repository instance with cached context resolver
func (r *SimpleEnhancedRepository) WithContext(resolver ScopeResolver, cache ContextCache) EnhancedMovableRepository {
	return &SimpleEnhancedRepository{
		SimpleRepository:   r.SimpleRepository,
		config:            r.config,
		contextCache:      cache,
		scopeResolver:     resolver,
		deltaRelations:    r.deltaRelations,
		performanceMetrics: r.performanceMetrics,
	}
}

// =============================================================================
// CONTEXT-AWARE OPERATIONS
// =============================================================================

// UpdatePositionWithContext updates position with context awareness for delta operations
func (r *SimpleEnhancedRepository) UpdatePositionWithContext(ctx context.Context, tx *sql.Tx, 
	itemID idwrap.IDWrap, listType ListType, position int, contextMeta *ContextMetadata) error {
	
	startTime := time.Now()
	defer r.trackPerformance("UpdatePositionWithContext", startTime)
	
	// Input validation
	if contextMeta == nil {
		return fmt.Errorf("context metadata is required for context-aware operations")
	}
	
	if position < 0 {
		return fmt.Errorf("invalid position: %d (must be >= 0)", position)
	}
	
	// Validate context scope
	if err := r.validateContextScope(ctx, itemID, contextMeta); err != nil {
		return fmt.Errorf("context scope validation failed: %w", err)
	}
	
	// Get transaction-aware repository
	repo := r
	if tx != nil {
		repo = r.TX(tx).(*SimpleEnhancedRepository)
	}
	
	// Check if this is a delta operation
	if contextMeta.IsHidden || contextMeta.OriginID != nil {
		return r.updateDeltaPosition(ctx, tx, itemID, listType, position, contextMeta)
	}
	
	// Cache context metadata
	r.contextCache.SetContext(itemID, contextMeta)
	
	// Delegate to base repository for regular operations
	return repo.SimpleRepository.UpdatePosition(ctx, tx, itemID, listType, position)
}

// UpdatePositionsWithContext updates multiple positions with context awareness
func (r *SimpleEnhancedRepository) UpdatePositionsWithContext(ctx context.Context, tx *sql.Tx, 
	updates []ContextualPositionUpdate) error {
	
	startTime := time.Now()
	defer r.trackPerformance("UpdatePositionsWithContext", startTime)
	
	if len(updates) == 0 {
		return nil
	}
	
	// Get transaction-aware repository
	repo := r
	if tx != nil {
		repo = r.TX(tx).(*SimpleEnhancedRepository)
	}
	
	// Group updates by context and delta status
	deltaUpdates := make([]ContextualPositionUpdate, 0)
	regularUpdates := make([]PositionUpdate, 0)
	
	for _, update := range updates {
		// Validate context if provided
		if update.Context != nil {
			if err := r.validateContextScope(ctx, update.ItemID, update.Context); err != nil {
				return fmt.Errorf("context validation failed for item %s: %w", update.ItemID.String(), err)
			}
			
			// Cache context metadata
			r.contextCache.SetContext(update.ItemID, update.Context)
		}
		
		if update.IsDelta {
			deltaUpdates = append(deltaUpdates, update)
		} else {
			regularUpdates = append(regularUpdates, PositionUpdate{
				ItemID:   update.ItemID,
				ListType: update.ListType,
				Position: update.Position,
			})
		}
	}
	
	// Process delta updates
	if len(deltaUpdates) > 0 {
		if err := r.processBatchDeltaUpdates(ctx, tx, deltaUpdates); err != nil {
			return fmt.Errorf("batch delta updates failed: %w", err)
		}
	}
	
	// Process regular updates
	if len(regularUpdates) > 0 {
		if err := repo.SimpleRepository.UpdatePositions(ctx, tx, regularUpdates); err != nil {
			return fmt.Errorf("batch regular updates failed: %w", err)
		}
	}
	
	return nil
}

// GetItemsByParentWithContext returns items filtered by context and delta visibility
func (r *SimpleEnhancedRepository) GetItemsByParentWithContext(ctx context.Context, 
	parentID idwrap.IDWrap, listType ListType, contextMeta *ContextMetadata) ([]ContextualMovableItem, error) {
	
	startTime := time.Now()
	defer r.trackPerformance("GetItemsByParentWithContext", startTime)
	
	// Get base items
	baseItems, err := r.SimpleRepository.GetItemsByParent(ctx, parentID, listType)
	if err != nil {
		return nil, fmt.Errorf("failed to get base items: %w", err)
	}
	
	// Convert to contextual items
	contextualItems := make([]ContextualMovableItem, 0, len(baseItems))
	
	for _, item := range baseItems {
		// Get or resolve context metadata
		itemContext := contextMeta
		if itemContext == nil {
			// Try to get from cache
			if cachedContext, found := r.contextCache.GetContext(item.ID); found {
				itemContext = cachedContext
			}
		}
		
		// Check if item should be included based on context
		if itemContext != nil && r.shouldIncludeInContext(item.ID, itemContext) {
			// Check for delta relationship
			deltaRelation := r.getDeltaRelation(item.ID)
			
			contextualItem := ContextualMovableItem{
				ID:            item.ID,
				ParentID:      item.ParentID,
				Position:      item.Position,
				ListType:      item.ListType,
				Context:       itemContext,
				IsDelta:       deltaRelation != nil,
				DeltaRelation: deltaRelation,
			}
			
			if deltaRelation != nil {
				contextualItem.OriginID = &deltaRelation.OriginID
			}
			
			contextualItems = append(contextualItems, contextualItem)
		}
	}
	
	return contextualItems, nil
}

// =============================================================================
// DELTA-SPECIFIC OPERATIONS
// =============================================================================

// CreateDelta creates a new delta relationship for an existing item
func (r *SimpleEnhancedRepository) CreateDelta(ctx context.Context, tx *sql.Tx, 
	originID idwrap.IDWrap, deltaID idwrap.IDWrap, relation *DeltaRelation) error {
	
	startTime := time.Now()
	defer r.trackPerformance("CreateDelta", startTime)
	
	// Input validation
	if relation == nil {
		return fmt.Errorf("delta relation is required")
	}
	
	// Validate scope
	if err := r.validateContextScope(ctx, deltaID, &ContextMetadata{
		Type:    relation.Context,
		ScopeID: relation.ScopeID.Bytes(),
	}); err != nil {
		return fmt.Errorf("delta scope validation failed: %w", err)
	}
	
	// Ensure delta ID and relation are consistent
	relation.DeltaID = deltaID
	relation.OriginID = originID
	relation.IsActive = true
	
	// Store delta relationship
	r.deltasMutex.Lock()
	r.deltaRelations[deltaID.String()] = relation
	r.deltasMutex.Unlock()
	
	// Create context metadata for delta
	originIDBytes := originID.Bytes()
	contextMeta := &ContextMetadata{
		Type:      relation.Context,
		ScopeID:   relation.ScopeID.Bytes(),
		IsHidden:  relation.RelationType == DeltaRelationSuppression,
		OriginID:  &originIDBytes,
		Priority:  relation.Priority,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Cache context metadata
	r.contextCache.SetContext(deltaID, contextMeta)
	
	// Call lifecycle handler if configured
	if r.config.DeltaHandler != nil {
		if err := r.config.DeltaHandler.OnDeltaCreated(ctx, relation); err != nil {
			return fmt.Errorf("delta lifecycle handler failed: %w", err)
		}
	}
	
	return nil
}

// ResolveDelta resolves the effective item considering delta overrides
func (r *SimpleEnhancedRepository) ResolveDelta(ctx context.Context, originID idwrap.IDWrap, 
	contextMeta *ContextMetadata) (*ResolvedItem, error) {
	
	startTime := time.Now()
	defer r.trackPerformance("ResolveDelta", startTime)
	
	// Find applicable deltas for this origin and context
	applicableDeltas := r.findApplicableDeltas(originID, contextMeta)
	
	if len(applicableDeltas) == 0 {
		// No deltas, return origin item
		return &ResolvedItem{
			ID:             originID,
			EffectiveID:    originID,
			OriginID:       &originID,
			ResolutionType: ResolutionOrigin,
			ResolutionTime: time.Now(),
			AppliedDeltas:  []DeltaRelation{},
		}, nil
	}
	
	// Sort deltas by priority (higher priority = higher precedence)
	// For performance, use a simple sort here
	for i := 0; i < len(applicableDeltas); i++ {
		for j := i + 1; j < len(applicableDeltas); j++ {
			if applicableDeltas[i].Priority < applicableDeltas[j].Priority {
				applicableDeltas[i], applicableDeltas[j] = applicableDeltas[j], applicableDeltas[i]
			}
		}
	}
	
	// Apply the highest priority delta
	topDelta := applicableDeltas[0]
	
	// Determine resolution type
	resolutionType := ResolutionDelta
	if len(applicableDeltas) > 1 {
		resolutionType = ResolutionMerged
	}
	if topDelta.RelationType == DeltaRelationSuppression {
		resolutionType = ResolutionSuppressed
	}
	
	resolved := &ResolvedItem{
		ID:             originID,
		EffectiveID:    topDelta.DeltaID,
		OriginID:       &originID,
		DeltaID:        &topDelta.DeltaID,
		ResolutionType: resolutionType,
		ResolutionTime: time.Now(),
		AppliedDeltas:  applicableDeltas,
	}
	
	// Get position and parent information from effective item
	if effectiveItem, err := r.getItemByID(ctx, topDelta.DeltaID); err == nil {
		resolved.Position = effectiveItem.GetPosition()
		parentID := effectiveItem.GetParentID()
		resolved.ParentID = &parentID
	}
	
	// Set context from metadata
	resolved.Context = contextMeta
	
	// Call lifecycle handler if configured
	if r.config.DeltaHandler != nil {
		if err := r.config.DeltaHandler.OnDeltaResolved(ctx, resolved); err != nil {
			return nil, fmt.Errorf("delta resolution lifecycle handler failed: %w", err)
		}
	}
	
	return resolved, nil
}

// GetDeltasByOrigin returns all deltas for an original item
func (r *SimpleEnhancedRepository) GetDeltasByOrigin(ctx context.Context, originID idwrap.IDWrap) ([]DeltaRelation, error) {
	startTime := time.Now()
	defer r.trackPerformance("GetDeltasByOrigin", startTime)
	
	r.deltasMutex.RLock()
	defer r.deltasMutex.RUnlock()
	
	deltas := make([]DeltaRelation, 0)
	originIDStr := originID.String()
	
	for _, relation := range r.deltaRelations {
		if relation.OriginID.String() == originIDStr && relation.IsActive {
			deltas = append(deltas, *relation)
		}
	}
	
	return deltas, nil
}

// GetDeltasByScope returns all deltas within a specific scope
func (r *SimpleEnhancedRepository) GetDeltasByScope(ctx context.Context, scopeID idwrap.IDWrap, 
	contextType MovableContext) ([]DeltaRelation, error) {
	
	startTime := time.Now()
	defer r.trackPerformance("GetDeltasByScope", startTime)
	
	r.deltasMutex.RLock()
	defer r.deltasMutex.RUnlock()
	
	deltas := make([]DeltaRelation, 0)
	scopeIDStr := scopeID.String()
	
	for _, relation := range r.deltaRelations {
		if relation.ScopeID.String() == scopeIDStr && 
		   relation.Context == contextType && 
		   relation.IsActive {
			deltas = append(deltas, *relation)
		}
	}
	
	return deltas, nil
}

// =============================================================================
// SCOPE MANAGEMENT
// =============================================================================

// ValidateContextScope ensures an item belongs to the expected context and scope
func (r *SimpleEnhancedRepository) ValidateContextScope(ctx context.Context, itemID idwrap.IDWrap, 
	expectedContext MovableContext, expectedScope idwrap.IDWrap) error {
	
	startTime := time.Now()
	defer r.trackPerformance("ValidateContextScope", startTime)
	
	if !r.config.EnableScopeValidation {
		return nil // Scope validation disabled
	}
	
	// Use scope resolver if available
	if r.scopeResolver != nil {
		actualContext, err := r.scopeResolver.ResolveContext(ctx, itemID)
		if err != nil {
			return fmt.Errorf("failed to resolve context: %w", err)
		}
		
		if actualContext != expectedContext {
			return fmt.Errorf("context mismatch: expected %s, got %s", 
				expectedContext.String(), actualContext.String())
		}
		
		actualScope, err := r.scopeResolver.ResolveScopeID(ctx, itemID, expectedContext)
		if err != nil {
			return fmt.Errorf("failed to resolve scope ID: %w", err)
		}
		
		if actualScope.Compare(expectedScope) != 0 {
			return fmt.Errorf("scope mismatch: expected %s, got %s", 
				expectedScope.String(), actualScope.String())
		}
		
		return nil
	}
	
	// Fallback validation using cached context metadata
	if cachedContext, found := r.contextCache.GetContext(itemID); found {
		if cachedContext.Type != expectedContext {
			return fmt.Errorf("cached context mismatch: expected %s, got %s", 
				expectedContext.String(), cachedContext.Type.String())
		}
		
		if string(cachedContext.ScopeID) != string(expectedScope.Bytes()) {
			return fmt.Errorf("cached scope mismatch")
		}
	}
	
	return nil
}

// GetEffectiveOrder returns the resolved order considering all delta overrides
func (r *SimpleEnhancedRepository) GetEffectiveOrder(ctx context.Context, parentID idwrap.IDWrap, 
	listType ListType, contextMeta *ContextMetadata) ([]ResolvedItem, error) {
	
	startTime := time.Now()
	defer r.trackPerformance("GetEffectiveOrder", startTime)
	
	// Get base items
	baseItems, err := r.SimpleRepository.GetItemsByParent(ctx, parentID, listType)
	if err != nil {
		return nil, fmt.Errorf("failed to get base items: %w", err)
	}
	
	resolvedItems := make([]ResolvedItem, 0, len(baseItems))
	
	for _, item := range baseItems {
		// Resolve each item considering deltas
		resolved, err := r.ResolveDelta(ctx, item.ID, contextMeta)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve delta for item %s: %w", item.ID.String(), err)
		}
		
		// Skip suppressed items
		if resolved.ResolutionType != ResolutionSuppressed {
			resolvedItems = append(resolvedItems, *resolved)
		}
	}
	
	return resolvedItems, nil
}

// =============================================================================
// PERFORMANCE OPERATIONS
// =============================================================================

// BatchCreateDeltas efficiently creates multiple delta relationships
func (r *SimpleEnhancedRepository) BatchCreateDeltas(ctx context.Context, tx *sql.Tx, 
	relations []DeltaRelation) error {
	
	startTime := time.Now()
	defer r.trackPerformance("BatchCreateDeltas", startTime)
	
	if len(relations) == 0 {
		return nil
	}
	
	// Validate batch size
	if len(relations) > r.config.BatchSize {
		return fmt.Errorf("batch size %d exceeds maximum %d", len(relations), r.config.BatchSize)
	}
	
	// Process in smaller chunks if needed
	batchSize := r.config.BatchSize
	if batchSize == 0 {
		batchSize = 50 // Default batch size
	}
	
	for i := 0; i < len(relations); i += batchSize {
		end := i + batchSize
		if end > len(relations) {
			end = len(relations)
		}
		
		chunk := relations[i:end]
		if err := r.processDeltaBatch(ctx, tx, chunk); err != nil {
			return fmt.Errorf("batch processing failed at chunk %d-%d: %w", i, end, err)
		}
	}
	
	return nil
}

// BatchResolveDeltas efficiently resolves multiple items with their deltas
func (r *SimpleEnhancedRepository) BatchResolveDeltas(ctx context.Context, originIDs []idwrap.IDWrap, 
	contextMeta *ContextMetadata) (map[idwrap.IDWrap]*ResolvedItem, error) {
	
	startTime := time.Now()
	defer r.trackPerformance("BatchResolveDeltas", startTime)
	
	results := make(map[idwrap.IDWrap]*ResolvedItem, len(originIDs))
	
	for _, originID := range originIDs {
		resolved, err := r.ResolveDelta(ctx, originID, contextMeta)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve item %s: %w", originID.String(), err)
		}
		results[originID] = resolved
	}
	
	return results, nil
}

// CompactDeltaPositions rebalances positions within a delta context
func (r *SimpleEnhancedRepository) CompactDeltaPositions(ctx context.Context, tx *sql.Tx, 
	scopeID idwrap.IDWrap, contextType MovableContext, listType ListType) error {
	
	startTime := time.Now()
	defer r.trackPerformance("CompactDeltaPositions", startTime)
	
	// Get all deltas in scope
	deltas, err := r.GetDeltasByScope(ctx, scopeID, contextType)
	if err != nil {
		return fmt.Errorf("failed to get deltas for scope: %w", err)
	}
	
	if len(deltas) == 0 {
		return nil // Nothing to compact
	}
	
	// Get transaction-aware repository (not used but kept for consistency)
	_ = r
	if tx != nil {
		_ = r.TX(tx).(*SimpleEnhancedRepository)
	}
	
	// Group deltas by parent for compaction
	parentGroups := make(map[idwrap.IDWrap][]DeltaRelation)
	for _, delta := range deltas {
		// Get parent ID for delta item
		if item, err := r.getItemByID(ctx, delta.DeltaID); err == nil {
			parentID := item.GetParentID()
			parentGroups[parentID] = append(parentGroups[parentID], delta)
		}
	}
	
	// Compact each parent group
	for parentID, groupDeltas := range parentGroups {
		if err := r.compactDeltaGroup(ctx, tx, parentID, groupDeltas, listType); err != nil {
			return fmt.Errorf("failed to compact deltas for parent %s: %w", parentID.String(), err)
		}
	}
	
	// Update compaction timestamp
	r.performanceMetrics.mutex.Lock()
	r.performanceMetrics.LastCompactionTime = time.Now()
	r.performanceMetrics.mutex.Unlock()
	
	return nil
}

// =============================================================================
// HELPER METHODS
// =============================================================================

// validateContextScope validates context metadata scope
func (r *SimpleEnhancedRepository) validateContextScope(ctx context.Context, itemID idwrap.IDWrap, 
	contextMeta *ContextMetadata) error {
	
	if !r.config.EnableScopeValidation {
		return nil
	}
	
	if contextMeta == nil {
		return fmt.Errorf("context metadata is required when scope validation is enabled")
	}
	
	// Use scope resolver for validation if available
	if r.scopeResolver != nil {
		return r.scopeResolver.ValidateScope(ctx, itemID, idwrap.NewFromBytesMust(contextMeta.ScopeID))
	}
	
	return nil
}

// updateDeltaPosition handles position updates for delta items
func (r *SimpleEnhancedRepository) updateDeltaPosition(ctx context.Context, tx *sql.Tx, 
	itemID idwrap.IDWrap, listType ListType, position int, contextMeta *ContextMetadata) error {
	
	// For delta items, we need to update the delta's position while preserving relationships
	relation := r.getDeltaRelation(itemID)
	if relation == nil {
		return fmt.Errorf("delta relation not found for item %s", itemID.String())
	}
	
	// Update position using base repository
	return r.SimpleRepository.UpdatePosition(ctx, tx, itemID, listType, position)
}

// processBatchDeltaUpdates processes multiple delta position updates
func (r *SimpleEnhancedRepository) processBatchDeltaUpdates(ctx context.Context, tx *sql.Tx, 
	updates []ContextualPositionUpdate) error {
	
	// Convert to regular position updates for base repository
	regularUpdates := make([]PositionUpdate, len(updates))
	for i, update := range updates {
		regularUpdates[i] = PositionUpdate{
			ItemID:   update.ItemID,
			ListType: update.ListType,
			Position: update.Position,
		}
	}
	
	return r.SimpleRepository.UpdatePositions(ctx, tx, regularUpdates)
}

// shouldIncludeInContext determines if item should be included based on context
func (r *SimpleEnhancedRepository) shouldIncludeInContext(itemID idwrap.IDWrap, contextMeta *ContextMetadata) bool {
	// Include if not hidden
	if contextMeta.IsHidden {
		return false
	}
	
	// Additional context-specific filtering could be added here
	return true
}

// getDeltaRelation retrieves delta relation for an item
func (r *SimpleEnhancedRepository) getDeltaRelation(itemID idwrap.IDWrap) *DeltaRelation {
	r.deltasMutex.RLock()
	defer r.deltasMutex.RUnlock()
	
	relation, exists := r.deltaRelations[itemID.String()]
	if exists && relation.IsActive {
		return relation
	}
	return nil
}

// findApplicableDeltas finds deltas applicable to an origin item in a context
func (r *SimpleEnhancedRepository) findApplicableDeltas(originID idwrap.IDWrap, 
	contextMeta *ContextMetadata) []DeltaRelation {
	
	r.deltasMutex.RLock()
	defer r.deltasMutex.RUnlock()
	
	applicable := make([]DeltaRelation, 0)
	originIDStr := originID.String()
	
	for _, relation := range r.deltaRelations {
		if relation.OriginID.String() == originIDStr && relation.IsActive {
			// Check context compatibility
			if contextMeta != nil && relation.Context == contextMeta.Type {
				// Check scope compatibility if available
				if string(relation.ScopeID.Bytes()) == string(contextMeta.ScopeID) {
					applicable = append(applicable, *relation)
				}
			} else if contextMeta == nil {
				// Include all if no context specified
				applicable = append(applicable, *relation)
			}
		}
	}
	
	return applicable
}

// getItemByID retrieves an item by ID (helper method)
func (r *SimpleEnhancedRepository) getItemByID(ctx context.Context, itemID idwrap.IDWrap) (Item, error) {
	// This would typically use the configured entity operations
	// For now, return a placeholder error
	return nil, fmt.Errorf("getItemByID not implemented - would use config.EntityOperations.GetEntity")
}

// processDeltaBatch processes a batch of delta relations
func (r *SimpleEnhancedRepository) processDeltaBatch(ctx context.Context, tx *sql.Tx, 
	relations []DeltaRelation) error {
	
	r.deltasMutex.Lock()
	defer r.deltasMutex.Unlock()
	
	for _, relation := range relations {
		// Validate and store each relation
		if err := r.validateDeltaRelation(&relation); err != nil {
			return fmt.Errorf("invalid delta relation for %s: %w", relation.DeltaID.String(), err)
		}
		
		r.deltaRelations[relation.DeltaID.String()] = &relation
		
		// Create context metadata
		originIDBytes := relation.OriginID.Bytes()
		contextMeta := &ContextMetadata{
			Type:      relation.Context,
			ScopeID:   relation.ScopeID.Bytes(),
			IsHidden:  relation.RelationType == DeltaRelationSuppression,
			OriginID:  &originIDBytes,
			Priority:  relation.Priority,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		
		r.contextCache.SetContext(relation.DeltaID, contextMeta)
	}
	
	return nil
}

// compactDeltaGroup compacts positions for a group of deltas
func (r *SimpleEnhancedRepository) compactDeltaGroup(ctx context.Context, tx *sql.Tx, 
	parentID idwrap.IDWrap, deltas []DeltaRelation, listType ListType) error {
	
	// Get all items for the parent
	_, err := r.SimpleRepository.GetItemsByParent(ctx, parentID, listType)
	if err != nil {
		return fmt.Errorf("failed to get items for parent %s: %w", parentID.String(), err)
	}
	
	// Rebalance positions using pure functions
	// This would use the rebalancing functions from pure_functions.go
	// For now, just ensure positions are sequential
	
	updates := make([]PositionUpdate, 0, len(deltas))
	for i, delta := range deltas {
		updates = append(updates, PositionUpdate{
			ItemID:   delta.DeltaID,
			ListType: listType,
			Position: i, // Sequential positions
		})
	}
	
	return r.SimpleRepository.UpdatePositions(ctx, tx, updates)
}

// validateDeltaRelation validates a delta relation
func (r *SimpleEnhancedRepository) validateDeltaRelation(relation *DeltaRelation) error {
	emptyID := idwrap.IDWrap{}
	
	if relation.DeltaID.Compare(emptyID) == 0 {
		return fmt.Errorf("delta ID is required")
	}
	
	if relation.OriginID.Compare(emptyID) == 0 {
		return fmt.Errorf("origin ID is required")
	}
	
	if relation.ScopeID.Compare(emptyID) == 0 {
		return fmt.Errorf("scope ID is required")
	}
	
	if relation.Priority < 0 {
		return fmt.Errorf("priority must be non-negative")
	}
	
	return nil
}

// trackPerformance tracks performance metrics
func (r *SimpleEnhancedRepository) trackPerformance(operation string, startTime time.Time) {
	duration := time.Since(startTime)
	
	r.performanceMetrics.mutex.Lock()
	r.performanceMetrics.TotalOperations++
	
	// Update average latency (simple moving average)
	if r.performanceMetrics.AverageLatency == 0 {
		r.performanceMetrics.AverageLatency = duration
	} else {
		r.performanceMetrics.AverageLatency = (r.performanceMetrics.AverageLatency + duration) / 2
	}
	
	r.performanceMetrics.mutex.Unlock()
}