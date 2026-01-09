package test

import (
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rhttp"
	httpv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/http/v1"
	importv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/import/v1"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// HAR with URL Dependency
// Request 1: Create User returns ID "6d316d59-4cb4-451e-b5b1-673ecbdd5609"
// Request 2: Delete User uses URL "/users/6d316d59-4cb4-451e-b5b1-673ecbdd5609"
const harWithUrlDep = `{
  "log": {
    "version": "1.2",
    "entries": [
      {
        "startedDateTime": "2023-10-26T10:00:00.000Z",
        "time": 50,
        "request": {
          "method": "POST",
          "url": "https://api.example.com/users",
          "headers": [],
          "postData": { "mimeType": "application/json", "text": "{}" }
        },
        "response": {
          "status": 201,
          "content": {
            "mimeType": "application/json",
            "text": "{\"id\": \"6d316d59-4cb4-451e-b5b1-673ecbdd5609\", \"name\": \"test\"}"
          }
        }
      },
      {
        "startedDateTime": "2023-10-26T10:00:01.000Z",
        "time": 50,
        "request": {
          "method": "DELETE",
          "url": "https://api.example.com/users/6d316d59-4cb4-451e-b5b1-673ecbdd5609",
          "headers": []
        },
        "response": { "status": 204, "content": { "text": "" } }
      }
    ]
  }
}`

func TestHARImport_URLDependencyDetection(t *testing.T) {
	suite := setupHARImportE2ETest(t)
	ctx := suite.ctx

	// 1. Setup Stream Listener for HTTP events
	httpEvents := make(chan rhttp.HttpEvent, 20)
	httpSub, err := suite.importHandler.HttpStream.Subscribe(ctx, func(t rhttp.HttpTopic) bool { return true })
	require.NoError(t, err)

	// Collector
	go func() {
		for {
			select {
			case evt := <-httpSub:
				httpEvents <- evt.Payload
			case <-ctx.Done():
				return
			}
		}
	}()

	// 2. Import
	importReq := &importv1.ImportRequest{
		WorkspaceId: suite.workspaceID.Bytes(),
		Name:        "URL Dep Test Import",
		Data:        []byte(harWithUrlDep),
		DomainData: []*importv1.ImportDomainData{
			{Enabled: true, Domain: "api.example.com", Variable: "baseUrl"},
		},
	}

	_, err = suite.importHandler.Import(ctx, connect.NewRequest(importReq))
	require.NoError(t, err)

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	// 3. Verify Dependency in Delta URL
	// We look for the DELETE request Delta
	var deleteDeltaEvent *rhttp.HttpEvent
	for len(httpEvents) > 0 {
		evt := <-httpEvents
		if evt.IsDelta && evt.Http.Method == httpv1.HttpMethod_HTTP_METHOD_DELETE {
			deleteDeltaEvent = &evt
			break
		}
	}

	require.NotNil(t, deleteDeltaEvent, "Should find a Delta DELETE request")

	// The URL should be templated, e.g., ".../users/{{...}}"
	t.Logf("Found Delta DELETE URL: %s", deleteDeltaEvent.Http.Url)
	assert.Contains(t, deleteDeltaEvent.Http.Url, "{{", "URL should contain variable template")
	assert.NotContains(t, deleteDeltaEvent.Http.Url, "6d316d59-4cb4-451e-b5b1-673ecbdd5609", "URL should NOT contain raw UUID")
}
