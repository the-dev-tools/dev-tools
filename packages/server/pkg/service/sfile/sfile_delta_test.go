package sfile

import (
	"context"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/testutil"
)

func TestFileService_CreateFile_Delta(t *testing.T) {
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
		ContentType: mfile.ContentTypeHTTPDelta, // This corresponds to kind 3
		Name:        "test-api-draft",
		Order:       1.0,
		UpdatedAt:   time.Now(),
	}

	// This should succeed if the CHECK constraint has been updated to allow kind 3
	err := service.CreateFile(ctx, file)
	assert.NoError(t, err)

	// Verify file was created
	retrieved, err := service.GetFile(ctx, fileID)
	assert.NoError(t, err)
	assert.Equal(t, fileID, retrieved.ID)
	assert.Equal(t, workspaceID, retrieved.WorkspaceID)
	assert.Equal(t, "test-api-draft", retrieved.Name)
	assert.Equal(t, mfile.ContentTypeHTTPDelta, retrieved.ContentType)
}
