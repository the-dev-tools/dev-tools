package rimport_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/testutil"
	"the-dev-tools/server/pkg/translate/tcurl"
)

func TestCurlHeaderConversionAndAppend(t *testing.T) {
	// Setup test context and database
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries

	// Initialize header service
	hs := sexampleheader.New(queries)

	// Test curl command with multiple headers
	curlStr := `curl 'https://api.example.com/test' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer token123' \
  -H 'Content-Type: application/json' \
  -H 'User-Agent: DevTools/1.0' \
  --data-raw '{"test":"data"}'`

	// Convert curl to resolved structure
	collectionID := idwrap.NewNow()
	resolved, err := tcurl.ConvertCurl(curlStr, collectionID)
	require.NoError(t, err, "Curl conversion should succeed")
	
	// Verify headers were extracted
	require.Len(t, resolved.Headers, 4, "Should have 4 headers")
	t.Logf("Headers from curl conversion:")
	for i, h := range resolved.Headers {
		t.Logf("  [%d] %s: %s (prev: %v, next: %v)", i, h.HeaderKey, h.Value, h.Prev, h.Next)
	}

	// Test that AppendBulkHeader works with headers that have nil Prev/Next
	err = hs.AppendBulkHeader(ctx, resolved.Headers)
	require.NoError(t, err, "AppendBulkHeader should work with headers from curl conversion")

	// Verify headers were created with proper linking
	exampleID := resolved.Headers[0].ExampleID
	headers, err := hs.GetHeaderByExampleIDOrdered(ctx, exampleID)
	require.NoError(t, err, "Should be able to get ordered headers")
	require.Len(t, headers, 4, "Should have 4 headers in the database")
	
	t.Logf("Headers from database (ordered):")
	for i, h := range headers {
		t.Logf("  [%d] %s: %s (prev: %v, next: %v)", i, h.HeaderKey, h.Value, h.Prev, h.Next)
	}

	// Verify linked-list structure
	hasHead := false
	hasTail := false
	for _, h := range headers {
		if h.Prev == nil {
			hasHead = true
		}
		if h.Next == nil {
			hasTail = true
		}
	}
	
	assert.True(t, hasHead, "Should have one header with Prev = nil (head)")
	assert.True(t, hasTail, "Should have one header with Next = nil (tail)")
	
	// Verify all expected headers are present
	headerKeys := make(map[string]string)
	for _, h := range headers {
		headerKeys[h.HeaderKey] = h.Value
	}
	
	expectedHeaders := map[string]string{
		"Accept":        "application/json",
		"Authorization": "Bearer token123",
		"Content-Type":  "application/json",
		"User-Agent":    "DevTools/1.0",
	}
	
	for expectedKey, expectedValue := range expectedHeaders {
		actualValue, exists := headerKeys[expectedKey]
		assert.True(t, exists, "Header %s should exist", expectedKey)
		assert.Equal(t, expectedValue, actualValue, "Header %s should have correct value", expectedKey)
	}
}