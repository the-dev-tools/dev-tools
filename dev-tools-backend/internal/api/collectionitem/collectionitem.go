package collectionitem

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/collection"
	"dev-tools-backend/internal/api/ritemfolder"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/permcheck"
	"dev-tools-backend/pkg/service/scollection"
	"dev-tools-backend/pkg/service/sitemapi"
	"dev-tools-backend/pkg/service/sitemfolder"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/translate/tfolder"
	"dev-tools-backend/pkg/translate/titemapi"
	itemv1 "dev-tools-spec/dist/buf/go/collection/item/v1"
	"dev-tools-spec/dist/buf/go/collection/item/v1/itemv1connect"
	"errors"

	"connectrpc.com/connect"
)

type CollectionItemRPC struct {
	DB  *sql.DB
	cs  scollection.CollectionService
	us  suser.UserService
	ifs sitemfolder.ItemFolderService
	ias sitemapi.ItemApiService
}

func New(db *sql.DB, cs scollection.CollectionService) CollectionItemRPC {
	return CollectionItemRPC{
		DB: db,
		cs: cs,
	}
}

func CreateService(srv CollectionItemRPC) (*api.Service, error) {
	path, handler := itemv1connect.NewCollectionItemServiceHandler(&srv)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c CollectionItemRPC) CollectionItemList(ctx context.Context, req *connect.Request[itemv1.CollectionItemListRequest]) (*connect.Response[itemv1.CollectionItemListResponse], error) {
	collectionID, err := idwrap.NewFromBytes(req.Msg.GetCollectionId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(collection.CheckOwnerCollection(ctx, c.cs, c.us, collectionID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	var folderidPtr *idwrap.IDWrap = nil
	if req.Msg.FolderId != nil {
		folderID, err := idwrap.NewFromBytes(req.Msg.FolderId)
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
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	endpoints, err := c.ias.GetApisWithCollectionID(ctx, collectionID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var items []*itemv1.CollectionItem
	if folderidPtr != nil {
		for _, folder := range folders {
			if folder.ParentID != nil && *folder.ParentID == *folderidPtr {
				items = append(items, &itemv1.CollectionItem{
					Kind:   itemv1.ItemKind_ITEM_KIND_FOLDER,
					Folder: tfolder.DeseralizeModelToRPCItem(&folder),
				})
			}
		}

		for _, endpoint := range endpoints {
			if endpoint.ParentID != nil && *endpoint.ParentID == *folderidPtr {
				items = append(items, &itemv1.CollectionItem{
					Kind:     itemv1.ItemKind_ITEM_KIND_ENDPOINT,
					Endpoint: titemapi.DeseralizeModelToRPCItem(&endpoint),
				})
			}
		}

	} else {

		for _, folder := range folders {
			if folder.ParentID == nil {
				items = append(items, &itemv1.CollectionItem{
					Kind:   itemv1.ItemKind_ITEM_KIND_FOLDER,
					Folder: tfolder.DeseralizeModelToRPCItem(&folder),
				})
			}
		}

		for _, endpoint := range endpoints {
			items = append(items, &itemv1.CollectionItem{
				Kind:     itemv1.ItemKind_ITEM_KIND_ENDPOINT,
				Endpoint: titemapi.DeseralizeModelToRPCItem(&endpoint),
			})
		}
	}

	resp := &itemv1.CollectionItemListResponse{
		Items: items,
	}
	return connect.NewResponse(resp), nil
}

func (c CollectionItemRPC) CollectionItemMove(ctx context.Context, req *connect.Request[itemv1.CollectionItemMoveRequest]) (*connect.Response[itemv1.CollectionItemMoveResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}
