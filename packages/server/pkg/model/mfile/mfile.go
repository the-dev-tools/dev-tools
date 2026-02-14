//nolint:revive // exported
package mfile

import (
	"fmt"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

// ContentType represents the type of content stored in a file
type ContentType int8

const (
	ContentTypeUnknown   ContentType = -1
	ContentTypeFolder    ContentType = 0 // folder
	ContentTypeHTTP      ContentType = 1 // http
	ContentTypeHTTPDelta ContentType = 2 // http delta (draft/overlay)
	ContentTypeFlow      ContentType = 3 // flow
	ContentTypeCredential ContentType = 4 // credential
	ContentTypeGraphQL   ContentType = 5 // graphql
)

// String returns the string representation of ContentType
func (ct ContentType) String() string {
	switch ct {
	case ContentTypeFolder:
		return "folder"
	case ContentTypeFlow:
		return "flow"
	case ContentTypeHTTP:
		return "http"
	case ContentTypeHTTPDelta:
		return "http_delta"
	case ContentTypeCredential:
		return "credential"
	case ContentTypeGraphQL:
		return "graphql"
	default:
		return "unknown"
	}
}

// File represents a file in the unified file system
// Uses simple pointer approach - just metadata + content reference
type File struct {
	ID          idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	ParentID    *idwrap.IDWrap // Optional parent folder
	ContentID   *idwrap.IDWrap // References content (can be nil for empty placeholders)
	ContentType ContentType    // Type of content
	Name        string
	Order       float64
	PathHash    *string
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
	return f.ContentType == ContentTypeFolder
}

// IsHTTP returns true if the file contains an HTTP request
func (f File) IsHTTP() bool {
	return f.ContentType == ContentTypeHTTP
}

// IsHTTPDelta returns true if the file contains an HTTP delta request
func (f File) IsHTTPDelta() bool {
	return f.ContentType == ContentTypeHTTPDelta
}

// IsFlow returns true if the file contains a flow
func (f File) IsFlow() bool {
	return f.ContentType == ContentTypeFlow
}

// IsCredential returns true if the file contains a credential
func (f File) IsCredential() bool {
	return f.ContentType == ContentTypeCredential
}

// IsGraphQL returns true if the file contains a GraphQL request
func (f File) IsGraphQL() bool {
	return f.ContentType == ContentTypeGraphQL
}

// IsRoot returns true if the file has no parent folder
func (f File) IsRoot() bool {
	return f.ParentID == nil
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
	if f.ContentType == ContentTypeUnknown {
		return fmt.Errorf("content type cannot be unknown")
	}
	if f.ContentType == ContentTypeFolder && f.Name == "" {
		return fmt.Errorf("file name cannot be empty")
	}
	if f.ContentID != nil && f.ContentID.Compare(idwrap.IDWrap{}) == 0 {
		return fmt.Errorf("content ID cannot be empty")
	}
	return nil
}

// ContentTypeFromString converts a string to ContentType
func ContentTypeFromString(s string) ContentType {
	switch s {
	case "folder":
		return ContentTypeFolder
	case "flow":
		return ContentTypeFlow
	case "http":
		return ContentTypeHTTP
	case "http_delta":
		return ContentTypeHTTPDelta
	case "credential":
		return ContentTypeCredential
	case "graphql":
		return ContentTypeGraphQL
	default:
		return ContentTypeUnknown
	}
}

// IsValidContentType checks if the content type is valid
func IsValidContentType(kind ContentType) bool {
	return kind == ContentTypeFolder || kind == ContentTypeFlow || kind == ContentTypeHTTP || kind == ContentTypeHTTPDelta || kind == ContentTypeCredential || kind == ContentTypeGraphQL
}

// IDEquals checks if two IDWrap values are equal
func IDEquals(id, other idwrap.IDWrap) bool {
	return id.Compare(other) == 0
}

// IDIsZero checks if the IDWrap is zero/empty
func IDIsZero(id idwrap.IDWrap) bool {
	return id.Compare(idwrap.IDWrap{}) == 0
}
