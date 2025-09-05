package ritemfolder

import (
	"context"
	"database/sql"
	"errors"
	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/rcollection"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/scollectionitem"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/translate/tfolder"
	folderv1 "the-dev-tools/spec/dist/buf/go/collection/item/folder/v1"
	"the-dev-tools/spec/dist/buf/go/collection/item/folder/v1/folderv1connect"

	"connectrpc.com/connect"
)

type ItemFolderRPC struct {
	DB  *sql.DB
	ifs sitemfolder.ItemFolderService
	us  suser.UserService
	cs  scollection.CollectionService
	cis *scollectionitem.CollectionItemService
}

func New(db *sql.DB, ifs sitemfolder.ItemFolderService, us suser.UserService, cs scollection.CollectionService, cis *scollectionitem.CollectionItemService) ItemFolderRPC {
	return ItemFolderRPC{
		DB:  db,
		ifs: ifs,
		us:  us,
		cs:  cs,
		cis: cis,
	}
}

func CreateService(srv ItemFolderRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := folderv1connect.NewFolderServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *ItemFolderRPC) FolderCreate(ctx context.Context, req *connect.Request[folderv1.FolderCreateRequest]) (*connect.Response[folderv1.FolderCreateResponse], error) {
	collectionID, err := idwrap.NewFromBytes(req.Msg.GetCollectionId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	folderConv := &folderv1.Folder{
		Name:           req.Msg.GetName(),
		ParentFolderId: req.Msg.GetParentFolderId(),
	}

	reqFolder, err := tfolder.SeralizeRPCToModelWithoutID(folderConv, collectionID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	ID := idwrap.NewNow()
	reqFolder.ID = ID

	rpcErr := permcheck.CheckPerm(rcollection.CheckOwnerCollection(ctx, c.cs, c.us, reqFolder.CollectionID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	if reqFolder.ParentID != nil {
		rpcErr = permcheck.CheckPerm(CheckOwnerFolder(ctx, c.ifs, c.cs, c.us, *reqFolder.ParentID))
		if rpcErr != nil {
			return nil, rpcErr
		}
	}

	tx, err := c.DB.Begin()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// Use CollectionItemService to create folder with unified ordering
	err = c.cis.CreateFolderTX(ctx, tx, reqFolder)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	respRaw := &folderv1.FolderCreateResponse{
		FolderId: ID.Bytes(),
	}
	return connect.NewResponse(respRaw), nil
}

func (c *ItemFolderRPC) FolderUpdate(ctx context.Context, req *connect.Request[folderv1.FolderUpdateRequest]) (*connect.Response[folderv1.FolderUpdateResponse], error) {
	folderID, err := idwrap.NewFromBytes(req.Msg.GetFolderId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerFolder(ctx, c.ifs, c.cs, c.us, folderID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	folder, err := c.ifs.GetFolder(ctx, folderID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	msg := req.Msg
	if msg.GetName() != "" {
		folder.Name = req.Msg.GetName()
	}
	parentID := msg.GetParentFolderId()
	if parentID != nil {
		if len(parentID) != 0 {
			parentID, err := idwrap.NewFromBytes(msg.ParentFolderId)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
			rpcErr = permcheck.CheckPerm(CheckOwnerFolder(ctx, c.ifs,
				c.cs, c.us, parentID))
			if rpcErr != nil {
				return nil, rpcErr
			}
			_, err = c.ifs.GetFolder(ctx, parentID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			folder.ParentID = &parentID
		}
	}

	err = c.ifs.UpdateItemFolder(ctx, folder)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&folderv1.FolderUpdateResponse{}), nil
}

func (c *ItemFolderRPC) FolderDelete(ctx context.Context, req *connect.Request[folderv1.FolderDeleteRequest]) (*connect.Response[folderv1.FolderDeleteResponse], error) {
	ulidID, err := idwrap.NewFromBytes(req.Msg.GetFolderId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	isOwner, err := CheckOwnerFolder(ctx, c.ifs, c.cs, c.us, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !isOwner {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

    // Read-only stage: fetch folder (validation) and prefetch mapping to collection_items BEFORE opening tx
    if _, err := c.ifs.GetFolder(ctx, ulidID); err != nil {
        return nil, connect.NewError(connect.CodeInternal, err)
    }
    mappedItemID, mapErr := c.cis.GetCollectionItemIDByLegacyID(ctx, ulidID)

	tx, err := c.DB.Begin()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

    txIfs, err := sitemfolder.NewTX(ctx, tx)
    if err != nil {
        return nil, connect.NewError(connect.CodeInternal, err)
    }
    txcis := c.cis.TX(tx)
    // Unlink from collection_items if mapping exists, then delete legacy folder row
    if mapErr == nil {
        if derr := txcis.DeleteCollectionItem(ctx, tx, mappedItemID); derr != nil {
            return nil, connect.NewError(connect.CodeInternal, derr)
        }
    }
    err = txIfs.DeleteItemFolder(ctx, ulidID)
    if err != nil {
        return nil, connect.NewError(connect.CodeInternal, err)
    }

	err = tx.Commit()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&folderv1.FolderDeleteResponse{}), nil
}

func CheckOwnerFolder(ctx context.Context, ifs sitemfolder.ItemFolderService, cs scollection.CollectionService, us suser.UserService, folderID idwrap.IDWrap) (bool, error) {
	folder, err := ifs.GetFolder(ctx, folderID)
	if err != nil {
		return false, err
	}

	isOwner, err := rcollection.CheckOwnerCollection(ctx, cs, us, folder.CollectionID)
	if err != nil {
		return false, err
	}
	return isOwner, nil
}
