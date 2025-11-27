package mfile

import (
	"testing"
	"time"

	"the-dev-tools/server/pkg/idwrap"
)

func TestContentType_String(t *testing.T) {
	tests := []struct {
		name     string
		kind     ContentType
		expected string
	}{
		{"unknown", ContentTypeUnknown, "unknown"},
		{"folder", ContentTypeFolder, "folder"},
		{"flow", ContentTypeFlow, "flow"},
		{"http", ContentTypeHTTP, "http"},
		{"invalid", ContentType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.kind.String(); got != tt.expected {
				t.Errorf("ContentType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFile_Validate(t *testing.T) {
	validID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()
	contentID := idwrap.NewNow()

	tests := []struct {
		name    string
		file    File
		wantErr bool
	}{
		{
			name: "valid file",
			file: File{
				ID:          validID,
				WorkspaceID: workspaceID,
				ContentType: ContentTypeHTTP,
				ContentID:   &contentID,
				Name:        "test.txt",
			},
			wantErr: false,
		},
		{
			name: "empty ID",
			file: File{
				ID:          idwrap.IDWrap{},
				WorkspaceID: workspaceID,
				ContentType: ContentTypeHTTP,
				ContentID:   &contentID,
				Name:        "test.txt",
			},
			wantErr: true,
		},
		{
			name: "empty workspace ID",
			file: File{
				ID:          validID,
				WorkspaceID: idwrap.IDWrap{},
				ContentType: ContentTypeHTTP,
				ContentID:   &contentID,
				Name:        "test.txt",
			},
			wantErr: true,
		},
		{
			name: "empty name allowed for non-folder",
			file: File{
				ID:          validID,
				WorkspaceID: workspaceID,
				ContentType: ContentTypeHTTP,
				ContentID:   &contentID,
				Name:        "",
			},
			wantErr: false,
		},
		{
			name: "unknown content type",
			file: File{
				ID:          validID,
				WorkspaceID: workspaceID,
				ContentType: ContentTypeUnknown,
				ContentID:   &contentID,
				Name:        "test.txt",
			},
			wantErr: true,
		},
		{
			name: "missing content ID allowed for placeholders",
			file: File{
				ID:          validID,
				WorkspaceID: workspaceID,
				ContentType: ContentTypeHTTP,
				ContentID:   nil,
				Name:        "test.txt",
			},
			wantErr: false,
		},
		{
			name: "folder requires name",
			file: File{
				ID:          validID,
				WorkspaceID: workspaceID,
				ContentType: ContentTypeFolder,
				Name:        "",
			},
			wantErr: true,
		},
		{
			name: "zero content ID is invalid",
			file: File{
				ID:          validID,
				WorkspaceID: workspaceID,
				ContentType: ContentTypeHTTP,
				ContentID:   &idwrap.IDWrap{},
				Name:        "test.txt",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.file.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("File.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFile_HelperMethods(t *testing.T) {
	fileID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()
	contentID := idwrap.NewNow()
	parentID := idwrap.NewNow()

	tests := []struct {
		name         string
		file         File
		isFolder     bool
		isHTTP       bool
		isFlow       bool
		isRoot       bool
		hasContent   bool
	}{
		{
			name: "folder file",
			file: File{
				ID:          fileID,
				WorkspaceID: workspaceID,
				ParentID:    &parentID, // Has parent
				ContentType: ContentTypeFolder,
				ContentID:   &contentID,
				Name:        "My Folder",
			},
			isFolder:   true,
			isHTTP:     false,
			isFlow:     false,
			isRoot:     false,
			hasContent: true,
		},
		{
			name: "http file",
			file: File{
				ID:          fileID,
				WorkspaceID: workspaceID,
				ParentID:    &parentID, // Has parent
				ContentType: ContentTypeHTTP,
				ContentID:   &contentID,
				Name:        "API Request",
			},
			isFolder:   false,
			isHTTP:     true,
			isFlow:     false,
			isRoot:     false,
			hasContent: true,
		},
		{
			name: "flow file",
			file: File{
				ID:          fileID,
				WorkspaceID: workspaceID,
				ParentID:    &parentID, // Has parent
				ContentType: ContentTypeFlow,
				ContentID:   &contentID,
				Name:        "My Flow",
			},
			isFolder:   false,
			isHTTP:     false,
			isFlow:     true,
			isRoot:     false,
			hasContent: true,
		},
		{
			name: "root file",
			file: File{
				ID:          fileID,
				WorkspaceID: workspaceID,
				ContentType: ContentTypeFolder,
				ContentID:   &contentID,
				Name:        "Root",
				ParentID:    nil,
			},
			isFolder:   true,
			isHTTP:     false,
			isFlow:     false,
			isRoot:     true,
			hasContent: true,
		},
		{
			name: "file without content",
			file: File{
				ID:          fileID,
				WorkspaceID: workspaceID,
				ParentID:    &parentID, // Has parent so not root
				ContentType: ContentTypeUnknown,
				ContentID:   nil,
				Name:        "Placeholder",
			},
			isFolder:   false,
			isHTTP:     false,
			isFlow:     false,
			isRoot:     false,
			hasContent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.file.IsFolder() != tt.isFolder {
				t.Errorf("File.IsFolder() = %v, want %v", tt.file.IsFolder(), tt.isFolder)
			}
			if tt.file.IsHTTP() != tt.isHTTP {
				t.Errorf("File.IsHTTP() = %v, want %v", tt.file.IsHTTP(), tt.isHTTP)
			}
			if tt.file.IsFlow() != tt.isFlow {
				t.Errorf("File.IsFlow() = %v, want %v", tt.file.IsFlow(), tt.isFlow)
			}
			if tt.file.IsRoot() != tt.isRoot {
				t.Errorf("File.IsRoot() = %v, want %v", tt.file.IsRoot(), tt.isRoot)
			}
			if tt.file.HasContent() != tt.hasContent {
				t.Errorf("File.HasContent() = %v, want %v", tt.file.HasContent(), tt.hasContent)
			}
		})
	}
}

func TestFile_GetCreatedTime(t *testing.T) {
	file := File{
		ID: idwrap.NewNow(),
	}
	createdTime := file.GetCreatedTime()

	if createdTime.IsZero() {
		t.Error("GetCreatedTime() returned zero time")
	}

	// Should be within last few seconds
	now := time.Now()
	if now.Sub(createdTime) > time.Second*5 {
		t.Errorf("GetCreatedTime() returned time too far in the past: %v", createdTime)
	}
}

func TestContentTypeFromString(t *testing.T) {
	tests := []struct {
		input    string
		expected ContentType
	}{
		{"folder", ContentTypeFolder},
		{"item_folder", ContentTypeFolder},
		{"flow", ContentTypeFlow},
		{"http", ContentTypeHTTP},
		{"unknown", ContentTypeUnknown},
		{"", ContentTypeUnknown},
		{"invalid", ContentTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ContentTypeFromString(tt.input); got != tt.expected {
				t.Errorf("ContentTypeFromString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsValidContentType(t *testing.T) {
	tests := []struct {
		kind     ContentType
		expected bool
	}{
		{ContentTypeFolder, true},
		{ContentTypeFlow, true},
		{ContentTypeHTTP, true},
		{ContentTypeUnknown, false},
		{ContentType(-1), false},
		{ContentType(99), false},
	}

	for _, tt := range tests {
		t.Run(tt.kind.String(), func(t *testing.T) {
			if got := IsValidContentType(tt.kind); got != tt.expected {
				t.Errorf("IsValidContentType(%v) = %v, want %v", tt.kind, got, tt.expected)
			}
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	id1 := idwrap.NewNow()
	id2 := idwrap.NewNow()

	if !IDEquals(id1, id1) {
		t.Error("IDEquals() should return true for same ID")
	}

	if IDEquals(id1, id2) {
		t.Error("IDEquals() should return false for different IDs")
	}

	if IDIsZero(idwrap.IDWrap{}) != true {
		t.Error("IDIsZero() should return true for zero ID")
	}

	if IDIsZero(id1) {
		t.Error("IDIsZero() should return false for valid ID")
	}
}
