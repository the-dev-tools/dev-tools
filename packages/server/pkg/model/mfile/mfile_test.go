package mfile

import (
	"testing"
	"time"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemfolder"
)

func TestContentKind_String(t *testing.T) {
	tests := []struct {
		kind ContentKind
		want string
	}{
		{ContentKindFolder, "folder"},
		{ContentKindAPI, "api"},
		{ContentKindFlow, "flow"},
		{ContentKindUnknown, "unknown"},
		{ContentKind(-1), "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.kind)), func(t *testing.T) {
			if got := tt.kind.String(); got != tt.want {
				t.Errorf("ContentKind.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFolderAdapter_Validate(t *testing.T) {
	tests := []struct {
		name    string
		content FileContent
		wantErr bool
	}{
		{
			name: "valid folder",
			content: NewFolderContent(&mitemfolder.ItemFolder{
				ID:   idwrap.NewNow(),
				Name: "Test Folder",
			}),
			wantErr: false,
		},
		{
			name: "empty ID",
			content: NewFolderContent(&mitemfolder.ItemFolder{
				ID:   idwrap.IDWrap{},
				Name: "Test Folder",
			}),
			wantErr: true,
		},
		{
			name: "empty name",
			content: NewFolderContent(&mitemfolder.ItemFolder{
				ID:   idwrap.NewNow(),
				Name: "",
			}),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.content.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("FolderAdapter.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAPIAdapter_Validate(t *testing.T) {
	tests := []struct {
		name    string
		content FileContent
		wantErr bool
	}{
		{
			name: "valid API",
			content: NewAPIContent(&mitemapi.ItemApi{
				ID:     idwrap.NewNow(),
				Name:   "Test API",
				Method: "GET",
				Url:    "https://example.com",
			}),
			wantErr: false,
		},
		{
			name: "empty ID",
			content: NewAPIContent(&mitemapi.ItemApi{
				ID:     idwrap.IDWrap{},
				Name:   "Test API",
				Method: "GET",
				Url:    "https://example.com",
			}),
			wantErr: true,
		},
		{
			name: "empty method",
			content: NewAPIContent(&mitemapi.ItemApi{
				ID:     idwrap.NewNow(),
				Name:   "Test API",
				Method: "",
				Url:    "https://example.com",
			}),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.content.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("APIAdapter.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFlowAdapter_Validate(t *testing.T) {
	tests := []struct {
		name    string
		content FileContent
		wantErr bool
	}{
		{
			name: "valid flow",
			content: NewFlowContent(&mflow.Flow{
				ID:       idwrap.NewNow(),
				Name:     "Test Flow",
				Duration: 1000,
			}),
			wantErr: false,
		},
		{
			name: "empty ID",
			content: NewFlowContent(&mflow.Flow{
				ID:       idwrap.IDWrap{},
				Name:     "Test Flow",
				Duration: 1000,
			}),
			wantErr: true,
		},
		{
			name: "negative duration",
			content: NewFlowContent(&mflow.Flow{
				ID:       idwrap.NewNow(),
				Name:     "Test Flow",
				Duration: -1,
			}),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.content.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("FlowAdapter.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFile_Validate(t *testing.T) {
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
				ID:          idwrap.NewNow(),
				WorkspaceID: workspaceID,
				ContentID:   &contentID,
				ContentKind: ContentKindAPI,
				Name:        "Test File",
				Order:       0,
				UpdatedAt:   time.Now(),
			},
			wantErr: false,
		},
		{
			name: "empty ID",
			file: File{
				ID:          idwrap.IDWrap{},
				WorkspaceID: workspaceID,
				ContentID:   &contentID,
				ContentKind: ContentKindAPI,
				Name:        "Test File",
				Order:       0,
				UpdatedAt:   time.Now(),
			},
			wantErr: true,
		},
		{
			name: "unknown content kind",
			file: File{
				ID:          idwrap.NewNow(),
				WorkspaceID: workspaceID,
				ContentID:   &contentID,
				ContentKind: ContentKindUnknown,
				Name:        "Test File",
				Order:       0,
				UpdatedAt:   time.Now(),
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
	workspaceID := idwrap.NewNow()
	contentID := idwrap.NewNow()

	t.Run("IsFolder", func(t *testing.T) {
		file := File{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaceID,
			ContentID:   &contentID,
			ContentKind: ContentKindFolder,
			Name:        "Folder",
		}
		if !file.IsFolder() {
			t.Error("Expected file to be folder")
		}
		if file.IsAPI() || file.IsFlow() {
			t.Error("Expected file to not be API or flow")
		}
	})

	t.Run("IsAPI", func(t *testing.T) {
		file := File{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaceID,
			ContentID:   &contentID,
			ContentKind: ContentKindAPI,
			Name:        "API",
		}
		if !file.IsAPI() {
			t.Error("Expected file to be API")
		}
		if file.IsFolder() || file.IsFlow() {
			t.Error("Expected file to not be folder or flow")
		}
	})

	t.Run("IsFlow", func(t *testing.T) {
		file := File{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaceID,
			ContentID:   &contentID,
			ContentKind: ContentKindFlow,
			Name:        "Flow",
		}
		if !file.IsFlow() {
			t.Error("Expected file to be flow")
		}
		if file.IsFolder() || file.IsAPI() {
			t.Error("Expected file to not be folder or API")
		}
	})

	t.Run("IsRoot", func(t *testing.T) {
		file := File{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaceID,
			ContentID:   &contentID,
			ContentKind: ContentKindAPI,
			Name:        "Root File",
		}
		if !file.IsRoot() {
			t.Error("Expected file to be root")
		}

		folderID := idwrap.NewNow()
		file.FolderID = &folderID
		if file.IsRoot() {
			t.Error("Expected file to not be root when folder ID is set")
		}
	})

	t.Run("HasContent", func(t *testing.T) {
		file := File{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaceID,
			ContentID:   &contentID,
			ContentKind: ContentKindAPI,
			Name:        "File with content",
		}
		if !file.HasContent() {
			t.Error("Expected file to have content")
		}

		file.ContentID = nil
		if file.HasContent() {
			t.Error("Expected file to not have content when ContentID is nil")
		}
	})
}

func TestFileWithContent_Validate(t *testing.T) {
	workspaceID := idwrap.NewNow()
	contentID := idwrap.NewNow()

	t.Run("valid file with content", func(t *testing.T) {
		file := File{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaceID,
			ContentID:   &contentID,
			ContentKind: ContentKindAPI,
			Name:        "Test File",
			Order:       0,
			UpdatedAt:   time.Now(),
		}

		content := NewAPIContent(&mitemapi.ItemApi{
			ID:     contentID,
			Name:   "Test API",
			Method: "GET",
			Url:    "https://example.com",
		})

		fwc := FileWithContent{
			File:    file,
			Content: content,
		}

		err := fwc.Validate()
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("content kind mismatch", func(t *testing.T) {
		file := File{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaceID,
			ContentID:   &contentID,
			ContentKind: ContentKindAPI,
			Name:        "Test File",
			Order:       0,
			UpdatedAt:   time.Now(),
		}

		content := NewFlowContent(&mflow.Flow{
			ID:       contentID,
			Name:     "Test Flow",
			Duration: 1000,
		})

		fwc := FileWithContent{
			File:    file,
			Content: content,
		}

		err := fwc.Validate()
		if err == nil {
			t.Error("Expected validation error for content kind mismatch")
		}
	})
}

func TestContentKindFromString(t *testing.T) {
	tests := []struct {
		input string
		want  ContentKind
	}{
		{"folder", ContentKindFolder},
		{"item_folder", ContentKindFolder},
		{"api", ContentKindAPI},
		{"item_api", ContentKindAPI},
		{"flow", ContentKindFlow},
		{"unknown", ContentKindUnknown},
		{"", ContentKindUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ContentKindFromString(tt.input); got != tt.want {
				t.Errorf("ContentKindFromString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidContentKind(t *testing.T) {
	tests := []struct {
		kind ContentKind
		want bool
	}{
		{ContentKindFolder, true},
		{ContentKindAPI, true},
		{ContentKindFlow, true},
		{ContentKindUnknown, false},
		{ContentKind(-1), false},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.kind)), func(t *testing.T) {
			if got := IsValidContentKind(tt.kind); got != tt.want {
				t.Errorf("IsValidContentKind(%v) = %v, want %v", tt.kind, got, tt.want)
			}
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	id1 := idwrap.NewNow()
	id2 := idwrap.NewNow()

	t.Run("IDEquals", func(t *testing.T) {
		if !IDEquals(id1, id1) {
			t.Error("Expected same IDs to be equal")
		}
		if IDEquals(id1, id2) {
			t.Error("Expected different IDs to not be equal")
		}
	})

	t.Run("IDIsZero", func(t *testing.T) {
		if IDIsZero(id1) {
			t.Error("Expected non-zero ID to not be zero")
		}
		if !IDIsZero(idwrap.IDWrap{}) {
			t.Error("Expected zero ID to be zero")
		}
	})
}
