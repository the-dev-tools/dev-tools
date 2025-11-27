package rhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"

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
	if _, err := f.handler.HttpRun(f.ctx, runReq); err != nil {
		t.Fatalf("HttpRun err: %v", err)
	}

	// Collect events
	var items []*httpv1.HttpResponseSync
	timeout := time.After(2 * time.Second)

	select {
	case resp, ok := <-msgCh:
		if !ok {
			t.Fatal("channel closed prematurely")
		}
		for _, item := range resp.GetItems() {
			if item != nil {
				items = append(items, item)
			}
		}
	case <-timeout:
		t.Fatal("timeout waiting for response sync event")
	}

	if len(items) == 0 {
		t.Fatal("no response sync events received")
	}

	val := items[0].GetValue()
	if val == nil {
		t.Fatal("response sync item missing value union")
	}

	if val.GetKind() != httpv1.HttpResponseSync_ValueUnion_KIND_INSERT {
		t.Fatalf("expected insert kind, got %v", val.GetKind())
	}

	insert := val.GetInsert()
	if insert == nil {
		t.Fatal("expected insert value, got nil")
	}

	// Check if HttpId is populated (this is what fails currently)
	// If GetHttpId() returns empty bytes, it means it's not populated (or not set)
	if len(insert.GetHttpId()) == 0 {
		t.Fatalf("expected HttpId to be populated, got empty bytes")
	}
}
