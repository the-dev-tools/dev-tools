package rhttp

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	apiv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/http/v1"
)

// TestHttpRun_SyncParityWithCollections verifies that every INSERT sync event
// published during HttpRun has a matching entry in the corresponding Collection
// endpoint. This catches bugs where the frontend receives sync events for data
// that doesn't exist in the collection, or vice versa.
func TestHttpRun_SyncParityWithCollections(t *testing.T) {
	t.Parallel()

	testServer := createTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Custom", "test-value")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"result":"ok"}`)
	})
	defer testServer.Close()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "sync-parity-workspace")
	httpID := f.createHttpWithUrl(t, ws, "sync-parity-test", testServer.URL, "POST")

	// Add child entities
	f.createHttpHeader(t, httpID, "X-Test", "header-val")
	f.createHttpHeader(t, httpID, "Authorization", "Bearer tok")
	f.createHttpSearchParam(t, httpID, "q", "search")
	f.createHttpAssertion(t, httpID, "response.status == 200", "status check")

	// -- Start sync streams BEFORE HttpRun --

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	readyHttp := make(chan struct{})
	readyHeader := make(chan struct{})
	readyParam := make(chan struct{})
	readyAssert := make(chan struct{})
	readyVersion := make(chan struct{})
	readyResponse := make(chan struct{})
	readyResponseHeader := make(chan struct{})

	// HttpSync (snapshot HTTP insert + base HTTP update)
	httpSyncCh := make(chan *apiv1.HttpSync, 20)
	go func() {
		f.handler.streamHttpSyncWithOptions(ctx, f.userID, func(resp *apiv1.HttpSyncResponse) error {
			for _, item := range resp.GetItems() {
				if item != nil {
					httpSyncCh <- item
				}
			}
			return nil
		}, &eventstream.BulkOptions{Ready: readyHttp})
		close(httpSyncCh)
	}()

	// HttpHeaderSync
	httpHeaderSyncCh := make(chan *apiv1.HttpHeaderSync, 20)
	go func() {
		f.handler.streamHttpHeaderSyncWithOptions(ctx, f.userID, func(resp *apiv1.HttpHeaderSyncResponse) error {
			for _, item := range resp.GetItems() {
				if item != nil {
					httpHeaderSyncCh <- item
				}
			}
			return nil
		}, &eventstream.BulkOptions{Ready: readyHeader})
		close(httpHeaderSyncCh)
	}()

	// HttpSearchParamSync
	httpParamSyncCh := make(chan *apiv1.HttpSearchParamSync, 20)
	go func() {
		f.handler.streamHttpSearchParamSyncWithOptions(ctx, f.userID, func(resp *apiv1.HttpSearchParamSyncResponse) error {
			for _, item := range resp.GetItems() {
				if item != nil {
					httpParamSyncCh <- item
				}
			}
			return nil
		}, &eventstream.BulkOptions{Ready: readyParam})
		close(httpParamSyncCh)
	}()

	// HttpAssertSync
	httpAssertSyncCh := make(chan *apiv1.HttpAssertSync, 20)
	go func() {
		f.handler.streamHttpAssertSyncWithOptions(ctx, f.userID, func(resp *apiv1.HttpAssertSyncResponse) error {
			for _, item := range resp.GetItems() {
				if item != nil {
					httpAssertSyncCh <- item
				}
			}
			return nil
		}, &eventstream.BulkOptions{Ready: readyAssert})
		close(httpAssertSyncCh)
	}()

	// HttpVersionSync
	httpVersionSyncCh := make(chan *apiv1.HttpVersionSync, 20)
	go func() {
		f.handler.streamHttpVersionSyncWithOptions(ctx, f.userID, func(resp *apiv1.HttpVersionSyncResponse) error {
			for _, item := range resp.GetItems() {
				if item != nil {
					httpVersionSyncCh <- item
				}
			}
			return nil
		}, &eventstream.BulkOptions{Ready: readyVersion})
		close(httpVersionSyncCh)
	}()

	// HttpResponseSync
	httpResponseSyncCh := make(chan *apiv1.HttpResponseSync, 20)
	go func() {
		f.handler.streamHttpResponseSyncWithOptions(ctx, f.userID, func(resp *apiv1.HttpResponseSyncResponse) error {
			for _, item := range resp.GetItems() {
				if item != nil {
					httpResponseSyncCh <- item
				}
			}
			return nil
		}, &eventstream.BulkOptions{Ready: readyResponse})
		close(httpResponseSyncCh)
	}()

	// HttpResponseHeaderSync
	httpResponseHeaderSyncCh := make(chan *apiv1.HttpResponseHeaderSync, 20)
	go func() {
		f.handler.streamHttpResponseHeaderSyncWithOptions(ctx, f.userID, func(resp *apiv1.HttpResponseHeaderSyncResponse) error {
			for _, item := range resp.GetItems() {
				if item != nil {
					httpResponseHeaderSyncCh <- item
				}
			}
			return nil
		}, &eventstream.BulkOptions{Ready: readyResponseHeader})
		close(httpResponseHeaderSyncCh)
	}()

	// Wait for all streams to be subscribed
	<-readyHttp
	<-readyHeader
	<-readyParam
	<-readyAssert
	<-readyVersion
	<-readyResponse
	<-readyResponseHeader

	// -- Execute HttpRun --
	runReq := connect.NewRequest(&apiv1.HttpRunRequest{HttpId: httpID.Bytes()})
	_, err := f.handler.HttpRun(f.ctx, runReq)
	require.NoError(t, err, "HttpRun should succeed")

	// -- Collect sync events with timeout (in parallel to avoid serial 3s waits) --
	timeout := 3 * time.Second

	var (
		httpSyncInsertIDs           [][]byte
		httpHeaderInsertIDs         [][]byte
		httpParamInsertIDs          [][]byte
		httpAssertInsertIDs         [][]byte
		httpVersionInsertIDs        [][]byte
		httpResponseInsertIDs       [][]byte
		httpResponseHeaderInsertIDs [][]byte
	)

	var wg sync.WaitGroup
	wg.Add(7)

	go func() {
		defer wg.Done()
		httpSyncInsertIDs = collectInsertIDs(t, httpSyncCh, timeout, "HttpSync", func(item *apiv1.HttpSync) ([]byte, bool) {
			v := item.GetValue()
			if v.GetKind() == apiv1.HttpSync_ValueUnion_KIND_INSERT {
				return v.GetInsert().GetHttpId(), true
			}
			return nil, false
		})
	}()

	go func() {
		defer wg.Done()
		httpHeaderInsertIDs = collectInsertIDs(t, httpHeaderSyncCh, timeout, "HttpHeaderSync", func(item *apiv1.HttpHeaderSync) ([]byte, bool) {
			v := item.GetValue()
			if v.GetKind() == apiv1.HttpHeaderSync_ValueUnion_KIND_INSERT {
				return v.GetInsert().GetHttpHeaderId(), true
			}
			return nil, false
		})
	}()

	go func() {
		defer wg.Done()
		httpParamInsertIDs = collectInsertIDs(t, httpParamSyncCh, timeout, "HttpSearchParamSync", func(item *apiv1.HttpSearchParamSync) ([]byte, bool) {
			v := item.GetValue()
			if v.GetKind() == apiv1.HttpSearchParamSync_ValueUnion_KIND_INSERT {
				return v.GetInsert().GetHttpSearchParamId(), true
			}
			return nil, false
		})
	}()

	go func() {
		defer wg.Done()
		httpAssertInsertIDs = collectInsertIDs(t, httpAssertSyncCh, timeout, "HttpAssertSync", func(item *apiv1.HttpAssertSync) ([]byte, bool) {
			v := item.GetValue()
			if v.GetKind() == apiv1.HttpAssertSync_ValueUnion_KIND_INSERT {
				return v.GetInsert().GetHttpAssertId(), true
			}
			return nil, false
		})
	}()

	go func() {
		defer wg.Done()
		httpVersionInsertIDs = collectInsertIDs(t, httpVersionSyncCh, timeout, "HttpVersionSync", func(item *apiv1.HttpVersionSync) ([]byte, bool) {
			v := item.GetValue()
			if v.GetKind() == apiv1.HttpVersionSync_ValueUnion_KIND_INSERT {
				return v.GetInsert().GetHttpVersionId(), true
			}
			return nil, false
		})
	}()

	go func() {
		defer wg.Done()
		httpResponseInsertIDs = collectInsertIDs(t, httpResponseSyncCh, timeout, "HttpResponseSync", func(item *apiv1.HttpResponseSync) ([]byte, bool) {
			v := item.GetValue()
			if v.GetKind() == apiv1.HttpResponseSync_ValueUnion_KIND_INSERT {
				return v.GetInsert().GetHttpResponseId(), true
			}
			return nil, false
		})
	}()

	go func() {
		defer wg.Done()
		httpResponseHeaderInsertIDs = collectInsertIDs(t, httpResponseHeaderSyncCh, timeout, "HttpResponseHeaderSync", func(item *apiv1.HttpResponseHeaderSync) ([]byte, bool) {
			v := item.GetValue()
			if v.GetKind() == apiv1.HttpResponseHeaderSync_ValueUnion_KIND_INSERT {
				return v.GetInsert().GetHttpResponseHeaderId(), true
			}
			return nil, false
		})
	}()

	wg.Wait()

	cancel() // Stop all streams

	// -- Get Collection data --

	t.Run("HttpSync_vs_HttpCollection", func(t *testing.T) {
		collResp, err := f.handler.HttpCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)
		collIDs := make(map[string]bool)
		for _, item := range collResp.Msg.Items {
			collIDs[string(item.GetHttpId())] = true
		}
		for _, syncID := range httpSyncInsertIDs {
			require.True(t, collIDs[string(syncID)],
				"HttpSync INSERT event with ID %v not found in HttpCollection", idFromBytes(syncID))
		}
	})

	t.Run("HttpHeaderSync_vs_HttpHeaderCollection", func(t *testing.T) {
		collResp, err := f.handler.HttpHeaderCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)
		collIDs := make(map[string]bool)
		for _, item := range collResp.Msg.Items {
			collIDs[string(item.GetHttpHeaderId())] = true
		}
		for _, syncID := range httpHeaderInsertIDs {
			require.True(t, collIDs[string(syncID)],
				"HttpHeaderSync INSERT event with ID %v not found in HttpHeaderCollection", idFromBytes(syncID))
		}
	})

	t.Run("HttpSearchParamSync_vs_HttpSearchParamCollection", func(t *testing.T) {
		collResp, err := f.handler.HttpSearchParamCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)
		collIDs := make(map[string]bool)
		for _, item := range collResp.Msg.Items {
			collIDs[string(item.GetHttpSearchParamId())] = true
		}
		for _, syncID := range httpParamInsertIDs {
			require.True(t, collIDs[string(syncID)],
				"HttpSearchParamSync INSERT event with ID %v not found in HttpSearchParamCollection", idFromBytes(syncID))
		}
	})

	t.Run("HttpAssertSync_vs_HttpAssertCollection", func(t *testing.T) {
		collResp, err := f.handler.HttpAssertCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)
		collIDs := make(map[string]bool)
		for _, item := range collResp.Msg.Items {
			collIDs[string(item.GetHttpAssertId())] = true
		}
		for _, syncID := range httpAssertInsertIDs {
			require.True(t, collIDs[string(syncID)],
				"HttpAssertSync INSERT event with ID %v not found in HttpAssertCollection", idFromBytes(syncID))
		}
	})

	t.Run("HttpVersionSync_vs_HttpVersionCollection", func(t *testing.T) {
		collResp, err := f.handler.HttpVersionCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)
		collIDs := make(map[string]bool)
		for _, item := range collResp.Msg.Items {
			collIDs[string(item.GetHttpVersionId())] = true
		}
		for _, syncID := range httpVersionInsertIDs {
			require.True(t, collIDs[string(syncID)],
				"HttpVersionSync INSERT event with ID %v not found in HttpVersionCollection", idFromBytes(syncID))
		}
		require.NotEmpty(t, httpVersionInsertIDs, "Expected at least 1 HttpVersionSync INSERT event")
	})

	t.Run("HttpResponseSync_vs_HttpResponseCollection", func(t *testing.T) {
		collResp, err := f.handler.HttpResponseCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)
		collIDs := make(map[string]bool)
		for _, item := range collResp.Msg.Items {
			collIDs[string(item.GetHttpResponseId())] = true
		}
		for _, syncID := range httpResponseInsertIDs {
			require.True(t, collIDs[string(syncID)],
				"HttpResponseSync INSERT event with ID %v not found in HttpResponseCollection", idFromBytes(syncID))
		}
		require.NotEmpty(t, httpResponseInsertIDs, "Expected at least 1 HttpResponseSync INSERT event")
	})

	t.Run("HttpResponseHeaderSync_vs_HttpResponseHeaderCollection", func(t *testing.T) {
		collResp, err := f.handler.HttpResponseHeaderCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)
		collIDs := make(map[string]bool)
		for _, item := range collResp.Msg.Items {
			collIDs[string(item.GetHttpResponseHeaderId())] = true
		}
		for _, syncID := range httpResponseHeaderInsertIDs {
			require.True(t, collIDs[string(syncID)],
				"HttpResponseHeaderSync INSERT event with ID %v not found in HttpResponseHeaderCollection", idFromBytes(syncID))
		}
		require.NotEmpty(t, httpResponseHeaderInsertIDs, "Expected at least 1 HttpResponseHeaderSync INSERT event")
	})

	// -- Log summary --
	t.Logf("Sync parity summary:")
	t.Logf("  HttpSync INSERTs: %d", len(httpSyncInsertIDs))
	t.Logf("  HttpHeaderSync INSERTs: %d", len(httpHeaderInsertIDs))
	t.Logf("  HttpSearchParamSync INSERTs: %d", len(httpParamInsertIDs))
	t.Logf("  HttpAssertSync INSERTs: %d", len(httpAssertInsertIDs))
	t.Logf("  HttpVersionSync INSERTs: %d", len(httpVersionInsertIDs))
	t.Logf("  HttpResponseSync INSERTs: %d", len(httpResponseInsertIDs))
	t.Logf("  HttpResponseHeaderSync INSERTs: %d", len(httpResponseHeaderInsertIDs))
}

// collectInsertIDs drains a sync channel and collects IDs from INSERT events.
func collectInsertIDs[T any](t *testing.T, ch <-chan T, timeout time.Duration, name string, extract func(T) ([]byte, bool)) [][]byte {
	t.Helper()

	var ids [][]byte
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case item, ok := <-ch:
			if !ok {
				return ids
			}
			if id, isInsert := extract(item); isInsert && len(id) > 0 {
				ids = append(ids, id)
			}
		case <-timer.C:
			return ids
		}
	}
}

// idFromBytes is a helper to format ID bytes for error messages.
func idFromBytes(b []byte) string {
	id, err := idwrap.NewFromBytes(b)
	if err != nil {
		return fmt.Sprintf("raw(%x)", b)
	}
	return id.String()
}

// TestHttpRun_SyncParityReverseCheck verifies that every item in the collection
// that was created by HttpRun has a corresponding sync INSERT event. This catches
// the case where collection returns data but no sync event was emitted.
func TestHttpRun_SyncParityReverseCheck(t *testing.T) {
	t.Parallel()

	testServer := createTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok":true}`)
	})
	defer testServer.Close()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "reverse-parity-workspace")
	httpID := f.createHttpWithUrl(t, ws, "reverse-parity", testServer.URL, "GET")
	f.createHttpHeader(t, httpID, "X-Key", "val")

	// -- Get collection BEFORE run to establish baseline --
	preHttpColl, err := f.handler.HttpCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	preHttpIDs := make(map[string]bool)
	for _, item := range preHttpColl.Msg.Items {
		preHttpIDs[string(item.GetHttpId())] = true
	}

	preVersionColl, err := f.handler.HttpVersionCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	preVersionIDs := make(map[string]bool)
	for _, item := range preVersionColl.Msg.Items {
		preVersionIDs[string(item.GetHttpVersionId())] = true
	}

	preResponseColl, err := f.handler.HttpResponseCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	preResponseIDs := make(map[string]bool)
	for _, item := range preResponseColl.Msg.Items {
		preResponseIDs[string(item.GetHttpResponseId())] = true
	}

	// -- Start sync streams --
	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	readyHttp := make(chan struct{})
	readyVersion := make(chan struct{})
	readyResponse := make(chan struct{})

	httpSyncIDs := make(chan []byte, 20)
	go func() {
		f.handler.streamHttpSyncWithOptions(ctx, f.userID, func(resp *apiv1.HttpSyncResponse) error {
			for _, item := range resp.GetItems() {
				v := item.GetValue()
				if v.GetKind() == apiv1.HttpSync_ValueUnion_KIND_INSERT {
					httpSyncIDs <- v.GetInsert().GetHttpId()
				}
			}
			return nil
		}, &eventstream.BulkOptions{Ready: readyHttp})
		close(httpSyncIDs)
	}()

	versionSyncIDs := make(chan []byte, 20)
	go func() {
		f.handler.streamHttpVersionSyncWithOptions(ctx, f.userID, func(resp *apiv1.HttpVersionSyncResponse) error {
			for _, item := range resp.GetItems() {
				v := item.GetValue()
				if v.GetKind() == apiv1.HttpVersionSync_ValueUnion_KIND_INSERT {
					versionSyncIDs <- v.GetInsert().GetHttpVersionId()
				}
			}
			return nil
		}, &eventstream.BulkOptions{Ready: readyVersion})
		close(versionSyncIDs)
	}()

	responseSyncIDs := make(chan []byte, 20)
	go func() {
		f.handler.streamHttpResponseSyncWithOptions(ctx, f.userID, func(resp *apiv1.HttpResponseSyncResponse) error {
			for _, item := range resp.GetItems() {
				v := item.GetValue()
				if v.GetKind() == apiv1.HttpResponseSync_ValueUnion_KIND_INSERT {
					responseSyncIDs <- v.GetInsert().GetHttpResponseId()
				}
			}
			return nil
		}, &eventstream.BulkOptions{Ready: readyResponse})
		close(responseSyncIDs)
	}()

	<-readyHttp
	<-readyVersion
	<-readyResponse

	// -- Execute HttpRun --
	_, err = f.handler.HttpRun(f.ctx, connect.NewRequest(&apiv1.HttpRunRequest{HttpId: httpID.Bytes()}))
	require.NoError(t, err)

	// Collect sync IDs
	collectTimeout := 3 * time.Second

	syncedHttpIDs := drainIDChannel(httpSyncIDs, collectTimeout)
	syncedVersionIDs := drainIDChannel(versionSyncIDs, collectTimeout)
	syncedResponseIDs := drainIDChannel(responseSyncIDs, collectTimeout)

	cancel()

	// -- Get collection AFTER run --
	postHttpColl, err := f.handler.HttpCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)

	postVersionColl, err := f.handler.HttpVersionCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)

	postResponseColl, err := f.handler.HttpResponseCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)

	// -- Reverse check: new collection items must have sync events --
	t.Run("NewHttpEntries_HaveSyncEvents", func(t *testing.T) {
		syncSet := byteSliceSet(syncedHttpIDs)
		for _, item := range postHttpColl.Msg.Items {
			id := item.GetHttpId()
			if preHttpIDs[string(id)] {
				continue // existed before run
			}
			require.True(t, syncSet[string(id)],
				"New HTTP entry %v in collection has no matching HttpSync INSERT event", idFromBytes(id))
		}
	})

	t.Run("NewVersions_HaveSyncEvents", func(t *testing.T) {
		syncSet := byteSliceSet(syncedVersionIDs)
		for _, item := range postVersionColl.Msg.Items {
			id := item.GetHttpVersionId()
			if preVersionIDs[string(id)] {
				continue
			}
			require.True(t, syncSet[string(id)],
				"New version %v in collection has no matching HttpVersionSync INSERT event", idFromBytes(id))
		}
	})

	t.Run("NewResponses_HaveSyncEvents", func(t *testing.T) {
		syncSet := byteSliceSet(syncedResponseIDs)
		for _, item := range postResponseColl.Msg.Items {
			id := item.GetHttpResponseId()
			if preResponseIDs[string(id)] {
				continue
			}
			require.True(t, syncSet[string(id)],
				"New response %v in collection has no matching HttpResponseSync INSERT event", idFromBytes(id))
		}
	})
}

// TestHttpRun_SyncEventFieldsMatchCollection verifies that the actual field
// values in sync INSERT events match the corresponding collection items. This
// catches serialization bugs where the ID matches but other fields differ.
func TestHttpRun_SyncEventFieldsMatchCollection(t *testing.T) {
	t.Parallel()

	testServer := createTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"field":"value"}`)
	})
	defer testServer.Close()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "field-match-workspace")
	httpID := f.createHttpWithUrl(t, ws, "field-match-test", testServer.URL, "PUT")
	f.createHttpHeader(t, httpID, "X-Field", "field-val")
	f.createHttpSearchParam(t, httpID, "key", "val")

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	readyVersion := make(chan struct{})
	readyResponse := make(chan struct{})

	// Collect version sync inserts with full data
	var versionInserts []*apiv1.HttpVersionSyncInsert
	versionSyncCh := make(chan *apiv1.HttpVersionSyncInsert, 10)
	go func() {
		f.handler.streamHttpVersionSyncWithOptions(ctx, f.userID, func(resp *apiv1.HttpVersionSyncResponse) error {
			for _, item := range resp.GetItems() {
				v := item.GetValue()
				if v.GetKind() == apiv1.HttpVersionSync_ValueUnion_KIND_INSERT {
					versionSyncCh <- v.GetInsert()
				}
			}
			return nil
		}, &eventstream.BulkOptions{Ready: readyVersion})
		close(versionSyncCh)
	}()

	// Collect response sync inserts with full data
	var responseInserts []*apiv1.HttpResponseSyncInsert
	responseSyncCh := make(chan *apiv1.HttpResponseSyncInsert, 10)
	go func() {
		f.handler.streamHttpResponseSyncWithOptions(ctx, f.userID, func(resp *apiv1.HttpResponseSyncResponse) error {
			for _, item := range resp.GetItems() {
				v := item.GetValue()
				if v.GetKind() == apiv1.HttpResponseSync_ValueUnion_KIND_INSERT {
					responseSyncCh <- v.GetInsert()
				}
			}
			return nil
		}, &eventstream.BulkOptions{Ready: readyResponse})
		close(responseSyncCh)
	}()

	<-readyVersion
	<-readyResponse

	// Execute
	_, err := f.handler.HttpRun(f.ctx, connect.NewRequest(&apiv1.HttpRunRequest{HttpId: httpID.Bytes()}))
	require.NoError(t, err)

	// Drain channels
	timeout := 3 * time.Second
	timer := time.NewTimer(timeout)
	for {
		select {
		case item, ok := <-versionSyncCh:
			if !ok {
				goto doneVersions
			}
			versionInserts = append(versionInserts, item)
		case <-timer.C:
			goto doneVersions
		}
	}
doneVersions:
	timer.Reset(timeout)
	for {
		select {
		case item, ok := <-responseSyncCh:
			if !ok {
				goto doneResponses
			}
			responseInserts = append(responseInserts, item)
		case <-timer.C:
			goto doneResponses
		}
	}
doneResponses:
	cancel()

	// Compare version fields
	t.Run("VersionFieldMatch", func(t *testing.T) {
		require.NotEmpty(t, versionInserts, "Expected version sync inserts")

		collResp, err := f.handler.HttpVersionCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)

		for _, syncVer := range versionInserts {
			found := false
			for _, collVer := range collResp.Msg.Items {
				if bytes.Equal(syncVer.GetHttpVersionId(), collVer.GetHttpVersionId()) {
					found = true
					require.True(t, bytes.Equal(syncVer.GetHttpId(), collVer.GetHttpId()),
						"Version HttpId mismatch: sync=%v, collection=%v",
						idFromBytes(syncVer.GetHttpId()), idFromBytes(collVer.GetHttpId()))
					require.Equal(t, syncVer.GetName(), collVer.GetName(),
						"Version Name mismatch")
					break
				}
			}
			require.True(t, found, "Version %v from sync not found in collection", idFromBytes(syncVer.GetHttpVersionId()))
		}
	})

	// Compare response fields
	t.Run("ResponseFieldMatch", func(t *testing.T) {
		require.NotEmpty(t, responseInserts, "Expected response sync inserts")

		collResp, err := f.handler.HttpResponseCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)

		for _, syncResp := range responseInserts {
			found := false
			for _, collRespItem := range collResp.Msg.Items {
				if bytes.Equal(syncResp.GetHttpResponseId(), collRespItem.GetHttpResponseId()) {
					found = true
					require.Equal(t, syncResp.GetStatus(), collRespItem.GetStatus(),
						"Response Status mismatch for %v", idFromBytes(syncResp.GetHttpResponseId()))
					require.True(t, bytes.Equal(syncResp.GetHttpId(), collRespItem.GetHttpId()),
						"Response HttpId mismatch: sync=%v, collection=%v",
						idFromBytes(syncResp.GetHttpId()), idFromBytes(collRespItem.GetHttpId()))
					break
				}
			}
			require.True(t, found, "Response %v from sync not found in collection", idFromBytes(syncResp.GetHttpResponseId()))
		}
	})
}

// drainIDChannel collects all ID byte slices from a channel until timeout.
func drainIDChannel(ch <-chan []byte, timeout time.Duration) [][]byte {
	var ids [][]byte
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case id, ok := <-ch:
			if !ok {
				return ids
			}
			if len(id) > 0 {
				ids = append(ids, id)
			}
		case <-timer.C:
			return ids
		}
	}
}

// byteSliceSet converts a slice of byte slices to a set for O(1) lookup.
func byteSliceSet(items [][]byte) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[string(item)] = true
	}
	return set
}
