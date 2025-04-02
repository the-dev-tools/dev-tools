package ioworkspace

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/massertres"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mexamplerespheader"
	"the-dev-tools/server/pkg/model/mflow"
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
	"the-dev-tools/server/pkg/translate/tgeneric"

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

	flowNodeService snode.NodeService

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
	flowService sflow.FlowService,
	flowNodeService snode.NodeService,
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
		flowService:           flowService,
		flowNodeService:       flowNodeService,
		flowRequestService:    flowRequestService,
		flowConditionService:  flowConditionService,
		flowNoopService:       flowNoopService,
		flowForService:        flowForService,
		flowForEachService:    flowForEachService,
		flowJSService:         flowJSService,
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
	FlowNodes []mnnode.MNode `yaml:"flow_nodes"`

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
	defer func() {
		localErr := tx.Rollback()
		if localErr != nil {
			log.Println(localErr)
		}
	}()

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
	txFlowService := s.flowService.TX(tx)
	txFlowNodeService := s.flowNodeService.TX(tx)
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
		flowNodes, err := s.flowNodeService.GetNodesByFlowID(ctx, flow.ID)
		if err != nil {
			return nil, err
		}

		data.FlowNodes = append(data.FlowNodes, flowNodes...)

		for _, node := range flowNodes {
			switch node.NodeKind {
			case mnnode.NODE_KIND_REQUEST:
				request, err := s.flowRequestService.GetNodeRequest(ctx, node.ID)
				if err != nil {
					return nil, err
				}
				if request.ExampleID != nil && FilterExport.FilterExampleIds != nil {
					*FilterExport.FilterExampleIds = tgeneric.RemoveElement(*FilterExport.FilterExampleIds, *request.ExampleID)
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

		for _, endpoint := range endpoints {
			examples, err := s.exampleService.GetApiExamples(ctx, endpoint.ID)
			if err != nil {
				return nil, err
			}

			// filter
			if FilterExport.FilterExampleIds != nil {

				filterMap := make(map[idwrap.IDWrap]struct{})
				for _, id := range *FilterExport.FilterExampleIds {
					filterMap[id] = struct{}{}
				}

				var sortedExamples []mitemapiexample.ItemApiExample
				for _, example := range examples {
					if _, found := filterMap[example.ID]; found {
						sortedExamples = append(sortedExamples, example)
					}
				}
				examples = sortedExamples
			}
			data.Examples = append(data.Examples, examples...)
		}

	}

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
		if err != nil {
			return nil, err
		}
		data.Rawbodies = append(data.Rawbodies, *rawBody)

		// form
		formBodies, err := s.formBodyService.GetBodyFormsByExampleID(ctx, example.ID)
		if err != nil {
			return nil, err
		}
		data.FormBodies = append(data.FormBodies, formBodies...)

		// url
		urlBodies, err := s.urlBodyService.GetBodyURLEncodedByExampleID(ctx, example.ID)
		if err != nil {
			return nil, err
		}
		data.UrlBodies = append(data.UrlBodies, urlBodies...)

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

func MarshalWorkspace(workspace *WorkspaceData) ([]byte, error) {
	data, err := yaml.Marshal(workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal workspace: %w", err)
	}
	return data, nil
}
