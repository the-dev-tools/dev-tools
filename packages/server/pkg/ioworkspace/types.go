package ioworkspace

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mworkspace"
)

// WorkspaceBundle contains all entities that make up a complete workspace
// including HTTP requests, flows, files, folders, environments, and all associated data.
// This structure is used for workspace import/export operations.
type WorkspaceBundle struct {
	// Workspace metadata
	Workspace mworkspace.Workspace

	// HTTP requests and associated data structures
	HTTPRequests       []mhttp.HTTP
	HTTPSearchParams   []mhttp.HTTPSearchParam
	HTTPHeaders        []mhttp.HTTPHeader
	HTTPBodyForms      []mhttp.HTTPBodyForm
	HTTPBodyUrlencoded []mhttp.HTTPBodyUrlencoded
	HTTPBodyRaw        []mhttp.HTTPBodyRaw
	HTTPAsserts        []mhttp.HTTPAssert

	// File organization
	Files []mfile.File

	// Flow structures
	Flows         []mflow.Flow
	FlowVariables []mflow.FlowVariable
	FlowNodes     []mflow.Node
	FlowEdges     []mflow.Edge

	// Flow node implementations by type
	FlowRequestNodes   []mflow.NodeRequest
	FlowConditionNodes []mflow.NodeIf
	FlowForNodes       []mflow.NodeFor
	FlowForEachNodes   []mflow.NodeForEach
	FlowJSNodes        []mflow.NodeJS

	// Environments and variables
	Environments    []menv.Env
	EnvironmentVars []menv.Variable
}

// CountEntities returns a map containing the count of each entity type in the bundle.
// Useful for logging, debugging, and displaying import/export statistics.
func (wb *WorkspaceBundle) CountEntities() map[string]int {
	return map[string]int{
		"http_requests":        len(wb.HTTPRequests),
		"http_search_params":   len(wb.HTTPSearchParams),
		"http_headers":         len(wb.HTTPHeaders),
		"http_body_forms":      len(wb.HTTPBodyForms),
		"http_body_urlencoded": len(wb.HTTPBodyUrlencoded),
		"http_body_raw":        len(wb.HTTPBodyRaw),
		"http_asserts":         len(wb.HTTPAsserts),
		"files":                len(wb.Files),
		"flows":                len(wb.Flows),
		"flow_variables":       len(wb.FlowVariables),
		"flow_nodes":           len(wb.FlowNodes),
		"flow_edges":           len(wb.FlowEdges),
		"flow_request_nodes":   len(wb.FlowRequestNodes),
		"flow_condition_nodes": len(wb.FlowConditionNodes),
		"flow_for_nodes":       len(wb.FlowForNodes),
		"flow_foreach_nodes":   len(wb.FlowForEachNodes),
		"flow_js_nodes":        len(wb.FlowJSNodes),
		"environments":         len(wb.Environments),
		"environment_vars":     len(wb.EnvironmentVars),
	}
}

// GetHTTPByID finds and returns an HTTP request by its ID.
// Returns nil if the HTTP request is not found.
func (wb *WorkspaceBundle) GetHTTPByID(id idwrap.IDWrap) *mhttp.HTTP {
	for i := range wb.HTTPRequests {
		if wb.HTTPRequests[i].ID.Compare(id) == 0 {
			return &wb.HTTPRequests[i]
		}
	}
	return nil
}

// GetFlowByID finds and returns a flow by its ID.
// Returns nil if the flow is not found.
func (wb *WorkspaceBundle) GetFlowByID(id idwrap.IDWrap) *mflow.Flow {
	for i := range wb.Flows {
		if wb.Flows[i].ID.Compare(id) == 0 {
			return &wb.Flows[i]
		}
	}
	return nil
}

// GetFlowByName finds and returns a flow by its name.
// Returns nil if the flow is not found.
func (wb *WorkspaceBundle) GetFlowByName(name string) *mflow.Flow {
	for i := range wb.Flows {
		if wb.Flows[i].Name == name {
			return &wb.Flows[i]
		}
	}
	return nil
}

// GetNodeByID finds and returns a flow node by its ID.
// Returns nil if the node is not found.
func (wb *WorkspaceBundle) GetNodeByID(id idwrap.IDWrap) *mflow.Node {
	for i := range wb.FlowNodes {
		if wb.FlowNodes[i].ID.Compare(id) == 0 {
			return &wb.FlowNodes[i]
		}
	}
	return nil
}

// GetFileByID finds and returns a file by its ID.
// Returns nil if the file is not found.
func (wb *WorkspaceBundle) GetFileByID(id idwrap.IDWrap) *mfile.File {
	for i := range wb.Files {
		if wb.Files[i].ID.Compare(id) == 0 {
			return &wb.Files[i]
		}
	}
	return nil
}

// GetFileByContentID finds and returns a file by its ContentID.
// Returns nil if no file is found with that content ID.
func (wb *WorkspaceBundle) GetFileByContentID(contentID idwrap.IDWrap) *mfile.File {
	for i := range wb.Files {
		if wb.Files[i].ContentID != nil && wb.Files[i].ContentID.Compare(contentID) == 0 {
			return &wb.Files[i]
		}
	}
	return nil
}

// GetEnvironmentByID finds and returns an environment by its ID.
// Returns nil if the environment is not found.
func (wb *WorkspaceBundle) GetEnvironmentByID(id idwrap.IDWrap) *menv.Env {
	for i := range wb.Environments {
		if wb.Environments[i].ID.Compare(id) == 0 {
			return &wb.Environments[i]
		}
	}
	return nil
}

// GetEnvironmentByName finds and returns an environment by its name.
// Returns nil if the environment is not found.
func (wb *WorkspaceBundle) GetEnvironmentByName(name string) *menv.Env {
	for i := range wb.Environments {
		if wb.Environments[i].Name == name {
			return &wb.Environments[i]
		}
	}
	return nil
}

// ImportOptions contains configuration options for workspace import operations.
type ImportOptions struct {
	// WorkspaceID is the target workspace ID for the import (required)
	WorkspaceID idwrap.IDWrap

	// ParentFolderID is the optional parent folder to import files under
	ParentFolderID *idwrap.IDWrap

	// CreateFiles determines whether to create file entries during import
	CreateFiles bool

	// MergeMode determines how to handle conflicts with existing entities
	// - "skip": Skip entities that already exist
	// - "replace": Replace existing entities with imported ones
	// - "create_new": Create new entities even if similar ones exist
	MergeMode string

	// PreserveIDs determines whether to preserve entity IDs from the source
	// If false, new IDs will be generated during import
	PreserveIDs bool

	// ImportHTTP determines whether to import HTTP requests
	ImportHTTP bool

	// ImportFlows determines whether to import flows
	ImportFlows bool

	// ImportEnvironments determines whether to import environments
	ImportEnvironments bool

	// StartOrder is the starting order value for imported files
	StartOrder float64
}

// ExportOptions contains configuration options for workspace export operations.
type ExportOptions struct {
	// WorkspaceID is the source workspace ID for the export (required)
	WorkspaceID idwrap.IDWrap

	// IncludeHTTP determines whether to include HTTP requests in the export
	IncludeHTTP bool

	// IncludeFlows determines whether to include flows in the export
	IncludeFlows bool

	// IncludeEnvironments determines whether to include environments in the export
	IncludeEnvironments bool

	// IncludeFiles determines whether to include file structure in the export
	IncludeFiles bool

	// ExportFormat specifies the output format (e.g., "json", "yaml", "zip")
	ExportFormat string

	// FilterByFolderID optionally filters export to a specific folder and its children
	FilterByFolderID *idwrap.IDWrap

	// FilterByFlowIDs optionally filters export to specific flows
	FilterByFlowIDs []idwrap.IDWrap

	// FilterByHTTPIDs optionally filters export to specific HTTP requests
	FilterByHTTPIDs []idwrap.IDWrap
}

// Validate validates the ImportOptions and returns an error if invalid.
func (opts ImportOptions) Validate() error {
	if opts.WorkspaceID.Compare(idwrap.IDWrap{}) == 0 {
		return ErrWorkspaceIDRequired
	}

	validMergeModes := map[string]bool{
		"skip":       true,
		"replace":    true,
		"create_new": true,
	}

	if opts.MergeMode != "" && !validMergeModes[opts.MergeMode] {
		return ErrInvalidMergeMode
	}

	return nil
}

// Validate validates the ExportOptions and returns an error if invalid.
func (opts ExportOptions) Validate() error {
	if opts.WorkspaceID.Compare(idwrap.IDWrap{}) == 0 {
		return ErrWorkspaceIDRequired
	}

	validFormats := map[string]bool{
		"json": true,
		"yaml": true,
		"zip":  true,
	}

	if opts.ExportFormat != "" && !validFormats[opts.ExportFormat] {
		return ErrInvalidExportFormat
	}

	return nil
}

// GetDefaultImportOptions returns ImportOptions with sensible defaults.
func GetDefaultImportOptions(workspaceID idwrap.IDWrap) ImportOptions {
	return ImportOptions{
		WorkspaceID:        workspaceID,
		ParentFolderID:     nil,
		CreateFiles:        true,
		MergeMode:          "create_new",
		PreserveIDs:        false,
		ImportHTTP:         true,
		ImportFlows:        true,
		ImportEnvironments: true,
		StartOrder:         0,
	}
}

// GetDefaultExportOptions returns ExportOptions with sensible defaults.
func GetDefaultExportOptions(workspaceID idwrap.IDWrap) ExportOptions {
	return ExportOptions{
		WorkspaceID:         workspaceID,
		IncludeHTTP:         true,
		IncludeFlows:        true,
		IncludeEnvironments: true,
		IncludeFiles:        true,
		ExportFormat:        "json",
		FilterByFolderID:    nil,
		FilterByFlowIDs:     nil,
		FilterByHTTPIDs:     nil,
	}
}
