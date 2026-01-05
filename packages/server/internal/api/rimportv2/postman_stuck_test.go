package rimportv2

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"the-dev-tools/server/pkg/model/mfile"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/import/v1"

	"connectrpc.com/connect"
)

func TestPostmanImport_GalaxyCollection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	fixture := newIntegrationTestFixture(t)

	// Load the Galaxy collection from packages/server/test/collection/
	collectionPath := filepath.Join("..", "..", "..", "test", "collection", "GalaxyCollection.json")
	data, err := os.ReadFile(collectionPath)
	require.NoError(t, err, "Failed to read GalaxyCollection.json")

	// Subscribe to event streamers BEFORE import to verify sync events
	httpCh, err := fixture.streamers.Http.Subscribe(fixture.ctx, nil)
	require.NoError(t, err)

	flowCh, err := fixture.streamers.Flow.Subscribe(fixture.ctx, nil)
	require.NoError(t, err)

	nodeCh, err := fixture.streamers.Node.Subscribe(fixture.ctx, nil)
	require.NoError(t, err)

	edgeCh, err := fixture.streamers.Edge.Subscribe(fixture.ctx, nil)
	require.NoError(t, err)

	fileCh, err := fixture.streamers.File.Subscribe(fixture.ctx, nil)
	require.NoError(t, err)

	headerCh, err := fixture.streamers.HttpHeader.Subscribe(fixture.ctx, nil)
	require.NoError(t, err)

	// Set a timeout to detect hangs
	ctx, cancel := context.WithTimeout(fixture.ctx, 15*time.Second)
	defer cancel()

	req := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Galaxy Collection Import",
		Data:        data,
		// Provide empty domain data to skip the two-step flow and proceed to storage
		DomainData: []*apiv1.ImportDomainData{},
	})

	t.Log("Starting RPC call for Galaxy collection...")
	start := time.Now()

	resp, err := fixture.rpc.Import(ctx, req)

	duration := time.Since(start)
	t.Logf("RPC call finished in %v", duration)

	require.NoError(t, err, "Import should not fail or hang")
	require.NotNil(t, resp)
	require.Equal(t, apiv1.ImportMissingDataKind_IMPORT_MISSING_DATA_KIND_UNSPECIFIED, resp.Msg.MissingData)

	// ========== Verify Sync Events ==========
	t.Log("Verifying sync events...")

	// Check Flow event
	select {
	case evt := <-flowCh:
		require.Equal(t, "insert", evt.Payload.Type)
		require.NotNil(t, evt.Payload.Flow)
		t.Logf("Received Flow sync event: %s", evt.Payload.Flow.Name)
	case <-time.After(2 * time.Second):
		require.Fail(t, "Timed out waiting for Flow sync event")
	}

	// Check HTTP events (should receive multiple)
	select {
	case evt := <-httpCh:
		require.Equal(t, "insert", evt.Payload.Type)
		require.NotNil(t, evt.Payload.Http)
		t.Logf("Received HTTP sync event: %s %s", evt.Payload.Http.Method, evt.Payload.Http.Name)
	case <-time.After(2 * time.Second):
		require.Fail(t, "Timed out waiting for HTTP sync event")
	}

	// Check Node events
	select {
	case evt := <-nodeCh:
		require.Equal(t, "insert", evt.Payload.Type)
		require.NotNil(t, evt.Payload.Node)
		t.Logf("Received Node sync event: %s", evt.Payload.Node.Name)
	case <-time.After(2 * time.Second):
		require.Fail(t, "Timed out waiting for Node sync event")
	}

	// Check Edge events
	select {
	case evt := <-edgeCh:
		require.Equal(t, "insert", evt.Payload.Type)
		require.NotNil(t, evt.Payload.Edge)
		t.Log("Received Edge sync event")
	case <-time.After(2 * time.Second):
		require.Fail(t, "Timed out waiting for Edge sync event")
	}

	// Check File events
	select {
	case evt := <-fileCh:
		require.Equal(t, "create", evt.Payload.Type)
		require.NotNil(t, evt.Payload.File)
		t.Logf("Received File sync event: kind=%v", evt.Payload.File.Kind)
	case <-time.After(2 * time.Second):
		require.Fail(t, "Timed out waiting for File sync event")
	}

	// Check Header events
	select {
	case evt := <-headerCh:
		require.Equal(t, "insert", evt.Payload.Type)
		require.NotNil(t, evt.Payload.HttpHeader)
		t.Logf("Received Header sync event: %s", evt.Payload.HttpHeader.Key)
	case <-time.After(2 * time.Second):
		require.Fail(t, "Timed out waiting for Header sync event")
	}

	// ========== Verify Database Storage ==========
	t.Log("Verifying database storage...")

	// Verify HTTP requests were stored
	httpReqs, err := fixture.services.HttpService.GetByWorkspaceID(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)
	t.Logf("Stored %d HTTP requests", len(httpReqs))
	require.GreaterOrEqual(t, len(httpReqs), 5, "Should have at least 5 HTTP requests")

	// Verify flows were created
	flows, err := fixture.services.FlowService.GetFlowsByWorkspaceID(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)
	require.Len(t, flows, 1, "Should have exactly 1 flow")
	t.Logf("Created flow: %s", flows[0].Name)

	// Verify nodes were created (start node + request nodes)
	nodes, err := fixture.rpc.NodeService.GetNodesByFlowID(fixture.ctx, flows[0].ID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(nodes), 5, "Should have at least 5 nodes (start + requests)")
	t.Logf("Created %d nodes", len(nodes))

	// Verify edges were created
	edges, err := fixture.rpc.EdgeService.GetEdgesByFlowID(fixture.ctx, flows[0].ID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(edges), 1, "Should have at least 1 edge")
	t.Logf("Created %d edges", len(edges))

	// Verify files were created (folder + HTTP files)
	files, err := fixture.services.FileService.ListFilesByWorkspace(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)
	require.Greater(t, len(files), 0, "Should have files created")

	// Count file types
	var folderCount, httpCount, flowCount int
	for _, f := range files {
		switch f.ContentType {
		case mfile.ContentTypeFolder:
			folderCount++
		case mfile.ContentTypeHTTP:
			httpCount++
		case mfile.ContentTypeFlow:
			flowCount++
		}
	}
	t.Logf("Files: %d folders, %d HTTP, %d flows", folderCount, httpCount, flowCount)

	require.Equal(t, 1, flowCount, "Should have 1 flow file")
	require.GreaterOrEqual(t, httpCount, 5, "Should have at least 5 HTTP files")

	t.Logf("Galaxy collection import completed successfully in %v", duration)
}
