package rexport

import (
	"context"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

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
	require.Empty(t, resp.Msg.GetTextData())
	require.True(t, strings.HasSuffix(resp.Msg.Name, ".yaml"))

	// ensure YAML output carries workspace metadata
	body := string(resp.Msg.GetData())
	require.Contains(t, body, "workspace_name")
}

func TestExport_CurlFormat(t *testing.T) {
	ctx := context.Background()
	svc, workspaceID, exampleID := setupExportRPC(t, ctx)

	resp, err := svc.Export(ctx, connect.NewRequest(&exportv1.ExportRequest{
		WorkspaceId: workspaceID.Bytes(),
		ExampleIds:  [][]byte{exampleID.Bytes()},
		Format:      exportv1.ExportFormat_EXPORT_FORMAT_CURL.Enum(),
	}))
	require.NoError(t, err)
	require.NotNil(t, resp)

	require.Empty(t, resp.Msg.GetData())
	text := resp.Msg.GetTextData()
	require.NotEmpty(t, text)
	require.True(t, strings.HasSuffix(resp.Msg.Name, ".curl.txt"))
	require.Contains(t, text, "curl '")
	require.Contains(t, text, "--data-raw")
	require.Contains(t, text, "-H 'Content-Type: application/json'")
	require.Contains(t, text, "?param=value")
}

func TestExportExample_SingleCommand(t *testing.T) {
	ctx := context.Background()
	svc, workspaceID, exampleID := setupExportRPC(t, ctx)

	resp, err := svc.ExportExample(ctx, connect.NewRequest(&exportv1.ExportExampleRequest{
		WorkspaceId: workspaceID.Bytes(),
		ExampleId:   exampleID.Bytes(),
	}))
	require.NoError(t, err)
	require.NotNil(t, resp)

	text := resp.Msg.GetTextData()
	require.NotEmpty(t, text)
	require.True(t, strings.HasSuffix(resp.Msg.Name, ".curl.txt"))
	require.Equal(t, 1, strings.Count(text, "curl '"))
	require.NotContains(t, text, "\n\ncurl '")
}

func setupExportRPC(t *testing.T, ctx context.Context) (ExportRPC, idwrap.IDWrap, idwrap.IDWrap) {
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

	workspaceData, workspaceID, exampleID := buildWorkspaceData()
	require.NoError(t, ioWorkspace.ImportWorkspace(ctx, workspaceData))

	svc := New(
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

	return svc, workspaceID, exampleID
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
