package flowresult

import (
	"context"
	"log/slog"
	"sync"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/ngraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nrequest"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sgraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
)

// ResponseDrain persists HTTP and GraphQL responses produced during flow execution.
// It runs two goroutines (one per protocol) that consume from channels, persist
// to the database, publish events, and signal completion via per-response channels.
//
// The signal mechanism allows the ExecutionStateTracker to wait for a response
// to be published before publishing the execution event that references it,
// ensuring correct event ordering for frontends.
type ResponseDrain struct {
	workspaceID idwrap.IDWrap

	httpChan chan nrequest.NodeRequestSideResp
	gqlChan  chan ngraphql.NodeGraphQLSideResp

	// Per-response signal: closed when the response has been persisted and published.
	httpSignals   map[string]chan struct{}
	httpSignalsMu sync.Mutex
	gqlSignals    map[string]chan struct{}
	gqlSignalsMu  sync.Mutex

	httpSvc shttp.HttpResponseService
	gqlSvc  sgraphql.GraphQLResponseService

	publisher EventPublisher
	logger    *slog.Logger

	httpWg sync.WaitGroup
	gqlWg  sync.WaitGroup
}

// ResponseDrainOpts configures a ResponseDrain.
type ResponseDrainOpts struct {
	WorkspaceID idwrap.IDWrap
	BufSize     int

	HTTPResponseService    shttp.HttpResponseService
	GraphQLResponseService sgraphql.GraphQLResponseService

	Publisher EventPublisher
	Logger    *slog.Logger
}

func newResponseDrain(opts ResponseDrainOpts) *ResponseDrain {
	return &ResponseDrain{
		workspaceID: opts.WorkspaceID,
		httpChan:    make(chan nrequest.NodeRequestSideResp, opts.BufSize),
		gqlChan:     make(chan ngraphql.NodeGraphQLSideResp, opts.BufSize),
		httpSignals: make(map[string]chan struct{}),
		gqlSignals:  make(map[string]chan struct{}),
		httpSvc:     opts.HTTPResponseService,
		gqlSvc:      opts.GraphQLResponseService,
		publisher:   opts.Publisher,
		logger:      opts.Logger,
	}
}

func (d *ResponseDrain) start(ctx context.Context) {
	d.httpWg.Add(1)
	go d.runHTTP(ctx)

	d.gqlWg.Add(1)
	go d.runGraphQL(ctx)
}

// closeAndWait closes both channels and waits for goroutines to finish.
func (d *ResponseDrain) closeAndWait() {
	close(d.httpChan)
	d.httpWg.Wait()

	close(d.gqlChan)
	d.gqlWg.Wait()
}

// WaitForResponse blocks until the response with the given ID has been
// persisted and its event published. Call this before publishing an
// execution event that references the response.
func (d *ResponseDrain) WaitForResponse(respID string) {
	// Check HTTP signals
	d.httpSignalsMu.Lock()
	ch, ok := d.httpSignals[respID]
	d.httpSignalsMu.Unlock()
	if ok {
		<-ch
		d.httpSignalsMu.Lock()
		delete(d.httpSignals, respID)
		d.httpSignalsMu.Unlock()
	}

	// Check GraphQL signals
	d.gqlSignalsMu.Lock()
	ch, ok = d.gqlSignals[respID]
	d.gqlSignalsMu.Unlock()
	if ok {
		<-ch
		d.gqlSignalsMu.Lock()
		delete(d.gqlSignals, respID)
		d.gqlSignalsMu.Unlock()
	}
}

func (d *ResponseDrain) runHTTP(ctx context.Context) {
	defer d.httpWg.Done()
	for resp := range d.httpChan {
		responseID := resp.Resp.HTTPResponse.ID.String()

		// Register signal before processing so state tracker can find it
		d.httpSignalsMu.Lock()
		signal := make(chan struct{})
		d.httpSignals[responseID] = signal
		d.httpSignalsMu.Unlock()

		// Save HTTP Response
		if err := d.httpSvc.Create(ctx, resp.Resp.HTTPResponse); err != nil {
			d.logger.Error("failed to save http response", "error", err)
		} else {
			d.publisher.PublishHTTPResponse(resp.Resp.HTTPResponse, d.workspaceID)
		}

		// Save Headers
		for _, h := range resp.Resp.ResponseHeaders {
			if err := d.httpSvc.CreateHeader(ctx, h); err != nil {
				d.logger.Error("failed to save http response header", "error", err)
			} else {
				d.publisher.PublishHTTPResponseHeader(h, d.workspaceID)
			}
		}

		// Save Asserts
		for _, a := range resp.Resp.ResponseAsserts {
			if err := d.httpSvc.CreateAssert(ctx, a); err != nil {
				d.logger.Error("failed to save http response assert", "error", err)
			} else {
				d.publisher.PublishHTTPResponseAssert(a, d.workspaceID)
			}
		}

		close(signal)

		if resp.Done != nil {
			close(resp.Done)
		}
	}
}

func (d *ResponseDrain) runGraphQL(ctx context.Context) {
	defer d.gqlWg.Done()
	for resp := range d.gqlChan {
		responseID := resp.Response.ID.String()

		d.gqlSignalsMu.Lock()
		signal := make(chan struct{})
		d.gqlSignals[responseID] = signal
		d.gqlSignalsMu.Unlock()

		// Save all entities first, THEN publish events
		responseSuccess := false
		if err := d.gqlSvc.Create(ctx, resp.Response); err != nil {
			d.logger.Error("failed to save graphql response", "error", err)
		} else {
			responseSuccess = true
		}

		var successHeaders []mgraphql.GraphQLResponseHeader
		for _, h := range resp.RespHeaders {
			if err := d.gqlSvc.CreateHeader(ctx, h); err != nil {
				d.logger.Error("failed to save graphql response header", "error", err)
			} else {
				successHeaders = append(successHeaders, h)
			}
		}

		var successAsserts []mgraphql.GraphQLResponseAssert
		for _, a := range resp.RespAsserts {
			if err := d.gqlSvc.CreateAssert(ctx, a); err != nil {
				d.logger.Error("failed to save graphql response assert", "error", err)
			} else {
				successAsserts = append(successAsserts, a)
			}
		}

		// Publish events atomically after saves
		if responseSuccess {
			d.publisher.PublishGraphQLResponse(resp.Response, d.workspaceID)
			for _, h := range successHeaders {
				d.publisher.PublishGraphQLResponseHeader(h, d.workspaceID)
			}
			for _, a := range successAsserts {
				d.publisher.PublishGraphQLResponseAssert(a, d.workspaceID)
			}
		}

		close(signal)

		if resp.Done != nil {
			close(resp.Done)
		}
	}
}
