package rhttp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
	"the-dev-tools/server/pkg/idwrap"
	httpv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

func TestHttpRunHandlesBinaryResponse(t *testing.T) {
	t.Parallel()

	f := newHttpStreamingFixture(t)
	wsID := f.createWorkspace(t, "binary-workspace")
	
	// Create a test server that returns invalid UTF-8 data
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Write invalid UTF-8 bytes (0xFF is invalid in UTF-8)
		w.Write([]byte{0xFF, 0xFE, 0xFD})
	}))
	defer ts.Close()

	httpID := f.createHttp(t, wsID, "binary-http", ts.URL, "GET")

	// Execute HttpRun
	runReq := connect.NewRequest(&httpv1.HttpRunRequest{
		HttpId: httpID.Bytes(),
	})
	if _, err := f.handler.HttpRun(f.ctx, runReq); err != nil {
		t.Fatalf("HttpRun err: %v", err)
	}

	// Call HttpResponseCollection to verify marshaling
	collReq := connect.NewRequest(&emptypb.Empty{})
	resp, err := f.handler.HttpResponseCollection(f.ctx, collReq)
	if err != nil {
		t.Fatalf("HttpResponseCollection err: %v", err)
	}
	
	// Verify the body is placeholder
	found := false
	for _, item := range resp.Msg.Items {
		// We need to find the one we just created. 
		// Since ID is random, we check if HttpId matches
		id, _ := idwrap.NewFromBytes(item.HttpId)
		if id == httpID {
			found = true
			if item.Body == "" {
				t.Error("Expected body to not be empty")
			}
			// Check if it contains "Binary data"
			// The exact string depends on implementation
			if item.Body != "[Binary data: 3 bytes]" {
				t.Errorf("Expected body '[Binary data: 3 bytes]', got '%s'", item.Body)
			}
		}
	}
	
	if !found {
		t.Error("Created response not found in collection")
	}
}
