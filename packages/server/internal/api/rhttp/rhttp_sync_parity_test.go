package rhttp

import (
	"bytes"
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

func TestHttpBodyRawDelta_SyncParity(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "parity-test-workspace")

	// 1. Setup Base Request
	baseHttpID := f.createHttp(t, ws, "Base Request")
	baseBody := &mhttp.HTTPBodyRaw{
		ID:          idwrap.NewNow(),
		HttpID:      baseHttpID,
		RawData:     []byte("base"),
		ContentType: "text/plain",
	}
	_, err := f.handler.bodyService.CreateFull(f.ctx, baseBody)
	require.NoError(t, err)

	// 2. Setup Delta Request
	deltaHttpID := idwrap.NewNow()
	deltaHttp := &mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  ws,
		Name:         "Delta Request",
		ParentHttpID: &baseHttpID,
		IsDelta:      true,
	}
	require.NoError(t, f.hs.Create(f.ctx, deltaHttp))

	deltaBody := &mhttp.HTTPBodyRaw{
		ID:              idwrap.NewNow(),
		HttpID:          deltaHttpID,
		ParentBodyRawID: &baseBody.ID,
		IsDelta:         true,
		DeltaRawData:    []byte("delta"),
	}
	_, err = f.handler.bodyService.CreateFull(f.ctx, deltaBody)
	require.NoError(t, err)

	// 3. Get Collection Response
	collResp, err := f.handler.HttpBodyRawDeltaCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)

	// 4. Get Sync Response (Mock Stream)
	var syncItems []*apiv1.HttpBodyRawDeltaSync

	// Create a cancellable context to stop the infinite stream loop
	syncCtx, cancel := context.WithCancel(f.ctx)

	// Start sync in goroutine
	go func() {
		defer cancel() // ensure cleanup
		_ = f.handler.streamHttpBodyRawDeltaSync(syncCtx, f.userID, func(resp *apiv1.HttpBodyRawDeltaSyncResponse) error {
			syncItems = append(syncItems, resp.Items...)
			return nil
		})
	}()

	// Give subscription time to establish
	time.Sleep(100 * time.Millisecond)

	// Trigger an update
	updateValStr := "delta-rpc-update"
	updateReq := &apiv1.HttpBodyRawDeltaUpdateRequest{
		Items: []*apiv1.HttpBodyRawDeltaUpdate{
			{
				HttpId: deltaHttpID.Bytes(),
				Data: &apiv1.HttpBodyRawDeltaUpdate_DataUnion{
					Kind:  apiv1.HttpBodyRawDeltaUpdate_DataUnion_KIND_VALUE,
					Value: &updateValStr,
				},
			},
		},
	}
	_, err = f.handler.HttpBodyRawDeltaUpdate(f.ctx, connect.NewRequest(updateReq))
	require.NoError(t, err)

	// Give it a moment to process
	time.Sleep(50 * time.Millisecond)
	cancel() // Stop the sync stream

	// 5. Compare
	require.NotEmpty(t, collResp.Msg.Items, "Collection should return items")
	require.NotEmpty(t, syncItems, "Sync should have caught the update event")

	collItem := collResp.Msg.Items[0]
	// Find the sync item that corresponds (likely the last one if update happened)
	syncItem := syncItems[len(syncItems)-1]

	// Check Collection Fields
	require.True(t, bytes.Equal(collItem.HttpId, deltaHttpID.Bytes()), "Collection HttpId mismatch")
	require.True(t, bytes.Equal(collItem.DeltaHttpId, deltaHttpID.Bytes()), "Collection DeltaHttpId mismatch (FIX VERIFICATION)")

	// Check Sync Fields
	// Sync item is a Union, handling Update
	require.NotNil(t, syncItem.Value, "Sync item value nil")
	updateVal := syncItem.Value.GetUpdate()
	require.NotNil(t, updateVal, "Sync item should be Update")

	require.True(t, bytes.Equal(updateVal.HttpId, deltaHttpID.Bytes()), "Sync HttpId mismatch")
	require.True(t, bytes.Equal(updateVal.DeltaHttpId, deltaHttpID.Bytes()), "Sync DeltaHttpId mismatch")

	// Verify Parity: Both must have DeltaHttpId populated
	require.Equal(t, collItem.DeltaHttpId, updateVal.DeltaHttpId, "Collection and Sync should have identical DeltaHttpId")
}
