package rexport

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/pkg/idwrap"
	yamlflowsimple "the-dev-tools/server/pkg/io/yamlflow/yamlflowsimple"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/overlay/merge"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/sassertres"
	"the-dev-tools/server/pkg/service/sbodyform"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/sbodyurl"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/senv"
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
	"the-dev-tools/server/pkg/service/soverlayheader"
	"the-dev-tools/server/pkg/service/soverlayquery"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/translate/tcurl"
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
	overlayMgr           *merge.Manager

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
	headerOverlay, _ := soverlayheader.New(DB)
	queryOverlay, _ := soverlayquery.New(DB)
	overlayMgr := merge.New(headerOverlay, queryOverlay)

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
		overlayMgr:            overlayMgr,
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

	var exampleIDs []idwrap.IDWrap
	if len(req.Msg.ExampleIds) != 0 {
		for _, exampleID := range req.Msg.ExampleIds {
			decoded, err := idwrap.NewFromBytes(exampleID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
			exampleIDs = append(exampleIDs, decoded)
		}
		filterExport.FilterExampleIds = &exampleIDs
	}

	workspaceData, err := c.exportWorkspaceData(ctx, workspaceID, filterExport)
	if err != nil {
		return nil, err
	}

	data, err := yamlflowsimple.ExportYamlFlowYAML(ctx, workspaceData, c.overlayMgr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	resp := &exportv1.ExportResponse{
		Name: constructExportName(workspaceData.Workspace.Name, ".yaml"),
		Data: data,
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

	workspaceData, err := c.exportWorkspaceData(ctx, workspaceID, filterExport)
	if err != nil {
		return nil, err
	}

	// Convert to simplified format
	simplifiedYAML, err := yamlflowsimple.ExportYamlFlowYAML(ctx, workspaceData, c.overlayMgr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	resp := &exportv1.ExportResponse{
		Name: constructExportName(workspaceData.Workspace.Name, "_simplified.yaml"),
		Data: simplifiedYAML,
	}

	return connect.NewResponse(resp), nil
}

func (c *ExportRPC) ExportCurl(ctx context.Context, req *connect.Request[exportv1.ExportCurlRequest]) (*connect.Response[exportv1.ExportCurlResponse], error) {
	workspaceID, err := idwrap.NewFromBytes(req.Msg.WorkspaceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	filterExport := ioworkspace.FilterExport{}

	var exampleIDs []idwrap.IDWrap
	if len(req.Msg.ExampleIds) != 0 {
		for _, rawID := range req.Msg.ExampleIds {
			decoded, err := idwrap.NewFromBytes(rawID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
			exampleIDs = append(exampleIDs, decoded)
		}
		filterExport.FilterExampleIds = &exampleIDs
	}

	workspaceData, err := c.exportWorkspaceData(ctx, workspaceID, filterExport)
	if err != nil {
		return nil, err
	}

	curlText, err := buildCurlExport(workspaceData, exampleIDs)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	resp := &exportv1.ExportCurlResponse{Data: curlText}
	return connect.NewResponse(resp), nil
}

func (c *ExportRPC) exportWorkspaceData(ctx context.Context, workspaceID idwrap.IDWrap, filterExport ioworkspace.FilterExport) (*ioworkspace.WorkspaceData, error) {
	ioWorkspace := c.newIOWorkspaceService()
	workspaceData, err := ioWorkspace.ExportWorkspace(ctx, workspaceID, filterExport)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	return workspaceData, nil
}

func (c *ExportRPC) newIOWorkspaceService() *ioworkspace.IOWorkspaceService {
	return ioworkspace.NewIOWorkspaceService(
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
}

func constructExportName(base, suffix string) string {
	if base == "" {
		base = "export"
	}
	return base + suffix
}

func buildCurlExport(workspaceData *ioworkspace.WorkspaceData, requested []idwrap.IDWrap) (string, error) {
	if workspaceData == nil {
		return "", fmt.Errorf("workspace data is nil")
	}

	if len(workspaceData.Examples) == 0 {
		return "", fmt.Errorf("no examples available for curl export")
	}

	endpointsByID := make(map[idwrap.IDWrap]mitemapi.ItemApi, len(workspaceData.Endpoints))
	for _, endpoint := range workspaceData.Endpoints {
		endpointsByID[endpoint.ID] = endpoint
	}

	examplesByID := make(map[idwrap.IDWrap]mitemapiexample.ItemApiExample, len(workspaceData.Examples))
	for _, example := range workspaceData.Examples {
		examplesByID[example.ID] = example
	}

	headersByExample := make(map[idwrap.IDWrap][]mexampleheader.Header)
	for _, header := range workspaceData.ExampleHeaders {
		headersByExample[header.ExampleID] = append(headersByExample[header.ExampleID], header)
	}

	queriesByExample := make(map[idwrap.IDWrap][]mexamplequery.Query)
	for _, query := range workspaceData.ExampleQueries {
		queriesByExample[query.ExampleID] = append(queriesByExample[query.ExampleID], query)
	}

	rawBodiesByExample := make(map[idwrap.IDWrap][]mbodyraw.ExampleBodyRaw)
	for _, body := range workspaceData.Rawbodies {
		rawBodiesByExample[body.ExampleID] = append(rawBodiesByExample[body.ExampleID], body)
	}

	formBodiesByExample := make(map[idwrap.IDWrap][]mbodyform.BodyForm)
	for _, body := range workspaceData.FormBodies {
		formBodiesByExample[body.ExampleID] = append(formBodiesByExample[body.ExampleID], body)
	}

	urlBodiesByExample := make(map[idwrap.IDWrap][]mbodyurl.BodyURLEncoded)
	for _, body := range workspaceData.UrlBodies {
		urlBodiesByExample[body.ExampleID] = append(urlBodiesByExample[body.ExampleID], body)
	}

	orderedIDs := orderedExampleIDs(workspaceData.Examples, requested)
	if len(orderedIDs) == 0 {
		return "", fmt.Errorf("no matching examples for curl export")
	}

	commands := make([]string, 0, len(orderedIDs))
	for _, exampleID := range orderedIDs {
		example, ok := examplesByID[exampleID]
		if !ok {
			return "", fmt.Errorf("example %s not found", exampleID.String())
		}

		endpoint, ok := endpointsByID[example.ItemApiID]
		if !ok {
			return "", fmt.Errorf("endpoint %s not found for example %s", example.ItemApiID.String(), exampleID.String())
		}

		resolved := tcurl.CurlResolved{
			Apis:             []mitemapi.ItemApi{endpoint},
			Examples:         []mitemapiexample.ItemApiExample{example},
			Headers:          headersByExample[exampleID],
			Queries:          queriesByExample[exampleID],
			RawBodies:        rawBodiesByExample[exampleID],
			FormBodies:       formBodiesByExample[exampleID],
			UrlEncodedBodies: urlBodiesByExample[exampleID],
		}

		command, err := tcurl.BuildCurl(resolved)
		if err != nil {
			return "", err
		}

		commands = append(commands, command)
	}

	if len(commands) == 0 {
		return "", fmt.Errorf("no curl commands generated")
	}

	return strings.Join(commands, "\n\n"), nil
}

func orderedExampleIDs(examples []mitemapiexample.ItemApiExample, requested []idwrap.IDWrap) []idwrap.IDWrap {
	if len(requested) > 0 {
		return dedupeExampleIDs(requested)
	}

	result := make([]idwrap.IDWrap, 0, len(examples))
	seen := make(map[idwrap.IDWrap]struct{}, len(examples))

	for _, example := range examples {
		if example.IsDefault {
			if _, ok := seen[example.ID]; !ok {
				seen[example.ID] = struct{}{}
				result = append(result, example.ID)
			}
		}
	}

	for _, example := range examples {
		if _, ok := seen[example.ID]; ok {
			continue
		}
		seen[example.ID] = struct{}{}
		result = append(result, example.ID)
	}

	return result
}

func dedupeExampleIDs(ids []idwrap.IDWrap) []idwrap.IDWrap {
	result := make([]idwrap.IDWrap, 0, len(ids))
	seen := make(map[idwrap.IDWrap]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}
