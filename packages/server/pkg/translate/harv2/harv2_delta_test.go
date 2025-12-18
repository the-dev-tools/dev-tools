package harv2_test

import (
	"testing"
	"time"

	"the-dev-tools/server/pkg/depfinder"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/translate/harv2"

	"github.com/stretchr/testify/require"
)

func TestConvertHAR_DeltaLinkage(t *testing.T) {
	entry := harv2.Entry{
		StartedDateTime: time.Now(),
		Request: harv2.Request{
			Method: "GET",
			URL:    "https://api.example.com/users",
		},
	}

	testHar := harv2.HAR{
		Log: harv2.Log{Entries: []harv2.Entry{entry}},
	}

	workspaceID := idwrap.NewNow()
	resolved, err := harv2.ConvertHAR(&testHar, workspaceID)
	require.NoError(t, err)

	// Verify we have 1 Request Node
	require.Len(t, resolved.RequestNodes, 1)
	reqNode := resolved.RequestNodes[0]

	// Verify IDs
	require.NotNil(t, reqNode.HttpID, "HttpID should be set")
	require.NotNil(t, reqNode.DeltaHttpID, "DeltaHttpID should be set")
	require.NotEqual(t, *reqNode.HttpID, *reqNode.DeltaHttpID, "Base and Delta IDs should differ")

	// Find the actual HTTP objects
	var baseReq, deltaReq *mhttp.HTTP
	// Note: harv2.MHTTP is alias for mhttp.HTTP in the test context if imported,
	// but here we access resolved.HTTPRequests which are mhttp.HTTP
	for _, r := range resolved.HTTPRequests {
		if r.ID == *reqNode.HttpID {
			baseReqCopy := r
			baseReq = &baseReqCopy
		} else if r.ID == *reqNode.DeltaHttpID {
			deltaReqCopy := r
			deltaReq = &deltaReqCopy
		}
	}

	require.NotNil(t, baseReq, "Base Request not found")
	require.NotNil(t, deltaReq, "Delta Request not found")
	require.True(t, deltaReq.IsDelta)
	require.Equal(t, baseReq.ID, *deltaReq.ParentHttpID)

	// Verify Files
	var baseFile, deltaFile *mfile.File
	for _, f := range resolved.Files {
		if f.ContentID != nil {
			if f.ContentID.Compare(baseReq.ID) == 0 {
				fCopy := f
				baseFile = &fCopy
			} else if f.ContentID.Compare(deltaReq.ID) == 0 {
				fCopy := f
				deltaFile = &fCopy
			}
		}
	}

	require.NotNil(t, baseFile, "Base File not found")
	require.NotNil(t, deltaFile, "Delta File not found")

	require.Equal(t, mfile.ContentTypeHTTP, baseFile.ContentType)
	require.Equal(t, mfile.ContentTypeHTTPDelta, deltaFile.ContentType)

	// Verify Colocation
	require.NotNil(t, baseFile.ParentID)
	require.NotNil(t, deltaFile.ParentID)
	require.Equal(t, baseFile.ID, *deltaFile.ParentID, "Delta file should be a child of the Base file")
}

func TestConvertHAR_DeltaDependencies(t *testing.T) {
	// Request 1: Returns an ID
	entry1 := harv2.Entry{
		StartedDateTime: time.Now(),
		Request: harv2.Request{
			Method: "POST",
			URL:    "https://api.example.com/login",
		},
		Response: harv2.Response{
			Status: 200,
			Content: harv2.Content{
				MimeType: "application/json",
				Text:     `{"token": "SECRET_TOKEN_123"}`,
			},
		},
	}

	// Request 2: Uses the ID
	entry2 := harv2.Entry{
		StartedDateTime: time.Now().Add(1 * time.Second),
		Request: harv2.Request{
			Method: "GET",
			URL:    "https://api.example.com/data",
			Headers: []harv2.Header{
				{Name: "X-Token", Value: "SECRET_TOKEN_123"},
			},
		},
	}

	testHar := harv2.HAR{
		Log: harv2.Log{Entries: []harv2.Entry{entry1, entry2}},
	}

	workspaceID := idwrap.NewNow()
	depFinder := depfinder.NewDepFinder()

	resolved, err := harv2.ConvertHARWithDepFinder(&testHar, workspaceID, &depFinder)
	require.NoError(t, err)

	// Get Request 2 Node
	require.Len(t, resolved.RequestNodes, 2)
	reqNode2 := resolved.RequestNodes[1] // 2nd request

	// Find Base and Delta for Request 2
	var baseReq2, deltaReq2 *mhttp.HTTP
	for _, r := range resolved.HTTPRequests {
		if r.ID == *reqNode2.HttpID {
			copy := r
			baseReq2 = &copy
		} else if r.ID == *reqNode2.DeltaHttpID {
			copy := r
			deltaReq2 = &copy
		}
	}

	require.NotNil(t, baseReq2)
	require.NotNil(t, deltaReq2)

	// Find the headers
	var baseHeader, deltaHeader string
	for _, h := range resolved.HTTPHeaders {
		if h.HttpID == baseReq2.ID && h.Key == "X-Token" {
			baseHeader = h.Value
		}
		if h.HttpID == deltaReq2.ID && h.Key == "X-Token" {
			deltaHeader = h.Value
		}
	}

	// Verify dependencies in the Delta Header
	require.Contains(t, deltaHeader, "{{ request_1.response.body.token }}", "Delta header should contain template")

	// Base header should contain the raw secret
	require.Contains(t, baseHeader, "SECRET_TOKEN_123", "Base header should contain raw secret")

	// And if the URL had a dependency, checking Delta Request URL would be valid.
	// Let's check if URL dependency is propagated.
}
