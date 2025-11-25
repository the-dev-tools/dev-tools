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
		ContentType: mfile.ContentTypeHTTP,
		Name:        "test-file",
		Order:       1.5,
	}

	apiFile := toAPIFile(file)

	assert.Equal(t, fileID.Bytes(), apiFile.FileId)
	assert.Equal(t, workspaceID.Bytes(), apiFile.WorkspaceId)
	assert.Equal(t, folderID.Bytes(), apiFile.ParentFolderId)
	assert.Equal(t, apiv1.FileKind_FILE_KIND_HTTP, apiFile.Kind)
	assert.Equal(t, float32(1.5), apiFile.Order)
}

func TestToAPIFileKind(t *testing.T) {
	tests := []struct {
		name     string
		input    mfile.ContentType
		expected apiv1.FileKind
	}{
		{"folder", mfile.ContentTypeFolder, apiv1.FileKind_FILE_KIND_FOLDER},
		{"http", mfile.ContentTypeHTTP, apiv1.FileKind_FILE_KIND_HTTP},
		{"flow", mfile.ContentTypeFlow, apiv1.FileKind_FILE_KIND_FLOW},
		{"unknown", mfile.ContentTypeUnknown, apiv1.FileKind_FILE_KIND_UNSPECIFIED},
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
		expected mfile.ContentType
	}{
		{"folder", apiv1.FileKind_FILE_KIND_FOLDER, mfile.ContentTypeFolder},
		{"http", apiv1.FileKind_FILE_KIND_HTTP, mfile.ContentTypeHTTP},
		{"flow", apiv1.FileKind_FILE_KIND_FLOW, mfile.ContentTypeFlow},
		{"unspecified", apiv1.FileKind_FILE_KIND_UNSPECIFIED, mfile.ContentTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fromAPIFileKind(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFromAPIFileInsert(t *testing.T) {
	fileID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()
	folderID := idwrap.NewNow()

	apiFile := &apiv1.FileInsert{
		FileId:         fileID.Bytes(),
		WorkspaceId:    workspaceID.Bytes(),
		ParentFolderId: folderID.Bytes(),
		Kind:           apiv1.FileKind_FILE_KIND_HTTP,
		Order:          1.5,
	}

	file, err := fromAPIFileInsert(apiFile)
	require.NoError(t, err)

	assert.Equal(t, fileID, file.ID)
	assert.Equal(t, workspaceID, file.WorkspaceID)
	assert.NotNil(t, file.FolderID)
	assert.Equal(t, folderID, *file.FolderID)
	assert.Equal(t, mfile.ContentTypeHTTP, file.ContentType)
	assert.Equal(t, float64(1.5), file.Order)
}

func TestFromAPIFileInsertWithoutOptionalFields(t *testing.T) {
	fileID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()

	apiFile := &apiv1.FileInsert{
		FileId:      fileID.Bytes(),
		WorkspaceId: workspaceID.Bytes(),
		Kind:        apiv1.FileKind_FILE_KIND_FOLDER,
		Order:       1.0,
	}

	file, err := fromAPIFileInsert(apiFile)
	require.NoError(t, err)

	assert.Equal(t, fileID, file.ID)
	assert.Equal(t, workspaceID, file.WorkspaceID)
	assert.Nil(t, file.FolderID)
	assert.Equal(t, mfile.ContentTypeFolder, file.ContentType)
	assert.Equal(t, "", file.Name) // API doesn't provide name, will be set later
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
					Order:       1.0,
				},
			},
			expected: &apiv1.FileSyncResponse{
				Items: []*apiv1.FileSync{
					{
						Value: &apiv1.FileSync_ValueUnion{
							Kind: apiv1.FileSync_ValueUnion_KIND_INSERT,
							Insert: &apiv1.FileSyncInsert{
								FileId:         fileID.Bytes(),
								WorkspaceId:    workspaceID.Bytes(),
								Kind:           apiv1.FileKind_FILE_KIND_HTTP,
								Order:          1.0,
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
		ContentType: mfile.ContentTypeHTTP,
		Name:        "old-name",
		Order:       1.0,
	}

	// Test update with new order only
	newOrder := float32(2.5)
	apiFile := &apiv1.FileUpdate{
		FileId:      fileID.Bytes(),
		WorkspaceId: workspaceID.Bytes(),
		Order:       &newOrder,
	}

	file, err := fromAPIFileUpdate(apiFile, existingFile)
	require.NoError(t, err)

	assert.Equal(t, fileID, file.ID)
	assert.Equal(t, workspaceID, file.WorkspaceID)
	assert.Equal(t, folderID, *file.FolderID)               // Should preserve existing
	assert.Equal(t, contentID, *file.ContentID)             // Should preserve existing
	assert.Equal(t, mfile.ContentTypeHTTP, file.ContentType) // Should preserve existing
	assert.Equal(t, "old-name", file.Name)                  // Should preserve existing
	assert.Equal(t, float64(2.5), file.Order)               // Should update order
}

func TestFromAPIFileUpdateWithFolderUnion(t *testing.T) {
	fileID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()
	newFolderID := idwrap.NewNow()

	existingFile := &mfile.File{
		ID:          fileID,
		WorkspaceID: workspaceID,
		ContentType: mfile.ContentTypeHTTP,
		Name:        "test-file",
		Order:       1.0,
	}

	// Test update with new folder using union
	apiFile := &apiv1.FileUpdate{
		FileId:      fileID.Bytes(),
		WorkspaceId: workspaceID.Bytes(),
		ParentFolderId: &apiv1.FileUpdate_ParentFolderIdUnion{
			Kind:  apiv1.FileUpdate_ParentFolderIdUnion_KIND_VALUE,
			Value: newFolderID.Bytes(),
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
		ContentType: mfile.ContentTypeHTTP,
		Name:        "test-file",
		Order:       1.0,
	}

	// Test update with unset folder
	apiFile := &apiv1.FileUpdate{
		FileId:      fileID.Bytes(),
		WorkspaceId: workspaceID.Bytes(),
		ParentFolderId: &apiv1.FileUpdate_ParentFolderIdUnion{
			Kind: apiv1.FileUpdate_ParentFolderIdUnion_KIND_UNSET,
		},
	}

	file, err := fromAPIFileUpdate(apiFile, existingFile)
	require.NoError(t, err)

	assert.Equal(t, fileID, file.ID)
	assert.Nil(t, file.FolderID) // Should be unset
}

// TestToAPIFolder tests conversion from model File (ContentTypeFolder) to API Folder
func TestToAPIFolder(t *testing.T) {
	folderID := idwrap.NewNow()

	file := mfile.File{
		ID:          folderID,
		WorkspaceID: idwrap.NewNow(),
		ContentType: mfile.ContentTypeFolder,
		Name:        "My Folder",
		Order:       1.0,
	}

	apiFolder := toAPIFolder(file)

	assert.Equal(t, folderID.Bytes(), apiFolder.FolderId)
	assert.Equal(t, "My Folder", apiFolder.Name)
}

// TestFromAPIFolderInsert tests conversion from API FolderInsert to model File
func TestFromAPIFolderInsert(t *testing.T) {
	folderID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()

	apiFolder := &apiv1.FolderInsert{
		FolderId: folderID.Bytes(),
		Name:     "New Folder",
	}

	file, err := fromAPIFolderInsert(apiFolder, workspaceID)
	require.NoError(t, err)

	assert.Equal(t, folderID, file.ID)
	assert.Equal(t, workspaceID, file.WorkspaceID)
	assert.Equal(t, mfile.ContentTypeFolder, file.ContentType)
	assert.Equal(t, "New Folder", file.Name)
	assert.Equal(t, float64(0), file.Order) // Folders have default order
	assert.Nil(t, file.FolderID)            // No parent folder for new folders
}

// TestFromAPIFolderInsertWithoutOptionalFields tests minimal folder creation
func TestFromAPIFolderInsertWithoutOptionalFields(t *testing.T) {
	folderID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()

	apiFolder := &apiv1.FolderInsert{
		FolderId: folderID.Bytes(),
		Name:     "", // Empty name should be allowed
	}

	file, err := fromAPIFolderInsert(apiFolder, workspaceID)
	require.NoError(t, err)

	assert.Equal(t, folderID, file.ID)
	assert.Equal(t, workspaceID, file.WorkspaceID)
	assert.Equal(t, mfile.ContentTypeFolder, file.ContentType)
	assert.Equal(t, "", file.Name)
	assert.Equal(t, float64(0), file.Order)
	assert.Nil(t, file.FolderID)
}

// TestFromAPIFolderInsertWithInvalidID tests error handling for invalid folder ID
func TestFromAPIFolderInsertWithInvalidID(t *testing.T) {
	workspaceID := idwrap.NewNow()

	apiFolder := &apiv1.FolderInsert{
		FolderId: []byte("invalid-id-length"), // Invalid ULID length
		Name:     "Test Folder",
	}

	_, err := fromAPIFolderInsert(apiFolder, workspaceID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ulid: bad data size when unmarshaling")
}

// TestFromAPIFolderUpdate tests conversion from API FolderUpdate to model File
func TestFromAPIFolderUpdate(t *testing.T) {
	folderID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()

	// Create existing folder file
	existingFile := &mfile.File{
		ID:          folderID,
		WorkspaceID: workspaceID,
		ContentType: mfile.ContentTypeFolder,
		Name:        "Old Name",
		Order:       1.0,
	}

	// Test update with new name
	newName := "Updated Name"
	apiFolder := &apiv1.FolderUpdate{
		FolderId: folderID.Bytes(),
		Name:     &newName,
	}

	file, err := fromAPIFolderUpdate(apiFolder, existingFile)
	require.NoError(t, err)

	assert.Equal(t, folderID, file.ID)
	assert.Equal(t, workspaceID, file.WorkspaceID)
	assert.Equal(t, mfile.ContentTypeFolder, file.ContentType)
	assert.Equal(t, "Updated Name", file.Name)
	assert.Equal(t, float64(1.0), file.Order) // Should preserve existing order
}

// TestFromAPIFolderUpdateWithoutChanges tests update with no changes
func TestFromAPIFolderUpdateWithoutChanges(t *testing.T) {
	folderID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()

	existingFile := &mfile.File{
		ID:          folderID,
		WorkspaceID: workspaceID,
		ContentType: mfile.ContentTypeFolder,
		Name:        "Original Name",
		Order:       2.0,
	}

	// Test update with no changes
	apiFolder := &apiv1.FolderUpdate{
		FolderId: folderID.Bytes(),
		// Name is nil, so no change
	}

	file, err := fromAPIFolderUpdate(apiFolder, existingFile)
	require.NoError(t, err)

	assert.Equal(t, folderID, file.ID)
	assert.Equal(t, workspaceID, file.WorkspaceID)
	assert.Equal(t, mfile.ContentTypeFolder, file.ContentType)
	assert.Equal(t, "Original Name", file.Name) // Should preserve existing
	assert.Equal(t, float64(2.0), file.Order)   // Should preserve existing
}

// TestFromAPIFolderUpdateWithInvalidID tests error handling for invalid folder ID
func TestFromAPIFolderUpdateWithInvalidID(t *testing.T) {
	existingFile := &mfile.File{
		ID:          idwrap.NewNow(),
		WorkspaceID: idwrap.NewNow(),
		ContentType: mfile.ContentTypeFolder,
		Name:        "Test Folder",
	}

	apiFolder := &apiv1.FolderUpdate{
		FolderId: []byte("invalid-id"), // Invalid ULID
	}

	_, err := fromAPIFolderUpdate(apiFolder, existingFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ulid: bad data size when unmarshaling")
}

// TestFolderSyncResponseFrom tests folder sync response generation with table-driven tests
func TestFolderSyncResponseFrom(t *testing.T) {
	folderID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()

	tests := []struct {
		name     string
		event    FileEvent
		expected *apiv1.FolderSyncResponse
	}{
		{
			name: "create event",
			event: FileEvent{
				Type: eventTypeCreate,
				File: &apiv1.File{
					FileId:      folderID.Bytes(),
					WorkspaceId: workspaceID.Bytes(),
					Kind:        apiv1.FileKind_FILE_KIND_FOLDER,
				},
			},
			expected: &apiv1.FolderSyncResponse{
				Items: []*apiv1.FolderSync{
					{
						Value: &apiv1.FolderSync_ValueUnion{
							Kind: apiv1.FolderSync_ValueUnion_KIND_INSERT,
							Insert: &apiv1.FolderSyncInsert{
								FolderId: folderID.Bytes(),
								Name:     "", // Will be populated by calling method
							},
						},
					},
				},
			},
		},
		{
			name: "update event",
			event: FileEvent{
				Type: eventTypeUpdate,
				File: &apiv1.File{
					FileId:      folderID.Bytes(),
					WorkspaceId: workspaceID.Bytes(),
					Kind:        apiv1.FileKind_FILE_KIND_FOLDER,
				},
			},
			expected: &apiv1.FolderSyncResponse{
				Items: []*apiv1.FolderSync{
					{
						Value: &apiv1.FolderSync_ValueUnion{
							Kind: apiv1.FolderSync_ValueUnion_KIND_UPDATE,
							Update: &apiv1.FolderSyncUpdate{
								FolderId: folderID.Bytes(),
							},
						},
					},
				},
			},
		},
		{
			name: "delete event",
			event: FileEvent{
				Type: eventTypeDelete,
				File: &apiv1.File{
					FileId:      folderID.Bytes(),
					WorkspaceId: workspaceID.Bytes(),
					Kind:        apiv1.FileKind_FILE_KIND_FOLDER,
				},
			},
			expected: &apiv1.FolderSyncResponse{
				Items: []*apiv1.FolderSync{
					{
						Value: &apiv1.FolderSync_ValueUnion{
							Kind: apiv1.FolderSync_ValueUnion_KIND_DELETE,
							Delete: &apiv1.FolderSyncDelete{
								FolderId: folderID.Bytes(),
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
		{
			name: "invalid event type",
			event: FileEvent{
				Type: "invalid",
				File: &apiv1.File{
					FileId:      folderID.Bytes(),
					WorkspaceId: workspaceID.Bytes(),
					Kind:        apiv1.FileKind_FILE_KIND_FOLDER,
				},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := folderSyncResponseFrom(tt.event)
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

// TestToAPIFolderEdgeCases tests edge cases for folder conversion
func TestToAPIFolderEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		file     mfile.File
		expected *apiv1.Folder
	}{
		{
			name: "folder with empty name",
			file: mfile.File{
				ID:          idwrap.NewNow(),
				ContentType: mfile.ContentTypeFolder,
				Name:        "",
			},
			expected: &apiv1.Folder{
				FolderId: idwrap.NewNow().Bytes(), // Will be different in test
				Name:     "",
			},
		},
		{
			name: "folder with special characters in name",
			file: mfile.File{
				ID:          idwrap.NewNow(),
				ContentType: mfile.ContentTypeFolder,
				Name:        "Folder (Test) & More",
			},
			expected: &apiv1.Folder{
				FolderId: idwrap.NewNow().Bytes(), // Will be different in test
				Name:     "Folder (Test) & More",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toAPIFolder(tt.file)
			require.NotNil(t, result)
			assert.Equal(t, tt.file.ID.Bytes(), result.FolderId)
			assert.Equal(t, tt.file.Name, result.Name)
		})
	}
}
