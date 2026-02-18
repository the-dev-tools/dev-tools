package rimportv2

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

func TestNewURLFetcher_HasTimeout(t *testing.T) {
	fetcher := NewURLFetcher()
	if fetcher.client.Timeout != 60*time.Second {
		t.Errorf("expected HTTP client timeout 60s, got %v", fetcher.client.Timeout)
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		private bool
	}{
		{"loopback IPv4", "127.0.0.1", true},
		{"loopback IPv6", "::1", true},
		{"RFC1918 10.x", "10.0.0.1", true},
		{"RFC1918 172.16.x", "172.16.0.1", true},
		{"RFC1918 192.168.x", "192.168.1.1", true},
		{"link-local unicast", "169.254.169.254", true},
		{"link-local multicast", "224.0.0.1", true},
		{"unspecified IPv4", "0.0.0.0", true},
		{"unspecified IPv6", "::", true},
		{"IPv6 link-local", "fe80::1", true},
		{"public IP", "8.8.8.8", false},
		{"public IP 2", "1.1.1.1", false},
		{"public IPv6", "2607:f8b0:4004:800::200e", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP %q", tt.ip)
			}
			if got := isPrivateIP(ip); got != tt.private {
				t.Errorf("isPrivateIP(%s) = %v, want %v", tt.ip, got, tt.private)
			}
		})
	}
}

func TestNewURLFetcher_HasSSRFProtection(t *testing.T) {
	fetcher := NewURLFetcher()
	transport, ok := fetcher.client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport on URL fetcher client")
	}
	if transport.DialContext == nil {
		t.Fatal("expected custom DialContext for SSRF protection, got nil")
	}
}

func TestImportUnifiedWithTextData_DoesNotConvertTextToData(t *testing.T) {
	// Bug 1 regression test: ImportUnifiedWithTextData must NOT convert
	// TextData to Data, because that would bypass the URL detection in
	// resolveInputData. Instead, it should pass the request through as-is
	// so resolveInputData can handle URLs, raw text, etc.

	urlText := "https://example.com/api/spec.json"
	fetchedContent := []byte(`{"openapi":"3.0.0","info":{"title":"Test"},"paths":{}}`)

	mockFetcher := &mockURLFetcher{
		fetchFunc: func(ctx context.Context, rawURL string) ([]byte, error) {
			if rawURL != urlText {
				t.Errorf("expected fetch URL %q, got %q", urlText, rawURL)
			}
			return fetchedContent, nil
		},
	}

	service := NewService(&MockImporter{}, &MockValidator{
		validateFunc: func(ctx context.Context, req *ImportRequest) error {
			return nil
		},
		validateWorkspaceFunc: func(ctx context.Context, workspaceID idwrap.IDWrap) error {
			return nil
		},
	}, WithURLFetcher(mockFetcher))

	req := &ImportRequest{
		WorkspaceID: idwrap.NewNow(),
		Name:        "URL import test",
		TextData:    urlText,
	}

	// Before the fix, TextData was converted to Data ([]byte of the URL string),
	// which made resolveInputData skip URL fetching. After the fix,
	// resolveInputData sees Data is empty and TextData is a URL, so it fetches it.
	_, err := service.ImportUnifiedWithTextData(context.Background(), req)
	// We don't care about the full import result (it needs real services to store),
	// but the fetcher MUST have been called.
	_ = err

	if !mockFetcher.called {
		t.Fatal("URL fetcher was not called — TextData URL was not resolved by resolveInputData")
	}
}

type mockURLFetcher struct {
	fetchFunc func(ctx context.Context, rawURL string) ([]byte, error)
	called    bool
}

func (m *mockURLFetcher) Fetch(ctx context.Context, rawURL string) ([]byte, error) {
	m.called = true
	return m.fetchFunc(ctx, rawURL)
}
