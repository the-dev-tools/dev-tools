package rimportv2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	filesystemv1 "the-dev-tools/spec/dist/buf/go/api/file_system/v1"

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

	// Verify ZERO sync events for HTTP requests and non-flow Files (since they are deduplicated)
	// Note: Flow file events ARE expected because HAR imports always create new flows (with new IDs)
	// We wait a bit to make sure no unwanted events arrive
	timeout2 := time.After(500 * time.Millisecond)
loop2:
	for {
		select {
		case evt := <-httpCh:
			require.Fail(t, "Should not receive HTTP sync event for deduplicated request", "Event: %+v", evt.Payload)
		case evt := <-fileCh:
			// Flow file events are allowed - HAR imports always create new flows
			if evt.Payload.File != nil && evt.Payload.File.Kind == filesystemv1.FileKind_FILE_KIND_FLOW {
				continue // Expected: new flow = new flow file entry
			}
			require.Fail(t, "Should not receive File sync event for deduplicated file", "Event: %+v", evt.Payload)
		case <-timeout2:
			// SUCCESS: No unwanted events received
			break loop2
		}
	}
}

// TestImport_EventPublishingOrder verifies that events are published in correct order
// WITHIN the same stream. Cross-stream ordering cannot be reliably tested because
// different channels may deliver messages in different order than they were published.
//
// What we CAN verify:
// - Flow file events come before other file events (both on File stream)
// - Flow events are received (on Flow stream)
// - File events are received (on File stream)
func TestImport_EventPublishingOrder(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	fixture := newIntegrationTestFixture(t)
	harData := createSimpleHAR(t)

	// Subscribe to file stream to verify file ordering
	fileCh, err := fixture.streamers.File.Subscribe(fixture.ctx, nil)
	require.NoError(t, err)
	flowCh, err := fixture.streamers.Flow.Subscribe(fixture.ctx, nil)
	require.NoError(t, err)

	req := connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Order Test Import",
		Data:        harData,
		DomainData: []*apiv1.ImportDomainData{
			{Enabled: true, Domain: "api.example.com", Variable: "API_HOST"},
		},
	})

	_, err = fixture.rpc.Import(fixture.ctx, req)
	require.NoError(t, err)

	// Track file event order specifically (same stream = guaranteed order)
	var fileEventOrder []string
	flowReceived := false
	timeout := time.After(500 * time.Millisecond)

loop:
	for {
		select {
		case <-flowCh:
			flowReceived = true
		case evt := <-fileCh:
			if evt.Payload.File != nil && evt.Payload.File.Kind == filesystemv1.FileKind_FILE_KIND_FLOW {
				fileEventOrder = append(fileEventOrder, "flow_file")
			} else {
				fileEventOrder = append(fileEventOrder, "other_file")
			}
		case <-timeout:
			break loop
		}
	}

	// Verify we received flow event
	require.True(t, flowReceived, "Should receive flow event")

	// Verify we got file events
	require.NotEmpty(t, fileEventOrder, "Should receive file events")

	// Find positions within file events
	flowFileIdx := -1
	otherFileIdx := -1
	for i, e := range fileEventOrder {
		if e == "flow_file" && flowFileIdx == -1 {
			flowFileIdx = i
		}
		if e == "other_file" && otherFileIdx == -1 {
			otherFileIdx = i
		}
	}

	// Verify order within file stream: flow_file -> other_file
	// (Cross-stream order between Flow and File streams can't be tested reliably)
	if flowFileIdx != -1 && otherFileIdx != -1 {
		require.Less(t, flowFileIdx, otherFileIdx, "Flow file event should come before other file events")
	}

	t.Logf("Flow received: %v, File event order: %v", flowReceived, fileEventOrder)
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
