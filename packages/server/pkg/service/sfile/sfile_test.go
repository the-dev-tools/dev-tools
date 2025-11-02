package sfile

import (
	"context"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/testutil"
)

func TestFileService_CreateFile(t *testing.T) {
	ctx := context.Background()
	baseDB := testutil.CreateBaseDB(ctx, t)
	defer baseDB.DB.Close()

	service := New(baseDB.Queries, nil)

	// Create test file
	workspaceID := idwrap.New(ulid.Make())
	fileID := idwrap.New(ulid.Make())
	contentID := idwrap.New(ulid.Make())

	file := &mfile.File{
		ID:          fileID,
		WorkspaceID: workspaceID,
		ContentID:   &contentID,
		ContentKind: mfile.ContentKindAPI,
		Name:        "test-api",
		Order:       1.0,
		UpdatedAt:   time.Now(),
	}

	err := service.CreateFile(ctx, file)
	assert.NoError(t, err)

	// Verify file was created
	retrieved, err := service.GetFile(ctx, fileID)
	assert.NoError(t, err)
	assert.Equal(t, fileID, retrieved.ID)
	assert.Equal(t, workspaceID, retrieved.WorkspaceID)
	assert.Equal(t, "test-api", retrieved.Name)
	assert.Equal(t, mfile.ContentKindAPI, retrieved.ContentKind)
}

func TestFileService_ListFilesByWorkspace(t *testing.T) {
	ctx := context.Background()
	baseDB := testutil.CreateBaseDB(ctx, t)
	defer baseDB.DB.Close()

	service := New(baseDB.Queries, nil)

	// Create test workspace and files
	workspaceID := idwrap.New(ulid.Make())

	folderContentID := idwrap.NewNow()
	flowContentID := idwrap.NewNow()

	file1 := &mfile.File{
		ID:          idwrap.New(ulid.Make()),
		WorkspaceID: workspaceID,
		ContentID:   &folderContentID,
		ContentKind: mfile.ContentKindFolder,
		Name:        "folder1",
		Order:       1.0,
		UpdatedAt:   time.Now(),
	}

	file2 := &mfile.File{
		ID:          idwrap.New(ulid.Make()),
		WorkspaceID: workspaceID,
		ContentID:   &flowContentID,
		ContentKind: mfile.ContentKindFlow,
		Name:        "flow1",
		Order:       2.0,
		UpdatedAt:   time.Now(),
	}

	// Create files
	err := service.CreateFile(ctx, file1)
	assert.NoError(t, err)
	err = service.CreateFile(ctx, file2)
	assert.NoError(t, err)

	// List files
	files, err := service.ListFilesByWorkspace(ctx, workspaceID)
	assert.NoError(t, err)
	assert.Len(t, files, 2)

	// Verify order (should be sorted by display_order)
	assert.Equal(t, "folder1", files[0].Name)
	assert.Equal(t, "flow1", files[1].Name)
}

func TestFileService_GetFileWithContent(t *testing.T) {
	ctx := context.Background()
	baseDB := testutil.CreateBaseDB(ctx, t)
	defer baseDB.DB.Close()

	service := New(baseDB.Queries, nil)

	// This test would require setting up content tables
	// For now, just test that the method exists and handles missing content correctly
	fileID := idwrap.New(ulid.Make())

	// Try to get file with content that doesn't exist
	_, err := service.GetFileWithContent(ctx, fileID)
	assert.Error(t, err)
	assert.Equal(t, ErrFileNotFound, err)
}

func TestFileService_MoveFile(t *testing.T) {
	ctx := context.Background()
	baseDB := testutil.CreateBaseDB(ctx, t)
	defer baseDB.DB.Close()

	service := New(baseDB.Queries, nil)

	// Create test workspace and folder
	workspaceID := idwrap.New(ulid.Make())
	folderID := idwrap.New(ulid.Make())

	// Create folder first
	folderContentID := idwrap.NewNow()
	folder := &mfile.File{
		ID:          folderID,
		WorkspaceID: workspaceID,
		ContentID:   &folderContentID,
		ContentKind: mfile.ContentKindFolder,
		Name:        "parent-folder",
		Order:       1.0,
		UpdatedAt:   time.Now(),
	}

	err := service.CreateFile(ctx, folder)
	require.NoError(t, err)

	// Create file to move
	fileID := idwrap.New(ulid.Make())
	apiContentID := idwrap.NewNow()
	file := &mfile.File{
		ID:          fileID,
		WorkspaceID: workspaceID,
		FolderID:    nil, // Root level
		ContentID:   &apiContentID,
		ContentKind: mfile.ContentKindAPI,
		Name:        "test-api",
		Order:       2.0,
		UpdatedAt:   time.Now(),
	}

	err = service.CreateFile(ctx, file)
	require.NoError(t, err)

	// Move file into folder
	err = service.MoveFile(ctx, fileID, &folderID)
	assert.NoError(t, err)

	// Verify file was moved
	retrieved, err := service.GetFile(ctx, fileID)
	assert.NoError(t, err)
	assert.NotNil(t, retrieved.FolderID)
	assert.Equal(t, folderID, *retrieved.FolderID)
}

func TestFileService_DeleteFile(t *testing.T) {
	ctx := context.Background()
	baseDB := testutil.CreateBaseDB(ctx, t)
	defer baseDB.DB.Close()

	service := New(baseDB.Queries, nil)

	// Create test file
	workspaceID := idwrap.New(ulid.Make())
	fileID := idwrap.New(ulid.Make())

	apiContentID := idwrap.NewNow()
	file := &mfile.File{
		ID:          fileID,
		WorkspaceID: workspaceID,
		ContentID:   &apiContentID,
		ContentKind: mfile.ContentKindAPI,
		Name:        "test-api",
		Order:       1.0,
		UpdatedAt:   time.Now(),
	}

	err := service.CreateFile(ctx, file)
	require.NoError(t, err)

	// Verify file exists
	_, err = service.GetFile(ctx, fileID)
	assert.NoError(t, err)

	// Delete file
	err = service.DeleteFile(ctx, fileID)
	assert.NoError(t, err)

	// Verify file was deleted
	_, err = service.GetFile(ctx, fileID)
	assert.Error(t, err)
	assert.Equal(t, ErrFileNotFound, err)
}

func TestFileService_GetFileWithContent_Integration(t *testing.T) {
	ctx := context.Background()
	baseDB := testutil.CreateBaseDB(ctx, t)
	defer baseDB.DB.Close()

	service := New(baseDB.Queries, nil)

	// Create test workspace
	workspaceID := idwrap.NewNow()

	// For this integration test, we'll create a file with folder content
	// but we need to manually insert the folder content first since we don't
	// have a service for item_folder creation yet
	folderContentID := idwrap.NewNow()

	// Insert folder content directly using raw SQL since we don't have generated queries
	_, err := baseDB.DB.ExecContext(ctx, `
		INSERT INTO item_folder (id, collection_id, parent_id, name) 
		VALUES (?, ?, ?, ?)`,
		folderContentID.Bytes(),
		idwrap.NewNow().Bytes(), // dummy collection_id
		nil,                     // no parent
		"test-folder",
	)
	require.NoError(t, err)

	// Create file that references the folder content
	fileID := idwrap.NewNow()
	file := &mfile.File{
		ID:          fileID,
		WorkspaceID: workspaceID,
		ContentID:   &folderContentID,
		ContentKind: mfile.ContentKindFolder,
		Name:        "folder-file",
		Order:       1.0,
		UpdatedAt:   time.Now(),
	}

	err = service.CreateFile(ctx, file)
	require.NoError(t, err)

	// Test the two-query pattern: GetFileWithContent should resolve both file and content
	result, err := service.GetFileWithContent(ctx, fileID)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify file metadata
	assert.Equal(t, fileID, result.File.ID)
	assert.Equal(t, "folder-file", result.File.Name)
	assert.Equal(t, mfile.ContentKindFolder, result.File.ContentKind)
	assert.NotNil(t, result.File.ContentID)
	assert.Equal(t, folderContentID, *result.File.ContentID)

	// Verify resolved content using interface methods
	assert.NotNil(t, result.Content)

	// Check the content kind
	assert.Equal(t, mfile.ContentKindFolder, result.Content.GetKind())
	assert.Equal(t, folderContentID, result.Content.GetID())
	assert.Equal(t, "test-folder", result.Content.GetName())

	// Verify content validates properly
	assert.NoError(t, result.Content.Validate())
}
