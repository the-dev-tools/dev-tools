package sfile

import (
	"context"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/testutil"
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
		ContentType: mfile.ContentTypeHTTP,
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
	assert.Equal(t, mfile.ContentTypeHTTP, retrieved.ContentType)
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
		ContentType: mfile.ContentTypeFolder,
		Name:        "folder1",
		Order:       1.0,
		UpdatedAt:   time.Now(),
	}

	file2 := &mfile.File{
		ID:          idwrap.New(ulid.Make()),
		WorkspaceID: workspaceID,
		ContentID:   &flowContentID,
		ContentType: mfile.ContentTypeFlow,
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
		ContentType: mfile.ContentTypeFolder,
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
		ParentID:    nil, // Root level
		ContentID:   &apiContentID,
		ContentType: mfile.ContentTypeHTTP,
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
	assert.NotNil(t, retrieved.ParentID)
	assert.Equal(t, folderID, *retrieved.ParentID)
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
		ContentType: mfile.ContentTypeHTTP,
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
