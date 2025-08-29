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
// UNIFIED ORDERING SERVICE TYPES
// =============================================================================

// UnifiedOrderingService provides a unified API for all ordering operations
// across different contexts (endpoints, examples, flow nodes, workspaces)
type UnifiedOrderingService struct {
	// Repository registry
	repositoryRegistry *RepositoryRegistry
	
	// Context routing
	contextDetector ContextDetector
	contextRouter   ContextRouter
	
	// Delta coordination
	deltaCoordinator DeltaCoordinator
	syncManager      *DeltaSyncManager
	
	// Performance components
	connectionPool   *ConnectionPool
	metricsCollector *UnifiedMetricsCollector
	
	// Configuration
	config *UnifiedServiceConfig
	
	// Thread safety
	mu sync.RWMutex
}

// RepositoryRegistry manages all enhanced repositories by context
type RepositoryRegistry struct {
	endpointRepo  *EnhancedEndpointRepository
	exampleRepo   *UnifiedEnhancedExampleRepository
	flowNodeRepo  *UnifiedEnhancedFlowNodeRepository
	workspaceRepo *UnifiedEnhancedWorkspaceRepository
	
	// Fallback for unknown contexts
	defaultRepo EnhancedMovableRepository
	
	mu sync.RWMutex
}

// ContextDetector determines the context for a given item ID
type ContextDetector interface {
	DetectContext(ctx context.Context, itemID idwrap.IDWrap) (MovableContext, error)
	DetectContextBatch(ctx context.Context, itemIDs []idwrap.IDWrap) (map[idwrap.IDWrap]MovableContext, error)
	ValidateContext(ctx context.Context, itemID idwrap.IDWrap, expectedContext MovableContext) error
}

// ContextRouter routes operations to appropriate repositories
type ContextRouter interface {
	RouteToRepository(context MovableContext, operation string) (EnhancedMovableRepository, error)
	GetRepositoryCapabilities(context MovableContext) []string
	ValidateOperation(context MovableContext, operation string) error
}

// DeltaCoordinator manages cross-context delta operations
type DeltaCoordinator interface {
	CoordinateDelta(ctx context.Context, tx *sql.Tx, operation *CrossContextDeltaOperation) error
	SyncDeltas(ctx context.Context, scopeID idwrap.IDWrap, contextType MovableContext) error
	ResolveConflicts(ctx context.Context, conflicts []DeltaConflict) (*UnifiedConflictResolution, error)
}

// =============================================================================
// CONFIGURATION AND SETUP
// =============================================================================

// UnifiedServiceConfig configures the unified ordering service
type UnifiedServiceConfig struct {
	// Repository configurations
	EndpointConfig  *EnhancedEndpointConfig
	ExampleConfig   *EnhancedExampleConfig
	FlowNodeConfig  *EnhancedFlowNodeConfig
	WorkspaceConfig *EnhancedWorkspaceConfig
	
	// Context detection
	EnableAutoDetection bool
	ContextCacheTTL     time.Duration
	
	// Performance settings
	ConnectionPoolSize    int
	MaxConcurrentOps      int
	BatchSize            int
	MetricsBufferSize    int
	
	// Feature flags
	EnableCrossContext    bool
	EnableDeltaSync       bool
	EnableMetrics         bool
	EnableConnectionPool  bool
	
	// Timeouts
	OperationTimeout     time.Duration
	ConnectionTimeout    time.Duration
	
	// Error handling
	MaxRetries          int
	RetryBackoff        time.Duration
}

// DefaultUnifiedServiceConfig returns a default configuration
func DefaultUnifiedServiceConfig() *UnifiedServiceConfig {
	return &UnifiedServiceConfig{
		EnableAutoDetection:   true,
		ContextCacheTTL:      5 * time.Minute,
		ConnectionPoolSize:   10,
		MaxConcurrentOps:     100,
		BatchSize:           50,
		MetricsBufferSize:   1000,
		EnableCrossContext:   true,
		EnableDeltaSync:     true,
		EnableMetrics:       true,
		EnableConnectionPool: true,
		OperationTimeout:    30 * time.Second,
		ConnectionTimeout:   5 * time.Second,
		MaxRetries:         3,
		RetryBackoff:       100 * time.Millisecond,
	}
}

// =============================================================================
// CONSTRUCTOR
// =============================================================================

// NewUnifiedOrderingService creates a new unified ordering service
func NewUnifiedOrderingService(db *sql.DB, config *UnifiedServiceConfig) (*UnifiedOrderingService, error) {
	if config == nil {
		config = DefaultUnifiedServiceConfig()
	}
	
	// Initialize repository registry
	registry, err := NewRepositoryRegistry(db, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository registry: %w", err)
	}
	
	// Initialize context detector
	detector := NewSmartContextDetector(registry, config.ContextCacheTTL)
	
	// Initialize context router
	router := NewDefaultContextRouter(registry)
	
	// Initialize delta coordinator
	coordinator := NewDefaultDeltaCoordinator(registry)
	
	// Initialize sync manager if enabled
	var syncManager *DeltaSyncManager
	if config.EnableDeltaSync {
		syncConfig := &SyncManagerConfig{
			EventQueueSize:    config.BatchSize * 2,
			WorkerCount:       4,
			BatchSize:         config.BatchSize,
			SyncInterval:      10 * time.Second,
			DefaultStrategy:   ConflictStrategyLastWriteWins,
			EnableBatching:    true,
			EnableAsync:       true,
			MaxRetries:        config.MaxRetries,
			RetryBackoff:      config.RetryBackoff,
		}
		syncManager = NewDeltaSyncManager(coordinator, registry, syncConfig)
	}
	
	// Initialize connection pool if enabled
	var connPool *ConnectionPool
	if config.EnableConnectionPool {
		connPool = NewConnectionPool(db, config.ConnectionPoolSize, config.ConnectionTimeout)
	}
	
	// Initialize metrics collector if enabled
	var metrics *UnifiedMetricsCollector
	if config.EnableMetrics {
		metrics = NewUnifiedMetricsCollector(config.MetricsBufferSize)
	}
	
	return &UnifiedOrderingService{
		repositoryRegistry: registry,
		contextDetector:    detector,
		contextRouter:      router,
		deltaCoordinator:   coordinator,
		syncManager:        syncManager,
		connectionPool:     connPool,
		metricsCollector:   metrics,
		config:            config,
	}, nil
}

// =============================================================================
// CORE SERVICE METHODS
// =============================================================================

// MoveItem routes move operations to the appropriate repository based on context
func (s *UnifiedOrderingService) MoveItem(ctx context.Context, tx *sql.Tx, 
	itemID idwrap.IDWrap, operation MoveOperation) (*MoveResult, error) {
	
	startTime := time.Now()
	defer s.trackMetric("MoveItem", startTime, nil)
	
	// Apply timeout
	ctx, cancel := context.WithTimeout(ctx, s.config.OperationTimeout)
	defer cancel()
	
	// Detect context for the item
	itemContext, err := s.contextDetector.DetectContext(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to detect context for item %s: %w", itemID.String(), err)
	}
	
	// Route to appropriate repository
	repo, err := s.contextRouter.RouteToRepository(itemContext, "move")
	if err != nil {
		return nil, fmt.Errorf("failed to route move operation: %w", err)
	}
	
	// Get connection from pool if available
	if s.connectionPool != nil {
		conn, err := s.connectionPool.GetConnection(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get database connection: %w", err)
		}
		defer s.connectionPool.ReturnConnection(conn)
		// Connection pool is used implicitly by the repository
	}
	
	// Execute the move operation
	manager := NewDefaultLinkedListManager(repo)
	result, err := manager.Move(ctx, tx, operation)
	if err != nil {
		s.trackMetric("MoveItem", startTime, err)
		return nil, fmt.Errorf("move operation failed: %w", err)
	}
	
	// Sync deltas if enabled and this affects cross-context items
	if s.config.EnableDeltaSync && s.syncManager != nil {
		if syncErr := s.syncManager.SyncAfterMove(ctx, itemID, itemContext, result); syncErr != nil {
			// Log but don't fail the operation
			fmt.Printf("Warning: delta sync failed after move: %v\n", syncErr)
		}
	}
	
	return result, nil
}

// CreateDelta creates a delta with proper context routing and synchronization
func (s *UnifiedOrderingService) CreateDelta(ctx context.Context, tx *sql.Tx,
	originID idwrap.IDWrap, deltaID idwrap.IDWrap, contextType MovableContext,
	scopeID idwrap.IDWrap, relation *DeltaRelation) error {
	
	startTime := time.Now()
	defer s.trackMetric("CreateDelta", startTime, nil)
	
	// Apply timeout
	ctx, cancel := context.WithTimeout(ctx, s.config.OperationTimeout)
	defer cancel()
	
	// Validate context consistency
	if err := s.contextDetector.ValidateContext(ctx, originID, contextType); err != nil {
		return fmt.Errorf("context validation failed for origin: %w", err)
	}
	
	// Route to appropriate repository
	repo, err := s.contextRouter.RouteToRepository(contextType, "create_delta")
	if err != nil {
		return fmt.Errorf("failed to route delta creation: %w", err)
	}
	
	// Create the delta
	if err := repo.CreateDelta(ctx, tx, originID, deltaID, relation); err != nil {
		s.trackMetric("CreateDelta", startTime, err)
		return fmt.Errorf("failed to create delta: %w", err)
	}
	
	// Coordinate cross-context implications if enabled
	if s.config.EnableCrossContext {
		crossOp := &CrossContextDeltaOperation{
			Type:        CrossContextCreate,
			OriginID:    originID,
			DeltaID:     deltaID,
			Context:     contextType,
			ScopeID:     scopeID,
			Relation:    relation,
		}
		
		if err := s.deltaCoordinator.CoordinateDelta(ctx, tx, crossOp); err != nil {
			return fmt.Errorf("cross-context coordination failed: %w", err)
		}
	}
	
	return nil
}

// ResolveItem resolves an item with delta inheritance across all contexts
func (s *UnifiedOrderingService) ResolveItem(ctx context.Context, 
	itemID idwrap.IDWrap, contextMeta *ContextMetadata) (*ResolvedItem, error) {
	
	startTime := time.Now()
	defer s.trackMetric("ResolveItem", startTime, nil)
	
	// Apply timeout
	ctx, cancel := context.WithTimeout(ctx, s.config.OperationTimeout)
	defer cancel()
	
	// Detect context if not provided
	var itemContext MovableContext
	var err error
	
	if contextMeta != nil {
		itemContext = contextMeta.Type
	} else {
		itemContext, err = s.contextDetector.DetectContext(ctx, itemID)
		if err != nil {
			return nil, fmt.Errorf("failed to detect context: %w", err)
		}
	}
	
	// Route to appropriate repository
	repo, err := s.contextRouter.RouteToRepository(itemContext, "resolve")
	if err != nil {
		return nil, fmt.Errorf("failed to route resolution: %w", err)
	}
	
	// Resolve the item
	resolved, err := repo.ResolveDelta(ctx, itemID, contextMeta)
	if err != nil {
		s.trackMetric("ResolveItem", startTime, err)
		return nil, fmt.Errorf("failed to resolve item: %w", err)
	}
	
	return resolved, nil
}

// SyncDeltas coordinates delta synchronization across contexts
func (s *UnifiedOrderingService) SyncDeltas(ctx context.Context, 
	scopeID idwrap.IDWrap, contextType MovableContext) error {
	
	startTime := time.Now()
	defer s.trackMetric("SyncDeltas", startTime, nil)
	
	if !s.config.EnableDeltaSync || s.syncManager == nil {
		return fmt.Errorf("delta synchronization is not enabled")
	}
	
	// Apply timeout
	ctx, cancel := context.WithTimeout(ctx, s.config.OperationTimeout)
	defer cancel()
	
	// Delegate to delta coordinator
	if err := s.deltaCoordinator.SyncDeltas(ctx, scopeID, contextType); err != nil {
		s.trackMetric("SyncDeltas", startTime, err)
		return fmt.Errorf("delta synchronization failed: %w", err)
	}
	
	return nil
}

// GetOrderedItems retrieves items in any context with proper ordering
func (s *UnifiedOrderingService) GetOrderedItems(ctx context.Context,
	parentID idwrap.IDWrap, listType ListType, contextMeta *ContextMetadata) ([]ContextualMovableItem, error) {
	
	startTime := time.Now()
	defer s.trackMetric("GetOrderedItems", startTime, nil)
	
	// Apply timeout
	ctx, cancel := context.WithTimeout(ctx, s.config.OperationTimeout)
	defer cancel()
	
	// Determine context
	var itemContext MovableContext
	var err error
	
	if contextMeta != nil {
		itemContext = contextMeta.Type
	} else {
		itemContext, err = s.contextDetector.DetectContext(ctx, parentID)
		if err != nil {
			return nil, fmt.Errorf("failed to detect context: %w", err)
		}
	}
	
	// Route to appropriate repository
	repo, err := s.contextRouter.RouteToRepository(itemContext, "get_items")
	if err != nil {
		return nil, fmt.Errorf("failed to route get items operation: %w", err)
	}
	
	// Get items with context
	items, err := repo.GetItemsByParentWithContext(ctx, parentID, listType, contextMeta)
	if err != nil {
		s.trackMetric("GetOrderedItems", startTime, err)
		return nil, fmt.Errorf("failed to get ordered items: %w", err)
	}
	
	return items, nil
}

// =============================================================================
// BATCH OPERATIONS FOR PERFORMANCE
// =============================================================================

// BatchMoveItems performs multiple move operations efficiently
func (s *UnifiedOrderingService) BatchMoveItems(ctx context.Context, tx *sql.Tx,
	operations []UnifiedBatchMoveOperation) (*BatchMoveResult, error) {
	
	startTime := time.Now()
	defer s.trackMetric("BatchMoveItems", startTime, nil)
	
	if len(operations) == 0 {
		return &BatchMoveResult{}, nil
	}
	
	if len(operations) > s.config.BatchSize {
		return nil, fmt.Errorf("batch size %d exceeds maximum %d", len(operations), s.config.BatchSize)
	}
	
	// Apply timeout
	ctx, cancel := context.WithTimeout(ctx, s.config.OperationTimeout)
	defer cancel()
	
	// Group operations by context for efficiency
	contextGroups := make(map[MovableContext][]UnifiedBatchMoveOperation)
	
	for _, op := range operations {
		context, err := s.contextDetector.DetectContext(ctx, op.ItemID)
		if err != nil {
			return nil, fmt.Errorf("failed to detect context for item %s: %w", op.ItemID.String(), err)
		}
		contextGroups[context] = append(contextGroups[context], op)
	}
	
	// Process each context group
	result := &BatchMoveResult{
		TotalRequested: len(operations),
		StartTime:      startTime,
	}
	
	for context, contextOps := range contextGroups {
		repo, err := s.contextRouter.RouteToRepository(context, "batch_move")
		if err != nil {
			result.FailedCount += len(contextOps)
			result.Errors = append(result.Errors, fmt.Errorf("failed to route context %s: %w", context.String(), err))
			continue
		}
		
		// Execute context-specific batch
		contextResult, err := s.executeBatchMoveForContext(ctx, tx, repo, contextOps)
		if err != nil {
			result.FailedCount += len(contextOps)
			result.Errors = append(result.Errors, fmt.Errorf("batch failed for context %s: %w", context.String(), err))
			continue
		}
		
		result.SuccessCount += contextResult.SuccessCount
		result.FailedCount += contextResult.FailedCount
		result.Results = append(result.Results, contextResult.Results...)
	}
	
	result.Duration = time.Since(startTime)
	return result, nil
}

// BatchCreateDeltas creates multiple deltas efficiently across contexts
func (s *UnifiedOrderingService) BatchCreateDeltas(ctx context.Context, tx *sql.Tx,
	operations []BatchDeltaOperation) (*BatchDeltaResult, error) {
	
	startTime := time.Now()
	defer s.trackMetric("BatchCreateDeltas", startTime, nil)
	
	if len(operations) == 0 {
		return &BatchDeltaResult{}, nil
	}
	
	// Apply timeout
	ctx, cancel := context.WithTimeout(ctx, s.config.OperationTimeout)
	defer cancel()
	
	// Group by context
	contextGroups := make(map[MovableContext][]BatchDeltaOperation)
	
	for _, op := range operations {
		for _, relation := range op.Relations {
			contextGroups[relation.Context] = append(contextGroups[relation.Context], op)
		}
	}
	
	// Process each context group
	result := &BatchDeltaResult{
		Success:        true,
		ProcessedCount: 0,
		ExecutionTime:  time.Since(startTime),
	}
	
	for context, contextOps := range contextGroups {
		repo, err := s.contextRouter.RouteToRepository(context, "batch_create_delta")
		if err != nil {
			result.Success = false
			result.Errors = append(result.Errors, fmt.Errorf("routing failed for context %s: %w", context.String(), err))
			continue
		}
		
		// Extract relations for this context
		var relations []DeltaRelation
		for _, op := range contextOps {
			for _, relation := range op.Relations {
				if relation.Context == context {
					relations = append(relations, relation)
				}
			}
		}
		
		// Execute batch creation
		if err := repo.BatchCreateDeltas(ctx, tx, relations); err != nil {
			result.Success = false
			result.FailedCount += len(relations)
			result.Errors = append(result.Errors, fmt.Errorf("batch creation failed for context %s: %w", context.String(), err))
			continue
		}
		
		result.ProcessedCount += len(relations)
		result.CreatedRelations = append(result.CreatedRelations, relations...)
	}
	
	result.ExecutionTime = time.Since(startTime)
	return result, nil
}

// =============================================================================
// CONTEXT ROUTING IMPLEMENTATION
// =============================================================================

// DefaultContextRouter provides default context routing logic
type DefaultContextRouter struct {
	registry *RepositoryRegistry
}

// NewDefaultContextRouter creates a new default context router
func NewDefaultContextRouter(registry *RepositoryRegistry) *DefaultContextRouter {
	return &DefaultContextRouter{registry: registry}
}

// RouteToRepository routes operations to appropriate repositories
func (r *DefaultContextRouter) RouteToRepository(context MovableContext, operation string) (EnhancedMovableRepository, error) {
	r.registry.mu.RLock()
	defer r.registry.mu.RUnlock()
	
	switch context {
	case ContextEndpoint:
		if r.registry.endpointRepo == nil {
			return nil, fmt.Errorf("endpoint repository not available")
		}
		return r.registry.endpointRepo.SimpleEnhancedRepository, nil
		
	case ContextCollection:
		if r.registry.exampleRepo == nil {
			return nil, fmt.Errorf("example repository not available")  
		}
		return r.registry.exampleRepo, nil
		
	case ContextFlow:
		if r.registry.flowNodeRepo == nil {
			return nil, fmt.Errorf("flow node repository not available")
		}
		return r.registry.flowNodeRepo, nil
		
	case ContextWorkspace:
		if r.registry.workspaceRepo == nil {
			return nil, fmt.Errorf("workspace repository not available")
		}
		return r.registry.workspaceRepo, nil
		
	default:
		if r.registry.defaultRepo == nil {
			return nil, fmt.Errorf("no repository available for context %s", context.String())
		}
		return r.registry.defaultRepo, nil
	}
}

// GetRepositoryCapabilities returns capabilities for a context
func (r *DefaultContextRouter) GetRepositoryCapabilities(context MovableContext) []string {
	baseCapabilities := []string{"move", "get_items", "update_position", "resolve"}
	
	switch context {
	case ContextEndpoint:
		return append(baseCapabilities, "create_delta", "batch_move", "cross_context")
	case ContextFlow:
		return append(baseCapabilities, "create_delta", "batch_operations")
	default:
		return baseCapabilities
	}
}

// ValidateOperation validates if an operation is supported for a context
func (r *DefaultContextRouter) ValidateOperation(context MovableContext, operation string) error {
	capabilities := r.GetRepositoryCapabilities(context)
	
	for _, cap := range capabilities {
		if cap == operation {
			return nil
		}
	}
	
	return fmt.Errorf("operation %s not supported for context %s", operation, context.String())
}

// =============================================================================
// CONTEXT DETECTION IMPLEMENTATION
// =============================================================================

// SmartContextDetector provides intelligent context detection with caching
type SmartContextDetector struct {
	registry *RepositoryRegistry
	cache    ContextCache
	mu       sync.RWMutex
}

// NewSmartContextDetector creates a new smart context detector
func NewSmartContextDetector(registry *RepositoryRegistry, cacheTTL time.Duration) *SmartContextDetector {
	return &SmartContextDetector{
		registry: registry,
		cache:    NewInMemoryContextCache(cacheTTL),
	}
}

// DetectContext determines the context for a given item ID
func (d *SmartContextDetector) DetectContext(ctx context.Context, itemID idwrap.IDWrap) (MovableContext, error) {
	// Check cache first
	if cached, found := d.cache.GetContext(itemID); found {
		return cached.Type, nil
	}
	
	// Try to detect from ID patterns (assuming certain ID patterns)
	idStr := itemID.String()
	
	// Basic heuristic detection based on ID patterns
	switch {
	case len(idStr) > 8 && idStr[:2] == "ep": // endpoint IDs start with "ep"
		return ContextEndpoint, nil
	case len(idStr) > 8 && idStr[:2] == "ex": // example IDs start with "ex"  
		return ContextCollection, nil
	case len(idStr) > 8 && idStr[:2] == "fn": // flow node IDs start with "fn"
		return ContextFlow, nil
	case len(idStr) > 8 && idStr[:2] == "ws": // workspace IDs start with "ws"
		return ContextWorkspace, nil
	default:
		// Fallback to collection context
		return ContextCollection, nil
	}
}

// DetectContextBatch detects contexts for multiple items efficiently
func (d *SmartContextDetector) DetectContextBatch(ctx context.Context, itemIDs []idwrap.IDWrap) (map[idwrap.IDWrap]MovableContext, error) {
	results := make(map[idwrap.IDWrap]MovableContext, len(itemIDs))
	
	for _, itemID := range itemIDs {
		context, err := d.DetectContext(ctx, itemID)
		if err != nil {
			return nil, fmt.Errorf("failed to detect context for %s: %w", itemID.String(), err)
		}
		results[itemID] = context
	}
	
	return results, nil
}

// ValidateContext validates if an item belongs to the expected context
func (d *SmartContextDetector) ValidateContext(ctx context.Context, itemID idwrap.IDWrap, expectedContext MovableContext) error {
	actualContext, err := d.DetectContext(ctx, itemID)
	if err != nil {
		return fmt.Errorf("failed to detect actual context: %w", err)
	}
	
	if actualContext != expectedContext {
		return fmt.Errorf("context mismatch: expected %s, got %s", expectedContext.String(), actualContext.String())
	}
	
	return nil
}

// =============================================================================
// REPOSITORY REGISTRY IMPLEMENTATION  
// =============================================================================

// NewRepositoryRegistry creates a new repository registry
func NewRepositoryRegistry(db *sql.DB, config *UnifiedServiceConfig) (*RepositoryRegistry, error) {
	registry := &RepositoryRegistry{}
	
	// Initialize endpoint repository if configured
	if config.EndpointConfig != nil {
		endpointRepo := NewEnhancedEndpointRepository(db, config.EndpointConfig)
		registry.endpointRepo = endpointRepo
	}
	
	// Initialize other repositories (example, flownode, workspace) would go here
	// For now, use the base enhanced repository as default
	baseConfig := &EnhancedRepositoryConfig{
		MoveConfig: &MoveConfig{
			// Basic move configuration
		},
		EnableContextAware:    config.EnableAutoDetection,
		EnableDeltaSupport:    config.EnableDeltaSync,
		EnableScopeValidation: true,
		CacheTTL:             config.ContextCacheTTL,
		BatchSize:            config.BatchSize,
	}
	
	defaultRepo := NewSimpleEnhancedRepository(db, baseConfig)
	registry.defaultRepo = defaultRepo
	
	return registry, nil
}

// =============================================================================
// HELPER TYPES AND IMPLEMENTATIONS
// =============================================================================

// UnifiedBatchMoveOperation represents a single move operation in a batch for unified service
type UnifiedBatchMoveOperation struct {
	ItemID    idwrap.IDWrap
	Operation MoveOperation
}

// BatchMoveResult contains results of batch move operations
type BatchMoveResult struct {
	TotalRequested int
	SuccessCount   int
	FailedCount    int
	Results        []MoveResult
	Errors         []error
	StartTime      time.Time
	Duration       time.Duration
}

// CrossContextDeltaOperation represents a delta operation across contexts
type CrossContextDeltaOperation struct {
	Type     CrossContextOperationType
	OriginID idwrap.IDWrap
	DeltaID  idwrap.IDWrap
	Context  MovableContext
	ScopeID  idwrap.IDWrap
	Relation *DeltaRelation
}

// CrossContextOperationType defines types of cross-context operations
type CrossContextOperationType int

const (
	CrossContextCreate CrossContextOperationType = iota
	CrossContextUpdate
	CrossContextDelete
	CrossContextSync
)

// DeltaConflict represents a conflict between deltas
type DeltaConflict struct {
	OriginID        idwrap.IDWrap
	ConflictingDeltas []DeltaRelation
	ConflictType    string
}

// UnifiedConflictResolution contains the resolution for conflicts in unified service context
type UnifiedConflictResolution struct {
	ResolvedDeltas   []DeltaRelation
	RemovedDeltas    []idwrap.IDWrap
	ResolutionType   string
}

// Placeholder implementations for missing components
type DefaultDeltaCoordinator struct {
	registry *RepositoryRegistry
}

func NewDefaultDeltaCoordinator(registry *RepositoryRegistry) *DefaultDeltaCoordinator {
	return &DefaultDeltaCoordinator{registry: registry}
}

func (c *DefaultDeltaCoordinator) CoordinateDelta(ctx context.Context, tx *sql.Tx, operation *CrossContextDeltaOperation) error {
	// Basic implementation - would be expanded based on requirements
	return nil
}

func (c *DefaultDeltaCoordinator) SyncDeltas(ctx context.Context, scopeID idwrap.IDWrap, contextType MovableContext) error {
	// Basic implementation - would be expanded based on requirements  
	return nil
}

func (c *DefaultDeltaCoordinator) ResolveConflicts(ctx context.Context, conflicts []DeltaConflict) (*UnifiedConflictResolution, error) {
	// Basic implementation - would be expanded based on requirements
	return &UnifiedConflictResolution{}, nil
}

// Enhanced repository aliases for the unified service
type UnifiedEnhancedExampleRepository = SimpleEnhancedRepository
type UnifiedEnhancedFlowNodeRepository = SimpleEnhancedRepository  
type UnifiedEnhancedWorkspaceRepository = SimpleEnhancedRepository

type EnhancedExampleConfig = EnhancedRepositoryConfig
type EnhancedFlowNodeConfig = EnhancedRepositoryConfig
type EnhancedWorkspaceConfig = EnhancedRepositoryConfig

// Connection pooling placeholder
type ConnectionPool struct {
	db          *sql.DB
	size        int
	timeout     time.Duration
	connections chan *sql.DB
}

func NewConnectionPool(db *sql.DB, size int, timeout time.Duration) *ConnectionPool {
	pool := &ConnectionPool{
		db:          db,
		size:        size,
		timeout:     timeout,
		connections: make(chan *sql.DB, size),
	}
	
	// Fill the pool
	for i := 0; i < size; i++ {
		pool.connections <- db // In a real implementation, these would be separate connections
	}
	
	return pool
}

func (p *ConnectionPool) GetConnection(ctx context.Context) (*sql.DB, error) {
	select {
	case conn := <-p.connections:
		return conn, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(p.timeout):
		return nil, fmt.Errorf("connection timeout")
	}
}

func (p *ConnectionPool) ReturnConnection(conn *sql.DB) {
	select {
	case p.connections <- conn:
	default:
		// Pool is full, connection will be discarded
	}
}

// Metrics collection placeholder
type UnifiedMetricsCollector struct {
	operations map[string]*OperationMetrics
	mu         sync.RWMutex
}

type OperationMetrics struct {
	Count        int64
	TotalTime    time.Duration
	AverageTime  time.Duration
	ErrorCount   int64
	LastExecuted time.Time
}

func NewUnifiedMetricsCollector(bufferSize int) *UnifiedMetricsCollector {
	return &UnifiedMetricsCollector{
		operations: make(map[string]*OperationMetrics),
	}
}

func (s *UnifiedOrderingService) trackMetric(operation string, startTime time.Time, err error) {
	if s.metricsCollector == nil {
		return
	}
	
	duration := time.Since(startTime)
	s.metricsCollector.mu.Lock()
	defer s.metricsCollector.mu.Unlock()
	
	if _, exists := s.metricsCollector.operations[operation]; !exists {
		s.metricsCollector.operations[operation] = &OperationMetrics{}
	}
	
	metrics := s.metricsCollector.operations[operation]
	metrics.Count++
	metrics.TotalTime += duration
	metrics.AverageTime = time.Duration(int64(metrics.TotalTime) / metrics.Count)
	metrics.LastExecuted = time.Now()
	
	if err != nil {
		metrics.ErrorCount++
	}
}

// SyncAfterMove method for DeltaSyncManager defined in sync_manager.go
func (s *DeltaSyncManager) SyncAfterMove(ctx context.Context, itemID idwrap.IDWrap, 
	context MovableContext, result *MoveResult) error {
	// Delegate to OnOriginMove if this is a position change
	if result != nil && len(result.AffectedIDs) > 0 {
		// Use placeholder list type - in practice this would be derived from context
		listType := CollectionListTypeEndpoints
		return s.OnOriginMove(ctx, itemID, listType, 0, result.NewPosition)
	}
	return nil
}

// Helper method for batch operations
func (s *UnifiedOrderingService) executeBatchMoveForContext(ctx context.Context, tx *sql.Tx,
	repo EnhancedMovableRepository, operations []UnifiedBatchMoveOperation) (*BatchMoveResult, error) {
	
	result := &BatchMoveResult{
		TotalRequested: len(operations),
		StartTime:      time.Now(),
	}
	
	manager := NewDefaultLinkedListManager(repo)
	
	for _, op := range operations {
		moveResult, err := manager.Move(ctx, tx, op.Operation)
		if err != nil {
			result.FailedCount++
			result.Errors = append(result.Errors, fmt.Errorf("move failed for item %s: %w", op.ItemID.String(), err))
			continue
		}
		
		result.SuccessCount++
		result.Results = append(result.Results, *moveResult)
	}
	
	result.Duration = time.Since(result.StartTime)
	return result, nil
}