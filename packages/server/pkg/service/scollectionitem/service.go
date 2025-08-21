package scollectionitem

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemfolder"
	"the-dev-tools/server/pkg/movable"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"github.com/oklog/ulid/v2"
)

// CollectionItemService provides operations for managing collection items using the simplified reference-based architecture
// where collection_items is the PRIMARY table containing ordering logic, and legacy tables reference it via FK.
type CollectionItemService struct {
	queries           *gen.Queries
	repository        *CollectionItemsMovableRepository
	folderService     sitemfolder.ItemFolderService
	apiService        sitemapi.ItemApiService
	logger            *slog.Logger
}

// CollectionItemType represents the type of collection item
type CollectionItemType int8

const (
	CollectionItemTypeFolder   CollectionItemType = 0
	CollectionItemTypeEndpoint CollectionItemType = 1
)

// CollectionItem represents a unified collection item with type-specific data
type CollectionItem struct {
	ID             idwrap.IDWrap
	CollectionID   idwrap.IDWrap
	ParentFolderID *idwrap.IDWrap
	Name           string
	ItemType       CollectionItemType
	URL            *string // Only for endpoints
	Method         *string // Only for endpoints
	PrevID         *idwrap.IDWrap
	NextID         *idwrap.IDWrap
	// Reference IDs to legacy tables
	FolderID   *idwrap.IDWrap
	EndpointID *idwrap.IDWrap
}

var (
	ErrCollectionItemNotFound = fmt.Errorf("collection item not found")
	ErrInvalidItemType        = fmt.Errorf("invalid item type")
	ErrPositionOutOfRange     = fmt.Errorf("position out of range")
	ErrInvalidTargetPosition  = fmt.Errorf("invalid target position")
)

// New creates a new CollectionItemService
func New(queries *gen.Queries, logger *slog.Logger) *CollectionItemService {
	return &CollectionItemService{
		queries:       queries,
		repository:    NewCollectionItemsMovableRepository(queries),
		folderService: sitemfolder.New(queries),
		apiService:    sitemapi.New(queries),
		logger:        logger,
	}
}

// TX returns a new service instance with transaction support
func (s *CollectionItemService) TX(tx *sql.Tx) *CollectionItemService {
	return &CollectionItemService{
		queries:       s.queries.WithTx(tx),
		repository:    s.repository.TX(tx),
		folderService: s.folderService.TX(tx),
		apiService:    s.apiService.TX(tx),
		logger:        s.logger,
	}
}

// NewTX creates a new service instance with prepared transaction queries
func NewTX(ctx context.Context, tx *sql.Tx, logger *slog.Logger) (*CollectionItemService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare transaction queries: %w", err)
	}
	return &CollectionItemService{
		queries:       queries,
		repository:    NewCollectionItemsMovableRepository(queries),
		folderService: sitemfolder.New(queries),
		apiService:    sitemapi.New(queries),
		logger:        logger,
	}, nil
}

// ListCollectionItems retrieves all collection items in a parent context, ordered by position
// This reads from the collection_items table only for core data and ordering
func (s *CollectionItemService) ListCollectionItems(ctx context.Context, collectionID idwrap.IDWrap, parentFolderID *idwrap.IDWrap) ([]CollectionItem, error) {
	s.logger.Debug("Listing collection items",
		"collection_id", collectionID.String(),
		"parent_folder_id", getIDString(parentFolderID))

	// Get items in order from the primary collection_items table
	orderedItems, err := s.queries.GetCollectionItemsInOrder(ctx, gen.GetCollectionItemsInOrderParams{
		CollectionID:   collectionID,
		ParentFolderID: parentFolderID,
		CollectionID_2: collectionID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			s.logger.Debug("No collection items found")
			return []CollectionItem{}, nil
		}
		return nil, fmt.Errorf("failed to get collection items in order: %w", err)
	}

	// Convert to CollectionItem structs
	items := make([]CollectionItem, len(orderedItems))
	for i, item := range orderedItems {
		items[i] = CollectionItem{
			ID:             idwrap.NewFromBytesMust(item.ID),
			CollectionID:   collectionID,
			ParentFolderID: parentFolderID,
			Name:           item.Name,
			ItemType:       CollectionItemType(item.ItemType),
			URL:            nil, // Note: URL and Method not available in GetCollectionItemsInOrderRow
			Method:         nil, // Would need separate query to get these from legacy tables
			PrevID:         convertBytesToIDWrap(item.PrevID),
			NextID:         convertBytesToIDWrap(item.NextID),
			FolderID:       convertBytesToIDWrap(item.FolderID),
			EndpointID:     convertBytesToIDWrap(item.EndpointID),
		}
	}

	s.logger.Debug("Successfully retrieved collection items", "count", len(items))
	return items, nil
}

// MoveCollectionItem moves a collection item to a new position
// Updates only the prev/next pointers in the collection_items table
func (s *CollectionItemService) MoveCollectionItem(ctx context.Context, itemID idwrap.IDWrap, targetID *idwrap.IDWrap, position movable.MovePosition) error {
	s.logger.Debug("Moving collection item",
		"item_id", itemID.String(),
		"target_id", getIDString(targetID),
		"position", position)

	// Get the item to validate it exists
	item, err := s.queries.GetCollectionItem(ctx, itemID)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrCollectionItemNotFound
		}
		return fmt.Errorf("failed to get collection item: %w", err)
	}

	// Validate target exists if specified
	var targetItem *gen.CollectionItem
	if targetID != nil {
		target, err := s.queries.GetCollectionItem(ctx, *targetID)
		if err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("target item not found")
			}
			return fmt.Errorf("failed to get target item: %w", err)
		}
		targetItem = &target
		
		// Validate items are in same parent context
		if item.CollectionID.Compare(targetItem.CollectionID) != 0 {
			return fmt.Errorf("items must be in same collection")
		}
		
		// Check parent folder compatibility
		if (item.ParentFolderID == nil) != (targetItem.ParentFolderID == nil) {
			return fmt.Errorf("items must be in same parent folder context")
		}
		if item.ParentFolderID != nil && targetItem.ParentFolderID != nil {
			if item.ParentFolderID.Compare(*targetItem.ParentFolderID) != 0 {
				return fmt.Errorf("items must be in same parent folder")
			}
		}
	}

	// Get current ordered list
	orderedItems, err := s.queries.GetCollectionItemsInOrder(ctx, gen.GetCollectionItemsInOrderParams{
		CollectionID:   item.CollectionID,
		ParentFolderID: item.ParentFolderID,
		CollectionID_2: item.CollectionID,
	})
	if err != nil {
		return fmt.Errorf("failed to get ordered items: %w", err)
	}

	// Find current and target positions
	currentPos := -1
	targetPos := -1
	
	for i, orderedItem := range orderedItems {
		orderItemID := idwrap.NewFromBytesMust(orderedItem.ID)
		if orderItemID.Compare(itemID) == 0 {
			currentPos = i
		}
		if targetID != nil && orderItemID.Compare(*targetID) == 0 {
			targetPos = i
		}
	}
	
	if currentPos == -1 {
		return fmt.Errorf("item not found in ordered list")
	}
	if targetID != nil && targetPos == -1 {
		return fmt.Errorf("target item not found in ordered list")
	}

	// Calculate new position based on move position
	var newPos int
	if targetID == nil {
		// No target specified, use position as absolute
		if position == movable.MovePositionAfter {
			newPos = len(orderedItems) - 1 // Move to end
		} else {
			newPos = 0 // Move to beginning
		}
	} else {
		// Position relative to target
		// We need to account for the fact that we'll remove the item first
		if position == movable.MovePositionAfter {
			if currentPos < targetPos {
				// Moving forward: target position shifts left by 1 after removal
				newPos = targetPos
			} else {
				// Moving backward: target position stays the same, insert after
				newPos = targetPos + 1
			}
		} else { // MovePositionBefore
			if currentPos < targetPos {
				// Moving forward: target position shifts left by 1 after removal
				newPos = targetPos - 1
			} else {
				// Moving backward: target position stays the same
				newPos = targetPos
			}
		}
	}
	
	// Clamp to valid range
	if newPos < 0 {
		newPos = 0
	}
	if newPos >= len(orderedItems) {
		newPos = len(orderedItems) - 1
	}
	
	// If no change needed
	if currentPos == newPos {
		return nil
	}
	
	// Directly update the linked list by manipulating prev/next pointers
	err = s.moveItemToPosition(ctx, itemID, orderedItems, currentPos, newPos)
	if err != nil {
		return fmt.Errorf("failed to move item to position: %w", err)
	}

	s.logger.Debug("Successfully moved collection item",
		"item_id", itemID.String(),
		"from_pos", currentPos,
		"to_pos", newPos)
	return nil
}

// CreateFolderTX creates a folder using the reference-based architecture
// 1. Creates collection_items entry (type=0, name, prev/next positioning)
// 2. Creates item_folder entry with collection_item_id FK
// 3. Single transaction ensures consistency
func (s *CollectionItemService) CreateFolderTX(ctx context.Context, tx *sql.Tx, folder *mitemfolder.ItemFolder) error {
	s.logger.Debug("Creating folder with collection item reference",
		"folder_id", folder.ID.String(),
		"collection_id", folder.CollectionID.String(),
		"name", folder.Name)

	// Get service with transaction support
	txService := s.TX(tx)

	// Find position to insert (append to end by default)
	maxPosition, err := txService.repository.GetMaxPosition(ctx, folder.CollectionID, movable.CollectionListTypeItems)
	if err != nil {
		return fmt.Errorf("failed to get max position: %w", err)
	}
	
	insertPosition := 0
	if maxPosition >= 0 {
		insertPosition = maxPosition + 1
	}

	// Step 1: Create collection_items entry (PRIMARY) with correct linked list position
	collectionItemID := idwrap.New(ulid.Make())
	err = txService.repository.InsertNewItemAtPosition(ctx, tx, gen.InsertCollectionItemParams{
		ID:             collectionItemID,
		CollectionID:   folder.CollectionID,
		ParentFolderID: folder.ParentID,
		ItemType:       int8(CollectionItemTypeFolder),
		FolderID:       &folder.ID, // Reference to legacy folder table
		EndpointID:     nil,
		Name:           folder.Name,
		PrevID:         nil, // Will be calculated by InsertNewItemAtPosition
		NextID:         nil, // Will be calculated by InsertNewItemAtPosition
	}, insertPosition)
	if err != nil {
		return fmt.Errorf("failed to insert collection item at position: %w", err)
	}

	// Step 2: Create item_folder entry (REFERENCE) with collection_item_id FK
	// Update folder to include collection_item_id reference (this might require schema changes)
	err = txService.folderService.CreateItemFolder(ctx, folder)
	if err != nil {
		return fmt.Errorf("failed to create folder reference: %w", err)
	}

	s.logger.Debug("Successfully created folder with collection item reference",
		"folder_id", folder.ID.String(),
		"collection_item_id", collectionItemID.String())
	return nil
}

// CreateEndpointTX creates an endpoint using the reference-based architecture  
// 1. Creates collection_items entry (type=1, name, url, method, prev/next positioning)
// 2. Creates item_api entry with collection_item_id FK
// 3. Single transaction ensures consistency
func (s *CollectionItemService) CreateEndpointTX(ctx context.Context, tx *sql.Tx, endpoint *mitemapi.ItemApi) error {
	s.logger.Debug("Creating endpoint with collection item reference",
		"endpoint_id", endpoint.ID.String(),
		"collection_id", endpoint.CollectionID.String(),
		"name", endpoint.Name,
		"url", endpoint.Url,
		"method", endpoint.Method)

	// Get service with transaction support
	txService := s.TX(tx)

	// Find position to insert (append to end by default)
	maxPosition, err := txService.repository.GetMaxPosition(ctx, endpoint.CollectionID, movable.CollectionListTypeItems)
	if err != nil {
		return fmt.Errorf("failed to get max position: %w", err)
	}
	
	insertPosition := 0
	if maxPosition >= 0 {
		insertPosition = maxPosition + 1
	}

	// Step 1: Create collection_items entry (PRIMARY) with correct linked list position
	collectionItemID := idwrap.New(ulid.Make())
	err = txService.repository.InsertNewItemAtPosition(ctx, tx, gen.InsertCollectionItemParams{
		ID:             collectionItemID,
		CollectionID:   endpoint.CollectionID,
		ParentFolderID: endpoint.FolderID,
		ItemType:       int8(CollectionItemTypeEndpoint),
		FolderID:       nil,
		EndpointID:     &endpoint.ID, // Reference to legacy endpoint table
		Name:           endpoint.Name,
		PrevID:         nil, // Will be calculated by InsertNewItemAtPosition
		NextID:         nil, // Will be calculated by InsertNewItemAtPosition
	}, insertPosition)
	if err != nil {
		return fmt.Errorf("failed to insert collection item at position: %w", err)
	}

	// Step 2: Create item_api entry (REFERENCE) with collection_item_id FK
	// Update endpoint to include collection_item_id reference (this might require schema changes)
	err = txService.apiService.CreateItemApi(ctx, endpoint)
	if err != nil {
		return fmt.Errorf("failed to create endpoint reference: %w", err)
	}

	s.logger.Debug("Successfully created endpoint with collection item reference",
		"endpoint_id", endpoint.ID.String(),
		"collection_item_id", collectionItemID.String())
	return nil
}

// GetCollectionItem retrieves a collection item by ID
func (s *CollectionItemService) GetCollectionItem(ctx context.Context, id idwrap.IDWrap) (*CollectionItem, error) {
	item, err := s.queries.GetCollectionItem(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrCollectionItemNotFound
		}
		return nil, fmt.Errorf("failed to get collection item: %w", err)
	}

	collectionItem := &CollectionItem{
		ID:             item.ID,
		CollectionID:   item.CollectionID,
		ParentFolderID: item.ParentFolderID,
		Name:           item.Name,
		ItemType:       CollectionItemType(item.ItemType),
		PrevID:         item.PrevID,
		NextID:         item.NextID,
		FolderID:       item.FolderID,
		EndpointID:     item.EndpointID,
	}

	return collectionItem, nil
}

// DeleteCollectionItem removes a collection item and its linked list connections
func (s *CollectionItemService) DeleteCollectionItem(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap) error {
	s.logger.Debug("Deleting collection item", "item_id", itemID.String())

	// Get service with transaction support
	txService := s.TX(tx)

	// Remove from linked list position first
	err := txService.repository.RemoveFromPosition(ctx, tx, itemID)
	if err != nil {
		return fmt.Errorf("failed to remove from position: %w", err)
	}

	// Delete the collection item
	err = txService.queries.DeleteCollectionItem(ctx, itemID)
	if err != nil {
		return fmt.Errorf("failed to delete collection item: %w", err)
	}

	s.logger.Debug("Successfully deleted collection item", "item_id", itemID.String())
	return nil
}

// Utility functions for type conversion

func convertBytesToIDWrap(bytes []byte) *idwrap.IDWrap {
	if len(bytes) == 0 {
		return nil
	}
	id := idwrap.NewFromBytesMust(bytes)
	return &id
}

func convertByteToString(bytes []byte) *string {
	if len(bytes) == 0 {
		return nil
	}
	str := string(bytes)
	return &str
}

func getIDString(id *idwrap.IDWrap) string {
	if id == nil {
		return "nil"
	}
	return id.String()
}

// GetWorkspaceID retrieves the workspace ID for a collection item
func (s *CollectionItemService) GetWorkspaceID(ctx context.Context, itemID idwrap.IDWrap) (idwrap.IDWrap, error) {
	item, err := s.queries.GetCollectionItem(ctx, itemID)
	if err != nil {
		if err == sql.ErrNoRows {
			return idwrap.IDWrap{}, ErrCollectionItemNotFound
		}
		return idwrap.IDWrap{}, fmt.Errorf("failed to get collection item: %w", err)
	}

	// Get workspace ID through collection
	collection, err := s.queries.GetCollection(ctx, item.CollectionID)
	if err != nil {
		return idwrap.IDWrap{}, fmt.Errorf("failed to get collection: %w", err)
	}

	return collection.WorkspaceID, nil
}

// CheckWorkspaceID verifies if a collection item belongs to a specific workspace
func (s *CollectionItemService) CheckWorkspaceID(ctx context.Context, itemID, workspaceID idwrap.IDWrap) (bool, error) {
	itemWorkspaceID, err := s.GetWorkspaceID(ctx, itemID)
	if err != nil {
		return false, err
	}
	return itemWorkspaceID.Compare(workspaceID) == 0, nil
}

// moveItemToPosition performs a simple move by rebuilding the entire linked list
// This approach is less efficient but much more reliable than complex pointer manipulation
func (s *CollectionItemService) moveItemToPosition(ctx context.Context, itemID idwrap.IDWrap, orderedItems []gen.GetCollectionItemsInOrderRow, fromPos, toPos int) error {
	if fromPos == toPos {
		return nil
	}

	// Create a new order by moving the item from fromPos to toPos
	newOrder := make([]idwrap.IDWrap, len(orderedItems))
	movingItem := idwrap.NewFromBytesMust(orderedItems[fromPos].ID)
	
	// Build new order: copy items skipping the moving item, insert moving item at toPos
	newIdx := 0
	movingItemInserted := false
	
	for i, item := range orderedItems {
		if i == fromPos {
			continue // Skip the moving item
		}
		
		// Check if we should insert the moving item before this position
		if newIdx == toPos && !movingItemInserted {
			newOrder[newIdx] = movingItem
			newIdx++
			movingItemInserted = true
		}
		
		newOrder[newIdx] = idwrap.NewFromBytesMust(item.ID)
		newIdx++
	}
	
	// If we haven't inserted the moving item yet, it goes at the end
	if !movingItemInserted {
		newOrder[len(newOrder)-1] = movingItem
	}

	// Rebuild the linked list with the new order
	return s.rebuildLinkedList(ctx, newOrder)
}

// rebuildLinkedList rebuilds the linked list structure for the given ordered items
func (s *CollectionItemService) rebuildLinkedList(ctx context.Context, orderedIDs []idwrap.IDWrap) error {
	for i, itemID := range orderedIDs {
		var prevID, nextID *idwrap.IDWrap
		
		if i > 0 {
			prevID = &orderedIDs[i-1]
		}
		if i < len(orderedIDs)-1 {
			nextID = &orderedIDs[i+1]
		}
		
		err := s.queries.UpdateCollectionItemOrder(ctx, gen.UpdateCollectionItemOrderParams{
			PrevID: prevID,
			NextID: nextID,
			ID:     itemID,
		})
		if err != nil {
			return fmt.Errorf("failed to update item %s: %w", itemID.String(), err)
		}
	}
	
	return nil
}