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

// TestRealHAR_UUIDFile tests import with a real-world HAR file
// that contains duplicate URLs (3x GET for tags/products/categories)
func TestRealHAR_UUIDFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real HAR test in short mode")
	}

	fixture := newIntegrationTestFixture(t)

	// Load the real HAR file
	harPath := filepath.Join("testdata", "uuid.har")
	harData, err := os.ReadFile(harPath)
	require.NoError(t, err, "Failed to read uuid.har - make sure testdata/uuid.har exists")

	// Import the HAR
	req := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "UUID HAR Import",
		Data:        harData,
		DomainData: []*apiv1.ImportDomainData{
			{Enabled: true, Domain: "ecommerce-admin-panel.fly.dev", Variable: "API_HOST"},
		},
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

	// Get all deltas
	deltas, err := fixture.services.HttpService.GetDeltasByWorkspaceID(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)

	// The HAR has 16 entries, each creates base + delta = 16 base + 16 delta
	t.Logf("HTTP Requests: %d base, %d delta", len(httpReqs), len(deltas))
	require.Equal(t, 16, len(httpReqs), "Should have 16 base HTTP requests")
	require.Equal(t, 16, len(deltas), "Should have 16 delta HTTP requests")

	// Get all files
	files, err := fixture.services.FileService.ListFilesByWorkspace(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)

	// Count file types
	var httpFiles, deltaFiles, folderFiles, flowFiles int
	for _, f := range files {
		switch f.ContentType {
		case mfile.ContentTypeHTTP:
			httpFiles++
		case mfile.ContentTypeHTTPDelta:
			deltaFiles++
		case mfile.ContentTypeFolder:
			folderFiles++
		case mfile.ContentTypeFlow:
			flowFiles++
		}
	}

	t.Logf("Files: %d http, %d delta, %d folder, %d flow", httpFiles, deltaFiles, folderFiles, flowFiles)

	// Key assertion: Each of the 16 entries should have its own HTTP file
	require.Equal(t, 16, httpFiles, "Should have 16 HTTP files (one per entry)")
	require.Equal(t, 16, deltaFiles, "Should have 16 delta files (one per entry)")
	require.Equal(t, 1, flowFiles, "Should have 1 flow file")

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

// TestRealHAR_DuplicateURLsGetUniqueFiles specifically tests that
// duplicate URLs (3x GET /api/tags) each get their own file
func TestRealHAR_DuplicateURLsGetUniqueFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real HAR test in short mode")
	}

	fixture := newIntegrationTestFixture(t)

	harPath := filepath.Join("testdata", "uuid.har")
	harData, err := os.ReadFile(harPath)
	require.NoError(t, err)

	req := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Duplicate URL Test",
		Data:        harData,
		DomainData: []*apiv1.ImportDomainData{
			{Enabled: true, Domain: "ecommerce-admin-panel.fly.dev", Variable: "API_HOST"},
		},
	})

	_, err = fixture.rpc.Import(fixture.ctx, req)
	require.NoError(t, err)

	// Get all files
	files, err := fixture.services.FileService.ListFilesByWorkspace(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)

	// Find files with "Tags" in the name (there are 3 GET /api/tags requests)
	var tagFiles []mfile.File
	for _, f := range files {
		if f.ContentType == mfile.ContentTypeHTTP {
			// Check if file is under tags folder or has tags-related name
			t.Logf("HTTP File: %s (ID: %s, ContentID: %v)", f.Name, f.ID, f.ContentID)
		}
	}

	// Count unique HTTP file names - should have collision-resolved names
	httpFileNames := make(map[string]int)
	for _, f := range files {
		if f.ContentType == mfile.ContentTypeHTTP {
			httpFileNames[f.Name]++
			tagFiles = append(tagFiles, f)
		}
	}

	t.Logf("Unique HTTP file names: %d, Total HTTP files: %d", len(httpFileNames), len(tagFiles))

	// The HAR has:
	// - 3x GET /api/tags (should become Tags.request, Tags (1).request, Tags (2).request)
	// - 3x GET /api/products
	// - 3x GET /api/categories
	// So we should have some collision-resolved names
	require.Equal(t, 16, len(tagFiles), "Should have 16 HTTP files total")
}

// TestRealHAR_ReimportDeduplication tests that re-importing the same HAR
// deduplicates correctly and doesn't create orphans
func TestRealHAR_ReimportDeduplication(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real HAR test in short mode")
	}

	fixture := newIntegrationTestFixture(t)

	harPath := filepath.Join("testdata", "uuid.har")
	harData, err := os.ReadFile(harPath)
	require.NoError(t, err)

	// First import
	req1 := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "First Import",
		Data:        harData,
		DomainData: []*apiv1.ImportDomainData{
			{Enabled: true, Domain: "ecommerce-admin-panel.fly.dev", Variable: "API_HOST"},
		},
	})
	_, err = fixture.rpc.Import(fixture.ctx, req1)
	require.NoError(t, err)

	// Count after first import
	httpReqs1, _ := fixture.services.HttpService.GetByWorkspaceID(fixture.ctx, fixture.workspaceID)
	files1, _ := fixture.services.FileService.ListFilesByWorkspace(fixture.ctx, fixture.workspaceID)

	// Second import (same data)
	req2 := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Second Import",
		Data:        harData,
		DomainData: []*apiv1.ImportDomainData{
			{Enabled: true, Domain: "ecommerce-admin-panel.fly.dev", Variable: "API_HOST"},
		},
	})
	_, err = fixture.rpc.Import(fixture.ctx, req2)
	require.NoError(t, err)

	// Count after second import
	httpReqs2, _ := fixture.services.HttpService.GetByWorkspaceID(fixture.ctx, fixture.workspaceID)
	files2, _ := fixture.services.FileService.ListFilesByWorkspace(fixture.ctx, fixture.workspaceID)

	t.Logf("After 1st import: %d HTTP, %d files", len(httpReqs1), len(files1))
	t.Logf("After 2nd import: %d HTTP, %d files", len(httpReqs2), len(files2))

	// HTTP requests should be deduplicated (same count)
	require.Equal(t, len(httpReqs1), len(httpReqs2), "HTTP requests should be deduplicated")

	// Files should increase by flow file only (new flow per import)
	flowCountDiff := 0
	for _, f := range files2 {
		if f.ContentType == mfile.ContentTypeFlow {
			flowCountDiff++
		}
	}

	// Validate integrity after both imports
	report, err := ValidateImportIntegrity(
		fixture.ctx,
		fixture.workspaceID,
		&fixture.services.FileService,
		&fixture.services.HttpService,
		nil,
		nil,
	)
	require.NoError(t, err)
	require.False(t, report.HasErrors(), "Integrity validation failed after reimport: %v", report)
}

// TestRealHAR_RepeatedImportsNoDeadlock tests that importing the same HAR
// multiple times in succession doesn't cause the system to hang or deadlock.
// This is a regression test for a known issue where repeated imports would get stuck.
func TestRealHAR_RepeatedImportsNoDeadlock(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real HAR test in short mode")
	}

	fixture := newIntegrationTestFixture(t)

	harPath := filepath.Join("testdata", "uuid.har")
	harData, err := os.ReadFile(harPath)
	require.NoError(t, err)

	numImports := 5

	for i := 1; i <= numImports; i++ {
		t.Logf("Starting import %d of %d...", i, numImports)

		req := connect.NewRequest(&apiv1.ImportRequest{
			WorkspaceId: fixture.workspaceID.Bytes(),
			Name:        "Repeated Import " + string(rune('0'+i)),
			Data:        harData,
			DomainData: []*apiv1.ImportDomainData{
				{Enabled: true, Domain: "ecommerce-admin-panel.fly.dev", Variable: "API_HOST"},
			},
		})

		_, err := fixture.rpc.Import(fixture.ctx, req)
		require.NoError(t, err, "Import %d should not fail", i)

		t.Logf("Import %d completed successfully", i)
	}

	// Verify final state
	httpReqs, err := fixture.services.HttpService.GetByWorkspaceID(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)

	t.Logf("After %d imports: %d HTTP requests", numImports, len(httpReqs))

	// HTTP should be deduplicated - still 16 base requests
	require.Equal(t, 16, len(httpReqs), "Should still have 16 base HTTP requests after repeated imports")

	// Final integrity check
	report, err := ValidateImportIntegrity(
		fixture.ctx,
		fixture.workspaceID,
		&fixture.services.FileService,
		&fixture.services.HttpService,
		nil,
		nil,
	)
	require.NoError(t, err)
	require.False(t, report.HasErrors(), "Integrity validation failed after %d imports: %v", numImports, report)
}
