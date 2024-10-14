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
	itemfolderv1 "dev-tools-services/gen/itemfolder/v1"
	"dev-tools-services/gen/itemfolder/v1/itemfolderv1connect"
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
	path, handler := itemfolderv1connect.NewItemFolderServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *ItemFolderRPC) CreateFolder(ctx context.Context, req *connect.Request[itemfolderv1.CreateFolderRequest]) (*connect.Response[itemfolderv1.CreateFolderResponse], error) {
	reqFolder, err := tfolder.SeralizeRPCToModelWithoutID(req.Msg.Folder)
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

	respRaw := &itemfolderv1.CreateFolderResponse{
		Id: reqFolder.ID.String(),
	}
	return connect.NewResponse(respRaw), nil
}

func (c *ItemFolderRPC) GetFolder(ctx context.Context, req *connect.Request[itemfolderv1.GetFolderRequest]) (*connect.Response[itemfolderv1.GetFolderResponse], error) {
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

func (c *ItemFolderRPC) UpdateFolder(ctx context.Context, req *connect.Request[itemfolderv1.UpdateFolderRequest]) (*connect.Response[itemfolderv1.UpdateFolderResponse], error) {
	ulidID, err := idwrap.NewWithParse(req.Msg.GetFolder().GetMeta().GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	collectionID, err := idwrap.NewWithParse(req.Msg.GetFolder().GetCollectionId())
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

	var parentUlidIDPtr *idwrap.IDWrap = nil
	if req.Msg.GetFolder().GetParentId() != "" {
		parentUlidID, err := idwrap.NewWithParse(req.Msg.GetFolder().GetParentId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		checkfolder, err := c.ifs.GetFolder(ctx, parentUlidID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if checkfolder.CollectionID.Compare(collectionID) != 0 {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
		}
		parentUlidIDPtr = &parentUlidID
	}

	folder := mitemfolder.ItemFolder{
		ID:           ulidID,
		CollectionID: collectionID,
		Name:         req.Msg.GetFolder().GetMeta().GetName(),
		ParentID:     parentUlidIDPtr,
	}

	err = c.ifs.UpdateItemFolder(ctx, &folder)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&itemfolderv1.UpdateFolderResponse{}), nil
}

// DeleteFolder calls collection.v1.CollectionService.DeleteFolder.
func (c *ItemFolderRPC) DeleteFolder(ctx context.Context, req *connect.Request[itemfolderv1.DeleteFolderRequest]) (*connect.Response[itemfolderv1.DeleteFolderResponse], error) {
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

	err = c.ifs.DeleteItemFolder(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&itemfolderv1.DeleteFolderResponse{}), nil
}

func (c *ItemFolderRPC) MoveFolder(context.Context, *connect.Request[itemfolderv1.MoveFolderRequest]) (*connect.Response[itemfolderv1.MoveFolderResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
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
