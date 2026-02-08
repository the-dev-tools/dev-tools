package rimportv2

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// maxFetchSize is the maximum size of data fetched from a URL (50MB).
const maxFetchSize = 50 * 1024 * 1024

// URLFetcher fetches content from URLs. It's an interface to allow testing.
type URLFetcher interface {
	Fetch(ctx context.Context, rawURL string) ([]byte, error)
}

// DefaultURLFetcher implements URLFetcher using net/http.
type DefaultURLFetcher struct {
	client *http.Client
}

// NewURLFetcher creates a new DefaultURLFetcher.
func NewURLFetcher() *DefaultURLFetcher {
	return &DefaultURLFetcher{
		client: &http.Client{},
	}
}

// Fetch downloads content from the given URL.
func (f *DefaultURLFetcher) Fetch(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set Accept header to prefer JSON/YAML
	req.Header.Set("Accept", "application/json, application/yaml, text/yaml, */*")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("URL returned status %d", resp.StatusCode)
	}

	// Limit the response size
	limitedReader := io.LimitReader(resp.Body, maxFetchSize+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if len(data) > maxFetchSize {
		return nil, fmt.Errorf("response exceeds maximum size of %d bytes", maxFetchSize)
	}

	return data, nil
}

// IsURL checks if a string looks like a fetchable HTTP(S) URL.
// Returns false for curl commands and other non-URL text.
func IsURL(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}

	// Must start with http:// or https://
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return false
	}

	// Should not contain spaces (URLs don't have spaces, curl commands do)
	if strings.ContainsAny(s, " \t\n") {
		return false
	}

	// Must be a valid URL
	u, err := url.Parse(s)
	if err != nil {
		return false
	}

	// Must have a host
	return u.Host != ""
}
