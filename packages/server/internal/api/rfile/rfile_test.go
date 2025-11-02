package rfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/file_system/v1"
)

func TestToAPIFile(t *testing.T) {
	fileID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()
	folderID := idwrap.NewNow()
	contentID := idwrap.NewNow()

	file := mfile.File{
		ID:          fileID,
		WorkspaceID: workspaceID,
		FolderID:    &folderID,
		ContentID:   &contentID,
		ContentKind: mfile.ContentKindAPI,
		Name:        "test-file",
		Order:       1.5,
	}

	apiFile := toAPIFile(file)

	assert.Equal(t, fileID.Bytes(), apiFile.FileId)
	assert.Equal(t, workspaceID.Bytes(), apiFile.WorkspaceId)
	assert.Equal(t, folderID.Bytes(), apiFile.FolderId)
	assert.Equal(t, contentID.Bytes(), apiFile.ContentId)
	assert.Equal(t, apiv1.FileKind_FILE_KIND_HTTP, apiFile.Kind)
	assert.Equal(t, "test-file", apiFile.Name)
	assert.Equal(t, float32(1.5), apiFile.Order)
}

func TestToAPIFileKind(t *testing.T) {
	tests := []struct {
		name     string
		input    mfile.ContentKind
		expected apiv1.FileKind
	}{
		{"folder", mfile.ContentKindFolder, apiv1.FileKind_FILE_KIND_FOLDER},
		{"api", mfile.ContentKindAPI, apiv1.FileKind_FILE_KIND_HTTP},
		{"flow", mfile.ContentKindFlow, apiv1.FileKind_FILE_KIND_FLOW},
		{"unknown", mfile.ContentKindUnknown, apiv1.FileKind_FILE_KIND_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toAPIFileKind(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFromAPIFileKind(t *testing.T) {
	tests := []struct {
		name     string
		input    apiv1.FileKind
		expected mfile.ContentKind
	}{
		{"folder", apiv1.FileKind_FILE_KIND_FOLDER, mfile.ContentKindFolder},
		{"api", apiv1.FileKind_FILE_KIND_HTTP, mfile.ContentKindAPI},
		{"flow", apiv1.FileKind_FILE_KIND_FLOW, mfile.ContentKindFlow},
		{"unspecified", apiv1.FileKind_FILE_KIND_UNSPECIFIED, mfile.ContentKindUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fromAPIFileKind(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFromAPIFileCreate(t *testing.T) {
	fileID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()
	folderID := idwrap.NewNow()
	contentID := idwrap.NewNow()

	apiFile := &apiv1.FileCreate{
		FileId:      fileID.Bytes(),
		WorkspaceId: workspaceID.Bytes(),
		FolderId:    folderID.Bytes(),
		Kind:        apiv1.FileKind_FILE_KIND_HTTP,
		ContentId:   contentID.Bytes(),
		Name:        "test-file",
		Order:       1.5,
	}

	file, err := fromAPIFileCreate(apiFile)
	require.NoError(t, err)

	assert.Equal(t, fileID, file.ID)
	assert.Equal(t, workspaceID, file.WorkspaceID)
	assert.NotNil(t, file.FolderID)
	assert.Equal(t, folderID, *file.FolderID)
	assert.NotNil(t, file.ContentID)
	assert.Equal(t, contentID, *file.ContentID)
	assert.Equal(t, mfile.ContentKindAPI, file.ContentKind)
	assert.Equal(t, "test-file", file.Name)
	assert.Equal(t, float64(1.5), file.Order)
}

func TestFromAPIFileCreateWithoutOptionalFields(t *testing.T) {
	fileID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()

	apiFile := &apiv1.FileCreate{
		FileId:      fileID.Bytes(),
		WorkspaceId: workspaceID.Bytes(),
		Kind:        apiv1.FileKind_FILE_KIND_FOLDER,
		Name:        "test-folder",
		Order:       1.0,
	}

	file, err := fromAPIFileCreate(apiFile)
	require.NoError(t, err)

	assert.Equal(t, fileID, file.ID)
	assert.Equal(t, workspaceID, file.WorkspaceID)
	assert.Nil(t, file.FolderID)
	assert.Nil(t, file.ContentID)
	assert.Equal(t, mfile.ContentKindFolder, file.ContentKind)
	assert.Equal(t, "test-folder", file.Name)
	assert.Equal(t, float64(1.0), file.Order)
}

func TestFileSyncResponseFrom(t *testing.T) {
	fileID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()

	tests := []struct {
		name     string
		event    FileEvent
		expected *apiv1.FileSyncResponse
	}{
		{
			name: "create",
			event: FileEvent{
				Type: eventTypeCreate,
				File: &apiv1.File{
					FileId:      fileID.Bytes(),
					WorkspaceId: workspaceID.Bytes(),
					Kind:        apiv1.FileKind_FILE_KIND_HTTP,
					Name:        "test-file",
					Order:       1.0,
				},
			},
			expected: &apiv1.FileSyncResponse{
				Items: []*apiv1.FileSync{
					{
						Value: &apiv1.FileSync_ValueUnion{
							Kind: apiv1.FileSync_ValueUnion_KIND_CREATE,
							Create: &apiv1.FileSyncCreate{
								FileId:      fileID.Bytes(),
								WorkspaceId: workspaceID.Bytes(),
								Kind:        apiv1.FileKind_FILE_KIND_HTTP,
								Name:        "test-file",
								Order:       1.0,
							},
						},
					},
				},
			},
		},
		{
			name: "delete",
			event: FileEvent{
				Type: eventTypeDelete,
				File: &apiv1.File{
					FileId:      fileID.Bytes(),
					WorkspaceId: workspaceID.Bytes(),
				},
			},
			expected: &apiv1.FileSyncResponse{
				Items: []*apiv1.FileSync{
					{
						Value: &apiv1.FileSync_ValueUnion{
							Kind: apiv1.FileSync_ValueUnion_KIND_DELETE,
							Delete: &apiv1.FileSyncDelete{
								FileId: fileID.Bytes(),
							},
						},
					},
				},
			},
		},
		{
			name:     "nil file",
			event:    FileEvent{Type: eventTypeCreate, File: nil},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fileSyncResponseFrom(tt.event)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Len(t, result.Items, 1)
				assert.Equal(t, tt.expected.Items[0].Value.Kind, result.Items[0].Value.Kind)
			}
		})
	}
}

func TestFromAPIFileUpdate(t *testing.T) {
	fileID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()
	folderID := idwrap.NewNow()
	contentID := idwrap.NewNow()

	// Create existing file
	existingFile := &mfile.File{
		ID:          fileID,
		WorkspaceID: workspaceID,
		FolderID:    &folderID,
		ContentID:   &contentID,
		ContentKind: mfile.ContentKindAPI,
		Name:        "old-name",
		Order:       1.0,
	}

	// Test update with new name
	newName := "updated-name"
	apiFile := &apiv1.FileUpdate{
		FileId:      fileID.Bytes(),
		WorkspaceId: workspaceID.Bytes(),
		Name:        &newName,
	}

	file, err := fromAPIFileUpdate(apiFile, existingFile)
	require.NoError(t, err)

	assert.Equal(t, fileID, file.ID)
	assert.Equal(t, workspaceID, file.WorkspaceID)
	assert.Equal(t, folderID, *file.FolderID)               // Should preserve existing
	assert.Equal(t, contentID, *file.ContentID)             // Should preserve existing
	assert.Equal(t, mfile.ContentKindAPI, file.ContentKind) // Should preserve existing
	assert.Equal(t, "updated-name", file.Name)
	assert.Equal(t, float64(1.0), file.Order) // Should preserve existing
}

func TestFromAPIFileUpdateWithFolderUnion(t *testing.T) {
	fileID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()
	newFolderID := idwrap.NewNow()

	existingFile := &mfile.File{
		ID:          fileID,
		WorkspaceID: workspaceID,
		ContentKind: mfile.ContentKindAPI,
		Name:        "test-file",
		Order:       1.0,
	}

	// Test update with new folder using union
	apiFile := &apiv1.FileUpdate{
		FileId:      fileID.Bytes(),
		WorkspaceId: workspaceID.Bytes(),
		FolderId: &apiv1.FileUpdate_FolderIdUnion{
			Kind:  apiv1.FileUpdate_FolderIdUnion_KIND_BYTES,
			Bytes: newFolderID.Bytes(),
		},
	}

	file, err := fromAPIFileUpdate(apiFile, existingFile)
	require.NoError(t, err)

	assert.Equal(t, fileID, file.ID)
	assert.NotNil(t, file.FolderID)
	assert.Equal(t, newFolderID, *file.FolderID)
}

func TestFromAPIFileUpdateWithUnsetFolder(t *testing.T) {
	fileID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()
	folderID := idwrap.NewNow()

	existingFile := &mfile.File{
		ID:          fileID,
		WorkspaceID: workspaceID,
		FolderID:    &folderID,
		ContentKind: mfile.ContentKindAPI,
		Name:        "test-file",
		Order:       1.0,
	}

	// Test update with unset folder
	apiFile := &apiv1.FileUpdate{
		FileId:      fileID.Bytes(),
		WorkspaceId: workspaceID.Bytes(),
		FolderId: &apiv1.FileUpdate_FolderIdUnion{
			Kind: apiv1.FileUpdate_FolderIdUnion_KIND_UNSET,
		},
	}

	file, err := fromAPIFileUpdate(apiFile, existingFile)
	require.NoError(t, err)

	assert.Equal(t, fileID, file.ID)
	assert.Nil(t, file.FolderID) // Should be unset
}
