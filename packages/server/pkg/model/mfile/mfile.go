package mfile

import (
	"fmt"
	"time"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemfolder"
)

// ContentKind represents the type of content stored in a file
type ContentKind int8

const (
	ContentKindUnknown ContentKind = -1
	ContentKindFolder  ContentKind = 0 // item_folder
	ContentKindAPI     ContentKind = 1 // item_api (legacy)
	ContentKindFlow    ContentKind = 2 // flow
	ContentKindHTTP    ContentKind = 3 // http (new model)
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
	case ContentKindHTTP:
		return "http"
	default:
		return "unknown"
	}
}

// Content represents any file content using composition instead of adapters
type Content struct {
	Kind  ContentKind
	Folder *mitemfolder.ItemFolder
	API    *mitemapi.ItemApi
	Flow   *mflow.Flow
	HTTP   *mhttp.HTTP
}

// NewContentFromFolder creates content from a folder
func NewContentFromFolder(folder *mitemfolder.ItemFolder) Content {
	return Content{
		Kind:   ContentKindFolder,
		Folder: folder,
	}
}

// NewContentFromAPI creates content from an API
func NewContentFromAPI(api *mitemapi.ItemApi) Content {
	return Content{
		Kind: ContentKindAPI,
		API:  api,
	}
}

// NewContentFromFlow creates content from a flow
func NewContentFromFlow(flow *mflow.Flow) Content {
	return Content{
		Kind: ContentKindFlow,
		Flow: flow,
	}
}

// NewContentFromHTTP creates content from an HTTP request
func NewContentFromHTTP(http *mhttp.HTTP) Content {
	return Content{
		Kind: ContentKindHTTP,
		HTTP: http,
	}
}

// GetID returns the content ID based on the content type
func (c Content) GetID() idwrap.IDWrap {
	switch c.Kind {
	case ContentKindFolder:
		if c.Folder != nil {
			return c.Folder.ID
		}
	case ContentKindAPI:
		if c.API != nil {
			return c.API.ID
		}
	case ContentKindFlow:
		if c.Flow != nil {
			return c.Flow.ID
		}
	case ContentKindHTTP:
		if c.HTTP != nil {
			return c.HTTP.ID
		}
	}
	return idwrap.IDWrap{}
}

// GetName returns the content name based on the content type
func (c Content) GetName() string {
	switch c.Kind {
	case ContentKindFolder:
		if c.Folder != nil {
			return c.Folder.Name
		}
	case ContentKindAPI:
		if c.API != nil {
			return c.API.Name
		}
	case ContentKindFlow:
		if c.Flow != nil {
			return c.Flow.Name
		}
	case ContentKindHTTP:
		if c.HTTP != nil {
			return c.HTTP.Name
		}
	}
	return ""
}

// Validate performs content-specific validation
func (c Content) Validate() error {
	switch c.Kind {
	case ContentKindFolder:
		if c.Folder == nil {
			return fmt.Errorf("folder content is nil")
		}
		return c.validateFolder()
	case ContentKindAPI:
		if c.API == nil {
			return fmt.Errorf("API content is nil")
		}
		return c.validateAPI()
	case ContentKindFlow:
		if c.Flow == nil {
			return fmt.Errorf("flow content is nil")
		}
		return c.validateFlow()
	case ContentKindHTTP:
		if c.HTTP == nil {
			return fmt.Errorf("HTTP content is nil")
		}
		return c.validateHTTP()
	default:
		return fmt.Errorf("unknown content kind: %s", c.Kind.String())
	}
}

func (c Content) validateFolder() error {
	if c.Folder.ID.Compare(idwrap.IDWrap{}) == 0 {
		return fmt.Errorf("folder content ID cannot be empty")
	}
	if c.Folder.Name == "" {
		return fmt.Errorf("folder name cannot be empty")
	}
	return nil
}

func (c Content) validateAPI() error {
	if c.API.ID.Compare(idwrap.IDWrap{}) == 0 {
		return fmt.Errorf("API content ID cannot be empty")
	}
	if c.API.Name == "" {
		return fmt.Errorf("API name cannot be empty")
	}
	if c.API.Method == "" {
		return fmt.Errorf("API method cannot be empty")
	}
	if c.API.Url == "" {
		return fmt.Errorf("API URL cannot be empty")
	}
	return nil
}

func (c Content) validateFlow() error {
	if c.Flow.ID.Compare(idwrap.IDWrap{}) == 0 {
		return fmt.Errorf("flow content ID cannot be empty")
	}
	if c.Flow.Name == "" {
		return fmt.Errorf("flow name cannot be empty")
	}
	if c.Flow.Duration < 0 {
		return fmt.Errorf("flow duration cannot be negative")
	}
	return nil
}

func (c Content) validateHTTP() error {
	if c.HTTP.ID.Compare(idwrap.IDWrap{}) == 0 {
		return fmt.Errorf("HTTP content ID cannot be empty")
	}
	if c.HTTP.Name == "" {
		return fmt.Errorf("HTTP name cannot be empty")
	}
	if c.HTTP.Method == "" {
		return fmt.Errorf("HTTP method cannot be empty")
	}
	if c.HTTP.Url == "" {
		return fmt.Errorf("HTTP URL cannot be empty")
	}
	return nil
}

// AsFolder returns the folder content, or nil if not a folder
func (c Content) AsFolder() *mitemfolder.ItemFolder {
	return c.Folder
}

// AsAPI returns the API content, or nil if not an API
func (c Content) AsAPI() *mitemapi.ItemApi {
	return c.API
}

// AsFlow returns the flow content, or nil if not a flow
func (c Content) AsFlow() *mflow.Flow {
	return c.Flow
}

// AsHTTP returns the HTTP content, or nil if not an HTTP request
func (c Content) AsHTTP() *mhttp.HTTP {
	return c.HTTP
}

// IsZero returns true if no content is set
func (c Content) IsZero() bool {
	return c.Folder == nil && c.API == nil && c.Flow == nil && c.HTTP == nil
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

// IsHTTP returns true if the file contains an HTTP request
func (f File) IsHTTP() bool {
	return f.ContentKind == ContentKindHTTP
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
func (f File) WithContent(content Content) FileWithContent {
	return FileWithContent{
		File:    f,
		Content: content,
	}
}

// FileWithContent represents a file along with its content data
type FileWithContent struct {
	File    File
	Content Content
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
	if fwc.File.ContentKind != fwc.Content.Kind {
		return fmt.Errorf("content kind mismatch: file has %s but content is %s",
			fwc.File.ContentKind.String(), fwc.Content.Kind.String())
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

// ContentKindFromString converts a string to ContentKind
func ContentKindFromString(s string) ContentKind {
	switch s {
	case "folder", "item_folder":
		return ContentKindFolder
	case "api", "item_api":
		return ContentKindAPI
	case "flow":
		return ContentKindFlow
	case "http":
		return ContentKindHTTP
	default:
		return ContentKindUnknown
	}
}

// IsValidContentKind checks if the content kind is valid
func IsValidContentKind(kind ContentKind) bool {
	return kind >= ContentKindFolder && kind <= ContentKindHTTP
}

// IDEquals checks if two IDWrap values are equal
func IDEquals(id, other idwrap.IDWrap) bool {
	return id.Compare(other) == 0
}

// IDIsZero checks if the IDWrap is zero/empty
func IDIsZero(id idwrap.IDWrap) bool {
	return id.Compare(idwrap.IDWrap{}) == 0
}

// Legacy compatibility functions for gradual migration

// FileContent represents the legacy interface for backwards compatibility
// Deprecated: Use Content struct instead
type FileContent interface {
	GetKind() ContentKind
	GetID() idwrap.IDWrap
	GetName() string
	Validate() error
}

// fileContentAdapter wraps Content to implement legacy FileContent interface
// Deprecated: Use Content struct directly instead
type fileContentAdapter struct {
	content Content
}

func (a *fileContentAdapter) GetKind() ContentKind {
	return a.content.Kind
}

func (a *fileContentAdapter) GetID() idwrap.IDWrap {
	return a.content.GetID()
}

func (a *fileContentAdapter) GetName() string {
	return a.content.GetName()
}

func (a *fileContentAdapter) Validate() error {
	return a.content.Validate()
}

// NewFolderContent creates a legacy FileContent from *mitemfolder.ItemFolder
// Deprecated: Use NewContentFromFolder instead
func NewFolderContent(folder *mitemfolder.ItemFolder) FileContent {
	return &fileContentAdapter{content: NewContentFromFolder(folder)}
}

// NewAPIContent creates a legacy FileContent from *mitemapi.ItemApi
// Deprecated: Use NewContentFromAPI instead
func NewAPIContent(api *mitemapi.ItemApi) FileContent {
	return &fileContentAdapter{content: NewContentFromAPI(api)}
}

// NewFlowContent creates a legacy FileContent from *mflow.Flow
// Deprecated: Use NewContentFromFlow instead
func NewFlowContent(flow *mflow.Flow) FileContent {
	return &fileContentAdapter{content: NewContentFromFlow(flow)}
}

// NewHTTPContent creates a legacy FileContent from *mhttp.HTTP
// Deprecated: Use NewContentFromHTTP instead
func NewHTTPContent(http *mhttp.HTTP) FileContent {
	return &fileContentAdapter{content: NewContentFromHTTP(http)}
}