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

// MoveCollectionItem moves a collection item to a new position, supporting cross-folder moves
// Updates prev/next pointers and parent_folder_id in the collection_items table
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

	// Validate target exists if specified and determine target parent context
	var targetItem *gen.CollectionItem
	var targetParentFolderID *idwrap.IDWrap
	if targetID != nil {
		target, err := s.queries.GetCollectionItem(ctx, *targetID)
		if err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("target item not found")
			}
			return fmt.Errorf("failed to get target item: %w", err)
		}
		targetItem = &target
		
		// Validate items are in same collection (still required)
		if item.CollectionID.Compare(targetItem.CollectionID) != 0 {
			return fmt.Errorf("items must be in same collection")
		}
		
		// Determine target parent context based on move semantics:
		// We need to distinguish between two cases when target is a folder:
		// 1. "Drop into folder" - move item to be inside the target folder
		// 2. "Position relative to folder" - position item before/after folder at the same level
		
		
		if targetItem.ItemType == int8(CollectionItemTypeFolder) {
			targetFolderID := idwrap.NewFromBytesMust(targetItem.ID.Bytes())
			
			// Check if item is already inside the target folder
			itemAlreadyInTargetFolder := item.ParentFolderID != nil && 
				item.ParentFolderID.Compare(targetFolderID) == 0
			
			
			if itemAlreadyInTargetFolder {
				// Item is already in the target folder and we're targeting that folder.
				// This means "move this item to be positioned relative to the folder itself"
				// i.e., move OUT of the folder to the same level as the folder.
				targetParentFolderID = targetItem.ParentFolderID
				s.logger.Debug("Item in target folder, moving out to folder's level")
			} else {
				// Item is NOT currently in the target folder
				// 
				// Key insight: When targeting a folder with BEFORE/AFTER position,
				// the item should be positioned at the SAME LEVEL as the target folder,
				// regardless of where the item currently is.
				// This provides consistent behavior for positioning operations.
				
				targetParentFolderID = targetItem.ParentFolderID
				s.logger.Debug("Position relative to target folder level")
			}
		} else {
			// Target is not a folder: use target's parent context (normal positioning)
			targetParentFolderID = targetItem.ParentFolderID
		}
	} else {
		// No target specified - use current parent context
		targetParentFolderID = item.ParentFolderID
	}

	// Check if this is a cross-folder move (parent folder change)
	isCrossFolderMove := false
	if (item.ParentFolderID == nil) != (targetParentFolderID == nil) {
		isCrossFolderMove = true
	} else if item.ParentFolderID != nil && targetParentFolderID != nil {
		if item.ParentFolderID.Compare(*targetParentFolderID) != 0 {
			isCrossFolderMove = true
		}
	}

	s.logger.Debug("Move operation analysis",
		"item_id", itemID.String(),
		"target_id", getIDString(targetID),
		"item_parent", getIDString(item.ParentFolderID),
		"target_parent", getIDString(targetParentFolderID),
		"target_item_type", func() string {
			if targetItem != nil {
				return fmt.Sprintf("type_%d", targetItem.ItemType)
			}
			return "nil"
		}(),
		"is_cross_folder_move", isCrossFolderMove)


	if isCrossFolderMove {
		// Cross-folder move: need to update parent_folder_id and handle two different lists
		return s.performCrossFolderMove(ctx, itemID, item, targetID, targetParentFolderID, position)
	} else {
		// Same-folder move: use existing logic
		return s.performSameFolderMove(ctx, itemID, item, targetID, position)
	}
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

	// Step 1: Create item_folder entry (LEGACY TABLE) first to satisfy foreign key constraints
	err = txService.folderService.CreateItemFolder(ctx, folder)
	if err != nil {
		return fmt.Errorf("failed to create folder reference: %w", err)
	}

	// Step 2: Create collection_items entry (PRIMARY) with correct linked list position
	// Now we can safely reference folder.ID since it exists in item_folder table
	collectionItemID := idwrap.New(ulid.Make())
	err = txService.repository.InsertNewItemAtPosition(ctx, tx, gen.InsertCollectionItemParams{
		ID:             collectionItemID,
		CollectionID:   folder.CollectionID,
		ParentFolderID: folder.ParentID,
		ItemType:       int8(CollectionItemTypeFolder),
		FolderID:       &folder.ID, // Reference to legacy folder table (now exists)
		EndpointID:     nil,
		Name:           folder.Name,
		PrevID:         nil, // Will be calculated by InsertNewItemAtPosition
		NextID:         nil, // Will be calculated by InsertNewItemAtPosition
	}, insertPosition)
	if err != nil {
		return fmt.Errorf("failed to insert collection item at position: %w", err)
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

	// Step 1: Create item_api entry (LEGACY TABLE) first to satisfy foreign key constraints
	err = txService.apiService.CreateItemApi(ctx, endpoint)
	if err != nil {
		return fmt.Errorf("failed to create endpoint reference: %w", err)
	}

	// Step 2: Create collection_items entry (PRIMARY) with correct linked list position
	// Now we can safely reference endpoint.ID since it exists in item_api table
	collectionItemID := idwrap.New(ulid.Make())
	err = txService.repository.InsertNewItemAtPosition(ctx, tx, gen.InsertCollectionItemParams{
		ID:             collectionItemID,
		CollectionID:   endpoint.CollectionID,
		ParentFolderID: endpoint.FolderID,
		ItemType:       int8(CollectionItemTypeEndpoint),
		FolderID:       nil,
		EndpointID:     &endpoint.ID, // Reference to legacy endpoint table (now exists)
		Name:           endpoint.Name,
		PrevID:         nil, // Will be calculated by InsertNewItemAtPosition
		NextID:         nil, // Will be calculated by InsertNewItemAtPosition
	}, insertPosition)
	if err != nil {
		return fmt.Errorf("failed to insert collection item at position: %w", err)
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

// GetCollectionItemIDByLegacyID converts a legacy table ID (folder_id or endpoint_id) to collection_items table ID
// This is needed for backward compatibility with move operations that receive legacy IDs
func (s *CollectionItemService) GetCollectionItemIDByLegacyID(ctx context.Context, legacyID idwrap.IDWrap) (idwrap.IDWrap, error) {
	s.logger.Debug("Converting legacy ID to collection_items ID", "legacy_id", legacyID.String())
	
	// Try to find by folder_id first
	folderItem, err := s.queries.GetCollectionItemByFolderID(ctx, &legacyID)
	if err == nil {
		s.logger.Debug("Found collection item by folder_id", 
			"legacy_id", legacyID.String(),
			"collection_item_id", folderItem.ID.String())
		return folderItem.ID, nil
	}
	
	// If not found by folder_id, try endpoint_id
	if err == sql.ErrNoRows {
		endpointItem, err := s.queries.GetCollectionItemByEndpointID(ctx, &legacyID)
		if err == nil {
			s.logger.Debug("Found collection item by endpoint_id", 
				"legacy_id", legacyID.String(),
				"collection_item_id", endpointItem.ID.String())
			return endpointItem.ID, nil
		}
		
		if err == sql.ErrNoRows {
			s.logger.Debug("Legacy ID not found in collection_items table", "legacy_id", legacyID.String())
			return idwrap.IDWrap{}, ErrCollectionItemNotFound
		}
		
		return idwrap.IDWrap{}, fmt.Errorf("failed to get collection item by endpoint_id: %w", err)
	}
	
	return idwrap.IDWrap{}, fmt.Errorf("failed to get collection item by folder_id: %w", err)
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

// performCrossFolderMove handles moving an item from one parent folder context to another
func (s *CollectionItemService) performCrossFolderMove(ctx context.Context, itemID idwrap.IDWrap, item gen.CollectionItem, targetID *idwrap.IDWrap, newParentFolderID *idwrap.IDWrap, position movable.MovePosition) error {
	s.logger.Debug("Performing cross-folder move",
		"item_id", itemID.String(),
		"from_parent", getIDString(item.ParentFolderID),
		"to_parent", getIDString(newParentFolderID),
		"target_id", getIDString(targetID))

	// Step 1: Remove item from current parent's linked list
	currentParentItems, err := s.queries.GetCollectionItemsInOrder(ctx, gen.GetCollectionItemsInOrderParams{
		CollectionID:   item.CollectionID,
		ParentFolderID: item.ParentFolderID,
		CollectionID_2: item.CollectionID,
	})
	if err != nil {
		return fmt.Errorf("failed to get current parent items: %w", err)
	}

	// Find current position and remove from list
	currentPos := -1
	for i, orderedItem := range currentParentItems {
		orderItemID := idwrap.NewFromBytesMust(orderedItem.ID)
		if orderItemID.Compare(itemID) == 0 {
			currentPos = i
			break
		}
	}
	
	if currentPos == -1 {
		return fmt.Errorf("item not found in current parent list")
	}

	// Rebuild current parent list without the moving item
	newCurrentParentOrder := make([]idwrap.IDWrap, 0, len(currentParentItems)-1)
	for i, orderedItem := range currentParentItems {
		if i != currentPos {
			newCurrentParentOrder = append(newCurrentParentOrder, idwrap.NewFromBytesMust(orderedItem.ID))
		}
	}

	// Step 2: Get target parent's current items BEFORE changing parent_folder_id
	// This prevents the moving item from appearing in the target parent list
	targetParentItems, err := s.queries.GetCollectionItemsInOrder(ctx, gen.GetCollectionItemsInOrderParams{
		CollectionID:   item.CollectionID,
		ParentFolderID: newParentFolderID,
		CollectionID_2: item.CollectionID,
	})
	if err != nil {
		return fmt.Errorf("failed to get target parent items: %w", err)
	}

	// Step 3: Update item's parent_folder_id
	s.logger.Debug("Updating parent_folder_id", 
		"item_id", itemID.String(), 
		"new_parent", getIDString(newParentFolderID))
	err = s.queries.UpdateCollectionItemParentFolder(ctx, gen.UpdateCollectionItemParentFolderParams{
		ParentFolderID: newParentFolderID,
		ID:             itemID,
	})
	if err != nil {
		return fmt.Errorf("failed to update item parent folder: %w", err)
	}

	// Step 4: Calculate insertion position
	var insertPos int
	if targetID != nil {
		// Look for the target item within the new parent context
		targetPos := -1
		for i, orderedItem := range targetParentItems {
			orderItemID := idwrap.NewFromBytesMust(orderedItem.ID)
			if orderItemID.Compare(*targetID) == 0 {
				targetPos = i
				break
			}
		}
		
		if targetPos != -1 {
			// Target found within new parent context: normal positioning relative to target
			if position == movable.MovePositionAfter {
				insertPos = targetPos + 1
			} else {
				insertPos = targetPos
			}
			s.logger.Debug("Target found in new parent context, positioning relative to it", 
				"target_pos", targetPos, "insert_pos", insertPos)
		} else {
			// Target not found in new parent context: add to end
			// This can happen when the target is a reference point from a different context
			insertPos = len(targetParentItems)
			s.logger.Debug("Target not in new parent context, adding to end", "pos", insertPos)
		}
	} else {
		// No target specified, add to end
		insertPos = len(targetParentItems)
	}

	// Clamp insert position
	if insertPos < 0 {
		insertPos = 0
	}
	if insertPos > len(targetParentItems) {
		insertPos = len(targetParentItems)
	}

	// Step 5: Insert item into target parent's list at calculated position
	newTargetParentOrder := make([]idwrap.IDWrap, 0, len(targetParentItems)+1)
	for i, orderedItem := range targetParentItems {
		if i == insertPos {
			newTargetParentOrder = append(newTargetParentOrder, itemID)
		}
		newTargetParentOrder = append(newTargetParentOrder, idwrap.NewFromBytesMust(orderedItem.ID))
	}
	// Handle case where item is inserted at the end
	if insertPos == len(targetParentItems) {
		newTargetParentOrder = append(newTargetParentOrder, itemID)
	}

	// Step 6: Rebuild both linked lists
	if len(newCurrentParentOrder) > 0 {
		s.logger.Debug("Rebuilding current parent linked list", 
			"items_count", len(newCurrentParentOrder),
			"items", func() []string {
				strs := make([]string, len(newCurrentParentOrder))
				for i, id := range newCurrentParentOrder {
					strs[i] = id.String()
				}
				return strs
			}())
		err = s.rebuildLinkedList(ctx, newCurrentParentOrder)
		if err != nil {
			return fmt.Errorf("failed to rebuild current parent linked list: %w", err)
		}
	}

	s.logger.Debug("Rebuilding target parent linked list", 
		"items_count", len(newTargetParentOrder),
		"items", func() []string {
			strs := make([]string, len(newTargetParentOrder))
			for i, id := range newTargetParentOrder {
				strs[i] = id.String()
			}
			return strs
		}())
	err = s.rebuildLinkedList(ctx, newTargetParentOrder)
	if err != nil {
		return fmt.Errorf("failed to rebuild target parent linked list: %w", err)
	}

	s.logger.Debug("Successfully completed cross-folder move",
		"item_id", itemID.String(),
		"insert_pos", insertPos,
		"target_parent_items", len(newTargetParentOrder),
		"new_parent_folder_id", getIDString(newParentFolderID))
	
	
	return nil
}

// performSameFolderMove handles moving an item within the same parent folder context
func (s *CollectionItemService) performSameFolderMove(ctx context.Context, itemID idwrap.IDWrap, item gen.CollectionItem, targetID *idwrap.IDWrap, position movable.MovePosition) error {
	s.logger.Debug("Performing same-folder move",
		"item_id", itemID.String(),
		"parent", getIDString(item.ParentFolderID))

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
		s.logger.Debug("No position change needed", "current_pos", currentPos, "new_pos", newPos)
		return nil
	}
	
	// Use existing move logic
	err = s.moveItemToPosition(ctx, itemID, orderedItems, currentPos, newPos)
	if err != nil {
		return fmt.Errorf("failed to move item to position: %w", err)
	}

	s.logger.Debug("Successfully moved collection item within same folder",
		"item_id", itemID.String(),
		"from_pos", currentPos,
		"to_pos", newPos)
	return nil
}