package ioworkspace

import (
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcredential"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/menv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mexpect"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mload"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mwebsocket"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
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

	// GraphQL requests and associated data
	GraphQLRequests []mgraphql.GraphQL
	GraphQLHeaders  []mgraphql.GraphQLHeader
	GraphQLAsserts  []mgraphql.GraphQLAssert

	// WebSocket requests and associated data
	WebSockets       []mwebsocket.WebSocket
	WebSocketHeaders []mwebsocket.WebSocketHeader

	// File organization
	Files []mfile.File

	// Flow structures
	Flows         []mflow.Flow
	FlowVariables []mflow.FlowVariable
	FlowNodes     []mflow.Node
	FlowEdges     []mflow.Edge

	// Flow node implementations by type
	FlowRequestNodes        []mflow.NodeRequest
	FlowConditionNodes      []mflow.NodeIf
	FlowForNodes            []mflow.NodeFor
	FlowForEachNodes        []mflow.NodeForEach
	FlowJSNodes             []mflow.NodeJS
	FlowAINodes             []mflow.NodeAI
	FlowAIProviderNodes     []mflow.NodeAiProvider
	FlowAIMemoryNodes       []mflow.NodeMemory
	FlowGraphQLNodes        []mflow.NodeGraphQL
	FlowWsConnectionNodes   []mflow.NodeWsConnection
	FlowWsSendNodes         []mflow.NodeWsSend
	FlowWaitNodes           []mflow.NodeWait
	FlowSubFlowTriggerNodes []mflow.NodeSubFlowTrigger
	FlowSubFlowReturnNodes  []mflow.NodeSubFlowReturn
	FlowRunSubFlowNodes     []mflow.NodeRunSubFlow

	// Environments and variables
	Environments    []menv.Env
	EnvironmentVars []menv.Variable

	// Credentials (metadata only, secrets are never exported)
	Credentials []mcredential.Credential

	// LoadScenarios carries the yamlflow `load:` block so it survives the
	// YAML import -> export round trip.
	//
	// Unlike every other field here it is NOT database-backed: Import ignores
	// it and Export never populates it, because there is no storage for load
	// scenarios yet (Phase 2). Only the file-to-file path (the CLI and the
	// yamlflow translator) reads and writes it.
	LoadScenarios []mload.Scenario

	// FlowCleanups carries each flow's yamlflow `cleanup:` steps: steps that
	// run after the flow's normal steps finish, whether they passed or failed.
	//
	// Like LoadScenarios it is file-only: the cleanup entities live in each
	// FlowCleanup's own Bundle rather than in the slices above, so a regular
	// Import does not store them (and says so) and Export never populates
	// this field. Import stores them only when ImportOptions.ImportFlowCleanups
	// is set, which the CLI does so it can execute them.
	FlowCleanups []FlowCleanup

	// AIChecks carries the yamlflow AI checks: the file's and flows' judge:,
	// quality: and iterations:, and each step's expect: block (by flow node ID).
	// Import stores the flows' settings (with the file's folded in) and the
	// steps' blocks; Export reads them back as flow settings and steps. Expect
	// blocks of cleanup steps live in the cleanup's own Bundle.
	AIChecks *mexpect.Checks

	// HTTPStreams carries request steps' stream: settings, by HTTP request.
	HTTPStreams []mhttp.HTTPStream
}

// StepExpects returns every step's expect: block, cleanup steps included, by flow node ID.
func (wb *WorkspaceBundle) StepExpects() map[idwrap.IDWrap]mexpect.Expect {
	out := map[idwrap.IDWrap]mexpect.Expect{}
	add := func(b *WorkspaceBundle) {
		if b != nil && b.AIChecks != nil {
			for id, e := range b.AIChecks.Steps {
				out[id] = e
			}
		}
	}
	add(wb)
	for _, c := range wb.FlowCleanups {
		add(c.Bundle)
	}
	return out
}

// AllHTTPStreams returns every HTTP request's stream: settings, cleanup steps included, by
// HTTP request ID.
func (wb *WorkspaceBundle) AllHTTPStreams() map[idwrap.IDWrap]mhttp.HTTPStream {
	out := map[idwrap.IDWrap]mhttp.HTTPStream{}
	for _, s := range wb.HTTPStreams {
		out[s.HttpID] = s
	}
	for _, c := range wb.FlowCleanups {
		if c.Bundle != nil {
			for _, s := range c.Bundle.HTTPStreams {
				out[s.HttpID] = s
			}
		}
	}
	return out
}

// FlowCleanup is the `cleanup:` block of one flow.
//
// The cleanup steps are converted into nodes of a separate, hidden flow
// (CleanupFlowID) with no start node and no edges between them, so they can
// never run as part of the owning flow's graph. A runner executes them one by
// one, in Steps order, after the owning flow reaches a terminal state, reusing
// the owning flow's variable map so they can read step outputs.
type FlowCleanup struct {
	// FlowID is the flow that owns the cleanup block.
	FlowID idwrap.IDWrap
	// CleanupFlowID is the hidden flow holding the cleanup nodes. It is
	// Bundle.Flows[0].ID.
	CleanupFlowID idwrap.IDWrap
	// Steps lists the cleanup steps in execution order.
	Steps []FlowCleanupStep
	// Bundle holds the cleanup flow and every entity its nodes need (nodes,
	// HTTP and GraphQL requests and their children).
	Bundle *WorkspaceBundle
}

// FlowCleanupStep is one cleanup step, in execution order.
type FlowCleanupStep struct {
	NodeID idwrap.IDWrap
	Name   string
	// DependsOn names other cleanup steps of the same block. A step whose
	// dependency did not succeed is skipped.
	DependsOn []string
	// References names the steps (normal or cleanup) whose output this step's
	// templates read. A step referencing a step that produced no output (it
	// never ran) is skipped instead of being sent with an unresolved template.
	References []string
}

// CleanupFlowIDs returns the IDs of every hidden cleanup flow in the bundle.
func (wb *WorkspaceBundle) CleanupFlowIDs() map[idwrap.IDWrap]bool {
	ids := make(map[idwrap.IDWrap]bool, len(wb.FlowCleanups))
	for _, c := range wb.FlowCleanups {
		ids[c.CleanupFlowID] = true
	}
	return ids
}

// CleanupsByFlowID indexes the bundle's cleanup blocks by owning flow ID.
func (wb *WorkspaceBundle) CleanupsByFlowID() map[idwrap.IDWrap]FlowCleanup {
	byFlow := make(map[idwrap.IDWrap]FlowCleanup, len(wb.FlowCleanups))
	for _, c := range wb.FlowCleanups {
		byFlow[c.FlowID] = c
	}
	return byFlow
}

// CountEntities returns a map containing the count of each entity type in the bundle.
// Useful for logging, debugging, and displaying import/export statistics.
func (wb *WorkspaceBundle) CountEntities() map[string]int {
	return map[string]int{
		"http_requests":               len(wb.HTTPRequests),
		"http_search_params":          len(wb.HTTPSearchParams),
		"http_headers":                len(wb.HTTPHeaders),
		"http_body_forms":             len(wb.HTTPBodyForms),
		"http_body_urlencoded":        len(wb.HTTPBodyUrlencoded),
		"http_body_raw":               len(wb.HTTPBodyRaw),
		"http_asserts":                len(wb.HTTPAsserts),
		"graphql_requests":            len(wb.GraphQLRequests),
		"graphql_headers":             len(wb.GraphQLHeaders),
		"graphql_asserts":             len(wb.GraphQLAsserts),
		"websockets":                  len(wb.WebSockets),
		"websocket_headers":           len(wb.WebSocketHeaders),
		"files":                       len(wb.Files),
		"flows":                       len(wb.Flows),
		"flow_variables":              len(wb.FlowVariables),
		"flow_nodes":                  len(wb.FlowNodes),
		"flow_edges":                  len(wb.FlowEdges),
		"flow_request_nodes":          len(wb.FlowRequestNodes),
		"flow_condition_nodes":        len(wb.FlowConditionNodes),
		"flow_for_nodes":              len(wb.FlowForNodes),
		"flow_foreach_nodes":          len(wb.FlowForEachNodes),
		"flow_js_nodes":               len(wb.FlowJSNodes),
		"flow_ai_nodes":               len(wb.FlowAINodes),
		"flow_ai_provider_nodes":      len(wb.FlowAIProviderNodes),
		"flow_ai_memory_nodes":        len(wb.FlowAIMemoryNodes),
		"flow_graphql_nodes":          len(wb.FlowGraphQLNodes),
		"flow_ws_connection_nodes":    len(wb.FlowWsConnectionNodes),
		"flow_ws_send_nodes":          len(wb.FlowWsSendNodes),
		"flow_wait_nodes":             len(wb.FlowWaitNodes),
		"flow_sub_flow_trigger_nodes": len(wb.FlowSubFlowTriggerNodes),
		"flow_sub_flow_return_nodes":  len(wb.FlowSubFlowReturnNodes),
		"flow_run_sub_flow_nodes":     len(wb.FlowRunSubFlowNodes),
		"environments":                len(wb.Environments),
		"environment_vars":            len(wb.EnvironmentVars),
		"credentials":                 len(wb.Credentials),
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

// GetGraphQLByID finds and returns a GraphQL request by its ID.
// Returns nil if the GraphQL request is not found.
func (wb *WorkspaceBundle) GetGraphQLByID(id idwrap.IDWrap) *mgraphql.GraphQL {
	for i := range wb.GraphQLRequests {
		if wb.GraphQLRequests[i].ID.Compare(id) == 0 {
			return &wb.GraphQLRequests[i]
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

	// ImportFlowCleanups stores each FlowCleanup's hidden cleanup flow and its
	// entities, so a runner can build and execute the cleanup nodes. It
	// requires PreserveIDs, because FlowCleanup refers to nodes and flows by
	// their bundle IDs. When false (the desktop import path), cleanup blocks
	// are dropped with a warning.
	ImportFlowCleanups bool
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

	if opts.ImportFlowCleanups && !opts.PreserveIDs {
		return ErrFlowCleanupsNeedPreservedIDs
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
