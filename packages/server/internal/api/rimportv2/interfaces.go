package rimportv2

import (
	"context"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/translate/harv2"
)

// HARTranslator handles HAR file processing and conversion to modern models
type HARTranslator interface {
	ConvertHAR(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*harv2.HarResolved, error)
}

// StorageManager coordinates all database operations for imported data
type StorageManager interface {
	StoreHTTPEntities(ctx context.Context, httpReqs []*mhttp.HTTP) error
	StoreFiles(ctx context.Context, files []*mfile.File) error
	StoreFlow(ctx context.Context, flow *mflow.Flow) error
	StoreImportResults(ctx context.Context, results *ImportResults) error
}

// FlowGenerator handles flow creation from imported HTTP requests
type FlowGenerator interface {
	CreateFlow(ctx context.Context, workspaceID idwrap.IDWrap, name string, httpReqs []*mhttp.HTTP) (*mflow.Flow, error)
}

// Validator handles input validation for import requests
type Validator interface {
	ValidateImportRequest(ctx context.Context, req *ImportRequest) error
	ValidateWorkspaceAccess(ctx context.Context, workspaceID idwrap.IDWrap) error
}

// DomainProcessor handles domain variable processing for templating
type DomainProcessor interface {
	ProcessDomainData(ctx context.Context, domainData []ImportDomainData, workspaceID idwrap.IDWrap) error
	ApplyDomainTemplate(ctx context.Context, httpReqs []*mhttp.HTTP, domainData []ImportDomainData) ([]*mhttp.HTTP, error)
}

// ImportResults represents the complete results of an import operation
type ImportResults struct {
	Flow       *mflow.Flow
	HTTPReqs   []*mhttp.HTTP
	Files      []*mfile.File
	Domains    []string
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