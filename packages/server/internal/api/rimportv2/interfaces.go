package rimportv2

import (
	"context"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/translate/harv2"
)

// Importer handles the complete import pipeline: HAR processing and storage
type Importer interface {
	// Process and store HAR data with modern models
	ImportAndStore(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*harv2.HarResolved, error)
	// Store individual entity types
	StoreHTTPEntities(ctx context.Context, httpReqs []*mhttp.HTTP) error
	StoreFiles(ctx context.Context, files []*mfile.File) error
	StoreFlow(ctx context.Context, flow *mflow.Flow) error
	// Store complete import results atomically
	StoreImportResults(ctx context.Context, results *ImportResults) error
}

// Validator handles input validation for import requests
type Validator interface {
	ValidateImportRequest(ctx context.Context, req *ImportRequest) error
	ValidateWorkspaceAccess(ctx context.Context, workspaceID idwrap.IDWrap) error
}


// ImportResults represents the complete results of an import operation
type ImportResults struct {
	Flow        *mflow.Flow
	HTTPReqs    []*mhttp.HTTP
	Files       []*mfile.File
	Domains     []string
	WorkspaceID idwrap.IDWrap
}

// ImportRequest represents the incoming import request with domain data
type ImportRequest struct {
	WorkspaceID idwrap.IDWrap
	Name        string
	Data        []byte
	TextData    string
	DomainData  []ImportDomainData
}

// ImportResponse represents the response to an import request
type ImportResponse struct {
	MissingData ImportMissingDataKind
	Domains     []string
}

// ImportMissingDataKind represents the type of missing data
type ImportMissingDataKind int32

const (
	ImportMissingDataKind_UNSPECIFIED ImportMissingDataKind = 0
	ImportMissingDataKind_DOMAIN      ImportMissingDataKind = 1
)

// ImportDomainData represents domain variable configuration
type ImportDomainData struct {
	Enabled  bool
	Domain   string
	Variable string
}
