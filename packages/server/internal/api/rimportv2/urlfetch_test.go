package rimportv2

import (
	"context"
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
