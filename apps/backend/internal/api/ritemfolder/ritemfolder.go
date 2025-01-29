package ritemfolder

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/backend/internal/api"
	"the-dev-tools/backend/internal/api/collection"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/permcheck"
	"the-dev-tools/backend/pkg/service/scollection"
	"the-dev-tools/backend/pkg/service/sitemfolder"
	"the-dev-tools/backend/pkg/service/suser"
	"the-dev-tools/backend/pkg/translate/tfolder"
	changev1 "the-dev-tools/spec/dist/buf/go/change/v1"
	folderv1 "the-dev-tools/spec/dist/buf/go/collection/item/folder/v1"
	"the-dev-tools/spec/dist/buf/go/collection/item/folder/v1/folderv1connect"
	itemv1 "the-dev-tools/spec/dist/buf/go/collection/item/v1"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/anypb"
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

	ID := idwrap.NewNow()
	reqFolder.ID = ID

	rpcErr := permcheck.CheckPerm(collection.CheckOwnerCollection(ctx, c.cs, c.us, reqFolder.CollectionID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	if reqFolder.ParentID != nil {
		rpcErr = permcheck.CheckPerm(CheckOwnerFolder(ctx, c.ifs, c.cs, c.us, *reqFolder.ParentID))
		if rpcErr != nil {
			return nil, rpcErr
		}
	}

	err = c.ifs.CreateItemFolder(ctx, reqFolder)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// INFO: this part added with new normalisation stuff
	// should be removed after spec api change to auto do this
	folderChange := itemv1.CollectionItem{
		Kind: itemv1.ItemKind_ITEM_KIND_FOLDER,
		Folder: &folderv1.FolderListItem{
			FolderId:       ID.Bytes(),
			ParentFolderId: req.Msg.ParentFolderId,
			Name:           reqFolder.Name,
		},
	}

	folderChangeAny, err := anypb.New(&folderChange)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	a := &itemv1.CollectionItemListResponse{
		CollectionId: collectionID.Bytes(),
		FolderId:     req.Msg.ParentFolderId,
	}

	changeAny, err := anypb.New(a)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	changeKind := changev1.ListChangeKind_LIST_CHANGE_KIND_APPEND

	listChanges := []*changev1.ListChange{
		{
			Kind:   changeKind,
			Parent: changeAny,
		},
	}

	kind := changev1.ChangeKind_CHANGE_KIND_UNSPECIFIED
	change := &changev1.Change{
		Kind: &kind,
		List: listChanges,
		Data: folderChangeAny,
	}

	changes := []*changev1.Change{
		change,
	}

	respRaw := &folderv1.FolderCreateResponse{
		FolderId: ID.Bytes(),
		Changes:  changes,
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
