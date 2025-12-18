package test

import (
	"testing"
	"time"

	"the-dev-tools/server/internal/api/rfile"
	"the-dev-tools/server/internal/api/rhttp"
	"the-dev-tools/server/internal/api/rlog"
	"the-dev-tools/server/pkg/eventstream/memory"
	"the-dev-tools/server/pkg/http/resolver"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/shttp"
	httpv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
	importv1 "the-dev-tools/spec/dist/buf/go/api/import/v1"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

// HAR with Dependency
// Request 1: Login returns token "abc-12345-xyz" (>= 8 chars for depfinder matching)
// Request 2: Profile uses "Bearer abc-12345-xyz"
const harWithDeps = `{
  "log": {
    "version": "1.2",
    "entries": [
      {
        "startedDateTime": "2023-10-26T10:00:00.000Z",
        "time": 50,
        "request": {
          "method": "POST",
          "url": "https://api.example.com/login",
          "headers": [],
          "postData": { "mimeType": "application/json", "text": "{}" }
        },
        "response": {
          "status": 200,
          "content": {
            "mimeType": "application/json",
            "text": "{\"token\": \"abc-12345-xyz\", \"userId\": 99}"
          }
        }
      },
      {
        "startedDateTime": "2023-10-26T10:00:01.000Z",
        "time": 50,
        "request": {
          "method": "GET",
          "url": "https://api.example.com/profile",
          "headers": [
            {
              "name": "Authorization",
              "value": "Bearer abc-12345-xyz"
            },
            {
              "name": "X-Static",
              "value": "static-value"
            }
          ]
        },
        "response": { "status": 200, "content": { "text": "{}" } }
      }
    ]
  }
}`

func TestHARImport_DependencyDetection(t *testing.T) {
	suite := setupHARImportE2ETest(t)
	ctx := suite.ctx

	// 1. Setup Stream Listeners
	headerEvents := make(chan rhttp.HttpHeaderEvent, 20)
	fileEvents := make(chan rfile.FileEvent, 20)

	headerSub, err := suite.importHandler.HttpHeaderStream.Subscribe(ctx, func(t rhttp.HttpHeaderTopic) bool { return true })
	require.NoError(t, err)

	fileSub, err := suite.importHandler.FileStream.Subscribe(ctx, func(t rfile.FileTopic) bool { return true })
	require.NoError(t, err)

	// Collector
	go func() {
		for {
			select {
			case evt := <-headerSub:
				headerEvents <- evt.Payload
			case evt := <-fileSub:
				fileEvents <- evt.Payload
			case <-ctx.Done():
				return
			}
		}
	}()

	// 2. Import
	importReq := &importv1.ImportRequest{
		WorkspaceId: suite.workspaceID.Bytes(),
		Name:        "Dep Test Import",
		Data:        []byte(harWithDeps),
		DomainData: []*importv1.ImportDomainData{
			{Enabled: true, Domain: "api.example.com", Variable: "baseUrl"},
		},
	}

	_, err = suite.importHandler.Import(ctx, connect.NewRequest(importReq))
	require.NoError(t, err)

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	// 3. Verify Dependency in Delta Header
	// We look for the Authorization header in the events
	var authHeaderEvent *rhttp.HttpHeaderEvent
	for len(headerEvents) > 0 {
		evt := <-headerEvents
		// We want the Delta header (IsDelta=true) for Authorization
		if evt.IsDelta && evt.HttpHeader.Key == "Authorization" {
			authHeaderEvent = &evt
			break
		}
	}

	require.NotNil(t, authHeaderEvent, "Should find a Delta Authorization header")

	// The value should be templated, e.g., "Bearer {{...}}"
	// It should NOT be "Bearer abc-12345-xyz"
	t.Logf("Found Delta Authorization Header in Sync: %s", authHeaderEvent.HttpHeader.Value)
	assert.Contains(t, authHeaderEvent.HttpHeader.Value, "{{", "Value should contain variable template")
	assert.NotContains(t, authHeaderEvent.HttpHeader.Value, "abc-12345-xyz", "Value should NOT contain raw secret")
	assert.Contains(t, authHeaderEvent.HttpHeader.Value, "Bearer ", "Value should preserve prefix")

	logStreamer := memory.NewInMemorySyncStreamer[rlog.LogTopic, rlog.LogEvent]()

	// 4. Verify Collection Consistency
	// Instantiate rhttp handler
	envService := senv.NewEnvironmentService(suite.baseDB.Queries, suite.importHandler.Logger)
	varService := senv.NewVariableService(suite.baseDB.Queries, suite.importHandler.Logger)
	httpAssertService := shttp.NewHttpAssertService(suite.baseDB.Queries)
	httpResponseService := shttp.NewHttpResponseService(suite.baseDB.Queries)

	// Create resolver for delta resolution
	requestResolver := resolver.NewStandardResolver(
		suite.importHandler.HttpService,
		&suite.importHandler.HttpHeaderService,
		suite.importHandler.HttpSearchParamService,
		suite.importHandler.HttpBodyRawService,
		suite.importHandler.HttpBodyFormService,
		suite.importHandler.HttpBodyUrlEncodedService,
		httpAssertService,
	)

	httpHandler := rhttp.New(
		suite.baseDB.DB,
		suite.importHandler.HttpService.Reader(),
		*suite.importHandler.HttpService,
		suite.services.Us,
		suite.services.Ws,
		suite.services.Wus,
		envService,
		varService,
		suite.importHandler.HttpBodyRawService,
		suite.importHandler.HttpHeaderService,
		suite.importHandler.HttpSearchParamService,
		suite.importHandler.HttpBodyFormService,
		suite.importHandler.HttpBodyUrlEncodedService,
		httpAssertService,
		httpResponseService,
		requestResolver,
		&rhttp.HttpStreamers{
			Http:               suite.importHandler.HttpStream,
			HttpHeader:         suite.importHandler.HttpHeaderStream,
			HttpSearchParam:    suite.importHandler.HttpSearchParamStream,
			HttpBodyForm:       suite.importHandler.HttpBodyFormStream,
			HttpBodyUrlEncoded: suite.importHandler.HttpBodyUrlEncodedStream,
			HttpAssert:         memory.NewInMemorySyncStreamer[rhttp.HttpAssertTopic, rhttp.HttpAssertEvent](),
			HttpVersion:        memory.NewInMemorySyncStreamer[rhttp.HttpVersionTopic, rhttp.HttpVersionEvent](),
			HttpResponse:       memory.NewInMemorySyncStreamer[rhttp.HttpResponseTopic, rhttp.HttpResponseEvent](),
			HttpResponseHeader: memory.NewInMemorySyncStreamer[rhttp.HttpResponseHeaderTopic, rhttp.HttpResponseHeaderEvent](),
			HttpResponseAssert: memory.NewInMemorySyncStreamer[rhttp.HttpResponseAssertTopic, rhttp.HttpResponseAssertEvent](),
			HttpBodyRaw:        suite.importHandler.HttpBodyRawStream,
			Log:                logStreamer,
		},
	)

	// Call HttpHeaderDeltaCollection
	headerDeltaCollResp, err := httpHandler.HttpHeaderDeltaCollection(ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)

	// Find the Authorization header in the collection
	var foundHeaderInColl *httpv1.HttpHeaderDelta
	for _, h := range headerDeltaCollResp.Msg.Items {
		if string(h.DeltaHttpHeaderId) == string(authHeaderEvent.HttpHeader.HttpHeaderId) {
			foundHeaderInColl = h
			break
		}
	}

	require.NotNil(t, foundHeaderInColl, "Delta Authorization Header should be in Delta Collection")

	// Check Value in Collection
	assert.Equal(t, authHeaderEvent.HttpHeader.Value, *foundHeaderInColl.Value, "Collection Value should match Sync Value")
	t.Logf("Found Delta Authorization Header in Collection: %s", *foundHeaderInColl.Value)

	// Check Parent Mapping
	// We need to find the Base Header ID.
	// We can assume the import created a base header.
	// Let's verify that `HttpHeaderId` in collection is NOT `DeltaHttpHeaderId`.
	assert.NotEqual(t, foundHeaderInColl.DeltaHttpHeaderId, foundHeaderInColl.HttpHeaderId, "Collection: HttpHeaderId should point to Parent, not Self")

	// 5. Verify File Events
	// Check if any file events were published.
	// Usually importing a HAR creates a Flow file.
	hasFiles := false
	for len(fileEvents) > 0 {
		<-fileEvents
		hasFiles = true
	}
	assert.True(t, hasFiles, "Should receive File events (e.g. Flow file)")
}
