package rhttp

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	apiv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/http/v1"
)

// TestHttpVersionSync_ConcurrentUpdatesNoVersions verifies that concurrent HTTP
// updates do NOT create version events. Versions are only created by HttpRun.
func TestHttpVersionSync_ConcurrentUpdatesNoVersions(t *testing.T) {
	f := newHttpFixture(t)
	ctx := f.ctx

	// 1. Create Workspace & HTTP
	f.createWorkspace(t, "Test Workspace")
	httpID := idwrap.NewNow()
	_, err := f.handler.HttpInsert(ctx, connect.NewRequest(&apiv1.HttpInsertRequest{
		Items: []*apiv1.HttpInsert{
			{
				HttpId:   httpID.Bytes(),
				Name:     "Test Request",
				Method:   apiv1.HttpMethod_HTTP_METHOD_GET,
				Url:      "https://example.com",
				BodyKind: apiv1.HttpBodyKind_HTTP_BODY_KIND_RAW,
			},
		},
	}))
	require.NoError(t, err)

	// 2. Call HttpUpdate 5 times concurrently
	var wg sync.WaitGroup
	count := 5

	// Capture events
	eventCount := 0
	var eventMu sync.Mutex

	ctxStream, cancel := context.WithCancel(ctx)
	defer cancel()

	ready := make(chan struct{})

	// Start listener
	go func() {
		f.handler.streamHttpVersionSyncWithOptions(ctxStream, f.userID, func(resp *apiv1.HttpVersionSyncResponse) error {
			if len(resp.Items) > 0 {
				for _, item := range resp.Items {
					if item.GetValue().GetInsert() != nil {
						eventMu.Lock()
						eventCount++
						eventMu.Unlock()
					}
				}
			}
			return nil
		}, &eventstream.BulkOptions{Ready: ready})
	}()

	<-ready

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := fmt.Sprintf("Updated Name %d", idx)
			_, err := f.handler.HttpUpdate(ctx, connect.NewRequest(&apiv1.HttpUpdateRequest{
				Items: []*apiv1.HttpUpdate{
					{
						HttpId: httpID.Bytes(),
						Name:   &name,
					},
				},
			}))
			require.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// Give events time to propagate
	time.Sleep(500 * time.Millisecond)
	cancel() // Stop listener

	eventMu.Lock()
	defer eventMu.Unlock()

	// Updates should NOT create version events - only HttpRun creates versions
	require.Equal(t, 0, eventCount, "HTTP updates should not produce version sync events")
}
