package harv2_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/translate/harv2"

	"github.com/stretchr/testify/require"
)

// TestIntegrationModernArchitecture demonstrates the key architectural improvements
func TestIntegrationModernArchitecture(t *testing.T) {
	// Create a sample HAR that demonstrates key transformation scenarios
	harJSON := `{
		"log": {
			"entries": [
				{
					"startedDateTime": "2023-01-01T00:00:00.000Z",
					"request": {
						"method": "GET",
						"url": "https://api.example.com/users",
						"httpVersion": "HTTP/1.1",
						"headers": [
							{"name": "Accept", "value": "application/json"},
							{"name": "Authorization", "value": "Bearer token123"}
						]
					},
					"response": {
						"status": 200,
						"statusText": "OK",
						"content": {"size": 1000, "mimeType": "application/json"}
					}
				},
				{
					"startedDateTime": "2023-01-01T00:00:00.025Z",
					"request": {
						"method": "POST",
						"url": "https://api.example.com/users",
						"httpVersion": "HTTP/1.1",
						"headers": [
							{"name": "Content-Type", "value": "application/json"},
							{"name": "Authorization", "value": "Bearer token123"}
						],
						"postData": {
							"mimeType": "application/json",
							"text": "{\"name\": \"John Doe\", \"email\": \"john@example.com\"}"
						}
					},
					"response": {
						"status": 201,
						"statusText": "Created",
						"content": {"size": 200, "mimeType": "application/json"}
					}
				}
			]
		}
	}`

	// Parse HAR using modern parser
	har, err := harv2.ConvertRaw([]byte(harJSON))
	require.NoError(t, err)

	// Convert to modern architecture
	workspaceID := idwrap.NewNow()
	result, err := harv2.ConvertHAR(har, workspaceID)
	require.NoError(t, err)

	// Verify modern mhttp.HTTP entities with delta system
	require.Len(t, result.HTTPRequests, 4, "Should have 2 original + 2 delta HTTP requests")

	var originalRequests []mhttp.HTTP
	var deltaRequests []mhttp.HTTP
	for _, req := range result.HTTPRequests {
		if req.IsDelta {
			deltaRequests = append(deltaRequests, req)
		} else {
			originalRequests = append(originalRequests, req)
		}
	}

	require.Len(t, originalRequests, 2, "Should have 2 original requests")
	require.Len(t, deltaRequests, 2, "Should have 2 delta requests")

	// Verify original request structure
	firstOriginal := originalRequests[0]
	require.Equal(t, "GET", firstOriginal.Method)
	require.Equal(t, "https://api.example.com/users", firstOriginal.Url)
	require.Equal(t, "Users", firstOriginal.Name)
	require.False(t, firstOriginal.IsDelta)
	require.Equal(t, workspaceID, firstOriginal.WorkspaceID)

	// Verify delta request structure
	firstDelta := deltaRequests[0]
	require.Equal(t, "Users (Delta)", firstDelta.Name)
	require.True(t, firstDelta.IsDelta)
	require.NotNil(t, firstDelta.ParentHttpID)
	require.Equal(t, firstOriginal.ID, *firstDelta.ParentHttpID)
	// Delta* fields should be nil when there's no actual difference from the base
	// (no depfinder templating in this test case)
	require.Nil(t, firstDelta.DeltaName, "DeltaName should be nil when no difference")
	require.Nil(t, firstDelta.DeltaUrl, "DeltaUrl should be nil when no difference")
	require.Nil(t, firstDelta.DeltaMethod, "DeltaMethod should be nil when no difference")

	// Verify modern file system structure
	httpFiles := make([]mfile.File, 0)
	for _, file := range result.Files {
		if file.ContentType == mfile.ContentTypeHTTP {
			httpFiles = append(httpFiles, file)
		}
	}
	require.Len(t, httpFiles, 2, "Should have 2 files for original requests")

	for _, file := range httpFiles {
		require.Equal(t, workspaceID, file.WorkspaceID)
		require.Equal(t, mfile.ContentTypeHTTP, file.ContentType)
		require.NotNil(t, file.ContentID)
		require.True(t, strings.HasSuffix(file.Name, ".request"))
		require.NotEmpty(t, file.Name)
	}

	// Verify flow generation
	require.NotZero(t, result.Flow.ID)
	require.Equal(t, "Imported HAR Flow", result.Flow.Name)
	require.Equal(t, workspaceID, result.Flow.WorkspaceID)

	// Verify nodes (only for original requests, not deltas)
	require.Len(t, result.Nodes, 3, "Should have 3 nodes (Start + 2 for original requests)")

	// Verify node naming convention (request_1, request_2)
	var requestNodes []mflow.Node
	for _, node := range result.Nodes {
		if node.NodeKind == mflow.NODE_KIND_REQUEST {
			requestNodes = append(requestNodes, node)
		}
	}
	require.Len(t, requestNodes, 2)
	// Nodes order is not guaranteed by map iteration but here slice order is preserved from append
	// First request node should be request_1
	// (Assuming result.Nodes order preserves insertion order which it does)
	// We need to find the one corresponding to first original request
	// Actually simpler: check that we have request_1 and request_2 names
	nodeNames := make(map[string]bool)
	for _, n := range requestNodes {
		nodeNames[n.Name] = true
	}
	require.True(t, nodeNames["request_1"], "Should contain node named request_1")
	require.True(t, nodeNames["request_2"], "Should contain node named request_2")

	require.Len(t, result.RequestNodes, 2, "Should have 2 request node data structures")

	// Verify edges (based on timestamp sequencing - both requests are within 50ms)
	require.NotEmpty(t, result.Edges, "Should have edges between closely-timed requests")

	// Verify edge structure
	for _, edge := range result.Edges {
		require.NotZero(t, edge.ID)
		require.Equal(t, result.Flow.ID, edge.FlowID)
		require.NotZero(t, edge.SourceID)
		require.NotZero(t, edge.TargetID)
	}
}

// TestIntegrationPerformanceCharacteristics validates performance characteristics
func TestIntegrationPerformanceCharacteristics(t *testing.T) {
	// Create a HAR with realistic complexity
	const numEntries = 50
	entries := make([]map[string]interface{}, 0, numEntries)

	for i := 0; i < numEntries; i++ {
		// Create timestamp with some variation
		timestamp := fmt.Sprintf("2023-01-01T00:00:%02d.%03dZ", i/60, (i%60)*1000/60)

		method := "GET"
		if i%5 == 0 {
			method = "POST"
		} else if i%7 == 0 {
			method = "PUT"
		}

		entry := map[string]interface{}{
			"startedDateTime": timestamp,
			"request": map[string]interface{}{
				"method":      method,
				"url":         fmt.Sprintf("https://api.example.com/resource/%d", i),
				"httpVersion": "HTTP/1.1",
				"headers": []map[string]string{
					{"name": "Accept", "value": "application/json"},
				},
			},
			"response": map[string]interface{}{
				"status":     200,
				"statusText": "OK",
				"content": map[string]interface{}{
					"size":     100,
					"mimeType": "application/json",
				},
			},
		}

		if method == "POST" || method == "PUT" {
			entry["request"].(map[string]interface{})["postData"] = map[string]interface{}{
				"mimeType": "application/json",
				"text":     fmt.Sprintf(`{"id": %d, "data": "test"}`, i),
			}
		}

		entries = append(entries, entry)
	}

	harData := map[string]interface{}{
		"log": map[string]interface{}{
			"entries": entries,
		},
	}

	harJSON, err := json.Marshal(harData)
	require.NoError(t, err)

	// Parse and convert
	har, err := harv2.ConvertRaw(harJSON)
	require.NoError(t, err)

	workspaceID := idwrap.NewNow()

	// Measure performance
	start := time.Now()
	result, err := harv2.ConvertHAR(har, workspaceID)
	duration := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Validate performance characteristics
	require.Less(t, duration, 100*time.Millisecond, "Processing 50 entries should take less than 100ms")
	require.Less(t, duration.Milliseconds(), int64(50), "Processing should be very fast")

	// Validate structure
	require.Len(t, result.HTTPRequests, numEntries*2, "Should have double entries due to delta system")

	httpFileCount := 0
	for _, file := range result.Files {
		if file.ContentType == mfile.ContentTypeHTTP {
			httpFileCount++
		}
	}
	require.Equal(t, numEntries, httpFileCount, "Should have one file per original request")

	require.Len(t, result.Nodes, numEntries+1, "Should have one node per original request plus start node")
	require.Len(t, result.RequestNodes, numEntries, "Should have one request node per original request")

	// Validate that memory usage is reasonable
	// (This is a basic check - in production you'd want more sophisticated memory profiling)
	require.LessOrEqual(t, len(result.Edges), numEntries, "Edge count should be reasonable after transitive reduction")
}

// TestIntegrationURLMapping demonstrates the URL-to-file path mapping
func TestIntegrationURLMapping(t *testing.T) {
	testCases := []struct {
		name           string
		url            string
		expectedFolder string
		expectedName   string
	}{
		{
			name:           "Simple API",
			url:            "https://api.example.com/users",
			expectedFolder: "/com/example/api",
			expectedName:   "Users.request",
		},
		{
			name:           "Complex nested path",
			url:            "https://service.api.example.com/v1/data/reports/daily",
			expectedFolder: "/com/example/service/api/v1/data/reports",
			expectedName:   "Data Reports Daily.request",
		},
		{
			name:           "PUT with ID",
			url:            "https://api.example.com/posts/12345",
			expectedFolder: "/com/example/api/posts",
			expectedName:   "Posts.request",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			harJSON := fmt.Sprintf(`{
				"log": {
					"entries": [{
						"startedDateTime": "2023-01-01T00:00:00.000Z",
						"request": {
							"method": "%s",
							"url": "%s",
							"httpVersion": "HTTP/1.1",
							"headers": [{"name": "Accept", "value": "application/json"}]
						},
						"response": {
							"status": 200,
							"statusText": "OK",
							"content": {"size": 100, "mimeType": "application/json"}
						}
					}]
				}
			}`, func() string {
				if strings.Contains(tc.url, "posts") {
					return "PUT"
				}
				return "GET"
			}(), tc.url)

			har, err := harv2.ConvertRaw([]byte(harJSON))
			require.NoError(t, err)

			workspaceID := idwrap.NewNow()
			result, err := harv2.ConvertHAR(har, workspaceID)
			require.NoError(t, err)

			// Find the HTTP file
			var file *mfile.File
			for i := range result.Files {
				if result.Files[i].ContentType == mfile.ContentTypeHTTP {
					file = &result.Files[i]
					break
				}
			}
			require.NotNil(t, file, "Should find HTTP file")

			// Verify file naming reflects URL structure
			require.True(t, strings.Contains(file.Name, tc.expectedName) ||
				strings.Contains(file.Name, strings.ReplaceAll(tc.expectedName, " ", "_")),
				"File name should reflect URL structure")

			// Verify content type
			require.Equal(t, mfile.ContentTypeHTTP, file.ContentType)
		})
	}
}
