package mfile

import (
	"fmt"
	"time"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemfolder"
)

// ContentKind represents the type of content stored in a file
type ContentKind int8

const (
	ContentKindUnknown ContentKind = -1
	ContentKindFolder  ContentKind = 0 // item_folder
	ContentKindAPI     ContentKind = 1 // item_api
	ContentKindFlow    ContentKind = 2 // flow
)

// String returns the string representation of ContentKind
func (ck ContentKind) String() string {
	switch ck {
	case ContentKindFolder:
		return "folder"
	case ContentKindAPI:
		return "api"
	case ContentKindFlow:
		return "flow"
	default:
		return "unknown"
	}
}

// FileContent represents the union interface for all file content types
// Now works with existing model pointers: *mitemapi.ItemApi, *mflow.Flow, *mitemfolder.ItemFolder
type FileContent interface {
	// GetKind returns the content kind
	GetKind() ContentKind
	// GetID returns the content ID
	GetID() idwrap.IDWrap
	// GetName returns the content name
	GetName() string
	// Validate performs content-specific validation
	Validate() error
}

// folderAdapter implements FileContent interface for *mitemfolder.ItemFolder
type folderAdapter struct {
	folder *mitemfolder.ItemFolder
}

func (f *folderAdapter) GetKind() ContentKind {
	return ContentKindFolder
}

func (f *folderAdapter) GetID() idwrap.IDWrap {
	return f.folder.ID
}

func (f *folderAdapter) GetName() string {
	return f.folder.Name
}

func (f *folderAdapter) Validate() error {
	if f.folder.ID.Compare(idwrap.IDWrap{}) == 0 {
		return fmt.Errorf("folder content ID cannot be empty")
	}
	if f.folder.Name == "" {
		return fmt.Errorf("folder name cannot be empty")
	}
	return nil
}

// apiAdapter implements FileContent interface for *mitemapi.ItemApi
type apiAdapter struct {
	api *mitemapi.ItemApi
}

func (a *apiAdapter) GetKind() ContentKind {
	return ContentKindAPI
}

func (a *apiAdapter) GetID() idwrap.IDWrap {
	return a.api.ID
}

func (a *apiAdapter) GetName() string {
	return a.api.Name
}

func (a *apiAdapter) Validate() error {
	if a.api.ID.Compare(idwrap.IDWrap{}) == 0 {
		return fmt.Errorf("API content ID cannot be empty")
	}
	if a.api.Name == "" {
		return fmt.Errorf("API name cannot be empty")
	}
	if a.api.Method == "" {
		return fmt.Errorf("API method cannot be empty")
	}
	if a.api.Url == "" {
		return fmt.Errorf("API URL cannot be empty")
	}
	return nil
}

// flowAdapter implements FileContent interface for *mflow.Flow
type flowAdapter struct {
	flow *mflow.Flow
}

func (f *flowAdapter) GetKind() ContentKind {
	return ContentKindFlow
}

func (f *flowAdapter) GetID() idwrap.IDWrap {
	return f.flow.ID
}

func (f *flowAdapter) GetName() string {
	return f.flow.Name
}

func (f *flowAdapter) Validate() error {
	if f.flow.ID.Compare(idwrap.IDWrap{}) == 0 {
		return fmt.Errorf("flow content ID cannot be empty")
	}
	if f.flow.Name == "" {
		return fmt.Errorf("flow name cannot be empty")
	}
	if f.flow.Duration < 0 {
		return fmt.Errorf("flow duration cannot be negative")
	}
	return nil
}

// File represents a file in the unified file system
type File struct {
	ID          idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	FolderID    *idwrap.IDWrap // Optional parent folder
	ContentID   *idwrap.IDWrap // References content (can be nil for empty placeholders)
	ContentKind ContentKind    // Type of content
	Name        string
	Order       float64
	UpdatedAt   time.Time
}

// GetCreatedTime returns the creation time from the ULID
func (f File) GetCreatedTime() time.Time {
	return f.ID.Time()
}

// GetCreatedTimeUnix returns the creation time as Unix milliseconds
func (f File) GetCreatedTimeUnix() int64 {
	return idwrap.GetUnixMilliFromULID(f.ID)
}

// IsFolder returns true if the file is a folder
func (f File) IsFolder() bool {
	return f.ContentKind == ContentKindFolder
}

// IsAPI returns true if the file contains an API request
func (f File) IsAPI() bool {
	return f.ContentKind == ContentKindAPI
}

// IsFlow returns true if the file contains a flow
func (f File) IsFlow() bool {
	return f.ContentKind == ContentKindFlow
}

// IsRoot returns true if the file has no parent folder
func (f File) IsRoot() bool {
	return f.FolderID == nil
}

// HasContent returns true if the file has associated content
func (f File) HasContent() bool {
	return f.ContentID != nil && f.ContentID.Compare(idwrap.IDWrap{}) != 0
}

// Validate performs basic validation on the file
func (f File) Validate() error {
	if f.ID.Compare(idwrap.IDWrap{}) == 0 {
		return fmt.Errorf("file ID cannot be empty")
	}
	if f.WorkspaceID.Compare(idwrap.IDWrap{}) == 0 {
		return fmt.Errorf("workspace ID cannot be empty")
	}
	if f.Name == "" {
		return fmt.Errorf("file name cannot be empty")
	}
	if f.ContentKind == ContentKindUnknown {
		return fmt.Errorf("content kind cannot be unknown")
	}
	// Validate that content_id is present for known content kinds
	if f.ContentKind != ContentKindUnknown && !f.HasContent() {
		return fmt.Errorf("content ID is required for content kind %s", f.ContentKind.String())
	}
	return nil
}

// WithContent returns a new FileWithContent containing the file and its content
func (f File) WithContent(content FileContent) FileWithContent {
	return FileWithContent{
		File:    f,
		Content: content,
	}
}

// FileWithContent represents a file along with its content data
type FileWithContent struct {
	File    File
	Content FileContent
}

// Validate validates both the file and its content
func (fwc FileWithContent) Validate() error {
	if err := fwc.File.Validate(); err != nil {
		return fmt.Errorf("file validation failed: %w", err)
	}
	if err := fwc.Content.Validate(); err != nil {
		return fmt.Errorf("content validation failed: %w", err)
	}
	// Ensure content kinds match
	if fwc.File.ContentKind != fwc.Content.GetKind() {
		return fmt.Errorf("content kind mismatch: file has %s but content is %s",
			fwc.File.ContentKind.String(), fwc.Content.GetKind().String())
	}
	// Ensure content IDs match
	if fwc.File.ContentID == nil || fwc.File.ContentID.Compare(idwrap.IDWrap{}) == 0 {
		return fmt.Errorf("file content ID is missing")
	}
	if fwc.File.ContentID.Compare(fwc.Content.GetID()) != 0 {
		return fmt.Errorf("content ID mismatch: file has %s but content is %s",
			fwc.File.ContentID.String(), fwc.Content.GetID().String())
	}
	return nil
}

// Helper functions for creating content types from existing models

// NewFolderContent creates a FileContent from *mitemfolder.ItemFolder
func NewFolderContent(folder *mitemfolder.ItemFolder) FileContent {
	return &folderAdapter{folder: folder}
}

// NewAPIContent creates a FileContent from *mitemapi.ItemApi
func NewAPIContent(api *mitemapi.ItemApi) FileContent {
	return &apiAdapter{api: api}
}

// NewFlowContent creates a FileContent from *mflow.Flow
func NewFlowContent(flow *mflow.Flow) FileContent {
	return &flowAdapter{flow: flow}
}

// ContentKindFromString converts a string to ContentKind
func ContentKindFromString(s string) ContentKind {
	switch s {
	case "folder", "item_folder":
		return ContentKindFolder
	case "api", "item_api":
		return ContentKindAPI
	case "flow":
		return ContentKindFlow
	default:
		return ContentKindUnknown
	}
}

// IsValidContentKind checks if the content kind is valid
func IsValidContentKind(kind ContentKind) bool {
	return kind >= ContentKindFolder && kind <= ContentKindFlow
}

// IDEquals checks if two IDWrap values are equal
func IDEquals(id, other idwrap.IDWrap) bool {
	return id.Compare(other) == 0
}

// IDIsZero checks if the IDWrap is zero/empty
func IDIsZero(id idwrap.IDWrap) bool {
	return id.Compare(idwrap.IDWrap{}) == 0
}
