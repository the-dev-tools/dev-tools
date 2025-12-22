package ioworkspace

import (
	"testing"
	"time"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mworkspace"

	"github.com/stretchr/testify/require"
)

// TestCountEntities_Empty tests CountEntities on an empty bundle
func TestCountEntities_Empty(t *testing.T) {
	bundle := &WorkspaceBundle{}
	counts := bundle.CountEntities()

	expectedZeroCounts := []string{
		"http_requests",
		"http_search_params",
		"http_headers",
		"http_body_forms",
		"http_body_urlencoded",
		"http_body_raw",
		"http_asserts",
		"files",
		"flows",
		"flow_variables",
		"flow_nodes",
		"flow_edges",
		"flow_request_nodes",
		"flow_condition_nodes",
		"flow_for_nodes",
		"flow_foreach_nodes",
		"flow_js_nodes",
		"environments",
		"environment_vars",
	}

	for _, key := range expectedZeroCounts {
		if counts[key] != 0 {
			t.Errorf("Expected count for %s to be 0, got %d", key, counts[key])
		}
	}
}

// TestCountEntities_WithData tests CountEntities with populated bundle
func TestCountEntities_WithData(t *testing.T) {
	workspaceID := idwrap.NewNow()
	now := time.Now().Unix()

	bundle := &WorkspaceBundle{
		Workspace: mworkspace.Workspace{
			ID:   workspaceID,
			Name: "Test Workspace",
		},
		HTTPRequests: []mhttp.HTTP{
			{ID: idwrap.NewNow(), WorkspaceID: workspaceID, Name: "Request 1", Url: "https://example.com", Method: "GET", CreatedAt: now, UpdatedAt: now},
			{ID: idwrap.NewNow(), WorkspaceID: workspaceID, Name: "Request 2", Url: "https://example.com/api", Method: "POST", CreatedAt: now, UpdatedAt: now},
		},
		HTTPSearchParams: []mhttp.HTTPSearchParam{
			{ID: idwrap.NewNow(), HttpID: idwrap.NewNow(), Key: "param1", Value: "value1", Enabled: true, CreatedAt: now, UpdatedAt: now},
		},
		HTTPHeaders: []mhttp.HTTPHeader{
			{ID: idwrap.NewNow(), HttpID: idwrap.NewNow(), Key: "Content-Type", Value: "application/json", Enabled: true, CreatedAt: now, UpdatedAt: now},
			{ID: idwrap.NewNow(), HttpID: idwrap.NewNow(), Key: "Authorization", Value: "Bearer token", Enabled: true, CreatedAt: now, UpdatedAt: now},
		},
		Flows: []mflow.Flow{
			{ID: idwrap.NewNow(), WorkspaceID: workspaceID, Name: "Flow 1"},
		},
		FlowNodes: []mflow.Node{
			{ID: idwrap.NewNow(), FlowID: idwrap.NewNow(), Name: "Node 1", NodeKind: mflow.NODE_KIND_REQUEST},
		},
		Environments: []menv.Env{
			{ID: idwrap.NewNow(), WorkspaceID: workspaceID, Name: "Production"},
		},
	}

	counts := bundle.CountEntities()

	if counts["http_requests"] != 2 {
		t.Errorf("Expected 2 http_requests, got %d", counts["http_requests"])
	}
	if counts["http_search_params"] != 1 {
		t.Errorf("Expected 1 http_search_params, got %d", counts["http_search_params"])
	}
	if counts["http_headers"] != 2 {
		t.Errorf("Expected 2 http_headers, got %d", counts["http_headers"])
	}
	if counts["flows"] != 1 {
		t.Errorf("Expected 1 flows, got %d", counts["flows"])
	}
	if counts["flow_nodes"] != 1 {
		t.Errorf("Expected 1 flow_nodes, got %d", counts["flow_nodes"])
	}
	if counts["environments"] != 1 {
		t.Errorf("Expected 1 environments, got %d", counts["environments"])
	}
}

// TestGetHTTPByID tests finding HTTP requests by ID
func TestGetHTTPByID(t *testing.T) {
	workspaceID := idwrap.NewNow()
	httpID1 := idwrap.NewNow()
	httpID2 := idwrap.NewNow()
	now := time.Now().Unix()

	bundle := &WorkspaceBundle{
		HTTPRequests: []mhttp.HTTP{
			{ID: httpID1, WorkspaceID: workspaceID, Name: "Request 1", Url: "https://example.com", Method: "GET", CreatedAt: now, UpdatedAt: now},
			{ID: httpID2, WorkspaceID: workspaceID, Name: "Request 2", Url: "https://api.example.com", Method: "POST", CreatedAt: now, UpdatedAt: now},
		},
	}

	// Test finding existing HTTP request
	http := bundle.GetHTTPByID(httpID1)
	require.NotNil(t, http, "Expected to find HTTP request, got nil")
	if http.ID.Compare(httpID1) != 0 {
		t.Errorf("Expected HTTP ID %s, got %s", httpID1, http.ID)
	}
	if http.Name != "Request 1" {
		t.Errorf("Expected HTTP name 'Request 1', got '%s'", http.Name)
	}

	// Test finding second HTTP request
	http2 := bundle.GetHTTPByID(httpID2)
	if http2 == nil {
		t.Fatal("Expected to find HTTP request 2, got nil")
	}
	if http2.Name != "Request 2" {
		t.Errorf("Expected HTTP name 'Request 2', got '%s'", http2.Name)
	}

	// Test non-existent ID
	nonExistentID := idwrap.NewNow()
	notFound := bundle.GetHTTPByID(nonExistentID)
	if notFound != nil {
		t.Errorf("Expected nil for non-existent ID, got %v", notFound)
	}
}

// TestGetFlowByID tests finding flows by ID
func TestGetFlowByID(t *testing.T) {
	workspaceID := idwrap.NewNow()
	flowID1 := idwrap.NewNow()
	flowID2 := idwrap.NewNow()

	bundle := &WorkspaceBundle{
		Flows: []mflow.Flow{
			{ID: flowID1, WorkspaceID: workspaceID, Name: "User Registration Flow"},
			{ID: flowID2, WorkspaceID: workspaceID, Name: "Order Processing Flow"},
		},
	}

	// Test finding existing flow
	flow := bundle.GetFlowByID(flowID1)
	require.NotNil(t, flow, "Expected to find flow, got nil")
	if flow.ID.Compare(flowID1) != 0 {
		t.Errorf("Expected flow ID %s, got %s", flowID1, flow.ID)
	}
	if flow.Name != "User Registration Flow" {
		t.Errorf("Expected flow name 'User Registration Flow', got '%s'", flow.Name)
	}

	// Test non-existent ID
	nonExistentID := idwrap.NewNow()
	notFound := bundle.GetFlowByID(nonExistentID)
	if notFound != nil {
		t.Errorf("Expected nil for non-existent ID, got %v", notFound)
	}
}

// TestGetFlowByName tests finding flows by name
func TestGetFlowByName(t *testing.T) {
	workspaceID := idwrap.NewNow()

	bundle := &WorkspaceBundle{
		Flows: []mflow.Flow{
			{ID: idwrap.NewNow(), WorkspaceID: workspaceID, Name: "User Registration Flow"},
			{ID: idwrap.NewNow(), WorkspaceID: workspaceID, Name: "Order Processing Flow"},
		},
	}

	// Test finding by name
	flow := bundle.GetFlowByName("User Registration Flow")
	require.NotNil(t, flow, "Expected to find flow by name, got nil")
	if flow.Name != "User Registration Flow" {
		t.Errorf("Expected flow name 'User Registration Flow', got '%s'", flow.Name)
	}

	// Test non-existent name
	notFound := bundle.GetFlowByName("Non-existent Flow")
	if notFound != nil {
		t.Errorf("Expected nil for non-existent name, got %v", notFound)
	}
}

// TestGetNodeByID tests finding nodes by ID
func TestGetNodeByID(t *testing.T) {
	flowID := idwrap.NewNow()
	nodeID1 := idwrap.NewNow()
	nodeID2 := idwrap.NewNow()

	bundle := &WorkspaceBundle{
		FlowNodes: []mflow.Node{
			{ID: nodeID1, FlowID: flowID, Name: "Start Node", NodeKind: mflow.NODE_KIND_MANUAL_START},
			{ID: nodeID2, FlowID: flowID, Name: "Request Node", NodeKind: mflow.NODE_KIND_REQUEST},
		},
	}

	// Test finding existing node
	node := bundle.GetNodeByID(nodeID1)
	require.NotNil(t, node, "Expected to find node, got nil")
	if node.ID.Compare(nodeID1) != 0 {
		t.Errorf("Expected node ID %s, got %s", nodeID1, node.ID)
	}
	if node.Name != "Start Node" {
		t.Errorf("Expected node name 'Start Node', got '%s'", node.Name)
	}

	// Test non-existent ID
	nonExistentID := idwrap.NewNow()
	notFound := bundle.GetNodeByID(nonExistentID)
	if notFound != nil {
		t.Errorf("Expected nil for non-existent ID, got %v", notFound)
	}
}

// TestGetFileByID tests finding files by ID
func TestGetFileByID(t *testing.T) {
	workspaceID := idwrap.NewNow()
	fileID1 := idwrap.NewNow()
	fileID2 := idwrap.NewNow()
	contentID1 := idwrap.NewNow()

	bundle := &WorkspaceBundle{
		Files: []mfile.File{
			{ID: fileID1, WorkspaceID: workspaceID, ContentID: &contentID1, ContentType: mfile.ContentTypeHTTP, Name: "API Request"},
			{ID: fileID2, WorkspaceID: workspaceID, ContentID: nil, ContentType: mfile.ContentTypeFolder, Name: "Folder"},
		},
	}

	// Test finding existing file
	file := bundle.GetFileByID(fileID1)
	require.NotNil(t, file, "Expected to find file, got nil")
	if file.ID.Compare(fileID1) != 0 {
		t.Errorf("Expected file ID %s, got %s", fileID1, file.ID)
	}
	if file.Name != "API Request" {
		t.Errorf("Expected file name 'API Request', got '%s'", file.Name)
	}

	// Test non-existent ID
	nonExistentID := idwrap.NewNow()
	notFound := bundle.GetFileByID(nonExistentID)
	if notFound != nil {
		t.Errorf("Expected nil for non-existent ID, got %v", notFound)
	}
}

// TestGetFileByContentID tests finding files by ContentID
func TestGetFileByContentID(t *testing.T) {
	workspaceID := idwrap.NewNow()
	fileID1 := idwrap.NewNow()
	fileID2 := idwrap.NewNow()
	contentID1 := idwrap.NewNow()
	contentID2 := idwrap.NewNow()

	bundle := &WorkspaceBundle{
		Files: []mfile.File{
			{ID: fileID1, WorkspaceID: workspaceID, ContentID: &contentID1, ContentType: mfile.ContentTypeHTTP, Name: "API Request"},
			{ID: fileID2, WorkspaceID: workspaceID, ContentID: &contentID2, ContentType: mfile.ContentTypeFlow, Name: "Flow File"},
		},
	}

	// Test finding by ContentID
	file := bundle.GetFileByContentID(contentID1)
	require.NotNil(t, file, "Expected to find file by ContentID, got nil")
	if file.ID.Compare(fileID1) != 0 {
		t.Errorf("Expected file ID %s, got %s", fileID1, file.ID)
	}

	// Test non-existent ContentID
	nonExistentContentID := idwrap.NewNow()
	notFound := bundle.GetFileByContentID(nonExistentContentID)
	if notFound != nil {
		t.Errorf("Expected nil for non-existent ContentID, got %v", notFound)
	}

	// Test file without ContentID
	fileID3 := idwrap.NewNow()
	bundle.Files = append(bundle.Files, mfile.File{
		ID:          fileID3,
		WorkspaceID: workspaceID,
		ContentID:   nil,
		ContentType: mfile.ContentTypeFolder,
		Name:        "Empty Folder",
	})

	// This should not find the file with nil ContentID
	notFound2 := bundle.GetFileByContentID(idwrap.NewNow())
	if notFound2 != nil {
		t.Errorf("Expected nil for ContentID when file has nil ContentID, got %v", notFound2)
	}
}

// TestGetEnvironmentByID tests finding environments by ID
func TestGetEnvironmentByID(t *testing.T) {
	workspaceID := idwrap.NewNow()
	envID1 := idwrap.NewNow()
	envID2 := idwrap.NewNow()

	bundle := &WorkspaceBundle{
		Environments: []menv.Env{
			{ID: envID1, WorkspaceID: workspaceID, Name: "Development"},
			{ID: envID2, WorkspaceID: workspaceID, Name: "Production"},
		},
	}

	// Test finding existing environment
	env := bundle.GetEnvironmentByID(envID1)
	require.NotNil(t, env, "Expected to find environment, got nil")
	if env.ID.Compare(envID1) != 0 {
		t.Errorf("Expected environment ID %s, got %s", envID1, env.ID)
	}
	if env.Name != "Development" {
		t.Errorf("Expected environment name 'Development', got '%s'", env.Name)
	}

	// Test non-existent ID
	nonExistentID := idwrap.NewNow()
	notFound := bundle.GetEnvironmentByID(nonExistentID)
	if notFound != nil {
		t.Errorf("Expected nil for non-existent ID, got %v", notFound)
	}
}

// TestGetEnvironmentByName tests finding environments by name
func TestGetEnvironmentByName(t *testing.T) {
	workspaceID := idwrap.NewNow()

	bundle := &WorkspaceBundle{
		Environments: []menv.Env{
			{ID: idwrap.NewNow(), WorkspaceID: workspaceID, Name: "Development"},
			{ID: idwrap.NewNow(), WorkspaceID: workspaceID, Name: "Production"},
		},
	}

	// Test finding by name
	env := bundle.GetEnvironmentByName("Production")
	require.NotNil(t, env, "Expected to find environment by name, got nil")
	if env.Name != "Production" {
		t.Errorf("Expected environment name 'Production', got '%s'", env.Name)
	}

	// Test non-existent name
	notFound := bundle.GetEnvironmentByName("Staging")
	if notFound != nil {
		t.Errorf("Expected nil for non-existent name, got %v", notFound)
	}
}

// TestWorkspaceBundle_CompleteStructure tests a fully populated bundle structure
func TestWorkspaceBundle_CompleteStructure(t *testing.T) {
	workspaceID := idwrap.NewNow()
	httpID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	nodeID := idwrap.NewNow()
	envID := idwrap.NewNow()
	now := time.Now().Unix()

	bundle := &WorkspaceBundle{
		Workspace: mworkspace.Workspace{
			ID:   workspaceID,
			Name: "Complete Test Workspace",
		},
		HTTPRequests: []mhttp.HTTP{
			{ID: httpID, WorkspaceID: workspaceID, Name: "Test Request", Url: "https://api.example.com", Method: "GET", CreatedAt: now, UpdatedAt: now},
		},
		HTTPSearchParams: []mhttp.HTTPSearchParam{
			{ID: idwrap.NewNow(), HttpID: httpID, Key: "query", Value: "test", Enabled: true, CreatedAt: now, UpdatedAt: now},
		},
		HTTPHeaders: []mhttp.HTTPHeader{
			{ID: idwrap.NewNow(), HttpID: httpID, Key: "Authorization", Value: "Bearer token", Enabled: true, CreatedAt: now, UpdatedAt: now},
		},
		HTTPBodyForms: []mhttp.HTTPBodyForm{
			{ID: idwrap.NewNow(), HttpID: httpID, Key: "field1", Value: "value1", Enabled: true, CreatedAt: now, UpdatedAt: now},
		},
		HTTPBodyUrlencoded: []mhttp.HTTPBodyUrlencoded{
			{ID: idwrap.NewNow(), HttpID: httpID, Key: "param", Value: "value", Enabled: true, CreatedAt: now, UpdatedAt: now},
		},
		HTTPBodyRaw: []mhttp.HTTPBodyRaw{
			{ID: idwrap.NewNow(), HttpID: httpID, RawData: []byte(`{"test": true}`), CreatedAt: now, UpdatedAt: now},
		},
		HTTPAsserts: []mhttp.HTTPAssert{
			{ID: idwrap.NewNow(), HttpID: httpID, Value: "response.status == 200", Enabled: true, CreatedAt: now, UpdatedAt: now},
		},
		Files: []mfile.File{
			{ID: idwrap.NewNow(), WorkspaceID: workspaceID, ContentID: &httpID, ContentType: mfile.ContentTypeHTTP, Name: "API Request"},
		},
		Flows: []mflow.Flow{
			{ID: flowID, WorkspaceID: workspaceID, Name: "Test Flow"},
		},
		FlowVariables: []mflow.FlowVariable{
			{ID: idwrap.NewNow(), FlowID: flowID, Name: "timeout", Value: "60"},
		},
		FlowNodes: []mflow.Node{
			{ID: nodeID, FlowID: flowID, Name: "Request Node", NodeKind: mflow.NODE_KIND_REQUEST},
		},
		FlowEdges: []mflow.Edge{
			{ID: idwrap.NewNow(), FlowID: flowID, SourceID: idwrap.NewNow(), TargetID: nodeID},
		},
		FlowRequestNodes: []mflow.NodeRequest{
			{FlowNodeID: nodeID, HttpID: &httpID},
		},
		FlowConditionNodes: []mflow.NodeIf{
			{FlowNodeID: idwrap.NewNow(), Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 200"}}},
		},
		FlowForNodes: []mflow.NodeFor{
			{FlowNodeID: idwrap.NewNow(), IterCount: 5},
		},
		FlowForEachNodes: []mflow.NodeForEach{
			{FlowNodeID: idwrap.NewNow(), IterExpression: "users"},
		},
		FlowJSNodes: []mflow.NodeJS{
			{FlowNodeID: idwrap.NewNow(), Code: []byte("console.log('test')")},
		},
		Environments: []menv.Env{
			{ID: envID, WorkspaceID: workspaceID, Name: "Production"},
		},
		EnvironmentVars: []menv.Variable{
			{ID: idwrap.NewNow(), EnvID: envID, VarKey: "API_URL", Value: "https://api.example.com"},
		},
	}

	// Verify count of all entities
	counts := bundle.CountEntities()

	expectedCounts := map[string]int{
		"http_requests":        1,
		"http_search_params":   1,
		"http_headers":         1,
		"http_body_forms":      1,
		"http_body_urlencoded": 1,
		"http_body_raw":        1,
		"http_asserts":         1,
		"files":                1,
		"flows":                1,
		"flow_variables":       1,
		"flow_nodes":           1,
		"flow_edges":           1,
		"flow_request_nodes":   1,
		"flow_condition_nodes": 1,
		"flow_for_nodes":       1,
		"flow_foreach_nodes":   1,
		"flow_js_nodes":        1,
		"environments":         1,
		"environment_vars":     1,
	}

	for key, expected := range expectedCounts {
		if counts[key] != expected {
			t.Errorf("Expected %s count to be %d, got %d", key, expected, counts[key])
		}
	}

	// Verify we can retrieve entities
	if http := bundle.GetHTTPByID(httpID); http == nil {
		t.Error("Failed to retrieve HTTP request")
	}
	if flow := bundle.GetFlowByID(flowID); flow == nil {
		t.Error("Failed to retrieve flow")
	}
	if node := bundle.GetNodeByID(nodeID); node == nil {
		t.Error("Failed to retrieve node")
	}
	if env := bundle.GetEnvironmentByID(envID); env == nil {
		t.Error("Failed to retrieve environment")
	}
}

// TestImportOptions_Validate tests ImportOptions validation
func TestImportOptions_Validate(t *testing.T) {
	workspaceID := idwrap.NewNow()

	t.Run("valid options", func(t *testing.T) {
		opts := ImportOptions{
			WorkspaceID: workspaceID,
			MergeMode:   "create_new",
		}
		err := opts.Validate()
		if err != nil {
			t.Errorf("Expected valid options to pass validation, got error: %v", err)
		}
	})

	t.Run("missing workspace ID", func(t *testing.T) {
		opts := ImportOptions{
			WorkspaceID: idwrap.IDWrap{},
		}
		err := opts.Validate()
		if err != ErrWorkspaceIDRequired {
			t.Errorf("Expected ErrWorkspaceIDRequired, got: %v", err)
		}
	})

	t.Run("invalid merge mode", func(t *testing.T) {
		opts := ImportOptions{
			WorkspaceID: workspaceID,
			MergeMode:   "invalid_mode",
		}
		err := opts.Validate()
		if err != ErrInvalidMergeMode {
			t.Errorf("Expected ErrInvalidMergeMode, got: %v", err)
		}
	})

	t.Run("valid merge modes", func(t *testing.T) {
		validModes := []string{"skip", "replace", "create_new"}
		for _, mode := range validModes {
			opts := ImportOptions{
				WorkspaceID: workspaceID,
				MergeMode:   mode,
			}
			err := opts.Validate()
			if err != nil {
				t.Errorf("Expected merge mode '%s' to be valid, got error: %v", mode, err)
			}
		}
	})

	t.Run("empty merge mode is valid", func(t *testing.T) {
		opts := ImportOptions{
			WorkspaceID: workspaceID,
			MergeMode:   "",
		}
		err := opts.Validate()
		if err != nil {
			t.Errorf("Expected empty merge mode to be valid, got error: %v", err)
		}
	})
}

// TestExportOptions_Validate tests ExportOptions validation
func TestExportOptions_Validate(t *testing.T) {
	workspaceID := idwrap.NewNow()

	t.Run("valid options", func(t *testing.T) {
		opts := ExportOptions{
			WorkspaceID:  workspaceID,
			ExportFormat: "json",
		}
		err := opts.Validate()
		if err != nil {
			t.Errorf("Expected valid options to pass validation, got error: %v", err)
		}
	})

	t.Run("missing workspace ID", func(t *testing.T) {
		opts := ExportOptions{
			WorkspaceID: idwrap.IDWrap{},
		}
		err := opts.Validate()
		if err != ErrWorkspaceIDRequired {
			t.Errorf("Expected ErrWorkspaceIDRequired, got: %v", err)
		}
	})

	t.Run("invalid export format", func(t *testing.T) {
		opts := ExportOptions{
			WorkspaceID:  workspaceID,
			ExportFormat: "invalid_format",
		}
		err := opts.Validate()
		if err != ErrInvalidExportFormat {
			t.Errorf("Expected ErrInvalidExportFormat, got: %v", err)
		}
	})

	t.Run("valid export formats", func(t *testing.T) {
		validFormats := []string{"json", "yaml", "zip"}
		for _, format := range validFormats {
			opts := ExportOptions{
				WorkspaceID:  workspaceID,
				ExportFormat: format,
			}
			err := opts.Validate()
			if err != nil {
				t.Errorf("Expected format '%s' to be valid, got error: %v", format, err)
			}
		}
	})

	t.Run("empty export format is valid", func(t *testing.T) {
		opts := ExportOptions{
			WorkspaceID:  workspaceID,
			ExportFormat: "",
		}
		err := opts.Validate()
		if err != nil {
			t.Errorf("Expected empty export format to be valid, got error: %v", err)
		}
	})
}

// TestGetDefaultImportOptions tests default import options
func TestGetDefaultImportOptions(t *testing.T) {
	workspaceID := idwrap.NewNow()
	opts := GetDefaultImportOptions(workspaceID)

	if opts.WorkspaceID.Compare(workspaceID) != 0 {
		t.Errorf("Expected WorkspaceID to match, got different ID")
	}
	if opts.ParentFolderID != nil {
		t.Errorf("Expected ParentFolderID to be nil, got %v", opts.ParentFolderID)
	}
	if !opts.CreateFiles {
		t.Error("Expected CreateFiles to be true")
	}
	if opts.MergeMode != "create_new" {
		t.Errorf("Expected MergeMode to be 'create_new', got '%s'", opts.MergeMode)
	}
	if opts.PreserveIDs {
		t.Error("Expected PreserveIDs to be false")
	}
	if !opts.ImportHTTP {
		t.Error("Expected ImportHTTP to be true")
	}
	if !opts.ImportFlows {
		t.Error("Expected ImportFlows to be true")
	}
	if !opts.ImportEnvironments {
		t.Error("Expected ImportEnvironments to be true")
	}
	if opts.StartOrder != 0 {
		t.Errorf("Expected StartOrder to be 0, got %f", opts.StartOrder)
	}

	// Verify validation passes
	err := opts.Validate()
	if err != nil {
		t.Errorf("Expected default options to pass validation, got error: %v", err)
	}
}

// TestGetDefaultExportOptions tests default export options
func TestGetDefaultExportOptions(t *testing.T) {
	workspaceID := idwrap.NewNow()
	opts := GetDefaultExportOptions(workspaceID)

	if opts.WorkspaceID.Compare(workspaceID) != 0 {
		t.Errorf("Expected WorkspaceID to match, got different ID")
	}
	if !opts.IncludeHTTP {
		t.Error("Expected IncludeHTTP to be true")
	}
	if !opts.IncludeFlows {
		t.Error("Expected IncludeFlows to be true")
	}
	if !opts.IncludeEnvironments {
		t.Error("Expected IncludeEnvironments to be true")
	}
	if !opts.IncludeFiles {
		t.Error("Expected IncludeFiles to be true")
	}
	if opts.ExportFormat != "json" {
		t.Errorf("Expected ExportFormat to be 'json', got '%s'", opts.ExportFormat)
	}
	if opts.FilterByFolderID != nil {
		t.Errorf("Expected FilterByFolderID to be nil, got %v", opts.FilterByFolderID)
	}
	if opts.FilterByFlowIDs != nil {
		t.Errorf("Expected FilterByFlowIDs to be nil, got %v", opts.FilterByFlowIDs)
	}
	if opts.FilterByHTTPIDs != nil {
		t.Errorf("Expected FilterByHTTPIDs to be nil, got %v", opts.FilterByHTTPIDs)
	}

	// Verify validation passes
	err := opts.Validate()
	if err != nil {
		t.Errorf("Expected default options to pass validation, got error: %v", err)
	}
}

// TestExportOptions_Filters tests export options with filters
func TestExportOptions_Filters(t *testing.T) {
	workspaceID := idwrap.NewNow()
	folderID := idwrap.NewNow()
	flowID1 := idwrap.NewNow()
	flowID2 := idwrap.NewNow()
	httpID1 := idwrap.NewNow()

	opts := ExportOptions{
		WorkspaceID:      workspaceID,
		IncludeHTTP:      true,
		IncludeFlows:     true,
		ExportFormat:     "json",
		FilterByFolderID: &folderID,
		FilterByFlowIDs:  []idwrap.IDWrap{flowID1, flowID2},
		FilterByHTTPIDs:  []idwrap.IDWrap{httpID1},
	}

	err := opts.Validate()
	if err != nil {
		t.Errorf("Expected options with filters to be valid, got error: %v", err)
	}

	// Verify filters are set
	if opts.FilterByFolderID == nil || opts.FilterByFolderID.Compare(folderID) != 0 {
		t.Error("Expected FilterByFolderID to be set correctly")
	}
	if len(opts.FilterByFlowIDs) != 2 {
		t.Errorf("Expected 2 flow IDs in filter, got %d", len(opts.FilterByFlowIDs))
	}
	if len(opts.FilterByHTTPIDs) != 1 {
		t.Errorf("Expected 1 HTTP ID in filter, got %d", len(opts.FilterByHTTPIDs))
	}
}

// TestImportOptions_WithParentFolder tests import options with parent folder
func TestImportOptions_WithParentFolder(t *testing.T) {
	workspaceID := idwrap.NewNow()
	parentFolderID := idwrap.NewNow()

	opts := ImportOptions{
		WorkspaceID:    workspaceID,
		ParentFolderID: &parentFolderID,
		CreateFiles:    true,
		MergeMode:      "create_new",
		ImportHTTP:     true,
	}

	err := opts.Validate()
	if err != nil {
		t.Errorf("Expected options with parent folder to be valid, got error: %v", err)
	}

	if opts.ParentFolderID == nil || opts.ParentFolderID.Compare(parentFolderID) != 0 {
		t.Error("Expected ParentFolderID to be set correctly")
	}
}

// TestImportOptions_SelectiveImport tests import options with selective flags
func TestImportOptions_SelectiveImport(t *testing.T) {
	workspaceID := idwrap.NewNow()

	t.Run("import only HTTP", func(t *testing.T) {
		opts := ImportOptions{
			WorkspaceID:        workspaceID,
			ImportHTTP:         true,
			ImportFlows:        false,
			ImportEnvironments: false,
			MergeMode:          "create_new",
		}
		err := opts.Validate()
		if err != nil {
			t.Errorf("Expected options to be valid, got error: %v", err)
		}
		if !opts.ImportHTTP || opts.ImportFlows || opts.ImportEnvironments {
			t.Error("Expected only ImportHTTP to be true")
		}
	})

	t.Run("import only flows", func(t *testing.T) {
		opts := ImportOptions{
			WorkspaceID:        workspaceID,
			ImportHTTP:         false,
			ImportFlows:        true,
			ImportEnvironments: false,
			MergeMode:          "create_new",
		}
		err := opts.Validate()
		if err != nil {
			t.Errorf("Expected options to be valid, got error: %v", err)
		}
		if opts.ImportHTTP || !opts.ImportFlows || opts.ImportEnvironments {
			t.Error("Expected only ImportFlows to be true")
		}
	})

	t.Run("import only environments", func(t *testing.T) {
		opts := ImportOptions{
			WorkspaceID:        workspaceID,
			ImportHTTP:         false,
			ImportFlows:        false,
			ImportEnvironments: true,
			MergeMode:          "create_new",
		}
		err := opts.Validate()
		if err != nil {
			t.Errorf("Expected options to be valid, got error: %v", err)
		}
		if opts.ImportHTTP || opts.ImportFlows || !opts.ImportEnvironments {
			t.Error("Expected only ImportEnvironments to be true")
		}
	})
}

// TestImportOptions_PreserveIDs tests preserve IDs functionality
func TestImportOptions_PreserveIDs(t *testing.T) {
	workspaceID := idwrap.NewNow()

	t.Run("preserve IDs enabled", func(t *testing.T) {
		opts := ImportOptions{
			WorkspaceID: workspaceID,
			PreserveIDs: true,
			MergeMode:   "create_new",
		}
		err := opts.Validate()
		if err != nil {
			t.Errorf("Expected options to be valid, got error: %v", err)
		}
		if !opts.PreserveIDs {
			t.Error("Expected PreserveIDs to be true")
		}
	})

	t.Run("preserve IDs disabled", func(t *testing.T) {
		opts := ImportOptions{
			WorkspaceID: workspaceID,
			PreserveIDs: false,
			MergeMode:   "create_new",
		}
		err := opts.Validate()
		if err != nil {
			t.Errorf("Expected options to be valid, got error: %v", err)
		}
		if opts.PreserveIDs {
			t.Error("Expected PreserveIDs to be false")
		}
	})
}
