package ioworkspace

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
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

type FilterExport struct {
	FilterExampleIds *[]idwrap.IDWrap
	FilterFlowIds    *[]idwrap.IDWrap
}

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
		response, err := s.responseService.GetExampleRespByExampleID(ctx, example.ID)
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

func MarshalWorkspace(workspace *WorkspaceData) ([]byte, error) {
	data, err := yaml.Marshal(workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal workspace: %w", err)
	}
	return data, nil
}

// WorkflowFormat represents the simplified workflow-centric YAML structure
type WorkflowFormat struct {
	WorkspaceName string         `yaml:"workspace_name"`
	Flows         []WorkflowFlow `yaml:"flows"`
}

type WorkflowFlow struct {
	Name      string             `yaml:"name"`
	Variables []WorkflowVariable `yaml:"variables"`
	Steps     []WorkflowStep     `yaml:"steps"` // This will be used for initial unmarshal structure check if needed, but main logic uses raw map
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
	Name         string              `yaml:"name"`
	Method       string              `yaml:"method"`
	URL          string              `yaml:"url"`
	Headers      []RequestStepHeader `yaml:"headers,omitempty"`
	Body         *RequestStepBody    `yaml:"body,omitempty"`
}

type RequestStepHeader struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type RequestStepBody struct {
	BodyJSON map[string]any `yaml:"body_json,omitempty"`
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

// UnmarshalWorkflowYAML parses the workflow-centric YAML format and converts it to WorkspaceData
func UnmarshalWorkflowYAML(data []byte) (*WorkspaceData, error) {
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

	// Process each flow
	for _, wflow := range workflow.Flows {
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

		// Process flow variables
		for _, v := range wflow.Variables {
			variable := mflowvariable.FlowVariable{
				ID:      idwrap.NewNow(),
				FlowID:  flow.ID,
				Name:    v.Name,
				Value:   v.Value,
				Enabled: true,
			}
			workspaceData.FlowVariables = append(workspaceData.FlowVariables, variable)
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
					err = processRequestStep(workspaceData, flow.ID, nodeID, nodeName, dataMap, collectionID)
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

	return workspaceData, nil
}

// Helper function to process request steps
func processRequestStep(workspaceData *WorkspaceData, flowID, nodeID idwrap.IDWrap, nodeName string, data map[string]any, collectionID idwrap.IDWrap) error {
	// Create base node
	node := mnnode.MNode{
		ID:       nodeID,
		FlowID:   flowID,
		Name:     nodeName,
		NodeKind: mnnode.NODE_KIND_REQUEST,
	}
	workspaceData.FlowNodes = append(workspaceData.FlowNodes, node)

	// Process request specific fields
	method, _ := data["method"].(string)
	if method == "" {
		method = "GET" // Default to GET if not specified
	}

	url, ok := data["url"].(string)
	if !ok || url == "" {
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

	example := mitemapiexample.ItemApiExample{
		ID:           exampleID,
		Name:         fmt.Sprintf("%s Example", nodeName), // Give example a distinct name
		ItemApiID:    endpointID,
		CollectionID: collectionID,
	}
	workspaceData.Examples = append(workspaceData.Examples, example)

	// Process headers
	headers := []mexampleheader.Header{}
	if headerData, ok := data["headers"].([]any); ok {
		for _, h := range headerData {
			if headerMap, ok := h.(map[string]any); ok {
				headerKey, _ := headerMap["name"].(string)
				headerValue, _ := headerMap["value"].(string)
				if headerKey == "" {
					continue // Skip headers without a name
				}
				header := mexampleheader.Header{
					ID:        idwrap.NewNow(),
					ExampleID: exampleID,
					HeaderKey: headerKey,
					Value:     headerValue,
				}
				headers = append(headers, header)
			}
		}
	}
	workspaceData.ExampleHeaders = append(workspaceData.ExampleHeaders, headers...)

	bodyRaw := mbodyraw.ExampleBodyRaw{
		ID:        idwrap.NewNow(),
		ExampleID: exampleID,
	}
	// Process body
	if bodyData, ok := data["body"].(map[string]any); ok {
		if bodyJSON, ok := bodyData["body_json"].(map[string]any); ok {
			jsonData, err := json.Marshal(bodyJSON)
			if err != nil {
				return fmt.Errorf("failed to marshal body_json for node '%s': %w", nodeName, err)
			}
			bodyRaw.Data = jsonData
		}
		// TODO: Add handling for other body types like text, form-data etc.
	}
	workspaceData.Rawbodies = append(workspaceData.Rawbodies, bodyRaw)

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
	path, _ := data["expression"].(string) // Allow empty path for now, validation might be needed elsewhere

	ifNode := mnif.MNIF{
		FlowNodeID: nodeID, // This should be the MNode ID
		Condition: mcondition.Condition{
			Comparisons: mcondition.Comparison{
				Expression: path,
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
	if !ok {
		// Allow empty code? Or return error? Assuming empty code is valid for now.
		code = ""
		// return fmt.Errorf("'code' field for 'js' node '%s' must be a string", nodeName)
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

// MarshalWorkflowYAML converts a WorkspaceData structure to the workflow-centric YAML format
func MarshalWorkflowYAML(workspaceData *WorkspaceData) ([]byte, error) {
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
			Variables: make([]WorkflowVariable, 0),
			Steps:     make([]WorkflowStep, 0),
		}

		// Extract variables
		for _, v := range workspaceData.FlowVariables {
			if v.FlowID == flow.ID {
				workflowFlow.Variables = append(workflowFlow.Variables, WorkflowVariable{
					Name:  v.Name,
					Value: v.Value,
				})
			}
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

					// Add headers
					if headers, ok := headersByExample[exampleID]; ok && len(headers) > 0 {
						headersData := make([]map[string]string, 0, len(headers))
						for _, h := range headers {
							headersData = append(headersData, map[string]string{
								"name":  h.HeaderKey,
								"value": h.Value,
							})
						}
						stepData["headers"] = headersData
					}

					// Add body if available
					if body, ok := bodiesByExample[exampleID]; ok {
						bodyData := map[string]interface{}{}

						// Try to unmarshal JSON body
						var jsonBody map[string]interface{}
						if err := json.Unmarshal(body.Data, &jsonBody); err == nil {
							bodyData["body_json"] = jsonBody
						}

						if len(bodyData) > 0 {
							stepData["body"] = bodyData
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

		// We need to create a raw structure for marshalling
		rawWorkflow := map[string]interface{}{
			"workspace_name": workflow.WorkspaceName,
			"flows":          rawFlows,
		}

		return yaml.Marshal(rawWorkflow)
	}

	// If we have no flows, just marshal the empty structure
	return yaml.Marshal(workflow)
}
