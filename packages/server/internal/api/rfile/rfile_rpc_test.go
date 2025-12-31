package rfile

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/db/pkg/dbtest"
	gen "the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/eventstream/memory"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/file_system/v1"
)

func setupTestService(t *testing.T) (*FileServiceRPC, *gen.Queries, context.Context, idwrap.IDWrap, idwrap.IDWrap) {
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	queries := gen.New(db)

	// Setup Services
	wsService := sworkspace.NewWorkspaceService(queries)
	fileService := sfile.New(queries, nil)
	userService := suser.New(queries)

	stream := memory.NewInMemorySyncStreamer[FileTopic, FileEvent]()

	handler := New(FileServiceRPCDeps{
		DB: db,
		Services: FileServiceRPCServices{
			File:      fileService,
			User:      userService,
			Workspace: wsService,
		},
		Stream: stream,
	})
	svc := &handler

	// Create User
	userID := idwrap.NewNow()
	ctx = mwauth.CreateAuthedContext(ctx, userID)

	err = queries.CreateUser(ctx, gen.CreateUserParams{
		ID:    userID,
		Email: "test@example.com",
	})
	require.NoError(t, err)

	// Create Workspace
	workspaceID := idwrap.NewNow()
	workspace := mworkspace.Workspace{
		ID:              workspaceID,
		Name:            "Test Workspace",
		Updated:         dbtime.DBNow(),
		CollectionCount: 0,
		FlowCount:       0,
	}
	err = wsService.Create(ctx, &workspace)
	require.NoError(t, err)

	// Link User to Workspace
	err = queries.CreateWorkspaceUser(ctx, gen.CreateWorkspaceUserParams{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        1,
	})
	require.NoError(t, err)

	return svc, queries, ctx, userID, workspaceID
}

func TestFileInsert(t *testing.T) {
	svc, _, ctx, _, workspaceID := setupTestService(t)

	fileID := idwrap.NewNow()
	req := connect.NewRequest(&apiv1.FileInsertRequest{
		Items: []*apiv1.FileInsert{
			{
				FileId:      fileID.Bytes(),
				WorkspaceId: workspaceID.Bytes(),
				Kind:        apiv1.FileKind_FILE_KIND_HTTP,
				Order:       1.0,
			},
		},
	})

	resp, err := svc.FileInsert(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.IsType(t, &emptypb.Empty{}, resp.Msg)

	// Verify in DB
	file, err := svc.fs.GetFile(ctx, fileID)
	require.NoError(t, err)
	assert.Equal(t, fileID, file.ID)
	assert.Equal(t, workspaceID, file.WorkspaceID)
	assert.Equal(t, mfile.ContentTypeHTTP, file.ContentType)
}

func TestFileUpdate(t *testing.T) {
	svc, _, ctx, _, workspaceID := setupTestService(t)

	// Create initial file
	fileID := idwrap.NewNow()
	err := svc.fs.CreateFile(ctx, &mfile.File{
		ID:          fileID,
		WorkspaceID: workspaceID,
		ContentType: mfile.ContentTypeHTTP,
		Order:       1.0,
		UpdatedAt:   time.Now(),
	})
	require.NoError(t, err)

	// Update the file (change order)
	newOrder := float32(2.0)
	req := connect.NewRequest(&apiv1.FileUpdateRequest{
		Items: []*apiv1.FileUpdate{
			{
				FileId: fileID.Bytes(),
				Order:  &newOrder,
			},
		},
	})

	resp, err := svc.FileUpdate(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)

	// Verify update in DB
	file, err := svc.fs.GetFile(ctx, fileID)
	require.NoError(t, err)
	assert.Equal(t, float64(newOrder), file.Order)
}

func TestFileDelete(t *testing.T) {
	svc, _, ctx, _, workspaceID := setupTestService(t)

	// Create file
	fileID := idwrap.NewNow()
	err := svc.fs.CreateFile(ctx, &mfile.File{
		ID:          fileID,
		WorkspaceID: workspaceID,
		ContentType: mfile.ContentTypeHTTP,
		Order:       1.0,
		UpdatedAt:   time.Now(),
	})
	require.NoError(t, err)

	// Delete file
	req := connect.NewRequest(&apiv1.FileDeleteRequest{
		Items: []*apiv1.FileDelete{
			{
				FileId: fileID.Bytes(),
			},
		},
	})

	resp, err := svc.FileDelete(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)

	// Verify deletion in DB
	_, err = svc.fs.GetFile(ctx, fileID)
	assert.Error(t, err)
	assert.Equal(t, sfile.ErrFileNotFound, err)
}

func TestFileCollection(t *testing.T) {
	svc, _, ctx, _, workspaceID := setupTestService(t)

	// Create multiple files
	fileID1 := idwrap.NewNow()
	fileID2 := idwrap.NewNow()

	err := svc.fs.CreateFile(ctx, &mfile.File{
		ID:          fileID1,
		WorkspaceID: workspaceID,
		ContentType: mfile.ContentTypeHTTP,
		Order:       1.0,
		UpdatedAt:   time.Now(),
	})
	require.NoError(t, err)

	err = svc.fs.CreateFile(ctx, &mfile.File{
		ID:          fileID2,
		WorkspaceID: workspaceID,
		ContentType: mfile.ContentTypeFlow,
		Order:       2.0,
		UpdatedAt:   time.Now(),
	})
	require.NoError(t, err)

	// List files
	req := connect.NewRequest(&emptypb.Empty{})
	resp, err := svc.FileCollection(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Msg.Items, 2)

	// Verify content of response
	found1 := false
	found2 := false
	for _, item := range resp.Msg.Items {
		if idwrap.NewFromBytesMust(item.FileId).Compare(fileID1) == 0 {
			found1 = true
			assert.Equal(t, apiv1.FileKind_FILE_KIND_HTTP, item.Kind)
		}
		if idwrap.NewFromBytesMust(item.FileId).Compare(fileID2) == 0 {
			found2 = true
			assert.Equal(t, apiv1.FileKind_FILE_KIND_FLOW, item.Kind)
		}
	}
	assert.True(t, found1)
	assert.True(t, found2)
}

func TestFolderInsert(t *testing.T) {
	svc, _, ctx, _, workspaceID := setupTestService(t)

	folderID := idwrap.NewNow()
	folderName := "My New Folder"
	req := connect.NewRequest(&apiv1.FolderInsertRequest{
		Items: []*apiv1.FolderInsert{
			{
				FolderId: folderID.Bytes(),
				Name:     folderName,
			},
		},
	})

	resp, err := svc.FolderInsert(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.IsType(t, &emptypb.Empty{}, resp.Msg)

	// Verify in DB
	folder, err := svc.fs.GetFile(ctx, folderID)
	require.NoError(t, err)
	assert.Equal(t, folderID, folder.ID)
	assert.Equal(t, workspaceID, folder.WorkspaceID)
	assert.Equal(t, mfile.ContentTypeFolder, folder.ContentType)
	assert.Equal(t, folderName, folder.Name)
}

func TestFolderUpdate(t *testing.T) {
	svc, _, ctx, _, workspaceID := setupTestService(t)

	// Create initial folder
	folderID := idwrap.NewNow()
	err := svc.fs.CreateFile(ctx, &mfile.File{
		ID:          folderID,
		WorkspaceID: workspaceID,
		ContentType: mfile.ContentTypeFolder,
		Name:        "Old Name",
		Order:       0,
		UpdatedAt:   time.Now(),
	})
	require.NoError(t, err)

	// Update the folder (rename)
	newName := "Updated Name"
	req := connect.NewRequest(&apiv1.FolderUpdateRequest{
		Items: []*apiv1.FolderUpdate{
			{
				FolderId: folderID.Bytes(),
				Name:     &newName,
			},
		},
	})

	resp, err := svc.FolderUpdate(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)

	// Verify update in DB
	folder, err := svc.fs.GetFile(ctx, folderID)
	require.NoError(t, err)
	assert.Equal(t, newName, folder.Name)
}

func TestFolderDelete(t *testing.T) {
	svc, _, ctx, _, workspaceID := setupTestService(t)

	// Create folder
	folderID := idwrap.NewNow()
	err := svc.fs.CreateFile(ctx, &mfile.File{
		ID:          folderID,
		WorkspaceID: workspaceID,
		ContentType: mfile.ContentTypeFolder,
		Name:        "To Delete",
		Order:       0,
		UpdatedAt:   time.Now(),
	})
	require.NoError(t, err)

	// Delete folder
	req := connect.NewRequest(&apiv1.FolderDeleteRequest{
		Items: []*apiv1.FolderDelete{
			{
				FolderId: folderID.Bytes(),
			},
		},
	})

	resp, err := svc.FolderDelete(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)

	// Verify deletion in DB
	_, err = svc.fs.GetFile(ctx, folderID)
	assert.Error(t, err)
	assert.Equal(t, sfile.ErrFileNotFound, err)
}

func TestFolderCollection(t *testing.T) {
	svc, _, ctx, _, workspaceID := setupTestService(t)

	// Create mixed files (folders and non-folders)
	folderID1 := idwrap.NewNow()
	fileID2 := idwrap.NewNow()

	// Folder
	err := svc.fs.CreateFile(ctx, &mfile.File{
		ID:          folderID1,
		WorkspaceID: workspaceID,
		ContentType: mfile.ContentTypeFolder,
		Name:        "Folder 1",
		Order:       0,
		UpdatedAt:   time.Now(),
	})
	require.NoError(t, err)

	// Regular File (should not appear in FolderCollection)
	err = svc.fs.CreateFile(ctx, &mfile.File{
		ID:          fileID2,
		WorkspaceID: workspaceID,
		ContentType: mfile.ContentTypeHTTP,
		Order:       1.0,
		UpdatedAt:   time.Now(),
	})
	require.NoError(t, err)

	// List folders
	req := connect.NewRequest(&emptypb.Empty{})
	resp, err := svc.FolderCollection(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Msg.Items, 1) // Only the folder should be returned

	// Verify content
	item := resp.Msg.Items[0]
	assert.Equal(t, folderID1.Bytes(), item.FolderId)
	assert.Equal(t, "Folder 1", item.Name)
}
