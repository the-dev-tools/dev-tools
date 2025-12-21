package rimportv2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	apiv1 "the-dev-tools/spec/dist/buf/go/api/import/v1"

	"connectrpc.com/connect"
)

// TestImport_SyncParity_Deduplication verifies that re-importing the same data
// triggers zero sync events for existing entities.
func TestImport_SyncParity_Deduplication(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	fixture := newIntegrationTestFixture(t)
	harData := createSimpleHAR(t)

	// Step 1: First Import (Everything is new)
	// ----------------------------------------

	// Subscribe to streamers to count events
	httpCh, err := fixture.streamers.Http.Subscribe(fixture.ctx, nil)
	require.NoError(t, err)
	fileCh, err := fixture.streamers.File.Subscribe(fixture.ctx, nil)
	require.NoError(t, err)

	req1 := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "First Import",
		Data:        harData,
		DomainData: []*apiv1.ImportDomainData{
			{Enabled: true, Domain: "api.example.com", Variable: "API_HOST"},
		},
	})

	_, err = fixture.rpc.Import(fixture.ctx, req1)
	require.NoError(t, err)

	// Drain events from first import
	eventCount := 0
	timeout := time.After(500 * time.Millisecond)
loop1:
	for {
		select {
		case <-httpCh:
			eventCount++
		case <-fileCh:
			eventCount++
		case <-timeout:
			break loop1
		}
	}
	require.Greater(t, eventCount, 0, "First import should trigger sync events")

	// Step 2: Second Import (Identical data -> Deduplication)
	// ------------------------------------------------------

	req2 := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Second Import",
		Data:        harData,
		DomainData: []*apiv1.ImportDomainData{
			{Enabled: true, Domain: "api.example.com", Variable: "API_HOST"},
		},
	})

	_, err = fixture.rpc.Import(fixture.ctx, req2)
	require.NoError(t, err)

	// Verify ZERO sync events for HTTP and Files (since they are deduplicated)
	// We wait a bit to make sure no events arrive
	select {
	case evt := <-httpCh:
		require.Fail(t, "Should not receive HTTP sync event for deduplicated request", "Event: %+v", evt.Payload)
	case evt := <-fileCh:
		require.Fail(t, "Should not receive File sync event for deduplicated file", "Event: %+v", evt.Payload)
	case <-time.After(500 * time.Millisecond):
		// SUCCESS: No events received
	}
}

// TestImport_SyncParity_MixedImport verifies that only NEW entities trigger sync events
func TestImport_SyncParity_MixedImport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	fixture := newIntegrationTestFixture(t)
	harData1 := createSimpleHAR(t)
	harData2 := createMultiEntryHAR(t) // Contains different requests

	// Step 1: Import First HAR
	req1 := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Base Import",
		Data:        harData1,
	})
	_, err := fixture.rpc.Import(fixture.ctx, req1)
	require.NoError(t, err)

	// Step 2: Subscribe and Import Second HAR (Mixed data)
	httpCh, err := fixture.streamers.Http.Subscribe(fixture.ctx, nil)
	require.NoError(t, err)

	req2 := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Mixed Import",
		Data:        harData2,
		DomainData: []*apiv1.ImportDomainData{
			{Enabled: true, Domain: "api.example.com", Variable: "API_HOST"},
		},
	})
	_, err = fixture.rpc.Import(fixture.ctx, req2)
	require.NoError(t, err)

	// Count events - should receive events for NEW requests in harData2, 
	// but NOT for any that might have overlapped with harData1 (if any).
	// harData2 created via createMultiEntryHAR has different URLs than harData1.
	
	newRequestEvents := 0
	timeout := time.After(500 * time.Millisecond)
loop2:
	for {
		select {
		case <-httpCh:
			newRequestEvents++
		case <-timeout:
			break loop2
		}
	}
	
	// createMultiEntryHAR creates 3 entries. 
	// harv2 processes each entry twice (Base + Delta), so we expect events for them.
	// We expect 3 base requests + 3 delta requests = 6 events.
	require.Equal(t, 6, newRequestEvents, "Should receive sync events only for new requests")
}
