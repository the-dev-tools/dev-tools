package rexport

import (
	"context"
	"database/sql"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
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
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/svar"
	yamlflowsimple "the-dev-tools/server/pkg/io/yamlflow/yamlflowsimple"
	exportv1 "the-dev-tools/spec/dist/buf/go/export/v1"
	"the-dev-tools/spec/dist/buf/go/export/v1/exportv1connect"

	"connectrpc.com/connect"
)

type ExportRPC struct {
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

	flowService         sflow.FlowService
	flowNodeService     snode.NodeService
	flowEdgeService     sedge.EdgeService
	flowVariableService sflowvariable.FlowVariableService

	flowRequestService   snoderequest.NodeRequestService
	flowConditionService snodeif.NodeIfService
	flowNoopService      snodenoop.NodeNoopService
	flowForService       snodefor.NodeForService
	flowForEachService   snodeforeach.NodeForEachService
	flowJSService        snodejs.NodeJSService

	envService senv.EnvService
	varService svar.VarService
}

func New(
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
	flowEdgeService sedge.EdgeService,
	flowVariableService sflowvariable.FlowVariableService,

	flowRequestService snoderequest.NodeRequestService,
	flowConditionService snodeif.NodeIfService,
	flowNoopService snodenoop.NodeNoopService,
	flowForService snodefor.NodeForService,
	flowForEachService snodeforeach.NodeForEachService,
	flowJSService snodejs.NodeJSService,
	envService senv.EnvService,
	varService svar.VarService,
) ExportRPC {
	return ExportRPC{
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
		flowEdgeService:       flowEdgeService,
		flowVariableService:   flowVariableService,
		flowRequestService:    flowRequestService,
		flowConditionService:  flowConditionService,
		flowNoopService:       flowNoopService,
		flowForService:        flowForService,
		flowForEachService:    flowForEachService,
		flowJSService:         flowJSService,
		envService:            envService,
		varService:            varService,
	}
}

func CreateService(srv ExportRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := exportv1connect.NewExportServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *ExportRPC) Export(ctx context.Context, req *connect.Request[exportv1.ExportRequest]) (*connect.Response[exportv1.ExportResponse], error) {

	workspaceID, err := idwrap.NewFromBytes(req.Msg.WorkspaceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	filterExport := ioworkspace.FilterExport{}
	if len(req.Msg.FlowIds) != 0 {
		filterIds := []idwrap.IDWrap{}
		for _, flowId := range req.Msg.FlowIds {
			filterID, err := idwrap.NewFromBytes(flowId)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
			filterIds = append(filterIds, filterID)
		}
		filterExport.FilterFlowIds = &filterIds
	}

	if len(req.Msg.ExampleIds) != 0 {
		exampleIds := []idwrap.IDWrap{}
		for _, exampleId := range req.Msg.ExampleIds {
			exampleId, err := idwrap.NewFromBytes(exampleId)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
			exampleIds = append(exampleIds, exampleId)
		}
		filterExport.FilterExampleIds = &exampleIds
	}

	ioWorkspace := ioworkspace.NewIOWorkspaceService(
		c.DB,
		c.workspaceService,
		c.collectionService,
		c.folderservice,
		c.endpointService,
		c.exampleService,
		c.exampleHeaderService,
		c.exampleQueryService,
		c.exampleAssertService,
		c.rawBodyService,
		c.formBodyService,
		c.urlBodyService,
		c.responseService,
		c.responseHeaderService,
		c.responseAssertService,
		c.flowService,
		c.flowNodeService,
		c.flowEdgeService,
		c.flowVariableService,
		c.flowRequestService,
		c.flowConditionService,
		c.flowNoopService,
		c.flowForService,
		c.flowForEachService,
		c.flowJSService,
		c.envService,
		c.varService,
	)

	workspaceData, err := ioWorkspace.ExportWorkspace(ctx, workspaceID, filterExport)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Use simplified YAML format by default
	simplifiedYAML, err := yamlflowsimple.ExportYamlFlowYAML(workspaceData)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	resp := &exportv1.ExportResponse{
		Name: workspaceData.Workspace.Name + ".yaml",
		Data: simplifiedYAML,
	}

	return connect.NewResponse(resp), nil
}

// ExportSimplified exports workspace in simplified YAML format
func (c *ExportRPC) ExportSimplified(ctx context.Context, req *connect.Request[exportv1.ExportRequest]) (*connect.Response[exportv1.ExportResponse], error) {
	workspaceID, err := idwrap.NewFromBytes(req.Msg.WorkspaceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	filterExport := ioworkspace.FilterExport{}
	if len(req.Msg.FlowIds) != 0 {
		filterIds := []idwrap.IDWrap{}
		for _, flowId := range req.Msg.FlowIds {
			filterID, err := idwrap.NewFromBytes(flowId)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
			filterIds = append(filterIds, filterID)
		}
		filterExport.FilterFlowIds = &filterIds
	}

	ioWorkspace := ioworkspace.NewIOWorkspaceService(
		c.DB,
		c.workspaceService,
		c.collectionService,
		c.folderservice,
		c.endpointService,
		c.exampleService,
		c.exampleHeaderService,
		c.exampleQueryService,
		c.exampleAssertService,
		c.rawBodyService,
		c.formBodyService,
		c.urlBodyService,
		c.responseService,
		c.responseHeaderService,
		c.responseAssertService,
		c.flowService,
		c.flowNodeService,
		c.flowEdgeService,
		c.flowVariableService,
		c.flowRequestService,
		c.flowConditionService,
		c.flowNoopService,
		c.flowForService,
		c.flowForEachService,
		c.flowJSService,
		c.envService,
		c.varService,
	)

	workspaceData, err := ioWorkspace.ExportWorkspace(ctx, workspaceID, filterExport)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Convert to simplified format
	simplifiedYAML, err := yamlflowsimple.ExportYamlFlowYAML(workspaceData)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	resp := &exportv1.ExportResponse{
		Name: workspaceData.Workspace.Name + "_simplified.yaml",
		Data: simplifiedYAML,
	}

	return connect.NewResponse(resp), nil
}
