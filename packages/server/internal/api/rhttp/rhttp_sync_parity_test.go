package rhttp

import (
	"bytes"
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/testutil"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

func TestHttpBodyRawDelta_SyncParity(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "parity-test-workspace")
	var deltaHttpID idwrap.IDWrap
	var deltaBodyID idwrap.IDWrap

	testutil.VerifySyncParity(t, testutil.SyncParityTestConfig[*apiv1.HttpBodyRawDelta, *apiv1.HttpBodyRawDeltaSync]{
		Setup: func(t *testing.T) (context.Context, func()) {
			// 1. Setup Base Request
			baseHttpID := f.createHttp(t, ws, "Base Request")
			baseBody := &mhttp.HTTPBodyRaw{
				ID:      idwrap.NewNow(),
				HttpID:  baseHttpID,
				RawData: []byte("base"),
			}
			_, err := f.handler.bodyService.CreateFull(f.ctx, baseBody)
			require.NoError(t, err)

			// 2. Setup Delta Request
			deltaHttpID = idwrap.NewNow()
			deltaHttp := &mhttp.HTTP{
				ID:           deltaHttpID,
				WorkspaceID:  ws,
				Name:         "Delta Request",
				ParentHttpID: &baseHttpID,
				IsDelta:      true,
			}
			require.NoError(t, f.hs.Create(f.ctx, deltaHttp))

			deltaBodyID = idwrap.NewNow()
			deltaBody := &mhttp.HTTPBodyRaw{
				ID:              deltaBodyID,
				HttpID:          deltaHttpID,
				ParentBodyRawID: &baseBody.ID,
				IsDelta:         true,
				DeltaRawData:    []byte("delta"),
			}
			_, err = f.handler.bodyService.CreateFull(f.ctx, deltaBody)
			require.NoError(t, err)

			return f.ctx, func() {}
		},
		StartSync: func(ctx context.Context, t *testing.T) (<-chan *apiv1.HttpBodyRawDeltaSync, func()) {
			ch := make(chan *apiv1.HttpBodyRawDeltaSync, 10)
			syncCtx, cancel := context.WithCancel(ctx)
			go func() {
				_ = f.handler.streamHttpBodyRawDeltaSync(syncCtx, f.userID, func(resp *apiv1.HttpBodyRawDeltaSyncResponse) error {
					for _, item := range resp.Items {
						ch <- item
					}
					return nil
				})
				close(ch)
			}()
			return ch, cancel
		},
		TriggerUpdate: func(ctx context.Context, t *testing.T) {
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
			_, err := f.handler.HttpBodyRawDeltaUpdate(ctx, connect.NewRequest(updateReq))
			require.NoError(t, err)
		},
		GetCollection: func(ctx context.Context, t *testing.T) []*apiv1.HttpBodyRawDelta {
			collResp, err := f.handler.HttpBodyRawDeltaCollection(ctx, connect.NewRequest(&emptypb.Empty{}))
			require.NoError(t, err)
			return collResp.Msg.Items
		},
		Compare: func(t *testing.T, collItem *apiv1.HttpBodyRawDelta, syncItem *apiv1.HttpBodyRawDeltaSync) {
			require.True(t, bytes.Equal(collItem.HttpId, deltaHttpID.Bytes()), "Collection HttpId mismatch")
			require.True(t, bytes.Equal(collItem.DeltaHttpId, deltaHttpID.Bytes()), "Collection DeltaHttpId mismatch")

			updateVal := syncItem.Value.GetUpdate()
			require.NotNil(t, updateVal, "Sync item should be Update")

			require.True(t, bytes.Equal(updateVal.HttpId, deltaHttpID.Bytes()), "Sync HttpId mismatch")
			require.True(t, bytes.Equal(updateVal.DeltaHttpId, deltaHttpID.Bytes()), "Sync DeltaHttpId mismatch")

			require.Equal(t, collItem.DeltaHttpId, updateVal.DeltaHttpId, "Collection and Sync should have identical DeltaHttpId")
		},
	})
}

func TestHttpHeaderDelta_SyncParity(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "parity-test-workspace")
	var deltaHttpID idwrap.IDWrap
	var deltaHeaderID idwrap.IDWrap

	testutil.VerifySyncParity(t, testutil.SyncParityTestConfig[*apiv1.HttpHeaderDelta, *apiv1.HttpHeaderDeltaSync]{
		Setup: func(t *testing.T) (context.Context, func()) {
			// 1. Setup Base Request
			baseHttpID := f.createHttp(t, ws, "Base Request")

			// Setup Base Header
			baseHeaderID := idwrap.NewNow()
			baseHeader := &mhttp.HTTPHeader{
				ID:      baseHeaderID,
				HttpID:  baseHttpID,
				Key:     "X-Delta",
				Value:   "base-val",
				Enabled: true,
			}
			require.NoError(t, f.handler.httpHeaderService.Create(f.ctx, baseHeader))

			// 2. Setup Delta Request
			deltaHttpID = idwrap.NewNow()
			deltaHttp := &mhttp.HTTP{
				ID:           deltaHttpID,
				WorkspaceID:  ws,
				Name:         "Delta Request",
				ParentHttpID: &baseHttpID,
				IsDelta:      true,
			}
			require.NoError(t, f.hs.Create(f.ctx, deltaHttp))

			// 3. Create Delta Header
			deltaHeaderID = idwrap.NewNow()
			deltaVal := "val"
			deltaHeader := &mhttp.HTTPHeader{
				ID:                 deltaHeaderID,
				HttpID:             deltaHttpID,
				ParentHttpHeaderID: &baseHeaderID,
				Key:                "X-Delta",
				IsDelta:            true,
				DeltaValue:         &deltaVal,
			}
			require.NoError(t, f.handler.httpHeaderService.Create(f.ctx, deltaHeader))

			return f.ctx, func() {}
		},
		StartSync: func(ctx context.Context, t *testing.T) (<-chan *apiv1.HttpHeaderDeltaSync, func()) {
			ch := make(chan *apiv1.HttpHeaderDeltaSync, 10)
			syncCtx, cancel := context.WithCancel(ctx)
			go func() {
				_ = f.handler.streamHttpHeaderDeltaSync(syncCtx, f.userID, func(resp *apiv1.HttpHeaderDeltaSyncResponse) error {
					for _, item := range resp.Items {
						ch <- item
					}
					return nil
				})
				close(ch)
			}()
			return ch, cancel
		},
		TriggerUpdate: func(ctx context.Context, t *testing.T) {
			// Trigger update via service or RPC
			newVal := "new-val"
			updateReq := &apiv1.HttpHeaderDeltaUpdateRequest{
				Items: []*apiv1.HttpHeaderDeltaUpdate{
					{
						DeltaHttpHeaderId: deltaHeaderID.Bytes(),
						Value: &apiv1.HttpHeaderDeltaUpdate_ValueUnion{
							Kind:  apiv1.HttpHeaderDeltaUpdate_ValueUnion_KIND_VALUE,
							Value: &newVal,
						},
					},
				},
			}
			_, err := f.handler.HttpHeaderDeltaUpdate(ctx, connect.NewRequest(updateReq))
			require.NoError(t, err)
		},
		GetCollection: func(ctx context.Context, t *testing.T) []*apiv1.HttpHeaderDelta {
			collResp, err := f.handler.HttpHeaderDeltaCollection(ctx, connect.NewRequest(&emptypb.Empty{}))
			require.NoError(t, err)
			return collResp.Msg.Items
		},
		Compare: func(t *testing.T, collItem *apiv1.HttpHeaderDelta, syncItem *apiv1.HttpHeaderDeltaSync) {
			require.True(t, bytes.Equal(collItem.DeltaHttpHeaderId, deltaHeaderID.Bytes()), "Collection ID mismatch")

			updateVal := syncItem.Value.GetUpdate()
			require.NotNil(t, updateVal, "Sync item should be Update")

			require.True(t, bytes.Equal(updateVal.DeltaHttpHeaderId, deltaHeaderID.Bytes()), "Sync ID mismatch")

			// Parity check (e.g. check HttpId / mapping)
			// Header delta collection returns HttpId as base/parent ID usually if mapped, or just link.
			// Let's check consistency.
			require.Equal(t, collItem.DeltaHttpHeaderId, updateVal.DeltaHttpHeaderId)
		},
	})
}
