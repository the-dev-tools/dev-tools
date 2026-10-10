package rhttp

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	apiv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/http/v1"
)

// A duplicated request keeps its stream: settings.
func TestHttpDuplicate_KeepsStream(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttpWithUrl(t, ws, "chat", "https://app.test/api/chat", "POST")
	streams := shttp.NewHTTPStreamService(f.base.Queries)
	require.NoError(t, streams.Upsert(f.ctx, mhttp.HTTPStream{HttpID: httpID, Preset: "vercel-ai", TimeoutMs: 5000}))

	_, err := f.handler.HttpDuplicate(f.ctx, connect.NewRequest(&apiv1.HttpDuplicateRequest{HttpId: httpID.Bytes()}))
	require.NoError(t, err)

	all, err := f.hs.GetByWorkspaceID(f.ctx, ws)
	require.NoError(t, err)
	require.Len(t, all, 2)
	for _, h := range all {
		st, err := streams.Get(f.ctx, h.ID)
		require.NoError(t, err)
		require.Equal(t, &mhttp.HTTPStream{HttpID: h.ID, Preset: "vercel-ai", TimeoutMs: 5000}, st)
	}
}
