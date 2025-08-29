package ioworkspace_test

import (
	"context"
	"strings"
	"testing"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/logger/mocklogger"
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
)

func createTestWorkspaceData() ioworkspace.WorkspaceData {
	return createTestWorkspaceDataWithIDs()
}

func createTestWorkspaceDataWithIDs() ioworkspace.WorkspaceData {
	workspaceID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	folderID := idwrap.NewNow()
	endpointID := idwrap.NewNow()
	exampleID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	nodeID := idwrap.NewNow()

	wsData := ioworkspace.WorkspaceData{}

	wsData.Workspace = mworkspace.Workspace{
		ID:   workspaceID,
		Name: "Test Workspace",
	}
	wsData.Collections = []mcollection.Collection{
		{
			ID:          collectionID,
			WorkspaceID: workspaceID,
			Name:        "Test Collection",
		},
	}
	wsData.Folders = []mitemfolder.ItemFolder{
		{
			ID:           folderID,
			Name:         "Test Folder",
			CollectionID: collectionID,
		},
	}
	wsData.Endpoints = []mitemapi.ItemApi{
		{
			ID:           endpointID,
			Name:         "Test Endpoint",
			Url:          "https://example.com/api",
			Method:       "GET",
			CollectionID: collectionID,
			FolderID:     &folderID,
		},
	}
	wsData.Examples = []mitemapiexample.ItemApiExample{
		{
			ID:           exampleID,
			ItemApiID:    endpointID,
			Name:         "Test Example",
			CollectionID: collectionID,
			BodyType:     mitemapiexample.BodyTypeRaw,
		},
	}

	wsData.Rawbodies = append(wsData.Rawbodies, mbodyraw.ExampleBodyRaw{
		Data:          []byte(`{"test": "data"}`),
		VisualizeMode: mbodyraw.VisualizeModeJSON,
		CompressType:  compress.CompressTypeNone,
		ID:            idwrap.NewNow(),
		ExampleID:     exampleID,
	})

	wsData.ExampleHeaders = []mexampleheader.Header{
		{
			ID:          idwrap.NewNow(),
			ExampleID:   exampleID,
			HeaderKey:   "Content-Type",
			Value:       "application/json",
			Description: "Content type header",
			Enable:      true,
		},
	}

	wsData.ExampleQueries = []mexamplequery.Query{
		{
			ID:          idwrap.NewNow(),
			ExampleID:   exampleID,
			QueryKey:    "param",
			Value:       "value",
			Description: "Test query param",
			Enable:      true,
		},
	}

	wsData.ExampleAsserts = []massert.Assert{
		{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			Condition: mcondition.Condition{
				Comparisons: mcondition.Comparison{
					Expression: "response == success",
				},
			},
			Enable: true,
		},
	}

	wsData.Rawbodies = []mbodyraw.ExampleBodyRaw{
		{
			ID:            idwrap.NewNow(),
			ExampleID:     exampleID,
			Data:          []byte(`{"test": "data"}`),
			VisualizeMode: mbodyraw.VisualizeModeJSON,
			CompressType:  compress.CompressTypeNone,
		},
	}

	wsData.FormBodies = []mbodyform.BodyForm{
		{
			ID:          idwrap.NewNow(),
			ExampleID:   exampleID,
			BodyKey:     "form-key",
			Value:       "form-value",
			Description: "Form field",
			Enable:      true,
		},
	}

	wsData.UrlBodies = []mbodyurl.BodyURLEncoded{
		{
			ID:          idwrap.NewNow(),
			ExampleID:   exampleID,
			BodyKey:     "url-key",
			Value:       "url-value",
			Description: "URL encoded field",
			Enable:      true,
		},
	}

	wsData.ExampleResponses = []mexampleresp.ExampleResp{
		{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			Status:    200,
			Duration:  100,
			Body:      []byte(`{"response": "success"}`),
		},
	}

	for _, exampleResp := range wsData.ExampleResponses {

		wsData.ExampleResponseHeaders = []mexamplerespheader.ExampleRespHeader{
			{
				ID:            idwrap.NewNow(),
				ExampleRespID: exampleResp.ID,
				HeaderKey:     "Content-Type",
				Value:         "application/json",
			},
		}

		for _, assert := range wsData.ExampleAsserts {
			wsData.ExampleResponseAsserts = []massertres.AssertResult{
				{
					ID:         idwrap.NewNow(),
					ResponseID: exampleResp.ID,
					AssertID:   assert.ID,
					Result:     true,
				},
			}
		}
	}

	wsData.Flows = []mflow.Flow{
		{
			ID:              flowID,
			WorkspaceID:     workspaceID,
			Name:            "Test Flow",
			VersionParentID: nil,
		},
	}

	wsData.FlowNodes = []mnnode.MNode{
		{
			ID:        nodeID,
			FlowID:    flowID,
			Name:      "Test Node",
			NodeKind:  mnnode.NODE_KIND_REQUEST,
			PositionY: 0.0,
			PositionX: 0.0,
		},
	}

	for _, flowNode := range wsData.FlowNodes {
		switch flowNode.NodeKind {
		case mnnode.NODE_KIND_REQUEST:
			wsData.FlowRequestNodes = append(wsData.FlowRequestNodes, mnrequest.MNRequest{
				FlowNodeID:     flowNode.ID,
				DeltaExampleID: nil,
				EndpointID:     &endpointID,
				ExampleID:      &exampleID,
			})
		case mnnode.NODE_KIND_CONDITION:
			wsData.FlowConditionNodes = append(wsData.FlowConditionNodes, mnif.MNIF{
				FlowNodeID: flowNode.ID,
				Condition: mcondition.Condition{
					Comparisons: mcondition.Comparison{
						Expression: `{{ response }} == "success"`,
					},
				},
			})
		case mnnode.NODE_KIND_NO_OP:
			wsData.FlowNoopNodes = append(wsData.FlowNoopNodes, mnnoop.NoopNode{
				FlowNodeID: flowNode.ID,
				Type:       mnnoop.NODE_NO_OP_KIND_UNSPECIFIED,
			})
		case mnnode.NODE_KIND_FOR:
			wsData.FlowForNodes = append(wsData.FlowForNodes, mnfor.MNFor{
				FlowNodeID: flowNode.ID,
			})
		case mnnode.NODE_KIND_FOR_EACH:
			wsData.FlowForEachNodes = append(wsData.FlowForEachNodes, mnforeach.MNForEach{
				FlowNodeID:     flowNode.ID,
				IterExpression: "array",
			})
		case mnnode.NODE_KIND_JS:
			wsData.FlowJSNodes = append(wsData.FlowJSNodes, mnjs.MNJS{
				FlowNodeID: flowNode.ID,
				Code:       []byte("console.log('test');"),
			})
		}
	}

	return wsData
}

// setupIOWorkspaceService creates and configures the IOWorkspaceService for testing
func setupIOWorkspaceService(ctx context.Context, t *testing.T) (*ioworkspace.IOWorkspaceService, *testutil.BaseDBQueries) {
	base := testutil.CreateBaseDB(ctx, t)
	db := base.DB
	queries := base.Queries

	mockLogger := mocklogger.NewMockLogger()

	// Create services
	workspaceService := sworkspace.New(queries)
	collectionService := scollection.New(queries, mockLogger)
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
	envService := senv.New(queries, mockLogger)
	varService := svar.New(queries, mockLogger)

	// Create IOWorkspaceService
	ioWorkspaceService := ioworkspace.NewIOWorkspaceService(
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

	return ioWorkspaceService, base
}

func TestImportWorkspace(t *testing.T) {
	ctx := context.Background()
	ioWorkspaceService, base := setupIOWorkspaceService(ctx, t)

	// Create test data
	data := createTestWorkspaceData()

	// Test ImportWorkspace
	err := ioWorkspaceService.ImportWorkspace(ctx, data)
	if err != nil {
		t.Fatalf("ImportWorkspace failed: %v", err)
	}

	// Verify workspace was created
	workspace, err := base.Queries.GetWorkspace(ctx, data.Workspace.ID)
	if err != nil {
		t.Fatalf("Failed to get workspace: %v", err)
	}
	if workspace.Name != data.Workspace.Name {
		t.Errorf("Workspace name mismatch: expected %s, got %s", data.Workspace.Name, workspace.Name)
	}

	// Verify collections were created
	collections, err := base.Queries.GetCollectionByWorkspaceID(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("Failed to get collections: %v", err)
	}
	if len(collections) != len(data.Collections) {
		t.Errorf("Collection count mismatch: expected %d, got %d", len(data.Collections), len(collections))
	}
	if len(collections) > 0 && collections[0].Name != data.Collections[0].Name {
		t.Errorf("Collection name mismatch: expected %s, got %s", data.Collections[0].Name, collections[0].Name)
	}

	// Verify folders were created
	folders, err := base.Queries.GetItemFoldersByCollectionID(ctx, data.Collections[0].ID)
	if err != nil {
		t.Fatalf("Failed to get folders: %v", err)
	}
	if len(folders) != len(data.Folders) {
		t.Errorf("Folder count mismatch: expected %d, got %d", len(data.Folders), len(folders))
	}
	if len(folders) > 0 && folders[0].Name != data.Folders[0].Name {
		t.Errorf("Folder name mismatch: expected %s, got %s", data.Folders[0].Name, folders[0].Name)
	}

	// Verify endpoints were created
	endpoints, err := base.Queries.GetItemsApiByCollectionID(ctx, data.Collections[0].ID)
	if err != nil {
		t.Fatalf("Failed to get endpoints: %v", err)
	}
	if len(endpoints) != len(data.Endpoints) {
		t.Errorf("Endpoint count mismatch: expected %d, got %d", len(data.Endpoints), len(endpoints))
	}
	if len(endpoints) > 0 {
		if endpoints[0].Name != data.Endpoints[0].Name {
			t.Errorf("Endpoint name mismatch: expected %s, got %s", data.Endpoints[0].Name, endpoints[0].Name)
		}
		if endpoints[0].Method != data.Endpoints[0].Method {
			t.Errorf("Endpoint method mismatch: expected %s, got %s", data.Endpoints[0].Method, endpoints[0].Method)
		}
		if endpoints[0].Url != data.Endpoints[0].Url {
			t.Errorf("Endpoint URL mismatch: expected %s, got %s", data.Endpoints[0].Url, endpoints[0].Url)
		}
	}

	examples, err := base.Queries.GetItemApiExamples(ctx, data.Endpoints[0].ID)
	if err != nil {
		t.Fatalf("Failed to get example: %v", err)
	}
	if len(examples) != len(data.Examples) {
		t.Errorf("Example count mismatch: expected %d, got %d", len(data.Examples), len(examples))
	}

	flows, err := base.Queries.GetFlowsByWorkspaceID(ctx, data.Workspace.ID)
	if err != nil {
		t.Fatalf("Failed to get flows: %v", err)
	}

	if len(flows) != len(data.Flows) {
		t.Errorf("Flow count mismatch: expected %d, got %d", len(data.Flows), len(flows))
	}

	flowNodes, err := base.Queries.GetFlowNodesByFlowID(ctx, data.Flows[0].ID)
	if err != nil {
		t.Fatalf("Failed to get flow nodes: %v", err)
	}
	if len(flowNodes) != len(data.FlowNodes) {
		t.Errorf("Flow node count mismatch: expected %d, got %d", len(data.FlowNodes), len(flowNodes))
	}
}

func TestExportWorkspace(t *testing.T) {
	ctx := context.Background()
	ioWorkspaceService, _ := setupIOWorkspaceService(ctx, t)

	// Create and import test data
	importData := createTestWorkspaceData()
	err := ioWorkspaceService.ImportWorkspace(ctx, importData)
	if err != nil {
		t.Fatalf("ImportWorkspace failed: %v", err)
	}

	// Test ExportWorkspace - full export without filters
	exportData, err := ioWorkspaceService.ExportWorkspace(ctx, importData.Workspace.ID, ioworkspace.FilterExport{})
	if err != nil {
		t.Fatalf("ExportWorkspace failed: %v", err)
	}

	// Verify exported data matches imported data
	if exportData.Workspace.ID != importData.Workspace.ID {
		t.Errorf("Workspace ID mismatch: expected %v, got %v", importData.Workspace.ID, exportData.Workspace.ID)
	}
	if exportData.Workspace.Name != importData.Workspace.Name {
		t.Errorf("Workspace name mismatch: expected %s, got %s", importData.Workspace.Name, exportData.Workspace.Name)
	}

	// Check collections
	if len(exportData.Collections) != len(importData.Collections) {
		t.Errorf("Collection count mismatch: expected %d, got %d", len(importData.Collections), len(exportData.Collections))
	} else if len(exportData.Collections) > 0 {
		if exportData.Collections[0].Name != importData.Collections[0].Name {
			t.Errorf("Collection name mismatch: expected %s, got %s",
				importData.Collections[0].Name, exportData.Collections[0].Name)
		}
		if exportData.Collections[0].ID != importData.Collections[0].ID {
			t.Errorf("Collection ID mismatch: expected %v, got %v",
				importData.Collections[0].ID, exportData.Collections[0].ID)
		}
	}

	// Check folders
	if len(exportData.Folders) != len(importData.Folders) {
		t.Errorf("Folder count mismatch: expected %d, got %d", len(importData.Folders), len(exportData.Folders))
	}

	// Check endpoints
	if len(exportData.Endpoints) != len(importData.Endpoints) {
		t.Errorf("Endpoint count mismatch: expected %d, got %d", len(importData.Endpoints), len(exportData.Endpoints))
	}

	// Check examples
	if len(exportData.Examples) != len(importData.Examples) {
		t.Errorf("Example count mismatch: expected %d, got %d", len(importData.Examples), len(exportData.Examples))
	}

	// Check flows
	if len(exportData.Flows) != len(importData.Flows) {
		t.Errorf("Flow count mismatch: expected %d, got %d", len(importData.Flows), len(exportData.Flows))
	}

	// Check flow nodes
	if len(exportData.FlowNodes) != len(importData.FlowNodes) {
		t.Errorf("Flow node count mismatch: expected %d, got %d", len(importData.FlowNodes), len(exportData.FlowNodes))
	}
}

func TestFilteredExport(t *testing.T) {
	ctx := context.Background()
	ioWorkspaceService, _ := setupIOWorkspaceService(ctx, t)

	// Create and import test data
	importData := createTestWorkspaceData()
	err := ioWorkspaceService.ImportWorkspace(ctx, importData)
	if err != nil {
		t.Fatalf("ImportWorkspace failed: %v", err)
	}

	// Create a filter for specific examples
	exampleIDs := []idwrap.IDWrap{importData.Examples[0].ID}
	filterExport := ioworkspace.FilterExport{
		FilterExampleIds: &exampleIDs,
		FilterFlowIds:    nil, // Include all flows
	}

	// Test ExportWorkspace with example filter
	exportData, err := ioWorkspaceService.ExportWorkspace(ctx, importData.Workspace.ID, filterExport)
	if err != nil {
		t.Fatalf("ExportWorkspace failed: %v", err)
	}

	// Should still contain all examples since we filtered for the only example we have
	if len(exportData.Examples) != len(importData.Examples) {
		t.Errorf("Example count mismatch with example filter: expected %d, got %d",
			len(importData.Examples), len(exportData.Examples))
	}

	// Create a filter for specific flows
	flowIDs := []idwrap.IDWrap{importData.Flows[0].ID}
	filterExport = ioworkspace.FilterExport{
		FilterExampleIds: nil, // Include all examples
		FilterFlowIds:    &flowIDs,
	}

	// Test ExportWorkspace with flow filter
	exportData, err = ioWorkspaceService.ExportWorkspace(ctx, importData.Workspace.ID, filterExport)
	if err != nil {
		t.Fatalf("ExportWorkspace failed: %v", err)
	}

	// Should still contain all flows since we filtered for the only flow we have
	if len(exportData.Flows) != len(importData.Flows) {
		t.Errorf("Flow count mismatch with flow filter: expected %d, got %d",
			len(importData.Flows), len(exportData.Flows))
	}
}

func TestImportExportRoundtrip(t *testing.T) {
	ctx := context.Background()
	ioWorkspaceService, _ := setupIOWorkspaceService(ctx, t)

	// Create and import initial test data
	originalData := createTestWorkspaceData()
	err := ioWorkspaceService.ImportWorkspace(ctx, originalData)
	if err != nil {
		t.Fatalf("Initial import failed: %v", err)
	}

	// Export the data
	exportData, err := ioWorkspaceService.ExportWorkspace(ctx, originalData.Workspace.ID, ioworkspace.FilterExport{})
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Modify the workspace ID to simulate importing to a new workspace
	newWorkspaceID := idwrap.NewNow()
	exportData.Workspace.ID = newWorkspaceID

	// Update all workspaceId references
	for i := range exportData.Collections {
		exportData.Collections[i].WorkspaceID = newWorkspaceID
	}

	for i := range exportData.Flows {
		exportData.Flows[i].WorkspaceID = newWorkspaceID
	}

	// Re-import the exported data
	err = ioWorkspaceService.ImportWorkspace(ctx, *exportData)
	if err == nil {
		t.Fatalf("Re-import should have failed due to duplicate workspace ID")
	}
}

// TODO: talk with team
/*
func TestModifyAndReimportWorkspace(t *testing.T) {
	ctx := context.Background()
	ioWorkspaceService, base := setupIOWorkspaceService(ctx, t)

	// Create and import initial test data
	originalData := createTestWorkspaceData()
	err := ioWorkspaceService.ImportWorkspace(ctx, originalData)
	if err != nil {
		t.Fatalf("Initial import failed: %v", err)
	}

	// Export the data
	exportData, err := ioWorkspaceService.ExportWorkspace(ctx, originalData.Workspace.ID, ioworkspace.FilterExport{})
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Modify the workspace name
	const updatedName = "Updated Workspace Name"
	exportData.Workspace.Name = updatedName

	// Add a new collection
	newCollectionID := idwrap.NewNow()
	exportData.Collections = append(exportData.Collections, mcollection.Collection{
		ID:      newCollectionID,
		OwnerID: exportData.Workspace.ID,
		Name:    "New Collection",
	})

	// Re-import the modified data
	err = ioWorkspaceService.ImportWorkspace(ctx, *exportData)
	if err != nil {
		t.Fatalf("Re-import failed: %v", err)
	}

	// Verify the workspace was updated
	workspace, err := base.Queries.GetWorkspace(ctx, originalData.Workspace.ID)
	if err != nil {
		t.Fatalf("Failed to get updated workspace: %v", err)
	}
	if workspace.Name != updatedName {
		t.Errorf("Updated workspace name mismatch: expected %s, got %s", updatedName, workspace.Name)
	}

	// Verify new collection was added
	collections, err := base.Queries.GetCollectionByOwnerID(ctx, originalData.Workspace.ID)
	if err != nil {
		t.Fatalf("Failed to get collections after update: %v", err)
	}
	if len(collections) != len(originalData.Collections)+1 {
		t.Errorf("Collection count after update mismatch: expected %d, got %d",
			len(originalData.Collections)+1, len(collections))
	}

	foundNewCollection := false
	for _, collection := range collections {
		if collection.ID == newCollectionID {
			foundNewCollection = true
			if collection.Name != "New Collection" {
				t.Errorf("New collection name mismatch: expected %s, got %s", "New Collection", collection.Name)
			}
			break
		}
	}
	if !foundNewCollection {
		t.Errorf("New collection was not found after re-import")
	}
}
*/

func TestImportMultipleWorkspaces(t *testing.T) {
	ctx := context.Background()
	ioWorkspaceService, base := setupIOWorkspaceService(ctx, t)

	// Create first workspace data
	workspace1 := createTestWorkspaceData()
	workspace1.Workspace.Name = "Workspace 1"

	// Create second workspace data - createTestWorkspaceData now generates unique IDs
	workspace2 := createTestWorkspaceData()
	workspace2.Workspace.Name = "Workspace 2"

	// Import both workspaces
	err := ioWorkspaceService.ImportWorkspace(ctx, workspace1)
	if err != nil {
		t.Fatalf("Import of workspace1 failed: %v", err)
	}

	err = ioWorkspaceService.ImportWorkspace(ctx, workspace2)
	if err != nil {
		t.Fatalf("Import of workspace2 failed: %v", err)
	}

	// Verify both workspaces exist
	workspace1DB, err := base.Queries.GetWorkspace(ctx, workspace1.Workspace.ID)
	if err != nil {
		t.Fatalf("Failed to get workspace1: %v", err)
	}
	if workspace1DB.Name != workspace1.Workspace.Name {
		t.Errorf("Workspace1 name mismatch: expected %s, got %s",
			workspace1.Workspace.Name, workspace1DB.Name)
	}

	workspace2DB, err := base.Queries.GetWorkspace(ctx, workspace2.Workspace.ID)
	if err != nil {
		t.Fatalf("Failed to get workspace2: %v", err)
	}
	if workspace2DB.Name != workspace2.Workspace.Name {
		t.Errorf("Workspace2 name mismatch: expected %s, got %s",
			workspace2.Workspace.Name, workspace2DB.Name)
	}
}

func TestImportWorkspaceWithLongFlowName(t *testing.T) {
	ctx := context.Background()
	ioWorkspaceService, base := setupIOWorkspaceService(ctx, t)

	// Create test data with a flow that has a very long name
	data := createTestWorkspaceData()
	longName := strings.Repeat("Very long flow name ", 50) // Create a name that's approximately 1000 characters
	data.Flows[0].Name = longName

	// Test ImportWorkspace with long flow name
	err := ioWorkspaceService.ImportWorkspace(ctx, data)
	if err != nil {
		t.Fatalf("ImportWorkspace failed with long flow name: %v", err)
	}

	// Verify flow was created with the long name
	flow, err := base.Queries.GetFlow(ctx, data.Flows[0].ID)
	if err != nil {
		t.Fatalf("Failed to get flow: %v", err)
	}
	if flow.Name != longName {
		t.Errorf("Flow name mismatch: expected long name of length %d, got name of length %d",
			len(longName), len(flow.Name))
	}
}

func TestExportWithExampleFilterIncludingFlowReferences(t *testing.T) {
	ctx := context.Background()
	ioWorkspaceService, _ := setupIOWorkspaceService(ctx, t)

	// Create base test data
	importData := createTestWorkspaceData()

	// Add a second example to the same endpoint
	secondExampleID := idwrap.NewNow()
	secondExample := mitemapiexample.ItemApiExample{
		ID:           secondExampleID,
		ItemApiID:    importData.Endpoints[0].ID,
		Name:         "Second Test Example",
		CollectionID: importData.Collections[0].ID,
		BodyType:     mitemapiexample.BodyTypeNone,
	}
	importData.Examples = append(importData.Examples, secondExample)
	rawBody := mbodyraw.ExampleBodyRaw{
		ID:        idwrap.NewNow(),
		ExampleID: secondExampleID,
	}
	importData.Rawbodies = append(importData.Rawbodies, rawBody)

	// Add a header for the second example
	importData.ExampleHeaders = append(importData.ExampleHeaders, mexampleheader.Header{
		ID:        idwrap.NewNow(),
		ExampleID: secondExampleID,
		HeaderKey: "X-Second-Example",
		Value:     "true",
		Enable:    true,
	})

	// Ensure the flow node references the *first* example
	if len(importData.FlowNodes) > 0 && len(importData.FlowRequestNodes) > 0 {
		firstExampleID := importData.Examples[0].ID
		importData.FlowRequestNodes[0].ExampleID = &firstExampleID
	} else {
		t.Fatal("Test setup error: No flow or request node found in initial data")
	}

	// Import the modified data
	err := ioWorkspaceService.ImportWorkspace(ctx, importData)
	if err != nil {
		t.Fatalf("ImportWorkspace failed: %v", err)
	}

	// Create a filter that explicitly requests *only* the second example
	filterExampleIDs := []idwrap.IDWrap{secondExampleID}
	filterExport := ioworkspace.FilterExport{
		FilterExampleIds: &filterExampleIDs,
		FilterFlowIds:    nil, // Include all flows
	}

	// Test ExportWorkspace with the filter
	exportData, err := ioWorkspaceService.ExportWorkspace(ctx, importData.Workspace.ID, filterExport)
	if err != nil {
		t.Fatalf("ExportWorkspace failed: %v", err)
	}

	// --- Verification ---

	// Verify both examples are present in the export
	if len(exportData.Examples) != 2 {
		t.Errorf("Expected 2 examples in export (1 filtered + 1 from flow), got %d", len(exportData.Examples))
	}

	foundFirstExample := false
	foundSecondExample := false
	for _, ex := range exportData.Examples {
		if ex.ID == importData.Examples[0].ID {
			foundFirstExample = true
		}
		if ex.ID == secondExampleID {
			foundSecondExample = true
		}
	}
	if !foundFirstExample {
		t.Errorf("First example (referenced by flow) was not found in export")
	}
	if !foundSecondExample {
		t.Errorf("Second example (explicitly filtered) was not found in export")
	}

	// Verify related data for both examples is present (e.g., headers)
	if len(exportData.ExampleHeaders) < 2 { // Should have at least one header from each example
		t.Errorf("Expected at least 2 example headers in export, got %d", len(exportData.ExampleHeaders))
	}
	// Add more checks for bodies, queries, asserts etc. if needed
}

func TestExportWithOrphanedFlowExample(t *testing.T) {
	ctx := context.Background()
	ioWorkspaceService, _ := setupIOWorkspaceService(ctx, t)

	// Create base test data
	importData := createTestWorkspaceData()

	// Create an "orphaned" example (not directly linked to the main endpoint initially)
	orphanedExampleID := idwrap.NewNow()
	orphanedExample := mitemapiexample.ItemApiExample{
		ID:           orphanedExampleID,
		ItemApiID:    importData.Endpoints[0].ID, // Link to endpoint for DB consistency
		Name:         "Orphaned Flow Example",
		CollectionID: importData.Collections[0].ID,
		BodyType:     mitemapiexample.BodyTypeRaw,
	}
	importData.Examples = append(importData.Examples, orphanedExample)

	// Add some data for the orphaned example
	importData.Rawbodies = append(importData.Rawbodies, mbodyraw.ExampleBodyRaw{
		ID:        idwrap.NewNow(),
		ExampleID: orphanedExampleID,
		Data:      []byte(`{"orphaned": true}`),
	})
	importData.ExampleHeaders = append(importData.ExampleHeaders, mexampleheader.Header{
		ID:        idwrap.NewNow(),
		ExampleID: orphanedExampleID,
		HeaderKey: "X-Orphaned",
		Value:     "yes",
		Enable:    true,
	})

	// Ensure the flow node references this *orphaned* example
	if len(importData.FlowNodes) > 0 && len(importData.FlowRequestNodes) > 0 {
		importData.FlowRequestNodes[0].ExampleID = &orphanedExampleID
	} else {
		t.Fatal("Test setup error: No flow or request node found in initial data")
	}

	// Import the modified data
	err := ioWorkspaceService.ImportWorkspace(ctx, importData)
	if err != nil {
		t.Fatalf("ImportWorkspace failed: %v", err)
	}

	// Create a filter that explicitly requests *only* the *first* example
	// The orphaned example should still be included because the flow needs it.
	filterExampleIDs := []idwrap.IDWrap{importData.Examples[0].ID}
	filterExport := ioworkspace.FilterExport{
		FilterExampleIds: &filterExampleIDs,
		FilterFlowIds:    nil, // Include all flows (which includes the one referencing the orphaned example)
	}

	// Test ExportWorkspace with the filter
	exportData, err := ioWorkspaceService.ExportWorkspace(ctx, importData.Workspace.ID, filterExport)
	if err != nil {
		t.Fatalf("ExportWorkspace failed: %v", err)
	}

	// --- Verification ---

	// Verify both examples are present in the export
	if len(exportData.Examples) != 2 {
		t.Errorf("Expected 2 examples in export (1 filtered + 1 orphaned from flow), got %d", len(exportData.Examples))
	}

	foundFirstExample := false
	foundOrphanedExample := false
	for _, ex := range exportData.Examples {
		if ex.ID == importData.Examples[0].ID {
			foundFirstExample = true
		}
		if ex.ID == orphanedExampleID {
			foundOrphanedExample = true
		}
	}
	if !foundFirstExample {
		t.Errorf("First example (explicitly filtered) was not found in export")
	}
	if !foundOrphanedExample {
		t.Errorf("Orphaned example (referenced by flow) was not found in export")
	}

	// Verify related data for the orphaned example is present
	foundOrphanedHeader := false
	for _, h := range exportData.ExampleHeaders {
		if h.ExampleID == orphanedExampleID && h.HeaderKey == "X-Orphaned" {
			foundOrphanedHeader = true
			break
		}
	}
	if !foundOrphanedHeader {
		t.Errorf("Header for orphaned example was not found in export")
	}

	foundOrphanedBody := false
	for _, b := range exportData.Rawbodies {
		if b.ExampleID == orphanedExampleID {
			foundOrphanedBody = true
			break
		}
	}
	if !foundOrphanedBody {
		t.Errorf("Raw body for orphaned example was not found in export")
	}
}

func TestUnmarshalWorkflowYAML(t *testing.T) {
	// YAML workflow definition
	yamlData := `
workspace_name: Example Workflow Workspace

flows:
  - name: UserDataProcessingFlow
    variables:
      - name: auth_token
        value: "bearer_token_123"
      - name: base_url
        value: "https://api.example.com"
    steps:
      # Request node - matches nrequest implementation
      - request:
          name: GetUser
          url: "{{base_url}}/users/1"
          method: GET
          headers:
            - name: Authorization
              value: "Bearer {{auth_token}}"
            - name: Accept
              value: "application/json"
          body:
            body_json:
                jsonRoot:
                  - JsonArray1: "{{auth_token}}"
                  - JsonArray2:
                    - NestedArray1: 1
                    - NestedArray2: 2
      # If node - matches nif implementation
      - if:
          name: CheckUserStatus
          expression: "GetUser-1.response.status == 200"
          then: GetUserPosts
          else: HandleError

      # Request node that's a target of the if-then branch
      - request:
          name: GetUserPosts
          url: "{{base_url}}/users/{{GetUser.response.body.id}}/posts"
          method: GET
          headers:
            - name: Authorization
              value: "Bearer {{auth_token}}"
            - name: Accept
              value: "application/json"

      # JS node that's a target of the if-else branch
      - js:
          name: HandleError
          code: |
            console.error("Failed to get user data");
            return { error: true, message: "User data fetch failed" };

      # For loop node - matches nfor implementation
      - for:
          name: ProcessPosts
          depends_on:
            - GetUserPosts
          # These match the actual nfor implementation
          iter_count: 5  # Maps to IterCount
          loop: ProcessSinglePost # Target node to execute in loop

      # Request node that's executed inside the loop
      - request:
          name: ProcessSinglePost
          url: "{{base_url}}/posts/something"
          method: GET
          headers:
            - name: Authorization
              value: "Bearer {{auth_token}}"
          # This is the loop body - it runs repeatedly

      # Final JS node
      - js:
          name: FinalSummary
          depends_on:
            - ProcessPosts
          code: |
            console.log("Flow completed successfully");
            return {
              status: "success",
              processedCount: Math.min(5, {{GetUserPosts.response.body.length}})
            };
`

	// Call the function to parse the YAML
	workspaceData, err := ioworkspace.UnmarshalWorkflowYAML([]byte(yamlData))
	if err != nil {
		t.Fatalf("Failed to unmarshal workflow YAML: %v", err)
	}

	// Verify workspace structure
	if workspaceData.Workspace.Name != "Example Workflow Workspace" {
		t.Errorf("Expected workspace name 'Example Workflow Workspace', got '%s'", workspaceData.Workspace.Name)
	}

	// Verify collections
	if len(workspaceData.Collections) != 1 {
		t.Fatalf("Expected 1 collection, got %d", len(workspaceData.Collections))
	}
	if workspaceData.Collections[0].Name != "Workflow Collection" {
		t.Errorf("Expected collection name 'Workflow Collection', got '%s'", workspaceData.Collections[0].Name)
	}

	// Verify flows
	if len(workspaceData.Flows) != 1 {
		t.Fatalf("Expected 1 flow, got %d", len(workspaceData.Flows))
	}
	if workspaceData.Flows[0].Name != "UserDataProcessingFlow" {
		t.Errorf("Expected flow name 'UserDataProcessingFlow', got '%s'", workspaceData.Flows[0].Name)
	}

	// Verify flow variables
	if len(workspaceData.FlowVariables) != 2 {
		t.Fatalf("Expected 2 flow variables, got %d", len(workspaceData.FlowVariables))
	}

	// Map node IDs to names for easier testing
	nodeNameToID := make(map[string]idwrap.IDWrap)
	nodeIDToName := make(map[idwrap.IDWrap]string)
	nodeTypes := make(map[string]mnnode.NodeKind)
	for _, node := range workspaceData.FlowNodes {
		nodeNameToID[node.Name] = node.ID
		nodeIDToName[node.ID] = node.Name
		nodeTypes[node.Name] = node.NodeKind
	}

	// Verify expected nodes exist
	expectedNodes := map[string]mnnode.NodeKind{
		"Start Node":        mnnode.NODE_KIND_NO_OP,
		"GetUser":           mnnode.NODE_KIND_REQUEST,
		"CheckUserStatus":   mnnode.NODE_KIND_CONDITION,
		"GetUserPosts":      mnnode.NODE_KIND_REQUEST,
		"HandleError":       mnnode.NODE_KIND_JS,
		"ProcessPosts":      mnnode.NODE_KIND_FOR,
		"ProcessSinglePost": mnnode.NODE_KIND_REQUEST,
		"FinalSummary":      mnnode.NODE_KIND_JS,
	}
	if len(workspaceData.FlowNodes) != len(expectedNodes) {
		t.Fatalf("Expected %d nodes, got %d", len(expectedNodes), len(workspaceData.FlowNodes))
	}
	for nodeName, expectedType := range expectedNodes {
		actualType, exists := nodeTypes[nodeName]
		if !exists {
			t.Errorf("Node '%s' not found", nodeName)
		} else if actualType != expectedType {
			t.Errorf("Node '%s' has type %v, expected %v", nodeName, actualType, expectedType)
		}
	}

	// Test node connections (edges)
	if len(workspaceData.FlowEdges) < 6 {
		t.Errorf("Expected at least 6 edges, got %d", len(workspaceData.FlowEdges))
	}

	// Helper function to check if edge exists
	edgeExists := func(sourceNode, targetNode string, handler edge.EdgeHandle) bool {
		sourceID, sourceExists := nodeNameToID[sourceNode]
		targetID, targetExists := nodeNameToID[targetNode]
		if !sourceExists {
			t.Errorf("Source node '%s' not found in nodeNameToID map", sourceNode)
			return false
		}
		if !targetExists {
			t.Errorf("Target node '%s' not found in nodeNameToID map", targetNode)
			return false
		}

		found := false
		for _, e := range workspaceData.FlowEdges {
			if e.SourceID == sourceID && e.TargetID == targetID && e.SourceHandler == handler {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("Expected edge from '%s' (%v) to '%s' (%v) with handler %v not found.",
				sourceNode, sourceID, targetNode, targetID, handler)
			// Print all edges from the source node for debugging
			t.Logf("Edges from source node '%s' (%v):", sourceNode, sourceID)
			for _, e := range workspaceData.FlowEdges {
				if e.SourceID == sourceID {
					targetName := nodeIDToName[e.TargetID]
					t.Logf("  - Target: '%s' (%v), Handler: %v", targetName, e.TargetID, e.SourceHandler)
				}
			}
		}

		return found
	}

	// Check specific edges
	if !edgeExists("CheckUserStatus", "GetUserPosts", edge.HandleThen) {
		t.Errorf("Missing 'then' edge from CheckUserStatus to GetUserPosts")
	}
	if !edgeExists("CheckUserStatus", "HandleError", edge.HandleElse) {
		t.Errorf("Missing 'else' edge from CheckUserStatus to HandleError")
	}
	if !edgeExists("ProcessPosts", "ProcessSinglePost", edge.HandleLoop) {
		t.Errorf("Missing 'loop' edge from ProcessPosts to ProcessSinglePost")
	}
	if !edgeExists("GetUserPosts", "ProcessPosts", edge.HandleUnspecified) {
		t.Errorf("Missing dependency edge from GetUserPosts to ProcessPosts")
	}
	if !edgeExists("ProcessPosts", "FinalSummary", edge.HandleUnspecified) {
		t.Errorf("Missing dependency edge from ProcessPosts to FinalSummary")
	}

	// Check request nodes
	requestCount := 0
	for _, reqNode := range workspaceData.FlowRequestNodes {
		requestCount++
		nodeName := nodeIDToName[reqNode.FlowNodeID]

		if reqNode.EndpointID == nil {
			t.Errorf("Request node '%s' has no endpoint ID", nodeName)
			continue
		}

		// Find corresponding endpoint
		var endpoint *mitemapi.ItemApi
		for i := range workspaceData.Endpoints {
			if workspaceData.Endpoints[i].ID == *reqNode.EndpointID {
				endpoint = &workspaceData.Endpoints[i]
				break
			}
		}

		if endpoint == nil {
			t.Errorf("No endpoint found for request node '%s'", nodeName)
			continue
		}

		// Check URL and method for specific nodes
		switch nodeName {
		case "GetUser":
			if endpoint.Url != "{{base_url}}/users/1" || endpoint.Method != "GET" {
				t.Errorf("GetUser endpoint incorrect: got URL '%s', method '%s'", endpoint.Url, endpoint.Method)
			}
		case "GetUserPosts":
			if !strings.Contains(endpoint.Url, "{{base_url}}/users/") {
				t.Errorf("GetUserPosts endpoint has incorrect URL: '%s'", endpoint.Url)
			}
		}
	}
	if requestCount != 3 {
		t.Errorf("Expected 3 request nodes, got %d", requestCount)
	}

	// Check JS nodes
	jsCount := 0
	for _, jsNode := range workspaceData.FlowJSNodes {
		jsCount++
		nodeName := nodeIDToName[jsNode.FlowNodeID]

		switch nodeName {
		case "HandleError":
			if !strings.Contains(string(jsNode.Code), "Failed to get user data") {
				t.Errorf("HandleError code missing expected content")
			}
		case "FinalSummary":
			if !strings.Contains(string(jsNode.Code), "Flow completed successfully") {
				t.Errorf("FinalSummary code missing expected content")
			}
		}
	}
	if jsCount != 2 {
		t.Errorf("Expected 2 JS nodes, got %d", jsCount)
	}

	// Check for loop node
	forCount := 0
	for _, forNode := range workspaceData.FlowForNodes {
		forCount++
		nodeName := nodeIDToName[forNode.FlowNodeID]
		if nodeName == "ProcessPosts" && forNode.IterCount != 5 {
			t.Errorf("ProcessPosts node has incorrect iteration count: got %d, expected 5", forNode.IterCount)
		}
	}
	if forCount != 1 {
		t.Errorf("Expected 1 for loop node, got %d", forCount)
	}
}

func TestMarshalWorkflowYAML(t *testing.T) {
	// Create a workspace data structure with a flow and nodes
	wsData := ioworkspace.WorkspaceData{}

	// Setup workspace
	wsID := idwrap.NewNow()
	wsData.Workspace = mworkspace.Workspace{
		ID:   wsID,
		Name: "Test Workflow Workspace",
	}

	// Setup collection
	collID := idwrap.NewNow()
	wsData.Collections = []mcollection.Collection{
		{
			ID:          collID,
			WorkspaceID: wsID,
			Name:        "Test Collection",
		},
	}

	// Setup flow
	flowID := idwrap.NewNow()
	wsData.Flows = []mflow.Flow{
		{
			ID:          flowID,
			WorkspaceID: wsID,
			Name:        "TestFlow",
		},
	}

	// Add variables
	wsData.FlowVariables = []mflowvariable.FlowVariable{
		{
			ID:     idwrap.NewNow(),
			FlowID: flowID,
			Name:   "api_url",
			Value:  "https://api.example.com",
		},
		{
			ID:     idwrap.NewNow(),
			FlowID: flowID,
			Name:   "auth_token",
			Value:  "token123",
		},
	}

	// Create nodes
	requestNodeID := idwrap.NewNow()
	ifNodeID := idwrap.NewNow()
	jsNodeID := idwrap.NewNow()
	forNodeID := idwrap.NewNow()
	loopBodyNodeID := idwrap.NewNow()

	// Add nodes to flow
	wsData.FlowNodes = []mnnode.MNode{
		{
			ID:       requestNodeID,
			FlowID:   flowID,
			Name:     "GetData",
			NodeKind: mnnode.NODE_KIND_REQUEST,
		},
		{
			ID:       ifNodeID,
			FlowID:   flowID,
			Name:     "CheckResponse",
			NodeKind: mnnode.NODE_KIND_CONDITION,
		},
		{
			ID:       jsNodeID,
			FlowID:   flowID,
			Name:     "ProcessData",
			NodeKind: mnnode.NODE_KIND_JS,
		},
		{
			ID:       forNodeID,
			FlowID:   flowID,
			Name:     "ProcessItems",
			NodeKind: mnnode.NODE_KIND_FOR,
		},
		{
			ID:       loopBodyNodeID,
			FlowID:   flowID,
			Name:     "ProcessItem",
			NodeKind: mnnode.NODE_KIND_REQUEST,
		},
	}

	// Setup endpoint for request node
	endpointID := idwrap.NewNow()
	exampleID := idwrap.NewNow()
	wsData.Endpoints = []mitemapi.ItemApi{
		{
			ID:           endpointID,
			CollectionID: collID,
			Name:         "TestEndpoint",
			Method:       "GET",
			Url:          "{{api_url}}/data",
		},
	}

	// Setup example
	wsData.Examples = []mitemapiexample.ItemApiExample{
		{
			ID:           exampleID,
			ItemApiID:    endpointID,
			CollectionID: collID,
			Name:         "TestExample",
			BodyType:     mitemapiexample.BodyTypeRaw,
		},
	}

	// Add headers
	wsData.ExampleHeaders = []mexampleheader.Header{
		{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			HeaderKey: "Authorization",
			Value:     "Bearer {{auth_token}}",
			Enable:    true,
		},
		{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			HeaderKey: "Content-Type",
			Value:     "application/json",
			Enable:    true,
		},
	}

	// Add body
	wsData.Rawbodies = []mbodyraw.ExampleBodyRaw{
		{
			ID:            idwrap.NewNow(),
			ExampleID:     exampleID,
			Data:          []byte(`{"query": "test"}`),
			VisualizeMode: mbodyraw.VisualizeModeJSON,
		},
	}

	// Setup request node
	wsData.FlowRequestNodes = []mnrequest.MNRequest{
		{
			FlowNodeID: requestNodeID,
			EndpointID: &endpointID,
			ExampleID:  &exampleID,
		},
		{
			FlowNodeID: loopBodyNodeID,
			EndpointID: &endpointID,
		},
	}

	// Setup condition node
	wsData.FlowConditionNodes = []mnif.MNIF{
		{
			FlowNodeID: ifNodeID,
			Condition: mcondition.Condition{
				Comparisons: mcondition.Comparison{
					Expression: "GetData.response.status == 200",
				},
			},
		},
	}

	// Setup JS node
	wsData.FlowJSNodes = []mnjs.MNJS{
		{
			FlowNodeID: jsNodeID,
			Code:       []byte(`console.log("Processing data"); return { processed: true };`),
		},
	}

	// Setup for loop node
	wsData.FlowForNodes = []mnfor.MNFor{
		{
			FlowNodeID: forNodeID,
			IterCount:  3,
		},
	}

	// Setup edges
	wsData.FlowEdges = []edge.Edge{
		{
			ID:            idwrap.NewNow(),
			FlowID:        flowID,
			SourceID:      requestNodeID,
			TargetID:      ifNodeID,
			SourceHandler: edge.HandleUnspecified,
		},
		{
			ID:            idwrap.NewNow(),
			FlowID:        flowID,
			SourceID:      ifNodeID,
			TargetID:      jsNodeID,
			SourceHandler: edge.HandleThen,
		},
		{
			ID:            idwrap.NewNow(),
			FlowID:        flowID,
			SourceID:      jsNodeID,
			TargetID:      forNodeID,
			SourceHandler: edge.HandleUnspecified,
		},
		{
			ID:            idwrap.NewNow(),
			FlowID:        flowID,
			SourceID:      forNodeID,
			TargetID:      loopBodyNodeID,
			SourceHandler: edge.HandleLoop,
		},
	}

	// Marshal to YAML
	yamlData, err := ioworkspace.MarshalWorkflowYAML(&wsData)
	if err != nil {
		t.Fatalf("Failed to marshal workflow to YAML: %v", err)
	}

	// Verify YAML content
	yamlStr := string(yamlData)

	// Check for expected content
	expectedParts := []string{
		"workspace_name: Test Workflow Workspace",
		"name: TestFlow",
		"- name: api_url",
		"value: https://api.example.com",
		"- name: auth_token",
		"value: token123",
		"name: GetData",
		"method: GET",
		"url: '{{api_url}}/data'",
		"name: Authorization",
		"value: Bearer {{auth_token}}",
		"name: CheckResponse",
		"expression: GetData.response.status == 200",
		"name: ProcessData",
		"code: 'console.log(\"Processing data\"); return { processed: true };'",
		"name: ProcessItems",
		"iter_count: 3",
	}

	for _, part := range expectedParts {
		if !strings.Contains(yamlStr, part) {
			t.Errorf("Expected YAML to contain '%s', but it was not found %s", part, yamlStr)
		}
	}
}
