package sitemapiexample

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mexamplebreadcrumb"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mitemfolder"
	"the-dev-tools/server/pkg/movable"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"time"
)

type ItemApiExampleService struct {
	Queries           *gen.Queries
	movableRepository *ExampleMovableRepository
}

var ErrNoItemApiExampleFound = errors.New("no example found")

func New(queries *gen.Queries) ItemApiExampleService {
	return ItemApiExampleService{
		Queries:           queries,
		movableRepository: NewExampleMovableRepository(queries),
	}
}

func (ias ItemApiExampleService) TX(tx *sql.Tx) ItemApiExampleService {
	return ItemApiExampleService{
		Queries:           ias.Queries.WithTx(tx),
		movableRepository: ias.movableRepository.TX(tx),
	}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*ItemApiExampleService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &ItemApiExampleService{
		Queries:           queries,
		movableRepository: NewExampleMovableRepository(queries),
	}, nil
}

func MassConvert[T any, O any](item []T, convFunc func(T) *O) []O {
	arr := make([]O, len(item))
	for i, v := range item {
		arr[i] = *convFunc(v)
	}
	return arr
}

func ConvertToDBItem(item mitemapiexample.ItemApiExample) gen.ItemApiExample {
	// TODO: add headers and query
	return gen.ItemApiExample{
		ID:           item.ID,
		ItemApiID:    item.ItemApiID,
		CollectionID: item.CollectionID,
		IsDefault:    item.IsDefault,
		BodyType:     int8(item.BodyType),
		Name:         item.Name,

		VersionParentID: item.VersionParentID,
		Prev:            item.Prev,
		Next:            item.Next,
	}
}

func ConvertToModelItem(item gen.ItemApiExample) *mitemapiexample.ItemApiExample {
	return &mitemapiexample.ItemApiExample{
		ID:           item.ID,
		ItemApiID:    item.ItemApiID,
		CollectionID: item.CollectionID,
		IsDefault:    item.IsDefault,
		BodyType:     mitemapiexample.BodyType(item.BodyType),
		Name:         item.Name,

		VersionParentID: item.VersionParentID,
		Prev:            item.Prev,
		Next:            item.Next,
	}
}

func ConvertOrderedRowToModelItem(row gen.GetExamplesByEndpointIDOrderedRow) *mitemapiexample.ItemApiExample {
	var versionParentID *idwrap.IDWrap
	if row.VersionParentID != nil {
		id := idwrap.NewFromBytesMust(row.VersionParentID)
		versionParentID = &id
	}

	var prev *idwrap.IDWrap
	if row.Prev != nil {
		id := idwrap.NewFromBytesMust(row.Prev)
		prev = &id
	}

	var next *idwrap.IDWrap
	if row.Next != nil {
		id := idwrap.NewFromBytesMust(row.Next)
		next = &id
	}

	return &mitemapiexample.ItemApiExample{
		ID:           idwrap.NewFromBytesMust(row.ID),
		ItemApiID:    idwrap.NewFromBytesMust(row.ItemApiID),
		CollectionID: idwrap.NewFromBytesMust(row.CollectionID),
		IsDefault:    row.IsDefault,
		BodyType:     mitemapiexample.BodyType(row.BodyType),
		Name:         row.Name,
		Updated:      time.Now(), // Note: GetExamplesByEndpointIDOrderedRow doesn't include Updated field

		VersionParentID: versionParentID,
		Prev:            prev,
		Next:            next,
	}
}

func (iaes ItemApiExampleService) GetApiExamples(ctx context.Context, apiUlid idwrap.IDWrap) ([]mitemapiexample.ItemApiExample, error) {
	itemApiExamples, err := iaes.Queries.GetItemApiExamples(ctx, apiUlid)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mitemapiexample.ItemApiExample{}, ErrNoItemApiExampleFound
		}
		return nil, err
	}
	return MassConvert(itemApiExamples, ConvertToModelItem), nil
}

func (iaes ItemApiExampleService) GetApiExamplesOrdered(ctx context.Context, apiUlid idwrap.IDWrap) ([]mitemapiexample.ItemApiExample, error) {
	itemApiExamples, err := iaes.Queries.GetExamplesByEndpointIDOrdered(ctx, gen.GetExamplesByEndpointIDOrderedParams{
		ItemApiID:   apiUlid,
		ItemApiID_2: apiUlid,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return []mitemapiexample.ItemApiExample{}, ErrNoItemApiExampleFound
		}
		return nil, err
	}
	return MassConvert(itemApiExamples, ConvertOrderedRowToModelItem), nil
}

// GetAllApiExamples returns ALL examples including isolated ones using the fallback query
func (iaes ItemApiExampleService) GetAllApiExamples(ctx context.Context, endpointID idwrap.IDWrap) ([]mitemapiexample.ItemApiExample, error) {
	examples, err := iaes.Queries.GetAllExamplesByEndpointID(ctx, endpointID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mitemapiexample.ItemApiExample{}, ErrNoItemApiExampleFound
		}
		return nil, err
	}
	return MassConvert(examples, ConvertToModelItem), nil
}

// AutoLinkIsolatedExamples detects and repairs isolated examples in an endpoint
// This method is defensive - it logs warnings but doesn't fail user operations
func (iaes ItemApiExampleService) AutoLinkIsolatedExamples(ctx context.Context, endpointID idwrap.IDWrap) error {
	// Get all examples (including isolated ones)
	allExamples, err := iaes.GetAllApiExamples(ctx, endpointID)
	if err != nil {
		if err == ErrNoItemApiExampleFound {
			return nil // No examples to repair
		}
		return fmt.Errorf("failed to get all examples: %w", err)
	}

	// Get connected examples via ordered query
	orderedExamples, err := iaes.GetApiExamplesOrdered(ctx, endpointID)
	if err != nil {
		if err == ErrNoItemApiExampleFound {
			// If no ordered examples but we have all examples, all are isolated
			if len(allExamples) > 0 {
				return iaes.movableRepository.RepairIsolatedExamples(ctx, nil, endpointID)
			}
			return nil
		}
		return fmt.Errorf("failed to get ordered examples: %w", err)
	}

	// Compare counts to detect isolated examples
	if len(allExamples) != len(orderedExamples) {
		// Found isolated examples - attempt repair
		fmt.Printf("Auto-linking detected %d isolated examples for endpoint %s (total: %d, connected: %d)\n",
			len(allExamples)-len(orderedExamples), endpointID.String(), len(allExamples), len(orderedExamples))

		err = iaes.movableRepository.RepairIsolatedExamples(ctx, nil, endpointID)
		if err != nil {
			// Log warning but don't fail the operation - this is defensive repair
			fmt.Printf("Warning: failed to auto-link isolated examples for endpoint %s: %v\n", endpointID.String(), err)
			return err
		}

		fmt.Printf("Successfully auto-linked isolated examples for endpoint %s\n", endpointID.String())
	}

	return nil
}

func (iaes ItemApiExampleService) GetApiExamplesWithDefaults(ctx context.Context, endpointID idwrap.IDWrap) ([]mitemapiexample.ItemApiExample, error) {
	itemApiExamples, err := iaes.Queries.GetItemApiExamplesWithDefaults(ctx, endpointID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mitemapiexample.ItemApiExample{}, ErrNoItemApiExampleFound
		}
		return nil, err
	}
	return MassConvert(itemApiExamples, ConvertToModelItem), nil
}

func (iaes ItemApiExampleService) GetDefaultApiExample(ctx context.Context, apiUlid idwrap.IDWrap) (*mitemapiexample.ItemApiExample, error) {
	itemApiExample, err := iaes.Queries.GetItemApiExampleDefault(ctx, apiUlid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoItemApiExampleFound
		}
		return nil, err
	}

	return ConvertToModelItem(itemApiExample), nil
}

func (iaes ItemApiExampleService) GetApiExample(ctx context.Context, id idwrap.IDWrap) (*mitemapiexample.ItemApiExample, error) {
	itemApiExample, err := iaes.Queries.GetItemApiExample(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoItemApiExampleFound
		}
		return nil, err
	}
	return ConvertToModelItem(itemApiExample), nil
}

func (iaes ItemApiExampleService) GetApiExamplesByIDs(ctx context.Context, ids []idwrap.IDWrap) (map[idwrap.IDWrap]*mitemapiexample.ItemApiExample, error) {
	result := make(map[idwrap.IDWrap]*mitemapiexample.ItemApiExample, len(ids))
	if len(ids) == 0 {
		return result, nil
	}

	examples, err := iaes.Queries.GetItemApiExamplesByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	for _, example := range examples {
		model := ConvertToModelItem(example)
		copy := *model
		result[example.ID] = &copy
	}

	return result, nil
}

func (iaes ItemApiExampleService) GetExampleAllParents(ctx context.Context, id idwrap.IDWrap, collectionService scollection.CollectionService, folderService sitemfolder.ItemFolderService, endpointService sitemapi.ItemApiService) ([]mexamplebreadcrumb.ExampleBreadcrumb, error) {

	example, err := iaes.GetApiExample(ctx, id)
	if err != nil {
		return nil, err
	}
	endpoint, err := endpointService.GetItemApi(ctx, example.ItemApiID)
	if err != nil {
		return nil, err
	}

	collection, err := collectionService.GetCollection(ctx, example.CollectionID)
	if err != nil {
		return nil, err
	}

	folderID := endpoint.FolderID
	var folders []mitemfolder.ItemFolder
	for folderID != nil {
		folder, err := folderService.GetFolder(ctx, *folderID)
		if err != nil {
			return nil, err
		}
		folders = append(folders, *folder)
		folderID = folder.ParentID
	}

	var crumbs []mexamplebreadcrumb.ExampleBreadcrumb

	crumbs = append(crumbs, mexamplebreadcrumb.ExampleBreadcrumb{
		Kind:       mexamplebreadcrumb.EXAMPLE_BREADCRUMB_KIND_COLLECTION,
		Collection: collection,
	})
	for _, folder := range folders {
		crumbs = append(crumbs, mexamplebreadcrumb.ExampleBreadcrumb{
			Kind:   mexamplebreadcrumb.EXAMPLE_BREADCRUMB_KIND_FOLDER,
			Folder: &folder,
		})
	}
	crumbs = append(crumbs, mexamplebreadcrumb.ExampleBreadcrumb{
		Kind:     mexamplebreadcrumb.EXAMPLE_BREADCRUMB_KIND_ENDPOINT,
		Endpoint: endpoint,
	})

	return crumbs, nil
}

func (iaes ItemApiExampleService) GetApiExampleByCollection(ctx context.Context, collectionID idwrap.IDWrap) ([]mitemapiexample.ItemApiExample, error) {
	itemApiExamples, err := iaes.Queries.GetItemApiExampleByCollectionID(ctx, collectionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mitemapiexample.ItemApiExample{}, ErrNoItemApiExampleFound
		}
		return nil, err
	}
	return MassConvert(itemApiExamples, ConvertToModelItem), nil
}

func (iaes ItemApiExampleService) GetApiExampleByVersionParentID(ctx context.Context, versionID idwrap.IDWrap) ([]mitemapiexample.ItemApiExample, error) {
	itemApiExamples, err := iaes.Queries.GetItemApiExampleByVersionParentID(ctx, &versionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mitemapiexample.ItemApiExample{}, ErrNoItemApiExampleFound
		}
		return nil, err
	}
	return MassConvert(itemApiExamples, ConvertToModelItem), nil
}

func (iaes ItemApiExampleService) CreateApiExample(ctx context.Context, item *mitemapiexample.ItemApiExample) error {
	// Use the movable repository for proper linked list management
	// This ensures examples are correctly linked when created
	return iaes.movableRepository.Create(ctx, nil, *item)
}

func (iaes ItemApiExampleService) CreateApiExampleBulk(ctx context.Context, items []mitemapiexample.ItemApiExample) error {
	// For bulk creation, use the individual CreateApiExample method to ensure proper linking
	// This is simpler and more reliable than trying to handle bulk linking logic
	for _, item := range items {
		err := iaes.CreateApiExample(ctx, &item)
		if err != nil {
			return fmt.Errorf("failed to create example %s: %w", item.ID.String(), err)
		}
	}

	return nil
}

func (iaes ItemApiExampleService) UpdateItemApiExample(ctx context.Context, item *mitemapiexample.ItemApiExample) error {
	return iaes.Queries.UpdateItemApiExample(ctx, gen.UpdateItemApiExampleParams{
		ID:       item.ID,
		Name:     item.Name,
		BodyType: int8(item.BodyType),
	})
}

func (iaes ItemApiExampleService) UpdateItemApiExampleOrder(ctx context.Context, example *mitemapiexample.ItemApiExample) error {
	return iaes.Queries.UpdateItemApiExampleOrder(ctx, gen.UpdateItemApiExampleOrderParams{
		ID:   example.ID,
		Next: example.Next,
		Prev: example.Prev,
	})
}

func (iaes ItemApiExampleService) DeleteApiExample(ctx context.Context, id idwrap.IDWrap) error {
	mgr := movable.NewDefaultLinkedListManager(iaes.movableRepository)
	return mgr.SafeDelete(ctx, nil, id, func(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap) error {
		return iaes.Queries.DeleteItemApiExample(ctx, itemID)
	})
}

// GetMovableRepository returns the movable repository for example operations
func (iaes ItemApiExampleService) GetMovableRepository() *ExampleMovableRepository {
	return iaes.movableRepository
}

// MoveExample moves an example to a specific position within an endpoint
func (iaes ItemApiExampleService) MoveExample(ctx context.Context, endpointID, exampleID idwrap.IDWrap, position int) error {
	err := iaes.MoveExampleTX(ctx, nil, endpointID, exampleID, position)
	if err != nil {
		return err
	}

	// Auto-repair any isolated examples after move (defensive programming)
	repairErr := iaes.AutoLinkIsolatedExamples(ctx, endpointID)
	if repairErr != nil {
		// Log warning but don't fail the move operation - user's move succeeded
		fmt.Printf("Warning: auto-linking after move failed for endpoint %s: %v\n", endpointID.String(), repairErr)
	}

	return nil
}

// MoveExampleTX moves an example to a specific position within a transaction
func (iaes ItemApiExampleService) MoveExampleTX(ctx context.Context, tx *sql.Tx, endpointID, exampleID idwrap.IDWrap, position int) error {
	service := iaes
	if tx != nil {
		service = iaes.TX(tx)
	}

	// Validate example belongs to endpoint
	example, err := service.GetApiExample(ctx, exampleID)
	if err != nil {
		return fmt.Errorf("example not found: %w", err)
	}

	if example.ItemApiID.Compare(endpointID) != 0 {
		return fmt.Errorf("example does not belong to the specified endpoint")
	}

	// Use repository to perform the move
	repo := service.GetMovableRepository()
	return repo.UpdatePosition(ctx, tx, exampleID, movable.CollectionListTypeExamples, position)
}

// MoveExampleAfter moves an example to be positioned after the target example
func (iaes ItemApiExampleService) MoveExampleAfter(ctx context.Context, endpointID, exampleID, targetExampleID idwrap.IDWrap) error {
	err := iaes.MoveExampleAfterTX(ctx, nil, endpointID, exampleID, targetExampleID)
	if err != nil {
		return err
	}

	// Auto-repair any isolated examples after move (defensive programming)
	repairErr := iaes.AutoLinkIsolatedExamples(ctx, endpointID)
	if repairErr != nil {
		// Log warning but don't fail the move operation - user's move succeeded
		fmt.Printf("Warning: auto-linking after move failed for endpoint %s: %v\n", endpointID.String(), repairErr)
	}

	return nil
}

// MoveExampleAfterTX moves an example to be positioned after the target example within a transaction
func (iaes ItemApiExampleService) MoveExampleAfterTX(ctx context.Context, tx *sql.Tx, endpointID, exampleID, targetExampleID idwrap.IDWrap) error {
	service := iaes
	if tx != nil {
		service = iaes.TX(tx)
	}

	// Validate examples belong to endpoint and prevent self-move
	if err := service.validateExampleMove(ctx, endpointID, exampleID, targetExampleID); err != nil {
		return err
	}

	// Get current position of target example
	targetPosition, err := service.getExamplePosition(ctx, endpointID, targetExampleID)
	if err != nil {
		return fmt.Errorf("failed to get target example position: %w", err)
	}

	// Get total number of examples to ensure position is valid
	orderedExamples, err := service.Queries.GetExamplesByEndpointIDOrdered(ctx, gen.GetExamplesByEndpointIDOrderedParams{
		ItemApiID:   endpointID,
		ItemApiID_2: endpointID,
	})
	if err != nil {
		return fmt.Errorf("failed to get ordered examples: %w", err)
	}

	// Move to position after target (target position + 1)
	// But ensure it doesn't exceed the valid range
	newPosition := targetPosition + 1
	maxPosition := len(orderedExamples) - 1
	if newPosition > maxPosition {
		newPosition = maxPosition
	}

	repo := service.GetMovableRepository()
	return repo.UpdatePosition(ctx, tx, exampleID, movable.CollectionListTypeExamples, newPosition)
}

// MoveExampleBefore moves an example to be positioned before the target example
func (iaes ItemApiExampleService) MoveExampleBefore(ctx context.Context, endpointID, exampleID, targetExampleID idwrap.IDWrap) error {
	err := iaes.MoveExampleBeforeTX(ctx, nil, endpointID, exampleID, targetExampleID)
	if err != nil {
		return err
	}

	// Auto-repair any isolated examples after move (defensive programming)
	repairErr := iaes.AutoLinkIsolatedExamples(ctx, endpointID)
	if repairErr != nil {
		// Log warning but don't fail the move operation - user's move succeeded
		fmt.Printf("Warning: auto-linking after move failed for endpoint %s: %v\n", endpointID.String(), repairErr)
	}

	return nil
}

// MoveExampleBeforeTX moves an example to be positioned before the target example within a transaction
func (iaes ItemApiExampleService) MoveExampleBeforeTX(ctx context.Context, tx *sql.Tx, endpointID, exampleID, targetExampleID idwrap.IDWrap) error {
	service := iaes
	if tx != nil {
		service = iaes.TX(tx)
	}

	// Validate examples belong to endpoint and prevent self-move
	if err := service.validateExampleMove(ctx, endpointID, exampleID, targetExampleID); err != nil {
		return err
	}

	// Get current position of target example
	targetPosition, err := service.getExamplePosition(ctx, endpointID, targetExampleID)
	if err != nil {
		return fmt.Errorf("failed to get target example position: %w", err)
	}

	// Move to position before target (target position)
	repo := service.GetMovableRepository()
	return repo.UpdatePosition(ctx, tx, exampleID, movable.CollectionListTypeExamples, targetPosition)
}

// validateExampleMove validates that examples belong to the endpoint and prevents self-moves
func (iaes ItemApiExampleService) validateExampleMove(ctx context.Context, endpointID, exampleID, targetExampleID idwrap.IDWrap) error {
	// Prevent self-move
	if exampleID.Compare(targetExampleID) == 0 {
		return fmt.Errorf("cannot move example relative to itself")
	}

	// Validate source example belongs to endpoint
	example, err := iaes.GetApiExample(ctx, exampleID)
	if err != nil {
		return fmt.Errorf("source example not found: %w", err)
	}

	if example.ItemApiID.Compare(endpointID) != 0 {
		return fmt.Errorf("source example does not belong to the specified endpoint")
	}

	// Validate target example belongs to endpoint
	targetExample, err := iaes.GetApiExample(ctx, targetExampleID)
	if err != nil {
		return fmt.Errorf("target example not found: %w", err)
	}

	if targetExample.ItemApiID.Compare(endpointID) != 0 {
		return fmt.Errorf("target example does not belong to the specified endpoint")
	}

	return nil
}

// getExamplePosition gets the position of an example within an endpoint
func (iaes ItemApiExampleService) getExamplePosition(ctx context.Context, endpointID, exampleID idwrap.IDWrap) (int, error) {
	// Get ordered examples for the endpoint
	orderedExamples, err := iaes.Queries.GetExamplesByEndpointIDOrdered(ctx, gen.GetExamplesByEndpointIDOrderedParams{
		ItemApiID:   endpointID,
		ItemApiID_2: endpointID,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to get ordered examples: %w", err)
	}

	// Find position of the example
	for i, ex := range orderedExamples {
		if idwrap.NewFromBytesMust(ex.ID).Compare(exampleID) == 0 {
			return i, nil
		}
	}

	return 0, fmt.Errorf("example not found in endpoint")
}
