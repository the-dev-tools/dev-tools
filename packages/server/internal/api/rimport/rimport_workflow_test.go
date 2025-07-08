package rimport_test

import (
	"context"
	"testing"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rexport"
	"the-dev-tools/server/internal/api/rimport"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/flow/edge"
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
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/testutil"
	exportv1 "the-dev-tools/spec/dist/buf/go/export/v1"
	importv1 "the-dev-tools/spec/dist/buf/go/import/v1"

	"connectrpc.com/connect"
)

func TestWorkflowSimplifiedYAMLImportExport(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB
	defer base.Close()

	// Create all services
	mockLogger := mocklogger.NewMockLogger()
	
	// Basic services
	ws := sworkspace.New(queries)
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	ifs := sitemfolder.New(queries)
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	
	// Example related services
	ehs := sexampleheader.New(queries)
	eqs := sexamplequery.New(queries)
	eas := sassert.New(queries)
	rbs := sbodyraw.New(queries)
	fbs := sbodyform.New(queries)
	ubs := sbodyurl.New(queries)
	
	// Response services
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	eras := sassertres.New(queries)
	
	// Flow services
	fs := sflow.New(queries)
	fns := snode.New(queries)
	fes := sedge.New(queries)
	fvs := sflowvariable.New(queries)
	
	// Node services
	frns := snoderequest.New(queries)
	fins := snodeif.New(queries)
	fnns := snodenoop.New(queries)
	ffns := snodefor.New(queries)
	ffens := snodeforeach.New(queries)
	fjsns := snodejs.New(queries)

	// Create test workspace and user
	wsID := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, wsID, wsuserID, userID, baseCollectionID)

	// Create a flow with nodes to export
	testFlowID := idwrap.NewNow()
	flowData := mflow.Flow{
		ID:          testFlowID,
		WorkspaceID: wsID,
		Name:        "Test Workflow",
	}
	err := fs.CreateFlow(ctx, flowData)
	testutil.AssertFatal(t, nil, err)

	// Create a collection for endpoints
	testCollectionID := idwrap.NewNow()
	collectionData := mcollection.Collection{
		ID:          testCollectionID,
		WorkspaceID: wsID,
		Name:        "Test Collection",
	}
	err = cs.CreateCollection(ctx, &collectionData)
	testutil.AssertFatal(t, nil, err)

	// Create start node
	startNodeID := idwrap.NewNow()
	startNode := mnnode.MNode{
		ID:        startNodeID,
		FlowID:    testFlowID,
		Name:      "Start",
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: 0,
		PositionY: 0,
	}
	err = fns.CreateNode(ctx, startNode)
	testutil.AssertFatal(t, nil, err)
	
	// Create noop node data for start node
	err = fnns.CreateNodeNoop(ctx, mnnoop.NoopNode{
		FlowNodeID: startNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	})
	testutil.AssertFatal(t, nil, err)

	// Create request node
	requestNodeID := idwrap.NewNow()
	requestNode := mnnode.MNode{
		ID:        requestNodeID,
		FlowID:    testFlowID,
		Name:      "API Request",
		NodeKind:  mnnode.NODE_KIND_REQUEST,
		PositionX: 400,
		PositionY: 0,
	}
	err = fns.CreateNode(ctx, requestNode)
	testutil.AssertFatal(t, nil, err)

	// Create endpoint
	endpointID := idwrap.NewNow()
	endpoint := mitemapi.ItemApi{
		ID:           endpointID,
		CollectionID: testCollectionID,
		Name:         "Test Endpoint",
		Url:          "https://api.example.com/{{version}}/users",
		Method:       "GET",
	}
	err = ias.CreateItemApi(ctx, &endpoint)
	testutil.AssertFatal(t, nil, err)

	// Create example
	exampleID := idwrap.NewNow()
	example := mitemapiexample.ItemApiExample{
		ID:           exampleID,
		ItemApiID:    endpointID,
		CollectionID: testCollectionID,
		Name:         "Test Example",
	}
	err = iaes.CreateApiExample(ctx, &example)
	testutil.AssertFatal(t, nil, err)

	// Create request node data
	requestNodeData := mnrequest.MNRequest{
		FlowNodeID: requestNodeID,
		EndpointID: &endpointID,
		ExampleID:  &exampleID,
	}
	err = frns.CreateNodeRequest(ctx, requestNodeData)
	testutil.AssertFatal(t, nil, err)

	// Create edge
	edgeData := edge.Edge{
		ID:            idwrap.NewNow(),
		FlowID:        testFlowID,
		SourceID:      startNodeID,
		TargetID:      requestNodeID,
		SourceHandler: edge.HandleUnspecified,
	}
	err = fes.CreateEdge(ctx, edgeData)
	testutil.AssertFatal(t, nil, err)

	// Create env and var services
	envs := senv.New(queries)
	vars := svar.New(queries)

	// Create export service
	exportService := rexport.New(
		db, ws, cs, ifs, ias, iaes,
		ehs, eqs, eas, rbs, fbs, ubs,
		ers, erhs, eras,
		fs, fns, fes, fvs,
		frns, *fins, fnns, ffns, ffens, fjsns,
		envs, vars,
	)

	// Export the workspace with flow filter
	exportReq := connect.NewRequest(&exportv1.ExportRequest{
		WorkspaceId: wsID.Bytes(),
		FlowIds:     [][]byte{testFlowID.Bytes()},
	})
	
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	exportResp, err := exportService.Export(authedCtx, exportReq)
	testutil.AssertFatal(t, nil, err)
	testutil.AssertNotFatal(t, nil, exportResp.Msg)

	// Verify export format is YAML
	testutil.Assert(t, "Test Workflow.yaml", exportResp.Msg.Name)
	
	// Create a new workspace for import
	newWsID := idwrap.NewNow()
	newWorkspace := base.GetBaseServices()
	newWorkspace.CreateTempCollection(t, ctx, newWsID, idwrap.NewNow(), userID, idwrap.NewNow())

	// Create import service
	importService := rimport.New(db, ws, cs, us, ifs, ias, iaes, ers, eas)

	// Import into new workspace
	importReq := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: newWsID.Bytes(),
		Name:        "Imported Workflow",
		Data:        exportResp.Msg.Data,
	})

	importResp, err := importService.Import(authedCtx, importReq)
	testutil.AssertFatal(t, nil, err)
	testutil.AssertNotFatal(t, nil, importResp.Msg)

	// Verify flow was imported
	if importResp.Msg.Flow != nil {
		importedFlowID, err := idwrap.NewFromBytes(importResp.Msg.Flow.FlowId)
		testutil.AssertFatal(t, nil, err)
		
		// Check flow exists
		importedFlow, err := fs.GetFlow(ctx, importedFlowID)
		testutil.AssertFatal(t, nil, err)
		testutil.Assert(t, "Test Workflow", importedFlow.Name)
		
		// Check nodes were imported
		nodes, err := fns.GetNodesByFlowID(ctx, importedFlowID)
		testutil.AssertFatal(t, nil, err)
		testutil.Assert(t, 2, len(nodes)) // Start node + request node
		
		// Check edges were imported
		edges, err := fes.GetEdgesByFlowID(ctx, importedFlowID)
		testutil.AssertFatal(t, nil, err)
		testutil.Assert(t, 1, len(edges))
	} else {
		t.Fatal("No flow was imported")
	}
}

func TestWorkflowSimplifiedYAMLImportWithVariables(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB
	defer base.Close()

	// Create services
	ws := sworkspace.New(queries)
	mockLogger := mocklogger.NewMockLogger()
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	ifs := sitemfolder.New(queries)
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	ers := sexampleresp.New(queries)
	eas := sassert.New(queries)

	// Create test workspace
	wsID := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, wsID, wsuserID, userID, baseCollectionID)

	// Create import service
	importService := rimport.New(db, ws, cs, us, ifs, ias, iaes, ers, eas)

	// Create YAML with variables
	yamlData := []byte(`
workspace_name: Variable Test Workspace
flows:
  - name: Variable Test Flow
    variables:
      - name: API_VERSION
        value: v1
      - name: BASE_URL
        value: https://api.example.com
    steps:
      - request:
          name: Get Users
          url: "{{BASE_URL}}/{{API_VERSION}}/users"
          method: GET
`)

	// Import YAML
	importReq := connect.NewRequest(&importv1.ImportRequest{
		WorkspaceId: wsID.Bytes(),
		Name:        "Variable Test",
		Data:        yamlData,
	})

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	importResp, err := importService.Import(authedCtx, importReq)
	testutil.AssertFatal(t, nil, err)
	testutil.AssertNotFatal(t, nil, importResp.Msg)

	// Verify import succeeded
	if importResp.Msg.Flow == nil {
		t.Fatal("No flow was imported")
	}
}