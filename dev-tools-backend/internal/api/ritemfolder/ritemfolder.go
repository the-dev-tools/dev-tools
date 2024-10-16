package ritemfolder

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/collection"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mitemfolder"
	"dev-tools-backend/pkg/permcheck"
	"dev-tools-backend/pkg/service/scollection"
	"dev-tools-backend/pkg/service/sitemfolder"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/translate/tfolder"
	folderv1 "dev-tools-spec/dist/buf/go/collection/item/folder/v1"
	"dev-tools-spec/dist/buf/go/collection/item/folder/v1/folderv1connect"
	"errors"

	"connectrpc.com/connect"
)

type ItemFolderRPC struct {
	DB  *sql.DB
	ifs sitemfolder.ItemFolderService
	us  suser.UserService
	cs  scollection.CollectionService
}

func New(db *sql.DB, ifs sitemfolder.ItemFolderService, us suser.UserService, cs scollection.CollectionService) ItemFolderRPC {
	return ItemFolderRPC{
		DB:  db,
		ifs: ifs,
		us:  us,
		cs:  cs,
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
	reqFolder.ID = idwrap.NewNow()

	rpcErr := permcheck.CheckPerm(collection.CheckOwnerCollection(ctx, c.cs, c.us, reqFolder.CollectionID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	rpcErr = permcheck.CheckPerm(CheckOwnerFolder(ctx, c.ifs, c.cs, c.us, *reqFolder.ParentID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	folder, err := c.ifs.GetLastFolder(ctx, reqFolder.CollectionID, reqFolder.ParentID, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	reqFolder.Prev = &folder.ID
	folder.Next = &reqFolder.ID

	tx, err := c.DB.Begin()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	ifsTX, err := sitemfolder.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = ifsTX.UpdateItemFolder(ctx, folder)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = ifsTX.CreateItemFolder(ctx, reqFolder)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	respRaw := &folderv1.FolderCreateResponse{
		FolderId: reqFolder.ID.Bytes(),
	}
	return connect.NewResponse(respRaw), nil
}

/*
func (c *ItemFolderRPC) GetFolder(ctx context.Context, req *connect.Request[folderv1.Folder]) (*connect.Response[itemfolderv1.GetFolderResponse], error) {
	ulidID, err := idwrap.NewWithParse(req.Msg.GetId())
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

	folder, err := c.ifs.GetFolder(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// TODO: add items
	respRaw := &itemfolderv1.GetFolderResponse{
		Folder: &itemfolderv1.Folder{
			Meta: &itemfolderv1.FolderMeta{
				Id:   folder.ID.String(),
				Name: folder.Name,
			},
			Items: []*itemfolderv1.Item{},
		},
	}

	return connect.NewResponse(respRaw), nil
}
*/

func (c *ItemFolderRPC) FolderUpdate(ctx context.Context, req *connect.Request[folderv1.FolderUpdateRequest]) (*connect.Response[folderv1.FolderUpdateResponse], error) {
	folderID, err := idwrap.NewFromBytes(req.Msg.GetFolderId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerFolder(ctx, c.ifs, c.cs, c.us, folderID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	var parentUlidIDPtr *idwrap.IDWrap = nil
	if req.Msg.FolderId != nil && len(req.Msg.FolderId) > 0 {
		parentUlidID, err := idwrap.NewFromBytes(req.Msg.FolderId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		rpcErr = permcheck.CheckPerm(CheckOwnerFolder(ctx, c.ifs, c.cs, c.us, parentUlidID))
		if rpcErr != nil {
			return nil, rpcErr
		}
		_, err = c.ifs.GetFolder(ctx, parentUlidID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		parentUlidIDPtr = &parentUlidID
	}

	folder := mitemfolder.ItemFolder{
		ID:       folderID,
		Name:     req.Msg.GetName(),
		ParentID: parentUlidIDPtr,
	}

	err = c.ifs.UpdateItemFolder(ctx, &folder)
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

	err = c.ifs.DeleteItemFolder(ctx, ulidID)
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

	isOwner, err := collection.CheckOwnerCollection(ctx, cs, us, folder.CollectionID)
	if err != nil {
		return false, err
	}
	return isOwner, nil
}
