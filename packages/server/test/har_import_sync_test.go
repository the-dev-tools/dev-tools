package test

import (
	"testing"
	"time"

	"the-dev-tools/server/internal/api/rhttp"
	"the-dev-tools/server/internal/api/rlog"
	"the-dev-tools/server/pkg/eventstream/memory"
	"the-dev-tools/server/pkg/http/resolver"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/svar"
	httpv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
	importv1 "the-dev-tools/spec/dist/buf/go/api/import/v1"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Mock HAR Data
const sampleHAR = `{
  "log": {
    "version": "1.2",
    "creator": {
      "name": "WebInspector",
      "version": "537.36"
    },
    "pages": [],
    "entries": [
      {
        "startedDateTime": "2023-10-26T10:00:00.000Z",
        "time": 50,
        "request": {
          "method": "GET",
          "url": "https://api.example.com/v1/users",
          "httpVersion": "HTTP/1.1",
          "headers": [
            {
              "name": "X-Delta-Test",
              "value": "true"
            }
          ],
          "queryString": [],
          "cookies": [],
          "headersSize": 100,
          "bodySize": 0
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "httpVersion": "HTTP/1.1",
          "headers": [],
          "cookies": [],
          "content": {
            "size": 0,
            "mimeType": "application/json",
            "text": "{}"
          },
          "redirectURL": "",
          "headersSize": 50,
          "bodySize": 2
        },
        "cache": {},
        "timings": {
          "send": 0,
          "wait": 50,
          "receive": 0
        }
      }
    ]
  }
}`

func TestHARImportAndSyncE2E(t *testing.T) {
	suite := setupHARImportE2ETest(t)
	ctx := suite.ctx

	// 1. Setup Stream Listeners
	// We need to listen to the streams *before* triggering the import

	// Channel to capture events
	receivedHttp := make([]rhttp.HttpEvent, 0)
	receivedHeaders := make([]rhttp.HttpHeaderEvent, 0)

	httpSub, err := suite.importHandler.HttpStream.Subscribe(ctx, func(topic rhttp.HttpTopic) bool { return true })
	require.NoError(t, err)

	headerSub, err := suite.importHandler.HttpHeaderStream.Subscribe(ctx, func(topic rhttp.HttpHeaderTopic) bool { return true })
	require.NoError(t, err)

	// Start collector goroutine
	collectDone := make(chan struct{})
	go func() {
		defer close(collectDone)
		timeout := time.After(2 * time.Second)
		for {
			select {
			case evt := <-httpSub:
				receivedHttp = append(receivedHttp, evt.Payload)
			case evt := <-headerSub:
				receivedHeaders = append(receivedHeaders, evt.Payload)
			case <-timeout:
				return
			}
			// Stop if we have enough data
			if len(receivedHttp) >= 2 && len(receivedHeaders) >= 1 {
				// Wait a tiny bit more for any stragglers
				time.Sleep(100 * time.Millisecond)
				return
			}
		}
	}()

	// 2. Execute Import
	importReq := &importv1.ImportRequest{
		WorkspaceId: suite.workspaceID.Bytes(),
		Name:        "Sync Test Import",
		Data:        []byte(sampleHAR),
		DomainData: []*importv1.ImportDomainData{
			{
				Enabled:  true,
				Domain:   "api.example.com",
				Variable: "baseUrl",
			},
		},
	}

	resp, err := suite.importHandler.Import(ctx, connect.NewRequest(importReq))
	require.NoError(t, err)

	t.Logf("Import Response: MissingData=%v, Domains=%d", resp.Msg.MissingData, len(resp.Msg.Domains))
	if resp.Msg.MissingData != importv1.ImportMissingDataKind_IMPORT_MISSING_DATA_KIND_UNSPECIFIED {
		t.Fatalf("Import reports missing data, events will not be published. MissingData: %v", resp.Msg.MissingData)
	}

	// Wait for collection to finish
	<-collectDone

	// 3. Validate Sync Events
	t.Logf("Received %d HTTP events and %d Header events", len(receivedHttp), len(receivedHeaders))
	assert.GreaterOrEqual(t, len(receivedHttp), 2, "Should receive at least 2 HTTP events (Base + Delta)")

	var baseHttp, deltaHttp *httpv1.Http
	for _, evt := range receivedHttp {
		if evt.IsDelta {
			deltaHttp = evt.Http
		} else {
			baseHttp = evt.Http
		}
	}

	assert.NotNil(t, baseHttp, "Should receive Base HTTP event")
	assert.NotNil(t, deltaHttp, "Should receive Delta HTTP event")
	assert.NotEmpty(t, receivedHeaders, "Should receive Header events")

	// 4. Validate Collections (RPC vs Sync Consistency)

	// Create missing services manually since they aren't exposed in suite.services
	// BaseDB is available in suite
	envService := senv.New(suite.baseDB.Queries, suite.importHandler.Logger) // Mock logger is fine
	varService := svar.New(suite.baseDB.Queries, suite.importHandler.Logger)
	httpAssertService := shttp.NewHttpAssertService(suite.baseDB.Queries)
	httpResponseService := shttp.NewHttpResponseService(suite.baseDB.Queries)

	logStreamer := memory.NewInMemorySyncStreamer[rlog.LogTopic, rlog.LogEvent]()

	requestResolver := resolver.NewStandardResolver(
		suite.importHandler.HttpService,
		&suite.importHandler.HttpHeaderService,
		suite.importHandler.HttpSearchParamService,
		suite.importHandler.HttpBodyRawService,
		suite.importHandler.HttpBodyFormService,
		suite.importHandler.HttpBodyUrlEncodedService,
		httpAssertService,
	)

	// Instantiate rhttp handler
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

	// Call HttpDeltaCollection
	deltaCollResp, err := httpHandler.HttpDeltaCollection(ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)

	// Call HttpHeaderDeltaCollection
	headerDeltaCollResp, err := httpHandler.HttpHeaderDeltaCollection(ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)

	// Verify HTTP Delta Collection
	var foundDeltaInColl *httpv1.HttpDelta
	for _, d := range deltaCollResp.Msg.Items {
		if string(d.DeltaHttpId) == string(deltaHttp.HttpId) {
			foundDeltaInColl = d
			break
		}
	}

	assert.NotNil(t, foundDeltaInColl, "Delta HTTP from Sync should be in Delta Collection")
	if foundDeltaInColl != nil {
		// Sync event `deltaHttp.HttpId` is the Delta's own ID.
		// Collection `foundDeltaInColl.DeltaHttpId` is the Delta's own ID.
		// Collection `foundDeltaInColl.HttpId` is the Parent (Base) ID.
		assert.Equal(t, deltaHttp.HttpId, foundDeltaInColl.DeltaHttpId, "IDs should match")
		assert.Equal(t, baseHttp.HttpId, foundDeltaInColl.HttpId, "Collection ParentID should match Base ID")
	}

	// Verify Header Delta Collection
	var deltaHeaderFromSync *httpv1.HttpHeader
	for _, hEvt := range receivedHeaders {
		if hEvt.IsDelta {
			deltaHeaderFromSync = hEvt.HttpHeader
			break
		}
	}

	if deltaHeaderFromSync != nil {
		var foundHeaderInColl *httpv1.HttpHeaderDelta
		for _, h := range headerDeltaCollResp.Msg.Items {
			if string(h.DeltaHttpHeaderId) == string(deltaHeaderFromSync.HttpHeaderId) {
				foundHeaderInColl = h
				break
			}
		}

		assert.NotNil(t, foundHeaderInColl, "Delta Header from Sync should be in Delta Collection")
		if foundHeaderInColl != nil {
			// Collection `HttpHeaderId` should be the Parent Header ID (Base Header).
			// We need to find the Base Header ID.
			var baseHeaderFromSync *httpv1.HttpHeader
			for _, hEvt := range receivedHeaders {
				if !hEvt.IsDelta && hEvt.HttpHeader.Key == deltaHeaderFromSync.Key {
					baseHeaderFromSync = hEvt.HttpHeader
					break
				}
			}

			if baseHeaderFromSync != nil {
				assert.Equal(t, baseHeaderFromSync.HttpHeaderId, foundHeaderInColl.HttpHeaderId, "Collection: HttpHeaderId should point to Base Header")
			}
		}
	} else {
		t.Log("ℹ️ No Delta Headers created by HAR import (expected if no templating used). Skipping Header Delta Collection verification.")
	}
}
