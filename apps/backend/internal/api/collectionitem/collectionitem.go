package collectionitem

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/backend/internal/api"
	"the-dev-tools/backend/internal/api/rcollection"
	"the-dev-tools/backend/internal/api/ritemfolder"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/permcheck"
	"the-dev-tools/backend/pkg/service/scollection"
	"the-dev-tools/backend/pkg/service/sexampleresp"
	"the-dev-tools/backend/pkg/service/sitemapi"
	"the-dev-tools/backend/pkg/service/sitemapiexample"
	"the-dev-tools/backend/pkg/service/sitemfolder"
	"the-dev-tools/backend/pkg/service/suser"
	"the-dev-tools/backend/pkg/translate/texample"
	"the-dev-tools/backend/pkg/translate/tfolder"
	"the-dev-tools/backend/pkg/translate/titemapi"
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
			if endpoint.ParentID != nil && *endpoint.ParentID == *folderidPtr {
				ex, err := c.iaes.GetDefaultApiExample(ctx, endpoint.ID)
				if err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
				resp, err := c.res.GetExampleRespByExampleID(ctx, ex.ID)
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
			if endpoint.ParentID == nil {
				ex, err := c.iaes.GetDefaultApiExample(ctx, endpoint.ID)
				if err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
				resp, err := c.res.GetExampleRespByExampleID(ctx, ex.ID)
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
		Items:        items,
		CollectionId: req.Msg.CollectionId,
		FolderId:     req.Msg.FolderId,
	}
	return connect.NewResponse(resp), nil
}

func (c CollectionItemRPC) CollectionItemMove(ctx context.Context, req *connect.Request[itemv1.CollectionItemMoveRequest]) (*connect.Response[itemv1.CollectionItemMoveResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}
