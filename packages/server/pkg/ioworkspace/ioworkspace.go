package ioworkspace

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/io/workflow"
	"the-dev-tools/server/pkg/io/workflow/simplified"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/massertres"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mexamplerespheader"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mitemfolder"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/sassertres"
	"the-dev-tools/server/pkg/service/sbodyform"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/sbodyurl"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sexamplerespheader"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/sworkspace"

	"gopkg.in/yaml.v3"
)

type IOWorkspaceService struct {
	DB *sql.DB

	workspaceService sworkspace.WorkspaceService

	collectionService scollection.CollectionService
	folderservice     sitemfolder.ItemFolderService
	endpointService   sitemapi.ItemApiService
	exampleService    sitemapiexample.ItemApiExampleService

	exampleHeaderService sexampleheader.HeaderService
	exampleQueryService  sexamplequery.ExampleQueryService
	exampleAssertService sassert.AssertService

	rawBodyService  sbodyraw.BodyRawService
	formBodyService sbodyform.BodyFormService
	urlBodyService  sbodyurl.BodyURLEncodedService

	responseService       sexampleresp.ExampleRespService
	responseHeaderService sexamplerespheader.ExampleRespHeaderService
	responseAssertService sassertres.AssertResultService

	flowService sflow.FlowService

	flowNodeService     snode.NodeService
	flowEdgeService     sedge.EdgeService
	flowVariableService sflowvariable.FlowVariableService

	flowRequestService   snoderequest.NodeRequestService
	flowConditionService snodeif.NodeIfService
	flowNoopService      snodenoop.NodeNoopService
	flowForService       snodefor.NodeForService
	flowForEachService   snodeforeach.NodeForEachService
	flowJSService        snodejs.NodeJSService
}

func NewIOWorkspaceService(
	DB *sql.DB,
	workspaceService sworkspace.WorkspaceService,
	collectionService scollection.CollectionService,
	folderservice sitemfolder.ItemFolderService,
	endpointService sitemapi.ItemApiService,
	exampleService sitemapiexample.ItemApiExampleService,
	exampleHeaderService sexampleheader.HeaderService,
	exampleQueryService sexamplequery.ExampleQueryService,
	exampleAssertService sassert.AssertService,
	rawBodyService sbodyraw.BodyRawService,
	formBodyService sbodyform.BodyFormService,
	urlBodyService sbodyurl.BodyURLEncodedService,
	responseService sexampleresp.ExampleRespService,
	responseHeaderService sexamplerespheader.ExampleRespHeaderService,
	responseAssertService sassertres.AssertResultService,

	// flow
	flowService sflow.FlowService,

	flowNodeService snode.NodeService,
	flowEdgeService sedge.EdgeService,
	flowVariableService sflowvariable.FlowVariableService,

	flowRequestService snoderequest.NodeRequestService,
	flowConditionService snodeif.NodeIfService,
	flowNoopService snodenoop.NodeNoopService,
	flowForService snodefor.NodeForService,
	flowForEachService snodeforeach.NodeForEachService,
	flowJSService snodejs.NodeJSService,
) *IOWorkspaceService {
	return &IOWorkspaceService{
		DB:                    DB,
		workspaceService:      workspaceService,
		collectionService:     collectionService,
		folderservice:         folderservice,
		endpointService:       endpointService,
		exampleService:        exampleService,
		exampleHeaderService:  exampleHeaderService,
		exampleQueryService:   exampleQueryService,
		exampleAssertService:  exampleAssertService,
		rawBodyService:        rawBodyService,
		formBodyService:       formBodyService,
		urlBodyService:        urlBodyService,
		responseService:       responseService,
		responseHeaderService: responseHeaderService,
		responseAssertService: responseAssertService,

		flowService: flowService,

		flowNodeService:     flowNodeService,
		flowEdgeService:     flowEdgeService,
		flowVariableService: flowVariableService,

		flowRequestService:   flowRequestService,
		flowConditionService: flowConditionService,
		flowNoopService:      flowNoopService,
		flowForService:       flowForService,
		flowForEachService:   flowForEachService,
		flowJSService:        flowJSService,
	}
}

type WorkspaceData struct {
	Workspace mworkspace.Workspace `yaml:"workspace"`

	// collections
	Collections []mcollection.Collection         `yaml:"collections"`
	Folders     []mitemfolder.ItemFolder         `yaml:"folders"`
	Endpoints   []mitemapi.ItemApi               `yaml:"endpoints"`
	Examples    []mitemapiexample.ItemApiExample `yaml:"examples"`

	// example sub items
	ExampleHeaders []mexampleheader.Header `yaml:"example_headers"`
	ExampleQueries []mexamplequery.Query   `yaml:"example_queries"`
	ExampleAsserts []massert.Assert        `yaml:"example_asserts"`

	// body
	Rawbodies  []mbodyraw.ExampleBodyRaw `yaml:"rawbodies"`
	FormBodies []mbodyform.BodyForm      `yaml:"form_bodies"`
	UrlBodies  []mbodyurl.BodyURLEncoded `yaml:"url_bodies"`

	// response
	ExampleResponses       []mexampleresp.ExampleResp             `yaml:"example_responses"`
	ExampleResponseHeaders []mexamplerespheader.ExampleRespHeader `yaml:"example_response_headers"`
	ExampleResponseAsserts []massertres.AssertResult              `yaml:"example_response_asserts"`

	// flows
	Flows []mflow.Flow `yaml:"flows"`

	// Root nodes
	FlowNodes     []mnnode.MNode               `yaml:"flow_nodes"`
	FlowEdges     []edge.Edge                  `yaml:"flow_edges"`
	FlowVariables []mflowvariable.FlowVariable `yaml:"flow_variable"`

	// Sub nodes
	FlowRequestNodes   []mnrequest.MNRequest `yaml:"flow_request_nodes"`
	FlowConditionNodes []mnif.MNIF           `yaml:"flow_condition_nodes"`
	FlowNoopNodes      []mnnoop.NoopNode     `yaml:"flow_noop_nodes"`
	FlowForNodes       []mnfor.MNFor         `yaml:"flow_for_nodes"`
	FlowForEachNodes   []mnforeach.MNForEach `yaml:"flow_foreach_nodes"`
	FlowJSNodes        []mnjs.MNJS           `yaml:"flow_js_nodes"`
}

// ============================================================================
// FULL WORKSPACE FORMAT - Complete database export/import
// ============================================================================
// These functions handle the complete workspace data structure that includes
// all database entities. This format is used for full workspace backup/restore
// and contains all the detailed information about every entity in the workspace.
// ============================================================================

// ImportWorkspace imports a complete workspace data structure into the database.
// This includes all collections, folders, endpoints, examples, flows, nodes, edges, etc.
func (s *IOWorkspaceService) ImportWorkspace(ctx context.Context, data WorkspaceData) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer devtoolsdb.TxnRollback(tx)

	err = s.workspaceService.TX(tx).Create(ctx, &data.Workspace)
	if err != nil {
		return err
	}

	// services
	txCollectionService := s.collectionService.TX(tx)
	txFolderService := s.folderservice.TX(tx)
	txEndpointService := s.endpointService.TX(tx)
	txExampleService := s.exampleService.TX(tx)
	txExampleHeaderService := s.exampleHeaderService.TX(tx)
	txExampleQueryService := s.exampleQueryService.TX(tx)
	txExampleAssertService := s.exampleAssertService.TX(tx)
	txRawBodyService := s.rawBodyService.TX(tx)
	txFormBodyService := s.formBodyService.TX(tx)
	txUrlBodyService := s.urlBodyService.TX(tx)
	txResponseService := s.responseService.TX(tx)
	txResponseHeaderService := s.responseHeaderService.TX(tx)
	txResponseAssertService := s.responseAssertService.TX(tx)

	// // flow
	txFlowService := s.flowService.TX(tx)
	txFlowNodeService := s.flowNodeService.TX(tx)
	txFlowEdgeService := s.flowEdgeService.TX(tx)
	txFlowVariableService := s.flowVariableService.TX(tx)

	tdFlowRequestService := s.flowRequestService.TX(tx)
	txFlowConditionService := s.flowConditionService.TX(tx)
	txFlowNoopService := s.flowNoopService.TX(tx)
	txFlowForService := s.flowForService.TX(tx)
	txFlowForEachService := s.flowForEachService.TX(tx)
	txFlowJSService := s.flowJSService.TX(tx)

	for _, collection := range data.Collections {
		err = txCollectionService.CreateCollection(ctx, &collection)
		if err != nil {
			return err
		}
	}

	err = txFolderService.CreateItemFolderBulk(ctx, data.Folders)
	if err != nil {
		return err
	}

	err = txEndpointService.CreateItemApiBulk(ctx, data.Endpoints)
	if err != nil {
		return err
	}

	err = txExampleService.CreateApiExampleBulk(ctx, data.Examples)
	if err != nil {
		return err
	}

	err = txExampleHeaderService.CreateBulkHeader(ctx, data.ExampleHeaders)
	if err != nil {
		return err
	}

	err = txExampleQueryService.CreateBulkQuery(ctx, data.ExampleQueries)
	if err != nil {
		return err
	}

	err = txExampleAssertService.CreateAssertBulk(ctx, data.ExampleAsserts)
	if err != nil {
		return err
	}

	err = txRawBodyService.CreateBulkBodyRaw(ctx, data.Rawbodies)
	if err != nil {
		return err
	}

	err = txFormBodyService.CreateBulkBodyForm(ctx, data.FormBodies)
	if err != nil {
		return err
	}

	err = txUrlBodyService.CreateBulkBodyURLEncoded(ctx, data.UrlBodies)
	if err != nil {
		return err
	}

	err = txResponseService.CreateExampleRespBulk(ctx, data.ExampleResponses)
	if err != nil {
		return err
	}

	err = txResponseHeaderService.CreateExampleRespHeaderBulk(ctx, data.ExampleResponseHeaders)
	if err != nil {
		return err
	}

	err = txResponseAssertService.CreateAssertResultBulk(ctx, data.ExampleResponseAsserts)
	if err != nil {
		return err
	}

	err = txFlowService.CreateFlowBulk(ctx, data.Flows)
	if err != nil {
		return err
	}

	err = txFlowNodeService.CreateNodeBulk(ctx, data.FlowNodes)
	if err != nil {
		return err
	}

	err = txFlowVariableService.CreateFlowVariableBulk(ctx, data.FlowVariables)
	if err != nil {
		return err
	}

	err = tdFlowRequestService.CreateNodeRequestBulk(ctx, data.FlowRequestNodes)
	if err != nil {
		return err
	}

	err = txFlowConditionService.CreateNodeIfBulk(ctx, data.FlowConditionNodes)
	if err != nil {
		return err
	}

	err = txFlowNoopService.CreateNodeNoopBulk(ctx, data.FlowNoopNodes)
	if err != nil {
		return err
	}

	err = txFlowJSService.CreateNodeJSBulk(ctx, data.FlowJSNodes)
	if err != nil {
		return err
	}

	err = txFlowForService.CreateNodeForBulk(ctx, data.FlowForNodes)
	if err != nil {
		return err
	}

	err = txFlowEdgeService.CreateEdgeBulk(ctx, data.FlowEdges)
	if err != nil {
		return err
	}

	err = txFlowForEachService.CreateNodeForEachBulk(ctx, data.FlowForEachNodes)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

// ImportIntoWorkspace imports data into an existing workspace.
// Unlike ImportWorkspace, this does not create a new workspace.
func (s *IOWorkspaceService) ImportIntoWorkspace(ctx context.Context, data WorkspaceData) error {
	// Use the improved version with better error handling and logging
	return s.ImportIntoWorkspaceImproved(ctx, data)
}

type FilterExport struct {
	FilterExampleIds *[]idwrap.IDWrap
	FilterFlowIds    *[]idwrap.IDWrap
}

// ExportWorkspace exports a complete workspace data structure from the database.
// It supports filtering to export only specific examples or flows.
// The exported data includes all related entities and can be used with ImportWorkspace.
func (s *IOWorkspaceService) ExportWorkspace(ctx context.Context, workspaceID idwrap.IDWrap, FilterExport FilterExport) (*WorkspaceData, error) {
	var data WorkspaceData

	workspace, err := s.workspaceService.Get(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	data.Workspace = *workspace

	requiredExampleIDs := make(map[idwrap.IDWrap]struct{})
	isFilteringExamples := FilterExport.FilterExampleIds != nil
	if isFilteringExamples {
		for _, id := range *FilterExport.FilterExampleIds {
			requiredExampleIDs[id] = struct{}{}
		}
	}

	collections, err := s.collectionService.ListCollections(ctx, workspace.ID)
	if err != nil {
		return nil, err
	}
	data.Collections = collections

	flows, err := s.flowService.GetFlowsByWorkspaceID(ctx, workspace.ID)
	if err != nil {
		return nil, err
	}
	data.Flows = flows

	for _, flow := range flows {
		// flow node
		flowNodes, err := s.flowNodeService.GetNodesByFlowID(ctx, flow.ID)
		if err != nil {
			return nil, err
		}

		data.FlowNodes = append(data.FlowNodes, flowNodes...)

		flowVariables, err := s.flowVariableService.GetFlowVariablesByFlowID(ctx, flow.ID)
		if err != nil {
			return nil, err
		}
		data.FlowVariables = append(data.FlowVariables, flowVariables...)

		// flow edge
		flowEdges, err := s.flowEdgeService.GetEdgesByFlowID(ctx, flow.ID)
		if err != nil {
			return nil, err
		}
		data.FlowEdges = append(data.FlowEdges, flowEdges...)

		for _, node := range flowNodes {
			switch node.NodeKind {
			case mnnode.NODE_KIND_REQUEST:
				request, err := s.flowRequestService.GetNodeRequest(ctx, node.ID)
				if err != nil {
					return nil, err
				}
				// Add referenced example IDs to the required set if filtering is enabled
				if isFilteringExamples {
					if request.ExampleID != nil {
						requiredExampleIDs[*request.ExampleID] = struct{}{}
					}
					if request.DeltaExampleID != nil {
						requiredExampleIDs[*request.DeltaExampleID] = struct{}{}
					}
				}
				data.FlowRequestNodes = append(data.FlowRequestNodes, *request)
			case mnnode.NODE_KIND_CONDITION:
				condition, err := s.flowConditionService.GetNodeIf(ctx, node.ID)
				if err != nil {
					return nil, err
				}
				data.FlowConditionNodes = append(data.FlowConditionNodes, *condition)

			case mnnode.NODE_KIND_NO_OP:
				noOp, err := s.flowNoopService.GetNodeNoop(ctx, node.ID)
				if err != nil {
					return nil, err
				}
				data.FlowNoopNodes = append(data.FlowNoopNodes, *noOp)
			case mnnode.NODE_KIND_FOR:
				forNode, err := s.flowForService.GetNodeFor(ctx, node.ID)
				if err != nil {
					return nil, err
				}
				data.FlowForNodes = append(data.FlowForNodes, *forNode)
			case mnnode.NODE_KIND_FOR_EACH:
				forEachNode, err := s.flowForEachService.GetNodeForEach(ctx, node.ID)
				if err != nil {
					return nil, err
				}
				data.FlowForEachNodes = append(data.FlowForEachNodes, *forEachNode)
			case mnnode.NODE_KIND_JS:
				jsNode, err := s.flowJSService.GetNodeJS(ctx, node.ID)
				if err != nil {
					return nil, err
				}
				data.FlowJSNodes = append(data.FlowJSNodes, jsNode)
			}
		}
	}

	for _, collection := range collections {
		folders, err := s.folderservice.GetFoldersWithCollectionID(ctx, collection.ID)
		if err != nil {
			return nil, err
		}
		data.Folders = append(data.Folders, folders...)

		endpoints, err := s.endpointService.GetApisWithCollectionID(ctx, collection.ID)
		if err != nil {
			return nil, err
		}
		data.Endpoints = append(data.Endpoints, endpoints...)

		// Filter examples if a filter list was provided
		if isFilteringExamples && len(requiredExampleIDs) > 0 {
			for exampleID := range requiredExampleIDs {
				example, err := s.exampleService.GetApiExample(ctx, exampleID)
				if err != nil {
					if err == sql.ErrNoRows { // Skip if example doesn't exist
						continue
					}
					return nil, err
				}
				// Add to examples list if not already included
				found := false
				for _, e := range data.Examples {
					if e.ID == exampleID {
						found = true
						break
					}
				}
				if !found {
					data.Examples = append(data.Examples, *example)
				}
			}
		} else {
			examples, err := s.exampleService.GetApiExampleByCollection(ctx, collection.ID)
			if err != nil {
				return nil, err
			}
			data.Examples = append(data.Examples, examples...)
		}

	}

	// Fetch details for the final list of examples
	for _, example := range data.Examples {
		// headers
		exampleHeaders, err := s.exampleHeaderService.GetHeaderByExampleID(ctx, example.ID)
		if err != nil {
			return nil, err
		}
		data.ExampleHeaders = append(data.ExampleHeaders, exampleHeaders...)

		// queries
		exampleQueries, err := s.exampleQueryService.GetExampleQueriesByExampleID(ctx, example.ID)
		if err != nil {
			return nil, err
		}
		data.ExampleQueries = append(data.ExampleQueries, exampleQueries...)

		// asserts
		exampleAsserts, err := s.exampleAssertService.GetAssertByExampleID(ctx, example.ID)
		if err != nil {
			return nil, err
		}
		data.ExampleAsserts = append(data.ExampleAsserts, exampleAsserts...)

		// // body
		//

		// raw
		rawBody, err := s.rawBodyService.GetBodyRawByExampleID(ctx, example.ID)
		if err != nil && err != sql.ErrNoRows { // Ignore not found errors
			return nil, err
		}
		if err == nil && rawBody != nil { // Only append if found
			data.Rawbodies = append(data.Rawbodies, *rawBody)
		}

		// form
		formBodies, err := s.formBodyService.GetBodyFormsByExampleID(ctx, example.ID)
		if err != nil && err != sql.ErrNoRows { // Ignore not found errors
			return nil, err
		}
		if err == nil { // Append if found (might be an empty slice)
			data.FormBodies = append(data.FormBodies, formBodies...)
		}

		// url
		urlBodies, err := s.urlBodyService.GetBodyURLEncodedByExampleID(ctx, example.ID)
		if err != nil && err != sql.ErrNoRows { // Ignore not found errors
			return nil, err
		}
		if err == nil { // Append if found (might be an empty slice)
			data.UrlBodies = append(data.UrlBodies, urlBodies...)
		}

		// response
		response, err := s.responseService.GetExampleRespByExampleIDLatest(ctx, example.ID)
		if err != nil {
			if err == sexampleresp.ErrNoRespFound {
				// didn't find response so there will be no resp header etc...
				continue
			} else {
				return nil, err
			}
		}
		data.ExampleResponses = append(data.ExampleResponses, *response)

		// response headers
		responseHeaders, err := s.responseHeaderService.GetHeaderByRespID(ctx, response.ID)
		if err != nil {
			return nil, err
		}
		data.ExampleResponseHeaders = append(data.ExampleResponseHeaders, responseHeaders...)

		// response asserts
		responseAsserts, err := s.responseAssertService.GetAssertResultsByResponseID(ctx, response.ID)
		if err != nil {
			return nil, err
		}
		data.ExampleResponseAsserts = append(data.ExampleResponseAsserts, responseAsserts...)

	}

	return &data, nil
}

// UnmarshalWorkspace deserializes a YAML representation of the complete workspace format
// into a WorkspaceData structure. This is used for importing full workspace backups.
func UnmarshalWorkspace(data []byte) (*WorkspaceData, error) {
	var workspace WorkspaceData
	err := yaml.Unmarshal(data, &workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal workspace: %w", err)
	}
	return &workspace, nil
}

func (wd WorkspaceData) VerifyIds() error {
	exampleIds := make(map[idwrap.IDWrap]struct{}, len(wd.Examples))
	for _, example := range wd.Examples {
		exampleIds[example.ID] = struct{}{}
	}

	for _, requestNode := range wd.FlowRequestNodes {
		exampleId := requestNode.ExampleID
		deltaExampleID := requestNode.DeltaExampleID
		if exampleId != nil {
			_, ok := exampleIds[*exampleId]
			if !ok {
				return fmt.Errorf("request node %s referance %s example but it is not exists", requestNode.FlowNodeID, exampleId)
			}
		}
		if deltaExampleID != nil {
			_, ok := exampleIds[*deltaExampleID]
			if !ok {
				return fmt.Errorf("request node %s referance %s example but it is not exists", requestNode.FlowNodeID, deltaExampleID)
			}
		}

	}

	return nil
}

// MarshalWorkspace serializes a WorkspaceData structure into YAML format.
// This creates a complete representation of all workspace entities suitable
// for backup or transfer to another system.
func MarshalWorkspace(workspace *WorkspaceData) ([]byte, error) {
	data, err := yaml.Marshal(workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal workspace: %w", err)
	}
	return data, nil
}

// ============================================================================
// SIMPLIFIED WORKFLOW YAML FORMAT - Human-friendly workflow definition
// ============================================================================
// These types and functions handle a simplified YAML format designed for
// human readability and ease of writing. This format focuses on workflows
// and uses features like:
// - Global request definitions to reduce duplication
// - Simplified header and body formats
// - Direct variable references
// - Minimal boilerplate
// ============================================================================

// WorkflowFormat represents the simplified workflow-centric YAML structure
type WorkflowFormat struct {
	WorkspaceName string         `yaml:"workspace_name"`
	Requests      []RequestDef   `yaml:"requests,omitempty"` // Global request definitions
	Flows         []WorkflowFlow `yaml:"flows"`
	Run           any            `yaml:"run,omitempty"`      // Can be []string or []RunStep
}

// RequestDefMap is a map of request name to RequestDef for quick lookup
type RequestDefMap map[string]*RequestDef

// RunStep represents a flow execution step in the run section
type RunStep struct {
	Flow      string `yaml:"flow"`
	DependsOn any    `yaml:"depends_on,omitempty"` // Can be string or []string
}

type WorkflowFlow struct {
	Name      string             `yaml:"name"`
	Variables any                `yaml:"variables"` // Can be []WorkflowVariable or map[string]string
	Steps     []WorkflowStep     `yaml:"steps"` // This will be used for initial unmarshal structure check if needed, but main logic uses raw map
}

// RequestDef defines a reusable request template
type RequestDef struct {
	Name    string           `yaml:"name"`
	Method  string           `yaml:"method"`
	URL     string           `yaml:"url"`
	Headers any              `yaml:"headers,omitempty"` // Can be []RequestStepHeader or map[string]string
	Body    *RequestStepBody `yaml:"body,omitempty"`
}

type WorkflowVariable struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

// Base step that all step types implement
type WorkflowStep struct {
	StepType  string   `yaml:"-"` // Not directly unmarshalled, determined by map key
	DependsOn []string `yaml:"depends_on,omitempty"`
}

// RequestStep defines HTTP request configuration
type RequestStep struct {
	WorkflowStep `yaml:",inline"`
	Name         string           `yaml:"name"`
	UseRequest   string           `yaml:"use_request,omitempty"` // Reference to global request definition
	Method       string           `yaml:"method,omitempty"`
	URL          string           `yaml:"url,omitempty"`
	Headers      any              `yaml:"headers,omitempty"` // Can be []RequestStepHeader or map[string]string
	Body         *RequestStepBody `yaml:"body,omitempty"`
}

type RequestStepHeader struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type RequestStepBody struct {
	Kind  string `yaml:"kind,omitempty"`  // "json", "form", "url", "raw", etc.
	Value any    `yaml:"value,omitempty"` // The actual body content
}

// UnmarshalYAML implements custom unmarshaling to support both simple and explicit formats
func (b *RequestStepBody) UnmarshalYAML(unmarshal func(any) error) error {
	// First try to unmarshal as the explicit format
	type explicitFormat struct {
		Kind  string `yaml:"kind"`
		Value any    `yaml:"value"`
	}
	
	var explicit explicitFormat
	if err := unmarshal(&explicit); err == nil && explicit.Kind != "" {
		b.Kind = explicit.Kind
		b.Value = explicit.Value
		return nil
	}
	
	// If that fails, try to unmarshal as a direct value (simple format)
	var value any
	if err := unmarshal(&value); err != nil {
		return err
	}
	
	// Default to JSON kind for simple format
	b.Kind = "json"
	b.Value = value
	return nil
}

// IfStep defines conditional branching
type IfStep struct {
	WorkflowStep `yaml:",inline"`
	Name         string `yaml:"name"`
	Expression   string `yaml:"expression"`
	Then         string `yaml:"then,omitempty"`
	Else         string `yaml:"else,omitempty"`
}

// ForStep defines loop iteration
type ForStep struct {
	WorkflowStep `yaml:",inline"`
	Name         string `yaml:"name"`
	IterCount    int    `yaml:"iter_count"`
	Loop         string `yaml:"loop,omitempty"`
}

// JSStep defines JavaScript execution
type JSStep struct {
	WorkflowStep `yaml:",inline"`
	Name         string `yaml:"name"`
	Code         string `yaml:"code"`
}

// unmarshalWorkflowYAMLLegacy parses the workflow-centric YAML format and converts it to WorkspaceData
// Deprecated: This is the legacy implementation. Use the new workflow interface instead.
func unmarshalWorkflowYAMLLegacy(data []byte) (*WorkspaceData, error) {
	var workflow WorkflowFormat
	var rawWorkflow map[string]any

	// First unmarshal to a generic map to handle the step types properly
	if err := yaml.Unmarshal(data, &rawWorkflow); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workflow format: %w", err)
	}

	// Then unmarshal to our structured format (mainly for top-level fields and flow names/variables)
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workflow format: %w", err)
	}

	// Validate required fields
	if workflow.WorkspaceName == "" {
		return nil, fmt.Errorf("workspace_name is required")
	}

	collectionID := idwrap.NewNow()

	// Create workspace data structure with proper IDs
	workspaceID := idwrap.NewNow()
	workspaceData := &WorkspaceData{
		Workspace: mworkspace.Workspace{
			ID:   workspaceID,
			Name: workflow.WorkspaceName,
		},
		Collections: []mcollection.Collection{
			{
				ID:          collectionID,
				Name:        "Workflow Collection", // Default collection name
				WorkspaceID: workspaceID,
			},
		},
		Flows:              make([]mflow.Flow, 0),
		FlowNodes:          make([]mnnode.MNode, 0),
		FlowEdges:          make([]edge.Edge, 0),
		FlowVariables:      make([]mflowvariable.FlowVariable, 0),
		Endpoints:          make([]mitemapi.ItemApi, 0),
		Examples:           make([]mitemapiexample.ItemApiExample, 0),
		ExampleHeaders:     make([]mexampleheader.Header, 0),
		Rawbodies:          make([]mbodyraw.ExampleBodyRaw, 0),
		FlowRequestNodes:   make([]mnrequest.MNRequest, 0),
		FlowConditionNodes: make([]mnif.MNIF, 0),
		FlowForNodes:       make([]mnfor.MNFor, 0),
		FlowJSNodes:        make([]mnjs.MNJS, 0),
	}

	// Create a map of global request definitions for quick lookup
	globalRequests := make(RequestDefMap)
	for i := range workflow.Requests {
		req := &workflow.Requests[i]
		globalRequests[req.Name] = req
	}

	// Process each flow
	for _, wflow := range workflow.Flows {
		// Validate flow name
		if wflow.Name == "" {
			return nil, fmt.Errorf("flow name is required")
		}
		
		// Create flow
		flowID := idwrap.NewNow()
		flow := mflow.Flow{
			ID:          flowID,
			Name:        wflow.Name,
			WorkspaceID: workspaceData.Workspace.ID,
		}
		workspaceData.Flows = append(workspaceData.Flows, flow)

		// Map to track node names to IDs for edge creation
		nodeNameToID := make(map[string]idwrap.IDWrap)
		nodeIndex := make(map[string]int) // Store node indices for dependency resolution

		// Process flow variables from raw data to support both array and map formats
		// First, find the raw flow data
		var rawFlow map[string]any
		if rawFlows, ok := rawWorkflow["flows"].([]any); ok {
			for _, rf := range rawFlows {
				if rfMap, ok := rf.(map[string]any); ok {
					if name, ok := rfMap["name"].(string); ok && name == wflow.Name {
						rawFlow = rfMap
						break
					}
				}
			}
		}
		
		// Process variables from raw flow data
		if rawFlow != nil && rawFlow["variables"] != nil {
			switch vars := rawFlow["variables"].(type) {
			case map[string]any:
				// New map format: variables: { key: value }
				for name, value := range vars {
					valueStr := ""
					switch v := value.(type) {
					case string:
						valueStr = v
					default:
						valueStr = fmt.Sprintf("%v", v)
					}
					variable := mflowvariable.FlowVariable{
						ID:      idwrap.NewNow(),
						FlowID:  flow.ID,
						Name:    name,
						Value:   valueStr,
						Enabled: true,
					}
					workspaceData.FlowVariables = append(workspaceData.FlowVariables, variable)
				}
			case []any:
				// Old array format: variables: [{ name: key, value: value }]
				if varArray, ok := wflow.Variables.([]WorkflowVariable); ok {
					for _, v := range varArray {
						variable := mflowvariable.FlowVariable{
							ID:      idwrap.NewNow(),
							FlowID:  flow.ID,
							Name:    v.Name,
							Value:   v.Value,
							Enabled: true,
						}
						workspaceData.FlowVariables = append(workspaceData.FlowVariables, variable)
					}
				} else {
					// Handle []any case
					for _, item := range vars {
						if varMap, ok := item.(map[string]any); ok {
							name, _ := varMap["name"].(string)
							value, _ := varMap["value"].(string)
							if name != "" {
								variable := mflowvariable.FlowVariable{
									ID:      idwrap.NewNow(),
									FlowID:  flow.ID,
									Name:    name,
									Value:   value,
									Enabled: true,
								}
								workspaceData.FlowVariables = append(workspaceData.FlowVariables, variable)
							}
						}
					}
				}
			}
		}

		startNodeID := idwrap.NewNow()

		startNodeRoot := mnnode.MNode{
			ID:       startNodeID,
			FlowID:   flowID,
			Name:     "Start Node",
			NodeKind: mnnode.NODE_KIND_NO_OP,
		}

		workspaceData.FlowNodes = append(workspaceData.FlowNodes, startNodeRoot)

		startNode := mnnoop.NoopNode{
			FlowNodeID: startNodeID,
			Type:       mnnoop.NODE_NO_OP_KIND_START,
		}

		workspaceData.FlowNoopNodes = append(workspaceData.FlowNoopNodes, startNode)

		// Process steps from raw data to correctly identify step types
		rawFlows, ok := rawWorkflow["flows"].([]any)
		if !ok || len(rawFlows) == 0 {
			// If no flows are defined in the raw data, continue to the next flow definition
			// This handles cases where a flow might be defined in the structured data but not raw, though unlikely
			continue
		}

		var rawSteps []map[string]any
		foundFlow := false
		for _, rf := range rawFlows {
			rfMap, ok := rf.(map[string]any)
			if !ok {
				continue
			}
			if name, ok := rfMap["name"].(string); ok && name == wflow.Name {
				foundFlow = true
				if steps, ok := rfMap["steps"].([]any); ok {
					for _, step := range steps {
						// Expecting each step to be a map with a single key (the type)
						if stepMap, ok := step.(map[string]any); ok && len(stepMap) == 1 {
							rawSteps = append(rawSteps, stepMap)
						} else {
							return nil, fmt.Errorf("invalid step format in flow '%s': expected map with single key", wflow.Name)
						}
					}
				}
				break
			}
		}

		if !foundFlow {
			// If the flow name from the structured data wasn't found in the raw data, skip processing its steps
			continue
		}

		// Process each step and create the appropriate node type
		for i, rawStep := range rawSteps {
			// Since we validated len(rawStep) == 1 above, this loop runs once
			for stepType, stepData := range rawStep {
				dataMap, ok := stepData.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid step data format for type '%s' in flow '%s'", stepType, wflow.Name)
				}

				nodeName, ok := dataMap["name"].(string)
				if !ok || nodeName == "" {
					return nil, fmt.Errorf("step type '%s' in flow '%s' missing required 'name' field", stepType, wflow.Name)
				}

				if _, exists := nodeNameToID[nodeName]; exists {
					return nil, fmt.Errorf("duplicate node name '%s' found in flow '%s'", nodeName, wflow.Name)
				}

				nodeID := idwrap.NewNow()

				// Process step based on type
				var err error
				switch stepType {
				case "request":
					err = processRequestStep(workspaceData, flow.ID, nodeID, nodeName, dataMap, collectionID, globalRequests)
				case "if":
					err = processIfStep(workspaceData, flow.ID, nodeID, nodeName, dataMap)
				case "for":
					err = processForStep(workspaceData, flow.ID, nodeID, nodeName, dataMap)
				case "js":
					err = processJSStep(workspaceData, flow.ID, nodeID, nodeName, dataMap)
				default:
					err = fmt.Errorf("unknown step type '%s' in flow '%s'", stepType, wflow.Name)
				}

				if err != nil {
					return nil, fmt.Errorf("error processing step '%s' (type %s) in flow '%s': %w", nodeName, stepType, wflow.Name, err)
				}

				nodeNameToID[nodeName] = nodeID
				nodeIndex[nodeName] = i
			}
		}

		// Process all edges and dependencies at the end
		if err := processEdgesAndDependencies(workspaceData, flow.ID, rawSteps, nodeNameToID, nodeIndex); err != nil {
			return nil, fmt.Errorf("error processing edges/dependencies in flow '%s': %w", wflow.Name, err)
		}
	}

	// Set up linked list relationships for endpoints
	for i := 0; i < len(workspaceData.Endpoints); i++ {
		if i > 0 {
			prev := &workspaceData.Endpoints[i-1].ID
			workspaceData.Endpoints[i].Prev = prev
		}
		if i < len(workspaceData.Endpoints)-1 {
			next := &workspaceData.Endpoints[i+1].ID
			workspaceData.Endpoints[i].Next = next
		}
	}

	// Set up linked list relationships for examples
	for i := 0; i < len(workspaceData.Examples); i++ {
		if i > 0 {
			prev := &workspaceData.Examples[i-1].ID
			workspaceData.Examples[i].Prev = prev
		}
		if i < len(workspaceData.Examples)-1 {
			next := &workspaceData.Examples[i+1].ID
			workspaceData.Examples[i].Next = next
		}
	}

	// Validate and process run field if present
	if workflow.Run != nil {
		runSteps, err := normalizeRunField(workflow.Run)
		if err != nil {
			return nil, fmt.Errorf("invalid run field: %w", err)
		}
		
		if len(runSteps) > 0 {
			// Create flow name lookup
			flowNames := make(map[string]bool)
			for _, flow := range workspaceData.Flows {
				flowNames[flow.Name] = true
			}
			
			// Validate all referenced flows exist
			for _, step := range runSteps {
				if !flowNames[step.Flow] {
					return nil, fmt.Errorf("run field references non-existent flow: %s", step.Flow)
				}
				
				// Validate dependencies
				deps := normalizeRunDependencies(step.DependsOn)
				for _, dep := range deps {
					if !flowNames[dep] {
						return nil, fmt.Errorf("run step '%s' depends on non-existent flow: %s", step.Flow, dep)
					}
				}
			}
			
			// Check for circular dependencies in run steps
			if err := checkRunCircularDependencies(runSteps); err != nil {
				return nil, fmt.Errorf("run field has circular dependency: %w", err)
			}
		}
	}

	return workspaceData, nil
}

// normalizeRunField converts the run field from various formats to []RunStep
func normalizeRunField(run any) ([]RunStep, error) {
	if run == nil {
		return nil, nil
	}
	
	var result []RunStep
	
	switch r := run.(type) {
	case []any:
		// Could be []string or []RunStep format
		for _, item := range r {
			switch i := item.(type) {
			case string:
				// Simple string format - convert to RunStep
				result = append(result, RunStep{Flow: i})
			case map[string]any:
				// RunStep format
				flow, ok := i["flow"].(string)
				if !ok || flow == "" {
					return nil, fmt.Errorf("run step missing 'flow' field")
				}
				
				step := RunStep{Flow: flow}
				if deps, ok := i["depends_on"]; ok {
					step.DependsOn = deps
				}
				result = append(result, step)
			default:
				return nil, fmt.Errorf("invalid run step format")
			}
		}
	case []string:
		// Old format - simple list of flow names
		for _, flow := range r {
			result = append(result, RunStep{Flow: flow})
		}
	default:
		return nil, fmt.Errorf("run field must be an array")
	}
	
	return result, nil
}

// normalizeRunDependencies converts depends_on field to []string
func normalizeRunDependencies(deps any) []string {
	if deps == nil {
		return nil
	}
	
	switch d := deps.(type) {
	case string:
		return []string{d}
	case []string:
		return d
	case []any:
		var result []string
		for _, dep := range d {
			if s, ok := dep.(string); ok {
				result = append(result, s)
			}
		}
		return result
	default:
		return nil
	}
}

// checkRunCircularDependencies checks for circular dependencies in run steps
func checkRunCircularDependencies(runSteps []RunStep) error {
	// Build adjacency list from dependencies
	adjacency := make(map[string][]string)
	flowSet := make(map[string]bool)
	
	for _, step := range runSteps {
		flowSet[step.Flow] = true
		deps := normalizeRunDependencies(step.DependsOn)
		for _, dep := range deps {
			adjacency[dep] = append(adjacency[dep], step.Flow)
		}
	}
	
	// Check for cycles using DFS
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	
	var hasCycle func(flow string, path []string) (bool, []string)
	hasCycle = func(flow string, path []string) (bool, []string) {
		visited[flow] = true
		recStack[flow] = true
		path = append(path, flow)
		
		for _, next := range adjacency[flow] {
			if !visited[next] {
				if found, cyclePath := hasCycle(next, path); found {
					return true, cyclePath
				}
			} else if recStack[next] {
				// Found a cycle
				cycleStart := -1
				for i, f := range path {
					if f == next {
						cycleStart = i
						break
					}
				}
				if cycleStart >= 0 {
					cyclePath := append(path[cycleStart:], next)
					return true, cyclePath
				}
			}
		}
		
		recStack[flow] = false
		return false, nil
	}
	
	// Check each flow that might be a root
	for flow := range flowSet {
		if !visited[flow] {
			if found, cyclePath := hasCycle(flow, []string{}); found {
				return fmt.Errorf("circular dependency detected: %s", strings.Join(cyclePath, " -> "))
			}
		}
	}
	
	return nil
}

// normalizeHeaders converts headers from either array or object format to []RequestStepHeader
func normalizeHeaders(headers any) []RequestStepHeader {
	if headers == nil {
		return nil
	}
	
	var result []RequestStepHeader
	
	switch h := headers.(type) {
	case []any:
		// Array format: [{ name: "...", value: "..." }, ...]
		for _, item := range h {
			if hMap, ok := item.(map[string]any); ok {
				name, _ := hMap["name"].(string)
				value, _ := hMap["value"].(string)
				if name != "" {
					result = append(result, RequestStepHeader{Name: name, Value: value})
				}
			}
		}
	case map[string]any:
		// Object format: { "HeaderName": "HeaderValue", ... }
		for name, value := range h {
			if valueStr, ok := value.(string); ok {
				result = append(result, RequestStepHeader{Name: name, Value: valueStr})
			}
		}
	case []RequestStepHeader:
		// Already in correct format (from global request)
		result = h
	}
	
	return result
}

// Helper function to process request steps
func processRequestStep(workspaceData *WorkspaceData, flowID, nodeID idwrap.IDWrap, nodeName string, data map[string]any, collectionID idwrap.IDWrap, globalRequests RequestDefMap) error {
	// Check for use_request first
	useRequestValue, hasUseRequest := data["use_request"].(string)
	// Create base node
	node := mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     nodeName,
		NodeKind: mnnode.NODE_KIND_REQUEST,
	}
	workspaceData.FlowNodes = append(workspaceData.FlowNodes, node)

	// Initialize variables for request data
	var method, url string
	var headers []RequestStepHeader
	var body *RequestStepBody

	// Check if this step references a global request
	if hasUseRequest && useRequestValue != "" {
		globalReq, exists := globalRequests[useRequestValue]
		if !exists {
			return fmt.Errorf("request node '%s' references undefined global request '%s'", nodeName, useRequestValue)
		}

		// Use values from global request as defaults
		method = globalReq.Method
		url = globalReq.URL
		headers = normalizeHeaders(globalReq.Headers)
		body = globalReq.Body
	}

	// Override with step-specific values if provided
	if stepMethod, ok := data["method"].(string); ok && stepMethod != "" {
		method = stepMethod
	}
	if stepURL, ok := data["url"].(string); ok && stepURL != "" {
		url = stepURL
	}

	// If method is still empty, default to GET
	if method == "" {
		method = "GET"
	}

	// Ensure we have a URL
	if url == "" {
		return fmt.Errorf("request node '%s' is missing required 'url' field", nodeName)
	}

	// Create endpoint and example
	endpointID := idwrap.NewNow()
	exampleID := idwrap.NewNow()

	endpoint := mitemapi.ItemApi{
		ID:           endpointID,
		Name:         fmt.Sprintf("%s Endpoint", nodeName), // Give endpoint a distinct name
		Url:          url,
		Method:       method,
		CollectionID: collectionID,
		// WorkspaceID: workspaceData.Workspace.ID, // Assuming ItemApi needs WorkspaceID
	}
	workspaceData.Endpoints = append(workspaceData.Endpoints, endpoint)

	// Create example first without body type (we'll set it later)
	example := mitemapiexample.ItemApiExample{
		ID:           exampleID,
		Name:         fmt.Sprintf("%s Example", nodeName), // Give example a distinct name
		ItemApiID:    endpointID,
		CollectionID: collectionID,
		BodyType:     mitemapiexample.BodyTypeNone, // Will be updated later based on body kind
	}

	// Process headers - merge step headers with global headers
	headerMap := make(map[string]string)
	
	// First, add global headers (if any)
	for _, h := range headers {
		headerMap[h.Name] = h.Value
	}
	
	// Then, override/add step-specific headers
	if stepHeaders, ok := data["headers"]; ok {
		// Normalize step headers to handle both array and object formats
		normalizedStepHeaders := normalizeHeaders(stepHeaders)
		for _, h := range normalizedStepHeaders {
			headerMap[h.Name] = h.Value
		}
	}
	
	// Convert merged headers to model format
	exampleHeaders := []mexampleheader.Header{}
	for key, value := range headerMap {
		header := mexampleheader.Header{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			HeaderKey: key,
			Value:     value,
			Enable:    true,
		}
		exampleHeaders = append(exampleHeaders, header)
	}
	workspaceData.ExampleHeaders = append(workspaceData.ExampleHeaders, exampleHeaders...)

	
	// Process body - handle different body kinds and merge with global body
	var finalBody *RequestStepBody
	
	// Start with global body if available
	if body != nil {
		finalBody = &RequestStepBody{
			Kind:  body.Kind,
			Value: body.Value,
		}
	}
	
	// Override/merge with step-specific body
	if bodyData, ok := data["body"]; ok {
		// Check if it's the old format with body_json
		if bodyMap, ok := bodyData.(map[string]any); ok {
			if bodyJSON, hasBodyJSON := bodyMap["body_json"]; hasBodyJSON {
				// Convert old format to new format
				newBody := &RequestStepBody{
					Kind:  "json",
					Value: bodyJSON,
				}
				
				if finalBody == nil {
					finalBody = newBody
				} else if finalBody.Kind == "json" {
					// Merge JSON bodies
					if globalMap, ok := finalBody.Value.(map[string]any); ok {
						if stepMap, ok := bodyJSON.(map[string]any); ok {
							mergedMap := make(map[string]any)
							// Copy global values
							for k, v := range globalMap {
								mergedMap[k] = v
							}
							// Override with step values
							for k, v := range stepMap {
								mergedMap[k] = v
							}
							finalBody.Value = mergedMap
						}
					}
				} else {
					// Different kinds, override completely
					finalBody = newBody
				}
			} else {
				// Handle new format
				stepBodyYAML, err := yaml.Marshal(bodyData)
				if err == nil {
					var stepBody RequestStepBody
					if err := yaml.Unmarshal(stepBodyYAML, &stepBody); err == nil {
						if finalBody == nil {
							finalBody = &stepBody
						} else {
							// Merge step body with global body
							if stepBody.Kind != "" {
								finalBody.Kind = stepBody.Kind
							}
							
							// For JSON bodies, merge the values
							if finalBody.Kind == "json" && stepBody.Kind == "json" {
								// Merge JSON objects
								if globalMap, ok := finalBody.Value.(map[string]any); ok {
									if stepMap, ok := stepBody.Value.(map[string]any); ok {
										mergedMap := make(map[string]any)
										// Copy global values
										for k, v := range globalMap {
											mergedMap[k] = v
										}
										// Override with step values
										for k, v := range stepMap {
											mergedMap[k] = v
										}
										finalBody.Value = mergedMap
									}
								}
							} else {
								// For non-JSON kinds or when kinds differ, step body completely overrides
								finalBody.Value = stepBody.Value
							}
						}
					}
				}
			}
		} else {
			// Handle new simple format
			stepBodyYAML, err := yaml.Marshal(bodyData)
			if err == nil {
				var stepBody RequestStepBody
				if err := yaml.Unmarshal(stepBodyYAML, &stepBody); err == nil {
					if finalBody == nil {
						finalBody = &stepBody
					} else {
						// Merge step body with global body
						if stepBody.Kind != "" {
							finalBody.Kind = stepBody.Kind
						}
						
						// For JSON bodies, merge the values
						if finalBody.Kind == "json" && stepBody.Kind == "json" {
							// Merge JSON objects
							if globalMap, ok := finalBody.Value.(map[string]any); ok {
								if stepMap, ok := stepBody.Value.(map[string]any); ok {
									mergedMap := make(map[string]any)
									// Copy global values
									for k, v := range globalMap {
										mergedMap[k] = v
									}
									// Override with step values
									for k, v := range stepMap {
										mergedMap[k] = v
									}
									finalBody.Value = mergedMap
								}
							}
						} else {
							// For non-JSON kinds or when kinds differ, step body completely overrides
							finalBody.Value = stepBody.Value
						}
					}
				}
			}
		}
	}
	
	// Process the final body based on its kind
	if finalBody != nil {
		switch finalBody.Kind {
		case "json", "":
			// Handle JSON body (empty kind defaults to JSON)
			example.BodyType = mitemapiexample.BodyTypeRaw
			if finalBody.Value != nil {
				jsonData, err := json.Marshal(finalBody.Value)
				if err != nil {
					return fmt.Errorf("failed to marshal JSON body for node '%s': %w", nodeName, err)
				}
				bodyRaw := mbodyraw.ExampleBodyRaw{
					ID:        idwrap.NewNow(),
					ExampleID: exampleID,
					Data:      jsonData,
				}
				workspaceData.Rawbodies = append(workspaceData.Rawbodies, bodyRaw)
			}
		case "form":
			// Handle form-encoded body
			example.BodyType = mitemapiexample.BodyTypeForm
			if valueMap, ok := finalBody.Value.(map[string]any); ok {
				for key, value := range valueMap {
					formBody := mbodyform.BodyForm{
						ID:          idwrap.NewNow(),
						ExampleID:   exampleID,
						BodyKey:     key,
						Value:       fmt.Sprintf("%v", value),
						Enable:      true,
						Description: "",
					}
					workspaceData.FormBodies = append(workspaceData.FormBodies, formBody)
				}
			} else {
				return fmt.Errorf("form body must be an object for node '%s'", nodeName)
			}
		case "url":
			// Handle URL-encoded body
			example.BodyType = mitemapiexample.BodyTypeUrlencoded
			if valueMap, ok := finalBody.Value.(map[string]any); ok {
				for key, value := range valueMap {
					urlBody := mbodyurl.BodyURLEncoded{
						ID:          idwrap.NewNow(),
						ExampleID:   exampleID,
						BodyKey:     key,
						Value:       fmt.Sprintf("%v", value),
						Enable:      true,
						Description: "",
					}
					workspaceData.UrlBodies = append(workspaceData.UrlBodies, urlBody)
				}
			} else {
				return fmt.Errorf("url-encoded body must be an object for node '%s'", nodeName)
			}
		case "raw":
			// Handle raw text body
			example.BodyType = mitemapiexample.BodyTypeRaw
			if strValue, ok := finalBody.Value.(string); ok {
				bodyRaw := mbodyraw.ExampleBodyRaw{
					ID:        idwrap.NewNow(),
					ExampleID: exampleID,
					Data:      []byte(strValue),
				}
				workspaceData.Rawbodies = append(workspaceData.Rawbodies, bodyRaw)
			} else {
				return fmt.Errorf("raw body must be a string for node '%s'", nodeName)
			}
		default:
			return fmt.Errorf("unknown body kind '%s' for node '%s'", finalBody.Kind, nodeName)
		}
	}

	// Add example to workspace data after setting body type
	workspaceData.Examples = append(workspaceData.Examples, example)

	requestNode := mnrequest.MNRequest{
		FlowNodeID: nodeID, // This should be the MNode ID
		EndpointID: &endpointID,
		ExampleID:  &exampleID,
	}
	workspaceData.FlowRequestNodes = append(workspaceData.FlowRequestNodes, requestNode)

	return nil
}

// Helper function to process if steps
func processIfStep(workspaceData *WorkspaceData, flowID, nodeID idwrap.IDWrap, nodeName string, data map[string]any) error {
	// Create base node
	node := mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     nodeName,
		NodeKind: mnnode.NODE_KIND_CONDITION,
	}
	workspaceData.FlowNodes = append(workspaceData.FlowNodes, node)

	// Process if specific fields
	expression, ok := data["expression"].(string)
	if !ok || expression == "" {
		return fmt.Errorf("'expression' field is required for if/condition nodes")
	}

	ifNode := mnif.MNIF{
		FlowNodeID: nodeID, // This should be the MNode ID
		Condition: mcondition.Condition{
			Comparisons: mcondition.Comparison{
				Expression: expression,
			},
		},
	}
	workspaceData.FlowConditionNodes = append(workspaceData.FlowConditionNodes, ifNode)

	return nil
}

// Helper function to process for steps
func processForStep(workspaceData *WorkspaceData, flowID, nodeID idwrap.IDWrap, nodeName string, data map[string]any) error {
	// Create base node
	node := mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     nodeName,
		NodeKind: mnnode.NODE_KIND_FOR,
	}
	workspaceData.FlowNodes = append(workspaceData.FlowNodes, node)

	// Process for specific fields
	iterCount := 0
	// Handle potential float64 from YAML unmarshal into any
	if icFloat, ok := data["iter_count"].(float64); ok {
		iterCount = int(icFloat)
	} else if icInt, ok := data["iter_count"].(int); ok {
		iterCount = icInt
	} else {
		return fmt.Errorf("'iter_count' field for 'for' node '%s' must be an integer", nodeName)
	}

	if iterCount < 0 {
		return fmt.Errorf("'iter_count' for 'for' node '%s' cannot be negative", nodeName)
	}

	forNode := mnfor.MNFor{
		FlowNodeID: nodeID, // This should be the MNode ID
		IterCount:  int64(iterCount),
		// TODO: Add IterVariable based on YAML structure if needed
	}
	workspaceData.FlowForNodes = append(workspaceData.FlowForNodes, forNode)

	return nil
}

// Helper function to process js steps
func processJSStep(workspaceData *WorkspaceData, flowID, nodeID idwrap.IDWrap, nodeName string, data map[string]any) error {
	// Create base node
	node := mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     nodeName,
		NodeKind: mnnode.NODE_KIND_JS,
	}
	workspaceData.FlowNodes = append(workspaceData.FlowNodes, node)

	// Process js specific fields
	code, ok := data["code"].(string)
	if !ok || code == "" {
		return fmt.Errorf("'code' field is required and must be a non-empty string for js node '%s'", nodeName)
	}

	jsNode := mnjs.MNJS{
		FlowNodeID: nodeID, // This should be the MNode ID
		Code:       []byte(code),
	}
	workspaceData.FlowJSNodes = append(workspaceData.FlowJSNodes, jsNode)

	return nil
}

func findStartNodeID(workspaceData *WorkspaceData, flowID idwrap.IDWrap) idwrap.IDWrap {
	for i, node := range workspaceData.FlowNodes {
		if node.FlowID == flowID && node.NodeKind == mnnode.NODE_KIND_NO_OP {
			for _, noopNode := range workspaceData.FlowNoopNodes {
				if noopNode.FlowNodeID == node.ID && noopNode.Type == mnnoop.NODE_NO_OP_KIND_START {
					return workspaceData.FlowNodes[i].ID
				}
			}
		}
	}
	return idwrap.IDWrap{} // Should never happen if flow initialization is correct
}

// Helper function to process all edges and dependencies
func processEdgesAndDependencies(workspaceData *WorkspaceData, flowID idwrap.IDWrap,
	rawSteps []map[string]any, nodeNameToID map[string]idwrap.IDWrap, nodeIndex map[string]int) error {

	startNodeID := findStartNodeID(workspaceData, flowID)
	createdEdges := make(map[string]struct{}) // Track edges to avoid duplicates "sourceID->targetID"

	addEdge := func(sourceID, targetID idwrap.IDWrap, sourceHandler edge.EdgeHandle) {
		key := fmt.Sprintf("%s->%s", sourceID.String(), targetID.String())
		if _, exists := createdEdges[key]; !exists {
			edge := edge.Edge{
				ID:            idwrap.NewNow(),
				FlowID:        flowID,
				SourceID:      sourceID,
				TargetID:      targetID,
				SourceHandler: sourceHandler, // Specify handler if needed (e.g., for if/for)
			}
			workspaceData.FlowEdges = append(workspaceData.FlowEdges, edge)
			createdEdges[key] = struct{}{}
		}
	}

	// First process explicit dependencies and special connections (then/else for if, loop for for)
	for _, step := range rawSteps {
		for stepType, stepData := range step {
			dataMap, ok := stepData.(map[string]any)
			if !ok {
				continue // Should not happen based on earlier checks
			}

			nodeName, _ := dataMap["name"].(string)   //nolint:all // Already validated
			sourceNodeID, _ := nodeNameToID[nodeName] //nolint:all // Already validated

			// Handle special connections based on step type
			switch stepType {
			case "if":
				// Process then/else connections
				if thenTarget, ok := dataMap["then"].(string); ok && thenTarget != "" {
					if targetNodeID, exists := nodeNameToID[thenTarget]; exists {
						addEdge(sourceNodeID, targetNodeID, edge.HandleThen) // Use specific handler
					} else {
						return fmt.Errorf("target node '%s' for 'then' branch of '%s' not found", thenTarget, nodeName)
					}
				}

				if elseTarget, ok := dataMap["else"].(string); ok && elseTarget != "" {
					if targetNodeID, exists := nodeNameToID[elseTarget]; exists {
						addEdge(sourceNodeID, targetNodeID, edge.HandleElse) // Use specific handler
					} else {
						return fmt.Errorf("target node '%s' for 'else' branch of '%s' not found", elseTarget, nodeName)
					}
				}

			case "for":
				// Process loop connections
				if loopTarget, ok := dataMap["loop"].(string); ok && loopTarget != "" {
					if targetNodeID, exists := nodeNameToID[loopTarget]; exists {
						addEdge(sourceNodeID, targetNodeID, edge.HandleLoop) // Use specific handler
					} else {
						return fmt.Errorf("target node '%s' for 'loop' of '%s' not found", loopTarget, nodeName)
					}
				}
			}

			// Process explicit dependencies for all step types
			if deps, ok := dataMap["depends_on"].([]any); ok {
				for _, dep := range deps {
					depName, ok := dep.(string)
					if !ok || depName == "" {
						continue // Skip invalid dependency entries
					}

					fromNodeID, exists := nodeNameToID[depName]
					if !exists {
						return fmt.Errorf("dependency node '%s' for node '%s' not found", depName, nodeName)
					}
					// Ensure dependency is not on self
					if fromNodeID == sourceNodeID {
						return fmt.Errorf("node '%s' cannot depend on itself", nodeName)
					}
					addEdge(fromNodeID, sourceNodeID, edge.HandleUnspecified) // Default handler for dependencies
				}
			}
		}
	}

	// Then create sequential edges where no explicit dependency or special connection implies an order
	nodeHasIncomingEdge := make(map[idwrap.IDWrap]bool)
	for _, e := range workspaceData.FlowEdges {
		nodeHasIncomingEdge[e.TargetID] = true
	}

	for i := 1; i < len(rawSteps); i++ {
		// Get nodes at index i
		var currentStepNodes []idwrap.IDWrap
		for nodeName, idx := range nodeIndex {
			if idx == i {
				currentStepNodes = append(currentStepNodes, nodeNameToID[nodeName])
			}
		}

		// Get nodes at index i-1
		var prevStepNodes []idwrap.IDWrap
		for nodeName, idx := range nodeIndex {
			if idx == i-1 {
				prevStepNodes = append(prevStepNodes, nodeNameToID[nodeName])
			}
		}

		// If there's exactly one node in the previous step and one in the current step,
		// and the current node has no incoming edges yet, create a sequential link.
		// This is a simple heuristic; more complex scenarios might need different logic.
		if len(prevStepNodes) > 0 && len(currentStepNodes) > 0 {
			// Connect *all* previous nodes to *all* current nodes that don't have incoming edges yet?
			// Or just connect the *first* previous node to the *first* current node without incoming?
			// Let's connect *each* previous node to *each* current node *if* the current node has no incoming edge yet.
			// This might create more edges than strictly necessary but ensures connectivity.

			for _, nodeID := range nodeNameToID {
				if !nodeHasIncomingEdge[nodeID] {
					addEdge(startNodeID, nodeID, edge.HandleUnspecified)
				}
			}
			for _, currentNodeID := range currentStepNodes {
				if !nodeHasIncomingEdge[currentNodeID] {
					// Find *a* node from the previous step to connect from.
					// Connecting from all previous nodes might be too much. Let's pick the first one for simplicity.
					if len(prevStepNodes) > 0 {
						prevNodeID := prevStepNodes[0] // Connect from the first node of the previous step
						addEdge(prevNodeID, currentNodeID, edge.HandleUnspecified)
						nodeHasIncomingEdge[currentNodeID] = true // Mark as having an incoming edge now
					}
				}
			}
		}
	}
	
	// Check for circular dependencies
	if err := checkCircularDependencies(workspaceData.FlowEdges, nodeNameToID); err != nil {
		return err
	}
	
	return nil
}

// checkCircularDependencies checks for cycles in the dependency graph
func checkCircularDependencies(edges []edge.Edge, nodeNameToID map[string]idwrap.IDWrap) error {
	// Build adjacency list from edges
	adjacencyList := make(map[idwrap.IDWrap][]idwrap.IDWrap)
	for _, e := range edges {
		adjacencyList[e.SourceID] = append(adjacencyList[e.SourceID], e.TargetID)
	}
	
	// Create reverse map for better error messages
	nodeIDToName := make(map[idwrap.IDWrap]string)
	for name, id := range nodeNameToID {
		nodeIDToName[id] = name
	}
	
	// Track visited nodes and recursion stack for cycle detection
	visited := make(map[idwrap.IDWrap]bool)
	recStack := make(map[idwrap.IDWrap]bool)
	
	// Helper function for DFS
	var hasCycle func(node idwrap.IDWrap, path []idwrap.IDWrap) (bool, []idwrap.IDWrap)
	hasCycle = func(node idwrap.IDWrap, path []idwrap.IDWrap) (bool, []idwrap.IDWrap) {
		visited[node] = true
		recStack[node] = true
		path = append(path, node)
		
		// Check all neighbors
		for _, neighbor := range adjacencyList[node] {
			if !visited[neighbor] {
				if found, cyclePath := hasCycle(neighbor, path); found {
					return true, cyclePath
				}
			} else if recStack[neighbor] {
				// Found a cycle, build the cycle path
				cycleStart := -1
				for i, n := range path {
					if n == neighbor {
						cycleStart = i
						break
					}
				}
				if cycleStart >= 0 {
					cyclePath := append(path[cycleStart:], neighbor)
					return true, cyclePath
				}
			}
		}
		
		recStack[node] = false
		return false, nil
	}
	
	// Check for cycles starting from each unvisited node
	for node := range adjacencyList {
		if !visited[node] {
			if found, cyclePath := hasCycle(node, []idwrap.IDWrap{}); found {
				// Build error message with node names
				var cycleNames []string
				for _, id := range cyclePath {
					if name, ok := nodeIDToName[id]; ok {
						cycleNames = append(cycleNames, name)
					} else {
						cycleNames = append(cycleNames, id.String())
					}
				}
				return fmt.Errorf("circular dependency detected: %s", strings.Join(cycleNames, " -> "))
			}
		}
	}
	
	return nil
}

// ImportWorkflowYAML parses workflow format YAML and imports it as a workspace
func (s *IOWorkspaceService) ImportWorkflowYAML(ctx context.Context, data []byte) error {
	workspace, err := UnmarshalWorkflowYAML(data)
	if err != nil {
		return fmt.Errorf("failed to parse workflow YAML: %w", err)
	}

	// Validate the workspace data before importing
	if len(workspace.Flows) == 0 {
		return fmt.Errorf("workflow must contain at least one flow")
	}

	// Check for any invalid references between nodes
	err = workspace.VerifyIds()
	if err != nil {
		return fmt.Errorf("workflow validation failed: %w", err)
	}

	// Import the workspace
	err = s.ImportWorkspace(ctx, *workspace)
	if err != nil {
		return fmt.Errorf("failed to import workflow: %w", err)
	}

	return nil
}

// marshalWorkflowYAMLLegacy converts a WorkspaceData structure to the workflow-centric YAML format
// Deprecated: This is the legacy implementation. Use the new workflow interface instead.
func marshalWorkflowYAMLLegacy(workspaceData *WorkspaceData) ([]byte, error) {
	if workspaceData == nil {
		return nil, fmt.Errorf("workspace data cannot be nil")
	}

	// Create workflow format structure
	workflow := WorkflowFormat{
		WorkspaceName: workspaceData.Workspace.Name,
		Flows:         make([]WorkflowFlow, 0, len(workspaceData.Flows)),
	}

	// Map node IDs to their details for easy lookup
	nodeMap := make(map[idwrap.IDWrap]mnnode.MNode)
	for _, node := range workspaceData.FlowNodes {
		nodeMap[node.ID] = node
	}

	// Map for special node implementations
	requestNodeMap := make(map[idwrap.IDWrap]mnrequest.MNRequest)
	for _, n := range workspaceData.FlowRequestNodes {
		requestNodeMap[n.FlowNodeID] = n
	}

	ifNodeMap := make(map[idwrap.IDWrap]mnif.MNIF)
	for _, n := range workspaceData.FlowConditionNodes {
		ifNodeMap[n.FlowNodeID] = n
	}

	forNodeMap := make(map[idwrap.IDWrap]mnfor.MNFor)
	for _, n := range workspaceData.FlowForNodes {
		forNodeMap[n.FlowNodeID] = n
	}

	jsNodeMap := make(map[idwrap.IDWrap]mnjs.MNJS)
	for _, n := range workspaceData.FlowJSNodes {
		jsNodeMap[n.FlowNodeID] = n
	}

	// Map edges by source node ID for dependency resolution
	edgesBySource := make(map[idwrap.IDWrap][]edge.Edge)
	for _, e := range workspaceData.FlowEdges {
		edgesBySource[e.SourceID] = append(edgesBySource[e.SourceID], e)
	}

	// Map endpoints and examples for request steps
	endpointMap := make(map[idwrap.IDWrap]mitemapi.ItemApi)
	for _, e := range workspaceData.Endpoints {
		endpointMap[e.ID] = e
	}

	exampleMap := make(map[idwrap.IDWrap]mitemapiexample.ItemApiExample)
	for _, e := range workspaceData.Examples {
		exampleMap[e.ID] = e
	}

	headersByExample := make(map[idwrap.IDWrap][]mexampleheader.Header)
	for _, h := range workspaceData.ExampleHeaders {
		headersByExample[h.ExampleID] = append(headersByExample[h.ExampleID], h)
	}

	bodiesByExample := make(map[idwrap.IDWrap]mbodyraw.ExampleBodyRaw)
	for _, b := range workspaceData.Rawbodies {
		bodiesByExample[b.ExampleID] = b
	}

	// Process each flow
	for _, flow := range workspaceData.Flows {
		workflowFlow := WorkflowFlow{
			Name:      flow.Name,
			Variables: make(map[string]string),
			Steps:     make([]WorkflowStep, 0),
		}

		// Extract variables as a map
		varMap := make(map[string]string)
		for _, v := range workspaceData.FlowVariables {
			if v.FlowID == flow.ID {
				varMap[v.Name] = v.Value
			}
		}
		if len(varMap) > 0 {
			workflowFlow.Variables = varMap
		} else {
			workflowFlow.Variables = nil
		}

		// Get all nodes for this flow
		var flowNodes []mnnode.MNode
		for _, node := range workspaceData.FlowNodes {
			if node.FlowID == flow.ID {
				flowNodes = append(flowNodes, node)
			}
		}

		// Create a step map for YAML structure (using map to properly structure output)
		stepMaps := make([]map[string]interface{}, 0, len(flowNodes))

		// Convert nodes to steps
		for _, node := range flowNodes {
			// Get incoming edges to determine dependencies
			var dependencies []string
			for _, e := range workspaceData.FlowEdges {
				if e.TargetID == node.ID && e.SourceHandler == edge.HandleUnspecified {
					// This is a standard dependency
					if sourceNode, ok := nodeMap[e.SourceID]; ok {
						dependencies = append(dependencies, sourceNode.Name)
					}
				}
			}

			switch node.NodeKind {
			case mnnode.NODE_KIND_REQUEST:
				requestNode, ok := requestNodeMap[node.ID]
				if !ok {
					continue // Skip if node implementation not found
				}

				stepData := map[string]interface{}{
					"name": node.Name,
				}

				if len(dependencies) > 0 {
					stepData["depends_on"] = dependencies
				}

				// Add request details if available
				if requestNode.EndpointID != nil {
					if endpoint, ok := endpointMap[*requestNode.EndpointID]; ok {
						stepData["method"] = endpoint.Method
						stepData["url"] = endpoint.Url
					}
				}

				// Add headers and body if available
				if requestNode.ExampleID != nil {
					exampleID := *requestNode.ExampleID

					// Add headers as object format
					if headers, ok := headersByExample[exampleID]; ok && len(headers) > 0 {
						headersData := make(map[string]string)
						for _, h := range headers {
							headersData[h.HeaderKey] = h.Value
						}
						stepData["headers"] = headersData
					}

					// Add body if available
					if body, ok := bodiesByExample[exampleID]; ok {
						// Try to unmarshal JSON body
						var jsonBody interface{}
						if err := json.Unmarshal(body.Data, &jsonBody); err == nil {
							// Use simplified format - just the JSON object directly
							stepData["body"] = jsonBody
						} else {
							// If not JSON, output as raw string
							stepData["body"] = map[string]interface{}{
								"kind": "raw",
								"data": string(body.Data),
							}
						}
					}
				}

				stepMaps = append(stepMaps, map[string]interface{}{
					"request": stepData,
				})

			case mnnode.NODE_KIND_CONDITION:
				ifNode, ok := ifNodeMap[node.ID]
				if !ok {
					continue // Skip if node implementation not found
				}

				stepData := map[string]interface{}{
					"name":       node.Name,
					"expression": ifNode.Condition.Comparisons.Expression, // Add double quotes
				}

				if len(dependencies) > 0 {
					stepData["depends_on"] = dependencies
				}

				// Look for then/else targets
				for _, e := range edgesBySource[node.ID] {
					switch e.SourceHandler {
					case edge.HandleThen:
						if targetNode, ok := nodeMap[e.TargetID]; ok {
							stepData["then"] = targetNode.Name
						}
					case edge.HandleElse:
						if targetNode, ok := nodeMap[e.TargetID]; ok {
							stepData["else"] = targetNode.Name
						}
					}
				}

				stepMaps = append(stepMaps, map[string]interface{}{
					"if": stepData,
				})

			case mnnode.NODE_KIND_FOR:
				forNode, ok := forNodeMap[node.ID]
				if !ok {
					continue // Skip if node implementation not found
				}

				stepData := map[string]interface{}{
					"name":       node.Name,
					"iter_count": forNode.IterCount,
				}

				if len(dependencies) > 0 {
					stepData["depends_on"] = dependencies
				}

				// Look for loop target
				for _, e := range edgesBySource[node.ID] {
					if e.SourceHandler == edge.HandleLoop {
						if targetNode, ok := nodeMap[e.TargetID]; ok {
							stepData["loop"] = targetNode.Name
						}
					}
				}

				stepMaps = append(stepMaps, map[string]interface{}{
					"for": stepData,
				})

			case mnnode.NODE_KIND_JS:
				jsNode, ok := jsNodeMap[node.ID]
				if !ok {
					continue // Skip if node implementation not found
				}

				stepData := map[string]interface{}{
					"name": node.Name,
					"code": string(jsNode.Code),
				}

				if len(dependencies) > 0 {
					stepData["depends_on"] = dependencies
				}

				stepMaps = append(stepMaps, map[string]interface{}{
					"js": stepData,
				})
			}
		}

		// Add steps array to flow
		workflowFlow.Steps = nil // Clear the typed field as we'll use the raw maps

		// Custom marshalling structure with steps as a dynamic field
		flowMap := map[string]interface{}{
			"name":      workflowFlow.Name,
			"variables": workflowFlow.Variables,
			"steps":     stepMaps,
		}

		workflow.Flows = append(workflow.Flows, workflowFlow)
		// Replace the last flow with our custom map to ensure proper YAML structure
		rawFlows := make([]interface{}, 0, len(workflow.Flows))
		for i, f := range workflow.Flows {
			if i == len(workflow.Flows)-1 {
				rawFlows = append(rawFlows, flowMap)
			} else {
				rawFlows = append(rawFlows, f)
			}
		}

		// Extract global requests from all request nodes
		globalRequests, globalRequestNames, patternDetails := extractGlobalRequests(workspaceData, endpointMap, exampleMap, headersByExample, bodiesByExample)
		
		// Now we need to update the request steps to use global requests
		// This needs to be done before we already processed the flows, so we need to reprocess
		if len(globalRequestNames) > 0 {
			// Update the rawFlows to use use_request where applicable
			rawFlows = updateFlowsWithGlobalRequests(rawFlows, workspaceData, endpointMap, globalRequestNames, patternDetails, headersByExample, bodiesByExample)
		}
		
		// We need to create a raw structure for marshalling with ordered fields
		// Using a custom type to ensure field order in YAML output
		type orderedWorkflow struct {
			WorkspaceName string                   `yaml:"workspace_name"`
			Run           interface{}              `yaml:"run,omitempty"`
			Requests      []map[string]interface{} `yaml:"requests,omitempty"`
			Flows         []interface{}            `yaml:"flows"`
		}
		
		// Build run field with all flow names (in order they were defined)
		var runField interface{}
		if len(workspaceData.Flows) > 0 {
			runOrder := make([]string, 0, len(workspaceData.Flows))
			for _, flow := range workspaceData.Flows {
				runOrder = append(runOrder, flow.Name)
			}
			runField = runOrder
		}
		
		rawWorkflow := orderedWorkflow{
			WorkspaceName: workflow.WorkspaceName,
			Run:           runField,
			Flows:         rawFlows,
		}
		
		// Only add requests section if we have global requests
		if len(globalRequests) > 0 {
			rawWorkflow.Requests = globalRequests
		}

		return yaml.Marshal(rawWorkflow)
	}

	// If we have no flows, just marshal the empty structure
	return yaml.Marshal(workflow)
}

// requestPattern represents a common pattern among requests
type requestPattern struct {
	method  string
	url     string
	headers map[string]string
	hasBody bool
	body    interface{} // Store the actual body data for comparison
}

// extractGlobalRequests analyzes all request nodes and extracts common patterns
// as global request definitions to reduce duplication in the YAML output.
// Returns the global requests and a map from pattern key to global request name.
func extractGlobalRequests(
	workspaceData *WorkspaceData,
	endpointMap map[idwrap.IDWrap]mitemapi.ItemApi,
	exampleMap map[idwrap.IDWrap]mitemapiexample.ItemApiExample,
	headersByExample map[idwrap.IDWrap][]mexampleheader.Header,
	bodiesByExample map[idwrap.IDWrap]mbodyraw.ExampleBodyRaw,
) ([]map[string]interface{}, map[string]string, map[string]requestPattern) {
	
	patternGroups := make(map[string][]mnrequest.MNRequest)
	patternDetails := make(map[string]requestPattern)
	
	// Analyze all request nodes
	for _, reqNode := range workspaceData.FlowRequestNodes {
		if reqNode.EndpointID == nil || reqNode.ExampleID == nil {
			continue
		}
		
		endpoint, ok := endpointMap[*reqNode.EndpointID]
		if !ok {
			continue
		}
		
		// Create a pattern key based on method and URL template
		// Remove variable parts from URL to find common patterns
		urlTemplate := extractURLTemplate(endpoint.Url)
		patternKey := fmt.Sprintf("%s:%s", endpoint.Method, urlTemplate)
		
		// Store the request node grouped by pattern
		patternGroups[patternKey] = append(patternGroups[patternKey], reqNode)
		
		// Store pattern details if not already stored
		if _, exists := patternDetails[patternKey]; !exists {
			// For the first occurrence, just store basic info
			// We'll determine common headers and body later
			patternDetails[patternKey] = requestPattern{
				method:  endpoint.Method,
				url:     urlTemplate,
				headers: make(map[string]string),
				hasBody: false,
				body:    nil,
			}
		}
	}
	
	// Analyze patterns to find common headers and body
	for patternKey, nodes := range patternGroups {
		if len(nodes) == 0 {
			continue
		}
		
		// Find common headers across all nodes in this pattern
		commonHeaders := make(map[string]string)
		
		// Start with headers from the first node
		if nodes[0].ExampleID != nil {
			if exHeaders, ok := headersByExample[*nodes[0].ExampleID]; ok {
				for _, h := range exHeaders {
					if h.Enable {
						commonHeaders[h.HeaderKey] = h.Value
					}
				}
			}
		}
		
		// Check if all other nodes have the same headers
		for i := 1; i < len(nodes); i++ {
			if nodes[i].ExampleID == nil {
				continue
			}
			
			nodeHeaders := make(map[string]string)
			if exHeaders, ok := headersByExample[*nodes[i].ExampleID]; ok {
				for _, h := range exHeaders {
					if h.Enable {
						nodeHeaders[h.HeaderKey] = h.Value
					}
				}
			}
			
			// Remove headers that don't match
			for key, value := range commonHeaders {
				if nodeValue, exists := nodeHeaders[key]; !exists || nodeValue != value {
					delete(commonHeaders, key)
				}
			}
		}
		
		// Update pattern with common headers
		pattern := patternDetails[patternKey]
		pattern.headers = commonHeaders
		patternDetails[patternKey] = pattern
	}
	
	// Create global requests for patterns that appear multiple times
	// or have complex configurations worth extracting
	globalRequests := make([]map[string]interface{}, 0)
	globalRequestNames := make(map[string]string) // pattern key -> global request name
	
	requestCounter := 1
	for patternKey, nodes := range patternGroups {
		// Extract global request if:
		// 1. Pattern is used multiple times, OR
		// 2. Has headers or body (worth extracting for clarity)
		pattern := patternDetails[patternKey]
		if len(nodes) > 1 || len(pattern.headers) > 0 {
			// Generate a name for this global request
			requestName := generateRequestName(pattern.url, pattern.method, requestCounter)
			requestCounter++
			
			globalRequestNames[patternKey] = requestName
			
			// Build the global request definition
			globalReq := map[string]interface{}{
				"name":   requestName,
				"method": pattern.method,
				"url":    pattern.url,
			}
			
			// Add headers if present
			if len(pattern.headers) > 0 {
				globalReq["headers"] = pattern.headers
			}
			
			// Add body if it's common across all uses (check first node as representative)
			if pattern.hasBody && pattern.body != nil {
				// Check if all nodes in this pattern have the same body
				allSameBody := true
				for _, node := range nodes {
					if node.ExampleID != nil {
						if nodeBody, ok := bodiesByExample[*node.ExampleID]; ok {
							var nodeBodyData interface{}
							if err := json.Unmarshal(nodeBody.Data, &nodeBodyData); err == nil {
								// Compare JSON bodies
								if fmt.Sprintf("%v", nodeBodyData) != fmt.Sprintf("%v", pattern.body) {
									allSameBody = false
									break
								}
							} else {
								// Compare string bodies
								if string(nodeBody.Data) != pattern.body {
									allSameBody = false
									break
								}
							}
						}
					}
				}
				
				// Only include body in global request if all instances have the same body
				if allSameBody {
					globalReq["body"] = pattern.body
				}
			}
			
			globalRequests = append(globalRequests, globalReq)
		}
	}
	
	// Return the global requests, name mapping, and pattern details
	return globalRequests, globalRequestNames, patternDetails
}

// extractURLTemplate converts a URL with variables into a template pattern
// e.g., "https://api.example.com/users/123" -> "https://api.example.com/users/{{id}}"
func extractURLTemplate(url string) string {
	// Simple heuristic: replace numeric IDs with {{id}} and UUIDs with {{uuid}}
	// This is a basic implementation and can be enhanced
	
	// Replace UUIDs
	uuidRegex := regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	url = uuidRegex.ReplaceAllString(url, "{{uuid}}")
	
	// Replace numeric IDs in paths
	// Match /123/ or /123 at end
	numericIDRegex := regexp.MustCompile(`/(\d+)(/|$)`)
	url = numericIDRegex.ReplaceAllString(url, "/{{id}}$2")
	
	return url
}

// generateRequestName creates a meaningful name for a global request
func generateRequestName(url, method string, counter int) string {
	// Extract meaningful parts from URL
	parts := strings.Split(url, "/")
	var resourceName string
	
	// Find the last non-variable part
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		if part != "" && !strings.Contains(part, "{{") {
			resourceName = part
			break
		}
	}
	
	if resourceName == "" {
		resourceName = "request"
	}
	
	// Create name like "get_users" or "create_post"
	methodLower := strings.ToLower(method)
	return fmt.Sprintf("%s_%s", methodLower, resourceName)
}

// updateFlowsWithGlobalRequests updates request steps to use global request references
func updateFlowsWithGlobalRequests(
	rawFlows []interface{},
	workspaceData *WorkspaceData,
	endpointMap map[idwrap.IDWrap]mitemapi.ItemApi,
	globalRequestNames map[string]string,
	patternDetails map[string]requestPattern,
	headersByExample map[idwrap.IDWrap][]mexampleheader.Header,
	bodiesByExample map[idwrap.IDWrap]mbodyraw.ExampleBodyRaw,
) []interface{} {
	// Map node names to their request node data for quick lookup
	nodeNameToRequest := make(map[string]mnrequest.MNRequest)
	for _, node := range workspaceData.FlowNodes {
		if node.NodeKind == mnnode.NODE_KIND_REQUEST {
			for _, reqNode := range workspaceData.FlowRequestNodes {
				if reqNode.FlowNodeID == node.ID {
					nodeNameToRequest[node.Name] = reqNode
					break
				}
			}
		}
	}
	
	// Process each flow
	updatedFlows := make([]interface{}, 0, len(rawFlows))
	for _, flow := range rawFlows {
		flowMap, ok := flow.(map[string]interface{})
		if !ok {
			updatedFlows = append(updatedFlows, flow)
			continue
		}
		
		// Process steps in the flow
		if steps, ok := flowMap["steps"].([]map[string]interface{}); ok {
			updatedSteps := make([]map[string]interface{}, 0, len(steps))
			
			for _, step := range steps {
				// Check if this is a request step
				if requestData, ok := step["request"].(map[string]interface{}); ok {
					// Get the node name to find the corresponding request node
					nodeName, _ := requestData["name"].(string)
					
					if reqNode, ok := nodeNameToRequest[nodeName]; ok && reqNode.EndpointID != nil {
						// Get the endpoint to create pattern key
						if endpoint, ok := endpointMap[*reqNode.EndpointID]; ok {
							urlTemplate := extractURLTemplate(endpoint.Url)
							patternKey := fmt.Sprintf("%s:%s", endpoint.Method, urlTemplate)
							
							// Check if this pattern has a global request
							if globalName, hasGlobal := globalRequestNames[patternKey]; hasGlobal {
								// Create new request data using use_request
								newRequestData := map[string]interface{}{
									"name":        nodeName,
									"use_request": globalName,
								}
								
								// Keep dependencies if present
								if deps, ok := requestData["depends_on"]; ok {
									newRequestData["depends_on"] = deps
								}
								
								// Get the global pattern details for comparison
								pattern := patternDetails[patternKey]
								
								// Check URL override - only include if different from pattern
								if url, ok := requestData["url"].(string); ok && url != pattern.url {
									newRequestData["url"] = url
								}
								
								// Check for body override
								if body, ok := requestData["body"]; ok && reqNode.ExampleID != nil {
									// Get the actual body data for this request
									var requestBodyData interface{}
									if bodyRaw, ok := bodiesByExample[*reqNode.ExampleID]; ok {
										var jsonBody interface{}
										if err := json.Unmarshal(bodyRaw.Data, &jsonBody); err == nil {
											requestBodyData = jsonBody
										} else {
											requestBodyData = string(bodyRaw.Data)
										}
									}
									
									// Only include body if it's different from the global pattern body
									if pattern.body == nil || fmt.Sprintf("%v", requestBodyData) != fmt.Sprintf("%v", pattern.body) {
										newRequestData["body"] = body
									}
								}
								
								// Check for header overrides
								if headers, ok := requestData["headers"]; ok && reqNode.ExampleID != nil {
									// Get the actual headers for this request
									requestHeaders := make(map[string]string)
									if exHeaders, ok := headersByExample[*reqNode.ExampleID]; ok {
										for _, h := range exHeaders {
											if h.Enable {
												requestHeaders[h.HeaderKey] = h.Value
											}
										}
									}
									
									// Compare with global pattern headers
									overrideHeaders := make(map[string]string)
									headersMap, _ := headers.(map[string]string)
									
									for key, value := range headersMap {
										// Include header if:
										// 1. It's not in the global pattern, OR
										// 2. Its value is different from the global pattern
										if globalValue, exists := pattern.headers[key]; !exists || globalValue != value {
											overrideHeaders[key] = value
										}
									}
									
									// Only include headers if there are overrides
									if len(overrideHeaders) > 0 {
										newRequestData["headers"] = overrideHeaders
									}
								}
								
								// Update the step with new request data
								step["request"] = newRequestData
							}
						}
					}
				}
				
				updatedSteps = append(updatedSteps, step)
			}
			
			flowMap["steps"] = updatedSteps
		}
		
		updatedFlows = append(updatedFlows, flowMap)
	}
	
	return updatedFlows
}

// UnmarshalWorkflowYAML parses the workflow-centric YAML format and converts it to WorkspaceData
// This function uses the new workflow interface for parsing
func UnmarshalWorkflowYAML(data []byte) (*WorkspaceData, error) {
	s := simplified.New()
	wd, err := s.Unmarshal(data, workflow.FormatYAML)
	if err != nil {
		return nil, err
	}
	
	// Convert from workflow.WorkspaceData to local WorkspaceData
	return convertFromWorkflowData(wd), nil
}

// MarshalWorkflowYAML converts a WorkspaceData structure to the workflow-centric YAML format
// This function uses the new workflow interface for marshaling
func MarshalWorkflowYAML(workspaceData *WorkspaceData) ([]byte, error) {
	// Convert from local WorkspaceData to workflow.WorkspaceData
	wd := convertToWorkflowData(workspaceData)
	
	s := simplified.New()
	return s.Marshal(wd, workflow.FormatYAML)
}

// convertFromWorkflowData converts from workflow.WorkspaceData to local WorkspaceData
func convertFromWorkflowData(wd *workflow.WorkspaceData) *WorkspaceData {
	if wd == nil {
		return nil
	}
	
	return &WorkspaceData{
		Workspace:              wd.Workspace,
		Collections:            []mcollection.Collection{wd.Collection}, // Convert single to array
		Folders:                wd.Folders,
		Endpoints:              wd.Endpoints,
		Examples:               wd.Examples,
		FlowNodes:              wd.FlowNodes,
		FlowEdges:              wd.FlowEdges,
		FlowVariables:          wd.FlowVariables,
		FlowRequestNodes:       wd.FlowRequestNodes,
		FlowConditionNodes:     wd.FlowConditionNodes,
		FlowNoopNodes:          wd.FlowNoopNodes,
		FlowForNodes:           wd.FlowForNodes,
		FlowForEachNodes:       wd.FlowForEachNodes,
		FlowJSNodes:            wd.FlowJSNodes,
		Flows:                  wd.Flows,
		ExampleQueries:         wd.RequestQueries,
		ExampleHeaders:         wd.RequestHeaders,
		ExampleAsserts:         wd.RequestAsserts,
		Rawbodies:              wd.RequestBodyRaw,
		FormBodies:             wd.RequestBodyForm,
		UrlBodies:              wd.RequestBodyUrlencoded,
		ExampleResponses:       wd.Responses,
		ExampleResponseHeaders: wd.ResponseHeaders,
		ExampleResponseAsserts: wd.ResponseAsserts,
	}
}

// convertToWorkflowData converts from local WorkspaceData to workflow.WorkspaceData
func convertToWorkflowData(wd *WorkspaceData) *workflow.WorkspaceData {
	if wd == nil {
		return nil
	}
	
	// Use the first collection if available, otherwise create a default one
	var collection mcollection.Collection
	if len(wd.Collections) > 0 {
		collection = wd.Collections[0]
	} else {
		collection = mcollection.Collection{
			ID:          idwrap.NewNow(),
			Name:        "Default Collection",
			WorkspaceID: wd.Workspace.ID,
		}
	}
	
	return &workflow.WorkspaceData{
		Workspace:              wd.Workspace,
		Collection:             collection, // Convert array to single
		Folders:                wd.Folders,
		Endpoints:              wd.Endpoints,
		Examples:               wd.Examples,
		FlowNodes:              wd.FlowNodes,
		FlowEdges:              wd.FlowEdges,
		FlowVariables:          wd.FlowVariables,
		FlowRequestNodes:       wd.FlowRequestNodes,
		FlowConditionNodes:     wd.FlowConditionNodes,
		FlowNoopNodes:          wd.FlowNoopNodes,
		FlowForNodes:           wd.FlowForNodes,
		FlowForEachNodes:       wd.FlowForEachNodes,
		FlowJSNodes:            wd.FlowJSNodes,
		Flows:                  wd.Flows,
		RequestQueries:         wd.ExampleQueries,
		RequestHeaders:         wd.ExampleHeaders,
		RequestAsserts:         wd.ExampleAsserts,
		RequestBodyRaw:         wd.Rawbodies,
		RequestBodyForm:        wd.FormBodies,
		RequestBodyUrlencoded:  wd.UrlBodies,
		Responses:              wd.ExampleResponses,
		ResponseHeaders:        wd.ExampleResponseHeaders,
		ResponseAsserts:        wd.ExampleResponseAsserts,
		ResponseBodyRaw:        nil, // TODO: Add to workflow.WorkspaceData
		ResponseBodyForm:       nil, // TODO: Add to workflow.WorkspaceData
		ResponseBodyUrlencoded: nil, // TODO: Add to workflow.WorkspaceData
		EndpointExampleMap:     make(map[idwrap.IDWrap][]idwrap.IDWrap), // TODO: Build from examples
	}
}
