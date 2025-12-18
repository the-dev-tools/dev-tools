package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"the-dev-tools/server/pkg/idwrap"
)

func TestCharsetConversion(t *testing.T) {
	// ISO-8859-1 encoded string "café" (cafe with acute accent)
	// 'é' in ISO-8859-1 is byte 0xE9 (233)
	iso88591Data := []byte{'c', 'a', 'f', 0xE9}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=ISO-8859-1")
		w.WriteHeader(http.StatusOK)
		w.Write(iso88591Data)
	}))
	defer ts.Close()

	client := New()
	req := &Request{
		Method: "GET",
		URL:    ts.URL,
	}

	resp, err := SendRequestAndConvertWithContext(context.Background(), client, req, idwrap.IDWrap{})
	if err != nil {
		t.Fatalf("SendRequestAndConvertWithContext failed: %v", err)
	}

	expected := "café" // UTF-8 encoded in Go source code
	if string(resp.Body) != expected {
		t.Errorf("Expected body %q, got %q (bytes: %v)", expected, string(resp.Body), resp.Body)
	}

	// Also verify that invalid UTF-8 handling in rhttp (from previous fix) doesn't mask this
	// if we fix it here, rhttp's fallback shouldn't trigger.
}
