package rhttp

import (
	"bytes"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	httpv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/http/v1"
)

func TestHttpHeaderDeltaCollection_ReturnsCorrectDeltas(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "delta-test-workspace")

	// 1. Setup Base Request & Header
	baseHttpID := f.createHttp(t, ws, "Base Request")

	baseHeaderID := idwrap.NewNow()
	baseHeader := &mhttp.HTTPHeader{
		ID:      baseHeaderID,
		HttpID:  baseHttpID,
		Key:     "X-Base",
		Value:   "true",
		Enabled: true,
		IsDelta: false,
	}
	require.NoError(t, f.handler.httpHeaderService.Create(f.ctx, baseHeader), "failed to create base header")

	// 2. Create Delta Header (Override)
	deltaHttpID := idwrap.NewNow()
	deltaHttp := &mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  ws,
		Name:         "Delta Request",
		ParentHttpID: &baseHttpID,
		IsDelta:      true,
	}
	require.NoError(t, f.hs.Create(f.ctx, deltaHttp), "failed to create delta http")

	deltaHeaderID := idwrap.NewNow()
	deltaValue := "false"
	deltaHeader := &mhttp.HTTPHeader{
		ID:                 deltaHeaderID,
		HttpID:             deltaHttpID,   // The Delta Request this override belongs to
		ParentHttpHeaderID: &baseHeaderID, // The Base Header this overrides
		IsDelta:            true,
		DeltaValue:         &deltaValue, // Override
	}
	// Create the delta header. Assuming Create handles IsDelta correctly (it does based on schema)
	require.NoError(t, f.handler.httpHeaderService.Create(f.ctx, deltaHeader), "failed to create delta header")

	// 3. Call RPC
	resp, err := f.handler.HttpHeaderDeltaCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err, "HttpHeaderDeltaCollection failed")

	// 4. Verify logic
	var foundDelta *httpv1.HttpHeaderDelta
	for _, item := range resp.Msg.Items {
		if bytes.Equal(item.DeltaHttpHeaderId, deltaHeaderID.Bytes()) {
			foundDelta = item
			break
		}
	}

	require.NotNil(t, foundDelta, "Delta header not found in response")

	// CHECK 1: HttpHeaderId should be the PARENT ID (Base Header ID)
	require.True(t, bytes.Equal(foundDelta.HttpHeaderId, baseHeaderID.Bytes()), "Expected HttpHeaderId to be %s (Base), got %x", baseHeaderID, foundDelta.HttpHeaderId)

	// CHECK 2: Value should be the delta override
	require.NotNil(t, foundDelta.Value, "Expected Value to be set")
	require.Equal(t, deltaValue, *foundDelta.Value, "Expected Value to be %s", deltaValue)

	// CHECK 3: Base header should NOT be returned as a delta
	for _, item := range resp.Msg.Items {
		require.False(t, bytes.Equal(item.DeltaHttpHeaderId, baseHeaderID.Bytes()), "Base header incorrectly returned in Delta Collection")
	}
}
