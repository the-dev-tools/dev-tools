package rhttp

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/idwrap"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

func TestHttpVersionSync_Concurrency(t *testing.T) {
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

	// Start listener
	go func() {
		f.handler.streamHttpVersionSync(ctxStream, f.userID, func(resp *apiv1.HttpVersionSyncResponse) error {
			if len(resp.Items) > 0 {
				for _, item := range resp.Items {
					if item.GetValue().GetInsert() != nil {
						// Log for debug
						// fmt.Printf("Received insert event for %s\n", item.GetValue().GetInsert().HttpVersionId)
						eventMu.Lock()
						eventCount++
						eventMu.Unlock()
					}
				}
			}
			return nil
		})
	}()

	// Give listener time to start
	time.Sleep(100 * time.Millisecond)

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
	
	// We expect 5 insert events (one per update) + 1 initial snapshot insert?
	// The initial snapshot might have 0 versions if we didn't create any manually.
	// HttpInsert does NOT create a version in current logic (only HttpUpdate).
	// So we expect exactly 5 events.
	require.Equal(t, count, eventCount, "Should receive exactly 5 HttpVersionSync insert events")
}

func TestHttpRun_DoesNotCreateVersion(t *testing.T) {
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

	// 2. Call HttpRun 5 times
	for i := 0; i < 5; i++ {
		_, err := f.handler.HttpRun(ctx, connect.NewRequest(&apiv1.HttpRunRequest{
			HttpId: httpID.Bytes(),
		}))
		require.NoError(t, err)
	}

	// 3. Verify Versions count
	versions, err := f.handler.getHttpVersionsByHttpID(ctx, httpID)
	require.NoError(t, err)
	
	require.Equal(t, 0, len(versions), "HttpRun should not create HttpVersion records")
}
