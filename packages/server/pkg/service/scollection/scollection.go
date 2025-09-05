package scollection

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/movable"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

var ErrNoCollectionFound = sql.ErrNoRows

type CollectionService struct {
	queries           *gen.Queries
	logger            *slog.Logger
	linkedListManager movable.LinkedListManager
	movableRepository movable.MovableRepository
}

func ConvertToDBCollection(collection mcollection.Collection) gen.Collection {
	return gen.Collection{
		ID:          collection.ID,
		WorkspaceID: collection.WorkspaceID,
		Name:        collection.Name,
	}
}

func ConvertToModelCollection(collection gen.Collection) *mcollection.Collection {
	return &mcollection.Collection{
		ID:          collection.ID,
		WorkspaceID: collection.WorkspaceID,
		Name:        collection.Name,
	}
}

func ConvertGetCollectionByWorkspaceIDRowToModel(row gen.GetCollectionByWorkspaceIDRow) *mcollection.Collection {
	return &mcollection.Collection{
		ID:          row.ID,
		WorkspaceID: row.WorkspaceID,
		Name:        row.Name,
	}
}

func New(queries *gen.Queries, logger *slog.Logger) CollectionService {
	// Create the movable repository for collections
	movableRepo := NewCollectionMovableRepository(queries)

	// Create the linked list manager with the movable repository
	linkedListManager := movable.NewDefaultLinkedListManager(movableRepo)

	return CollectionService{
		queries:           queries,
		logger:            logger,
		linkedListManager: linkedListManager,
		movableRepository: movableRepo,
	}
}

func (cs CollectionService) TX(tx *sql.Tx) CollectionService {
	// Create new instances with transaction support
	txQueries := cs.queries.WithTx(tx)
	movableRepo := NewCollectionMovableRepository(txQueries)
	linkedListManager := movable.NewDefaultLinkedListManager(movableRepo)

	return CollectionService{
		queries:           txQueries,
		logger:            cs.logger,
		linkedListManager: linkedListManager,
		movableRepository: movableRepo,
	}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*CollectionService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}

	// Create movable repository and linked list manager
	movableRepo := NewCollectionMovableRepository(queries)
	linkedListManager := movable.NewDefaultLinkedListManager(movableRepo)

	service := CollectionService{
		queries:           queries,
		linkedListManager: linkedListManager,
		movableRepository: movableRepo,
	}
	return &service, nil
}

func (cs CollectionService) ListCollections(ctx context.Context, workspaceID idwrap.IDWrap) ([]mcollection.Collection, error) {
	rows, err := cs.queries.GetCollectionByWorkspaceID(ctx, workspaceID)
	if err != nil {
		if err == sql.ErrNoRows {
			cs.logger.InfoContext(ctx, fmt.Sprintf("workspaceID: %s not found", workspaceID.String()))
			return nil, ErrNoCollectionFound
		}
		return nil, err
	}
	return tgeneric.MassConvertPtr(rows, ConvertGetCollectionByWorkspaceIDRowToModel), nil
}

func (cs CollectionService) CreateCollection(ctx context.Context, collection *mcollection.Collection) error {
	// Super-safe append-to-end using planner + guarded updates.
	// Step 1: Plan append using MovableRepository (preflight integrity + tail detection).
	plan, err := movable.BuildAppendPlanFromRepo(ctx, cs.movableRepository, collection.WorkspaceID, movable.CollectionListTypeCollections, collection.ID)
	if err != nil {
		return fmt.Errorf("plan append failed: %w", err)
	}

	// Step 2: If there is a tail (PrevID), patch tail's next to the new ID.
	if plan.PrevID != nil {
		tailRow, err := cs.queries.GetCollection(ctx, *plan.PrevID)
		if err != nil {
			return fmt.Errorf("failed to read tail collection: %w", err)
		}
		if tailRow.Next != nil {
			return fmt.Errorf("concurrent tail advance detected for tail %s", plan.PrevID.String())
		}
		// Update only the next pointer while preserving existing prev.
		if err := cs.queries.UpdateCollectionOrder(ctx, gen.UpdateCollectionOrderParams{
			Prev:        tailRow.Prev,
			Next:        &collection.ID,
			ID:          *plan.PrevID,
			WorkspaceID: collection.WorkspaceID,
		}); err != nil {
			return fmt.Errorf("failed to set tail.next: %w", err)
		}
	}

	// Step 3: Insert the new collection as the new tail (prev=plan.PrevID, next=NULL).
	return cs.queries.CreateCollection(ctx, gen.CreateCollectionParams{
		ID:          collection.ID,
		WorkspaceID: collection.WorkspaceID,
		Name:        collection.Name,
		Prev:        plan.PrevID,
		Next:        nil,
	})
}

func (cs CollectionService) GetCollection(ctx context.Context, id idwrap.IDWrap) (*mcollection.Collection, error) {
	collection, err := cs.queries.GetCollection(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			cs.logger.DebugContext(ctx, fmt.Sprintf("CollectionID: %s not found", id.String()))
			return nil, ErrNoCollectionFound
		}
		return nil, err
	}
	return ConvertToModelCollection(collection), nil
}

func (cs CollectionService) UpdateCollection(ctx context.Context, collection *mcollection.Collection) error {
	err := cs.queries.UpdateCollection(ctx, gen.UpdateCollectionParams{
		ID:          collection.ID,
		WorkspaceID: collection.WorkspaceID,
		Name:        collection.Name,
	})
	return err
}

func (cs CollectionService) DeleteCollection(ctx context.Context, id idwrap.IDWrap) error {
	return cs.queries.DeleteCollection(ctx, id)
}

func (cs CollectionService) GetWorkspaceID(ctx context.Context, id idwrap.IDWrap) (idwrap.IDWrap, error) {
	ulidData, err := cs.queries.GetCollectionWorkspaceID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return idwrap.IDWrap{}, ErrNoCollectionFound
		}
		return idwrap.IDWrap{}, err
	}
	return ulidData, nil
}

func (cs CollectionService) CheckWorkspaceID(ctx context.Context, id, ownerID idwrap.IDWrap) (bool, error) {
	CollectionWorkspaceID, err := cs.GetWorkspaceID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, ErrNoCollectionFound
		}
		return false, err
	}
	return ownerID.Compare(CollectionWorkspaceID) == 0, nil
}

func (cs CollectionService) GetCollectionByWorkspaceIDAndName(ctx context.Context, workspaceID idwrap.IDWrap, name string) (*mcollection.Collection, error) {
	collection, err := cs.queries.GetCollectionByWorkspaceIDAndName(ctx, gen.GetCollectionByWorkspaceIDAndNameParams{
		WorkspaceID: workspaceID,
		Name:        name,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoCollectionFound
		}
		return nil, err
	}
	return &mcollection.Collection{
		ID:          collection.ID,
		Name:        collection.Name,
		WorkspaceID: collection.WorkspaceID,
	}, nil
}

// Movable operations for collections

// MoveCollectionAfter moves a collection to be positioned after the target collection
func (cs CollectionService) MoveCollectionAfter(ctx context.Context, collectionID, targetID idwrap.IDWrap) error {
	return cs.MoveCollectionAfterTX(ctx, nil, collectionID, targetID)
}

// MoveCollectionAfterTX moves a collection to be positioned after the target collection within a transaction
func (cs CollectionService) MoveCollectionAfterTX(ctx context.Context, tx *sql.Tx, collectionID, targetID idwrap.IDWrap) error {
	service := cs
	if tx != nil {
		service = cs.TX(tx)
	}

	// Get workspace ID for both collections to ensure they're in the same workspace
	sourceWorkspaceID, err := service.GetWorkspaceID(ctx, collectionID)
	if err != nil {
		return fmt.Errorf("failed to get source collection workspace: %w", err)
	}

	targetWorkspaceID, err := service.GetWorkspaceID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("failed to get target collection workspace: %w", err)
	}

	if sourceWorkspaceID.Compare(targetWorkspaceID) != 0 {
		return fmt.Errorf("collections must be in the same workspace")
	}

	// Get all collections in the workspace in order
	collections, err := service.GetCollectionsOrdered(ctx, sourceWorkspaceID)
	if err != nil {
		return fmt.Errorf("failed to get collections in order: %w", err)
	}

	// Find positions of source and target collections
	var sourcePos, targetPos int = -1, -1
	for i, col := range collections {
		if col.ID.Compare(collectionID) == 0 {
			sourcePos = i
		}
		if col.ID.Compare(targetID) == 0 {
			targetPos = i
		}
	}

	if sourcePos == -1 {
		return fmt.Errorf("source collection not found in workspace")
	}
	if targetPos == -1 {
		return fmt.Errorf("target collection not found in workspace")
	}

	if sourcePos == targetPos {
		return fmt.Errorf("cannot move collection relative to itself")
	}

	// Calculate new order: move source to be after target
	newOrder := make([]idwrap.IDWrap, 0, len(collections))

	for i, col := range collections {
		if i == sourcePos {
			continue // Skip source collection
		}
		newOrder = append(newOrder, col.ID)
		if i == targetPos {
			newOrder = append(newOrder, collectionID) // Insert source after target
		}
	}

	// Reorder collections
	return service.ReorderCollectionsTX(ctx, tx, sourceWorkspaceID, newOrder)
}

// MoveCollectionBefore moves a collection to be positioned before the target collection
func (cs CollectionService) MoveCollectionBefore(ctx context.Context, collectionID, targetID idwrap.IDWrap) error {
	return cs.MoveCollectionBeforeTX(ctx, nil, collectionID, targetID)
}

// MoveCollectionBeforeTX moves a collection to be positioned before the target collection within a transaction
func (cs CollectionService) MoveCollectionBeforeTX(ctx context.Context, tx *sql.Tx, collectionID, targetID idwrap.IDWrap) error {
	service := cs
	if tx != nil {
		service = cs.TX(tx)
	}

	// Get workspace ID for both collections to ensure they're in the same workspace
	sourceWorkspaceID, err := service.GetWorkspaceID(ctx, collectionID)
	if err != nil {
		return fmt.Errorf("failed to get source collection workspace: %w", err)
	}

	targetWorkspaceID, err := service.GetWorkspaceID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("failed to get target collection workspace: %w", err)
	}

	if sourceWorkspaceID.Compare(targetWorkspaceID) != 0 {
		return fmt.Errorf("collections must be in the same workspace")
	}

	// Get all collections in the workspace in order
	collections, err := service.GetCollectionsOrdered(ctx, sourceWorkspaceID)
	if err != nil {
		return fmt.Errorf("failed to get collections in order: %w", err)
	}

	// Find positions of source and target collections
	var sourcePos, targetPos int = -1, -1
	for i, col := range collections {
		if col.ID.Compare(collectionID) == 0 {
			sourcePos = i
		}
		if col.ID.Compare(targetID) == 0 {
			targetPos = i
		}
	}

	if sourcePos == -1 {
		return fmt.Errorf("source collection not found in workspace")
	}
	if targetPos == -1 {
		return fmt.Errorf("target collection not found in workspace")
	}

	if sourcePos == targetPos {
		return fmt.Errorf("cannot move collection relative to itself")
	}

	// Calculate new order: move source to be before target
	newOrder := make([]idwrap.IDWrap, 0, len(collections))

	for i, col := range collections {
		if i == sourcePos {
			continue // Skip source collection
		}
		if i == targetPos {
			newOrder = append(newOrder, collectionID) // Insert source before target
		}
		newOrder = append(newOrder, col.ID)
	}

	// Reorder collections
	return service.ReorderCollectionsTX(ctx, tx, sourceWorkspaceID, newOrder)
}

// GetCollectionsOrdered returns collections in the workspace in their proper order
func (cs CollectionService) GetCollectionsOrdered(ctx context.Context, workspaceID idwrap.IDWrap) ([]mcollection.Collection, error) {
	// Use the underlying query that maintains the linked list order
	orderedCollections, err := cs.queries.GetCollectionsInOrder(ctx, gen.GetCollectionsInOrderParams{
		WorkspaceID:   workspaceID,
		WorkspaceID_2: workspaceID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			cs.logger.InfoContext(ctx, fmt.Sprintf("workspaceID: %s has no collections", workspaceID.String()))
			return []mcollection.Collection{}, nil
		}
		return nil, err
	}

	// Convert to model collections
	collections := make([]mcollection.Collection, len(orderedCollections))
	for i, col := range orderedCollections {
		collections[i] = mcollection.Collection{
			ID:          idwrap.NewFromBytesMust(col.ID),
			WorkspaceID: idwrap.NewFromBytesMust(col.WorkspaceID),
			Name:        col.Name,
			// Note: Updated field not available in the ordered query result
			// If needed, we could make a separate query to get this field
		}
	}

	return collections, nil
}

// ReorderCollections performs a bulk reorder of collections using the movable system
func (cs CollectionService) ReorderCollections(ctx context.Context, workspaceID idwrap.IDWrap, orderedIDs []idwrap.IDWrap) error {
	return cs.ReorderCollectionsTX(ctx, nil, workspaceID, orderedIDs)
}

// ReorderCollectionsTX performs a bulk reorder of collections using the movable system within a transaction
func (cs CollectionService) ReorderCollectionsTX(ctx context.Context, tx *sql.Tx, workspaceID idwrap.IDWrap, orderedIDs []idwrap.IDWrap) error {
	service := cs
	if tx != nil {
		service = cs.TX(tx)
	}

	// Build position updates
	updates := make([]movable.PositionUpdate, len(orderedIDs))
	for i, id := range orderedIDs {
		updates[i] = movable.PositionUpdate{
			ItemID:   id,
			ListType: movable.CollectionListTypeCollections,
			Position: i,
		}
	}

	// Execute the batch update using the movable repository
	if err := service.movableRepository.UpdatePositions(ctx, tx, updates); err != nil {
		return fmt.Errorf("failed to reorder collections: %w", err)
	}

	cs.logger.DebugContext(ctx, "Collections reordered",
		"workspaceID", workspaceID.String(),
		"collectionCount", len(orderedIDs))

	return nil
}

// CompactCollectionPositions recalculates and compacts position values to eliminate gaps
func (cs CollectionService) CompactCollectionPositions(ctx context.Context, workspaceID idwrap.IDWrap) error {
	return cs.CompactCollectionPositionsTX(ctx, nil, workspaceID)
}

// CompactCollectionPositionsTX recalculates and compacts position values within a transaction
func (cs CollectionService) CompactCollectionPositionsTX(ctx context.Context, tx *sql.Tx, workspaceID idwrap.IDWrap) error {
	service := cs
	if tx != nil {
		service = cs.TX(tx)
	}

	if err := service.linkedListManager.CompactPositions(ctx, tx, workspaceID, movable.CollectionListTypeCollections); err != nil {
		return fmt.Errorf("failed to compact collection positions: %w", err)
	}

	cs.logger.DebugContext(ctx, "Collection positions compacted", "workspaceID", workspaceID.String())
	return nil
}
