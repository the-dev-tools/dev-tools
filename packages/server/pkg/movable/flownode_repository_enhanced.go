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
// FLOW NODE SPECIFIC TYPES
// =============================================================================

// FlowNodeOrderType defines the type of ordering for flow nodes
type FlowNodeOrderType int

const (
	FlowNodeOrderSpatial    FlowNodeOrderType = iota // Position-based ordering (x, y coordinates)
	FlowNodeOrderSequential                          // Sequential ordering for variables
)

// String returns the string representation of the ordering type
func (f FlowNodeOrderType) String() string {
	switch f {
	case FlowNodeOrderSpatial:
		return "spatial"
	case FlowNodeOrderSequential:
		return "sequential"
	default:
		return "unknown"
	}
}

// FlowNodeDeltaReference represents delta references for REQUEST nodes
type FlowNodeDeltaReference struct {
	NodeID          idwrap.IDWrap
	EndpointID      *idwrap.IDWrap // Original endpoint reference
	ExampleID       *idwrap.IDWrap // Original example reference
	DeltaEndpointID *idwrap.IDWrap // Node-specific endpoint override
	DeltaExampleID  *idwrap.IDWrap // Node-specific example override
	Context         MovableContext
	FlowID          idwrap.IDWrap
	IsActive        bool
}

// FlowNodePosition represents spatial position for nodes
type FlowNodePosition struct {
	X float64
	Y float64
}

// FlowNodeItem represents a flow node item with enhanced metadata
type FlowNodeItem struct {
	ID         idwrap.IDWrap
	FlowID     idwrap.IDWrap
	ParentID   *idwrap.IDWrap // Parent node for nested structures
	Position   FlowNodePosition
	Sequential int                     // Sequential position for variables
	NodeKind   int32                   // Type of node (REQUEST, FOR, etc.)
	ListType   ListType                // Flow-specific list type
	Context    *ContextMetadata        // Context information
	DeltaRef   *FlowNodeDeltaReference // Delta reference for REQUEST nodes
	OrderType  FlowNodeOrderType       // Type of ordering
}

// =============================================================================
// ENHANCED FLOW NODE REPOSITORY INTERFACE
// =============================================================================

// EnhancedFlowNodeRepository provides flow-scoped ordering with delta awareness
type EnhancedFlowNodeRepository interface {
	// Embed enhanced repository for base functionality
	EnhancedMovableRepository

	// === FLOW-SPECIFIC OPERATIONS ===

	// GetNodesInFlow returns all nodes in a flow with proper ordering
	GetNodesInFlow(ctx context.Context, flowID idwrap.IDWrap, orderType FlowNodeOrderType) ([]FlowNodeItem, error)

	// UpdateNodePosition updates a node's spatial position within a flow
	UpdateNodePosition(ctx context.Context, tx *sql.Tx, nodeID idwrap.IDWrap,
		position FlowNodePosition, flowContext *FlowBoundaryContext) error

	// BatchUpdateNodePositions efficiently updates multiple node positions
	BatchUpdateNodePositions(ctx context.Context, tx *sql.Tx,
		updates []FlowNodePositionUpdate, flowContext *FlowBoundaryContext) error

	// ValidateFlowBoundary ensures all operations stay within flow scope
	ValidateFlowBoundary(ctx context.Context, nodeID idwrap.IDWrap, expectedFlowID idwrap.IDWrap) error

	// === REQUEST NODE DELTA OPERATIONS ===

	// HandleRequestNode manages REQUEST node with delta references
	HandleRequestNode(ctx context.Context, tx *sql.Tx, nodeID idwrap.IDWrap,
		deltaRef *FlowNodeDeltaReference) error

	// ResolveRequestNodeDeltas resolves effective endpoints/examples for REQUEST nodes
	ResolveRequestNodeDeltas(ctx context.Context, nodeID idwrap.IDWrap,
		flowContext *FlowBoundaryContext) (*ResolvedRequestNode, error)

	// GetRequestNodeDeltas returns all delta references for REQUEST nodes in a flow
	GetRequestNodeDeltas(ctx context.Context, flowID idwrap.IDWrap) ([]FlowNodeDeltaReference, error)

	// UpdateRequestNodeDeltas updates delta references for a REQUEST node
	UpdateRequestNodeDeltas(ctx context.Context, tx *sql.Tx, nodeID idwrap.IDWrap,
		deltaRef *FlowNodeDeltaReference) error

	// === FLOW VARIABLE OPERATIONS ===

	// GetFlowVariablesOrder returns variables in sequential order
	GetFlowVariablesOrder(ctx context.Context, flowID idwrap.IDWrap) ([]FlowNodeItem, error)

	// ReorderFlowVariable moves a flow variable in sequential ordering
	ReorderFlowVariable(ctx context.Context, tx *sql.Tx, variableID idwrap.IDWrap,
		newPosition int, flowContext *FlowBoundaryContext) error

	// === BATCH OPERATIONS FOR PERFORMANCE ===

	// BatchCreateFlowNodes efficiently creates multiple flow nodes
	BatchCreateFlowNodes(ctx context.Context, tx *sql.Tx, nodes []FlowNodeItem) error

	// BatchResolveFlowNodes efficiently resolves multiple nodes with deltas
	BatchResolveFlowNodes(ctx context.Context, nodeIDs []idwrap.IDWrap,
		flowContext *FlowBoundaryContext) (map[idwrap.IDWrap]*FlowNodeItem, error)

	// CompactFlowPositions rebalances all positions within a flow
	CompactFlowPositions(ctx context.Context, tx *sql.Tx, flowID idwrap.IDWrap) error
}

// =============================================================================
// FLOW CONTEXT TYPES
// =============================================================================

// FlowBoundaryContext enforces flow scope boundaries
type FlowBoundaryContext struct {
	FlowID        idwrap.IDWrap
	WorkspaceID   idwrap.IDWrap
	CollectionID  *idwrap.IDWrap // Optional collection context
	UserID        idwrap.IDWrap
	EnforceStrict bool // Whether to strictly enforce boundaries
}

// FlowNodePositionUpdate represents a position update for a flow node
type FlowNodePositionUpdate struct {
	NodeID     idwrap.IDWrap
	Position   FlowNodePosition
	Sequential int // For sequential ordering
	OrderType  FlowNodeOrderType
	Context    *ContextMetadata
	DeltaRef   *FlowNodeDeltaReference // Updated delta reference
}

// ResolvedRequestNode represents a REQUEST node with resolved delta references
type ResolvedRequestNode struct {
	NodeID              idwrap.IDWrap
	EffectiveEndpointID *idwrap.IDWrap // Resolved endpoint (delta if exists, origin otherwise)
	EffectiveExampleID  *idwrap.IDWrap // Resolved example (delta if exists, origin otherwise)
	OriginEndpointID    *idwrap.IDWrap
	OriginExampleID     *idwrap.IDWrap
	DeltaEndpointID     *idwrap.IDWrap
	DeltaExampleID      *idwrap.IDWrap
	Context             *FlowBoundaryContext
	ResolutionTime      time.Time
	HasDeltas           bool
}

// =============================================================================
// ENHANCED FLOW NODE REPOSITORY IMPLEMENTATION
// =============================================================================

// EnhancedFlowNodeRepositoryImpl implements flow-scoped ordering with delta awareness
type EnhancedFlowNodeRepositoryImpl struct {
	// Embed enhanced repository for base functionality
	*SimpleEnhancedRepository

	// Flow-specific configuration
	flowConfig *FlowNodeConfig

	// Delta reference tracking for REQUEST nodes
	requestNodeDeltas map[string]*FlowNodeDeltaReference
	deltasMutex       sync.RWMutex

	// Flow boundary cache for performance
	flowBoundaryCache  map[string]*FlowBoundaryContext
	boundaryCacheTTL   time.Duration
	boundaryCacheMutex sync.RWMutex
}

// FlowNodeConfig provides configuration for flow node operations
type FlowNodeConfig struct {
	// Delta support
	EnableDeltaSupport       bool
	EnableBoundaryValidation bool
	EnablePositionCompaction bool

	// Performance tuning
	BatchSize                   int
	MaxNodesPerFlow             int
	PositionCompactionThreshold float64

	// Spatial ordering
	DefaultSpatialGrid   float64 // Grid size for spatial positioning
	MinSpatialDistance   float64 // Minimum distance between nodes
	MaxSpatialCoordinate float64 // Maximum coordinate value
}

// NewEnhancedFlowNodeRepository creates a new enhanced flow node repository
func NewEnhancedFlowNodeRepository(db *sql.DB, config *EnhancedRepositoryConfig,
	flowConfig *FlowNodeConfig) *EnhancedFlowNodeRepositoryImpl {

	// Create enhanced base repository
	baseRepo := NewSimpleEnhancedRepository(db, config)

	// Set default flow configuration if not provided
	if flowConfig == nil {
		flowConfig = &FlowNodeConfig{
			EnableDeltaSupport:          true,
			EnableBoundaryValidation:    true,
			EnablePositionCompaction:    true,
			BatchSize:                   50,
			MaxNodesPerFlow:             1000,
			PositionCompactionThreshold: 10000.0,
			DefaultSpatialGrid:          50.0,
			MinSpatialDistance:          20.0,
			MaxSpatialCoordinate:        10000.0,
		}
	}

	return &EnhancedFlowNodeRepositoryImpl{
		SimpleEnhancedRepository: baseRepo,
		flowConfig:               flowConfig,
		requestNodeDeltas:        make(map[string]*FlowNodeDeltaReference),
		flowBoundaryCache:        make(map[string]*FlowBoundaryContext),
		boundaryCacheTTL:         5 * time.Minute,
	}
}

// TX returns a flow node repository instance with transaction support
func (r *EnhancedFlowNodeRepositoryImpl) TX(tx *sql.Tx) EnhancedFlowNodeRepository {
	baseTxRepo := r.SimpleEnhancedRepository.TX(tx).(*SimpleEnhancedRepository)

	return &EnhancedFlowNodeRepositoryImpl{
		SimpleEnhancedRepository: baseTxRepo,
		flowConfig:               r.flowConfig,
		requestNodeDeltas:        r.requestNodeDeltas, // Share delta references
		flowBoundaryCache:        r.flowBoundaryCache, // Share boundary cache
		boundaryCacheTTL:         r.boundaryCacheTTL,
	}
}

// =============================================================================
// FLOW-SPECIFIC OPERATIONS IMPLEMENTATION
// =============================================================================

// GetNodesInFlow returns all nodes in a flow with proper ordering
func (r *EnhancedFlowNodeRepositoryImpl) GetNodesInFlow(ctx context.Context,
	flowID idwrap.IDWrap, orderType FlowNodeOrderType) ([]FlowNodeItem, error) {

	// Input validation
	emptyID := idwrap.IDWrap{}
	if flowID.Compare(emptyID) == 0 {
		return nil, fmt.Errorf("flow ID is required")
	}

	// Get base items from parent repository using flow as parent
	baseItems, err := r.GetItemsByParent(ctx, flowID, FlowListTypeNodes)
	if err != nil {
		return nil, fmt.Errorf("failed to get flow nodes: %w", err)
	}

	// Convert to flow node items
	flowNodes := make([]FlowNodeItem, 0, len(baseItems))
	for _, item := range baseItems {
		flowNode, err := r.convertToFlowNodeItem(ctx, item, orderType)
		if err != nil {
			return nil, fmt.Errorf("failed to convert node %s: %w", item.ID.String(), err)
		}
		if flowNode != nil {
			flowNodes = append(flowNodes, *flowNode)
		}
	}

	// Sort based on order type
	r.sortFlowNodes(flowNodes, orderType)

	return flowNodes, nil
}

// UpdateNodePosition updates a node's spatial position within a flow
func (r *EnhancedFlowNodeRepositoryImpl) UpdateNodePosition(ctx context.Context, tx *sql.Tx,
	nodeID idwrap.IDWrap, position FlowNodePosition, flowContext *FlowBoundaryContext) error {

	// Input validation
	emptyID := idwrap.IDWrap{}
	if nodeID.Compare(emptyID) == 0 {
		return fmt.Errorf("node ID is required")
	}
	if flowContext == nil {
		return fmt.Errorf("flow boundary context is required")
	}

	// Validate flow boundary
	if r.flowConfig.EnableBoundaryValidation {
		if err := r.ValidateFlowBoundary(ctx, nodeID, flowContext.FlowID); err != nil {
			return fmt.Errorf("flow boundary validation failed: %w", err)
		}
	}

	// Validate spatial coordinates
	if err := r.validateSpatialPosition(position); err != nil {
		return fmt.Errorf("invalid spatial position: %w", err)
	}

	// For spatial positioning, we don't use traditional sequential positions
	// Instead, we'll encode spatial coordinates into a single position value
	encodedPosition := r.encodeSpatialPosition(position)

	// Create context metadata
	contextMeta := &ContextMetadata{
		Type:      ContextFlow,
		ScopeID:   flowContext.FlowID.Bytes(),
		IsHidden:  false,
		Priority:  0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Update position using enhanced repository
	return r.UpdatePositionWithContext(ctx, tx, nodeID, FlowListTypeNodes,
		encodedPosition, contextMeta)
}

// BatchUpdateNodePositions efficiently updates multiple node positions
func (r *EnhancedFlowNodeRepositoryImpl) BatchUpdateNodePositions(ctx context.Context, tx *sql.Tx,
	updates []FlowNodePositionUpdate, flowContext *FlowBoundaryContext) error {

	if len(updates) == 0 {
		return nil
	}
	if flowContext == nil {
		return fmt.Errorf("flow boundary context is required")
	}

	// Validate batch size
	if len(updates) > r.flowConfig.BatchSize {
		return fmt.Errorf("batch size %d exceeds maximum %d", len(updates), r.flowConfig.BatchSize)
	}

	// Convert to contextual updates
	contextualUpdates := make([]ContextualPositionUpdate, 0, len(updates))

	for _, update := range updates {
		// Validate flow boundary for each node
		if r.flowConfig.EnableBoundaryValidation {
			if err := r.ValidateFlowBoundary(ctx, update.NodeID, flowContext.FlowID); err != nil {
				return fmt.Errorf("flow boundary validation failed for node %s: %w",
					update.NodeID.String(), err)
			}
		}

		// Validate spatial position
		if update.OrderType == FlowNodeOrderSpatial {
			if err := r.validateSpatialPosition(update.Position); err != nil {
				return fmt.Errorf("invalid spatial position for node %s: %w",
					update.NodeID.String(), err)
			}
		}

		// Determine position value based on order type
		var position int
		if update.OrderType == FlowNodeOrderSpatial {
			position = r.encodeSpatialPosition(update.Position)
		} else {
			position = update.Sequential
		}

		// Create context metadata
		contextMeta := &ContextMetadata{
			Type:      ContextFlow,
			ScopeID:   flowContext.FlowID.Bytes(),
			IsHidden:  false,
			Priority:  0,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		contextualUpdate := ContextualPositionUpdate{
			ItemID:   update.NodeID,
			ListType: FlowListTypeNodes,
			Position: position,
			Context:  contextMeta,
			IsDelta:  update.DeltaRef != nil,
		}

		if update.DeltaRef != nil {
			// Handle REQUEST node delta update
			if err := r.handleRequestNodeDeltaUpdate(ctx, update.NodeID, update.DeltaRef); err != nil {
				return fmt.Errorf("failed to handle delta update for node %s: %w",
					update.NodeID.String(), err)
			}
		}

		contextualUpdates = append(contextualUpdates, contextualUpdate)
	}

	// Perform batch update
	return r.UpdatePositionsWithContext(ctx, tx, contextualUpdates)
}

// ValidateFlowBoundary ensures all operations stay within flow scope
func (r *EnhancedFlowNodeRepositoryImpl) ValidateFlowBoundary(ctx context.Context,
	nodeID idwrap.IDWrap, expectedFlowID idwrap.IDWrap) error {

	if !r.flowConfig.EnableBoundaryValidation {
		return nil
	}

	// This would typically query the database to get the actual flow ID for the node
	// For now, we'll use a simplified validation using the scope resolver
	if r.scopeResolver != nil {
		actualFlowScope, err := r.scopeResolver.ResolveScopeID(ctx, nodeID, ContextFlow)
		if err != nil {
			return fmt.Errorf("failed to resolve flow scope: %w", err)
		}

		if actualFlowScope.Compare(expectedFlowID) != 0 {
			return fmt.Errorf("node %s belongs to flow %s, not expected flow %s",
				nodeID.String(), actualFlowScope.String(), expectedFlowID.String())
		}
	}

	return nil
}

// =============================================================================
// REQUEST NODE DELTA OPERATIONS IMPLEMENTATION
// =============================================================================

// HandleRequestNode manages REQUEST node with delta references
func (r *EnhancedFlowNodeRepositoryImpl) HandleRequestNode(ctx context.Context, tx *sql.Tx,
	nodeID idwrap.IDWrap, deltaRef *FlowNodeDeltaReference) error {

	if deltaRef == nil {
		return fmt.Errorf("delta reference is required for REQUEST node")
	}

	// Ensure node ID and flow ID are consistent
	deltaRef.NodeID = nodeID
	deltaRef.IsActive = true

	// Store delta reference
	r.deltasMutex.Lock()
	r.requestNodeDeltas[nodeID.String()] = deltaRef
	r.deltasMutex.Unlock()

	// Create context metadata for the REQUEST node
	contextMeta := &ContextMetadata{
		Type:      ContextFlow,
		ScopeID:   deltaRef.FlowID.Bytes(),
		IsHidden:  false,
		Priority:  10, // Higher priority for REQUEST nodes with deltas
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Cache the context
	r.contextCache.SetContext(nodeID, contextMeta)

	return nil
}

// ResolveRequestNodeDeltas resolves effective endpoints/examples for REQUEST nodes
func (r *EnhancedFlowNodeRepositoryImpl) ResolveRequestNodeDeltas(ctx context.Context,
	nodeID idwrap.IDWrap, flowContext *FlowBoundaryContext) (*ResolvedRequestNode, error) {

	// Get delta reference for the node
	r.deltasMutex.RLock()
	deltaRef, exists := r.requestNodeDeltas[nodeID.String()]
	r.deltasMutex.RUnlock()

	if !exists || !deltaRef.IsActive {
		// No delta reference, return nil
		return nil, nil
	}

	// Build resolved request node
	resolved := &ResolvedRequestNode{
		NodeID:           nodeID,
		OriginEndpointID: deltaRef.EndpointID,
		OriginExampleID:  deltaRef.ExampleID,
		DeltaEndpointID:  deltaRef.DeltaEndpointID,
		DeltaExampleID:   deltaRef.DeltaExampleID,
		Context:          flowContext,
		ResolutionTime:   time.Now(),
		HasDeltas:        false,
	}

	// Determine effective IDs based on delta availability
	emptyID := idwrap.IDWrap{}
	if deltaRef.DeltaEndpointID != nil && deltaRef.DeltaEndpointID.Compare(emptyID) != 0 {
		resolved.EffectiveEndpointID = deltaRef.DeltaEndpointID
		resolved.HasDeltas = true
	} else if deltaRef.EndpointID != nil {
		resolved.EffectiveEndpointID = deltaRef.EndpointID
	}

	if deltaRef.DeltaExampleID != nil && deltaRef.DeltaExampleID.Compare(emptyID) != 0 {
		resolved.EffectiveExampleID = deltaRef.DeltaExampleID
		resolved.HasDeltas = true
	} else if deltaRef.ExampleID != nil {
		resolved.EffectiveExampleID = deltaRef.ExampleID
	}

	return resolved, nil
}

// GetRequestNodeDeltas returns all delta references for REQUEST nodes in a flow
func (r *EnhancedFlowNodeRepositoryImpl) GetRequestNodeDeltas(ctx context.Context,
	flowID idwrap.IDWrap) ([]FlowNodeDeltaReference, error) {

	r.deltasMutex.RLock()
	defer r.deltasMutex.RUnlock()

	flowIDStr := flowID.String()
	deltas := make([]FlowNodeDeltaReference, 0)

	for _, deltaRef := range r.requestNodeDeltas {
		if deltaRef.IsActive && deltaRef.FlowID.String() == flowIDStr {
			deltas = append(deltas, *deltaRef)
		}
	}

	return deltas, nil
}

// UpdateRequestNodeDeltas updates delta references for a REQUEST node
func (r *EnhancedFlowNodeRepositoryImpl) UpdateRequestNodeDeltas(ctx context.Context, tx *sql.Tx,
	nodeID idwrap.IDWrap, deltaRef *FlowNodeDeltaReference) error {

	if deltaRef == nil {
		// Remove delta reference
		r.deltasMutex.Lock()
		delete(r.requestNodeDeltas, nodeID.String())
		r.deltasMutex.Unlock()

		// Remove context cache
		r.contextCache.InvalidateContext(nodeID)
		return nil
	}

	// Update delta reference
	deltaRef.NodeID = nodeID
	deltaRef.IsActive = true

	r.deltasMutex.Lock()
	r.requestNodeDeltas[nodeID.String()] = deltaRef
	r.deltasMutex.Unlock()

	// Update context cache
	contextMeta := &ContextMetadata{
		Type:      ContextFlow,
		ScopeID:   deltaRef.FlowID.Bytes(),
		IsHidden:  false,
		Priority:  10,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	r.contextCache.SetContext(nodeID, contextMeta)

	return nil
}

// =============================================================================
// FLOW VARIABLE OPERATIONS IMPLEMENTATION
// =============================================================================

// GetFlowVariablesOrder returns variables in sequential order
func (r *EnhancedFlowNodeRepositoryImpl) GetFlowVariablesOrder(ctx context.Context,
	flowID idwrap.IDWrap) ([]FlowNodeItem, error) {

	// Get items using flow variables list type
	baseItems, err := r.GetItemsByParent(ctx, flowID, FlowListTypeVariables)
	if err != nil {
		return nil, fmt.Errorf("failed to get flow variables: %w", err)
	}

	// Convert to flow node items with sequential ordering
	flowNodes := make([]FlowNodeItem, 0, len(baseItems))
	for _, item := range baseItems {
		flowNode, err := r.convertToFlowNodeItem(ctx, item, FlowNodeOrderSequential)
		if err != nil {
			return nil, fmt.Errorf("failed to convert variable %s: %w", item.ID.String(), err)
		}
		if flowNode != nil {
			flowNodes = append(flowNodes, *flowNode)
		}
	}

	// Sort by sequential position
	r.sortFlowNodes(flowNodes, FlowNodeOrderSequential)

	return flowNodes, nil
}

// ReorderFlowVariable moves a flow variable in sequential ordering
func (r *EnhancedFlowNodeRepositoryImpl) ReorderFlowVariable(ctx context.Context, tx *sql.Tx,
	variableID idwrap.IDWrap, newPosition int, flowContext *FlowBoundaryContext) error {

	emptyID := idwrap.IDWrap{}
	if variableID.Compare(emptyID) == 0 {
		return fmt.Errorf("variable ID is required")
	}
	if newPosition < 0 {
		return fmt.Errorf("position must be non-negative")
	}
	if flowContext == nil {
		return fmt.Errorf("flow boundary context is required")
	}

	// Validate flow boundary
	if r.flowConfig.EnableBoundaryValidation {
		if err := r.ValidateFlowBoundary(ctx, variableID, flowContext.FlowID); err != nil {
			return fmt.Errorf("flow boundary validation failed: %w", err)
		}
	}

	// Create context metadata
	contextMeta := &ContextMetadata{
		Type:      ContextFlow,
		ScopeID:   flowContext.FlowID.Bytes(),
		IsHidden:  false,
		Priority:  0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Update position using enhanced repository
	return r.UpdatePositionWithContext(ctx, tx, variableID, FlowListTypeVariables,
		newPosition, contextMeta)
}

// =============================================================================
// BATCH OPERATIONS FOR PERFORMANCE
// =============================================================================

// BatchCreateFlowNodes efficiently creates multiple flow nodes
func (r *EnhancedFlowNodeRepositoryImpl) BatchCreateFlowNodes(ctx context.Context, tx *sql.Tx,
	nodes []FlowNodeItem) error {

	if len(nodes) == 0 {
		return nil
	}

	// Validate batch size
	if len(nodes) > r.flowConfig.BatchSize {
		return fmt.Errorf("batch size %d exceeds maximum %d", len(nodes), r.flowConfig.BatchSize)
	}

	// Group by flow ID for validation
	flowGroups := make(map[idwrap.IDWrap][]FlowNodeItem)
	for _, node := range nodes {
		flowGroups[node.FlowID] = append(flowGroups[node.FlowID], node)
	}

	// Validate node counts per flow
	for flowID, flowNodes := range flowGroups {
		if len(flowNodes) > r.flowConfig.MaxNodesPerFlow {
			return fmt.Errorf("flow %s would exceed maximum nodes limit", flowID.String())
		}
	}

	// Process creation
	for _, node := range nodes {
		// Handle REQUEST node delta references
		if node.DeltaRef != nil {
			if err := r.HandleRequestNode(ctx, tx, node.ID, node.DeltaRef); err != nil {
				return fmt.Errorf("failed to handle REQUEST node %s: %w", node.ID.String(), err)
			}
		}

		// Create context metadata
		contextMeta := &ContextMetadata{
			Type:      ContextFlow,
			ScopeID:   node.FlowID.Bytes(),
			IsHidden:  false,
			Priority:  0,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Cache context
		r.contextCache.SetContext(node.ID, contextMeta)
	}

	return nil
}

// BatchResolveFlowNodes efficiently resolves multiple nodes with deltas
func (r *EnhancedFlowNodeRepositoryImpl) BatchResolveFlowNodes(ctx context.Context,
	nodeIDs []idwrap.IDWrap, flowContext *FlowBoundaryContext) (map[idwrap.IDWrap]*FlowNodeItem, error) {

	results := make(map[idwrap.IDWrap]*FlowNodeItem, len(nodeIDs))

	for _, nodeID := range nodeIDs {
		// Get base node information (this would typically involve database queries)
		// For now, we'll create a basic node item
		nodeItem := &FlowNodeItem{
			ID:        nodeID,
			FlowID:    flowContext.FlowID,
			OrderType: FlowNodeOrderSpatial, // Default to spatial
		}

		// Resolve REQUEST node deltas if applicable
		if resolved, err := r.ResolveRequestNodeDeltas(ctx, nodeID, flowContext); err == nil && resolved != nil {
			// Create delta reference based on resolved information
			deltaRef := &FlowNodeDeltaReference{
				NodeID:          nodeID,
				EndpointID:      resolved.OriginEndpointID,
				ExampleID:       resolved.OriginExampleID,
				DeltaEndpointID: resolved.DeltaEndpointID,
				DeltaExampleID:  resolved.DeltaExampleID,
				Context:         ContextFlow,
				FlowID:          flowContext.FlowID,
				IsActive:        resolved.HasDeltas,
			}
			nodeItem.DeltaRef = deltaRef
		}

		// Get cached context
		if contextMeta, found := r.contextCache.GetContext(nodeID); found {
			nodeItem.Context = contextMeta
		}

		results[nodeID] = nodeItem
	}

	return results, nil
}

// CompactFlowPositions rebalances all positions within a flow
func (r *EnhancedFlowNodeRepositoryImpl) CompactFlowPositions(ctx context.Context, tx *sql.Tx,
	flowID idwrap.IDWrap) error {

	if !r.flowConfig.EnablePositionCompaction {
		return nil // Compaction disabled
	}

	// Get all nodes in the flow
	spatialNodes, err := r.GetNodesInFlow(ctx, flowID, FlowNodeOrderSpatial)
	if err != nil {
		return fmt.Errorf("failed to get spatial nodes: %w", err)
	}

	variables, err := r.GetFlowVariablesOrder(ctx, flowID)
	if err != nil {
		return fmt.Errorf("failed to get flow variables: %w", err)
	}

	// Compact spatial positions if needed
	if len(spatialNodes) > 0 && r.shouldCompactSpatialPositions(spatialNodes) {
		if err := r.compactSpatialPositions(ctx, tx, spatialNodes, flowID); err != nil {
			return fmt.Errorf("failed to compact spatial positions: %w", err)
		}
	}

	// Compact sequential positions for variables
	if len(variables) > 0 {
		if err := r.compactSequentialPositions(ctx, tx, variables, flowID, FlowListTypeVariables); err != nil {
			return fmt.Errorf("failed to compact variable positions: %w", err)
		}
	}

	return nil
}

// =============================================================================
// HELPER METHODS
// =============================================================================

// convertToFlowNodeItem converts a basic MovableItem to FlowNodeItem
func (r *EnhancedFlowNodeRepositoryImpl) convertToFlowNodeItem(ctx context.Context,
	item MovableItem, orderType FlowNodeOrderType) (*FlowNodeItem, error) {

	flowNode := &FlowNodeItem{
		ID:        item.ID,
		FlowID:    *item.ParentID, // Flow ID is the parent
		ParentID:  nil,            // Top-level node
		ListType:  item.ListType,
		OrderType: orderType,
	}

	// Decode position based on order type
	if orderType == FlowNodeOrderSpatial {
		flowNode.Position = r.decodeSpatialPosition(item.Position)
		flowNode.Sequential = 0
	} else {
		flowNode.Sequential = item.Position
		flowNode.Position = FlowNodePosition{X: 0, Y: 0}
	}

	// Get cached context
	if contextMeta, found := r.contextCache.GetContext(item.ID); found {
		flowNode.Context = contextMeta
	}

	// Get delta reference for REQUEST nodes
	r.deltasMutex.RLock()
	if deltaRef, exists := r.requestNodeDeltas[item.ID.String()]; exists && deltaRef.IsActive {
		flowNode.DeltaRef = deltaRef
	}
	r.deltasMutex.RUnlock()

	return flowNode, nil
}

// sortFlowNodes sorts flow nodes based on the order type
func (r *EnhancedFlowNodeRepositoryImpl) sortFlowNodes(nodes []FlowNodeItem, orderType FlowNodeOrderType) {
	if len(nodes) <= 1 {
		return
	}

	// Simple bubble sort implementation for this example
	// In production, consider using sort.Slice for better performance
	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			var shouldSwap bool

			if orderType == FlowNodeOrderSpatial {
				// Sort by Y first, then X (top to bottom, left to right)
				if nodes[i].Position.Y > nodes[j].Position.Y {
					shouldSwap = true
				} else if nodes[i].Position.Y == nodes[j].Position.Y && nodes[i].Position.X > nodes[j].Position.X {
					shouldSwap = true
				}
			} else {
				// Sort by sequential position
				shouldSwap = nodes[i].Sequential > nodes[j].Sequential
			}

			if shouldSwap {
				nodes[i], nodes[j] = nodes[j], nodes[i]
			}
		}
	}
}

// validateSpatialPosition validates spatial coordinates
func (r *EnhancedFlowNodeRepositoryImpl) validateSpatialPosition(position FlowNodePosition) error {
	if position.X < 0 || position.Y < 0 {
		return fmt.Errorf("coordinates must be non-negative")
	}
	if position.X > r.flowConfig.MaxSpatialCoordinate || position.Y > r.flowConfig.MaxSpatialCoordinate {
		return fmt.Errorf("coordinates exceed maximum allowed value")
	}
	return nil
}

// encodeSpatialPosition encodes spatial coordinates into a single integer
func (r *EnhancedFlowNodeRepositoryImpl) encodeSpatialPosition(position FlowNodePosition) int {
	// Simple encoding: use Y * 10000 + X
	return int(position.Y*10000 + position.X)
}

// decodeSpatialPosition decodes spatial coordinates from an integer
func (r *EnhancedFlowNodeRepositoryImpl) decodeSpatialPosition(encoded int) FlowNodePosition {
	y := float64(encoded / 10000)
	x := float64(encoded % 10000)
	return FlowNodePosition{X: x, Y: y}
}

// handleRequestNodeDeltaUpdate handles delta updates for REQUEST nodes
func (r *EnhancedFlowNodeRepositoryImpl) handleRequestNodeDeltaUpdate(ctx context.Context,
	nodeID idwrap.IDWrap, deltaRef *FlowNodeDeltaReference) error {

	r.deltasMutex.Lock()
	defer r.deltasMutex.Unlock()

	deltaRef.NodeID = nodeID
	deltaRef.IsActive = true
	r.requestNodeDeltas[nodeID.String()] = deltaRef

	return nil
}

// shouldCompactSpatialPositions determines if spatial positions need compaction
func (r *EnhancedFlowNodeRepositoryImpl) shouldCompactSpatialPositions(nodes []FlowNodeItem) bool {
	if len(nodes) == 0 {
		return false
	}

	// Check if any position exceeds the compaction threshold
	for _, node := range nodes {
		if node.Position.X > r.flowConfig.PositionCompactionThreshold ||
			node.Position.Y > r.flowConfig.PositionCompactionThreshold {
			return true
		}
	}

	return false
}

// compactSpatialPositions rebalances spatial positions
func (r *EnhancedFlowNodeRepositoryImpl) compactSpatialPositions(ctx context.Context, tx *sql.Tx,
	nodes []FlowNodeItem, flowID idwrap.IDWrap) error {

	if len(nodes) == 0 {
		return nil
	}

	// Create a grid layout for compaction
	gridSize := r.flowConfig.DefaultSpatialGrid
	cols := int(float64(len(nodes))*0.5 + 0.5) // Approximate square layout
	if cols < 1 {
		cols = 1
	}

	updates := make([]FlowNodePositionUpdate, len(nodes))
	for i, node := range nodes {
		row := i / cols
		col := i % cols

		newPosition := FlowNodePosition{
			X: float64(col) * gridSize,
			Y: float64(row) * gridSize,
		}

		updates[i] = FlowNodePositionUpdate{
			NodeID:    node.ID,
			Position:  newPosition,
			OrderType: FlowNodeOrderSpatial,
			Context:   node.Context,
			DeltaRef:  node.DeltaRef,
		}
	}

	// Create flow boundary context
	flowContext := &FlowBoundaryContext{
		FlowID:        flowID,
		EnforceStrict: false,
	}

	return r.BatchUpdateNodePositions(ctx, tx, updates, flowContext)
}

// compactSequentialPositions rebalances sequential positions
func (r *EnhancedFlowNodeRepositoryImpl) compactSequentialPositions(ctx context.Context, tx *sql.Tx,
	nodes []FlowNodeItem, flowID idwrap.IDWrap, listType ListType) error {

	if len(nodes) == 0 {
		return nil
	}

	// Create sequential updates
	contextualUpdates := make([]ContextualPositionUpdate, len(nodes))
	for i, node := range nodes {
		contextualUpdates[i] = ContextualPositionUpdate{
			ItemID:   node.ID,
			ListType: listType,
			Position: i, // Sequential positions starting from 0
			Context:  node.Context,
			IsDelta:  node.DeltaRef != nil,
		}
	}

	return r.UpdatePositionsWithContext(ctx, tx, contextualUpdates)
}
