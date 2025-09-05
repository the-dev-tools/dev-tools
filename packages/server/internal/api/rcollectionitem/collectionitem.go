package rcollectionitem

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/rcollection"
	"the-dev-tools/server/internal/api/ritemfolder"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/movable"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/scollectionitem"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/translate/texample"
	"the-dev-tools/server/pkg/translate/tfolder"
	"the-dev-tools/server/pkg/translate/titemapi"
	itemv1 "the-dev-tools/spec/dist/buf/go/collection/item/v1"
	"the-dev-tools/spec/dist/buf/go/collection/item/v1/itemv1connect"
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resources/v1"

	"connectrpc.com/connect"
)

// validateMoveKindCompatibility validates that a move operation from sourceKind to targetKind is semantically valid
func validateMoveKindCompatibility(sourceKind, targetKind itemv1.ItemKind) error {
    // If targetKind is unspecified, no validation needed
    if targetKind == itemv1.ItemKind_ITEM_KIND_UNSPECIFIED {
        return nil
    }

    switch sourceKind {
    case itemv1.ItemKind_ITEM_KIND_FOLDER:
        // Moving a folder relative to an endpoint is valid for sibling reordering in collection_items.
        // Actual parent validation happens elsewhere (only folders can be parents).
        return nil

    case itemv1.ItemKind_ITEM_KIND_ENDPOINT:
        switch targetKind {
        case itemv1.ItemKind_ITEM_KIND_FOLDER:
            // ✅ Endpoint to folder: valid (endpoint into folder)
            return nil
        case itemv1.ItemKind_ITEM_KIND_ENDPOINT:
            // ✅ Endpoint to endpoint: valid (endpoint reordering)
            return nil
        default:
            return errors.New("invalid targetKind specified")
        }

    case itemv1.ItemKind_ITEM_KIND_UNSPECIFIED:
        return errors.New("source kind must be specified")

    default:
        return errors.New("invalid source kind specified")
    }
}

type CollectionItemRPC struct {
	DB   *sql.DB
	cs   scollection.CollectionService
	cis  *scollectionitem.CollectionItemService
	us   suser.UserService
	ifs  sitemfolder.ItemFolderService
	ias  sitemapi.ItemApiService
	iaes sitemapiexample.ItemApiExampleService
	res  sexampleresp.ExampleRespService
}

func New(db *sql.DB, cs scollection.CollectionService, cis *scollectionitem.CollectionItemService, us suser.UserService,
	ifs sitemfolder.ItemFolderService, ias sitemapi.ItemApiService,
	iaes sitemapiexample.ItemApiExampleService, res sexampleresp.ExampleRespService,
) CollectionItemRPC {
	return CollectionItemRPC{
		DB:   db,
		cs:   cs,
		cis:  cis,
		us:   us,
		ifs:  ifs,
		ias:  ias,
		iaes: iaes,
		res:  res,
	}
}

func CreateService(srv CollectionItemRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := itemv1connect.NewCollectionItemServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c CollectionItemRPC) CollectionItemList(ctx context.Context, req *connect.Request[itemv1.CollectionItemListRequest]) (*connect.Response[itemv1.CollectionItemListResponse], error) {
	collectionID, err := idwrap.NewFromBytes(req.Msg.GetCollectionId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(rcollection.CheckOwnerCollection(ctx, c.cs, c.us, collectionID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	var folderidPtr *idwrap.IDWrap = nil
	if req.Msg.ParentFolderId != nil {
		legacyFolderID, err := idwrap.NewFromBytes(req.Msg.ParentFolderId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		rpcErr = permcheck.CheckPerm(ritemfolder.CheckOwnerFolder(ctx, c.ifs, c.cs, c.us, legacyFolderID))
		if rpcErr != nil {
			return nil, rpcErr
		}
		
		// Convert legacy folder ID to collection_items folder ID for consistent lookups
		collectionItemsFolderID, err := c.cis.GetCollectionItemIDByLegacyID(ctx, legacyFolderID)
		if err != nil {
			if err == scollectionitem.ErrCollectionItemNotFound {
				return nil, connect.NewError(connect.CodeNotFound, errors.New("folder collection item not found"))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		folderidPtr = &collectionItemsFolderID
	}

	// Use collection_items table to get ordered items
	collectionItems, err := c.cis.ListCollectionItems(ctx, collectionID, folderidPtr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Build RPC response from collection_items data
	items := make([]*itemv1.CollectionItem, 0, len(collectionItems))
	for _, collectionItem := range collectionItems {
		switch collectionItem.ItemType {
		case 0: // Folder
			if collectionItem.FolderID == nil {
				slog.Error("Collection item has folder type but no folder_id", "item_id", collectionItem.ID.String())
				continue
			}
			
			folder, err := c.ifs.GetFolder(ctx, *collectionItem.FolderID)
			if err != nil {
				slog.Error("Failed to get folder for collection item", 
					"item_id", collectionItem.ID.String(), 
					"folder_id", collectionItem.FolderID.String(),
					"error", err)
				continue
			}
			
			items = append(items, &itemv1.CollectionItem{
				Kind:   itemv1.ItemKind_ITEM_KIND_FOLDER,
				Folder: tfolder.SeralizeModelToRPCItem(*folder),
			})

		case 1: // Endpoint
			if collectionItem.EndpointID == nil {
				slog.Error("Collection item has endpoint type but no endpoint_id", "item_id", collectionItem.ID.String())
				continue
			}
			
			endpoint, err := c.ias.GetItemApi(ctx, *collectionItem.EndpointID)
			if err != nil {
				slog.Error("Failed to get endpoint for collection item", 
					"item_id", collectionItem.ID.String(), 
					"endpoint_id", collectionItem.EndpointID.String(),
					"error", err)
				continue
			}
			
			ex, err := c.iaes.GetDefaultApiExample(ctx, endpoint.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			
			resp, err := c.res.GetExampleRespByExampleIDLatest(ctx, ex.ID)
			var respID *idwrap.IDWrap = nil
			if err != nil {
				if err != sql.ErrNoRows {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
			} else {
				respID = &resp.ID
			}

			items = append(items, &itemv1.CollectionItem{
				Kind:     itemv1.ItemKind_ITEM_KIND_ENDPOINT,
				Endpoint: titemapi.SeralizeModelToRPCItem(endpoint),
				Example:  texample.SerializeModelToRPCItem(*ex, respID),
			})

		default:
			slog.Error("Unknown collection item type", "item_id", collectionItem.ID.String(), "type", collectionItem.ItemType)
		}
	}

	resp := &itemv1.CollectionItemListResponse{
		Items: items,
	}
	return connect.NewResponse(resp), nil
}

func (c CollectionItemRPC) CollectionItemMove(ctx context.Context, req *connect.Request[itemv1.CollectionItemMoveRequest]) (*connect.Response[itemv1.CollectionItemMoveResponse], error) {
	// Validate required fields
	if len(req.Msg.GetItemId()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("item_id is required"))
	}
	
	if len(req.Msg.GetCollectionId()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("collection_id is required"))
	}

	// Parse item ID (this could be a legacy ID)
	itemIDRaw, err := idwrap.NewFromBytes(req.Msg.GetItemId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Parse collection ID
	collectionID, err := idwrap.NewFromBytes(req.Msg.GetCollectionId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Check collection permission
	rpcErr := permcheck.CheckPerm(rcollection.CheckOwnerCollection(ctx, c.cs, c.us, collectionID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Get the workspace ID for additional security check
	collectionWorkspaceID, err := c.cs.GetWorkspaceID(ctx, collectionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("collection not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Convert legacy item ID to collection_items ID if needed
	itemID, err := c.cis.GetCollectionItemIDByLegacyID(ctx, itemIDRaw)
	if err != nil {
		if err == scollectionitem.ErrCollectionItemNotFound {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("collection item not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Verify the collection item belongs to this workspace (additional security check)
	belongsToWorkspace, err := c.cis.CheckWorkspaceID(ctx, itemID, collectionWorkspaceID)
	if err != nil {
		if err == scollectionitem.ErrCollectionItemNotFound {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("collection item not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !belongsToWorkspace {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("collection item does not belong to the specified workspace"))
	}

	// Parse target item ID if provided
	var targetID *idwrap.IDWrap
	if len(req.Msg.GetTargetItemId()) > 0 {
		targetIDRaw, err := idwrap.NewFromBytes(req.Msg.GetTargetItemId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		
		// Convert legacy target ID to collection_items ID if needed
		targetIDConverted, err := c.cis.GetCollectionItemIDByLegacyID(ctx, targetIDRaw)
		if err != nil {
			if err == scollectionitem.ErrCollectionItemNotFound {
				return nil, connect.NewError(connect.CodeNotFound, errors.New("target collection item not found"))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		targetID = &targetIDConverted
		
		// Validate target item exists and belongs to same workspace
		targetBelongsToWorkspace, err := c.cis.CheckWorkspaceID(ctx, *targetID, collectionWorkspaceID)
		if err != nil {
			if err == scollectionitem.ErrCollectionItemNotFound {
				return nil, connect.NewError(connect.CodeNotFound, errors.New("target collection item not found"))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if !targetBelongsToWorkspace {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("target collection item does not belong to the specified workspace"))
		}
	}

	// Parse target parent folder ID if provided
	var targetParentFolderID *idwrap.IDWrap
	if len(req.Msg.GetTargetParentFolderId()) > 0 {
		targetParentIDRaw, err := idwrap.NewFromBytes(req.Msg.GetTargetParentFolderId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		
		// Convert legacy parent folder ID to collection_items ID if needed
		targetParentIDConverted, err := c.cis.GetCollectionItemIDByLegacyID(ctx, targetParentIDRaw)
		if err != nil {
			if err == scollectionitem.ErrCollectionItemNotFound {
				return nil, connect.NewError(connect.CodeNotFound, errors.New("target parent folder not found"))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		targetParentFolderID = &targetParentIDConverted
		
		// Validate target parent folder exists and belongs to same workspace
		targetParentBelongsToWorkspace, err := c.cis.CheckWorkspaceID(ctx, *targetParentFolderID, collectionWorkspaceID)
		if err != nil {
			if err == scollectionitem.ErrCollectionItemNotFound {
				return nil, connect.NewError(connect.CodeNotFound, errors.New("target parent folder not found"))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if !targetParentBelongsToWorkspace {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("target parent folder does not belong to the specified workspace"))
		}
	}

	// Parse target collection ID if provided (for cross-collection moves)
	var targetCollectionID *idwrap.IDWrap
	if len(req.Msg.GetTargetCollectionId()) > 0 {
		targetCollectionIDRaw, err := idwrap.NewFromBytes(req.Msg.GetTargetCollectionId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		targetCollectionID = &targetCollectionIDRaw
		
		// Check permission on target collection
		rpcErr := permcheck.CheckPerm(rcollection.CheckOwnerCollection(ctx, c.cs, c.us, targetCollectionIDRaw))
		if rpcErr != nil {
			return nil, rpcErr
		}
		
		// Validate target collection exists and is in same workspace as source collection
		targetCollectionWorkspaceID, err := c.cs.GetWorkspaceID(ctx, targetCollectionIDRaw)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, connect.NewError(connect.CodeNotFound, errors.New("target collection not found"))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		
		if collectionWorkspaceID.Compare(targetCollectionWorkspaceID) != 0 {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot move items between different workspaces"))
		}
	}

	// Parse targetKind field if provided (for semantic validation)
	targetKind := req.Msg.GetTargetKind()
	sourceKind := req.Msg.GetKind()

	// Validate targetKind semantics if specified
	if targetKind != itemv1.ItemKind_ITEM_KIND_UNSPECIFIED {
		err := validateMoveKindCompatibility(sourceKind, targetKind)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
	}

	// Parse and validate position
	rpcPosition := req.Msg.GetPosition()
	if rpcPosition == resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED && targetID != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("position must be specified when target_item_id is provided"))
	}

	// Convert RPC position to internal position
	var movePosition movable.MovePosition
	switch rpcPosition {
	case resourcesv1.MovePosition_MOVE_POSITION_AFTER:
		movePosition = movable.MovePositionAfter
	case resourcesv1.MovePosition_MOVE_POSITION_BEFORE:
		movePosition = movable.MovePositionBefore
	case resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED:
		// Default to after if no target specified (move to end)
		movePosition = movable.MovePositionAfter
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid move position"))
	}

	// Prevent moving item relative to itself
	if targetID != nil && itemID.Compare(*targetID) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cannot move item relative to itself"))
	}

    // Determine source collection to route same-collection vs cross-collection correctly
    currentItem, err := c.cis.GetCollectionItem(ctx, itemID)
    if err != nil {
        return nil, connect.NewError(connect.CodeInternal, err)
    }

    // Same-collection when targetCollectionID is nil or equals current item's collection
    isCrossCollectionMove := targetCollectionID != nil && currentItem.CollectionID.Compare(*targetCollectionID) != 0
    isTargetParentFolderMove := targetParentFolderID != nil || req.Msg.TargetParentFolderId != nil

	// Add comprehensive logging for move operations
	slog.Debug("Collection item move operation",
		"item_id", itemID.String(),
		"source_kind", sourceKind.String(),
		"target_kind", targetKind.String(),
		"collection_id", collectionID.String(),
		"target_collection_id", func() string {
			if targetCollectionID != nil {
				return targetCollectionID.String()
			}
			return "nil"
		}(),
		"target_parent_folder_id", func() string {
			if targetParentFolderID != nil {
				return targetParentFolderID.String()
			}
			return "nil"
		}(),
		"target_item_id", func() string {
			if targetID != nil {
				return targetID.String()
			}
			return "nil"
		}(),
		"position", movePosition,
		"is_cross_collection", isCrossCollectionMove,
		"is_target_parent_folder", isTargetParentFolderMove)

	// Route to appropriate service method based on operation type
    if isCrossCollectionMove {
        // Cross-collection move: use dedicated cross-collection service method
        slog.Debug("Executing cross-collection move",
            "source_collection", collectionID.String(),
            "target_collection", targetCollectionID.String())
        err = c.cis.MoveCollectionItemCrossCollection(ctx, itemID, *targetCollectionID, targetParentFolderID, targetID, movePosition)
    } else if isTargetParentFolderMove {
        // Same-collection move with target parent folder specified
        slog.Debug("Executing same-collection parent folder move")
        // For same-collection parent move, ignore targetCollectionID (even if provided and equal)
        err = c.cis.MoveCollectionItemToFolder(ctx, itemID, targetParentFolderID, targetID, movePosition, nil)
	} else {
		// Traditional same-collection move with target item positioning
		slog.Debug("Executing traditional same-collection move")
		err = c.cis.MoveCollectionItem(ctx, itemID, targetID, movePosition)
	}
	
	if err != nil {
		slog.Error("Failed to move collection item", 
			"error", err,
			"item_id", itemID.String(),
			"source_kind", sourceKind.String(),
			"target_kind", targetKind.String(),
			"collection_id", collectionID.String(),
			"target_collection_id", func() string {
				if targetCollectionID != nil {
					return targetCollectionID.String()
				}
				return "nil"
			}(),
			"target_id", func() string {
				if targetID != nil {
					return targetID.String()
				}
				return "nil"
			}(),
			"position", movePosition,
			"is_cross_collection", isCrossCollectionMove,
			"is_target_parent_folder", isTargetParentFolderMove)

		switch {
		case errors.Is(err, scollectionitem.ErrCollectionItemNotFound):
			return nil, connect.NewError(connect.CodeNotFound, err)
		case errors.Is(err, scollectionitem.ErrInvalidItemType):
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		case errors.Is(err, scollectionitem.ErrInvalidTargetPosition):
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		case errors.Is(err, scollectionitem.ErrCrossWorkspaceMove):
			return nil, connect.NewError(connect.CodePermissionDenied, err)
		case errors.Is(err, scollectionitem.ErrTargetCollectionNotFound):
			return nil, connect.NewError(connect.CodeNotFound, err)
		default:
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&itemv1.CollectionItemMoveResponse{}), nil
}
