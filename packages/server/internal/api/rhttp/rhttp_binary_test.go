package rhttp

import (
	"compress/gzip"
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
	
	// Case 1: Invalid UTF-8 binary data
	tsBinary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte{0xFF, 0xFE, 0xFD})
	}))
	defer tsBinary.Close()

	httpIDBinary := f.createHttp(t, wsID, "binary-http", tsBinary.URL, "GET")
	runReqBinary := connect.NewRequest(&httpv1.HttpRunRequest{HttpId: httpIDBinary.Bytes()})
	if _, err := f.handler.HttpRun(f.ctx, runReqBinary); err != nil {
		t.Fatalf("HttpRun binary err: %v", err)
	}

	// Case 2: Gzip compressed text with Content-Encoding header
	// Go client might transparently decompress if we don't set Accept-Encoding, 
	// so we explicitly set Content-Encoding on server.
	// To ensure our manual decompression logic is triggered (if Go client doesn't), 
	// we rely on the fact that if Go client DOES decompress, it removes the header.
	// If Go client DOES NOT, it keeps header.
	// BUT we want to test OUR manual decompression logic.
	// To force Go client NOT to decompress, we can probably set a custom Accept-Encoding in the request?
	// But we are calling HttpRun -> executeHTTPRequest -> httpclient.
	// We can't easily modify request headers in this test fixture helper `createHttp`.
	// However, if the server sends `Content-Encoding: gzip` but NO `Content-Type` or `Vary`, maybe?
	
	// Actually, if we use "GZIP" (uppercase), Go's default transport might miss it if it's case sensitive?
	// Go's transport implementation of gzip is quite robust.
	
	// Let's just test that "it works" regardless of WHO decompressed it.
	// The user's issue was "invalid UTF-8". 
	// If we send compressed data and it ends up as "Hello World" string, we are good.
	
	tsGzip := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "GZIP") // Uppercase to test case insensitivity fix
		w.WriteHeader(http.StatusOK)
		gw := gzip.NewWriter(w)
		gw.Write([]byte("Hello Compressed World"))
		gw.Close()
	}))
	defer tsGzip.Close()

	httpIDGzip := f.createHttp(t, wsID, "gzip-http", tsGzip.URL, "GET")
	runReqGzip := connect.NewRequest(&httpv1.HttpRunRequest{HttpId: httpIDGzip.Bytes()})
	if _, err := f.handler.HttpRun(f.ctx, runReqGzip); err != nil {
		t.Fatalf("HttpRun gzip err: %v", err)
	}

	// Call HttpResponseCollection to verify
	collReq := connect.NewRequest(&emptypb.Empty{})
	resp, err := f.handler.HttpResponseCollection(f.ctx, collReq)
	if err != nil {
		t.Fatalf("HttpResponseCollection err: %v", err)
	}
	
	// Verify results
	foundBinary := false
	foundGzip := false
	
	for _, item := range resp.Msg.Items {
		id, _ := idwrap.NewFromBytes(item.HttpId)
		if id == httpIDBinary {
			foundBinary = true
			if item.Body != "[Binary data: 3 bytes]" {
				t.Errorf("Expected binary body placeholder, got '%s'", item.Body)
			}
		} else if id == httpIDGzip {
			foundGzip = true
			if item.Body != "Hello Compressed World" {
				t.Errorf("Expected decompressed body 'Hello Compressed World', got '%s'", item.Body)
			}
		}
	}
	
	if !foundBinary {
		t.Error("Binary response not found")
	}
	if !foundGzip {
		t.Error("Gzip response not found")
	}
}
