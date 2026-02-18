package rimportv2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// maxFetchSize is the maximum size of data fetched from a URL (50MB).
const maxFetchSize = 50 * 1024 * 1024

// ErrPrivateIP is returned when a URL resolves to a private or internal IP address.
var ErrPrivateIP = errors.New("URL resolves to a private or internal IP address")

// URLFetcher fetches content from URLs. It's an interface to allow testing.
type URLFetcher interface {
	Fetch(ctx context.Context, rawURL string) ([]byte, error)
}

// DefaultURLFetcher implements URLFetcher using net/http.
type DefaultURLFetcher struct {
	client *http.Client
}

// NewURLFetcher creates a new DefaultURLFetcher with SSRF protection.
// The underlying transport resolves DNS and rejects private/internal IPs
// before establishing a TCP connection, protecting against both direct
// and redirect-based SSRF attacks.
func NewURLFetcher() *DefaultURLFetcher {
	return &DefaultURLFetcher{
		client: &http.Client{
			Timeout:   60 * time.Second,
			Transport: safeTransport(),
		},
	}
}

// isPrivateIP reports whether ip is a loopback, private, link-local,
// or otherwise non-routable address.
func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified()
}

// safeTransport returns an *http.Transport whose DialContext resolves
// the target host and rejects connections to private IP ranges.
func safeTransport() *http.Transport {
	dialer := &net.Dialer{Timeout: 10 * time.Second}

	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}

			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}

			for _, ip := range ips {
				if isPrivateIP(ip.IP) {
					return nil, fmt.Errorf("%w: %s resolves to %s", ErrPrivateIP, host, ip.IP)
				}
			}

			return dialer.DialContext(ctx, network, net.JoinHostPort(host, port))
		},
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
