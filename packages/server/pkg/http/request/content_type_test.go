package request

import (
	"testing"

	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/model/mhttp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectContentType(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "empty data",
			input:    []byte{},
			expected: "",
		},
		{
			name:     "whitespace only",
			input:    []byte("   \t\n\r  "),
			expected: "text/plain",
		},
		{
			name:     "JSON object",
			input:    []byte(`{"name": "test", "value": 123}`),
			expected: "application/json",
		},
		{
			name:     "JSON array",
			input:    []byte(`[1, 2, 3, "hello"]`),
			expected: "application/json",
		},
		{
			name:     "JSON with leading whitespace",
			input:    []byte("  \n\t{\"hello\": \"world\"}"),
			expected: "application/json",
		},
		{
			name:     "invalid JSON starting with brace",
			input:    []byte(`{not valid json`),
			expected: "text/plain", // Falls back to text/plain since it's valid UTF-8
		},
		{
			name:     "XML declaration",
			input:    []byte(`<?xml version="1.0" encoding="UTF-8"?><root></root>`),
			expected: "application/xml",
		},
		{
			name:     "HTML doctype",
			input:    []byte(`<!DOCTYPE html><html><body></body></html>`),
			expected: "text/html",
		},
		{
			name:     "HTML tag",
			input:    []byte(`<html><head></head><body></body></html>`),
			expected: "text/html",
		},
		{
			name:     "Generic XML",
			input:    []byte(`<root><child>value</child></root>`),
			expected: "application/xml",
		},
		{
			name:     "plain text",
			input:    []byte("Hello, this is plain text"),
			expected: "text/plain",
		},
		{
			name:     "binary data",
			input:    []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0x80},
			expected: "application/octet-stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectContentType(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasContentTypeHeader(t *testing.T) {
	tests := []struct {
		name     string
		headers  []httpclient.Header
		expected bool
	}{
		{
			name:     "empty headers",
			headers:  []httpclient.Header{},
			expected: false,
		},
		{
			name: "has Content-Type",
			headers: []httpclient.Header{
				{HeaderKey: "Authorization", Value: "Bearer token"},
				{HeaderKey: "Content-Type", Value: "application/json"},
			},
			expected: true,
		},
		{
			name: "has content-type lowercase",
			headers: []httpclient.Header{
				{HeaderKey: "content-type", Value: "text/plain"},
			},
			expected: true,
		},
		{
			name: "no Content-Type",
			headers: []httpclient.Header{
				{HeaderKey: "Authorization", Value: "Bearer token"},
				{HeaderKey: "Accept", Value: "application/json"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasContentTypeHeader(tt.headers)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPrepareHTTPRequestWithTracking_AutoDetectContentType(t *testing.T) {
	t.Run("auto-detects JSON content type", func(t *testing.T) {
		httpReq := newTestHTTPForContentType()
		rawBody := newTestRawBodyForContentType([]byte(`{"message": "hello"}`))
		headers := []mhttp.HTTPHeader{} // No Content-Type header

		result, err := PrepareHTTPRequestWithTracking(httpReq, headers, nil, &rawBody, nil, nil, nil)
		require.NoError(t, err)

		// Should have auto-detected Content-Type
		var foundContentType bool
		var contentTypeValue string
		for _, h := range result.Request.Headers {
			if h.HeaderKey == "Content-Type" {
				foundContentType = true
				contentTypeValue = h.Value
				break
			}
		}
		assert.True(t, foundContentType, "Content-Type header should be auto-added")
		assert.Equal(t, "application/json", contentTypeValue)
	})

	t.Run("respects existing Content-Type header", func(t *testing.T) {
		httpReq := newTestHTTPForContentType()
		rawBody := newTestRawBodyForContentType([]byte(`{"message": "hello"}`))
		headers := []mhttp.HTTPHeader{
			{Key: "Content-Type", Value: "text/plain", Enabled: true},
		}

		result, err := PrepareHTTPRequestWithTracking(httpReq, headers, nil, &rawBody, nil, nil, nil)
		require.NoError(t, err)

		// Should use the existing header, not override
		var contentTypeValue string
		for _, h := range result.Request.Headers {
			if h.HeaderKey == "Content-Type" {
				contentTypeValue = h.Value
				break
			}
		}
		assert.Equal(t, "text/plain", contentTypeValue)
	})

	t.Run("auto-detects XML content type", func(t *testing.T) {
		httpReq := newTestHTTPForContentType()
		rawBody := newTestRawBodyForContentType([]byte(`<?xml version="1.0"?><root/>`))
		headers := []mhttp.HTTPHeader{}

		result, err := PrepareHTTPRequestWithTracking(httpReq, headers, nil, &rawBody, nil, nil, nil)
		require.NoError(t, err)

		var contentTypeValue string
		for _, h := range result.Request.Headers {
			if h.HeaderKey == "Content-Type" {
				contentTypeValue = h.Value
				break
			}
		}
		assert.Equal(t, "application/xml", contentTypeValue)
	})
}

// Helper functions for tests
func newTestHTTPForContentType() mhttp.HTTP {
	return mhttp.HTTP{
		Url:      "https://example.com/api",
		Method:   "POST",
		BodyKind: mhttp.HttpBodyKindRaw,
	}
}

func newTestRawBodyForContentType(data []byte) mhttp.HTTPBodyRaw {
	return mhttp.HTTPBodyRaw{
		RawData: data,
	}
}
