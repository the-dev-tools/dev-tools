package harv2_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"the-dev-tools/server/pkg/depfinder"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/translate/harv2"

	"github.com/stretchr/testify/require"
)

func TestHarResolvedSimple(t *testing.T) {
	entry := harv2.Entry{}
	entry.Request.Method = "GET"
	entry.Request.URL = "http://example.com"
	entry.Request.HTTPVersion = "HTTP/1.1"
	entry.Request.Headers = []harv2.Header{
		{Name: "Content-Type", Value: "application/json"},
	}
	entry.StartedDateTime = time.Now()

	testHar := harv2.HAR{
		Log: harv2.Log{
			Entries: []harv2.Entry{entry},
		},
	}

	workspaceID := idwrap.NewNow()

	resolved, err := harv2.ConvertHAR(&testHar, workspaceID)
	require.NoError(t, err)
	require.NotNil(t, resolved)

	// With delta system: 1 original + 1 delta = 2 HTTP requests
	require.Len(t, resolved.HTTPRequests, 2, "Expected 2 HTTP requests (1 original + 1 delta)")

	// Verify one original and one delta HTTP request
	var originalHTTP, deltaHTTP *mhttp.HTTP
	for i := range resolved.HTTPRequests {
		if !resolved.HTTPRequests[i].IsDelta {
			originalHTTP = &resolved.HTTPRequests[i]
		} else {
			deltaHTTP = &resolved.HTTPRequests[i]
		}
	}

	require.NotNil(t, originalHTTP, "Expected to find one original HTTP request")
	require.NotNil(t, deltaHTTP, "Expected to find one delta HTTP request")
	require.Equal(t, originalHTTP.ID, *deltaHTTP.ParentHttpID, "Expected delta HTTP request to reference original HTTP request as parent")

	// Verify file system structure
	// We expect 1 HTTP file, plus folder files and flow file
	require.NotEmpty(t, resolved.Files)

	var httpFile *mfile.File
	for i := range resolved.Files {
		if resolved.Files[i].ContentType == mfile.ContentTypeHTTP {
			httpFile = &resolved.Files[i]
			break
		}
	}

	require.NotNil(t, httpFile, "Expected 1 HTTP file to be created")
	require.Equal(t, originalHTTP.ID, *httpFile.ContentID)

	// Verify flow generation
	require.NotZero(t, resolved.Flow.ID, "Expected flow to be created")
	require.NotEmpty(t, resolved.RequestNodes, "Expected request nodes to be created")
}

func TestConvertHARWithDepFinder(t *testing.T) {
	entry := harv2.Entry{}
	entry.Request.Method = "POST"
	entry.Request.URL = "https://api.example.com/users"
	entry.Request.HTTPVersion = "HTTP/1.1"
	entry.Request.Headers = []harv2.Header{
		{Name: "Content-Type", Value: "application/json"},
	}
	entry.Request.PostData = &harv2.PostData{
		MimeType: "application/json",
		Text:     `{"name": "John Doe", "email": "john@example.com"}`,
	}
	entry.StartedDateTime = time.Now()

	testHar := harv2.HAR{
		Log: harv2.Log{
			Entries: []harv2.Entry{entry},
		},
	}

	workspaceID := idwrap.NewNow()
	depFinder := depfinder.NewDepFinder()

	resolved, err := harv2.ConvertHARWithDepFinder(&testHar, workspaceID, &depFinder)
	require.NoError(t, err)
	require.NotNil(t, resolved)

	require.Len(t, resolved.HTTPRequests, 2, "Expected 2 HTTP requests with delta system")
	require.NotNil(t, resolved.Files)
}

func TestConvertRaw(t *testing.T) {
	harJSON := `{
		"log": {
			"entries": [
				{
					"startedDateTime": "2023-01-01T00:00:00.000Z",
					"request": {
						"method": "GET",
						"url": "https://api.example.com/test",
						"httpVersion": "HTTP/1.1",
						"headers": [{"name": "Accept", "value": "application/json"}]
					},
					"response": {
						"status": 200,
						"statusText": "OK",
						"content": {"size": 0, "mimeType": "application/json"}
					}
				}
			]
		}
	}`

	parsed, err := harv2.ConvertRaw([]byte(harJSON))
	require.NoError(t, err)
	require.NotNil(t, parsed)
	require.Len(t, parsed.Log.Entries, 1)
	require.Equal(t, "GET", parsed.Log.Entries[0].Request.Method)
	require.Equal(t, "https://api.example.com/test", parsed.Log.Entries[0].Request.URL)
}

func TestConvertRawInvalidJSON(t *testing.T) {
	invalidJSON := `{"invalid": json}`
	_, err := harv2.ConvertRaw([]byte(invalidJSON))
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse HAR data")
}

func TestConvertHARWithNilInput(t *testing.T) {
	workspaceID := idwrap.NewNow()
	_, err := harv2.ConvertHAR(nil, workspaceID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "HAR input cannot be nil")
}

func TestConvertHARWithEmptyEntries(t *testing.T) {
	emptyHar := harv2.HAR{Log: harv2.Log{Entries: []harv2.Entry{}}}
	workspaceID := idwrap.NewNow()

	resolved, err := harv2.ConvertHAR(&emptyHar, workspaceID)
	require.NoError(t, err)
	require.NotNil(t, resolved)
	require.Empty(t, resolved.HTTPRequests)
	require.Empty(t, resolved.Files)
}

func TestURLToFilePathMapping(t *testing.T) {
	testCases := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "Simple API",
			url:      "https://api.example.com/users",
			expected: "/com/example/api/users",
		},
		{
			name:     "Complex nested path",
			url:      "https://api.service.example.com/v1/users/123/posts",
			expected: "/com/example/service/api/v1/users/posts",
		},
		{
			name:     "WWW subdomain",
			url:      "https://www.example.com/api/data",
			expected: "/com/example/www/api/data",
		},
		{
			name:     "Port specification",
			url:      "http://localhost:8080/api/health",
			expected: "localhost:8080/api/health",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entry := harv2.Entry{
				StartedDateTime: time.Now(),
				Request: harv2.Request{
					Method: "GET",
					URL:    tc.url,
				},
			}

			testHar := harv2.HAR{
				Log: harv2.Log{Entries: []harv2.Entry{entry}},
			}

			workspaceID := idwrap.NewNow()
			resolved, err := harv2.ConvertHAR(&testHar, workspaceID)
			require.NoError(t, err)

			// The folder path should be reflected in the file's parent folder structure
			require.NotEmpty(t, resolved.Files)
			// Find the HTTP file
			var file *mfile.File
			for i := range resolved.Files {
				if resolved.Files[i].ContentType == mfile.ContentTypeHTTP {
					file = &resolved.Files[i]
					break
				}
			}
			require.NotNil(t, file, "Should find HTTP file")
			require.NotNil(t, file.ParentID)
		})
	}
}

func TestRequestNameGeneration(t *testing.T) {
	testCases := []struct {
		name     string
		method   string
		url      string
		expected string
	}{
		{
			name:     "Simple GET",
			method:   "GET",
			url:      "https://api.example.com/users",
			expected: "GET Users",
		},
		{
			name:     "POST with nested path",
			method:   "POST",
			url:      "https://api.example.com/v1/users/create",
			expected: "POST V1 Users Create",
		},
		{
			name:     "PUT with ID parameter",
			method:   "PUT",
			url:      "https://api.example.com/users/12345",
			expected: "PUT Users",
		},
		{
			name:     "DELETE with ID",
			method:   "DELETE",
			url:      "https://api.example.com/posts/67890",
			expected: "DELETE Posts",
		},
		{
			name:     "Complex API path",
			method:   "GET",
			url:      "https://service.api.example.com/v1/data/reports/daily",
			expected: "GET Data Reports Daily",
		},
		{
			name:     "Root path with hostname only",
			method:   "GET",
			url:      "https://api.test.com/",
			expected: "GET Api Test Com",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entry := harv2.Entry{
				StartedDateTime: time.Now(),
				Request: harv2.Request{
					Method: tc.method,
					URL:    tc.url,
				},
			}

			testHar := harv2.HAR{
				Log: harv2.Log{Entries: []harv2.Entry{entry}},
			}

			workspaceID := idwrap.NewNow()
			resolved, err := harv2.ConvertHAR(&testHar, workspaceID)
			require.NoError(t, err)
			require.Len(t, resolved.HTTPRequests, 2)

			// Check the original HTTP request name
			var originalReq *mhttp.HTTP
			for _, req := range resolved.HTTPRequests {
				if !req.IsDelta {
					originalReq = &req
					break
				}
			}
			require.NotNil(t, originalReq)
			require.Equal(t, tc.expected, originalReq.Name)
		})
	}
}

func TestTimestampSequencing(t *testing.T) {
	baseTime := time.Now()

	entries := []harv2.Entry{
		{
			StartedDateTime: baseTime,
			Request: harv2.Request{
				Method: "GET",
				URL:    "https://api.example.com/users",
			},
		},
		{
			StartedDateTime: baseTime.Add(10 * time.Millisecond), // Within threshold
			Request: harv2.Request{
				Method: "GET",
				URL:    "https://api.example.com/posts",
			},
		},
		{
			StartedDateTime: baseTime.Add(100 * time.Millisecond), // Outside threshold
			Request: harv2.Request{
				Method: "GET",
				URL:    "https://api.example.com/comments",
			},
		},
		{
			StartedDateTime: baseTime.Add(110 * time.Millisecond), // Within threshold of previous
			Request: harv2.Request{
				Method: "POST",
				URL:    "https://api.example.com/data",
			},
		},
	}

	testHar := harv2.HAR{
		Log: harv2.Log{Entries: entries},
	}

	workspaceID := idwrap.NewNow()
	resolved, err := harv2.ConvertHAR(&testHar, workspaceID)
	require.NoError(t, err)

	// Should have edges for closely timed requests
	// Entry 0 -> Entry 1 (10ms difference, within threshold)
	// Entry 2 -> Entry 3 (10ms difference, within threshold)
	// No edge Entry 1 -> Entry 2 (90ms difference, outside threshold)
	// Plus rooting edges from Start node to orphans (Entry 0 and Entry 2)

	expectedEdges := 4
	require.Len(t, resolved.Edges, expectedEdges, "Expected %d edges based on timestamp sequencing", expectedEdges)
}

func TestMutationMethodDetection(t *testing.T) {
	mutationMethods := []string{"POST", "PUT", "PATCH", "DELETE"}
	nonMutationMethods := []string{"GET", "HEAD", "OPTIONS"}

	for _, method := range mutationMethods {
		t.Run(fmt.Sprintf("Mutation_%s", method), func(t *testing.T) {
			// This tests the helper function logic
			// In a real implementation, mutation methods might affect edge creation
			entry := harv2.Entry{
				StartedDateTime: time.Now(),
				Request: harv2.Request{
					Method: method,
					URL:    "https://api.example.com/test",
				},
			}

			testHar := harv2.HAR{
				Log: harv2.Log{Entries: []harv2.Entry{entry}},
			}

			workspaceID := idwrap.NewNow()
			resolved, err := harv2.ConvertHAR(&testHar, workspaceID)
			require.NoError(t, err)
			require.Len(t, resolved.HTTPRequests, 2)

			var originalReq *mhttp.HTTP
			for _, req := range resolved.HTTPRequests {
				if !req.IsDelta {
					originalReq = &req
					break
				}
			}
			require.Equal(t, method, originalReq.Method)
		})
	}

	for _, method := range nonMutationMethods {
		t.Run(fmt.Sprintf("NonMutation_%s", method), func(t *testing.T) {
			entry := harv2.Entry{
				StartedDateTime: time.Now(),
				Request: harv2.Request{
					Method: method,
					URL:    "https://api.example.com/test",
				},
			}

			testHar := harv2.HAR{
				Log: harv2.Log{Entries: []harv2.Entry{entry}},
			}

			workspaceID := idwrap.NewNow()
			resolved, err := harv2.ConvertHAR(&testHar, workspaceID)
			require.NoError(t, err)
			require.Len(t, resolved.HTTPRequests, 2)
		})
	}
}

func TestDeltaSystem(t *testing.T) {
	entry := harv2.Entry{
		StartedDateTime: time.Now(),
		Request: harv2.Request{
			Method: "GET",
			URL:    "https://api.example.com/users",
			Headers: []harv2.Header{
				{Name: "Authorization", Value: "Bearer token123"},
				{Name: "Content-Type", Value: "application/json"},
			},
		},
	}

	testHar := harv2.HAR{
		Log: harv2.Log{Entries: []harv2.Entry{entry}},
	}

	workspaceID := idwrap.NewNow()
	resolved, err := harv2.ConvertHAR(&testHar, workspaceID)
	require.NoError(t, err)

	// Should have exactly 2 HTTP requests: original + delta
	require.Len(t, resolved.HTTPRequests, 2)

	var originalReq, deltaReq *mhttp.HTTP
	for i := range resolved.HTTPRequests {
		if !resolved.HTTPRequests[i].IsDelta {
			originalReq = &resolved.HTTPRequests[i]
		} else {
			deltaReq = &resolved.HTTPRequests[i]
		}
	}

	require.NotNil(t, originalReq, "Original request should exist")
	require.NotNil(t, deltaReq, "Delta request should exist")

	// Verify delta properties
	require.True(t, deltaReq.IsDelta, "Delta request should have IsDelta=true")
	require.Equal(t, originalReq.ID, *deltaReq.ParentHttpID, "Delta should reference original as parent")

	// Delta* fields should be nil when there's no actual difference from the base
	// (no depfinder templating in this test case)
	// This allows domain variable replacements to work correctly without being overwritten
	require.Nil(t, deltaReq.DeltaName, "DeltaName should be nil when no difference")
	require.Nil(t, deltaReq.DeltaUrl, "DeltaUrl should be nil when no difference")
	require.Nil(t, deltaReq.DeltaMethod, "DeltaMethod should be nil when no difference")
	require.Nil(t, deltaReq.DeltaDescription, "DeltaDescription should be nil when no difference")

	// Verify the delta's actual fields (not Delta* override fields) contain expected values
	require.Contains(t, deltaReq.Name, "(Delta)", "Delta name should indicate it's a delta")
	require.Equal(t, originalReq.Url, deltaReq.Url, "Delta URL should match original")
	require.Equal(t, originalReq.Method, deltaReq.Method, "Delta method should match original")
	require.Contains(t, deltaReq.Description, "[Delta Version]", "Delta description should indicate it's a delta version")
}

func TestFlowGraphGeneration(t *testing.T) {
	baseTime := time.Now()

	entries := []harv2.Entry{
		{
			StartedDateTime: baseTime,
			Request: harv2.Request{
				Method: "GET",
				URL:    "https://api.example.com/users",
			},
		},
		{
			StartedDateTime: baseTime.Add(10 * time.Millisecond),
			Request: harv2.Request{
				Method: "POST",
				URL:    "https://api.example.com/users",
			},
		},
		{
			StartedDateTime: baseTime.Add(20 * time.Millisecond),
			Request: harv2.Request{
				Method: "GET",
				URL:    "https://api.example.com/users/123",
			},
		},
	}

	testHar := harv2.HAR{
		Log: harv2.Log{Entries: entries},
	}

	workspaceID := idwrap.NewNow()
	resolved, err := harv2.ConvertHAR(&testHar, workspaceID)
	require.NoError(t, err)

	// Verify flow creation
	require.NotZero(t, resolved.Flow.ID, "Flow should be created")
	require.Equal(t, workspaceID, resolved.Flow.WorkspaceID, "Flow should belong to correct workspace")
	require.Equal(t, "Imported HAR Flow", resolved.Flow.Name, "Flow should have default name")

	// Verify request nodes (only non-delta requests)
	require.Len(t, resolved.RequestNodes, 3, "Should have 3 request nodes (no deltas)")

	// Verify each node corresponds to an original HTTP request
	nodeIDs := make(map[idwrap.IDWrap]bool)
	for _, node := range resolved.RequestNodes {
		nodeIDs[node.FlowNodeID] = true
		require.NotZero(t, node.HttpID, "Each node should reference an HTTP request")
	}
	// Also verify MNode structure
	for _, node := range resolved.Nodes {
		nodeIDs[node.ID] = true
		if node.NodeKind == mnnode.NODE_KIND_NO_OP {
			continue
		}
		require.Equal(t, mnnode.NODE_KIND_REQUEST, node.NodeKind, "All nodes should be request nodes")
	}

	// Verify edges exist for sequential requests
	require.NotEmpty(t, resolved.Edges, "Should have edges for sequential requests")

	// Verify edge structure
	for _, edge := range resolved.Edges {
		require.NotZero(t, edge.ID, "Edge should have valid ID")
		require.Equal(t, resolved.Flow.ID, edge.FlowID, "Edge should belong to flow")
		require.True(t, nodeIDs[edge.SourceID], "Edge source should be a valid node")
		require.True(t, nodeIDs[edge.TargetID], "Edge target should be a valid node")
	}
}

func TestTransitiveReduction(t *testing.T) {
	// This test verifies that the transitive reduction algorithm removes redundant edges
	// Create a scenario where A -> B -> C and A -> C (redundant)
	baseTime := time.Now()

	entries := []harv2.Entry{
		{
			StartedDateTime: baseTime,
			Request: harv2.Request{
				Method: "GET",
				URL:    "https://api.example.com/start",
			},
		},
		{
			StartedDateTime: baseTime.Add(10 * time.Millisecond),
			Request: harv2.Request{
				Method: "POST",
				URL:    "https://api.example.com/middle",
			},
		},
		{
			StartedDateTime: baseTime.Add(20 * time.Millisecond),
			Request: harv2.Request{
				Method: "GET",
				URL:    "https://api.example.com/end",
			},
		},
	}

	testHar := harv2.HAR{
		Log: harv2.Log{Entries: entries},
	}

	workspaceID := idwrap.NewNow()
	resolved, err := harv2.ConvertHAR(&testHar, workspaceID)
	require.NoError(t, err)

	// Should have sequential edges, but no redundant edges
	// A->B, B->C, plus Start->A (Rooting) = 3 edges
	require.Len(t, resolved.Edges, 3, "Should have 3 edges (Start->A, A->B, B->C), not 2 or 4")

	// Verify no redundant direct edge from first to last node
	firstNode := resolved.Nodes[0]
	lastNode := resolved.Nodes[len(resolved.Nodes)-1]

	hasDirectEdge := false
	for _, edge := range resolved.Edges {
		if edge.SourceID == firstNode.ID && edge.TargetID == lastNode.ID {
			hasDirectEdge = true
			break
		}
	}
	require.False(t, hasDirectEdge, "Should not have direct edge from first to last node (transitive reduction)")
}

func TestNodeLevelCalculation(t *testing.T) {
	// Node level calculation has been simplified in the new implementation
	// Positioning is now handled by the layout algorithm using PositionX/PositionY fields
	entries := []harv2.Entry{
		{
			StartedDateTime: time.Now(),
			Request: harv2.Request{
				Method: "GET",
				URL:    "https://api.example.com/level0",
			},
		},
		{
			StartedDateTime: time.Now().Add(10 * time.Millisecond),
			Request: harv2.Request{
				Method: "POST",
				URL:    "https://api.example.com/level1a",
			},
		},
		{
			StartedDateTime: time.Now().Add(15 * time.Millisecond),
			Request: harv2.Request{
				Method: "PUT",
				URL:    "https://api.example.com/level1b",
			},
		},
		{
			StartedDateTime: time.Now().Add(25 * time.Millisecond),
			Request: harv2.Request{
				Method: "GET",
				URL:    "https://api.example.com/level2",
			},
		},
	}

	testHar := harv2.HAR{
		Log: harv2.Log{Entries: entries},
	}

	workspaceID := idwrap.NewNow()
	resolved, err := harv2.ConvertHAR(&testHar, workspaceID)
	require.NoError(t, err)

	require.Len(t, resolved.RequestNodes, 4, "Should have 4 request nodes")
	require.Len(t, resolved.Nodes, 5, "Should have 5 visualization nodes (Start + 4 requests)")

	// Verify that nodes have proper positioning fields (they start at 0,0 and will be positioned by layout)
	for _, node := range resolved.Nodes {
		if node.NodeKind == mnnode.NODE_KIND_NO_OP {
			continue
		}
		require.Equal(t, mnnode.NODE_KIND_REQUEST, node.NodeKind, "All nodes should be request nodes")
	}
}

func TestMultipleEntriesComplexFlow(t *testing.T) {
	// Test with a more complex flow scenario
	baseTime := time.Now()

	entries := []harv2.Entry{
		// Initial setup requests
		{
			StartedDateTime: baseTime,
			Request: harv2.Request{
				Method: "POST",
				URL:    "https://api.example.com/auth/login",
			},
		},
		{
			StartedDateTime: baseTime.Add(50 * time.Millisecond),
			Request: harv2.Request{
				Method: "GET",
				URL:    "https://api.example.com/user/profile",
			},
		},
		// Data fetching requests (parallel potential)
		{
			StartedDateTime: baseTime.Add(60 * time.Millisecond),
			Request: harv2.Request{
				Method: "GET",
				URL:    "https://api.example.com/users",
			},
		},
		{
			StartedDateTime: baseTime.Add(65 * time.Millisecond),
			Request: harv2.Request{
				Method: "GET",
				URL:    "https://api.example.com/posts",
			},
		},
		// Data modification
		{
			StartedDateTime: baseTime.Add(200 * time.Millisecond),
			Request: harv2.Request{
				Method: "POST",
				URL:    "https://api.example.com/posts",
			},
		},
		{
			StartedDateTime: baseTime.Add(210 * time.Millisecond),
			Request: harv2.Request{
				Method: "PUT",
				URL:    "https://api.example.com/posts/123",
			},
		},
	}

	testHar := harv2.HAR{
		Log: harv2.Log{Entries: entries},
	}

	workspaceID := idwrap.NewNow()
	resolved, err := harv2.ConvertHAR(&testHar, workspaceID)
	require.NoError(t, err)

	// Verify structure
	require.Len(t, resolved.HTTPRequests, 12, "Should have 12 HTTP requests (6 original + 6 delta)")
	require.Len(t, resolved.RequestNodes, 6, "Should have 6 request nodes (no deltas)")
	require.NotEmpty(t, resolved.Edges, "Should have edges between related requests")
	require.NotEmpty(t, resolved.Files, "Should have files created")

	// Verify file structure
	// Filter for HTTP files
	httpFiles := make([]mfile.File, 0)
	for _, file := range resolved.Files {
		if file.ContentType == mfile.ContentTypeHTTP {
			httpFiles = append(httpFiles, file)
		}
	}

	require.Len(t, httpFiles, 6, "Should have 6 HTTP files (one per original request)")
	for _, file := range httpFiles {
		require.Equal(t, mfile.ContentTypeHTTP, file.ContentType, "All filtered files should be HTTP content")
		require.NotNil(t, file.ContentID, "All files should reference HTTP content")
	}

	// Verify workspace consistency
	for _, httpReq := range resolved.HTTPRequests {
		require.Equal(t, workspaceID, httpReq.WorkspaceID, "All HTTP requests should belong to workspace")
	}
	for _, file := range resolved.Files {
		require.Equal(t, workspaceID, file.WorkspaceID, "All files should belong to workspace")
	}
}

func TestErrorHandlingInvalidURL(t *testing.T) {
	entry := harv2.Entry{
		StartedDateTime: time.Now(),
		Request: harv2.Request{
			Method: "GET",
			URL:    "http://[invalid-ipv6", // Invalid URL with malformed IPv6
		},
	}

	testHar := harv2.HAR{
		Log: harv2.Log{Entries: []harv2.Entry{entry}},
	}

	workspaceID := idwrap.NewNow()
	_, err := harv2.ConvertHAR(&testHar, workspaceID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse URL")
}

func TestLargeNumberOfEntries(t *testing.T) {
	// Test performance with many entries
	const numEntries = 100
	entries := make([]harv2.Entry, numEntries)
	baseTime := time.Now()

	for i := 0; i < numEntries; i++ {
		entries[i] = harv2.Entry{
			StartedDateTime: baseTime.Add(time.Duration(i) * 10 * time.Millisecond),
			Request: harv2.Request{
				Method: "GET",
				URL:    fmt.Sprintf("https://api.example.com/resource/%d", i),
			},
		}
	}

	testHar := harv2.HAR{
		Log: harv2.Log{Entries: entries},
	}

	workspaceID := idwrap.NewNow()

	start := time.Now()
	resolved, err := harv2.ConvertHAR(&testHar, workspaceID)
	duration := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, resolved)
	require.Len(t, resolved.HTTPRequests, numEntries*2, "Should have double entries due to delta system")
	require.Len(t, resolved.RequestNodes, numEntries, "Should have one node per original request")

	// Count HTTP files
	httpFileCount := 0
	for _, file := range resolved.Files {
		if file.ContentType == mfile.ContentTypeHTTP {
			httpFileCount++
		}
	}
	require.Equal(t, numEntries, httpFileCount, "Should have one HTTP file per original request")

	// Performance should be reasonable (less than 1 second for 100 entries)
	require.Less(t, duration, time.Second, "Processing 100 entries should take less than 1 second")

	// Memory usage should be reasonable (rough estimation)
	require.Less(t, len(resolved.Edges), numEntries*2, "Edge count should be reasonable after transitive reduction")
}

func TestFileNamingSanitization(t *testing.T) {
	// Test that file names are properly generated
	testCases := []struct {
		url      string
		expected string
	}{
		{
			url:      "https://api.example.com/users?id=123&name=John%20Doe",
			expected: "GET_Users.request",
		},
		{
			url:      "https://api.example.com/users/john.doe/profile",
			expected: "GET_Users_John.doe_Profile.request",
		},
		{
			url:      "https://api.example.com/path-with-dashes/another_path",
			expected: "GET_Path_With_Dashes_Another_path.request",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.url, func(t *testing.T) {
			entry := harv2.Entry{
				StartedDateTime: time.Now(),
				Request: harv2.Request{
					Method: "GET",
					URL:    tc.url,
				},
			}

			testHar := harv2.HAR{
				Log: harv2.Log{Entries: []harv2.Entry{entry}},
			}

			workspaceID := idwrap.NewNow()
			resolved, err := harv2.ConvertHAR(&testHar, workspaceID)
			require.NoError(t, err)

			// Find the HTTP file
			var file *mfile.File
			for i := range resolved.Files {
				if resolved.Files[i].ContentType == mfile.ContentTypeHTTP {
					file = &resolved.Files[i]
					break
				}
			}
			require.NotNil(t, file, "Should create an HTTP file")

			require.Equal(t, tc.expected, file.Name)
		})
	}
}

func TestOverwriteDetectionWithoutService(t *testing.T) {
	// Test that when no service is provided, overwrite detection doesn't work (backward compatibility)
	entries := []harv2.Entry{
		{
			StartedDateTime: time.Now(),
			Request: harv2.Request{
				Method: "GET",
				URL:    "https://api.example.com/users",
				Headers: []harv2.Header{
					{Name: "Accept", Value: "application/json"},
				},
			},
		},
	}

	testHar := harv2.HAR{
		Log: harv2.Log{Entries: entries},
	}

	workspaceID := idwrap.NewNow()

	// First import
	resolved1, err := harv2.ConvertHAR(&testHar, workspaceID)
	require.NoError(t, err)
	require.Len(t, resolved1.HTTPRequests, 2) // Original + delta

	// Second import without service should create new records
	resolved2, err := harv2.ConvertHAR(&testHar, workspaceID)
	require.NoError(t, err)
	require.Len(t, resolved2.HTTPRequests, 2) // Original + delta (new ones)
}

func TestOverwriteDetectionWithService(t *testing.T) {
	// This test would require a mock HTTP service for proper testing
	// For now, we'll test the structure without a real service
	t.Skip("Requires mock HTTP service - test structure ready for implementation")

	entries := []harv2.Entry{
		{
			StartedDateTime: time.Now(),
			Request: harv2.Request{
				Method: "GET",
				URL:    "https://api.example.com/users",
				Headers: []harv2.Header{
					{Name: "Accept", Value: "application/json"},
				},
			},
		},
	}

	testHar := harv2.HAR{
		Log: harv2.Log{Entries: entries},
	}

	workspaceID := idwrap.NewNow()
	ctx := context.Background()

	// Test with nil service (should fall back to no overwrite detection)
	resolved, err := harv2.ConvertHARWithService(ctx, &testHar, workspaceID, nil)
	require.NoError(t, err)
	require.Len(t, resolved.HTTPRequests, 2) // Original + delta
}

func TestDeltaChildEntityCreation(t *testing.T) {
	// Test that delta child entities are created properly
	entry := harv2.Entry{
		StartedDateTime: time.Now(),
		Request: harv2.Request{
			Method: "POST",
			URL:    "https://api.example.com/users",
			Headers: []harv2.Header{
				{Name: "Content-Type", Value: "application/json"},
				{Name: "Authorization", Value: "Bearer token123"},
			},
			PostData: &harv2.PostData{
				MimeType: "application/json",
				Text:     `{"name": "John Doe", "email": "john@example.com"}`,
			},
		},
	}

	testHar := harv2.HAR{
		Log: harv2.Log{Entries: []harv2.Entry{entry}},
	}

	workspaceID := idwrap.NewNow()
	resolved, err := harv2.ConvertHAR(&testHar, workspaceID)
	require.NoError(t, err)

	// Should have original and delta HTTP requests
	require.Len(t, resolved.HTTPRequests, 2)

	// Should have child entities for both requests
	require.NotEmpty(t, resolved.HTTPHeaders, "Should have HTTP headers")
	require.NotEmpty(t, resolved.HTTPBodyRaws, "Should have raw body")

	// Find original and delta requests
	var original, delta *mhttp.HTTP
	for _, req := range resolved.HTTPRequests {
		if !req.IsDelta {
			original = &req
		} else {
			delta = &req
		}
	}

	require.NotNil(t, original, "Should have original request")
	require.NotNil(t, delta, "Should have delta request")

	// Verify delta properties
	require.True(t, delta.IsDelta, "Delta request should be marked as delta")
	require.Equal(t, original.ID, *delta.ParentHttpID, "Delta should reference original")
}

func TestDeltaHeaderComparison(t *testing.T) {
	// Test delta header creation with different scenarios
	originalHeaders := []mhttp.HTTPHeader{
		{
			ID:        idwrap.NewNow(),
			Key:       "Content-Type",
			Value:     "application/json",
			HttpID:    idwrap.NewNow(),
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
	}

	newHeaders := []mhttp.HTTPHeader{
		{
			ID:        idwrap.NewNow(),
			Key:       "Content-Type",
			Value:     "application/xml", // Different value
			HttpID:    idwrap.NewNow(),
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
		{
			ID:        idwrap.NewNow(),
			Key:       "Authorization", // New header
			Value:     "Bearer new-token",
			HttpID:    idwrap.NewNow(),
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
	}

	deltaHttpID := idwrap.NewNow()
	deltaHeaders := harv2.CreateDeltaHeaders(originalHeaders, newHeaders, deltaHttpID)

	// Should create deltas for changed and new headers
	require.Len(t, deltaHeaders, 2, "Should create 2 delta headers")

	// Verify delta structure
	for i, delta := range deltaHeaders {
		require.True(t, delta.IsDelta, "Delta header should be marked as delta")
		require.Equal(t, deltaHttpID, delta.HttpID, "Delta should reference correct HTTP")
		require.NotNil(t, delta.DeltaKey, "Delta should have delta key")
		require.NotNil(t, delta.DeltaValue, "Delta should have delta value")

		// The first header (Content-Type) should have a parent (it was changed)
		// The second header (Authorization) should have no parent (it's new)
		if i == 0 {
			require.NotNil(t, delta.ParentHttpHeaderID, "Changed header should have parent reference")
		} else {
			require.Nil(t, delta.ParentHttpHeaderID, "New header should not have parent reference")
		}
	}
}

func TestDeltaSearchParamsComparison(t *testing.T) {
	// Test delta search params creation
	originalParams := []mhttp.HTTPSearchParam{
		{
			ID:        idwrap.NewNow(),
			Key:       "page",
			Value:     "1",
			HttpID:    idwrap.NewNow(),
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
	}

	newParams := []mhttp.HTTPSearchParam{
		{
			ID:        idwrap.NewNow(),
			Key:       "page",
			Value:     "2", // Different value
			HttpID:    idwrap.NewNow(),
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
		{
			ID:        idwrap.NewNow(),
			Key:       "limit", // New param
			Value:     "10",
			HttpID:    idwrap.NewNow(),
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
	}

	deltaHttpID := idwrap.NewNow()
	deltaParams := harv2.CreateDeltaSearchParams(originalParams, newParams, deltaHttpID)

	// Should create deltas for changed and new params
	require.Len(t, deltaParams, 2, "Should create 2 delta search params")

	// Verify delta structure
	for _, delta := range deltaParams {
		require.True(t, delta.IsDelta, "Delta param should be marked as delta")
		require.Equal(t, deltaHttpID, delta.HttpID, "Delta should reference correct HTTP")
	}
}

func TestDeltaBodyFormComparison(t *testing.T) {
	// Test delta body form creation
	originalForms := []mhttp.HTTPBodyForm{
		{
			ID:        idwrap.NewNow(),
			Key:       "username",
			Value:     "olduser",
			HttpID:    idwrap.NewNow(),
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
	}

	newForms := []mhttp.HTTPBodyForm{
		{
			ID:        idwrap.NewNow(),
			Key:       "username",
			Value:     "newuser", // Different value
			HttpID:    idwrap.NewNow(),
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
		{
			ID:        idwrap.NewNow(),
			Key:       "password", // New form field
			Value:     "secret123",
			HttpID:    idwrap.NewNow(),
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		},
	}

	deltaHttpID := idwrap.NewNow()
	deltaForms := harv2.CreateDeltaBodyForms(originalForms, newForms, deltaHttpID)

	// Should create deltas for changed and new forms
	require.Len(t, deltaForms, 2, "Should create 2 delta body forms")

	// Verify delta structure
	for _, delta := range deltaForms {
		require.True(t, delta.IsDelta, "Delta form should be marked as delta")
		require.Equal(t, deltaHttpID, delta.HttpID, "Delta should reference correct HTTP")
	}
}

func TestDeltaBodyRawComparison(t *testing.T) {
	// Test delta raw body creation
	originalRaw := &mhttp.HTTPBodyRaw{
		ID:        idwrap.NewNow(),
		RawData:   []byte(`{"old": "data"}`),
		HttpID:    idwrap.NewNow(),
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	newRaw := &mhttp.HTTPBodyRaw{
		ID:        idwrap.NewNow(),
		RawData:   []byte(`{"new": "data"}`),
		HttpID:    idwrap.NewNow(),
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	deltaHttpID := idwrap.NewNow()
	deltaRaw := harv2.CreateDeltaBodyRaw(originalRaw, newRaw, deltaHttpID)

	// Should create delta because data is different
	require.NotNil(t, deltaRaw, "Should create delta raw body")
	require.True(t, deltaRaw.IsDelta, "Delta raw should be marked as delta")
	require.Equal(t, deltaHttpID, deltaRaw.HttpID, "Delta should reference correct HTTP")
	require.Equal(t, originalRaw.ID, *deltaRaw.ParentBodyRawID, "Delta should reference parent")
	require.NotNil(t, deltaRaw.DeltaRawData, "Delta should have delta data")

	// Test with identical data (should not create delta)
	sameRaw := &mhttp.HTTPBodyRaw{
		ID:        idwrap.NewNow(),
		RawData:   []byte(`{"old": "data"}`), // Same as original
		HttpID:    idwrap.NewNow(),
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	noDeltaRaw := harv2.CreateDeltaBodyRaw(originalRaw, sameRaw, deltaHttpID)
	require.Nil(t, noDeltaRaw, "Should not create delta for identical data")

	// Test with no original (should create new raw body, not delta)
	noOriginalRaw := harv2.CreateDeltaBodyRaw(nil, newRaw, deltaHttpID)
	require.NotNil(t, noOriginalRaw, "Should create raw body when no original exists")
	require.False(t, noOriginalRaw.IsDelta, "Should create non-delta raw body when no original")
}

func TestResponseStatusAssertions(t *testing.T) {
	// Test that assertions are created for response status codes in HAR entries
	entries := []harv2.Entry{
		{
			StartedDateTime: time.Now(),
			Request: harv2.Request{
				Method: "GET",
				URL:    "https://api.example.com/users",
				Headers: []harv2.Header{
					{Name: "Accept", Value: "application/json"},
				},
			},
			Response: harv2.Response{
				Status:     200,
				StatusText: "OK",
			},
		},
		{
			StartedDateTime: time.Now().Add(time.Second),
			Request: harv2.Request{
				Method: "POST",
				URL:    "https://api.example.com/users",
				Headers: []harv2.Header{
					{Name: "Content-Type", Value: "application/json"},
				},
				PostData: &harv2.PostData{
					MimeType: "application/json",
					Text:     `{"name": "Test User"}`,
				},
			},
			Response: harv2.Response{
				Status:     201,
				StatusText: "Created",
			},
		},
		{
			StartedDateTime: time.Now().Add(2 * time.Second),
			Request: harv2.Request{
				Method: "GET",
				URL:    "https://api.example.com/users/404",
				Headers: []harv2.Header{
					{Name: "Accept", Value: "application/json"},
				},
			},
			Response: harv2.Response{
				Status:     404,
				StatusText: "Not Found",
			},
		},
	}

	testHar := harv2.HAR{
		Log: harv2.Log{Entries: entries},
	}

	workspaceID := idwrap.NewNow()
	resolved, err := harv2.ConvertHAR(&testHar, workspaceID)
	require.NoError(t, err)

	// Should have 3 requests * 2 (base + delta) = 6 requests
	require.Len(t, resolved.HTTPRequests, 6, "Expected 6 HTTP requests (3 original + 3 delta)")

	// Should have 3 entries * 2 (base + delta) = 6 assertions
	require.Len(t, resolved.HTTPAsserts, 6, "Expected 6 HTTP assertions (3 base + 3 delta)")

	// Verify assertion structure
	var baseAsserts, deltaAsserts []mhttp.HTTPAssert
	for _, assert := range resolved.HTTPAsserts {
		if assert.IsDelta {
			deltaAsserts = append(deltaAsserts, assert)
		} else {
			baseAsserts = append(baseAsserts, assert)
		}
	}

	require.Len(t, baseAsserts, 3, "Expected 3 base assertions")
	require.Len(t, deltaAsserts, 3, "Expected 3 delta assertions")

	// Verify base assertions
	expectedValues := []string{"response.status == 200", "response.status == 201", "response.status == 404"}
	for i, assert := range baseAsserts {
		require.Equal(t, expectedValues[i], assert.Value, "Assertion value should contain the expression")
		require.True(t, assert.Enabled, "Assertion should be enabled")
		require.Contains(t, assert.Description, "HAR import", "Description should mention HAR import")
	}

	// Verify delta assertions link to base assertions
	for _, deltaAssert := range deltaAsserts {
		require.True(t, deltaAssert.IsDelta, "Delta assertion should be marked as delta")
		require.NotNil(t, deltaAssert.ParentHttpAssertID, "Delta assertion should have parent reference")
		require.Contains(t, deltaAssert.Value, "response.status ==", "Delta assertion value should contain 'response.status =='")
	}
}

func TestResponseStatusAssertionNoStatus(t *testing.T) {
	// Test that assertions are NOT created when response status is 0 or invalid
	entries := []harv2.Entry{
		{
			StartedDateTime: time.Now(),
			Request: harv2.Request{
				Method: "GET",
				URL:    "https://api.example.com/users",
			},
			Response: harv2.Response{
				Status: 0, // No status
			},
		},
	}

	testHar := harv2.HAR{
		Log: harv2.Log{Entries: entries},
	}

	workspaceID := idwrap.NewNow()
	resolved, err := harv2.ConvertHAR(&testHar, workspaceID)
	require.NoError(t, err)

	// Should have 1 request * 2 (base + delta) = 2 requests
	require.Len(t, resolved.HTTPRequests, 2, "Expected 2 HTTP requests")

	// Should have 0 assertions since status is 0
	require.Len(t, resolved.HTTPAsserts, 0, "Expected 0 HTTP assertions when status is 0")
}
