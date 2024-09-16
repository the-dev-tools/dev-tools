package ritemfolder

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/collection"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/internal/api/middleware/mwcompress"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mitemfolder"
	"dev-tools-backend/pkg/service/scollection"
	"dev-tools-backend/pkg/service/sitemfolder"
	"dev-tools-backend/pkg/service/suser"
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

func CreateService(ctx context.Context, db *sql.DB, secret []byte) (*api.Service, error) {
	cs, err := scollection.New(ctx, db)
	if err != nil {
		return nil, err
	}

	ifs, err := sitemfolder.New(ctx, db)
	if err != nil {
		return nil, err
	}

	us, err := suser.New(ctx, db)
	if err != nil {
		return nil, err
	}

	var options []connect.HandlerOption
	options = append(options, connect.WithCompression("zstd", mwcompress.NewDecompress, mwcompress.NewCompress))
	options = append(options, connect.WithCompression("gzip", nil, nil))
	options = append(options, connect.WithInterceptors(mwauth.NewAuthInterceptor(secret)))
	server := &ItemFolderRPC{
		DB:  db,
		ifs: *ifs,
		cs:  *cs,
		us:  *us,
	}

	path, handler := itemfolderv1connect.NewItemFolderServiceHandler(server, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *ItemFolderRPC) CreateFolder(ctx context.Context, req *connect.Request[itemfolderv1.CreateFolderRequest]) (*connect.Response[itemfolderv1.CreateFolderResponse], error) {
	itemID := idwrap.NewNow()
	collectionUlidID, err := idwrap.NewWithParse(req.Msg.GetCollectionId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	ok, err := collection.CheckOwnerCollection(ctx, c.cs, c.us, collectionUlidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not owner"))
	}

	// TODO: parentID
	folder := mitemfolder.ItemFolder{
		ID:           itemID,
		CollectionID: collectionUlidID,
		Name:         req.Msg.GetName(),
	}
	err = c.ifs.CreateItemFolder(ctx, &folder)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	respRaw := &itemfolderv1.CreateFolderResponse{
		Id:   itemID.String(),
		Name: req.Msg.GetName(),
	}
	resp := connect.NewResponse(respRaw)
	return resp, nil
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

	folder, err := c.ifs.GetItemFolder(ctx, ulidID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

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

// UpdateFolder calls collection.v1.CollectionService.UpdateFolder.
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
		checkfolder, err := c.ifs.GetItemFolder(ctx, parentUlidID)
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

func CheckOwnerFolder(ctx context.Context, ifs sitemfolder.ItemFolderService, cs scollection.CollectionService, us suser.UserService, folderID idwrap.IDWrap) (bool, error) {
	folder, err := ifs.GetItemFolder(ctx, folderID)
	if err != nil {
		return false, err
	}

	isOwner, err := collection.CheckOwnerCollection(ctx, cs, us, folder.CollectionID)
	if err != nil {
		return false, err
	}
	return isOwner, nil
}
