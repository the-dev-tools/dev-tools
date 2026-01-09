//nolint:revive // exported
package ioworkspace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
)

// Export exports a workspace and all its entities to a WorkspaceBundle
func (s *IOWorkspaceService) Export(ctx context.Context, opts ExportOptions) (*WorkspaceBundle, error) {
	s.logger.InfoContext(ctx, "Starting workspace export",
		"workspace_id", opts.WorkspaceID.String(),
		"include_http", opts.IncludeHTTP,
		"include_flows", opts.IncludeFlows,
		"include_environments", opts.IncludeEnvironments,
		"include_files", opts.IncludeFiles)

	// Validate options
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid export options: %w", err)
	}

	bundle := &WorkspaceBundle{}

	// Get workspace metadata
	workspaceService := sworkspace.NewWorkspaceService(s.queries)
	workspace, err := workspaceService.Get(ctx, opts.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace: %w", err)
	}
	bundle.Workspace = *workspace

	// Export files if requested
	if opts.IncludeFiles {
		if err := s.exportFiles(ctx, opts, bundle); err != nil {
			return nil, fmt.Errorf("failed to export files: %w", err)
		}
	}

	// Export HTTP requests if requested
	if opts.IncludeHTTP {
		if err := s.exportHTTP(ctx, opts, bundle); err != nil {
			return nil, fmt.Errorf("failed to export HTTP requests: %w", err)
		}
	}

	// Export flows if requested
	if opts.IncludeFlows {
		if err := s.exportFlows(ctx, opts, bundle); err != nil {
			return nil, fmt.Errorf("failed to export flows: %w", err)
		}
	}

	// Export environments if requested
	if opts.IncludeEnvironments {
		if err := s.exportEnvironments(ctx, opts, bundle); err != nil {
			return nil, fmt.Errorf("failed to export environments: %w", err)
		}
	}

	counts := bundle.CountEntities()
	s.logger.InfoContext(ctx, "Workspace export completed", "counts", counts)

	return bundle, nil
}

// exportFiles exports file structure
func (s *IOWorkspaceService) exportFiles(ctx context.Context, opts ExportOptions, bundle *WorkspaceBundle) error {
	fileService := sfile.New(s.queries, s.logger)

	// If filtering by folder, get files from that folder, otherwise get all workspace files
	if opts.FilterByFolderID != nil {
		fileList, err := fileService.ListFilesByParent(ctx, opts.WorkspaceID, opts.FilterByFolderID)
		if err != nil {
			return fmt.Errorf("failed to get files by folder: %w", err)
		}
		bundle.Files = fileList
	} else {
		fileList, err := fileService.ListFilesByWorkspace(ctx, opts.WorkspaceID)
		if err != nil {
			return fmt.Errorf("failed to get files by workspace: %w", err)
		}
		bundle.Files = fileList
	}

	s.logger.DebugContext(ctx, "Exported files", "count", len(bundle.Files))
	return nil
}

// exportHTTP exports HTTP requests and all associated data
func (s *IOWorkspaceService) exportHTTP(ctx context.Context, opts ExportOptions, bundle *WorkspaceBundle) error {
	httpService := shttp.New(s.queries, s.logger)
	httpHeaderService := shttp.NewHttpHeaderService(s.queries)
	httpSearchParamSvc := shttp.NewHttpSearchParamService(s.queries)
	httpBodyFormSvc := shttp.NewHttpBodyFormService(s.queries)
	httpBodyUrlencodedSvc := shttp.NewHttpBodyUrlEncodedService(s.queries)
	httpBodyRawSvc := shttp.NewHttpBodyRawService(s.queries)
	httpAssertSvc := shttp.NewHttpAssertService(s.queries)

	var httpRequests []idwrap.IDWrap

	// Determine which HTTP requests to export
	if len(opts.FilterByHTTPIDs) > 0 {
		// Export specific HTTP requests
		for _, httpID := range opts.FilterByHTTPIDs {
			http, err := httpService.Get(ctx, httpID)
			if err != nil {
				return fmt.Errorf("failed to get HTTP request %s: %w", httpID.String(), err)
			}
			bundle.HTTPRequests = append(bundle.HTTPRequests, *http)
			httpRequests = append(httpRequests, httpID)
		}
	} else {
		// Export all HTTP requests in workspace (base requests)
		https, err := httpService.GetByWorkspaceID(ctx, opts.WorkspaceID)
		if err != nil {
			return fmt.Errorf("failed to get HTTP requests: %w", err)
		}
		bundle.HTTPRequests = https
		for _, http := range https {
			httpRequests = append(httpRequests, http.ID)
		}

		// Also export delta HTTP requests
		deltaHttps, err := httpService.GetDeltasByWorkspaceID(ctx, opts.WorkspaceID)
		if err != nil {
			return fmt.Errorf("failed to get delta HTTP requests: %w", err)
		}
		bundle.HTTPRequests = append(bundle.HTTPRequests, deltaHttps...)
		for _, deltaHttp := range deltaHttps {
			httpRequests = append(httpRequests, deltaHttp.ID)
		}
	}

	s.logger.DebugContext(ctx, "Exported HTTP requests", "count", len(bundle.HTTPRequests))

	// Export all HTTP-related data for each request
	for _, httpID := range httpRequests {
		// Export headers
		headers, err := httpHeaderService.GetByHttpID(ctx, httpID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to get headers for HTTP %s: %w", httpID.String(), err)
		}
		bundle.HTTPHeaders = append(bundle.HTTPHeaders, headers...)

		// Export search params
		searchParams, err := httpSearchParamSvc.GetByHttpID(ctx, httpID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to get search params for HTTP %s: %w", httpID.String(), err)
		}
		bundle.HTTPSearchParams = append(bundle.HTTPSearchParams, searchParams...)

		// Export body forms
		bodyForms, err := httpBodyFormSvc.GetByHttpID(ctx, httpID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to get body forms for HTTP %s: %w", httpID.String(), err)
		}
		bundle.HTTPBodyForms = append(bundle.HTTPBodyForms, bodyForms...)

		// Export body urlencoded
		bodyUrlencoded, err := httpBodyUrlencodedSvc.GetByHttpID(ctx, httpID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to get body urlencoded for HTTP %s: %w", httpID.String(), err)
		}
		bundle.HTTPBodyUrlencoded = append(bundle.HTTPBodyUrlencoded, bodyUrlencoded...)

		// Export body raw (may not exist for all HTTP requests)
		bodyRaw, err := httpBodyRawSvc.GetByHttpID(ctx, httpID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) && !errors.Is(err, shttp.ErrNoHttpBodyRawFound) {
			return fmt.Errorf("failed to get body raw for HTTP %s: %w", httpID.String(), err)
		}
		if bodyRaw != nil {
			bundle.HTTPBodyRaw = append(bundle.HTTPBodyRaw, *bodyRaw)
		}

		// Export asserts
		asserts, err := httpAssertSvc.GetByHttpID(ctx, httpID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to get asserts for HTTP %s: %w", httpID.String(), err)
		}
		bundle.HTTPAsserts = append(bundle.HTTPAsserts, asserts...)
	}

	s.logger.DebugContext(ctx, "Exported HTTP details",
		"headers", len(bundle.HTTPHeaders),
		"search_params", len(bundle.HTTPSearchParams),
		"body_forms", len(bundle.HTTPBodyForms),
		"body_urlencoded", len(bundle.HTTPBodyUrlencoded),
		"body_raw", len(bundle.HTTPBodyRaw),
		"asserts", len(bundle.HTTPAsserts))

	return nil
}

// exportFlows exports flows and all associated data (nodes, edges, variables, node implementations)
func (s *IOWorkspaceService) exportFlows(ctx context.Context, opts ExportOptions, bundle *WorkspaceBundle) error {
	flowService := sflow.NewFlowService(s.queries)
	flowVariableService := sflow.NewFlowVariableService(s.queries)
	nodeService := sflow.NewNodeService(s.queries)
	edgeService := sflow.NewEdgeService(s.queries)
	nodeRequestService := sflow.NewNodeRequestService(s.queries)
	nodeIfService := sflow.NewNodeIfService(s.queries)
	nodeForService := sflow.NewNodeForService(s.queries)
	nodeForEachService := sflow.NewNodeForEachService(s.queries)
	nodeJSService := sflow.NewNodeJsService(s.queries)

	var flowIDs []idwrap.IDWrap

	// Determine which flows to export
	if len(opts.FilterByFlowIDs) > 0 {
		// Export specific flows
		for _, flowID := range opts.FilterByFlowIDs {
			flow, err := flowService.GetFlow(ctx, flowID)
			if err != nil {
				return fmt.Errorf("failed to get flow %s: %w", flowID.String(), err)
			}
			bundle.Flows = append(bundle.Flows, flow)
			flowIDs = append(flowIDs, flowID)
		}
	} else {
		// Export all flows in workspace
		flows, err := flowService.GetFlowsByWorkspaceID(ctx, opts.WorkspaceID)
		if err != nil {
			return fmt.Errorf("failed to get flows: %w", err)
		}
		bundle.Flows = flows
		for _, flow := range flows {
			flowIDs = append(flowIDs, flow.ID)
		}
	}

	s.logger.DebugContext(ctx, "Exported flows", "count", len(bundle.Flows))

	// Export flow details for each flow
	for _, flowID := range flowIDs {
		// Export flow variables
		flowVars, err := flowVariableService.GetFlowVariablesByFlowID(ctx, flowID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to get flow variables for flow %s: %w", flowID.String(), err)
		}
		bundle.FlowVariables = append(bundle.FlowVariables, flowVars...)

		// Export nodes
		nodes, err := nodeService.GetNodesByFlowID(ctx, flowID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to get nodes for flow %s: %w", flowID.String(), err)
		}
		bundle.FlowNodes = append(bundle.FlowNodes, nodes...)

		// Export edges
		edges, err := edgeService.GetEdgesByFlowID(ctx, flowID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to get edges for flow %s: %w", flowID.String(), err)
		}
		bundle.FlowEdges = append(bundle.FlowEdges, edges...)

		// Export node implementations based on node types
		for _, node := range nodes {
			if err := s.exportNodeImplementation(ctx, node, bundle, nodeRequestService, nodeIfService, nodeForService, nodeForEachService, nodeJSService); err != nil {
				return fmt.Errorf("failed to export node implementation for node %s: %w", node.ID.String(), err)
			}
		}
	}

	s.logger.DebugContext(ctx, "Exported flow details",
		"variables", len(bundle.FlowVariables),
		"nodes", len(bundle.FlowNodes),
		"edges", len(bundle.FlowEdges),
		"request_nodes", len(bundle.FlowRequestNodes),
		"condition_nodes", len(bundle.FlowConditionNodes),
		"for_nodes", len(bundle.FlowForNodes),
		"foreach_nodes", len(bundle.FlowForEachNodes),
		"js_nodes", len(bundle.FlowJSNodes))

	return nil
}

// exportNodeImplementation exports the specific implementation for a node based on its type
func (s *IOWorkspaceService) exportNodeImplementation(
	ctx context.Context,
	node mflow.Node,
	bundle *WorkspaceBundle,
	nodeRequestService sflow.NodeRequestService,
	nodeIfService *sflow.NodeIfService,
	nodeForService sflow.NodeForService,
	nodeForEachService sflow.NodeForEachService,
	nodeJSService sflow.NodeJsService,
) error {
	switch node.NodeKind {
	case mflow.NODE_KIND_REQUEST:
		nodeRequest, err := nodeRequestService.GetNodeRequest(ctx, node.ID)
		if err != nil {
			return fmt.Errorf("failed to get request node: %w", err)
		}
		if nodeRequest != nil {
			bundle.FlowRequestNodes = append(bundle.FlowRequestNodes, *nodeRequest)
		}

	case mflow.NODE_KIND_CONDITION:
		nodeIf, err := nodeIfService.GetNodeIf(ctx, node.ID)
		if err != nil {
			return fmt.Errorf("failed to get if node: %w", err)
		}
		if nodeIf != nil {
			bundle.FlowConditionNodes = append(bundle.FlowConditionNodes, *nodeIf)
		}

	case mflow.NODE_KIND_FOR:
		nodeFor, err := nodeForService.GetNodeFor(ctx, node.ID)
		if err != nil {
			return fmt.Errorf("failed to get for node: %w", err)
		}
		if nodeFor != nil {
			bundle.FlowForNodes = append(bundle.FlowForNodes, *nodeFor)
		}

	case mflow.NODE_KIND_FOR_EACH:
		nodeForEach, err := nodeForEachService.GetNodeForEach(ctx, node.ID)
		if err != nil {
			return fmt.Errorf("failed to get foreach node: %w", err)
		}
		if nodeForEach != nil {
			bundle.FlowForEachNodes = append(bundle.FlowForEachNodes, *nodeForEach)
		}

	case mflow.NODE_KIND_JS:
		nodeJS, err := nodeJSService.GetNodeJS(ctx, node.ID)
		if err != nil {
			return fmt.Errorf("failed to get js node: %w", err)
		}
		if nodeJS != nil {
			bundle.FlowJSNodes = append(bundle.FlowJSNodes, *nodeJS)
		}
	}

	return nil
}

// exportEnvironments exports environments and their variables
func (s *IOWorkspaceService) exportEnvironments(ctx context.Context, opts ExportOptions, bundle *WorkspaceBundle) error {
	envService := senv.NewEnvironmentService(s.queries, s.logger)
	varService := senv.NewVariableService(s.queries, s.logger)

	// Export all environments in workspace
	envs, err := envService.ListEnvironments(ctx, opts.WorkspaceID)
	if err != nil {
		return fmt.Errorf("failed to get environments: %w", err)
	}
	bundle.Environments = envs

	s.logger.DebugContext(ctx, "Exported environments", "count", len(bundle.Environments))

	// Export variables for each environment
	for _, env := range envs {
		vars, err := varService.GetVariableByEnvID(ctx, env.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to get variables for env %s: %w", env.ID.String(), err)
		}
		bundle.EnvironmentVars = append(bundle.EnvironmentVars, vars...)
	}

	s.logger.DebugContext(ctx, "Exported environment variables", "count", len(bundle.EnvironmentVars))

	return nil
}
