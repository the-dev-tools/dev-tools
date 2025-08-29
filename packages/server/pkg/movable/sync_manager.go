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
// SYNC MANAGER TYPES AND INTERFACES
// =============================================================================

// DeltaSyncManager manages synchronization across contexts when deltas change
type DeltaSyncManager struct {
	// Core dependencies
	coordinator DeltaCoordinator
	deltaManager *DeltaAwareManager
	registry    *RepositoryRegistry
	
	// Synchronization state
	mu               sync.RWMutex
	syncRelationships map[idwrap.IDWrap]*SyncRelationship // DeltaID -> SyncRelationship
	originListeners   map[idwrap.IDWrap][]SyncListener    // OriginID -> Listeners
	
	// Event processing
	eventQueue   *SyncEventQueue
	eventWorkers []*SyncEventWorker
	
	// Configuration
	config *SyncManagerConfig
	
	// Performance tracking
	metrics *SyncMetrics
}

// SyncManagerConfig configures synchronization behavior
type SyncManagerConfig struct {
	// Event processing
	EventQueueSize    int
	WorkerCount       int
	BatchSize         int
	
	// Timing settings
	SyncInterval      time.Duration
	ConflictTimeout   time.Duration
	PropagationDelay  time.Duration
	
	// Conflict resolution
	DefaultStrategy   ConflictStrategy
	EnableMerging     bool
	PreserveDelta     bool
	
	// Performance
	EnableBatching    bool
	EnableAsync       bool
	MaxRetries        int
	RetryBackoff      time.Duration
	
	// Validation
	ValidateOnSync    bool
	StrictConsistency bool
}

// DefaultSyncManagerConfig returns sensible defaults
func DefaultSyncManagerConfig() *SyncManagerConfig {
	return &SyncManagerConfig{
		EventQueueSize:    1000,
		WorkerCount:       4,
		BatchSize:         50,
		SyncInterval:      10 * time.Second,
		ConflictTimeout:   30 * time.Second,
		PropagationDelay:  100 * time.Millisecond,
		DefaultStrategy:   ConflictStrategyLastWriteWins,
		EnableMerging:     true,
		PreserveDelta:     true,
		EnableBatching:    true,
		EnableAsync:       true,
		MaxRetries:        3,
		RetryBackoff:      200 * time.Millisecond,
		ValidateOnSync:    true,
		StrictConsistency: false,
	}
}

// =============================================================================
// SYNC RELATIONSHIP TRACKING
// =============================================================================

// SyncRelationship tracks synchronization metadata for a delta
type SyncRelationship struct {
	DeltaID      idwrap.IDWrap
	OriginID     idwrap.IDWrap
	Context      MovableContext
	ScopeID      idwrap.IDWrap
	
	// Synchronization metadata
	LastSync      *time.Time
	SyncVersion   int64
	ConflictCount int
	
	// State tracking
	IsDirty       bool
	IsConflicted  bool
	IsStale       bool
	
	// Configuration
	Strategy      ConflictStrategy
	Priority      int
	
	mu sync.RWMutex
}

// SyncListener defines callbacks for sync events
type SyncListener interface {
	OnOriginMove(ctx context.Context, originID idwrap.IDWrap, event *OriginMoveEvent) error
	OnDeltaCreate(ctx context.Context, deltaID idwrap.IDWrap, event *DeltaCreateEvent) error
	OnDeltaDelete(ctx context.Context, deltaID idwrap.IDWrap, event *DeltaDeleteEvent) error
	OnConflictDetected(ctx context.Context, conflict *SyncConflict) error
}

// =============================================================================
// EVENT SYSTEM
// =============================================================================

// SyncEvent represents a synchronization event
type SyncEvent struct {
	Type      SyncEventType
	ID        string
	Timestamp time.Time
	Context   MovableContext
	Data      interface{}
	Priority  int
	Retries   int
}

// SyncEventType defines types of sync events
type SyncEventType int

const (
	SyncEventOriginMove SyncEventType = iota
	SyncEventDeltaCreate
	SyncEventDeltaDelete
	SyncEventDeltaUpdate
	SyncEventConflictDetected
	SyncEventPropagateChange
)

// OriginMoveEvent contains data for origin move events
type OriginMoveEvent struct {
	OriginID      idwrap.IDWrap
	ListType      ListType
	OldPosition   int
	NewPosition   int
	AffectedDeltas []idwrap.IDWrap
}

// DeltaCreateEvent contains data for delta creation events
type DeltaCreateEvent struct {
	DeltaID     idwrap.IDWrap
	OriginID    idwrap.IDWrap
	Context     MovableContext
	ScopeID     idwrap.IDWrap
	Relation    *DeltaRelation
}

// DeltaDeleteEvent contains data for delta deletion events
type DeltaDeleteEvent struct {
	DeltaID     idwrap.IDWrap
	OriginID    idwrap.IDWrap
	Context     MovableContext
	ScopeID     idwrap.IDWrap
}

// SyncEventQueue manages async event processing
type SyncEventQueue struct {
	events  chan *SyncEvent
	mu      sync.RWMutex
	closed  bool
	metrics *EventQueueMetrics
}

// EventQueueMetrics tracks queue performance
type EventQueueMetrics struct {
	EventsQueued    int64
	EventsProcessed int64
	EventsFailed    int64
	QueueSize       int
	AverageWaitTime time.Duration
}

// SyncEventWorker processes sync events asynchronously
type SyncEventWorker struct {
	id       int
	manager  *DeltaSyncManager
	queue    *SyncEventQueue
	stopCh   chan struct{}
	doneCh   chan struct{}
	
	mu      sync.Mutex
	active  bool
	metrics *WorkerMetrics
}

// WorkerMetrics tracks individual worker performance
type WorkerMetrics struct {
	EventsProcessed int64
	ErrorCount      int64
	AverageTime     time.Duration
	LastActive      time.Time
}

// =============================================================================
// CONFLICT RESOLUTION
// =============================================================================

// ConflictStrategy defines how conflicts are resolved
type ConflictStrategy int

const (
	ConflictStrategyLastWriteWins ConflictStrategy = iota
	ConflictStrategyOriginPriority
	ConflictStrategyDeltaPriority
	ConflictStrategyMergeStrategy
	ConflictStrategyUserChoice
)

func (c ConflictStrategy) String() string {
	switch c {
	case ConflictStrategyLastWriteWins:
		return "last_write_wins"
	case ConflictStrategyOriginPriority:
		return "origin_priority"
	case ConflictStrategyDeltaPriority:
		return "delta_priority"
	case ConflictStrategyMergeStrategy:
		return "merge_strategy"
	case ConflictStrategyUserChoice:
		return "user_choice"
	default:
		return "unknown"
	}
}

// SyncConflict represents a synchronization conflict
type SyncConflict struct {
	ConflictID      string
	OriginID        idwrap.IDWrap
	ConflictingDeltas []ConflictingDelta
	ConflictType    ConflictType
	DetectedAt      time.Time
	Context         MovableContext
	
	// Resolution state
	IsResolved      bool
	Resolution      *ConflictResolution
	ResolvedAt      *time.Time
	ResolvedBy      ConflictStrategy
}

// ConflictingDelta represents a delta involved in a conflict
type ConflictingDelta struct {
	DeltaID       idwrap.IDWrap
	ScopeID       idwrap.IDWrap
	Position      int
	LastModified  time.Time
	ModificationCount int
}

// ConflictType defines types of synchronization conflicts
type ConflictType int

const (
	ConflictTypePositionMismatch ConflictType = iota
	ConflictTypeMultipleDeltas
	ConflictTypeStaleData
	ConflictTypeCircularReference
)

func (c ConflictType) String() string {
	switch c {
	case ConflictTypePositionMismatch:
		return "position_mismatch"
	case ConflictTypeMultipleDeltas:
		return "multiple_deltas"
	case ConflictTypeStaleData:
		return "stale_data"
	case ConflictTypeCircularReference:
		return "circular_reference"
	default:
		return "unknown"
	}
}

// =============================================================================
// CONSTRUCTOR
// =============================================================================

// NewDeltaSyncManager creates a new delta sync manager
func NewDeltaSyncManager(coordinator DeltaCoordinator, registry *RepositoryRegistry, config *SyncManagerConfig) *DeltaSyncManager {
	if config == nil {
		config = DefaultSyncManagerConfig()
	}
	
	manager := &DeltaSyncManager{
		coordinator:       coordinator,
		registry:          registry,
		syncRelationships: make(map[idwrap.IDWrap]*SyncRelationship),
		originListeners:   make(map[idwrap.IDWrap][]SyncListener),
		config:           config,
		metrics:          NewSyncMetrics(),
	}
	
	// Initialize event processing
	if config.EnableAsync {
		manager.eventQueue = NewSyncEventQueue(config.EventQueueSize)
		manager.eventWorkers = make([]*SyncEventWorker, config.WorkerCount)
		
		// Start workers
		for i := 0; i < config.WorkerCount; i++ {
			worker := NewSyncEventWorker(i, manager, manager.eventQueue)
			manager.eventWorkers[i] = worker
			go worker.Start()
		}
	}
	
	return manager
}

// =============================================================================
// CORE SYNCHRONIZATION OPERATIONS
// =============================================================================

// OnOriginMove handles origin position changes and syncs to deltas
func (m *DeltaSyncManager) OnOriginMove(ctx context.Context, originID idwrap.IDWrap, 
	listType ListType, oldPosition, newPosition int) error {
	
	startTime := time.Now()
	defer m.trackMetric("OnOriginMove", startTime, nil)
	
	// Create move event
	event := &OriginMoveEvent{
		OriginID:    originID,
		ListType:    listType,
		OldPosition: oldPosition,
		NewPosition: newPosition,
	}
	
	// Find affected deltas
	m.mu.RLock()
	var affectedDeltas []idwrap.IDWrap
	for deltaID, rel := range m.syncRelationships {
		if rel.OriginID == originID {
			affectedDeltas = append(affectedDeltas, deltaID)
		}
	}
	m.mu.RUnlock()
	
	event.AffectedDeltas = affectedDeltas
	
	// Process sync - async if enabled, sync otherwise
	if m.config.EnableAsync && m.eventQueue != nil {
		syncEvent := &SyncEvent{
			Type:      SyncEventOriginMove,
			ID:        fmt.Sprintf("origin_move_%s_%d", originID.String(), time.Now().UnixNano()),
			Timestamp: time.Now(),
			Data:      event,
			Priority:  5, // Medium priority
		}
		
		return m.eventQueue.Enqueue(syncEvent)
	}
	
	// Synchronous processing
	return m.processOriginMoveSync(ctx, event)
}

// OnDeltaCreate registers a new delta for synchronization
func (m *DeltaSyncManager) OnDeltaCreate(ctx context.Context, deltaID, originID idwrap.IDWrap,
	context MovableContext, scopeID idwrap.IDWrap, relation *DeltaRelation) error {
	
	startTime := time.Now()
	defer m.trackMetric("OnDeltaCreate", startTime, nil)
	
	// Create sync relationship
	syncRel := &SyncRelationship{
		DeltaID:      deltaID,
		OriginID:     originID,
		Context:      context,
		ScopeID:      scopeID,
		SyncVersion:  1,
		Strategy:     m.config.DefaultStrategy,
		Priority:     5, // Default priority
		IsDirty:      true,
	}
	
	// Register relationship
	m.mu.Lock()
	m.syncRelationships[deltaID] = syncRel
	
	// Add to origin listeners
	listeners := m.originListeners[originID]
	// In a full implementation, we'd add actual listeners here
	m.originListeners[originID] = listeners
	m.mu.Unlock()
	
	// Create event
	event := &DeltaCreateEvent{
		DeltaID:  deltaID,
		OriginID: originID,
		Context:  context,
		ScopeID:  scopeID,
		Relation: relation,
	}
	
	// Process async if enabled
	if m.config.EnableAsync && m.eventQueue != nil {
		syncEvent := &SyncEvent{
			Type:      SyncEventDeltaCreate,
			ID:        fmt.Sprintf("delta_create_%s_%d", deltaID.String(), time.Now().UnixNano()),
			Timestamp: time.Now(),
			Data:      event,
			Priority:  7, // High priority for creation
		}
		
		return m.eventQueue.Enqueue(syncEvent)
	}
	
	return m.processDeltaCreateSync(ctx, event)
}

// OnDeltaDelete cleans up synchronization relationships
func (m *DeltaSyncManager) OnDeltaDelete(ctx context.Context, deltaID idwrap.IDWrap) error {
	startTime := time.Now()
	defer m.trackMetric("OnDeltaDelete", startTime, nil)
	
	// Get relationship before deletion
	m.mu.RLock()
	rel, exists := m.syncRelationships[deltaID]
	m.mu.RUnlock()
	
	if !exists {
		return nil // Nothing to clean up
	}
	
	// Create event
	event := &DeltaDeleteEvent{
		DeltaID:  deltaID,
		OriginID: rel.OriginID,
		Context:  rel.Context,
		ScopeID:  rel.ScopeID,
	}
	
	// Process async if enabled
	if m.config.EnableAsync && m.eventQueue != nil {
		syncEvent := &SyncEvent{
			Type:      SyncEventDeltaDelete,
			ID:        fmt.Sprintf("delta_delete_%s_%d", deltaID.String(), time.Now().UnixNano()),
			Timestamp: time.Now(),
			Data:      event,
			Priority:  6, // Medium-high priority for cleanup
		}
		
		return m.eventQueue.Enqueue(syncEvent)
	}
	
	return m.processDeltaDeleteSync(ctx, event)
}

// ResolveConflicts handles competing changes with configured strategy
func (m *DeltaSyncManager) ResolveConflicts(ctx context.Context, conflicts []SyncConflict) (*ConflictResolutionResult, error) {
	startTime := time.Now()
	defer m.trackMetric("ResolveConflicts", startTime, nil)
	
	if len(conflicts) == 0 {
		return &ConflictResolutionResult{}, nil
	}
	
	result := &ConflictResolutionResult{
		TotalConflicts:   len(conflicts),
		StartTime:        startTime,
		ResolvedConflicts: make([]ResolvedConflict, 0),
		FailedConflicts:   make([]FailedConflict, 0),
	}
	
	// Process each conflict
	for _, conflict := range conflicts {
		resolution, err := m.resolveConflict(ctx, &conflict)
		if err != nil {
			result.FailedCount++
			result.FailedConflicts = append(result.FailedConflicts, FailedConflict{
				ConflictID: conflict.ConflictID,
				Error:      err,
				Reason:     "resolution_failed",
			})
			continue
		}
		
		result.ResolvedCount++
		result.ResolvedConflicts = append(result.ResolvedConflicts, ResolvedConflict{
			ConflictID: conflict.ConflictID,
			Strategy:   resolution.Strategy,
			Resolution: resolution,
		})
	}
	
	result.Duration = time.Since(startTime)
	return result, nil
}

// PropagateChanges applies synchronized changes to affected deltas
func (m *DeltaSyncManager) PropagateChanges(ctx context.Context, tx *sql.Tx,
	originID idwrap.IDWrap, changes *ChangeSet) error {
	
	startTime := time.Now()
	defer m.trackMetric("PropagateChanges", startTime, nil)
	
	// Find affected sync relationships
	m.mu.RLock()
	var affectedRels []*SyncRelationship
	for _, rel := range m.syncRelationships {
		if rel.OriginID == originID {
			affectedRels = append(affectedRels, rel)
		}
	}
	m.mu.RUnlock()
	
	if len(affectedRels) == 0 {
		return nil // No deltas to update
	}
	
	// Process changes with batching if enabled
	if m.config.EnableBatching {
		return m.propagateChangesBatch(ctx, tx, affectedRels, changes)
	}
	
	// Process changes individually
	for _, rel := range affectedRels {
		if err := m.propagateToSingleDelta(ctx, tx, rel, changes); err != nil {
			m.metrics.TrackPropagationError()
			// Continue with other deltas rather than failing completely
			fmt.Printf("Warning: failed to propagate changes to delta %s: %v\n", rel.DeltaID.String(), err)
		}
	}
	
	return nil
}

// =============================================================================
// SYNCHRONOUS PROCESSING IMPLEMENTATIONS
// =============================================================================

// processOriginMoveSync handles origin moves synchronously
func (m *DeltaSyncManager) processOriginMoveSync(ctx context.Context, event *OriginMoveEvent) error {
	// Get deltas for this origin
	for _, deltaID := range event.AffectedDeltas {
		m.mu.RLock()
		rel, exists := m.syncRelationships[deltaID]
		m.mu.RUnlock()
		
		if !exists {
			continue
		}
		
		// Update delta position to match origin
		if err := m.updateDeltaPosition(ctx, rel, event.NewPosition); err != nil {
			m.metrics.TrackSyncError()
			continue
		}
		
		// Update sync metadata
		rel.mu.Lock()
		now := time.Now()
		rel.LastSync = &now
		rel.SyncVersion++
		rel.IsDirty = false
		rel.mu.Unlock()
	}
	
	return nil
}

// processDeltaCreateSync handles delta creation synchronously
func (m *DeltaSyncManager) processDeltaCreateSync(ctx context.Context, event *DeltaCreateEvent) error {
	// Initial sync for the new delta
	rel := m.syncRelationships[event.DeltaID]
	if rel == nil {
		return fmt.Errorf("sync relationship not found for delta %s", event.DeltaID.String())
	}
	
	// Perform initial position sync
	return m.syncDeltaWithOrigin(ctx, rel)
}

// processDeltaDeleteSync handles delta deletion synchronously
func (m *DeltaSyncManager) processDeltaDeleteSync(ctx context.Context, event *DeltaDeleteEvent) error {
	// Clean up sync relationship
	m.mu.Lock()
	delete(m.syncRelationships, event.DeltaID)
	
	// Remove from origin listeners
	if listeners, exists := m.originListeners[event.OriginID]; exists {
		// Filter out deleted delta (in a full implementation with actual listeners)
		m.originListeners[event.OriginID] = listeners
	}
	m.mu.Unlock()
	
	return nil
}

// =============================================================================
// CONFLICT RESOLUTION IMPLEMENTATION
// =============================================================================

// resolveConflict resolves a single conflict using the configured strategy
func (m *DeltaSyncManager) resolveConflict(ctx context.Context, conflict *SyncConflict) (*ConflictResolution, error) {
	switch m.config.DefaultStrategy {
	case ConflictStrategyLastWriteWins:
		return m.resolveLastWriteWins(ctx, conflict)
	case ConflictStrategyOriginPriority:
		return m.resolveOriginPriority(ctx, conflict)
	case ConflictStrategyDeltaPriority:
		return m.resolveDeltaPriority(ctx, conflict)
	case ConflictStrategyMergeStrategy:
		return m.resolveMergeStrategy(ctx, conflict)
	default:
		return nil, fmt.Errorf("unsupported conflict resolution strategy: %s", m.config.DefaultStrategy.String())
	}
}

// resolveLastWriteWins resolves conflicts by choosing the most recently modified delta
func (m *DeltaSyncManager) resolveLastWriteWins(ctx context.Context, conflict *SyncConflict) (*ConflictResolution, error) {
	if len(conflict.ConflictingDeltas) == 0 {
		return nil, fmt.Errorf("no conflicting deltas found")
	}
	
	// Find the most recently modified delta
	var winner *ConflictingDelta
	var latestTime time.Time
	
	for i := range conflict.ConflictingDeltas {
		delta := &conflict.ConflictingDeltas[i]
		if winner == nil || delta.LastModified.After(latestTime) {
			winner = delta
			latestTime = delta.LastModified
		}
	}
	
	// Create resolution
	resolution := &ConflictResolution{
		Strategy:    ConflictStrategyLastWriteWins,
		WinnerID:    winner.DeltaID,
		ResolvedAt:  time.Now(),
		Description: fmt.Sprintf("Selected delta %s (modified at %v)", winner.DeltaID.String(), winner.LastModified),
	}
	
	// Mark losing deltas for removal/update
	for i := range conflict.ConflictingDeltas {
		delta := &conflict.ConflictingDeltas[i]
		if delta.DeltaID != winner.DeltaID {
			resolution.RemovedDeltas = append(resolution.RemovedDeltas, delta.DeltaID)
		}
	}
	
	return resolution, nil
}

// resolveOriginPriority always chooses origin over deltas
func (m *DeltaSyncManager) resolveOriginPriority(ctx context.Context, conflict *SyncConflict) (*ConflictResolution, error) {
	resolution := &ConflictResolution{
		Strategy:    ConflictStrategyOriginPriority,
		WinnerID:    conflict.OriginID,
		ResolvedAt:  time.Now(),
		Description: "Origin always takes precedence",
	}
	
	// All deltas should be updated to match origin
	for i := range conflict.ConflictingDeltas {
		delta := &conflict.ConflictingDeltas[i]
		resolution.UpdatedDeltas = append(resolution.UpdatedDeltas, delta.DeltaID)
	}
	
	return resolution, nil
}

// resolveDeltaPriority preserves delta changes over origin
func (m *DeltaSyncManager) resolveDeltaPriority(ctx context.Context, conflict *SyncConflict) (*ConflictResolution, error) {
	if len(conflict.ConflictingDeltas) == 0 {
		return nil, fmt.Errorf("no delta found for delta priority resolution")
	}
	
	// Choose the delta with highest modification count (most actively used)
	var winner *ConflictingDelta
	maxMods := -1
	
	for i := range conflict.ConflictingDeltas {
		delta := &conflict.ConflictingDeltas[i]
		if delta.ModificationCount > maxMods {
			winner = delta
			maxMods = delta.ModificationCount
		}
	}
	
	resolution := &ConflictResolution{
		Strategy:    ConflictStrategyDeltaPriority,
		WinnerID:    winner.DeltaID,
		ResolvedAt:  time.Now(),
		Description: fmt.Sprintf("Preserved delta %s with %d modifications", winner.DeltaID.String(), winner.ModificationCount),
	}
	
	return resolution, nil
}

// resolveMergeStrategy attempts intelligent merging of changes
func (m *DeltaSyncManager) resolveMergeStrategy(ctx context.Context, conflict *SyncConflict) (*ConflictResolution, error) {
	if !m.config.EnableMerging {
		return nil, fmt.Errorf("merge strategy disabled")
	}
	
	// For position conflicts, use average or median position
	if conflict.ConflictType == ConflictTypePositionMismatch {
		return m.resolvePositionMerge(ctx, conflict)
	}
	
	// For other conflict types, fall back to last write wins
	return m.resolveLastWriteWins(ctx, conflict)
}

// resolvePositionMerge resolves position conflicts by finding compromise position
func (m *DeltaSyncManager) resolvePositionMerge(ctx context.Context, conflict *SyncConflict) (*ConflictResolution, error) {
	if len(conflict.ConflictingDeltas) == 0 {
		return nil, fmt.Errorf("no conflicting deltas for position merge")
	}
	
	// Calculate average position
	totalPos := 0
	for _, delta := range conflict.ConflictingDeltas {
		totalPos += delta.Position
	}
	averagePos := totalPos / len(conflict.ConflictingDeltas)
	
	resolution := &ConflictResolution{
		Strategy:    ConflictStrategyMergeStrategy,
		ResolvedAt:  time.Now(),
		Description: fmt.Sprintf("Merged position to average: %d", averagePos),
		MergedPosition: &averagePos,
	}
	
	// All deltas will be updated to the merged position
	for _, delta := range conflict.ConflictingDeltas {
		resolution.UpdatedDeltas = append(resolution.UpdatedDeltas, delta.DeltaID)
	}
	
	return resolution, nil
}

// =============================================================================
// PERFORMANCE OPTIMIZATIONS
// =============================================================================

// propagateChangesBatch efficiently processes multiple delta updates
func (m *DeltaSyncManager) propagateChangesBatch(ctx context.Context, tx *sql.Tx,
	relationships []*SyncRelationship, changes *ChangeSet) error {
	
	if len(relationships) == 0 {
		return nil
	}
	
	// Group by context for efficient repository usage
	contextGroups := make(map[MovableContext][]*SyncRelationship)
	for _, rel := range relationships {
		contextGroups[rel.Context] = append(contextGroups[rel.Context], rel)
	}
	
	// Process each context group
	for context, contextRels := range contextGroups {
		if err := m.propagateToContext(ctx, tx, context, contextRels, changes); err != nil {
			m.metrics.TrackPropagationError()
			// Continue with other contexts
			fmt.Printf("Warning: failed to propagate changes to context %s: %v\n", context.String(), err)
		}
	}
	
	return nil
}

// propagateToContext updates all deltas in a specific context efficiently
func (m *DeltaSyncManager) propagateToContext(ctx context.Context, tx *sql.Tx,
	context MovableContext, relationships []*SyncRelationship, changes *ChangeSet) error {
	
	// Get appropriate repository for context
	m.registry.mu.RLock()
	repo := m.getRepositoryForContext(context)
	m.registry.mu.RUnlock()
	
	if repo == nil {
		return fmt.Errorf("no repository available for context %s", context.String())
	}
	
	// Prepare batch updates
	var updates []PositionUpdate
	for _, rel := range relationships {
		if changes.PositionChanges != nil {
			if newPos, exists := changes.PositionChanges[rel.OriginID]; exists {
				updates = append(updates, PositionUpdate{
					ItemID:   rel.DeltaID,
					ListType: changes.ListType,
					Position: newPos,
				})
			}
		}
	}
	
	// Execute batch update
	if len(updates) > 0 {
		return repo.UpdatePositions(ctx, tx, updates)
	}
	
	return nil
}

// propagateToSingleDelta updates a single delta
func (m *DeltaSyncManager) propagateToSingleDelta(ctx context.Context, tx *sql.Tx,
	rel *SyncRelationship, changes *ChangeSet) error {
	
	repo := m.getRepositoryForContext(rel.Context)
	if repo == nil {
		return fmt.Errorf("no repository for context %s", rel.Context.String())
	}
	
	// Apply position changes
	if changes.PositionChanges != nil {
		if newPos, exists := changes.PositionChanges[rel.OriginID]; exists {
			if err := repo.UpdatePosition(ctx, tx, rel.DeltaID, changes.ListType, newPos); err != nil {
				return fmt.Errorf("failed to update position: %w", err)
			}
		}
	}
	
	// Update sync metadata
	rel.mu.Lock()
	now := time.Now()
	rel.LastSync = &now
	rel.SyncVersion++
	rel.IsDirty = false
	rel.mu.Unlock()
	
	return nil
}

// getRepositoryForContext returns the appropriate repository for a context
func (m *DeltaSyncManager) getRepositoryForContext(context MovableContext) MovableRepository {
	switch context {
	case ContextEndpoint:
		if m.registry.endpointRepo != nil {
			return m.registry.endpointRepo.SimpleEnhancedRepository
		}
	case ContextCollection:
		if m.registry.exampleRepo != nil {
			return m.registry.exampleRepo
		}
	case ContextFlow:
		if m.registry.flowNodeRepo != nil {
			return m.registry.flowNodeRepo
		}
	case ContextWorkspace:
		if m.registry.workspaceRepo != nil {
			return m.registry.workspaceRepo
		}
	}
	
	return m.registry.defaultRepo
}

// =============================================================================
// HELPER METHODS
// =============================================================================

// updateDeltaPosition updates a single delta's position
func (m *DeltaSyncManager) updateDeltaPosition(ctx context.Context, rel *SyncRelationship, position int) error {
	repo := m.getRepositoryForContext(rel.Context)
	if repo == nil {
		return fmt.Errorf("no repository for context %s", rel.Context.String())
	}
	
	// Use a placeholder list type - in practice this would be determined from the sync context
	listType := CollectionListTypeEndpoints // This would be context-specific
	
	return repo.UpdatePosition(ctx, nil, rel.DeltaID, listType, position)
}

// syncDeltaWithOrigin performs initial synchronization for a new delta
func (m *DeltaSyncManager) syncDeltaWithOrigin(ctx context.Context, rel *SyncRelationship) error {
	// In a full implementation, this would:
	// 1. Get origin item position
	// 2. Update delta to match
	// 3. Handle any conflicts
	
	rel.mu.Lock()
	now := time.Now()
	rel.LastSync = &now
	rel.SyncVersion = 1
	rel.IsDirty = false
	rel.mu.Unlock()
	
	return nil
}

// trackMetric tracks performance metrics
func (m *DeltaSyncManager) trackMetric(operation string, startTime time.Time, err error) {
	if m.metrics != nil {
		duration := time.Since(startTime)
		m.metrics.TrackOperation(operation, duration, err)
	}
}

// =============================================================================
// SUPPORTING TYPES
// =============================================================================

// ChangeSet represents a set of changes to propagate
type ChangeSet struct {
	ListType        ListType
	PositionChanges map[idwrap.IDWrap]int // OriginID -> NewPosition
	Metadata        map[string]interface{}
}

// ConflictResolution contains the result of conflict resolution
type ConflictResolution struct {
	Strategy        ConflictStrategy
	WinnerID        idwrap.IDWrap
	ResolvedAt      time.Time
	Description     string
	
	// Action lists
	RemovedDeltas   []idwrap.IDWrap
	UpdatedDeltas   []idwrap.IDWrap
	MergedPosition  *int
}

// ConflictResolutionResult contains results of batch conflict resolution
type ConflictResolutionResult struct {
	TotalConflicts    int
	ResolvedCount     int
	FailedCount       int
	ResolvedConflicts []ResolvedConflict
	FailedConflicts   []FailedConflict
	StartTime         time.Time
	Duration          time.Duration
}

// ResolvedConflict represents a successfully resolved conflict
type ResolvedConflict struct {
	ConflictID string
	Strategy   ConflictStrategy
	Resolution *ConflictResolution
}

// FailedConflict represents a conflict that couldn't be resolved
type FailedConflict struct {
	ConflictID string
	Error      error
	Reason     string
}

// SyncMetrics tracks synchronization performance
type SyncMetrics struct {
	mu                sync.RWMutex
	totalOperations   int64
	successfulSyncs   int64
	failedSyncs       int64
	propagationErrors int64
	averageSyncTime   time.Duration
	lastActivity      time.Time
}

// NewSyncMetrics creates a new sync metrics instance
func NewSyncMetrics() *SyncMetrics {
	return &SyncMetrics{
		lastActivity: time.Now(),
	}
}

// TrackOperation records a sync operation
func (m *SyncMetrics) TrackOperation(operation string, duration time.Duration, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.totalOperations++
	m.lastActivity = time.Now()
	
	if err == nil {
		m.successfulSyncs++
	} else {
		m.failedSyncs++
	}
	
	// Update average time (simple moving average)
	if m.averageSyncTime == 0 {
		m.averageSyncTime = duration
	} else {
		m.averageSyncTime = (m.averageSyncTime + duration) / 2
	}
}

// TrackSyncError records a synchronization error
func (m *SyncMetrics) TrackSyncError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.failedSyncs++
}

// TrackPropagationError records a propagation error
func (m *SyncMetrics) TrackPropagationError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.propagationErrors++
}

// GetStats returns current metrics
func (m *SyncMetrics) GetStats() SyncStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return SyncStats{
		TotalOperations:   m.totalOperations,
		SuccessfulSyncs:   m.successfulSyncs,
		FailedSyncs:       m.failedSyncs,
		PropagationErrors: m.propagationErrors,
		AverageSyncTime:   m.averageSyncTime,
		LastActivity:      m.lastActivity,
	}
}

// SyncStats contains synchronization statistics
type SyncStats struct {
	TotalOperations   int64
	SuccessfulSyncs   int64
	FailedSyncs       int64
	PropagationErrors int64
	AverageSyncTime   time.Duration
	LastActivity      time.Time
}

// =============================================================================
// EVENT QUEUE IMPLEMENTATION
// =============================================================================

// NewSyncEventQueue creates a new sync event queue
func NewSyncEventQueue(size int) *SyncEventQueue {
	return &SyncEventQueue{
		events:  make(chan *SyncEvent, size),
		metrics: &EventQueueMetrics{},
	}
}

// Enqueue adds an event to the queue
func (q *SyncEventQueue) Enqueue(event *SyncEvent) error {
	q.mu.RLock()
	defer q.mu.RUnlock()
	
	if q.closed {
		return fmt.Errorf("queue is closed")
	}
	
	select {
	case q.events <- event:
		q.metrics.EventsQueued++
		return nil
	default:
		return fmt.Errorf("queue is full")
	}
}

// Dequeue retrieves an event from the queue
func (q *SyncEventQueue) Dequeue() (*SyncEvent, bool) {
	select {
	case event := <-q.events:
		return event, true
	default:
		return nil, false
	}
}

// Close closes the event queue
func (q *SyncEventQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	if !q.closed {
		close(q.events)
		q.closed = true
	}
}

// =============================================================================
// EVENT WORKER IMPLEMENTATION
// =============================================================================

// NewSyncEventWorker creates a new sync event worker
func NewSyncEventWorker(id int, manager *DeltaSyncManager, queue *SyncEventQueue) *SyncEventWorker {
	return &SyncEventWorker{
		id:      id,
		manager: manager,
		queue:   queue,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
		metrics: &WorkerMetrics{},
	}
}

// Start starts the worker's event processing loop
func (w *SyncEventWorker) Start() {
	defer close(w.doneCh)
	
	w.mu.Lock()
	w.active = true
	w.mu.Unlock()
	
	for {
		select {
		case <-w.stopCh:
			return
		default:
			if event, ok := w.queue.Dequeue(); ok {
				w.processEvent(event)
			} else {
				// No events available, short sleep to prevent busy waiting
				time.Sleep(10 * time.Millisecond)
			}
		}
	}
}

// Stop stops the worker
func (w *SyncEventWorker) Stop() {
	close(w.stopCh)
	<-w.doneCh // Wait for worker to finish
	
	w.mu.Lock()
	w.active = false
	w.mu.Unlock()
}

// processEvent processes a single sync event
func (w *SyncEventWorker) processEvent(event *SyncEvent) {
	startTime := time.Now()
	defer func() {
		w.metrics.EventsProcessed++
		w.metrics.LastActive = time.Now()
		duration := time.Since(startTime)
		
		// Update average time
		if w.metrics.AverageTime == 0 {
			w.metrics.AverageTime = duration
		} else {
			w.metrics.AverageTime = (w.metrics.AverageTime + duration) / 2
		}
	}()
	
	ctx := context.Background()
	var err error
	
	switch event.Type {
	case SyncEventOriginMove:
		if moveEvent, ok := event.Data.(*OriginMoveEvent); ok {
			err = w.manager.processOriginMoveSync(ctx, moveEvent)
		}
	case SyncEventDeltaCreate:
		if createEvent, ok := event.Data.(*DeltaCreateEvent); ok {
			err = w.manager.processDeltaCreateSync(ctx, createEvent)
		}
	case SyncEventDeltaDelete:
		if deleteEvent, ok := event.Data.(*DeltaDeleteEvent); ok {
			err = w.manager.processDeltaDeleteSync(ctx, deleteEvent)
		}
	}
	
	if err != nil {
		w.metrics.ErrorCount++
		
		// Retry logic
		if event.Retries < w.manager.config.MaxRetries {
			event.Retries++
			time.Sleep(w.manager.config.RetryBackoff * time.Duration(event.Retries))
			// Re-enqueue for retry
			w.queue.Enqueue(event)
		}
	}
}