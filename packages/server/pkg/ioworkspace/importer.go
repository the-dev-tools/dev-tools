package ioworkspace

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
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
	FlowForNodesCreated       int
	FlowForEachNodesCreated   int
	FlowJSNodesCreated          int
	FlowAINodesCreated          int
	FlowAIProviderNodesCreated  int
	FlowAIMemoryNodesCreated    int
	EnvironmentsCreated         int
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

	flowService := sflow.NewFlowService(s.queries).TX(tx)
	flowVariableService := sflow.NewFlowVariableService(s.queries).TX(tx)
	nodeService := sflow.NewNodeService(s.queries).TX(tx)
	edgeService := sflow.NewEdgeService(s.queries).TX(tx)

	nodeRequestService := sflow.NewNodeRequestService(s.queries).TX(tx)
	nodeIfService := sflow.NewNodeIfService(s.queries).TX(tx)
	nodeForService := sflow.NewNodeForService(s.queries).TX(tx)
	nodeForEachService := sflow.NewNodeForEachService(s.queries).TX(tx)
	nodeJSService := sflow.NewNodeJsService(s.queries).TX(tx)
	nodeAIService := sflow.NewNodeAIService(s.queries).TX(tx)
	nodeAIProviderService := sflow.NewNodeAiProviderService(s.queries).TX(tx)
	nodeMemoryService := sflow.NewNodeMemoryService(s.queries).TX(tx)

	fileService := sfile.New(s.queries, nil).TX(tx)
	envService := senv.NewEnvironmentService(s.queries, nil).TX(tx)
	varService := senv.NewVariableService(s.queries, nil).TX(tx)

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

		if len(bundle.FlowAINodes) > 0 {
			if err := s.importFlowAINodes(ctx, nodeAIService, bundle, opts, result); err != nil {
				return nil, fmt.Errorf("failed to import flow AI nodes: %w", err)
			}
		}

		if len(bundle.FlowAIProviderNodes) > 0 {
			if err := s.importFlowAIProviderNodes(ctx, nodeAIProviderService, bundle, opts, result); err != nil {
				return nil, fmt.Errorf("failed to import flow AI provider nodes: %w", err)
			}
		}

		if len(bundle.FlowAIMemoryNodes) > 0 {
			if err := s.importFlowAIMemoryNodes(ctx, nodeMemoryService, bundle, opts, result); err != nil {
				return nil, fmt.Errorf("failed to import flow AI memory nodes: %w", err)
			}
		}
	}

	return result, nil
}

// Flow import functions have been moved to importer_flow.go
