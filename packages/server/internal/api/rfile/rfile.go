package rfile

import (
	"context"
	"database/sql"
	"errors"
	"sync"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rworkspace"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/file_system/v1"
	"the-dev-tools/spec/dist/buf/go/api/file_system/v1/file_systemv1connect"
)

const (
	eventTypeCreate = "create"
	eventTypeUpdate = "update"
	eventTypeDelete = "delete"
)

type FileTopic struct {
	WorkspaceID idwrap.IDWrap
}

type FileEvent struct {
	Type string
	File *apiv1.File
	Name string
}

type FileServiceRPC struct {
	DB *sql.DB

	fs *sfile.FileService
	us suser.UserService
	ws sworkspace.WorkspaceService

	stream eventstream.SyncStreamer[FileTopic, FileEvent]
}

func New(
	db *sql.DB,
	fs *sfile.FileService,
	us suser.UserService,
	ws sworkspace.WorkspaceService,
	stream eventstream.SyncStreamer[FileTopic, FileEvent],
) FileServiceRPC {
	return FileServiceRPC{
		DB:     db,
		fs:     fs,
		us:     us,
		ws:     ws,
		stream: stream,
	}
}

func CreateService(srv FileServiceRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := file_systemv1connect.NewFileSystemServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

// Helper functions for pointer conversion
func stringPtr(s string) *string    { return &s }
func float32Ptr(f float32) *float32 { return &f }

// Convert model File to API File
func toAPIFile(file mfile.File) *apiv1.File {
	apiFile := &apiv1.File{
		FileId:      file.ID.Bytes(),
		WorkspaceId: file.WorkspaceID.Bytes(),
		Order:       float32(file.Order),
		Kind:        toAPIFileKind(file.ContentType),
	}

	if file.FolderID != nil {
		apiFile.ParentId = file.FolderID.Bytes()
	}

	return apiFile
}

// Convert model ContentType to API FileKind
func toAPIFileKind(kind mfile.ContentType) apiv1.FileKind {
	switch kind {
	case mfile.ContentTypeFolder:
		return apiv1.FileKind_FILE_KIND_FOLDER
	case mfile.ContentTypeHTTP:
		return apiv1.FileKind_FILE_KIND_HTTP
	case mfile.ContentTypeHTTPDelta:
		return apiv1.FileKind_FILE_KIND_HTTP_DELTA
	case mfile.ContentTypeFlow:
		return apiv1.FileKind_FILE_KIND_FLOW
	default:
		return apiv1.FileKind_FILE_KIND_UNSPECIFIED
	}
}

// Convert API FileKind to model ContentType
func fromAPIFileKind(kind apiv1.FileKind) mfile.ContentType {
	switch kind {
	case apiv1.FileKind_FILE_KIND_FOLDER:
		return mfile.ContentTypeFolder
	case apiv1.FileKind_FILE_KIND_HTTP:
		return mfile.ContentTypeHTTP
	case apiv1.FileKind_FILE_KIND_HTTP_DELTA:
		return mfile.ContentTypeHTTPDelta
	case apiv1.FileKind_FILE_KIND_FLOW:
		return mfile.ContentTypeFlow
	default:
		return mfile.ContentTypeUnknown
	}
}

// Convert API FileInsert to model File
func fromAPIFileInsert(apiFile *apiv1.FileInsert) (*mfile.File, error) {
	fileID, err := idwrap.NewFromBytes(apiFile.FileId)
	if err != nil {
		return nil, err
	}

	workspaceID, err := idwrap.NewFromBytes(apiFile.WorkspaceId)
	if err != nil {
		return nil, err
	}

	var folderID *idwrap.IDWrap
	if len(apiFile.ParentId) > 0 {
		fid, err := idwrap.NewFromBytes(apiFile.ParentId)
		if err != nil {
			return nil, err
		}
		folderID = &fid
	}

	return &mfile.File{
		ID:          fileID,
		WorkspaceID: workspaceID,
		FolderID:    folderID,
		ContentType: fromAPIFileKind(apiFile.Kind),
		Name:        "", // API doesn't have name field, will be set based on kind
		Order:       float64(apiFile.Order),
	}, nil
}

// Convert API FileUpdate to model File
func fromAPIFileUpdate(apiFile *apiv1.FileUpdate, existingFile *mfile.File) (*mfile.File, error) {
	fileID, err := idwrap.NewFromBytes(apiFile.FileId)
	if err != nil {
		return nil, err
	}

	// Start with existing file
	file := *existingFile
	file.ID = fileID

	// Update optional fields
	if apiFile.WorkspaceId != nil {
		workspaceID, err := idwrap.NewFromBytes(apiFile.WorkspaceId)
		if err != nil {
			return nil, err
		}
		file.WorkspaceID = workspaceID
	}

	if apiFile.ParentId != nil {
		if apiFile.ParentId.Kind == apiv1.FileUpdate_ParentIdUnion_KIND_VALUE && len(apiFile.ParentId.Value) > 0 {
			folderID, err := idwrap.NewFromBytes(apiFile.ParentId.Value)
			if err != nil {
				return nil, err
			}
			file.FolderID = &folderID
		} else {
			file.FolderID = nil
		}
	}

	if apiFile.Kind != nil {
		file.ContentType = fromAPIFileKind(*apiFile.Kind)
	}

	if apiFile.Order != nil {
		file.Order = float64(*apiFile.Order)
	}

	return &file, nil
}

// Folder conversion functions
// Convert model File (with ContentTypeFolder) to API Folder
func toAPIFolder(file mfile.File) *apiv1.Folder {
	return &apiv1.Folder{
		FolderId: file.ID.Bytes(),
		Name:     file.Name,
	}
}

// Convert API FolderInsert to model File (with ContentTypeFolder)
func fromAPIFolderInsert(apiFolder *apiv1.FolderInsert, workspaceID idwrap.IDWrap) (*mfile.File, error) {
	folderID, err := idwrap.NewFromBytes(apiFolder.FolderId)
	if err != nil {
		return nil, err
	}

	return &mfile.File{
		ID:          folderID,
		WorkspaceID: workspaceID,
		ContentType: mfile.ContentTypeFolder,
		Name:        apiFolder.Name,
		Order:       0, // Folders have default order
	}, nil
}

// Convert API FolderUpdate to model File (with ContentTypeFolder)
func fromAPIFolderUpdate(apiFolder *apiv1.FolderUpdate, existingFile *mfile.File) (*mfile.File, error) {
	folderID, err := idwrap.NewFromBytes(apiFolder.FolderId)
	if err != nil {
		return nil, err
	}

	// Start with existing file
	file := *existingFile
	file.ID = folderID

	if apiFolder.Name != nil {
		file.Name = *apiFolder.Name
	}

	return &file, nil
}

// Generate folder sync response from event
func folderSyncResponseFrom(evt FileEvent) *apiv1.FolderSyncResponse {
	if evt.File == nil {
		return nil
	}

	// We need to extract the folder data from the File model
	// Since the API File doesn't have Name, we'll need to reconstruct from the model

	switch evt.Type {
	case eventTypeCreate:
		msg := &apiv1.FolderSync{
			Value: &apiv1.FolderSync_ValueUnion{
				Kind: apiv1.FolderSync_ValueUnion_KIND_INSERT,
				Insert: &apiv1.FolderSyncInsert{
					FolderId: evt.File.FileId,
					Name:     evt.Name,
				},
			},
		}
		return &apiv1.FolderSyncResponse{Items: []*apiv1.FolderSync{msg}}
	case eventTypeUpdate:
		update := &apiv1.FolderSyncUpdate{
			FolderId: evt.File.FileId,
		}

		if evt.Name != "" {
			update.Name = &evt.Name
		}

		msg := &apiv1.FolderSync{
			Value: &apiv1.FolderSync_ValueUnion{
				Kind:   apiv1.FolderSync_ValueUnion_KIND_UPDATE,
				Update: update,
			},
		}
		return &apiv1.FolderSyncResponse{Items: []*apiv1.FolderSync{msg}}
	case eventTypeDelete:
		msg := &apiv1.FolderSync{
			Value: &apiv1.FolderSync_ValueUnion{
				Kind: apiv1.FolderSync_ValueUnion_KIND_DELETE,
				Delete: &apiv1.FolderSyncDelete{
					FolderId: evt.File.FileId,
				},
			},
		}
		return &apiv1.FolderSyncResponse{Items: []*apiv1.FolderSync{msg}}
	default:
		return nil
	}
}

// Generate sync response from event
func fileSyncResponseFrom(evt FileEvent) *apiv1.FileSyncResponse {
	if evt.File == nil {
		return nil
	}

	switch evt.Type {
	case eventTypeCreate:
		msg := &apiv1.FileSync{
			Value: &apiv1.FileSync_ValueUnion{
				Kind: apiv1.FileSync_ValueUnion_KIND_INSERT,
				Insert: &apiv1.FileSyncInsert{
					FileId:      evt.File.FileId,
					WorkspaceId: evt.File.WorkspaceId,
					ParentId:    evt.File.ParentId,
					Kind:        evt.File.Kind,
					Order:       evt.File.Order,
				},
			},
		}
		return &apiv1.FileSyncResponse{Items: []*apiv1.FileSync{msg}}
	case eventTypeUpdate:
		update := &apiv1.FileSyncUpdate{
			FileId: evt.File.FileId,
			Order:  float32Ptr(evt.File.Order),
		}

		if evt.File.WorkspaceId != nil {
			update.WorkspaceId = evt.File.WorkspaceId
		}

		if len(evt.File.ParentId) > 0 {
			update.ParentId = &apiv1.FileSyncUpdate_ParentIdUnion{
				Kind:  apiv1.FileSyncUpdate_ParentIdUnion_KIND_VALUE,
				Value: evt.File.ParentId,
			}
		} else {
			update.ParentId = &apiv1.FileSyncUpdate_ParentIdUnion{
				Kind: apiv1.FileSyncUpdate_ParentIdUnion_KIND_UNSET,
			}
		}

		if evt.File.Kind != apiv1.FileKind_FILE_KIND_UNSPECIFIED {
			update.Kind = &evt.File.Kind
		}

		msg := &apiv1.FileSync{
			Value: &apiv1.FileSync_ValueUnion{
				Kind:   apiv1.FileSync_ValueUnion_KIND_UPDATE,
				Update: update,
			},
		}
		return &apiv1.FileSyncResponse{Items: []*apiv1.FileSync{msg}}
	case eventTypeDelete:
		msg := &apiv1.FileSync{
			Value: &apiv1.FileSync_ValueUnion{
				Kind: apiv1.FileSync_ValueUnion_KIND_DELETE,
				Delete: &apiv1.FileSyncDelete{
					FileId: evt.File.FileId,
				},
			},
		}
		return &apiv1.FileSyncResponse{Items: []*apiv1.FileSync{msg}}
	default:
		return nil
	}
}

// listUserFiles returns all files the user has access to
func (f *FileServiceRPC) listUserFiles(ctx context.Context) ([]mfile.File, error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, err
	}

	workspaces, err := f.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		if errors.Is(err, sworkspace.ErrNoWorkspaceFound) {
			return []mfile.File{}, nil
		}
		return nil, err
	}

	var allFiles []mfile.File
	for _, workspace := range workspaces {
		files, err := f.fs.ListFilesByWorkspace(ctx, workspace.ID)
		if err != nil {
			if errors.Is(err, sfile.ErrFileNotFound) {
				continue
			}
			return nil, err
		}
		allFiles = append(allFiles, files...)
	}
	return allFiles, nil
}

// listUserFolders returns all folders (files with ContentTypeFolder) the user has access to
func (f *FileServiceRPC) listUserFolders(ctx context.Context) ([]mfile.File, error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, err
	}

	workspaces, err := f.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		if errors.Is(err, sworkspace.ErrNoWorkspaceFound) {
			return []mfile.File{}, nil
		}
		return nil, err
	}

	var allFolders []mfile.File
	for _, workspace := range workspaces {
		files, err := f.fs.ListFilesByWorkspace(ctx, workspace.ID)
		if err != nil {
			if errors.Is(err, sfile.ErrFileNotFound) {
				continue
			}
			return nil, err
		}
		// Filter only folders
		for _, file := range files {
			if file.ContentType == mfile.ContentTypeFolder {
				allFolders = append(allFolders, file)
			}
		}
	}
	return allFolders, nil
}

// FileCollection returns all files the user has access to (TanStack pattern)
func (f *FileServiceRPC) FileCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.FileCollectionResponse], error) {
	files, err := f.listUserFiles(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	items := make([]*apiv1.File, 0, len(files))
	for _, file := range files {
		items = append(items, toAPIFile(file))
	}

	return connect.NewResponse(&apiv1.FileCollectionResponse{Items: items}), nil
}

// FileInsert creates new files with batch operations
func (f *FileServiceRPC) FileInsert(ctx context.Context, req *connect.Request[apiv1.FileInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one file must be provided"))
	}

	// Step 1: Process request data and create file models OUTSIDE transaction
	var fileModels []*mfile.File
	for _, fileInsert := range req.Msg.Items {
		// Convert API to model
		file, err := fromAPIFileInsert(fileInsert)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Set default name for folders since API doesn't include it
		if file.ContentType == mfile.ContentTypeFolder && file.Name == "" {
			file.Name = "New Folder"
		}

		fileModels = append(fileModels, file)
	}

	// Step 2: Check permissions for all files OUTSIDE transaction
	for _, file := range fileModels {
		// Check workspace permissions
		rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, f.us, file.WorkspaceID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		// Validate file
		if err := file.Validate(); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
	}

	// Step 3: Minimal write transaction for fast inserts only
	tx, err := f.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	fileService := f.fs.TX(tx)
	var createdFiles []mfile.File

	// Fast inserts inside minimal transaction
	for _, file := range fileModels {
		// Create file
		if err := fileService.CreateFile(ctx, file); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		createdFiles = append(createdFiles, *file)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish events for real-time sync
	for _, file := range createdFiles {
		f.stream.Publish(FileTopic{WorkspaceID: file.WorkspaceID}, FileEvent{
			Type: eventTypeCreate,
			File: toAPIFile(file),
			Name: file.Name,
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// FileUpdate updates existing files
func (f *FileServiceRPC) FileUpdate(ctx context.Context, req *connect.Request[apiv1.FileUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one file must be provided"))
	}

	var updatedFiles []*mfile.File

	// Step 1: Validate and check permissions OUTSIDE transaction
	for _, fileUpdate := range req.Msg.Items {
		fileID, err := idwrap.NewFromBytes(fileUpdate.FileId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing file using read-only service (no transaction)
		existingFile, err := f.fs.GetFile(ctx, fileID)
		if err != nil {
			if errors.Is(err, sfile.ErrFileNotFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check workspace permissions
		rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, f.us, existingFile.WorkspaceID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		// Convert API to model
		file, err := fromAPIFileUpdate(fileUpdate, existingFile)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Validate file
		if err := file.Validate(); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		updatedFiles = append(updatedFiles, file)
	}

	// Step 2: Minimal write transaction
	tx, err := f.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	fileService := f.fs.TX(tx)
	var successFiles []mfile.File

	for _, file := range updatedFiles {
		// Update file
		if err := fileService.UpdateFile(ctx, file); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		successFiles = append(successFiles, *file)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish events for real-time sync
	for _, file := range successFiles {
		f.stream.Publish(FileTopic{WorkspaceID: file.WorkspaceID}, FileEvent{
			Type: eventTypeUpdate,
			File: toAPIFile(file),
			Name: file.Name,
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// FileDelete deletes files
func (f *FileServiceRPC) FileDelete(ctx context.Context, req *connect.Request[apiv1.FileDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one file must be provided"))
	}

	var filesToDelete []*mfile.File

	// Step 1: Validate and check permissions OUTSIDE transaction
	for _, fileDelete := range req.Msg.Items {
		fileID, err := idwrap.NewFromBytes(fileDelete.FileId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing file for workspace permission check and event publishing
		existingFile, err := f.fs.GetFile(ctx, fileID)
		if err != nil {
			if errors.Is(err, sfile.ErrFileNotFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check workspace permissions
		rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, f.us, existingFile.WorkspaceID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		filesToDelete = append(filesToDelete, existingFile)
	}

	// Step 2: Minimal write transaction
	tx, err := f.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	fileService := f.fs.TX(tx)
	var deletedFiles []mfile.File

	for _, file := range filesToDelete {
		// Delete file
		if err := fileService.DeleteFile(ctx, file.ID); err != nil {
			if errors.Is(err, sfile.ErrFileNotFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedFiles = append(deletedFiles, *file)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish events for real-time sync
	for _, file := range deletedFiles {
		f.stream.Publish(FileTopic{WorkspaceID: file.WorkspaceID}, FileEvent{
			Type: eventTypeDelete,
			File: &apiv1.File{
				FileId:      file.ID.Bytes(),
				WorkspaceId: file.WorkspaceID.Bytes(),
			},
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// FileSync handles real-time synchronization for files
func (f *FileServiceRPC) FileSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.FileSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return f.streamFileSync(ctx, userID, stream.Send)
}

func (f *FileServiceRPC) streamFileSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.FileSyncResponse) error) error {
	var workspaceSet sync.Map

	filter := func(topic FileTopic) bool {
		if _, ok := workspaceSet.Load(topic.WorkspaceID.String()); ok {
			return true
		}
		belongs, err := f.us.CheckUserBelongsToWorkspace(ctx, userID, topic.WorkspaceID)
		if err != nil || !belongs {
			return false
		}
		workspaceSet.Store(topic.WorkspaceID.String(), struct{}{})
		return true
	}

	events, err := f.stream.Subscribe(ctx, filter)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp := fileSyncResponseFrom(evt.Payload)
			if resp == nil {
				continue
			}
			if err := send(resp); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// FolderCollection returns all folders the user has access to (TanStack pattern)
func (f *FileServiceRPC) FolderCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.FolderCollectionResponse], error) {
	folders, err := f.listUserFolders(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	items := make([]*apiv1.Folder, 0, len(folders))
	for _, folder := range folders {
		items = append(items, toAPIFolder(folder))
	}

	return connect.NewResponse(&apiv1.FolderCollectionResponse{Items: items}), nil
}

// FolderInsert creates new folders with batch operations
func (f *FileServiceRPC) FolderInsert(ctx context.Context, req *connect.Request[apiv1.FolderInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one folder must be provided"))
	}

	// Step 1: Get user's default workspace for folder creation since API doesn't include workspace
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	workspaces, err := f.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil || len(workspaces) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("user has no workspaces"))
	}
	defaultWorkspace := workspaces[0] // Use first workspace as default

	// Step 2: Check workspace permissions OUTSIDE transaction
	rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, f.us, defaultWorkspace.ID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Step 3: Process request data and create folder models OUTSIDE transaction
	var folderModels []*mfile.File
	for _, folderInsert := range req.Msg.Items {
		// Convert API to model
		folder, err := fromAPIFolderInsert(folderInsert, defaultWorkspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Validate folder
		if err := folder.Validate(); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		folderModels = append(folderModels, folder)
	}

	// Step 4: Minimal write transaction for fast inserts only
	tx, err := f.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	fileService := f.fs.TX(tx)
	var createdFolders []mfile.File

	// Fast inserts inside minimal transaction
	for _, folder := range folderModels {
		// Create folder
		if err := fileService.CreateFile(ctx, folder); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		createdFolders = append(createdFolders, *folder)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish events for real-time sync
	for _, folder := range createdFolders {
		f.stream.Publish(FileTopic{WorkspaceID: folder.WorkspaceID}, FileEvent{
			Type: eventTypeCreate,
			File: toAPIFile(folder),
			Name: folder.Name,
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// FolderUpdate updates existing folders
func (f *FileServiceRPC) FolderUpdate(ctx context.Context, req *connect.Request[apiv1.FolderUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one folder must be provided"))
	}

	var updatedFolders []*mfile.File

	// Step 1: Validate and check permissions OUTSIDE transaction
	for _, folderUpdate := range req.Msg.Items {
		folderID, err := idwrap.NewFromBytes(folderUpdate.FolderId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing folder (read-only, no TX)
		existingFolder, err := f.fs.GetFile(ctx, folderID)
		if err != nil {
			if errors.Is(err, sfile.ErrFileNotFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify it's actually a folder
		if existingFolder.ContentType != mfile.ContentTypeFolder {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("not a folder"))
		}

		// Check workspace permissions
		rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, f.us, existingFolder.WorkspaceID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		// Convert API to model
		folder, err := fromAPIFolderUpdate(folderUpdate, existingFolder)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Validate folder
		if err := folder.Validate(); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		updatedFolders = append(updatedFolders, folder)
	}

	// Step 2: Minimal write transaction
	tx, err := f.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	fileService := f.fs.TX(tx)
	var successFolders []mfile.File

	for _, folder := range updatedFolders {
		// Update folder
		if err := fileService.UpdateFile(ctx, folder); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		successFolders = append(successFolders, *folder)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish events for real-time sync
	for _, folder := range successFolders {
		f.stream.Publish(FileTopic{WorkspaceID: folder.WorkspaceID}, FileEvent{
			Type: eventTypeUpdate,
			File: toAPIFile(folder),
			Name: folder.Name,
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// FolderDelete deletes folders
func (f *FileServiceRPC) FolderDelete(ctx context.Context, req *connect.Request[apiv1.FolderDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one folder must be provided"))
	}

	var foldersToDelete []*mfile.File

	// Step 1: Validate and check permissions OUTSIDE transaction
	for _, folderDelete := range req.Msg.Items {
		folderID, err := idwrap.NewFromBytes(folderDelete.FolderId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing folder for workspace permission check and event publishing
		existingFolder, err := f.fs.GetFile(ctx, folderID)
		if err != nil {
			if errors.Is(err, sfile.ErrFileNotFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify it's actually a folder
		if existingFolder.ContentType != mfile.ContentTypeFolder {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("not a folder"))
		}

		// Check workspace permissions
		rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, f.us, existingFolder.WorkspaceID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		foldersToDelete = append(foldersToDelete, existingFolder)
	}

	// Step 2: Minimal write transaction
	tx, err := f.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	fileService := f.fs.TX(tx)
	var deletedFolders []mfile.File

	for _, folder := range foldersToDelete {
		// Delete folder
		if err := fileService.DeleteFile(ctx, folder.ID); err != nil {
			if errors.Is(err, sfile.ErrFileNotFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deletedFolders = append(deletedFolders, *folder)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish events for real-time sync
	for _, folder := range deletedFolders {
		f.stream.Publish(FileTopic{WorkspaceID: folder.WorkspaceID}, FileEvent{
			Type: eventTypeDelete,
			File: &apiv1.File{
				FileId:      folder.ID.Bytes(),
				WorkspaceId: folder.WorkspaceID.Bytes(),
			},
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// FolderSync handles real-time synchronization for folders
func (f *FileServiceRPC) FolderSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.FolderSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return f.streamFolderSync(ctx, userID, stream.Send)
}

func (f *FileServiceRPC) streamFolderSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.FolderSyncResponse) error) error {
	var workspaceSet sync.Map

	filter := func(topic FileTopic) bool {
		if _, ok := workspaceSet.Load(topic.WorkspaceID.String()); ok {
			return true
		}
		belongs, err := f.us.CheckUserBelongsToWorkspace(ctx, userID, topic.WorkspaceID)
		if err != nil || !belongs {
			return false
		}
		workspaceSet.Store(topic.WorkspaceID.String(), struct{}{})
		return true
	}

	events, err := f.stream.Subscribe(ctx, filter)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			// Filter only folder events
			if evt.Payload.File != nil && evt.Payload.File.Kind == apiv1.FileKind_FILE_KIND_FOLDER {
				resp := folderSyncResponseFrom(evt.Payload)
				if resp == nil {
					continue
				}
				if err := send(resp); err != nil {
					return err
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// CheckOwnerFile verifies if a user owns a file via workspace membership
func CheckOwnerFile(ctx context.Context, fs sfile.FileService, us suser.UserService, fileID idwrap.IDWrap) (bool, error) {
	workspaceID, err := fs.GetWorkspaceID(ctx, fileID)
	if err != nil {
		return false, err
	}
	return rworkspace.CheckOwnerWorkspace(ctx, us, workspaceID)
}
