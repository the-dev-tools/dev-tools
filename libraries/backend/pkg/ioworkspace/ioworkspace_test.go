package ioworkspace_test

import (
	"context"
	"testing"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/ioworkspace"
	"the-dev-tools/backend/pkg/model/massert"
	"the-dev-tools/backend/pkg/model/massertres"
	"the-dev-tools/backend/pkg/model/mbodyform"
	"the-dev-tools/backend/pkg/model/mbodyraw"
	"the-dev-tools/backend/pkg/model/mbodyurl"
	"the-dev-tools/backend/pkg/model/mcollection"
	"the-dev-tools/backend/pkg/model/mcondition"
	"the-dev-tools/backend/pkg/model/mexampleheader"
	"the-dev-tools/backend/pkg/model/mexamplequery"
	"the-dev-tools/backend/pkg/model/mexampleresp"
	"the-dev-tools/backend/pkg/model/mexamplerespheader"
	"the-dev-tools/backend/pkg/model/mflow"
	"the-dev-tools/backend/pkg/model/mitemapi"
	"the-dev-tools/backend/pkg/model/mitemapiexample"
	"the-dev-tools/backend/pkg/model/mitemfolder"
	"the-dev-tools/backend/pkg/model/mnnode"
	"the-dev-tools/backend/pkg/model/mnnode/mnfor"
	"the-dev-tools/backend/pkg/model/mnnode/mnforeach"
	"the-dev-tools/backend/pkg/model/mnnode/mnif"
	"the-dev-tools/backend/pkg/model/mnnode/mnjs"
	"the-dev-tools/backend/pkg/model/mnnode/mnnoop"
	"the-dev-tools/backend/pkg/model/mnnode/mnrequest"
	"the-dev-tools/backend/pkg/model/mworkspace"
	"the-dev-tools/backend/pkg/service/sassert"
	"the-dev-tools/backend/pkg/service/sassertres"
	"the-dev-tools/backend/pkg/service/sbodyform"
	"the-dev-tools/backend/pkg/service/sbodyraw"
	"the-dev-tools/backend/pkg/service/sbodyurl"
	"the-dev-tools/backend/pkg/service/scollection"
	"the-dev-tools/backend/pkg/service/sexampleheader"
	"the-dev-tools/backend/pkg/service/sexamplequery"
	"the-dev-tools/backend/pkg/service/sexampleresp"
	"the-dev-tools/backend/pkg/service/sexamplerespheader"
	"the-dev-tools/backend/pkg/service/sflow"
	"the-dev-tools/backend/pkg/service/sitemapi"
	"the-dev-tools/backend/pkg/service/sitemapiexample"
	"the-dev-tools/backend/pkg/service/sitemfolder"
	"the-dev-tools/backend/pkg/service/snode"
	"the-dev-tools/backend/pkg/service/snodefor"
	"the-dev-tools/backend/pkg/service/snodeforeach"
	"the-dev-tools/backend/pkg/service/snodeif"
	"the-dev-tools/backend/pkg/service/snodejs"
	"the-dev-tools/backend/pkg/service/snodenoop"
	"the-dev-tools/backend/pkg/service/snoderequest"
	"the-dev-tools/backend/pkg/service/sworkspace"
	"the-dev-tools/backend/pkg/testutil"
)

func createTestWorkspaceData() ioworkspace.WorkspaceData {
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
			ID:      collectionID,
			OwnerID: workspaceID,
			Name:    "Test Collection",
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
			Type:      massert.AssertTypeEqual,
			Path:      "response",
			Value:     "success",
			Enable:    true,
		},
	}

	wsData.Rawbodies = []mbodyraw.ExampleBodyRaw{
		{
			ID:            idwrap.NewNow(),
			ExampleID:     exampleID,
			Data:          []byte(`{"test": "data"}`),
			VisualizeMode: mbodyraw.VisualizeModeJSON,
			CompressType:  mbodyraw.CompressTypeNone,
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
			State:     mnnode.NODE_STATE_SUCCESS,
			StateData: []byte("test"),
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
				EndpointID:     nil,
				ExampleID:      nil,
			})
		case mnnode.NODE_KIND_CONDITION:
			wsData.FlowConditionNodes = append(wsData.FlowConditionNodes, mnif.MNIF{
				FlowNodeID: flowNode.ID,
				Condition: mcondition.Condition{
					Comparisons: mcondition.Comparison{
						Kind:  mcondition.COMPARISON_KIND_EQUAL,
						Path:  "response",
						Value: "success",
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
				FlowNodeID: flowNode.ID,
				IterPath:   "array",
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

	// Create services
	workspaceService := sworkspace.New(queries)
	collectionService := scollection.New(queries)
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
	flowRequestService := snoderequest.New(queries)
	flowConditionService := snodeif.New(queries)
	flowNoopService := snodenoop.New(queries)
	flowForService := snodefor.New(queries)
	flowForEachService := snodeforeach.New(queries)
	flowJSService := snodejs.New(queries)

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
		flowRequestService,
		*flowConditionService,
		flowNoopService,
		flowForService,
		flowForEachService,
		flowJSService,
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
	collections, err := base.Queries.GetCollectionByOwnerID(ctx, workspace.ID)
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
}

/*
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

/*
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
		exportData.Collections[i].OwnerID = newWorkspaceID
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

	// Create second workspace data
	workspace2 := createTestWorkspaceData()
	workspace2.Workspace.ID = idwrap.NewNow() // Use a different workspace ID
	workspace2.Workspace.Name = "Workspace 2"

	// Update references in workspace2 to point to the new workspace ID
	for i := range workspace2.Collections {
		workspace2.Collections[i].ID = idwrap.NewNow() // Different collection ID
		workspace2.Collections[i].OwnerID = workspace2.Workspace.ID
	}

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
