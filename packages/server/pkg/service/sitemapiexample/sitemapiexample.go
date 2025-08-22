package sitemapiexample

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mexamplebreadcrumb"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mitemfolder"
	"the-dev-tools/server/pkg/movable"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemfolder"
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
	arg := ConvertToDBItem(*item)
	return iaes.Queries.CreateItemApiExample(ctx, gen.CreateItemApiExampleParams{
		ID:              arg.ID,
		ItemApiID:       arg.ItemApiID,
		CollectionID:    arg.CollectionID,
		IsDefault:       arg.IsDefault,
		BodyType:        arg.BodyType,
		Name:            arg.Name,
		VersionParentID: arg.VersionParentID,
		Prev:            arg.Prev,
		Next:            arg.Next,
	})
}

func (iaes ItemApiExampleService) CreateApiExampleBulk(ctx context.Context, items []mitemapiexample.ItemApiExample) error {
	sizeOfChunks := 10

	for chunk := range slices.Chunk(items, sizeOfChunks) {
		if len(chunk) < sizeOfChunks {
			for _, item := range chunk {
				err := iaes.CreateApiExample(ctx, &item)
				if err != nil {
					return err
				}
			}
			continue
		}

		// Convert all items in the chunk using ConvertToDBItem
		dbItems := make([]gen.ItemApiExample, len(chunk))
		for i, item := range chunk {
			dbItems[i] = ConvertToDBItem(item)
		}

		params := gen.CreateItemApiExampleBulkParams{
			ID:              dbItems[0].ID,
			ItemApiID:       dbItems[0].ItemApiID,
			CollectionID:    dbItems[0].CollectionID,
			IsDefault:       dbItems[0].IsDefault,
			BodyType:        dbItems[0].BodyType,
			Name:            dbItems[0].Name,
			VersionParentID: dbItems[0].VersionParentID,
			Prev:            dbItems[0].Prev,
			Next:            dbItems[0].Next,

			ID_2:              dbItems[1].ID,
			ItemApiID_2:       dbItems[1].ItemApiID,
			CollectionID_2:    dbItems[1].CollectionID,
			IsDefault_2:       dbItems[1].IsDefault,
			BodyType_2:        dbItems[1].BodyType,
			Name_2:            dbItems[1].Name,
			VersionParentID_2: dbItems[1].VersionParentID,
			Prev_2:            dbItems[1].Prev,
			Next_2:            dbItems[1].Next,

			ID_3:              dbItems[2].ID,
			ItemApiID_3:       dbItems[2].ItemApiID,
			CollectionID_3:    dbItems[2].CollectionID,
			IsDefault_3:       dbItems[2].IsDefault,
			BodyType_3:        dbItems[2].BodyType,
			Name_3:            dbItems[2].Name,
			VersionParentID_3: dbItems[2].VersionParentID,
			Prev_3:            dbItems[2].Prev,
			Next_3:            dbItems[2].Next,

			ID_4:              dbItems[3].ID,
			ItemApiID_4:       dbItems[3].ItemApiID,
			CollectionID_4:    dbItems[3].CollectionID,
			IsDefault_4:       dbItems[3].IsDefault,
			BodyType_4:        dbItems[3].BodyType,
			Name_4:            dbItems[3].Name,
			VersionParentID_4: dbItems[3].VersionParentID,
			Prev_4:            dbItems[3].Prev,
			Next_4:            dbItems[3].Next,

			ID_5:              dbItems[4].ID,
			ItemApiID_5:       dbItems[4].ItemApiID,
			CollectionID_5:    dbItems[4].CollectionID,
			IsDefault_5:       dbItems[4].IsDefault,
			BodyType_5:        dbItems[4].BodyType,
			Name_5:            dbItems[4].Name,
			VersionParentID_5: dbItems[4].VersionParentID,
			Prev_5:            dbItems[4].Prev,
			Next_5:            dbItems[4].Next,

			ID_6:              dbItems[5].ID,
			ItemApiID_6:       dbItems[5].ItemApiID,
			CollectionID_6:    dbItems[5].CollectionID,
			IsDefault_6:       dbItems[5].IsDefault,
			BodyType_6:        dbItems[5].BodyType,
			Name_6:            dbItems[5].Name,
			VersionParentID_6: dbItems[5].VersionParentID,
			Prev_6:            dbItems[5].Prev,
			Next_6:            dbItems[5].Next,

			ID_7:              dbItems[6].ID,
			ItemApiID_7:       dbItems[6].ItemApiID,
			CollectionID_7:    dbItems[6].CollectionID,
			IsDefault_7:       dbItems[6].IsDefault,
			BodyType_7:        dbItems[6].BodyType,
			Name_7:            dbItems[6].Name,
			VersionParentID_7: dbItems[6].VersionParentID,
			Prev_7:            dbItems[6].Prev,
			Next_7:            dbItems[6].Next,

			ID_8:              dbItems[7].ID,
			ItemApiID_8:       dbItems[7].ItemApiID,
			CollectionID_8:    dbItems[7].CollectionID,
			IsDefault_8:       dbItems[7].IsDefault,
			BodyType_8:        dbItems[7].BodyType,
			Name_8:            dbItems[7].Name,
			VersionParentID_8: dbItems[7].VersionParentID,
			Prev_8:            dbItems[7].Prev,
			Next_8:            dbItems[7].Next,

			ID_9:              dbItems[8].ID,
			ItemApiID_9:       dbItems[8].ItemApiID,
			CollectionID_9:    dbItems[8].CollectionID,
			IsDefault_9:       dbItems[8].IsDefault,
			BodyType_9:        dbItems[8].BodyType,
			Name_9:            dbItems[8].Name,
			VersionParentID_9: dbItems[8].VersionParentID,
			Prev_9:            dbItems[8].Prev,
			Next_9:            dbItems[8].Next,

			ID_10:              dbItems[9].ID,
			ItemApiID_10:       dbItems[9].ItemApiID,
			CollectionID_10:    dbItems[9].CollectionID,
			IsDefault_10:       dbItems[9].IsDefault,
			BodyType_10:        dbItems[9].BodyType,
			Name_10:            dbItems[9].Name,
			VersionParentID_10: dbItems[9].VersionParentID,
			Prev_10:            dbItems[9].Prev,
			Next_10:            dbItems[9].Next,
		}

		if err := iaes.Queries.CreateItemApiExampleBulk(ctx, params); err != nil {
			return fmt.Errorf("failed to create bulk examples: %w", err)
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
	return iaes.Queries.DeleteItemApiExample(ctx, id)
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
