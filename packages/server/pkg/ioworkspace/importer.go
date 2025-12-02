package ioworkspace

import (
	"context"
	"database/sql"
	"fmt"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/svar"
)

// ImportResult contains statistics and mappings from the import operation.
type ImportResult struct {
	// Entity counts
	HTTPRequestsCreated       int
	HTTPSearchParamsCreated   int
	HTTPHeadersCreated        int
	HTTPBodyFormsCreated      int
	HTTPBodyUrlencodedCreated int
	HTTPBodyRawCreated        int
	HTTPAssertsCreated        int
	FilesCreated              int
	FlowsCreated              int
	FlowVariablesCreated      int
	FlowNodesCreated          int
	FlowEdgesCreated          int
	FlowRequestNodesCreated   int
	FlowConditionNodesCreated int
	FlowNoopNodesCreated      int
	FlowForNodesCreated       int
	FlowForEachNodesCreated   int
	FlowJSNodesCreated        int
	EnvironmentsCreated       int
	EnvironmentVarsCreated    int

	// ID mappings for reference (old ID -> new ID)
	HTTPIDMap        map[idwrap.IDWrap]idwrap.IDWrap
	FlowIDMap        map[idwrap.IDWrap]idwrap.IDWrap
	NodeIDMap        map[idwrap.IDWrap]idwrap.IDWrap
	FileIDMap        map[idwrap.IDWrap]idwrap.IDWrap
	EnvironmentIDMap map[idwrap.IDWrap]idwrap.IDWrap
}

// Import imports a WorkspaceBundle into the database using the provided options.
// This operation should be performed within a transaction for atomicity.
func (s *IOWorkspaceService) Import(ctx context.Context, tx *sql.Tx, bundle *WorkspaceBundle, opts ImportOptions) (*ImportResult, error) {
	// Validate options
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid import options: %w", err)
	}

	// Initialize result
	result := &ImportResult{
		HTTPIDMap:        make(map[idwrap.IDWrap]idwrap.IDWrap),
		FlowIDMap:        make(map[idwrap.IDWrap]idwrap.IDWrap),
		NodeIDMap:        make(map[idwrap.IDWrap]idwrap.IDWrap),
		FileIDMap:        make(map[idwrap.IDWrap]idwrap.IDWrap),
		EnvironmentIDMap: make(map[idwrap.IDWrap]idwrap.IDWrap),
	}

	// Create service instances with transaction support
	httpService := shttp.New(s.queries, nil).TX(tx)
	httpHeaderService := shttp.NewHttpHeaderService(s.queries).TX(tx)
	httpSearchParamService := shttp.NewHttpSearchParamService(s.queries).TX(tx)
	httpBodyFormService := shttp.NewHttpBodyFormService(s.queries).TX(tx)
	httpBodyUrlencodedService := shttp.NewHttpBodyUrlEncodedService(s.queries).TX(tx)
	httpBodyRawService := shttp.NewHttpBodyRawService(s.queries).TX(tx)
	httpAssertService := shttp.NewHttpAssertService(s.queries).TX(tx)

	flowService := sflow.New(s.queries).TX(tx)
	flowVariableService := sflowvariable.New(s.queries).TX(tx)
	nodeService := snode.New(s.queries).TX(tx)
	edgeService := sedge.New(s.queries).TX(tx)

	nodeRequestService := snoderequest.New(s.queries).TX(tx)
	nodeIfService := snodeif.New(s.queries).TX(tx)
	nodeNoopService := snodenoop.New(s.queries).TX(tx)
	nodeForService := snodefor.New(s.queries).TX(tx)
	nodeForEachService := snodeforeach.New(s.queries).TX(tx)
	nodeJSService := snodejs.New(s.queries).TX(tx)

	fileService := sfile.New(s.queries, nil).TX(tx)
	envService := senv.New(s.queries, nil).TX(tx)
	varService := svar.New(s.queries, nil).TX(tx)

	// Layer 0: Flows (no dependencies)
	if opts.ImportFlows && len(bundle.Flows) > 0 {
		if err := s.importFlows(ctx, flowService, bundle, opts, result); err != nil {
			return nil, fmt.Errorf("failed to import flows: %w", err)
		}
	}

	// Layer 1: HTTP requests, Files, Environments
	if opts.ImportHTTP && len(bundle.HTTPRequests) > 0 {
		if err := s.importHTTPRequests(ctx, httpService, bundle, opts, result); err != nil {
			return nil, fmt.Errorf("failed to import HTTP requests: %w", err)
		}
	}

	if opts.CreateFiles && len(bundle.Files) > 0 {
		if err := s.importFiles(ctx, fileService, bundle, opts, result); err != nil {
			return nil, fmt.Errorf("failed to import files: %w", err)
		}
	}

	if opts.ImportEnvironments && len(bundle.Environments) > 0 {
		if err := s.importEnvironments(ctx, envService, bundle, opts, result); err != nil {
			return nil, fmt.Errorf("failed to import environments: %w", err)
		}
	}

	// Layer 2: Flow variables, Flow nodes, HTTP sub-entities
	if opts.ImportFlows {
		if len(bundle.FlowVariables) > 0 {
			if err := s.importFlowVariables(ctx, flowVariableService, bundle, opts, result); err != nil {
				return nil, fmt.Errorf("failed to import flow variables: %w", err)
			}
		}

		if len(bundle.FlowNodes) > 0 {
			if err := s.importFlowNodes(ctx, nodeService, bundle, opts, result); err != nil {
				return nil, fmt.Errorf("failed to import flow nodes: %w", err)
			}
		}
	}

	if opts.ImportHTTP {
		if len(bundle.HTTPHeaders) > 0 {
			if err := s.importHTTPHeaders(ctx, httpHeaderService, bundle, opts, result); err != nil {
				return nil, fmt.Errorf("failed to import HTTP headers: %w", err)
			}
		}

		if len(bundle.HTTPSearchParams) > 0 {
			if err := s.importHTTPSearchParams(ctx, httpSearchParamService, bundle, opts, result); err != nil {
				return nil, fmt.Errorf("failed to import HTTP search params: %w", err)
			}
		}

		if len(bundle.HTTPBodyForms) > 0 {
			if err := s.importHTTPBodyForms(ctx, httpBodyFormService, bundle, opts, result); err != nil {
				return nil, fmt.Errorf("failed to import HTTP body forms: %w", err)
			}
		}

		if len(bundle.HTTPBodyUrlencoded) > 0 {
			if err := s.importHTTPBodyUrlencoded(ctx, httpBodyUrlencodedService, bundle, opts, result); err != nil {
				return nil, fmt.Errorf("failed to import HTTP body urlencoded: %w", err)
			}
		}

		if len(bundle.HTTPBodyRaw) > 0 {
			if err := s.importHTTPBodyRaw(ctx, httpBodyRawService, bundle, opts, result); err != nil {
				return nil, fmt.Errorf("failed to import HTTP body raw: %w", err)
			}
		}

		if len(bundle.HTTPAsserts) > 0 {
			if err := s.importHTTPAsserts(ctx, httpAssertService, bundle, opts, result); err != nil {
				return nil, fmt.Errorf("failed to import HTTP asserts: %w", err)
			}
		}
	}

	if opts.ImportEnvironments && len(bundle.EnvironmentVars) > 0 {
		if err := s.importEnvironmentVars(ctx, varService, bundle, opts, result); err != nil {
			return nil, fmt.Errorf("failed to import environment vars: %w", err)
		}
	}

	// Layer 3: Flow edges and node implementations
	if opts.ImportFlows {
		if len(bundle.FlowEdges) > 0 {
			if err := s.importFlowEdges(ctx, edgeService, bundle, opts, result); err != nil {
				return nil, fmt.Errorf("failed to import flow edges: %w", err)
			}
		}

		// Import node implementations
		if len(bundle.FlowRequestNodes) > 0 {
			if err := s.importFlowRequestNodes(ctx, nodeRequestService, bundle, opts, result); err != nil {
				return nil, fmt.Errorf("failed to import flow request nodes: %w", err)
			}
		}

		if len(bundle.FlowConditionNodes) > 0 {
			if err := s.importFlowConditionNodes(ctx, nodeIfService, bundle, opts, result); err != nil {
				return nil, fmt.Errorf("failed to import flow condition nodes: %w", err)
			}
		}

		if len(bundle.FlowNoopNodes) > 0 {
			if err := s.importFlowNoopNodes(ctx, nodeNoopService, bundle, opts, result); err != nil {
				return nil, fmt.Errorf("failed to import flow noop nodes: %w", err)
			}
		}

		if len(bundle.FlowForNodes) > 0 {
			if err := s.importFlowForNodes(ctx, nodeForService, bundle, opts, result); err != nil {
				return nil, fmt.Errorf("failed to import flow for nodes: %w", err)
			}
		}

		if len(bundle.FlowForEachNodes) > 0 {
			if err := s.importFlowForEachNodes(ctx, nodeForEachService, bundle, opts, result); err != nil {
				return nil, fmt.Errorf("failed to import flow foreach nodes: %w", err)
			}
		}

		if len(bundle.FlowJSNodes) > 0 {
			if err := s.importFlowJSNodes(ctx, nodeJSService, bundle, opts, result); err != nil {
				return nil, fmt.Errorf("failed to import flow JS nodes: %w", err)
			}
		}
	}

	return result, nil
}

// importFlows imports flows from the bundle.
func (s *IOWorkspaceService) importFlows(ctx context.Context, flowService sflow.FlowService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, flow := range bundle.Flows {
		oldID := flow.ID

		// Generate new ID if not preserving
		if !opts.PreserveIDs {
			flow.ID = idwrap.NewNow()
		}

		// Update workspace ID
		flow.WorkspaceID = opts.WorkspaceID

		// Update version parent ID if it exists in the mapping
		if flow.VersionParentID != nil {
			if newParentID, ok := result.FlowIDMap[*flow.VersionParentID]; ok {
				flow.VersionParentID = &newParentID
			}
		}

		// Create flow
		if err := flowService.CreateFlow(ctx, flow); err != nil {
			return fmt.Errorf("failed to create flow %s: %w", flow.Name, err)
		}

		// Track ID mapping
		result.FlowIDMap[oldID] = flow.ID
		result.FlowsCreated++
	}
	return nil
}

// importHTTPRequests imports HTTP requests from the bundle.
func (s *IOWorkspaceService) importHTTPRequests(ctx context.Context, httpService shttp.HTTPService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, http := range bundle.HTTPRequests {
		oldID := http.ID

		// Generate new ID if not preserving
		if !opts.PreserveIDs {
			http.ID = idwrap.NewNow()
		}

		// Update workspace ID
		http.WorkspaceID = opts.WorkspaceID

		// Update folder ID if specified
		if opts.ParentFolderID != nil {
			http.FolderID = opts.ParentFolderID
		} else if http.FolderID != nil {
			// Remap folder ID if it exists in file mapping
			if newFolderID, ok := result.FileIDMap[*http.FolderID]; ok {
				http.FolderID = &newFolderID
			}
		}

		// Update parent HTTP ID if it exists in the mapping (for deltas)
		if http.ParentHttpID != nil {
			if newParentID, ok := result.HTTPIDMap[*http.ParentHttpID]; ok {
				http.ParentHttpID = &newParentID
			}
		}

		// Create HTTP request
		if err := httpService.Create(ctx, &http); err != nil {
			return fmt.Errorf("failed to create HTTP request %s: %w", http.Name, err)
		}

		// Track ID mapping
		result.HTTPIDMap[oldID] = http.ID
		result.HTTPRequestsCreated++
	}
	return nil
}

// importFiles imports files from the bundle.
func (s *IOWorkspaceService) importFiles(ctx context.Context, fileService *sfile.FileService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, file := range bundle.Files {
		oldID := file.ID

		// Generate new ID if not preserving
		if !opts.PreserveIDs {
			file.ID = idwrap.NewNow()
		}

		// Update workspace ID
		file.WorkspaceID = opts.WorkspaceID

		// Update parent folder ID
		if opts.ParentFolderID != nil {
			file.ParentID = opts.ParentFolderID
		} else if file.ParentID != nil {
			// Remap parent ID if it exists in file mapping
			if newParentID, ok := result.FileIDMap[*file.ParentID]; ok {
				file.ParentID = &newParentID
			}
		}

		// Update content ID references (HTTP or Flow)
		if file.ContentID != nil {
			if newContentID, ok := result.HTTPIDMap[*file.ContentID]; ok {
				file.ContentID = &newContentID
			} else if newContentID, ok := result.FlowIDMap[*file.ContentID]; ok {
				file.ContentID = &newContentID
			}
		}

		// Adjust order if needed
		if opts.StartOrder > 0 {
			file.Order = opts.StartOrder + file.Order
		}

		// Create file
		if err := fileService.CreateFile(ctx, &file); err != nil {
			return fmt.Errorf("failed to create file %s: %w", file.Name, err)
		}

		// Track ID mapping
		result.FileIDMap[oldID] = file.ID
		result.FilesCreated++
	}
	return nil
}

// importEnvironments imports environments from the bundle.
func (s *IOWorkspaceService) importEnvironments(ctx context.Context, envService senv.EnvironmentService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, env := range bundle.Environments {
		oldID := env.ID

		// Generate new ID if not preserving
		if !opts.PreserveIDs {
			env.ID = idwrap.NewNow()
		}

		// Update workspace ID
		env.WorkspaceID = opts.WorkspaceID

		// Create environment
		if err := envService.CreateEnvironment(ctx, &env); err != nil {
			return fmt.Errorf("failed to create environment %s: %w", env.Name, err)
		}

		// Track ID mapping
		result.EnvironmentIDMap[oldID] = env.ID
		result.EnvironmentsCreated++
	}
	return nil
}

// importFlowVariables imports flow variables from the bundle.
func (s *IOWorkspaceService) importFlowVariables(ctx context.Context, flowVariableService sflowvariable.FlowVariableService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, flowVar := range bundle.FlowVariables {
		// Generate new ID if not preserving
		if !opts.PreserveIDs {
			flowVar.ID = idwrap.NewNow()
		}

		// Remap flow ID
		if newFlowID, ok := result.FlowIDMap[flowVar.FlowID]; ok {
			flowVar.FlowID = newFlowID
		}

		// Create flow variable
		if err := flowVariableService.CreateFlowVariable(ctx, flowVar); err != nil {
			return fmt.Errorf("failed to create flow variable %s: %w", flowVar.Name, err)
		}

		result.FlowVariablesCreated++
	}
	return nil
}

// importFlowNodes imports flow nodes from the bundle.
func (s *IOWorkspaceService) importFlowNodes(ctx context.Context, nodeService snode.NodeService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, node := range bundle.FlowNodes {
		oldID := node.ID

		// Generate new ID if not preserving
		if !opts.PreserveIDs {
			node.ID = idwrap.NewNow()
		}

		// Remap flow ID
		if newFlowID, ok := result.FlowIDMap[node.FlowID]; ok {
			node.FlowID = newFlowID
		}

		// Create node
		if err := nodeService.CreateNode(ctx, node); err != nil {
			return fmt.Errorf("failed to create node %s: %w", node.Name, err)
		}

		// Track ID mapping
		result.NodeIDMap[oldID] = node.ID
		result.FlowNodesCreated++
	}
	return nil
}

// importHTTPHeaders imports HTTP headers from the bundle.
func (s *IOWorkspaceService) importHTTPHeaders(ctx context.Context, headerService shttp.HttpHeaderService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, header := range bundle.HTTPHeaders {
		// Generate new ID if not preserving
		if !opts.PreserveIDs {
			header.ID = idwrap.NewNow()
		}

		// Remap HTTP ID
		if newHTTPID, ok := result.HTTPIDMap[header.HttpID]; ok {
			header.HttpID = newHTTPID
		}

		// Remap parent header ID if it exists in the mapping
		if header.ParentHttpHeaderID != nil {
			// Note: We'd need to track header ID mappings for this to work properly
			// For now, we'll clear parent references on import
			header.ParentHttpHeaderID = nil
			header.IsDelta = false
		}

		// Create header
		if err := headerService.Create(ctx, &header); err != nil {
			return fmt.Errorf("failed to create HTTP header: %w", err)
		}

		result.HTTPHeadersCreated++
	}
	return nil
}

// importHTTPSearchParams imports HTTP search params from the bundle.
func (s *IOWorkspaceService) importHTTPSearchParams(ctx context.Context, searchParamService *shttp.HttpSearchParamService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, param := range bundle.HTTPSearchParams {
		// Generate new ID if not preserving
		if !opts.PreserveIDs {
			param.ID = idwrap.NewNow()
		}

		// Remap HTTP ID
		if newHTTPID, ok := result.HTTPIDMap[param.HttpID]; ok {
			param.HttpID = newHTTPID
		}

		// Clear parent references (similar to headers)
		if param.ParentHttpSearchParamID != nil {
			param.ParentHttpSearchParamID = nil
			param.IsDelta = false
		}

		// Create search param
		if err := searchParamService.Create(ctx, &param); err != nil {
			return fmt.Errorf("failed to create HTTP search param: %w", err)
		}

		result.HTTPSearchParamsCreated++
	}
	return nil
}

// importHTTPBodyForms imports HTTP body forms from the bundle.
func (s *IOWorkspaceService) importHTTPBodyForms(ctx context.Context, bodyFormService *shttp.HttpBodyFormService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, bodyForm := range bundle.HTTPBodyForms {
		// Generate new ID if not preserving
		if !opts.PreserveIDs {
			bodyForm.ID = idwrap.NewNow()
		}

		// Remap HTTP ID
		if newHTTPID, ok := result.HTTPIDMap[bodyForm.HttpID]; ok {
			bodyForm.HttpID = newHTTPID
		}

		// Clear parent references
		if bodyForm.ParentHttpBodyFormID != nil {
			bodyForm.ParentHttpBodyFormID = nil
			bodyForm.IsDelta = false
		}

		// Create body form
		if err := bodyFormService.Create(ctx, &bodyForm); err != nil {
			return fmt.Errorf("failed to create HTTP body form: %w", err)
		}

		result.HTTPBodyFormsCreated++
	}
	return nil
}

// importHTTPBodyUrlencoded imports HTTP body urlencoded from the bundle.
func (s *IOWorkspaceService) importHTTPBodyUrlencoded(ctx context.Context, bodyUrlencodedService *shttp.HttpBodyUrlEncodedService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, bodyUrlencoded := range bundle.HTTPBodyUrlencoded {
		// Generate new ID if not preserving
		if !opts.PreserveIDs {
			bodyUrlencoded.ID = idwrap.NewNow()
		}

		// Remap HTTP ID
		if newHTTPID, ok := result.HTTPIDMap[bodyUrlencoded.HttpID]; ok {
			bodyUrlencoded.HttpID = newHTTPID
		}

		// Clear parent references
		if bodyUrlencoded.ParentHttpBodyUrlEncodedID != nil {
			bodyUrlencoded.ParentHttpBodyUrlEncodedID = nil
			bodyUrlencoded.IsDelta = false
		}

		// Create body urlencoded
		if err := bodyUrlencodedService.Create(ctx, &bodyUrlencoded); err != nil {
			return fmt.Errorf("failed to create HTTP body urlencoded: %w", err)
		}

		result.HTTPBodyUrlencodedCreated++
	}
	return nil
}

// importHTTPBodyRaw imports HTTP body raw from the bundle.
func (s *IOWorkspaceService) importHTTPBodyRaw(ctx context.Context, bodyRawService *shttp.HttpBodyRawService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, bodyRaw := range bundle.HTTPBodyRaw {
		// Remap HTTP ID
		newHTTPID := bodyRaw.HttpID
		if mappedID, ok := result.HTTPIDMap[bodyRaw.HttpID]; ok {
			newHTTPID = mappedID
		}

		// Create body raw using the service's Create method signature
		_, err := bodyRawService.Create(ctx, newHTTPID, bodyRaw.RawData, bodyRaw.ContentType)
		if err != nil {
			return fmt.Errorf("failed to create HTTP body raw: %w", err)
		}

		result.HTTPBodyRawCreated++
	}
	return nil
}

// importHTTPAsserts imports HTTP asserts from the bundle.
func (s *IOWorkspaceService) importHTTPAsserts(ctx context.Context, assertService *shttp.HttpAssertService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, assert := range bundle.HTTPAsserts {
		// Generate new ID if not preserving
		if !opts.PreserveIDs {
			assert.ID = idwrap.NewNow()
		}

		// Remap HTTP ID
		if newHTTPID, ok := result.HTTPIDMap[assert.HttpID]; ok {
			assert.HttpID = newHTTPID
		}

		// Create assert
		if err := assertService.Create(ctx, &assert); err != nil {
			return fmt.Errorf("failed to create HTTP assert: %w", err)
		}

		result.HTTPAssertsCreated++
	}
	return nil
}

// importEnvironmentVars imports environment variables from the bundle.
func (s *IOWorkspaceService) importEnvironmentVars(ctx context.Context, varService svar.VarService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, envVar := range bundle.EnvironmentVars {
		// Generate new ID if not preserving
		if !opts.PreserveIDs {
			envVar.ID = idwrap.NewNow()
		}

		// Remap environment ID
		if newEnvID, ok := result.EnvironmentIDMap[envVar.EnvID]; ok {
			envVar.EnvID = newEnvID
		}

		// Create environment variable
		if err := varService.Create(ctx, envVar); err != nil {
			return fmt.Errorf("failed to create environment variable %s: %w", envVar.VarKey, err)
		}

		result.EnvironmentVarsCreated++
	}
	return nil
}

// importFlowEdges imports flow edges from the bundle.
func (s *IOWorkspaceService) importFlowEdges(ctx context.Context, edgeService sedge.EdgeService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, edge := range bundle.FlowEdges {
		// Generate new ID if not preserving
		if !opts.PreserveIDs {
			edge.ID = idwrap.NewNow()
		}

		// Remap flow ID
		if newFlowID, ok := result.FlowIDMap[edge.FlowID]; ok {
			edge.FlowID = newFlowID
		}

		// Remap source and target node IDs
		if newSourceID, ok := result.NodeIDMap[edge.SourceID]; ok {
			edge.SourceID = newSourceID
		}
		if newTargetID, ok := result.NodeIDMap[edge.TargetID]; ok {
			edge.TargetID = newTargetID
		}

		// Create edge
		if err := edgeService.CreateEdge(ctx, edge); err != nil {
			return fmt.Errorf("failed to create flow edge: %w", err)
		}

		result.FlowEdgesCreated++
	}
	return nil
}

// importFlowRequestNodes imports flow request nodes from the bundle.
func (s *IOWorkspaceService) importFlowRequestNodes(ctx context.Context, nodeRequestService snoderequest.NodeRequestService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, requestNode := range bundle.FlowRequestNodes {
		// Remap flow node ID
		if newNodeID, ok := result.NodeIDMap[requestNode.FlowNodeID]; ok {
			requestNode.FlowNodeID = newNodeID
		}

		// Remap HTTP ID
		if requestNode.HttpID != nil {
			if newHTTPID, ok := result.HTTPIDMap[*requestNode.HttpID]; ok {
				requestNode.HttpID = &newHTTPID
			}
		}

		// Remap delta HTTP ID
		if requestNode.DeltaHttpID != nil {
			if newDeltaHTTPID, ok := result.HTTPIDMap[*requestNode.DeltaHttpID]; ok {
				requestNode.DeltaHttpID = &newDeltaHTTPID
			}
		}

		// Create request node
		if err := nodeRequestService.CreateNodeRequest(ctx, requestNode); err != nil {
			return fmt.Errorf("failed to create flow request node: %w", err)
		}

		result.FlowRequestNodesCreated++
	}
	return nil
}

// importFlowConditionNodes imports flow condition nodes from the bundle.
func (s *IOWorkspaceService) importFlowConditionNodes(ctx context.Context, nodeIfService *snodeif.NodeIfService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, conditionNode := range bundle.FlowConditionNodes {
		// Remap flow node ID
		if newNodeID, ok := result.NodeIDMap[conditionNode.FlowNodeID]; ok {
			conditionNode.FlowNodeID = newNodeID
		}

		// Create condition node
		if err := nodeIfService.CreateNodeIf(ctx, conditionNode); err != nil {
			return fmt.Errorf("failed to create flow condition node: %w", err)
		}

		result.FlowConditionNodesCreated++
	}
	return nil
}

// importFlowNoopNodes imports flow noop nodes from the bundle.
func (s *IOWorkspaceService) importFlowNoopNodes(ctx context.Context, nodeNoopService snodenoop.NodeNoopService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, noopNode := range bundle.FlowNoopNodes {
		// Remap flow node ID
		if newNodeID, ok := result.NodeIDMap[noopNode.FlowNodeID]; ok {
			noopNode.FlowNodeID = newNodeID
		}

		// Create noop node
		if err := nodeNoopService.CreateNodeNoop(ctx, noopNode); err != nil {
			return fmt.Errorf("failed to create flow noop node: %w", err)
		}

		result.FlowNoopNodesCreated++
	}
	return nil
}

// importFlowForNodes imports flow for nodes from the bundle.
func (s *IOWorkspaceService) importFlowForNodes(ctx context.Context, nodeForService snodefor.NodeForService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, forNode := range bundle.FlowForNodes {
		// Remap flow node ID
		if newNodeID, ok := result.NodeIDMap[forNode.FlowNodeID]; ok {
			forNode.FlowNodeID = newNodeID
		}

		// Create for node
		if err := nodeForService.CreateNodeFor(ctx, forNode); err != nil {
			return fmt.Errorf("failed to create flow for node: %w", err)
		}

		result.FlowForNodesCreated++
	}
	return nil
}

// importFlowForEachNodes imports flow foreach nodes from the bundle.
func (s *IOWorkspaceService) importFlowForEachNodes(ctx context.Context, nodeForEachService snodeforeach.NodeForEachService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, forEachNode := range bundle.FlowForEachNodes {
		// Remap flow node ID
		if newNodeID, ok := result.NodeIDMap[forEachNode.FlowNodeID]; ok {
			forEachNode.FlowNodeID = newNodeID
		}

		// Create foreach node
		if err := nodeForEachService.CreateNodeForEach(ctx, forEachNode); err != nil {
			return fmt.Errorf("failed to create flow foreach node: %w", err)
		}

		result.FlowForEachNodesCreated++
	}
	return nil
}

// importFlowJSNodes imports flow JS nodes from the bundle.
func (s *IOWorkspaceService) importFlowJSNodes(ctx context.Context, nodeJSService snodejs.NodeJSService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, jsNode := range bundle.FlowJSNodes {
		// Remap flow node ID
		if newNodeID, ok := result.NodeIDMap[jsNode.FlowNodeID]; ok {
			jsNode.FlowNodeID = newNodeID
		}

		// Create JS node
		if err := nodeJSService.CreateNodeJS(ctx, jsNode); err != nil {
			return fmt.Errorf("failed to create flow JS node: %w", err)
		}

		result.FlowJSNodesCreated++
	}
	return nil
}
