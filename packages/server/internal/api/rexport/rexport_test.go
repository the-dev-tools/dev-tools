package rexport

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
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
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/testutil"
	exportv1 "the-dev-tools/spec/dist/buf/go/export/v1"
)

func TestExport_DefaultBinary(t *testing.T) {
	ctx := context.Background()
	svc, workspaceID, _ := setupExportRPC(t, ctx)

	resp, err := svc.Export(ctx, connect.NewRequest(&exportv1.ExportRequest{
		WorkspaceId: workspaceID.Bytes(),
	}))
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.Msg.GetData())
	require.True(t, strings.HasSuffix(resp.Msg.Name, ".yaml"))

	// ensure YAML output carries workspace metadata
	body := string(resp.Msg.GetData())
	require.Contains(t, body, "workspace_name")
}

func TestExportCurl(t *testing.T) {
	ctx := context.Background()
	svc, workspaceID, exampleID := setupExportRPC(t, ctx)

	resp, err := svc.ExportCurl(ctx, connect.NewRequest(&exportv1.ExportCurlRequest{
		WorkspaceId: workspaceID.Bytes(),
		ExampleIds:  [][]byte{exampleID.Bytes()},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp)

	text := resp.Msg.GetData()
	require.NotEmpty(t, text)
	require.Contains(t, text, "curl '")
	require.Contains(t, text, "--data-raw")
	require.Contains(t, text, "-H 'Content-Type: application/json'")
	require.Contains(t, text, "?param=value")
	require.Equal(t, 1, strings.Count(text, "curl '"))
	require.NotContains(t, text, "\n\ncurl '")
}

func TestExport_WithExampleFilterLimitsOutput(t *testing.T) {
	ctx := context.Background()
	workspaceData, workspaceID, _, secondExampleID := buildWorkspaceDataWithTwoRequests()
	svc := setupExportRPCWithWorkspaceData(t, ctx, workspaceData)

	resp, err := svc.Export(ctx, connect.NewRequest(&exportv1.ExportRequest{
		WorkspaceId: workspaceID.Bytes(),
		ExampleIds:  [][]byte{secondExampleID.Bytes()},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp)

	body := string(resp.Msg.GetData())
	require.NotContains(t, body, "First Request")
	require.Contains(t, body, "Second Request")

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(resp.Msg.GetData(), &doc))

	requests, ok := doc["requests"].([]any)
	require.True(t, ok)
	require.Len(t, requests, 1)
	reqMap, ok := requests[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Second Request", reqMap["name"])

	flowsAny, ok := doc["flows"].([]any)
	require.True(t, ok)
	require.Len(t, flowsAny, 1)
	flowMap, ok := flowsAny[0].(map[string]any)
	require.True(t, ok)
	steps, ok := flowMap["steps"].([]any)
	require.True(t, ok)
	require.Len(t, steps, 1)
	stepMap, ok := steps[0].(map[string]any)
	require.True(t, ok)
	requestStep, ok := stepMap["request"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Second Request", requestStep["name"])
}

func TestExport_WithExampleFilterStandaloneExample(t *testing.T) {
	ctx := context.Background()
	workspaceData, workspaceID, exampleID := buildWorkspaceDataWithoutFlows()
	svc := setupExportRPCWithWorkspaceData(t, ctx, workspaceData)

	resp, err := svc.Export(ctx, connect.NewRequest(&exportv1.ExportRequest{
		WorkspaceId: workspaceID.Bytes(),
		ExampleIds:  [][]byte{exampleID.Bytes()},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp)

	body := string(resp.Msg.GetData())
	require.Contains(t, body, "Standalone Example")

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(resp.Msg.GetData(), &doc))

	flowsAny, ok := doc["flows"].([]any)
	require.True(t, ok)
	require.Len(t, flowsAny, 1)
	flowMap, ok := flowsAny[0].(map[string]any)
	require.True(t, ok)
	steps, ok := flowMap["steps"].([]any)
	require.True(t, ok)
	require.Len(t, steps, 1)
	stepMap, ok := steps[0].(map[string]any)
	require.True(t, ok)
	requestStep, ok := stepMap["request"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Standalone Example", requestStep["name"])
}

func setupExportRPC(t *testing.T, ctx context.Context) (ExportRPC, idwrap.IDWrap, idwrap.IDWrap) {
	deps := newExportDeps(t, ctx)
	workspaceData, workspaceID, exampleID := buildWorkspaceData()
	require.NoError(t, deps.ioWorkspace.ImportWorkspace(ctx, workspaceData))
	return deps.newExportRPC(), workspaceID, exampleID
}

func setupExportRPCWithWorkspaceData(t *testing.T, ctx context.Context, workspaceData ioworkspace.WorkspaceData) ExportRPC {
	deps := newExportDeps(t, ctx)
	require.NoError(t, deps.ioWorkspace.ImportWorkspace(ctx, workspaceData))
	return deps.newExportRPC()
}

type exportDeps struct {
	db                    *sql.DB
	workspaceService      sworkspace.WorkspaceService
	collectionService     scollection.CollectionService
	folderService         sitemfolder.ItemFolderService
	endpointService       sitemapi.ItemApiService
	exampleService        sitemapiexample.ItemApiExampleService
	exampleHeaderService  sexampleheader.HeaderService
	exampleQueryService   sexamplequery.ExampleQueryService
	exampleAssertService  sassert.AssertService
	rawBodyService        sbodyraw.BodyRawService
	formBodyService       sbodyform.BodyFormService
	urlBodyService        sbodyurl.BodyURLEncodedService
	responseService       sexampleresp.ExampleRespService
	responseHeaderService sexamplerespheader.ExampleRespHeaderService
	responseAssertService sassertres.AssertResultService
	flowService           sflow.FlowService
	flowNodeService       snode.NodeService
	flowEdgeService       sedge.EdgeService
	flowVariableService   sflowvariable.FlowVariableService
	flowRequestService    snoderequest.NodeRequestService
	flowConditionService  *snodeif.NodeIfService
	flowNoopService       snodenoop.NodeNoopService
	flowForService        snodefor.NodeForService
	flowForEachService    snodeforeach.NodeForEachService
	flowJSService         snodejs.NodeJSService
	envService            senv.EnvService
	varService            svar.VarService
	ioWorkspace           *ioworkspace.IOWorkspaceService
}

func newExportDeps(t *testing.T, ctx context.Context) exportDeps {
	base := testutil.CreateBaseDB(ctx, t)
	t.Cleanup(base.Close)

	queries := base.Queries
	db := base.DB
	logger := mocklogger.NewMockLogger()

	workspaceService := sworkspace.New(queries)
	collectionService := scollection.New(queries, logger)
	folderService := sitemfolder.New(queries)
	endpointService := sitemapi.New(queries)
	exampleService := sitemapiexample.New(queries)
	exampleHeaderService := sexampleheader.New(queries)
	exampleQueryService := sexamplequery.New(queries)
	exampleAssertService := sassert.New(queries)
	rawBodyService := sbodyraw.New(queries)
	formBodyService := sbodyform.New(queries)
	urlBodyService := sbodyurl.New(queries)
	responseService := sexampleresp.New(queries)
	responseHeaderService := sexamplerespheader.New(queries)
	responseAssertService := sassertres.New(queries)
	flowService := sflow.New(queries)
	flowNodeService := snode.New(queries)
	flowEdgeService := sedge.New(queries)
	flowVariableService := sflowvariable.New(queries)
	flowRequestService := snoderequest.New(queries)
	flowConditionService := snodeif.New(queries)
	flowNoopService := snodenoop.New(queries)
	flowForService := snodefor.New(queries)
	flowForEachService := snodeforeach.New(queries)
	flowJSService := snodejs.New(queries)
	envService := senv.New(queries, logger)
	varService := svar.New(queries, logger)

	ioWorkspace := ioworkspace.NewIOWorkspaceService(
		db,
		workspaceService,
		collectionService,
		folderService,
		endpointService,
		exampleService,
		exampleHeaderService,
		exampleQueryService,
		exampleAssertService,
		rawBodyService,
		formBodyService,
		urlBodyService,
		responseService,
		responseHeaderService,
		responseAssertService,
		flowService,
		flowNodeService,
		flowEdgeService,
		flowVariableService,
		flowRequestService,
		*flowConditionService,
		flowNoopService,
		flowForService,
		flowForEachService,
		flowJSService,
		envService,
		varService,
	)

	return exportDeps{
		db:                    db,
		workspaceService:      workspaceService,
		collectionService:     collectionService,
		folderService:         folderService,
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
		ioWorkspace:           ioWorkspace,
	}
}

func (d exportDeps) newExportRPC() ExportRPC {
	return New(
		d.db,
		d.workspaceService,
		d.collectionService,
		d.folderService,
		d.endpointService,
		d.exampleService,
		d.exampleHeaderService,
		d.exampleQueryService,
		d.exampleAssertService,
		d.rawBodyService,
		d.formBodyService,
		d.urlBodyService,
		d.responseService,
		d.responseHeaderService,
		d.responseAssertService,
		d.flowService,
		d.flowNodeService,
		d.flowEdgeService,
		d.flowVariableService,
		d.flowRequestService,
		*d.flowConditionService,
		d.flowNoopService,
		d.flowForService,
		d.flowForEachService,
		d.flowJSService,
		d.envService,
		d.varService,
	)
}

func buildWorkspaceData() (ioworkspace.WorkspaceData, idwrap.IDWrap, idwrap.IDWrap) {
	workspaceID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	endpointID := idwrap.NewNow()
	exampleID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	startNodeID := idwrap.NewNow()
	requestNodeID := idwrap.NewNow()

	workspace := mworkspace.Workspace{
		ID:      workspaceID,
		Name:    "Test Workspace",
		Updated: time.Now(),
	}

	collection := mcollection.Collection{
		ID:          collectionID,
		WorkspaceID: workspaceID,
		Name:        "Test Collection",
		Updated:     time.Now(),
	}

	endpoint := mitemapi.ItemApi{
		ID:           endpointID,
		CollectionID: collectionID,
		Name:         "Test Endpoint",
		Method:       "POST",
		Url:          "https://example.com/resource",
	}

	example := mitemapiexample.ItemApiExample{
		ID:           exampleID,
		ItemApiID:    endpointID,
		CollectionID: collectionID,
		Name:         "Primary",
		IsDefault:    true,
		BodyType:     mitemapiexample.BodyTypeRaw,
	}

	rawBody := mbodyraw.ExampleBodyRaw{
		ID:            idwrap.NewNow(),
		ExampleID:     exampleID,
		Data:          []byte(`{"payload":true}`),
		VisualizeMode: mbodyraw.VisualizeModeJSON,
	}

	header := mexampleheader.Header{
		ID:        idwrap.NewNow(),
		ExampleID: exampleID,
		HeaderKey: "Content-Type",
		Value:     "application/json",
		Enable:    true,
	}

	query := mexamplequery.Query{
		ID:        idwrap.NewNow(),
		ExampleID: exampleID,
		QueryKey:  "param",
		Value:     "value",
		Enable:    true,
	}

	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	}

	startNode := mnnode.MNode{
		ID:        startNodeID,
		FlowID:    flowID,
		Name:      "Start",
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: 0,
		PositionY: 0,
	}

	requestNode := mnnode.MNode{
		ID:        requestNodeID,
		FlowID:    flowID,
		Name:      "Invoke Endpoint",
		NodeKind:  mnnode.NODE_KIND_REQUEST,
		PositionX: 100,
		PositionY: 0,
	}

	flowRequest := mnrequest.MNRequest{
		FlowNodeID:       requestNodeID,
		EndpointID:       &endpointID,
		ExampleID:        &exampleID,
		HasRequestConfig: true,
	}

	flowEdge := edge.Edge{
		ID:            idwrap.NewNow(),
		FlowID:        flowID,
		SourceID:      startNodeID,
		TargetID:      requestNodeID,
		SourceHandler: edge.HandleThen,
		Kind:          int32(edge.EdgeKindNoOp),
	}

	startNoop := mnnoop.NoopNode{
		FlowNodeID: startNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	}

	workspaceData := ioworkspace.WorkspaceData{
		Workspace:        workspace,
		Collections:      []mcollection.Collection{collection},
		Endpoints:        []mitemapi.ItemApi{endpoint},
		Examples:         []mitemapiexample.ItemApiExample{example},
		ExampleHeaders:   []mexampleheader.Header{header},
		ExampleQueries:   []mexamplequery.Query{query},
		Rawbodies:        []mbodyraw.ExampleBodyRaw{rawBody},
		Flows:            []mflow.Flow{flow},
		FlowNodes:        []mnnode.MNode{startNode, requestNode},
		FlowEdges:        []edge.Edge{flowEdge},
		FlowRequestNodes: []mnrequest.MNRequest{flowRequest},
		FlowNoopNodes:    []mnnoop.NoopNode{startNoop},
	}

	return workspaceData, workspaceID, exampleID
}

func buildWorkspaceDataWithTwoRequests() (ioworkspace.WorkspaceData, idwrap.IDWrap, idwrap.IDWrap, idwrap.IDWrap) {
	workspaceID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	endpointID := idwrap.NewNow()
	firstExampleID := idwrap.NewNow()
	secondExampleID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	startNodeID := idwrap.NewNow()
	firstRequestNodeID := idwrap.NewNow()
	secondRequestNodeID := idwrap.NewNow()

	workspace := mworkspace.Workspace{
		ID:      workspaceID,
		Name:    "Workspace",
		Updated: time.Now(),
	}

	collection := mcollection.Collection{
		ID:          collectionID,
		WorkspaceID: workspaceID,
		Name:        "Collection",
		Updated:     time.Now(),
	}

	endpoint := mitemapi.ItemApi{
		ID:           endpointID,
		CollectionID: collectionID,
		Name:         "Shared Endpoint",
		Method:       "POST",
		Url:          "https://example.dev/api",
	}

	firstExample := mitemapiexample.ItemApiExample{
		ID:           firstExampleID,
		ItemApiID:    endpointID,
		CollectionID: collectionID,
		Name:         "First Request",
		BodyType:     mitemapiexample.BodyTypeRaw,
	}

	secondExample := mitemapiexample.ItemApiExample{
		ID:           secondExampleID,
		ItemApiID:    endpointID,
		CollectionID: collectionID,
		Name:         "Second Request",
		BodyType:     mitemapiexample.BodyTypeRaw,
	}

	firstBody := mbodyraw.ExampleBodyRaw{
		ID:            idwrap.NewNow(),
		ExampleID:     firstExampleID,
		Data:          []byte(`{"first":true}`),
		VisualizeMode: mbodyraw.VisualizeModeJSON,
	}

	secondBody := mbodyraw.ExampleBodyRaw{
		ID:            idwrap.NewNow(),
		ExampleID:     secondExampleID,
		Data:          []byte(`{"second":true}`),
		VisualizeMode: mbodyraw.VisualizeModeJSON,
	}

	firstHeader := mexampleheader.Header{
		ID:        idwrap.NewNow(),
		ExampleID: firstExampleID,
		HeaderKey: "X-First",
		Value:     "true",
		Enable:    true,
	}

	secondHeader := mexampleheader.Header{
		ID:        idwrap.NewNow(),
		ExampleID: secondExampleID,
		HeaderKey: "X-Second",
		Value:     "true",
		Enable:    true,
	}

	firstQuery := mexamplequery.Query{
		ID:        idwrap.NewNow(),
		ExampleID: firstExampleID,
		QueryKey:  "one",
		Value:     "1",
		Enable:    true,
	}

	secondQuery := mexamplequery.Query{
		ID:        idwrap.NewNow(),
		ExampleID: secondExampleID,
		QueryKey:  "two",
		Value:     "2",
		Enable:    true,
	}

	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Multi Request Flow",
	}

	startNode := mnnode.MNode{
		ID:        startNodeID,
		FlowID:    flowID,
		Name:      "Start",
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: 0,
		PositionY: 0,
	}

	firstRequestNode := mnnode.MNode{
		ID:        firstRequestNodeID,
		FlowID:    flowID,
		Name:      "First Request",
		NodeKind:  mnnode.NODE_KIND_REQUEST,
		PositionX: 120,
		PositionY: 0,
	}

	secondRequestNode := mnnode.MNode{
		ID:        secondRequestNodeID,
		FlowID:    flowID,
		Name:      "Second Request",
		NodeKind:  mnnode.NODE_KIND_REQUEST,
		PositionX: 240,
		PositionY: 0,
	}

	firstRequest := mnrequest.MNRequest{
		FlowNodeID:       firstRequestNodeID,
		EndpointID:       &endpointID,
		ExampleID:        &firstExampleID,
		HasRequestConfig: true,
	}

	secondRequest := mnrequest.MNRequest{
		FlowNodeID:       secondRequestNodeID,
		EndpointID:       &endpointID,
		ExampleID:        &secondExampleID,
		HasRequestConfig: true,
	}

	firstEdge := edge.Edge{
		ID:            idwrap.NewNow(),
		FlowID:        flowID,
		SourceID:      startNodeID,
		TargetID:      firstRequestNodeID,
		SourceHandler: edge.HandleThen,
		Kind:          int32(edge.EdgeKindNoOp),
	}

	secondEdge := edge.Edge{
		ID:            idwrap.NewNow(),
		FlowID:        flowID,
		SourceID:      firstRequestNodeID,
		TargetID:      secondRequestNodeID,
		SourceHandler: edge.HandleThen,
		Kind:          int32(edge.EdgeKindNoOp),
	}

	startNoop := mnnoop.NoopNode{
		FlowNodeID: startNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	}

	workspaceData := ioworkspace.WorkspaceData{
		Workspace:        workspace,
		Collections:      []mcollection.Collection{collection},
		Endpoints:        []mitemapi.ItemApi{endpoint},
		Examples:         []mitemapiexample.ItemApiExample{firstExample, secondExample},
		ExampleHeaders:   []mexampleheader.Header{firstHeader, secondHeader},
		ExampleQueries:   []mexamplequery.Query{firstQuery, secondQuery},
		Rawbodies:        []mbodyraw.ExampleBodyRaw{firstBody, secondBody},
		Flows:            []mflow.Flow{flow},
		FlowNodes:        []mnnode.MNode{startNode, firstRequestNode, secondRequestNode},
		FlowEdges:        []edge.Edge{firstEdge, secondEdge},
		FlowRequestNodes: []mnrequest.MNRequest{firstRequest, secondRequest},
		FlowNoopNodes:    []mnnoop.NoopNode{startNoop},
	}

	return workspaceData, workspaceID, firstExampleID, secondExampleID
}

func buildWorkspaceDataWithoutFlows() (ioworkspace.WorkspaceData, idwrap.IDWrap, idwrap.IDWrap) {
	workspaceID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	endpointID := idwrap.NewNow()
	exampleID := idwrap.NewNow()

	workspace := mworkspace.Workspace{
		ID:      workspaceID,
		Name:    "Standalone Workspace",
		Updated: time.Now(),
	}

	collection := mcollection.Collection{
		ID:          collectionID,
		WorkspaceID: workspaceID,
		Name:        "Standalone Collection",
		Updated:     time.Now(),
	}

	endpoint := mitemapi.ItemApi{
		ID:           endpointID,
		CollectionID: collectionID,
		Name:         "Standalone Endpoint",
		Method:       "GET",
		Url:          "https://example.dev/standalone",
	}

	example := mitemapiexample.ItemApiExample{
		ID:           exampleID,
		ItemApiID:    endpointID,
		CollectionID: collectionID,
		Name:         "Standalone Example",
		BodyType:     mitemapiexample.BodyTypeRaw,
	}

	header := mexampleheader.Header{
		ID:        idwrap.NewNow(),
		ExampleID: exampleID,
		HeaderKey: "X-Standalone",
		Value:     "true",
		Enable:    true,
	}

	query := mexamplequery.Query{
		ID:        idwrap.NewNow(),
		ExampleID: exampleID,
		QueryKey:  "mode",
		Value:     "solo",
		Enable:    true,
	}

	rawBody := mbodyraw.ExampleBodyRaw{
		ID:            idwrap.NewNow(),
		ExampleID:     exampleID,
		Data:          []byte(`{"standalone":true}`),
		VisualizeMode: mbodyraw.VisualizeModeJSON,
	}

	workspaceData := ioworkspace.WorkspaceData{
		Workspace:      workspace,
		Collections:    []mcollection.Collection{collection},
		Endpoints:      []mitemapi.ItemApi{endpoint},
		Examples:       []mitemapiexample.ItemApiExample{example},
		ExampleHeaders: []mexampleheader.Header{header},
		ExampleQueries: []mexamplequery.Query{query},
		Rawbodies:      []mbodyraw.ExampleBodyRaw{rawBody},
	}

	return workspaceData, workspaceID, exampleID
}
