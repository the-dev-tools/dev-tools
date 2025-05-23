package rcollectionitem

import (
	"context"
	"database/sql"
	"errors"
	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/rcollection"
	"the-dev-tools/server/internal/api/ritemfolder"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemfolder"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/service/scollection"
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

	"connectrpc.com/connect"
)

type CollectionItemRPC struct {
	DB   *sql.DB
	cs   scollection.CollectionService
	us   suser.UserService
	ifs  sitemfolder.ItemFolderService
	ias  sitemapi.ItemApiService
	iaes sitemapiexample.ItemApiExampleService
	res  sexampleresp.ExampleRespService
}

func New(db *sql.DB, cs scollection.CollectionService, us suser.UserService,
	ifs sitemfolder.ItemFolderService, ias sitemapi.ItemApiService,
	iaes sitemapiexample.ItemApiExampleService, res sexampleresp.ExampleRespService,
) CollectionItemRPC {
	return CollectionItemRPC{
		DB:   db,
		cs:   cs,
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
		folderID, err := idwrap.NewFromBytes(req.Msg.ParentFolderId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		rpcErr = permcheck.CheckPerm(ritemfolder.CheckOwnerFolder(ctx, c.ifs, c.cs, c.us, folderID))
		if rpcErr != nil {
			return nil, rpcErr
		}
		folderidPtr = &folderID
	}

	// TODO: add queries to just get root folders
	folders, err := c.ifs.GetFoldersWithCollectionID(ctx, collectionID)
	if err != nil {
		if err != sitemfolder.ErrNoItemFolderFound {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	endpoints, err := c.ias.GetApisWithCollectionID(ctx, collectionID)
	if err != nil {
		if err != sitemapi.ErrNoItemApiFound {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// TODO: make this more efficient
	items := make([]*itemv1.CollectionItem, 0, len(folders)+len(endpoints))
	if folderidPtr != nil {
		for _, folder := range folders {
			if folder.ParentID != nil && *folder.ParentID == *folderidPtr {
				// grow the slice
				items = append(items, &itemv1.CollectionItem{
					Kind:   itemv1.ItemKind_ITEM_KIND_FOLDER,
					Folder: tfolder.SeralizeModelToRPCItem(folder),
				})
			}
		}

		for _, endpoint := range endpoints {
			if endpoint.FolderID != nil && *endpoint.FolderID == *folderidPtr {
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
					Endpoint: titemapi.SeralizeModelToRPCItem(&endpoint),
					Example:  texample.SerializeModelToRPCItem(*ex, respID),
				})
			}
		}

	} else {
		for _, folder := range folders {
			if folder.ParentID == nil {
				items = append(items, &itemv1.CollectionItem{
					Kind:   itemv1.ItemKind_ITEM_KIND_FOLDER,
					Folder: tfolder.SeralizeModelToRPCItem(folder),
				})
			}
		}

		for _, endpoint := range endpoints {
			if endpoint.FolderID == nil {
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

				rpcEx := texample.SerializeModelToRPCItem(*ex, respID)

				items = append(items, &itemv1.CollectionItem{
					Kind:     itemv1.ItemKind_ITEM_KIND_ENDPOINT,
					Endpoint: titemapi.SeralizeModelToRPCItem(&endpoint),
					Example:  rpcEx,
				})
			}
		}
	}

	resp := &itemv1.CollectionItemListResponse{
		Items: items,
	}
	return connect.NewResponse(resp), nil
}

func (c CollectionItemRPC) CollectionItemMove(ctx context.Context, req *connect.Request[itemv1.CollectionItemMoveRequest]) (*connect.Response[itemv1.CollectionItemMoveResponse], error) {
	var targetNextID, targetPrevID *idwrap.IDWrap

	if len(req.Msg.NextItemId) != 0 {
		targetNextIDTemp, err := idwrap.NewFromBytes(req.Msg.NextItemId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		targetNextID = &targetNextIDTemp
	}

	if len(req.Msg.PreviousItemId) != 0 {
		targetPrevIDTemp, err := idwrap.NewFromBytes(req.Msg.PreviousItemId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		targetPrevID = &targetPrevIDTemp
	}

	switch req.Msg.Kind {
	case itemv1.ItemKind_ITEM_KIND_FOLDER:
		folderID, err := idwrap.NewFromBytes(req.Msg.ItemId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		tx, err := c.DB.Begin()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		defer devtoolsdb.TxnRollback(tx)

		txIfs, err := sitemfolder.NewTX(ctx, tx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		folder, err := c.ifs.GetFolder(ctx, folderID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		currentPrev, currentNext := folder.Prev, folder.Next
		var currentPrevFolder, currentNextFolder *mitemfolder.ItemFolder
		// Get the current prev and next folders
		if currentPrev != nil {
			currentPrevFolder, err = c.ifs.GetFolder(ctx, *currentPrev)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			currentPrevFolder.Next = currentNext
		}
		if currentNext != nil {
			currentNextFolder, err = c.ifs.GetFolder(ctx, *currentNext)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			currentNextFolder.Prev = currentPrev

		}

		// Get the target prev and next folders
		var targetPrevFolder, targetNextFolder *mitemfolder.ItemFolder
		if targetNextID != nil {
			targetNextFolder, err := c.ifs.GetFolder(ctx, *targetNextID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			folder.Next = targetNextID
			targetNextFolder.Prev = &folder.ID
		}
		if targetPrevID != nil {
			targetPrevFolder, err := c.ifs.GetFolder(ctx, *targetPrevID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			folder.Prev = targetPrevID
			targetPrevFolder.Next = &folder.ID
		}

		updateOrderFolderArr := []*mitemfolder.ItemFolder{
			currentPrevFolder, currentNextFolder, targetPrevFolder, targetNextFolder, folder,
		}

		for _, folder := range updateOrderFolderArr {
			err = txIfs.UpdateOrder(ctx, folder)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}

		err = tx.Commit()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

	case itemv1.ItemKind_ITEM_KIND_ENDPOINT:
		endpointID, err := idwrap.NewFromBytes(req.Msg.ItemId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		tx, err := c.DB.Begin()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		defer devtoolsdb.TxnRollback(tx)

		txIas, err := sitemapi.NewTX(ctx, tx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		endpoint, err := c.ias.GetItemApi(ctx, endpointID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		currentPrev, currentNext := endpoint.Prev, endpoint.Next
		var currentPrevEndpoint, currentNextEndpoint *mitemapi.ItemApi
		if currentPrev != nil {
			currentPrevEndpoint, err = c.ias.GetItemApi(ctx, *currentPrev)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			currentPrevEndpoint.Next = currentNext
		}
		if currentNext != nil {
			currentNextEndpoint, err = c.ias.GetItemApi(ctx, *currentNext)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			currentNextEndpoint.Prev = currentPrev
		}

		var targetPrevEndpoint, targetNextEndpoint *mitemapi.ItemApi
		if targetNextID != nil {
			targetNextEndpoint, err = c.ias.GetItemApi(ctx, *targetNextID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			endpoint.Next = targetNextID
			targetNextEndpoint.Prev = &endpoint.ID
		}
		if targetPrevID != nil {
			targetPrevEndpoint, err = c.ias.GetItemApi(ctx, *targetPrevID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			endpoint.Prev = targetPrevID
			targetPrevEndpoint.Next = &endpoint.ID
		}

		updateOrderEndpointArr := []*mitemapi.ItemApi{
			currentPrevEndpoint, currentNextEndpoint, targetPrevEndpoint, targetNextEndpoint, endpoint,
		}

		for _, ep := range updateOrderEndpointArr {
			if ep != nil {
				err = txIas.UpdateItemApiOrder(ctx, ep)
				if err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
			}
		}

		err = tx.Commit()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid item kind"))
	}

	return connect.NewResponse(&itemv1.CollectionItemMoveResponse{}), nil
}
