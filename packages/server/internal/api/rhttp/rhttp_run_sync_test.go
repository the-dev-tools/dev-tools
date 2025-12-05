package rhttp

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	httpv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

func TestHttpRunPublishesResponseSyncEvent(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	wsID := f.createWorkspace(t, "run-workspace")

	// Create a test server to receive the request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer ts.Close()

	httpID := f.createHttp(t, wsID, "run-http", ts.URL, "GET")

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *httpv1.HttpResponseSyncResponse, 10)
	errCh := make(chan error, 1)

	// Start streaming HttpResponseSync
	go func() {
		err := f.handler.streamHttpResponseSync(ctx, f.userID, func(resp *httpv1.HttpResponseSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	// Wait a bit for stream to subscribe (though memory stream is synchronous, the goroutine might take time)
	time.Sleep(100 * time.Millisecond)

	// Execute HttpRun
	runReq := connect.NewRequest(&httpv1.HttpRunRequest{
		HttpId: httpID.Bytes(),
	})
	_, err := f.handler.HttpRun(f.ctx, runReq)
	require.NoError(t, err, "HttpRun err")

	// Collect events
	var items []*httpv1.HttpResponseSync
	timeout := time.After(2 * time.Second)

	select {
	case resp, ok := <-msgCh:
		require.True(t, ok, "channel closed prematurely")
		for _, item := range resp.GetItems() {
			if item != nil {
				items = append(items, item)
			}
		}
	case <-timeout:
		require.FailNow(t, "timeout waiting for response sync event")
	}

	require.NotEmpty(t, items, "no response sync events received")

	val := items[0].GetValue()
	require.NotNil(t, val, "response sync item missing value union")

	require.Equal(t, httpv1.HttpResponseSync_ValueUnion_KIND_INSERT, val.GetKind(), "expected insert kind")

	insert := val.GetInsert()
	require.NotNil(t, insert, "expected insert value, got nil")

	// Check if HttpId is populated (this is what fails currently)
	// If GetHttpId() returns empty bytes, it means it's not populated (or not set)
	require.NotEmpty(t, insert.GetHttpId(), "expected HttpId to be populated")
}

func TestHttpRunDeltaPublishesResponseSyncEvent(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	wsID := f.createWorkspace(t, "run-delta-workspace")

	// Create a test server to receive the request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer ts.Close()

	// Create base HTTP request
	baseID := f.createHttp(t, wsID, "base-http", ts.URL, "GET")

	// Create delta HTTP request
	deltaHTTP := mhttp.HTTP{
		ID:           idwrap.NewNow(),
		WorkspaceID:  wsID,
		Name:         "delta-http",
		Url:          ts.URL,
		Method:       "POST",
		BodyKind:     mhttp.HttpBodyKindNone,
		IsDelta:      true,
		ParentHttpID: &baseID,
	}
	require.NoError(t, f.hs.Create(f.ctx, &deltaHTTP), "create delta http")

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *httpv1.HttpResponseSyncResponse, 10)
	errCh := make(chan error, 1)

	// Start streaming HttpResponseSync
	go func() {
		err := f.handler.streamHttpResponseSync(ctx, f.userID, func(resp *httpv1.HttpResponseSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	// Wait a bit for stream to subscribe
	time.Sleep(100 * time.Millisecond)

	// Execute HttpRun with the delta request
	runReq := connect.NewRequest(&httpv1.HttpRunRequest{
		HttpId: deltaHTTP.ID.Bytes(),
	})
	_, err := f.handler.HttpRun(f.ctx, runReq)
	require.NoError(t, err, "HttpRun err")

	// Collect events
	var items []*httpv1.HttpResponseSync
	timeout := time.After(2 * time.Second)

	select {
	case resp, ok := <-msgCh:
		require.True(t, ok, "channel closed prematurely")
		for _, item := range resp.GetItems() {
			if item != nil {
				items = append(items, item)
			}
		}
	case <-timeout:
		require.FailNow(t, "timeout waiting for response sync event for delta request")
	}

	require.NotEmpty(t, items, "no response sync events received for delta request")

	val := items[0].GetValue()
	require.NotNil(t, val, "response sync item missing value union")

	require.Equal(t, httpv1.HttpResponseSync_ValueUnion_KIND_INSERT, val.GetKind(), "expected insert kind")

	insert := val.GetInsert()
	require.NotNil(t, insert, "expected insert value, got nil")

	// Check if HttpId is the delta ID (not base ID)
	require.NotEmpty(t, insert.GetHttpId(), "expected HttpId to be populated")

	// Verify the HttpId matches the delta request ID
	require.True(t, bytes.Equal(insert.GetHttpId(), deltaHTTP.ID.Bytes()), "expected HttpId to match delta ID %v, got %v", deltaHTTP.ID.Bytes(), insert.GetHttpId())

	t.Logf("Delta request response sync works correctly - HttpId: %v", insert.GetHttpId())
}

func TestHttpResponseCollectionIncludesDeltaResponses(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	wsID := f.createWorkspace(t, "collection-workspace")

	// Create a test server to receive the request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer ts.Close()

	// Create base HTTP request
	baseID := f.createHttp(t, wsID, "base-http", ts.URL, "GET")

	// Create delta HTTP request
	deltaHTTP := mhttp.HTTP{
		ID:           idwrap.NewNow(),
		WorkspaceID:  wsID,
		Name:         "delta-http",
		Url:          ts.URL,
		Method:       "POST",
		BodyKind:     mhttp.HttpBodyKindNone,
		IsDelta:      true,
		ParentHttpID: &baseID,
	}
	require.NoError(t, f.hs.Create(f.ctx, &deltaHTTP), "create delta http")

	// Execute both base and delta requests to generate responses
	baseRunReq := connect.NewRequest(&httpv1.HttpRunRequest{
		HttpId: baseID.Bytes(),
	})
	_, err := f.handler.HttpRun(f.ctx, baseRunReq)
	require.NoError(t, err, "HttpRun (base) err")

	deltaRunReq := connect.NewRequest(&httpv1.HttpRunRequest{
		HttpId: deltaHTTP.ID.Bytes(),
	})
	_, err = f.handler.HttpRun(f.ctx, deltaRunReq)
	require.NoError(t, err, "HttpRun (delta) err")

	// Call HttpResponseCollection and verify both responses are returned
	collectionResp, err := f.handler.HttpResponseCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err, "HttpResponseCollection err")

	responses := collectionResp.Msg.GetItems()
	require.GreaterOrEqual(t, len(responses), 2, "expected at least 2 responses (base + delta)")

	// Verify we have responses for both base and delta HTTP IDs
	foundBase := false
	foundDelta := false
	for _, resp := range responses {
		httpID, err := idwrap.NewFromBytes(resp.GetHttpId())
		if err != nil {
			continue
		}
		if httpID == baseID {
			foundBase = true
		}
		if httpID == deltaHTTP.ID {
			foundDelta = true
		}
	}

	require.True(t, foundBase, "HttpResponseCollection missing response for base HTTP request")
	require.True(t, foundDelta, "HttpResponseCollection missing response for delta HTTP request")

	t.Logf("HttpResponseCollection correctly includes %d responses (base: %v, delta: %v)", len(responses), foundBase, foundDelta)
}
