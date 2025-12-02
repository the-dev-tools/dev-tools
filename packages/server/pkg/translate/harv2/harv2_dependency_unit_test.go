package harv2_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"the-dev-tools/server/pkg/depfinder"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/translate/harv2"
)

func TestHARv2_DependencyChain_Unit(t *testing.T) {
	// 1. Setup HAR with dependency chain
	// Request A: Returns {"token": "abc-123-xyz-token"}
	// Request B: Uses "Bearer abc-123-xyz-token" in header
	// Note: Token must be >= 8 chars for substring matching to avoid false positives

	har := &harv2.HAR{
		Log: harv2.Log{
			Entries: []harv2.Entry{
				{
					StartedDateTime: time.Now(),
					Request: harv2.Request{
						Method: "POST",
						URL:    "https://api.com/login",
					},
					Response: harv2.Response{
						Status: 200,
						Content: harv2.Content{
							MimeType: "application/json",
							Text:     `{"token": "abc-123-xyz-token"}`,
						},
					},
				},
				{
					StartedDateTime: time.Now().Add(1 * time.Second),
					Request: harv2.Request{
						Method: "GET",
						URL:    "https://api.com/profile",
						Headers: []harv2.Header{
							{Name: "Authorization", Value: "Bearer abc-123-xyz-token"},
						},
					},
					Response: harv2.Response{
						Status: 200,
					},
				},
			},
		},
	}

	// 2. Run Convert
	workspaceID := idwrap.NewNow()
	depFinder := depfinder.NewDepFinder()
	result, err := harv2.ConvertHARWithDepFinder(har, workspaceID, &depFinder)
	require.NoError(t, err)

	// 3. Verify Requests
	// Expect 4 requests: Base A, Delta A, Base B, Delta B
	require.Len(t, result.HTTPRequests, 4)

	// Find Base B and Delta B
	var baseB, deltaB *mhttp.HTTP
	// We assume order or find by URL/IsDelta
	for i := range result.HTTPRequests {
		req := result.HTTPRequests[i]
		if req.Url == "https://api.com/profile" {
			if req.IsDelta {
				deltaB = &result.HTTPRequests[i]
			} else {
				baseB = &result.HTTPRequests[i]
			}
		}
	}
	require.NotNil(t, baseB, "Base Request B not found")
	require.NotNil(t, deltaB, "Delta Request B not found")

	// 4. Verify Headers
	// Base B should have raw value "Bearer abc-123"
	// Delta B should have templated value "Bearer {{...}}"
	
	// Helper to find headers for a request ID
	findHeader := func(httpID idwrap.IDWrap, key string) *mhttp.HTTPHeader {
		for i := range result.HTTPHeaders {
			h := result.HTTPHeaders[i]
			if h.HttpID == httpID && h.Key == key {
				return &h
			}
		}
		return nil
	}

	headerBase := findHeader(baseB.ID, "Authorization")
	require.NotNil(t, headerBase, "Base header not found")
	assert.Equal(t, "Bearer abc-123-xyz-token", headerBase.Value, "Base header should have raw value")
	assert.False(t, headerBase.IsDelta, "Base header should NOT be delta")

	headerDelta := findHeader(deltaB.ID, "Authorization")
	require.NotNil(t, headerDelta, "Delta header not found")
	assert.True(t, headerDelta.IsDelta, "Delta header should be marked IsDelta")
	assert.NotNil(t, headerDelta.ParentHttpHeaderID, "Delta header should have ParentID")
	assert.Equal(t, headerBase.ID, *headerDelta.ParentHttpHeaderID, "Delta header should point to Base header")
	
	// Check Delta Value for template
	require.NotNil(t, headerDelta.DeltaValue)
	assert.Contains(t, *headerDelta.DeltaValue, "{{", "Delta header value should contain template start")
	assert.Contains(t, *headerDelta.DeltaValue, "}}", "Delta header value should contain template end")
	assert.NotContains(t, *headerDelta.DeltaValue, "abc-123-xyz-token", "Delta header value should NOT contain raw token")
}
