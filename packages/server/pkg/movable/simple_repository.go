package movable

import (
	"context"
	"database/sql"
	"fmt"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
)

// =============================================================================
// TYPES NEEDED FOR SIMPLE REPOSITORY
// =============================================================================

// Item represents any entity that can participate in linked list operations
// Replaces complex generic TEntity/TOrderedRow types with simple interface
type Item interface {
	GetID() idwrap.IDWrap
	GetPrev() *idwrap.IDWrap
	GetNext() *idwrap.IDWrap
	GetParentID() idwrap.IDWrap
	GetPosition() int
}

// Update represents a single pointer/position update operation
type Update struct {
	ItemID   idwrap.IDWrap
	Prev     *idwrap.IDWrap
	Next     *idwrap.IDWrap
	Position int
}

// EntityData provides common fields from any entity
type EntityData struct {
	ID       idwrap.IDWrap
	Prev     *idwrap.IDWrap
	Next     *idwrap.IDWrap
	ParentID idwrap.IDWrap
	Position int
}

// Note: ParentScopeConfig and related types are defined in config_structs.go

// Minimal MoveConfig for compilation - would be enhanced in practice
type MoveConfig struct {
	EntityOperations  EntityOperations
	QueryOperations   QueryOperations
	ParentScope      ParentScopeConfig
}

// Minimal EntityOperations for compilation
type EntityOperations struct {
	GetEntity    func(ctx context.Context, queries *gen.Queries, id idwrap.IDWrap) (interface{}, error)
	UpdateOrder  func(ctx context.Context, queries *gen.Queries, id idwrap.IDWrap, prev, next *idwrap.IDWrap) error
	ExtractData  func(entity interface{}) EntityData
}

// Minimal QueryOperations for compilation
type QueryOperations struct {
	BuildOrderedQuery func(config QueryConfig) (string, []interface{})
}

// Minimal QueryConfig for compilation
type QueryConfig struct {
	TableName       string
	IDColumn        string
	PrevColumn      string
	NextColumn      string
	PositionColumn  string
	ParentID        idwrap.IDWrap
	ParentScope     ParentScopeConfig
}

// Minimal ParentScopeConfig for compilation
type ParentScopeConfig struct {
	Pattern ParentScopePattern
}

// ParentScopePattern enum
type ParentScopePattern int

const (
	DirectFKPattern ParentScopePattern = iota
	JoinTablePattern
	UserLookupPattern
)

// CalculatePointerUpdates - placeholder function to avoid compilation error
// This would be replaced by the actual pure function implementation
func CalculatePointerUpdates(items []Item, targetID, itemID string, position MovePosition) ([]Update, error) {
	// Placeholder implementation - to be replaced with actual pure function
	return []Update{}, fmt.Errorf("CalculatePointerUpdates not implemented")
}

// ValidateMove - placeholder function to avoid compilation error
// This would be replaced by the actual pure function implementation
func ValidateMove(itemID, targetID, parentID string) error {
	// Placeholder implementation - to be replaced with actual pure function
	if itemID == "" || targetID == "" {
		return fmt.Errorf("ValidateMove not implemented")
	}
	return nil
}

// =============================================================================
// SIMPLE REPOSITORY STRUCT - COMPOSITION PATTERN
// =============================================================================

// SimpleRepository replaces BaseLinkedListRepository with composition pattern
// Uses pure functions from Phase 2A and configuration from Phase 2B
type SimpleRepository struct {
	// Essential fields only - no inheritance, no generic types
	db     *sql.DB          // Database connection
	config *MoveConfig      // Configuration from Phase 2B
	ops    *OrderOperations // Pure function operations from Phase 2A
	
	// Internal state (minimal)
	queries *gen.Queries    // Generated queries
}

// OrderOperations composes pure functions from Phase 2A
type OrderOperations struct {
	// Core position operations
	CalculatePositions     func([]Item, string, string, MovePosition) ([]Update, error)
	MoveItem              func([]Item, string, string, MovePosition) ([]Item, error)
	MoveItemAfter         func([]Item, string, string) ([]Item, error)
	MoveItemBefore        func([]Item, string, string) ([]Item, error)
	
	// Validation operations
	ValidateMove          func(string, string, string) error
	ValidateOrdering      func([]Item) error
	ValidateParentScope   func([]Item, idwrap.IDWrap) error
	
	// Batch operations  
	BatchMove             func([]Item, []BatchMoveOperation) ([]Item, error)
	BatchInsert           func([]Item, []Item, []int) ([]Item, error)
	
	// Gap management
	FindGaps              func([]Item) []RepositoryGap
	RebalancePositions    func([]Item) []Item
	
	// Utility operations
	FindItemByID          func([]Item, string) (*Item, int, bool)
	GetOrderedItemIDs     func([]Item) []string
	UpdatePrevNextPointers func([]Item) []Item
}

// RepositoryGap represents a gap in position sequence (avoid conflict with pure_functions.go Gap)
type RepositoryGap struct {
	StartPosition int
	EndPosition   int
	GapSize       int
}

// BatchMoveOperation represents a move operation for batch processing (avoid conflict with MoveOperation)
type BatchMoveOperation struct {
	ItemID   idwrap.IDWrap
	ListType ListType
	Position MovePosition
}

// NewOrderOperations creates composed operations using Phase 2A pure functions
func NewOrderOperations() *OrderOperations {
	return &OrderOperations{
		CalculatePositions:     CalculatePointerUpdates,     // From Phase 2A pure_functions.go
		MoveItem:              performMoveItem,              // Composition function
		MoveItemAfter:         performMoveItemAfter,         // Composition function
		MoveItemBefore:        performMoveItemBefore,        // Composition function
		ValidateMove:          ValidateMove,                 // From Phase 2A pure_functions.go
		ValidateOrdering:      validateOrdering,             // Helper function
		ValidateParentScope:   validateParentScope,          // Helper function
		BatchMove:            performBatchMove,              // Batch function
		BatchInsert:          performBatchInsert,            // Batch function
		FindGaps:             findGaps,                      // Gap detection
		RebalancePositions:   rebalancePositions,           // Rebalancing
		FindItemByID:         findItemByID,                 // Search function
		GetOrderedItemIDs:    getOrderedItemIDs,            // Utility function
		UpdatePrevNextPointers: updatePrevNextPointers,     // Pointer update
	}
}

// =============================================================================
// SIMPLE REPOSITORY CONSTRUCTOR
// =============================================================================

// NewSimpleRepository creates a repository with pure function composition
func NewSimpleRepository(db *sql.DB, config *MoveConfig) *SimpleRepository {
	return &SimpleRepository{
		db:      db,
		config:  config,
		ops:     NewOrderOperations(),
		queries: gen.New(db),
	}
}

// TX returns transaction-aware repository instance
func (r *SimpleRepository) TX(tx *sql.Tx) MovableRepository {
	return &SimpleRepository{
		db:      r.db,
		config:  r.config,     // Configuration is immutable and shared
		ops:     r.ops,        // Pure functions are stateless and shared
		queries: r.queries.WithTx(tx),
	}
}

// =============================================================================
// MOVABLE REPOSITORY INTERFACE IMPLEMENTATION
// =============================================================================

// UpdatePosition updates the position of an item in a specific list type
// EXACT interface from Phase 2C analysis - maintains compatibility
func (r *SimpleRepository) UpdatePosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, listType ListType, position int) error {
	// Input validation
	if position < 0 {
		return fmt.Errorf("invalid position: %d (must be >= 0)", position)
	}
	
	// Get transaction-aware repository
	repo := r
	if tx != nil {
		repo = r.TX(tx).(*SimpleRepository)
	}
	
	// Step 1: Get current items using configuration
	items, parentID, err := repo.getCurrentItems(ctx, itemID)
	if err != nil {
		return err
	}
	
	// Step 2: Validate position bounds (Phase 2C contract)
	if position >= len(items) {
		return fmt.Errorf("invalid position: %d (valid range: 0-%d)", position, len(items)-1)
	}
	
	// Step 3: Use pure function from Phase 2A for move logic
	targetPosition := MovePositionAfter
	if position == 0 {
		targetPosition = MovePositionBefore
	}
	
	// Find item at target position
	var targetID string
	if position < len(items) {
		targetID = items[position].GetID().String()
	} else {
		targetID = items[len(items)-1].GetID().String()
	}
	
	updates, err := repo.ops.CalculatePositions(items, targetID, itemID.String(), targetPosition)
	if err != nil {
		return fmt.Errorf("move operation failed: %w", err)
	}
	
	// Step 4: Persist changes using configured operations
	return repo.persistMoveUpdates(ctx, updates, parentID)
}

// UpdatePositions updates positions for multiple items in batch
// EXACT interface from Phase 2C analysis - maintains compatibility
func (r *SimpleRepository) UpdatePositions(ctx context.Context, tx *sql.Tx, updates []PositionUpdate) error {
	// Phase 2C contract: handle empty slice
	if len(updates) == 0 {
		return nil
	}
	
	// Get transaction-aware repository
	repo := r
	if tx != nil {
		repo = r.TX(tx).(*SimpleRepository)
	}
	
	// Group by parent (Phase 2C contract requirement)
	parentGroups, err := repo.groupUpdatesByParent(ctx, updates)
	if err != nil {
		return err
	}
	
	// Process each parent group atomically
	for parentID, groupUpdates := range parentGroups {
		items, err := repo.getItemsByParentID(ctx, parentID)
		if err != nil {
			return fmt.Errorf("failed to get items for parent %s: %w", parentID.String(), err)
		}
		
		// Convert to batch move operations
		moveOps := repo.convertToBatchMoves(groupUpdates)
		
		// Use pure function from Phase 2A for batch processing
		updatedItems, err := repo.ops.BatchMove(items, moveOps)
		if err != nil {
			return fmt.Errorf("batch move failed for parent %s: %w", parentID.String(), err)
		}
		
		// Convert back to updates
		itemUpdates := repo.convertItemsToUpdates(updatedItems)
		
		// Persist batch changes
		if err := repo.persistMoveUpdates(ctx, itemUpdates, parentID); err != nil {
			return err
		}
	}
	
	return nil
}

// GetMaxPosition returns the maximum position value for a list
// EXACT interface from Phase 2C analysis - maintains compatibility
func (r *SimpleRepository) GetMaxPosition(ctx context.Context, parentID idwrap.IDWrap, listType ListType) (int, error) {
	items, err := r.getItemsByParentID(ctx, parentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return -1, nil  // Phase 2C contract: -1 for empty lists
		}
		return 0, err
	}
	
	if len(items) == 0 {
		return -1, nil  // Phase 2C contract: -1 for empty lists
	}
	
	return len(items) - 1, nil  // Phase 2C contract: count-1 for non-empty lists
}

// GetItemsByParent returns all items under a parent, ordered by position
// EXACT interface from Phase 2C analysis - maintains compatibility
func (r *SimpleRepository) GetItemsByParent(ctx context.Context, parentID idwrap.IDWrap, listType ListType) ([]MovableItem, error) {
	items, err := r.getItemsByParentID(ctx, parentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []MovableItem{}, nil  // Phase 2C contract: empty slice, not nil
		}
		return nil, err
	}
	
	// Convert to MovableItem structs (Phase 2C contract)
	movableItems := make([]MovableItem, len(items))
	for i, item := range items {
		movableItems[i] = MovableItem{
			ID:       item.GetID(),
			ParentID: &parentID,
			Position: item.GetPosition(),
			ListType: listType,
		}
	}
	
	return movableItems, nil
}

// =============================================================================
// REPOSITORY HELPER FUNCTIONS - FUNCTION COMPOSITION
// =============================================================================

// getCurrentItems gets current ordered items using configuration
func (r *SimpleRepository) getCurrentItems(ctx context.Context, itemID idwrap.IDWrap) ([]Item, idwrap.IDWrap, error) {
	// Step 1: Get entity using configured operation
	entity, err := r.config.EntityOperations.GetEntity(ctx, r.queries, itemID)
	if err != nil {
		return nil, idwrap.IDWrap{}, fmt.Errorf("failed to get entity: %w", err)
	}
	
	// Step 2: Extract parent ID using configured extraction
	entityData := r.config.EntityOperations.ExtractData(entity)
	parentID := entityData.ParentID
	
	// Step 3: Get ordered items using configured query
	query, args := r.config.QueryOperations.BuildOrderedQuery(QueryConfig{
		TableName:       "items", // Would be configured per repository
		IDColumn:        "id",
		PrevColumn:      "prev",
		NextColumn:      "next", 
		PositionColumn:  "position",
		ParentID:        parentID,
		ParentScope:     r.config.ParentScope,
	})
	
	items, err := r.executeOrderedQuery(ctx, nil, query, args)
	if err != nil {
		return nil, idwrap.IDWrap{}, fmt.Errorf("failed to execute ordered query: %w", err)
	}
	
	return items, parentID, nil
}

// getItemsByParentID retrieves items by parent ID using configuration
func (r *SimpleRepository) getItemsByParentID(ctx context.Context, parentID idwrap.IDWrap) ([]Item, error) {
	query, args := r.config.QueryOperations.BuildOrderedQuery(QueryConfig{
		TableName:       "items", // Would be configured per repository
		IDColumn:        "id",
		PrevColumn:      "prev",
		NextColumn:      "next",
		PositionColumn:  "position",
		ParentID:        parentID,
		ParentScope:     r.config.ParentScope,
	})
	
	return r.executeOrderedQuery(ctx, nil, query, args)
}

// persistMoveUpdates persists changes using configured operations
func (r *SimpleRepository) persistMoveUpdates(ctx context.Context, updates []Update, parentID idwrap.IDWrap) error {
	// Use pure function to ensure pointer consistency
	optimizedUpdates := r.ops.UpdatePrevNextPointers(r.convertUpdatesToItems(updates))
	finalUpdates := r.convertItemsToUpdates(optimizedUpdates)
	
	// Persist each item using configured update operation
	for _, update := range finalUpdates {
		err := r.config.EntityOperations.UpdateOrder(ctx, r.queries, 
			update.ItemID, update.Prev, update.Next)
		if err != nil {
			return fmt.Errorf("failed to update item %s: %w", update.ItemID.String(), err)
		}
	}
	
	return nil
}

// executeOrderedQuery executes configured ordered query and converts to Item
func (r *SimpleRepository) executeOrderedQuery(ctx context.Context, tx *sql.Tx, query string, args []interface{}) ([]Item, error) {
	// Execute query using appropriate database connection
	db := r.db
	if tx != nil {
		// Use transaction if provided
		rows, err := tx.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return r.scanItemRows(rows)
	}
	
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	return r.scanItemRows(rows)
}

// scanItemRows converts SQL rows to Item structs
func (r *SimpleRepository) scanItemRows(rows *sql.Rows) ([]Item, error) {
	var items []Item
	
	for rows.Next() {
		var item GenericItem
		var prevBytes, nextBytes []byte
		
		err := rows.Scan(&item.ID, &prevBytes, &nextBytes, &item.Position)
		if err != nil {
			return nil, err
		}
		
		// Handle nullable prev/next pointers
		if prevBytes != nil {
			prev := idwrap.NewFromBytesMust(prevBytes)
			item.Prev = &prev
		}
		if nextBytes != nil {
			next := idwrap.NewFromBytesMust(nextBytes)
			item.Next = &next
		}
		
		items = append(items, &item)
	}
	
	return items, rows.Err()
}

// groupUpdatesByParent groups position updates by parent for atomic processing
func (r *SimpleRepository) groupUpdatesByParent(ctx context.Context, updates []PositionUpdate) (map[idwrap.IDWrap][]PositionUpdate, error) {
	groups := make(map[idwrap.IDWrap][]PositionUpdate)
	
	for _, update := range updates {
		// Get entity to determine parent
		entity, err := r.config.EntityOperations.GetEntity(ctx, r.queries, update.ItemID)
		if err != nil {
			return nil, fmt.Errorf("failed to get entity %s: %w", update.ItemID.String(), err)
		}
		
		entityData := r.config.EntityOperations.ExtractData(entity)
		parentID := entityData.ParentID
		
		groups[parentID] = append(groups[parentID], update)
	}
	
	return groups, nil
}

// convertToBatchMoves converts PositionUpdate to BatchMoveOperation for pure functions
func (r *SimpleRepository) convertToBatchMoves(updates []PositionUpdate) []BatchMoveOperation {
	operations := make([]BatchMoveOperation, len(updates))
	
	for i, update := range updates {
		operations[i] = BatchMoveOperation{
			ItemID:   update.ItemID,
			ListType: update.ListType,
			Position: MovePositionAfter, // Default - would be refined based on actual position
		}
	}
	
	return operations
}

// convertUpdatesToItems converts Update structs to Item interface for processing
func (r *SimpleRepository) convertUpdatesToItems(updates []Update) []Item {
	items := make([]Item, len(updates))
	for i, update := range updates {
		items[i] = &GenericItem{
			ID:       update.ItemID,
			Prev:     update.Prev,
			Next:     update.Next,
			Position: update.Position,
		}
	}
	return items
}

// convertItemsToUpdates converts Item interface back to Update structs
func (r *SimpleRepository) convertItemsToUpdates(items []Item) []Update {
	updates := make([]Update, len(items))
	for i, item := range items {
		updates[i] = Update{
			ItemID:   item.GetID(),
			Prev:     item.GetPrev(),
			Next:     item.GetNext(),
			Position: item.GetPosition(),
		}
	}
	return updates
}

// =============================================================================
// GENERIC ITEM IMPLEMENTATION
// =============================================================================

// GenericItem implements Item interface for basic operations
type GenericItem struct {
	ID       idwrap.IDWrap
	Prev     *idwrap.IDWrap
	Next     *idwrap.IDWrap
	Position int
	ParentID idwrap.IDWrap
}

func (g *GenericItem) GetID() idwrap.IDWrap        { return g.ID }
func (g *GenericItem) GetPrev() *idwrap.IDWrap    { return g.Prev }
func (g *GenericItem) GetNext() *idwrap.IDWrap    { return g.Next }
func (g *GenericItem) GetPosition() int           { return g.Position }
func (g *GenericItem) GetParentID() idwrap.IDWrap { return g.ParentID }

// =============================================================================
// PURE FUNCTION IMPLEMENTATIONS - COMPOSITION HELPERS
// =============================================================================

// performMoveItem orchestrates move using pure functions
func performMoveItem(items []Item, targetID, itemID string, position MovePosition) ([]Item, error) {
	updates, err := CalculatePointerUpdates(items, targetID, itemID, position)
	if err != nil {
		return nil, err
	}
	
	// Apply updates to create new item list
	itemMap := make(map[string]Item)
	for _, item := range items {
		itemMap[item.GetID().String()] = item
	}
	
	// Create updated items
	updatedItems := make([]Item, len(items))
	copy(updatedItems, items)
	
	for _, update := range updates {
		if item, exists := itemMap[update.ItemID.String()]; exists {
			// Create updated item
			newItem := &GenericItem{
				ID:       update.ItemID,
				Prev:     update.Prev,
				Next:     update.Next,
				Position: update.Position,
				ParentID: item.GetParentID(),
			}
			
			// Replace in array
			for i, oldItem := range updatedItems {
				if oldItem.GetID().String() == update.ItemID.String() {
					updatedItems[i] = newItem
					break
				}
			}
		}
	}
	
	return updatedItems, nil
}

// performMoveItemAfter moves item after target
func performMoveItemAfter(items []Item, itemID, targetID string) ([]Item, error) {
	return performMoveItem(items, targetID, itemID, MovePositionAfter)
}

// performMoveItemBefore moves item before target
func performMoveItemBefore(items []Item, itemID, targetID string) ([]Item, error) {
	return performMoveItem(items, targetID, itemID, MovePositionBefore)
}

// validateOrdering checks if items are properly ordered
func validateOrdering(items []Item) error {
	if len(items) == 0 {
		return nil
	}
	
	// Check for duplicate positions
	positions := make(map[int]bool)
	for _, item := range items {
		pos := item.GetPosition()
		if positions[pos] {
			return fmt.Errorf("duplicate position found: %d", pos)
		}
		positions[pos] = true
	}
	
	return nil
}

// validateParentScope checks if all items belong to same parent
func validateParentScope(items []Item, expectedParent idwrap.IDWrap) error {
	for _, item := range items {
		if item.GetParentID().Compare(expectedParent) != 0 {
			return fmt.Errorf("item %s has wrong parent: expected %s, got %s", 
				item.GetID().String(), expectedParent.String(), item.GetParentID().String())
		}
	}
	return nil
}

// performBatchMove processes multiple moves efficiently  
func performBatchMove(items []Item, operations []BatchMoveOperation) ([]Item, error) {
	result := items
	
	// Process each operation sequentially
	for _, op := range operations {
		var err error
		result, err = performMoveItem(result, "", op.ItemID.String(), op.Position)
		if err != nil {
			return nil, fmt.Errorf("batch operation failed for item %s: %w", op.ItemID.String(), err)
		}
	}
	
	return result, nil
}

// performBatchInsert inserts multiple items efficiently
func performBatchInsert(items []Item, newItems []Item, positions []int) ([]Item, error) {
	// Simple implementation - would be optimized in practice
	result := append([]Item{}, items...)
	
	for i, newItem := range newItems {
		if i < len(positions) {
			// Insert at specified position
			pos := positions[i]
			if pos >= len(result) {
				result = append(result, newItem)
			} else {
				// Insert at position
				result = append(result[:pos+1], result[pos:]...)
				result[pos] = newItem
			}
		}
	}
	
	return result, nil
}

// findGaps identifies gaps in position sequence
func findGaps(items []Item) []RepositoryGap {
	if len(items) == 0 {
		return nil
	}
	
	var gaps []RepositoryGap
	positions := make([]int, len(items))
	for i, item := range items {
		positions[i] = item.GetPosition()
	}
	
	// Sort positions to find gaps
	for i := 1; i < len(positions); i++ {
		if positions[i] > positions[i-1]+1 {
			gaps = append(gaps, RepositoryGap{
				StartPosition: positions[i-1] + 1,
				EndPosition:   positions[i] - 1,
				GapSize:       positions[i] - positions[i-1] - 1,
			})
		}
	}
	
	return gaps
}

// rebalancePositions rebalances position values to eliminate gaps
func rebalancePositions(items []Item) []Item {
	if len(items) == 0 {
		return items
	}
	
	result := make([]Item, len(items))
	for i, item := range items {
		// Create new item with rebalanced position
		newItem := &GenericItem{
			ID:       item.GetID(),
			Prev:     item.GetPrev(),
			Next:     item.GetNext(),
			Position: i, // Sequential positions starting from 0
			ParentID: item.GetParentID(),
		}
		result[i] = newItem
	}
	
	return result
}

// findItemByID finds item by ID and returns item, index, and found flag
func findItemByID(items []Item, itemID string) (*Item, int, bool) {
	for i, item := range items {
		if item.GetID().String() == itemID {
			return &item, i, true
		}
	}
	return nil, -1, false
}

// getOrderedItemIDs returns ordered list of item IDs
func getOrderedItemIDs(items []Item) []string {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.GetID().String()
	}
	return ids
}

// updatePrevNextPointers ensures prev/next pointers are consistent
func updatePrevNextPointers(items []Item) []Item {
	if len(items) == 0 {
		return items
	}
	
	result := make([]Item, len(items))
	for i, item := range items {
		var prev, next *idwrap.IDWrap
		
		if i > 0 {
			prevID := items[i-1].GetID()
			prev = &prevID
		}
		if i < len(items)-1 {
			nextID := items[i+1].GetID()
			next = &nextID
		}
		
		// Create updated item
		result[i] = &GenericItem{
			ID:       item.GetID(),
			Prev:     prev,
			Next:     next,
			Position: i,
			ParentID: item.GetParentID(),
		}
	}
	
	return result
}