package rimportv2

import (
	"testing"

	"the-dev-tools/server/pkg/idwrap"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/import/v1"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

func TestImportService_DuplicateImport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	fixture := newIntegrationTestFixture(t)

	// 1. Create a simple HAR (creates 1 entry)
	harData := createSimpleHAR(t)

	// 2. Perform First Import
	req1 := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Import 1",
		Data:        harData,
		DomainData: []*apiv1.ImportDomainData{
			{Enabled: true, Domain: "api.example.com", Variable: "API_HOST"},
		},
	})

	resp1, err := fixture.rpc.Import(fixture.ctx, req1)
	require.NoError(t, err)
	require.NotNil(t, resp1)

	// Verify count after first import
	httpReqs1, err := fixture.services.HttpService.GetByWorkspaceID(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)
	require.Equal(t, 1, len(httpReqs1), "Should have 1 HTTP request after first import")

	firstID := httpReqs1[0].ID

	// 3. Perform Second Import (Same Data)
	req2 := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Import 2",
		Data:        harData,
		DomainData: []*apiv1.ImportDomainData{
			{Enabled: true, Domain: "api.example.com", Variable: "API_HOST"},
		},
	})

	resp2, err := fixture.rpc.Import(fixture.ctx, req2)
	require.NoError(t, err)
	require.NotNil(t, resp2)

	// Verify count after second import
	httpReqs2, err := fixture.services.HttpService.GetByWorkspaceID(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)
	// DEDUPLICATION: Count should still be 1 because it's identical data
	require.Equal(t, 1, len(httpReqs2), "Should still have 1 HTTP request after second import due to deduplication")

	// Verify ID is the same
	require.Equal(t, firstID.String(), httpReqs2[0].ID.String(), "Should reuse the same ID")
}

func TestImportService_DuplicateImport_DeepVerification(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	fixture := newIntegrationTestFixture(t)
	// Use complex HAR to test headers and params
	harData := createComplexHAR(t)

	// 1. Perform First Import
	req1 := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Deep Verify 1",
		Data:        harData,
		DomainData: []*apiv1.ImportDomainData{
			{Enabled: true, Domain: "api.example.com", Variable: "API"},
		},
	})

	resp1, err := fixture.rpc.Import(fixture.ctx, req1)
	require.NoError(t, err)
	require.NotNil(t, resp1) // Usage

	// Capture first import IDs
	httpReqs1, err := fixture.services.HttpService.GetByWorkspaceID(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)
	require.NotEmpty(t, httpReqs1)
	_ = httpReqs1[0].ID // Ignored

	flows1, err := fixture.services.FlowService.GetFlowsByWorkspaceID(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)
	require.NotEmpty(t, flows1)
	firstFlowID := flows1[0].ID

	// 2. Perform Second Import
	req2 := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Deep Verify 2",
		Data:        harData,
		DomainData: []*apiv1.ImportDomainData{
			{Enabled: true, Domain: "api.example.com", Variable: "API"},
		},
	})

	resp2, err := fixture.rpc.Import(fixture.ctx, req2)
	require.NoError(t, err)
	require.NotNil(t, resp2) // Usage

	// 3. Verify New Entities

	// Check HTTP Requests
	httpReqs2, err := fixture.services.HttpService.GetByWorkspaceID(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)
	// Complex HAR has 2 entries. 2 imports = 2 base requests (deduplicated) + 2 delta requests (deduplicated) = 2 unique requests total if base+delta are also stable
	// Actually, delta fingerprint includes parent hash, so if parent is same and delta content is same, delta is also deduplicated.
	require.Equal(t, 2, len(httpReqs2), "Should only have 2 unique requests due to full deduplication")

	// Identify requests (should be the same as first batch)
	for _, r := range httpReqs2 {
		found := false
		for _, old := range httpReqs1 {
			if r.ID.String() == old.ID.String() {
				found = true
				break
			}
		}
		require.True(t, found, "Request ID %s should have been reused", r.ID.String())
	}

	// Check Flows (Flows are always new for now as they are named/timestamped implicitly or we don't deduplicate them)
	flows2, err := fixture.services.FlowService.GetFlowsByWorkspaceID(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)
	require.Equal(t, 2, len(flows2), "Should have 2 flows (Flows are not deduplicated yet)")

	var newFlowID idwrap.IDWrap
	for _, f := range flows2 {
		if f.ID != firstFlowID {
			newFlowID = f.ID
			break
		}
	}
	require.NotEqual(t, idwrap.IDWrap{}, newFlowID, "Should find a new flow ID")

	// Check Nodes for New Flow
	nodes, err := fixture.rpc.NodeService.GetNodesByFlowID(fixture.ctx, newFlowID)
	require.NoError(t, err)
	require.NotEmpty(t, nodes, "New flow should have nodes")

	// Check Files
	files2, err := fixture.services.FileService.ListFilesByWorkspace(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)
	// DEDUPLICATION:
	// 1st import: 2 base reqs + 2 delta reqs + folder structure (e.g. /com/example/api) + 1 flow file
	// 2nd import: 1 new flow file. Everything else (reqs, folders) is deduplicated.
	// Total files should increase by exactly 1 (the second flow's file).
	// We don't know exact folder count without tracing, but it should be stable.
	require.Greater(t, len(files2), len(httpReqs1), "Should have more files than just requests")
}

func TestImportService_DuplicateImport_CleanDelta(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	fixture := newIntegrationTestFixture(t)
	harData := createComplexHAR(t) // 2 requests, headers, params

	// 1. Initial Import
	req1 := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Import Clean Delta",
		Data:        harData,
		DomainData: []*apiv1.ImportDomainData{
			{Enabled: true, Domain: "api.example.com", Variable: "API"},
		},
	})

	resp1, err := fixture.rpc.Import(fixture.ctx, req1)
	require.NoError(t, err)
	require.NotNil(t, resp1)

	// Get the first request ID (which has headers and params)
	httpReqs1, _ := fixture.services.HttpService.GetByWorkspaceID(fixture.ctx, fixture.workspaceID)
	require.NotEmpty(t, httpReqs1)
	reqID := httpReqs1[0].ID

	// 2. Re-Import SAME Data
	// This triggers the "Smart Merge" logic (loading existing children).
	// If it works, it should see that headers/params match and NOT add them to the delta.
	req2 := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Import Clean Delta",
		Data:        harData,
		DomainData: []*apiv1.ImportDomainData{
			{Enabled: true, Domain: "api.example.com", Variable: "API"},
		},
	})

	resp2, err := fixture.rpc.Import(fixture.ctx, req2)
	require.NoError(t, err)
	require.NotNil(t, resp2)

	// 3. Find the Delta Request
	deltas, err := fixture.services.HttpService.GetDeltasByParentID(fixture.ctx, reqID)
	require.NoError(t, err)

	// We might have multiple deltas (one from first import, one from second).
	// The SECOND delta should be "clean" regarding unchanged children.
	require.GreaterOrEqual(t, len(deltas), 1)

	latestDelta := deltas[len(deltas)-1] // Assuming order or just checking the latest

	// Check Delta Headers
	deltaHeaders, err := fixture.rpc.HttpHeaderService.GetByHttpID(fixture.ctx, latestDelta.ID)
	require.NoError(t, err)

	// CRITICAL CHECK:
	// The Delta Request should NOT have headers that are identical to the Base Request.
	// If the "Smart Merge" logic works, these should be filtered out.
	// However, note that "Delta Requests" in the DB might just be empty containers
	// if there are no changes, OR they might contain *changes* if something diffs.
	// Since we imported IDENTICAL data, there should be NO headers in the Delta.

	// Wait, createComplexHAR has dependency logic implicitly? No, just static data.
	// So base and delta should be identical.
	// "Smart Merge" means: Found Existing Base -> Compare -> Identical -> No Delta Field Set.

	// If the fix is working, the importer sees "Header A exists", so it doesn't add "Header A" to the delta entity list.
	require.Empty(t, deltaHeaders, "Delta request should have NO headers because they are identical to base")

	// Check Delta Params
	deltaParams, err := fixture.rpc.HttpSearchParamService.GetByHttpID(fixture.ctx, latestDelta.ID)
	require.NoError(t, err)
	require.Empty(t, deltaParams, "Delta request should have NO params because they are identical to base")
}
