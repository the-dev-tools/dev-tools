package movable

import (
	"context"
	"database/sql"
	"sync"
	"time"
	"the-dev-tools/server/pkg/idwrap"
)

// =============================================================================
// CORE CONTEXT TYPES
// =============================================================================

// MovableContext defines the different contexts for ordering operations
type MovableContext int

const (
	ContextCollection MovableContext = iota
	ContextFlow
	ContextEndpoint
	ContextWorkspace
	ContextRequest
)

// String returns the string representation of the context
func (c MovableContext) String() string {
	switch c {
	case ContextCollection:
		return "collection"
	case ContextFlow:
		return "flow"
	case ContextEndpoint:
		return "endpoint"
	case ContextWorkspace:
		return "workspace"
	case ContextRequest:
		return "request"
	default:
		return "unknown"
	}
}

// ContextMetadata provides rich context information for delta operations
type ContextMetadata struct {
	Type      MovableContext
	ScopeID   []byte
	IsHidden  bool
	OriginID  *[]byte // Reference to original item this delta overrides
	Priority  int     // Priority for delta resolution
	CreatedAt time.Time
	UpdatedAt time.Time
}

// =============================================================================
// DELTA RELATIONSHIP TRACKING
// =============================================================================

// DeltaRelation represents the relationship between an original item and its delta
type DeltaRelation struct {
	DeltaID     idwrap.IDWrap
	OriginID    idwrap.IDWrap
	Context     MovableContext
	ScopeID     idwrap.IDWrap
	RelationType DeltaRelationType
	Priority    int
	IsActive    bool
}

// DeltaRelationType defines the type of relationship between delta and origin
type DeltaRelationType int

const (
	DeltaRelationOverride DeltaRelationType = iota
	DeltaRelationExtension
	DeltaRelationSuppression
	DeltaRelationCustomization
)

// String returns the string representation of the relation type
func (r DeltaRelationType) String() string {
	switch r {
	case DeltaRelationOverride:
		return "override"
	case DeltaRelationExtension:
		return "extension"
	case DeltaRelationSuppression:
		return "suppression"
	case DeltaRelationCustomization:
		return "customization"
	default:
		return "unknown"
	}
}

// =============================================================================
// SCOPE RESOLUTION
// =============================================================================

// ScopeResolver determines the appropriate context and scope for operations
type ScopeResolver interface {
	// ResolveContext determines the context for a given item
	ResolveContext(ctx context.Context, itemID idwrap.IDWrap) (MovableContext, error)
	
	// ResolveScopeID determines the scope ID for ordering operations
	ResolveScopeID(ctx context.Context, itemID idwrap.IDWrap, contextType MovableContext) (idwrap.IDWrap, error)
	
	// ValidateScope ensures an item belongs to the expected scope
	ValidateScope(ctx context.Context, itemID idwrap.IDWrap, expectedScope idwrap.IDWrap) error
	
	// GetScopeHierarchy returns the hierarchical scopes for an item
	GetScopeHierarchy(ctx context.Context, itemID idwrap.IDWrap) ([]ScopeLevel, error)
}

// ScopeLevel represents a level in the scope hierarchy
type ScopeLevel struct {
	Context MovableContext
	ScopeID idwrap.IDWrap
	Name    string
	Level   int
}

// =============================================================================
// CONTEXT CACHE FOR PERFORMANCE
// =============================================================================

// ContextCache provides cached access to context information
type ContextCache interface {
	// GetContext returns cached context information
	GetContext(itemID idwrap.IDWrap) (*ContextMetadata, bool)
	
	// SetContext stores context information in cache
	SetContext(itemID idwrap.IDWrap, metadata *ContextMetadata)
	
	// InvalidateContext removes cached context for an item
	InvalidateContext(itemID idwrap.IDWrap)
	
	// InvalidateScope removes all cached contexts for a scope
	InvalidateScope(scopeID idwrap.IDWrap)
	
	// Clear removes all cached contexts
	Clear()
}

// InMemoryContextCache provides a thread-safe in-memory cache implementation
type InMemoryContextCache struct {
	mu    sync.RWMutex
	cache map[string]*ContextMetadata
	ttl   time.Duration
}

// NewInMemoryContextCache creates a new in-memory context cache
func NewInMemoryContextCache(ttl time.Duration) *InMemoryContextCache {
	return &InMemoryContextCache{
		cache: make(map[string]*ContextMetadata),
		ttl:   ttl,
	}
}

// GetContext returns cached context information
func (c *InMemoryContextCache) GetContext(itemID idwrap.IDWrap) (*ContextMetadata, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	metadata, exists := c.cache[itemID.String()]
	if !exists {
		return nil, false
	}
	
	// Check TTL expiration
	if c.ttl > 0 && time.Since(metadata.UpdatedAt) > c.ttl {
		return nil, false
	}
	
	return metadata, true
}

// SetContext stores context information in cache
func (c *InMemoryContextCache) SetContext(itemID idwrap.IDWrap, metadata *ContextMetadata) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	metadata.UpdatedAt = time.Now()
	c.cache[itemID.String()] = metadata
}

// InvalidateContext removes cached context for an item
func (c *InMemoryContextCache) InvalidateContext(itemID idwrap.IDWrap) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	delete(c.cache, itemID.String())
}

// InvalidateScope removes all cached contexts for a scope
func (c *InMemoryContextCache) InvalidateScope(scopeID idwrap.IDWrap) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	scopeIDStr := string(scopeID.Bytes())
	for key, metadata := range c.cache {
		if string(metadata.ScopeID) == scopeIDStr {
			delete(c.cache, key)
		}
	}
}

// Clear removes all cached contexts
func (c *InMemoryContextCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.cache = make(map[string]*ContextMetadata)
}

// =============================================================================
// ENHANCED MOVABLE REPOSITORY INTERFACE
// =============================================================================

// EnhancedMovableRepository extends MovableRepository with context-aware operations
// This interface maintains full backward compatibility - all existing methods work unchanged
type EnhancedMovableRepository interface {
	// Embed existing interface for backward compatibility
	MovableRepository
	
	// === CONTEXT-AWARE OPERATIONS ===
	
	// UpdatePositionWithContext updates position with context awareness for delta operations
	UpdatePositionWithContext(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, 
		listType ListType, position int, contextMeta *ContextMetadata) error
	
	// UpdatePositionsWithContext updates multiple positions with context awareness
	UpdatePositionsWithContext(ctx context.Context, tx *sql.Tx, updates []ContextualPositionUpdate) error
	
	// GetItemsByParentWithContext returns items filtered by context and delta visibility
	GetItemsByParentWithContext(ctx context.Context, parentID idwrap.IDWrap, 
		listType ListType, contextMeta *ContextMetadata) ([]ContextualMovableItem, error)
	
	// === DELTA-SPECIFIC OPERATIONS ===
	
	// CreateDelta creates a new delta relationship for an existing item
	CreateDelta(ctx context.Context, tx *sql.Tx, originID idwrap.IDWrap, 
		deltaID idwrap.IDWrap, relation *DeltaRelation) error
	
	// ResolveDelta resolves the effective item considering delta overrides
	ResolveDelta(ctx context.Context, originID idwrap.IDWrap, 
		contextMeta *ContextMetadata) (*ResolvedItem, error)
	
	// GetDeltasByOrigin returns all deltas for an original item
	GetDeltasByOrigin(ctx context.Context, originID idwrap.IDWrap) ([]DeltaRelation, error)
	
	// GetDeltasByScope returns all deltas within a specific scope
	GetDeltasByScope(ctx context.Context, scopeID idwrap.IDWrap, 
		contextType MovableContext) ([]DeltaRelation, error)
	
	// === SCOPE MANAGEMENT ===
	
	// ValidateContextScope ensures an item belongs to the expected context and scope
	ValidateContextScope(ctx context.Context, itemID idwrap.IDWrap, 
		expectedContext MovableContext, expectedScope idwrap.IDWrap) error
	
	// GetEffectiveOrder returns the resolved order considering all delta overrides
	GetEffectiveOrder(ctx context.Context, parentID idwrap.IDWrap, 
		listType ListType, contextMeta *ContextMetadata) ([]ResolvedItem, error)
	
	// === PERFORMANCE OPERATIONS ===
	
	// BatchCreateDeltas efficiently creates multiple delta relationships
	BatchCreateDeltas(ctx context.Context, tx *sql.Tx, relations []DeltaRelation) error
	
	// BatchResolveDeltas efficiently resolves multiple items with their deltas
	BatchResolveDeltas(ctx context.Context, originIDs []idwrap.IDWrap, 
		contextMeta *ContextMetadata) (map[idwrap.IDWrap]*ResolvedItem, error)
	
	// CompactDeltaPositions rebalances positions within a delta context
	CompactDeltaPositions(ctx context.Context, tx *sql.Tx, scopeID idwrap.IDWrap, 
		contextType MovableContext, listType ListType) error
}

// =============================================================================
// CONTEXTUAL DATA TYPES
// =============================================================================

// ContextualPositionUpdate extends PositionUpdate with context information
type ContextualPositionUpdate struct {
	ItemID       idwrap.IDWrap
	ListType     ListType
	Position     int
	Context      *ContextMetadata
	IsDelta      bool
	OriginID     *idwrap.IDWrap // Set if this is a delta update
}

// ContextualMovableItem extends MovableItem with context and delta information
type ContextualMovableItem struct {
	ID           idwrap.IDWrap
	ParentID     *idwrap.IDWrap
	Position     int
	ListType     ListType
	Context      *ContextMetadata
	IsDelta      bool
	OriginID     *idwrap.IDWrap
	DeltaRelation *DeltaRelation
}

// ResolvedItem represents an item after delta resolution
type ResolvedItem struct {
	ID              idwrap.IDWrap
	EffectiveID     idwrap.IDWrap // The ID to use (delta if exists, origin otherwise)
	OriginID        *idwrap.IDWrap
	DeltaID         *idwrap.IDWrap
	ParentID        *idwrap.IDWrap
	Position        int
	ListType        ListType
	Context         *ContextMetadata
	ResolutionType  ResolutionType
	ResolutionTime  time.Time
	AppliedDeltas   []DeltaRelation
}

// ResolutionType indicates how an item was resolved
type ResolutionType int

const (
	ResolutionOrigin ResolutionType = iota
	ResolutionDelta
	ResolutionMerged
	ResolutionSuppressed
)

// String returns the string representation of the resolution type
func (r ResolutionType) String() string {
	switch r {
	case ResolutionOrigin:
		return "origin"
	case ResolutionDelta:
		return "delta"
	case ResolutionMerged:
		return "merged"
	case ResolutionSuppressed:
		return "suppressed"
	default:
		return "unknown"
	}
}

// =============================================================================
// ENHANCED TRANSACTION SUPPORT
// =============================================================================

// TransactionAwareEnhancedRepository extends TransactionAwareRepository with enhanced features
type TransactionAwareEnhancedRepository interface {
	EnhancedMovableRepository
	
	// TX returns an enhanced repository instance with transaction support
	TX(tx *sql.Tx) EnhancedMovableRepository
	
	// WithContext returns a repository instance with cached context resolver
	WithContext(resolver ScopeResolver, cache ContextCache) EnhancedMovableRepository
}

// =============================================================================
// CONTEXT-AWARE MOVABLE INTERFACE
// =============================================================================

// ContextAwareMovable extends Movable with context awareness
type ContextAwareMovable interface {
	Movable
	
	// GetContext returns the context metadata for this item
	GetContext() *ContextMetadata
	
	// GetDeltaRelation returns delta relation if this is a delta item
	GetDeltaRelation() *DeltaRelation
	
	// IsHidden returns true if this item should be hidden in the current context
	IsHidden(contextType MovableContext) bool
	
	// GetEffectiveParent returns the effective parent considering context
	GetEffectiveParent(contextType MovableContext) (*idwrap.IDWrap, error)
	
	// ResolveWithDeltas returns the resolved version considering all applicable deltas
	ResolveWithDeltas(ctx context.Context, resolver ScopeResolver) (*ResolvedItem, error)
}

// =============================================================================
// DELTA OPERATION RESULTS
// =============================================================================

// DeltaMoveResult extends MoveResult with delta-specific information
type DeltaMoveResult struct {
	*MoveResult
	AffectedDeltas    []DeltaRelation
	ResolvedItems     []ResolvedItem
	InvalidatedCache  []idwrap.IDWrap
	ContextChanges    map[idwrap.IDWrap]*ContextMetadata
}

// DeltaValidationResult contains validation results for delta operations
type DeltaValidationResult struct {
	IsValid          bool
	Errors           []error
	Warnings         []string
	ConflictingDeltas []DeltaRelation
	ScopeViolations  []ScopeViolation
}

// ScopeViolation represents a scope boundary violation
type ScopeViolation struct {
	ItemID        idwrap.IDWrap
	ExpectedScope idwrap.IDWrap
	ActualScope   idwrap.IDWrap
	ViolationType string
}

// =============================================================================
// BATCH OPERATIONS FOR PERFORMANCE
// =============================================================================

// BatchDeltaOperation represents a batch operation on deltas
type BatchDeltaOperation struct {
	Type       BatchDeltaOperationType
	Relations  []DeltaRelation
	Updates    []ContextualPositionUpdate
	Context    *ContextMetadata
	TargetScope idwrap.IDWrap
}

// BatchDeltaOperationType defines types of batch operations
type BatchDeltaOperationType int

const (
	BatchDeltaCreate BatchDeltaOperationType = iota
	BatchDeltaUpdate
	BatchDeltaDelete
	BatchDeltaReorder
	BatchDeltaResolve
)

// String returns the string representation of the batch operation type
func (b BatchDeltaOperationType) String() string {
	switch b {
	case BatchDeltaCreate:
		return "create"
	case BatchDeltaUpdate:
		return "update"
	case BatchDeltaDelete:
		return "delete"
	case BatchDeltaReorder:
		return "reorder"
	case BatchDeltaResolve:
		return "resolve"
	default:
		return "unknown"
	}
}

// BatchDeltaResult contains results of batch delta operations
type BatchDeltaResult struct {
	Success           bool
	ProcessedCount    int
	FailedCount       int
	CreatedRelations  []DeltaRelation
	UpdatedItems      []ResolvedItem
	InvalidatedCache  []idwrap.IDWrap
	ExecutionTime     time.Duration
	Errors           []error
}

// =============================================================================
// UTILITY INTERFACES FOR EXTENSION
// =============================================================================

// DeltaLifecycleHandler handles delta lifecycle events
type DeltaLifecycleHandler interface {
	OnDeltaCreated(ctx context.Context, relation *DeltaRelation) error
	OnDeltaUpdated(ctx context.Context, oldRelation, newRelation *DeltaRelation) error
	OnDeltaDeleted(ctx context.Context, relation *DeltaRelation) error
	OnDeltaResolved(ctx context.Context, result *ResolvedItem) error
}

// ContextMigrator helps migrate between different context versions
type ContextMigrator interface {
	MigrateContext(ctx context.Context, itemID idwrap.IDWrap, 
		fromContext, toContext MovableContext) error
	ValidateMigration(ctx context.Context, itemID idwrap.IDWrap, 
		targetContext MovableContext) (*DeltaValidationResult, error)
}

// =============================================================================
// CONFIGURATION AND FACTORY TYPES
// =============================================================================

// EnhancedRepositoryConfig extends repository configuration with context-aware features
type EnhancedRepositoryConfig struct {
	*MoveConfig
	
	// Context resolution
	ScopeResolver ScopeResolver
	ContextCache  ContextCache
	
	// Delta handling
	DeltaHandler      DeltaLifecycleHandler
	ContextMigrator   ContextMigrator
	
	// Performance tuning
	CacheTTL          time.Duration
	BatchSize         int
	EnablePrefetch    bool
	
	// Feature flags
	EnableContextAware bool
	EnableDeltaSupport bool
	EnableScopeValidation bool
}

// RepositoryFactory creates repository instances with appropriate configuration
type RepositoryFactory interface {
	CreateSimpleRepository(db *sql.DB, config *MoveConfig) MovableRepository
	CreateEnhancedRepository(db *sql.DB, config *EnhancedRepositoryConfig) EnhancedMovableRepository
	CreateTransactionAware(repo EnhancedMovableRepository) TransactionAwareEnhancedRepository
}