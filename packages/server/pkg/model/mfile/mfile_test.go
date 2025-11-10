package mfile

import (
	"testing"
	"time"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
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
		{ContentKindHTTP, "http"},
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

func TestContent_Validate(t *testing.T) {
	tests := []struct {
		name    string
		content Content
		wantErr bool
	}{
		{
			name: "valid folder",
			content: NewContentFromFolder(&mitemfolder.ItemFolder{
				ID:   idwrap.NewNow(),
				Name: "Test Folder",
			}),
			wantErr: false,
		},
		{
			name: "empty folder ID",
			content: NewContentFromFolder(&mitemfolder.ItemFolder{
				ID:   idwrap.IDWrap{},
				Name: "Test Folder",
			}),
			wantErr: true,
		},
		{
			name: "empty folder name",
			content: NewContentFromFolder(&mitemfolder.ItemFolder{
				ID:   idwrap.NewNow(),
				Name: "",
			}),
			wantErr: true,
		},
		{
			name: "valid API",
			content: NewContentFromAPI(&mitemapi.ItemApi{
				ID:     idwrap.NewNow(),
				Name:   "Test API",
				Method: "GET",
				Url:    "https://example.com",
			}),
			wantErr: false,
		},
		{
			name: "empty API ID",
			content: NewContentFromAPI(&mitemapi.ItemApi{
				ID:     idwrap.IDWrap{},
				Name:   "Test API",
				Method: "GET",
				Url:    "https://example.com",
			}),
			wantErr: true,
		},
		{
			name: "empty API method",
			content: NewContentFromAPI(&mitemapi.ItemApi{
				ID:     idwrap.NewNow(),
				Name:   "Test API",
				Method: "",
				Url:    "https://example.com",
			}),
			wantErr: true,
		},
		{
			name: "valid flow",
			content: NewContentFromFlow(&mflow.Flow{
				ID:       idwrap.NewNow(),
				Name:     "Test Flow",
				Duration: 1000,
			}),
			wantErr: false,
		},
		{
			name: "empty flow ID",
			content: NewContentFromFlow(&mflow.Flow{
				ID:       idwrap.IDWrap{},
				Name:     "Test Flow",
				Duration: 1000,
			}),
			wantErr: true,
		},
		{
			name: "negative flow duration",
			content: NewContentFromFlow(&mflow.Flow{
				ID:       idwrap.NewNow(),
				Name:     "Test Flow",
				Duration: -1,
			}),
			wantErr: true,
		},
		{
			name: "valid HTTP",
			content: NewContentFromHTTP(&mhttp.HTTP{
				ID:     idwrap.NewNow(),
				Name:   "Test HTTP",
				Method: "POST",
				Url:    "https://api.example.com",
			}),
			wantErr: false,
		},
		{
			name: "empty HTTP method",
			content: NewContentFromHTTP(&mhttp.HTTP{
				ID:     idwrap.NewNow(),
				Name:   "Test HTTP",
				Method: "",
				Url:    "https://api.example.com",
			}),
			wantErr: true,
		},
		{
			name: "nil content",
			content: Content{
				Kind: ContentKindFolder,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.content.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Content.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestContent_GetMethods(t *testing.T) {
	folderID := idwrap.NewNow()
	apiID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	httpID := idwrap.NewNow()

	tests := []struct {
		name    string
		content Content
		wantID  idwrap.IDWrap
		wantName string
	}{
		{
			name: "folder content",
			content: NewContentFromFolder(&mitemfolder.ItemFolder{
				ID:   folderID,
				Name: "Test Folder",
			}),
			wantID:   folderID,
			wantName: "Test Folder",
		},
		{
			name: "API content",
			content: NewContentFromAPI(&mitemapi.ItemApi{
				ID:     apiID,
				Name:   "Test API",
				Method: "GET",
				Url:    "https://example.com",
			}),
			wantID:   apiID,
			wantName: "Test API",
		},
		{
			name: "flow content",
			content: NewContentFromFlow(&mflow.Flow{
				ID:       flowID,
				Name:     "Test Flow",
				Duration: 1000,
			}),
			wantID:   flowID,
			wantName: "Test Flow",
		},
		{
			name: "HTTP content",
			content: NewContentFromHTTP(&mhttp.HTTP{
				ID:     httpID,
				Name:   "Test HTTP",
				Method: "POST",
				Url:    "https://api.example.com",
			}),
			wantID:   httpID,
			wantName: "Test HTTP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.content.GetID(); got.Compare(tt.wantID) != 0 {
				t.Errorf("Content.GetID() = %v, want %v", got.String(), tt.wantID.String())
			}
			if got := tt.content.GetName(); got != tt.wantName {
				t.Errorf("Content.GetName() = %v, want %v", got, tt.wantName)
			}
		})
	}
}

func TestContent_AsMethods(t *testing.T) {
	folder := &mitemfolder.ItemFolder{ID: idwrap.NewNow(), Name: "Folder"}
	api := &mitemapi.ItemApi{ID: idwrap.NewNow(), Name: "API", Method: "GET", Url: "https://example.com"}
	flow := &mflow.Flow{ID: idwrap.NewNow(), Name: "Flow", Duration: 1000}
	http := &mhttp.HTTP{ID: idwrap.NewNow(), Name: "HTTP", Method: "POST", Url: "https://api.example.com"}

	tests := []struct {
		name               string
		content            Content
		wantFolder         *mitemfolder.ItemFolder
		wantAPI            *mitemapi.ItemApi
		wantFlow           *mflow.Flow
		wantHTTP           *mhttp.HTTP
	}{
		{
			name:       "folder content",
			content:    NewContentFromFolder(folder),
			wantFolder: folder,
		},
		{
			name:    "API content",
			content: NewContentFromAPI(api),
			wantAPI: api,
		},
		{
			name:     "flow content",
			content:  NewContentFromFlow(flow),
			wantFlow: flow,
		},
		{
			name:    "HTTP content",
			content: NewContentFromHTTP(http),
			wantHTTP: http,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.content.AsFolder(); got != tt.wantFolder {
				t.Errorf("Content.AsFolder() = %v, want %v", got, tt.wantFolder)
			}
			if got := tt.content.AsAPI(); got != tt.wantAPI {
				t.Errorf("Content.AsAPI() = %v, want %v", got, tt.wantAPI)
			}
			if got := tt.content.AsFlow(); got != tt.wantFlow {
				t.Errorf("Content.AsFlow() = %v, want %v", got, tt.wantFlow)
			}
			if got := tt.content.AsHTTP(); got != tt.wantHTTP {
				t.Errorf("Content.AsHTTP() = %v, want %v", got, tt.wantHTTP)
			}
		})
	}
}

func TestContent_IsZero(t *testing.T) {
	t.Run("zero content", func(t *testing.T) {
		content := Content{Kind: ContentKindFolder}
		if !content.IsZero() {
			t.Error("Expected content to be zero")
		}
	})

	t.Run("non-zero folder content", func(t *testing.T) {
		content := NewContentFromFolder(&mitemfolder.ItemFolder{
			ID:   idwrap.NewNow(),
			Name: "Test",
		})
		if content.IsZero() {
			t.Error("Expected content to be non-zero")
		}
	})
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
		if file.IsAPI() || file.IsFlow() || file.IsHTTP() {
			t.Error("Expected file to not be API, flow, or HTTP")
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
		if file.IsFolder() || file.IsFlow() || file.IsHTTP() {
			t.Error("Expected file to not be folder, flow, or HTTP")
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
		if file.IsFolder() || file.IsAPI() || file.IsHTTP() {
			t.Error("Expected file to not be folder, API, or HTTP")
		}
	})

	t.Run("IsHTTP", func(t *testing.T) {
		file := File{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaceID,
			ContentID:   &contentID,
			ContentKind: ContentKindHTTP,
			Name:        "HTTP",
		}
		if !file.IsHTTP() {
			t.Error("Expected file to be HTTP")
		}
		if file.IsFolder() || file.IsAPI() || file.IsFlow() {
			t.Error("Expected file to not be folder, API, or flow")
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

		content := NewContentFromAPI(&mitemapi.ItemApi{
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

		content := NewContentFromFlow(&mflow.Flow{
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
		{"http", ContentKindHTTP},
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
		{ContentKindHTTP, true},
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

// Legacy compatibility tests
func TestLegacyFileContentInterface(t *testing.T) {
	folder := &mitemfolder.ItemFolder{
		ID:   idwrap.NewNow(),
		Name: "Test Folder",
	}

	// Test legacy interface still works
	content := NewFolderContent(folder)

	if content.GetKind() != ContentKindFolder {
		t.Errorf("Expected ContentKindFolder, got %v", content.GetKind())
	}

	if content.GetID().Compare(folder.ID) != 0 {
		t.Error("Expected IDs to match")
	}

	if content.GetName() != folder.Name {
		t.Error("Expected names to match")
	}

	if err := content.Validate(); err != nil {
		t.Errorf("Expected no validation error, got %v", err)
	}
}