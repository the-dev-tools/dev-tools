package rimportv2

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mfile"
	apiv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/import/v1"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

// TestRealYAML_EcommerceFile tests import with a real-world YAML file
// that contains 15 requests, flows, for loops, and request dependencies.
func TestRealYAML_EcommerceFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real YAML test in short mode")
	}

	fixture := newIntegrationTestFixture(t)

	// Load the real YAML file
	yamlPath := filepath.Join("testdata", "ecommerce.yaml")
	yamlData, err := os.ReadFile(yamlPath)
	require.NoError(t, err, "Failed to read ecommerce.yaml - make sure testdata/ecommerce.yaml exists")

	// Import the YAML
	req := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "E-commerce YAML Import",
		Data:        yamlData,
	})

	resp, err := fixture.rpc.Import(fixture.ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify import succeeded (no MissingData)
	require.Equal(t, apiv1.ImportMissingDataKind_IMPORT_MISSING_DATA_KIND_UNSPECIFIED, resp.Msg.MissingData,
		"Import should succeed without MissingData")

	// Get all HTTP requests
	httpReqs, err := fixture.services.HttpService.GetByWorkspaceID(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)

	// The YAML has 15 requests
	t.Logf("HTTP Requests: %d", len(httpReqs))
	require.Equal(t, 15, len(httpReqs), "Should have 15 HTTP requests")

	// Get all files
	files, err := fixture.services.FileService.ListFilesByWorkspace(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)

	// Count file types
	var httpFiles, folderFiles, flowFiles int
	for _, f := range files {
		switch f.ContentType {
		case mfile.ContentTypeHTTP:
			httpFiles++
		case mfile.ContentTypeFolder:
			folderFiles++
		case mfile.ContentTypeFlow:
			flowFiles++
		}
	}

	t.Logf("Files: %d http, %d folder, %d flow", httpFiles, folderFiles, flowFiles)

	// Key assertions for YAML import
	require.Equal(t, 15, httpFiles, "Should have 15 HTTP files (one per request)")
	require.Equal(t, 1, flowFiles, "Should have 1 flow file")
	// YAML creates folders for organizing requests
	require.GreaterOrEqual(t, folderFiles, 1, "Should have at least 1 folder file")

	// Run integrity validation
	report, err := ValidateImportIntegrity(
		fixture.ctx,
		fixture.workspaceID,
		&fixture.services.FileService,
		&fixture.services.HttpService,
		nil, // nodeService not needed for file validation
		nil, // nodeRequestService not needed for file validation
	)
	require.NoError(t, err)
	require.False(t, report.HasErrors(), "Integrity validation failed: %v", report)
}

// TestRealYAML_FlowCreated tests that YAML import creates a flow
func TestRealYAML_FlowCreated(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real YAML test in short mode")
	}

	fixture := newIntegrationTestFixture(t)

	yamlPath := filepath.Join("testdata", "ecommerce.yaml")
	yamlData, err := os.ReadFile(yamlPath)
	require.NoError(t, err)

	req := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Flow Test",
		Data:        yamlData,
	})

	_, err = fixture.rpc.Import(fixture.ctx, req)
	require.NoError(t, err)

	// Get all flows in workspace
	flows, err := fixture.services.FlowService.GetFlowsByWorkspaceID(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)
	require.Equal(t, 1, len(flows), "Should have 1 flow")

	flow := flows[0]
	t.Logf("Flow: %s (ID: %s)", flow.Name, flow.ID)
	require.Equal(t, "Imported HAR Flow", flow.Name, "Flow should have correct name from YAML")
}

// TestRealYAML_MultipleRequests tests that YAML import handles many requests correctly
func TestRealYAML_MultipleRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real YAML test in short mode")
	}

	fixture := newIntegrationTestFixture(t)

	yamlPath := filepath.Join("testdata", "ecommerce.yaml")
	yamlData, err := os.ReadFile(yamlPath)
	require.NoError(t, err)

	req := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Multiple Requests Test",
		Data:        yamlData,
	})

	_, err = fixture.rpc.Import(fixture.ctx, req)
	require.NoError(t, err)

	// Get all HTTP requests
	httpReqs, err := fixture.services.HttpService.GetByWorkspaceID(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)

	// Log all request names and methods
	methodCounts := make(map[string]int)
	for _, httpReq := range httpReqs {
		t.Logf("HTTP Request: %s %s", httpReq.Method, httpReq.Url)
		methodCounts[httpReq.Method]++
	}

	t.Logf("Method counts: %v", methodCounts)

	// The YAML has various HTTP methods
	require.Equal(t, 15, len(httpReqs), "Should have 15 HTTP requests")
	require.Greater(t, methodCounts["GET"], 0, "Should have GET requests")
	require.Greater(t, methodCounts["POST"], 0, "Should have POST requests")
	require.Greater(t, methodCounts["DELETE"], 0, "Should have DELETE requests")
}
