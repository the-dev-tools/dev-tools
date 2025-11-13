package rfile

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/suser"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/file_system/v1"
)

// TestFileValidation tests file validation logic
func TestFileValidation(t *testing.T) {
	t.Run("valid_file", func(t *testing.T) {
		fileID := idwrap.NewNow()
		workspaceID := idwrap.NewNow()
		contentID := idwrap.NewNow()

		file := mfile.File{
			ID:          fileID,
			WorkspaceID: workspaceID,
			ContentID:   &contentID,
			ContentType: mfile.ContentTypeHTTP,
			Name:        "test-file",
			Order:       1.0,
		}

		err := file.Validate()
		require.NoError(t, err)
	})

	t.Run("invalid_file_missing_id", func(t *testing.T) {
		workspaceID := idwrap.NewNow()
		contentID := idwrap.NewNow()

		file := mfile.File{
			ID:          idwrap.IDWrap{}, // Empty ID
			WorkspaceID: workspaceID,
			ContentID:   &contentID,
			ContentType: mfile.ContentTypeHTTP,
			Name:        "test-file",
			Order:       1.0,
		}

		err := file.Validate()
		require.Error(t, err)
		require.Contains(t, err.Error(), "file ID cannot be empty")
	})

	t.Run("invalid_file_missing_workspace", func(t *testing.T) {
		fileID := idwrap.NewNow()
		contentID := idwrap.NewNow()

		file := mfile.File{
			ID:          fileID,
			WorkspaceID: idwrap.IDWrap{}, // Empty workspace ID
			ContentID:   &contentID,
			ContentType: mfile.ContentTypeHTTP,
			Name:        "test-file",
			Order:       1.0,
		}

		err := file.Validate()
		require.Error(t, err)
		require.Contains(t, err.Error(), "workspace ID cannot be empty")
	})

	t.Run("invalid_file_missing_name", func(t *testing.T) {
		fileID := idwrap.NewNow()
		workspaceID := idwrap.NewNow()
		contentID := idwrap.NewNow()

		file := mfile.File{
			ID:          fileID,
			WorkspaceID: workspaceID,
			ContentID:   &contentID,
			ContentType: mfile.ContentTypeHTTP,
			Name:        "", // Empty name
			Order:       1.0,
		}

		err := file.Validate()
		require.Error(t, err)
		require.Contains(t, err.Error(), "file name cannot be empty")
	})
}

// TestEventTypes tests event type constants
func TestEventTypes(t *testing.T) {
	require.Equal(t, "create", eventTypeCreate)
	require.Equal(t, "update", eventTypeUpdate)
	require.Equal(t, "delete", eventTypeDelete)
}

// TestFileTopic tests the topic structure
func TestFileTopic(t *testing.T) {
	workspaceID := idwrap.NewNow()
	topic := FileTopic{WorkspaceID: workspaceID}

	require.Equal(t, workspaceID, topic.WorkspaceID)
}

// TestFileEvent tests the event structure
func TestFileEvent(t *testing.T) {
	fileID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()

	file := &apiv1.File{
		FileId:      fileID.Bytes(),
		WorkspaceId: workspaceID.Bytes(),
		Kind:        apiv1.FileKind_FILE_KIND_HTTP,
		Order:       1.0,
	}

	event := FileEvent{
		Type: eventTypeCreate,
		File: file,
	}

	require.Equal(t, eventTypeCreate, event.Type)
	require.Equal(t, file, event.File)
}

// TestCheckOwnerFile tests the ownership check function signature
func TestCheckOwnerFile(t *testing.T) {
	// Test that the function has the correct signature
	var _ func(context.Context, sfile.FileService, suser.UserService, idwrap.IDWrap) (bool, error) = CheckOwnerFile
	_ = CheckOwnerFile // Just to verify the function signature
}
