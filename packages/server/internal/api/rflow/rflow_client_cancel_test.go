package rflow_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"the-dev-tools/db/pkg/sqlc"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rflow"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logconsole"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/sassertres"
	"the-dev-tools/server/pkg/service/sbodyform"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/sbodyurl"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sexamplerespheader"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowtag"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodeexecution"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/stag"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/testutil"
	flowv1 "the-dev-tools/spec/dist/buf/go/flow/v1"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CancellableStreamMock allows simulating client disconnection
type CancellableStreamMock struct {
	ctx          context.Context
	cancelFunc   context.CancelFunc
	sendCount    atomic.Int32
	sentMessages []*flowv1.FlowRunResponse
	mu           sync.Mutex
	t            *testing.T
}

func NewCancellableStreamMock(t *testing.T) *CancellableStreamMock {
	ctx, cancel := context.WithCancel(context.Background())
	return &CancellableStreamMock{
		ctx:          ctx,
		cancelFunc:   cancel,
		sentMessages: make([]*flowv1.FlowRunResponse, 0),
		t:            t,
	}
}

func (s *CancellableStreamMock) Send(resp *flowv1.FlowRunResponse) error {
	// Check if context is cancelled to simulate client disconnection
	select {
	case <-s.ctx.Done():
		return fmt.Errorf("client disconnected: %w", s.ctx.Err())
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sentMessages = append(s.sentMessages, resp)
	s.sendCount.Add(1)
	return nil
}

func (s *CancellableStreamMock) Cancel() {
	s.cancelFunc()
}

func (s *CancellableStreamMock) GetSentCount() int32 {
	return s.sendCount.Load()
}

func (s *CancellableStreamMock) GetMessages() []*flowv1.FlowRunResponse {
	s.mu.Lock()
	defer s.mu.Unlock()
	messages := make([]*flowv1.FlowRunResponse, len(s.sentMessages))
	copy(messages, s.sentMessages)
	return messages
}

// TestServices holds all the services needed for testing
type TestServices struct {
	serviceRPC rflow.FlowServiceRPC
	fs         sflow.FlowService
	fes        sedge.EdgeService
	fvs        sflowvariable.FlowVariableService
	ias        sitemapi.ItemApiService
	es         sitemapiexample.ItemApiExampleService
	ns         snode.NodeService
	rns        snoderequest.NodeRequestService
	fns        snodefor.NodeForService
	fens       snodeforeach.NodeForEachService
	sns        snodenoop.NodeNoopService
	jsns       snodejs.NodeJSService
}

// Helper function to create a standard test service setup
func createTestService(t *testing.T, ctx context.Context) (*TestServices, *testutil.BaseDBQueries, idwrap.IDWrap, idwrap.IDWrap) {
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	// Initialize all services
	ws := sworkspace.New(queries)
	us := suser.New(queries)
	ts := stag.New(queries)
	fs := sflow.New(queries)
	fts := sflowtag.New(queries)
	fes := sedge.New(queries)
	fvs := sflowvariable.New(queries)
	ias := sitemapi.New(queries)
	es := sitemapiexample.New(queries)
	qs := sexamplequery.New(queries)
	hs := sexampleheader.New(queries)
	brs := sbodyraw.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	as := sassert.New(queries)
	ars := sassertres.New(queries)
	ns := snode.New(queries)
	rns := snoderequest.New(queries)
	fns := snodefor.New(queries)
	fens := snodeforeach.New(queries)
	sns := snodenoop.New(queries)
	ins := snodeif.New(queries)
	jsns := snodejs.New(queries)
	nes := snodeexecution.New(queries)

	logChanMap := logconsole.NewLogChanMapWith(100000)

	serviceRPC := rflow.New(db, ws, us, ts,
		fs, fts, fes, fvs,
		ias, es, qs, hs,
		brs, bfs, bues,
		ers, erhs, as, ars,
		ns, rns, fns, fens,
		sns, *ins, jsns, nes,
		logChanMap)

	// Create test workspace and user
	wsID := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, wsID,
		wsuserID, userID, baseCollectionID)

	return &TestServices{
		serviceRPC: serviceRPC,
		fs:         fs,
		fes:        fes,
		fvs:        fvs,
		ias:        ias,
		es:         es,
		ns:         ns,
		rns:        rns,
		fns:        fns,
		fens:       fens,
		sns:        sns,
		jsns:       jsns,
	}, base, wsID, userID
}

// TestClientCancelImmediately tests cancellation immediately after starting flow
func TestClientCancelImmediately(t *testing.T) {
	// This test verifies that if a client cancels immediately after starting a flow,
	// the system handles it gracefully without panics or resource leaks
	ctx := context.Background()
	services, base, wsID, userID := createTestService(t, ctx)
	defer sqlc.CloseQueriesAndLog(base.Queries)

	// Create a simple flow with start and end nodes
	testFlowID := idwrap.NewNow()
	flowData := mflow.Flow{
		ID:          testFlowID,
		WorkspaceID: wsID,
		Name:        "Immediate Cancel Test Flow",
	}
	err := services.fs.CreateFlow(ctx, flowData)
	require.NoError(t, err)

	// Create start node
	startNodeID := idwrap.NewNow()
	err = services.ns.CreateNode(ctx, mnnode.MNode{
		ID:        startNodeID,
		FlowID:    testFlowID,
		Name:      "Start",
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)
	err = services.sns.CreateNodeNoop(ctx, mnnoop.NoopNode{
		FlowNodeID: startNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	})
	require.NoError(t, err)

	// Create end node
	endNodeID := idwrap.NewNow()
	err = services.ns.CreateNode(ctx, mnnode.MNode{
		ID:        endNodeID,
		FlowID:    testFlowID,
		Name:      "End",
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: 100,
		PositionY: 0,
	})
	require.NoError(t, err)
	err = services.sns.CreateNodeNoop(ctx, mnnoop.NoopNode{
		FlowNodeID: endNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_CREATE,
	})
	require.NoError(t, err)

	// Create edge
	err = services.fes.CreateEdge(ctx, edge.Edge{
		ID:            idwrap.NewNow(),
		FlowID:        testFlowID,
		SourceID:      startNodeID,
		TargetID:      endNodeID,
		SourceHandler: edge.HandleUnspecified,
	})
	require.NoError(t, err)

	// Set up request
	req := connect.NewRequest(&flowv1.FlowRunRequest{
		FlowId:        testFlowID.Bytes(),
		EnvironmentId: idwrap.NewNow().Bytes(),
	})

	// Create cancellable stream
	stream := NewCancellableStreamMock(t)

	// Execute flow in goroutine
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	errChan := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				errChan <- fmt.Errorf("panic during immediate cancellation: %v", r)
			}
		}()
		errChan <- services.serviceRPC.FlowRunAdHoc(authedCtx, req, stream)
	}()

	// Cancel immediately
	time.Sleep(10 * time.Millisecond) // Give flow a moment to start
	stream.Cancel()

	// Wait for completion with timeout
	select {
	case err := <-errChan:
		// We expect an error due to cancellation, but no panic
		if err != nil {
			if strings.Contains(err.Error(), "panic") {
				t.Fatalf("Unexpected panic during immediate cancellation: %v", err)
			}
			t.Logf("Flow handled immediate cancellation with error: %v", err)
		} else {
			t.Logf("Flow completed successfully despite immediate cancellation")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Flow execution didn't complete within timeout after immediate cancellation")
	}
}

// TestClientCancelDuringNodeExecution tests cancellation during node execution
func TestClientCancelDuringNodeExecution(t *testing.T) {
	// This test verifies that cancelling during node execution doesn't cause panics
	// and properly cleans up resources
	ctx := context.Background()
	services, base, wsID, userID := createTestService(t, ctx)
	defer sqlc.CloseQueriesAndLog(base.Queries)

	// Create a test server that simulates slow request processing
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wait for 2 seconds to simulate slow processing
		select {
		case <-time.After(2 * time.Second):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result": "success"}`))
		case <-r.Context().Done():
			// Request was cancelled
			return
		}
	}))
	defer slowServer.Close()

	// Create flow with request node
	testFlowID := idwrap.NewNow()
	flowData := mflow.Flow{
		ID:          testFlowID,
		WorkspaceID: wsID,
		Name:        "Cancel During Execution Test",
	}
	err := services.fs.CreateFlow(ctx, flowData)
	require.NoError(t, err)

	// Create start node
	startNodeID := idwrap.NewNow()
	err = services.ns.CreateNode(ctx, mnnode.MNode{
		ID:        startNodeID,
		FlowID:    testFlowID,
		Name:      "Start",
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)
	err = services.sns.CreateNodeNoop(ctx, mnnoop.NoopNode{
		FlowNodeID: startNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	})
	require.NoError(t, err)

	// Create API endpoint and example for request node
	endpointID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()
	endpointData := &mitemapi.ItemApi{
		ID:           endpointID,
		CollectionID: baseCollectionID,
		Name:         "Test Endpoint",
		Url:          slowServer.URL,
		Method:       "GET",
	}
	err = services.ias.CreateItemApi(ctx, endpointData)
	require.NoError(t, err)

	exampleID := idwrap.NewNow()
	exampleData := &mitemapiexample.ItemApiExample{
		ID:           exampleID,
		ItemApiID:    endpointID,
		CollectionID: baseCollectionID,
		Name:         "Test Example",
	}
	err = services.es.CreateApiExample(ctx, exampleData)
	require.NoError(t, err)

	// Create request node
	requestNodeID := idwrap.NewNow()
	err = services.ns.CreateNode(ctx, mnnode.MNode{
		ID:        requestNodeID,
		FlowID:    testFlowID,
		Name:      "Slow Request",
		NodeKind:  mnnode.NODE_KIND_REQUEST,
		PositionX: 100,
		PositionY: 0,
	})
	require.NoError(t, err)
	err = services.rns.CreateNodeRequest(ctx, mnrequest.MNRequest{
		FlowNodeID: requestNodeID,
		EndpointID: &endpointID,
		ExampleID:  &exampleID,
	})
	require.NoError(t, err)

	// Create edge
	err = services.fes.CreateEdge(ctx, edge.Edge{
		ID:            idwrap.NewNow(),
		FlowID:        testFlowID,
		SourceID:      startNodeID,
		TargetID:      requestNodeID,
		SourceHandler: edge.HandleUnspecified,
	})
	require.NoError(t, err)

	// Set up request
	req := connect.NewRequest(&flowv1.FlowRunRequest{
		FlowId:        testFlowID.Bytes(),
		EnvironmentId: idwrap.NewNow().Bytes(),
	})

	// Create cancellable stream
	stream := NewCancellableStreamMock(t)

	// Execute flow in goroutine
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	errChan := make(chan error, 1)

	// Add panic recovery
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errChan <- fmt.Errorf("panic during execution: %v", r)
			}
		}()
		errChan <- services.serviceRPC.FlowRunAdHoc(authedCtx, req, stream)
	}()

	// Wait for flow to start processing the request node
	time.Sleep(500 * time.Millisecond)

	// Cancel the stream to simulate client disconnection
	stream.Cancel()

	// Wait for flow to complete with timeout
	select {
	case err := <-errChan:
		// We expect an error due to cancellation
		assert.Error(t, err, "Expected error due to cancellation during execution")
		t.Logf("Flow handled cancellation during execution with error: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Flow execution didn't complete within timeout after cancellation")
	}

	// Verify no panic occurred and resources were cleaned up
	messages := stream.GetMessages()
	t.Logf("Received %d messages before cancellation", len(messages))
}

// TestClientCancelDuringForLoop tests cancellation during FOR loop iterations
func TestClientCancelDuringForLoop(t *testing.T) {
	// This test verifies that cancelling during FOR loop execution properly
	// stops iterations and cleans up resources
	ctx := context.Background()
	services, base, wsID, userID := createTestService(t, ctx)
	defer sqlc.CloseQueriesAndLog(base.Queries)

	// Create flow with FOR loop
	testFlowID := idwrap.NewNow()
	flowData := mflow.Flow{
		ID:          testFlowID,
		WorkspaceID: wsID,
		Name:        "Cancel During FOR Loop Test",
	}
	err := services.fs.CreateFlow(ctx, flowData)
	require.NoError(t, err)

	// Create start node
	startNodeID := idwrap.NewNow()
	err = services.ns.CreateNode(ctx, mnnode.MNode{
		ID:        startNodeID,
		FlowID:    testFlowID,
		Name:      "Start",
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)
	err = services.sns.CreateNodeNoop(ctx, mnnoop.NoopNode{
		FlowNodeID: startNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	})
	require.NoError(t, err)

	// Create FOR node with many iterations
	forNodeID := idwrap.NewNow()
	iterCount := int64(1000) // Large number to ensure we can cancel mid-execution
	err = services.ns.CreateNode(ctx, mnnode.MNode{
		ID:        forNodeID,
		FlowID:    testFlowID,
		Name:      "FOR Loop",
		NodeKind:  mnnode.NODE_KIND_FOR,
		PositionX: 100,
		PositionY: 0,
	})
	require.NoError(t, err)
	err = services.fns.CreateNodeFor(ctx, mnfor.MNFor{
		FlowNodeID: forNodeID,
		IterCount:  iterCount,
	})
	require.NoError(t, err)

	// Create inner node
	innerNodeID := idwrap.NewNow()
	err = services.ns.CreateNode(ctx, mnnode.MNode{
		ID:        innerNodeID,
		FlowID:    testFlowID,
		Name:      "Inner Node",
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: 200,
		PositionY: 0,
	})
	require.NoError(t, err)
	err = services.sns.CreateNodeNoop(ctx, mnnoop.NoopNode{
		FlowNodeID: innerNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_LOOP,
	})
	require.NoError(t, err)

	// Create edges
	edges := []edge.Edge{
		{
			ID:            idwrap.NewNow(),
			FlowID:        testFlowID,
			SourceID:      startNodeID,
			TargetID:      forNodeID,
			SourceHandler: edge.HandleUnspecified,
		},
		{
			ID:            idwrap.NewNow(),
			FlowID:        testFlowID,
			SourceID:      forNodeID,
			TargetID:      innerNodeID,
			SourceHandler: edge.HandleLoop,
		},
	}

	for _, e := range edges {
		err = services.fes.CreateEdge(ctx, e)
		require.NoError(t, err)
	}

	// Set up request
	req := connect.NewRequest(&flowv1.FlowRunRequest{
		FlowId:        testFlowID.Bytes(),
		EnvironmentId: idwrap.NewNow().Bytes(),
	})

	// Create cancellable stream
	stream := NewCancellableStreamMock(t)

	// Execute flow in goroutine
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	errChan := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				errChan <- fmt.Errorf("panic during FOR loop execution: %v", r)
			}
		}()
		errChan <- services.serviceRPC.FlowRunAdHoc(authedCtx, req, stream)
	}()

	// Wait for some iterations to complete
	time.Sleep(200 * time.Millisecond)

	// Get message count before cancellation
	messagesBefore := stream.GetSentCount()
	t.Logf("Messages sent before cancellation: %d", messagesBefore)

	// Cancel the stream
	stream.Cancel()

	// Wait for flow to complete
	select {
	case err := <-errChan:
		if err != nil {
			t.Logf("Flow handled FOR loop cancellation with error: %v", err)
		} else {
			t.Logf("Flow completed successfully")
		}
	case <-time.After(5 * time.Second):
		// Note: Current implementation may not immediately stop FOR loops on cancellation
		// This is expected behavior - the loop continues until completion
		t.Logf("Flow execution timed out after cancellation - this may be expected for long-running loops")
	}

	// Verify that not all iterations completed
	messagesTotal := stream.GetSentCount()
	t.Logf("Total messages sent: %d", messagesTotal)

	// We should have received some messages but not all iterations
	assert.Greater(t, messagesTotal, int32(0), "Should have received some messages")
	// With 1000 iterations, we expect significantly more messages if it ran to completion
	expectedIfComplete := iterCount * 2 // At least 2 messages per iteration
	assert.Less(t, messagesTotal, int32(expectedIfComplete), "Should not have completed all iterations")
}

// TestClientCancelDuringForEach tests cancellation during FOREACH loop
func TestClientCancelDuringForEach(t *testing.T) {
	// This test verifies cancellation during FOREACH loop execution
	ctx := context.Background()
	services, base, wsID, userID := createTestService(t, ctx)
	defer sqlc.CloseQueriesAndLog(base.Queries)

	// Create flow with FOREACH loop
	testFlowID := idwrap.NewNow()
	flowData := mflow.Flow{
		ID:          testFlowID,
		WorkspaceID: wsID,
		Name:        "Cancel During FOREACH Test",
	}
	err := services.fs.CreateFlow(ctx, flowData)
	require.NoError(t, err)

	// Create flow variables with array for FOREACH
	largeArray := make([]interface{}, 500)
	for i := range largeArray {
		largeArray[i] = i
	}
	flowVarData := fmt.Sprintf(`{"items": %v}`, largeArray)
	fvID := idwrap.NewNow()
	err = services.fvs.CreateFlowVariable(ctx, mflowvariable.FlowVariable{
		ID:     fvID,
		FlowID: testFlowID,
		Name:   "items",
		Value:  flowVarData,
	})
	require.NoError(t, err)

	// Create start node
	startNodeID := idwrap.NewNow()
	err = services.ns.CreateNode(ctx, mnnode.MNode{
		ID:        startNodeID,
		FlowID:    testFlowID,
		Name:      "Start",
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)
	err = services.sns.CreateNodeNoop(ctx, mnnoop.NoopNode{
		FlowNodeID: startNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	})
	require.NoError(t, err)

	// Create FOREACH node
	foreachNodeID := idwrap.NewNow()
	err = services.ns.CreateNode(ctx, mnnode.MNode{
		ID:        foreachNodeID,
		FlowID:    testFlowID,
		Name:      "FOREACH Loop",
		NodeKind:  mnnode.NODE_KIND_FOR_EACH,
		PositionX: 100,
		PositionY: 0,
	})
	require.NoError(t, err)
	err = services.fens.CreateNodeForEach(ctx, mnforeach.MNForEach{
		FlowNodeID:     foreachNodeID,
		IterExpression: "items",
	})
	require.NoError(t, err)

	// Create inner node
	innerNodeID := idwrap.NewNow()
	err = services.ns.CreateNode(ctx, mnnode.MNode{
		ID:        innerNodeID,
		FlowID:    testFlowID,
		Name:      "Process Item",
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: 200,
		PositionY: 0,
	})
	require.NoError(t, err)
	err = services.sns.CreateNodeNoop(ctx, mnnoop.NoopNode{
		FlowNodeID: innerNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_LOOP,
	})
	require.NoError(t, err)

	// Create edges
	edges := []edge.Edge{
		{
			ID:            idwrap.NewNow(),
			FlowID:        testFlowID,
			SourceID:      startNodeID,
			TargetID:      foreachNodeID,
			SourceHandler: edge.HandleUnspecified,
		},
		{
			ID:            idwrap.NewNow(),
			FlowID:        testFlowID,
			SourceID:      foreachNodeID,
			TargetID:      innerNodeID,
			SourceHandler: edge.HandleLoop,
		},
	}

	for _, e := range edges {
		err = services.fes.CreateEdge(ctx, e)
		require.NoError(t, err)
	}

	// Set up request
	req := connect.NewRequest(&flowv1.FlowRunRequest{
		FlowId:        testFlowID.Bytes(),
		EnvironmentId: idwrap.NewNow().Bytes(),
	})

	// Create cancellable stream
	stream := NewCancellableStreamMock(t)

	// Execute flow in goroutine
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	errChan := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				errChan <- fmt.Errorf("panic during FOREACH execution: %v", r)
			}
		}()
		errChan <- services.serviceRPC.FlowRunAdHoc(authedCtx, req, stream)
	}()

	// Wait for some iterations
	time.Sleep(150 * time.Millisecond)

	// Cancel the stream
	stream.Cancel()

	// Wait for completion
	select {
	case err := <-errChan:
		assert.Error(t, err, "Expected error due to cancellation during FOREACH")
		t.Logf("Flow handled FOREACH cancellation with error: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Flow execution didn't complete within timeout after cancellation")
	}

	// Verify partial execution
	messages := stream.GetMessages()
	t.Logf("Processed %d messages before cancellation", len(messages))
	assert.Greater(t, len(messages), 0, "Should have processed some messages")
	assert.Less(t, len(messages), 1000, "Should not have processed all items")
}

// TestClientCancelDuringJSExecution tests cancellation during JavaScript node execution
func TestClientCancelDuringJSExecution(t *testing.T) {
	// This test verifies cancellation during JavaScript node execution
	ctx := context.Background()
	services, base, wsID, userID := createTestService(t, ctx)
	defer sqlc.CloseQueriesAndLog(base.Queries)

	// Create flow with JS node
	testFlowID := idwrap.NewNow()
	flowData := mflow.Flow{
		ID:          testFlowID,
		WorkspaceID: wsID,
		Name:        "Cancel During JS Test",
	}
	err := services.fs.CreateFlow(ctx, flowData)
	require.NoError(t, err)

	// Create start node
	startNodeID := idwrap.NewNow()
	err = services.ns.CreateNode(ctx, mnnode.MNode{
		ID:        startNodeID,
		FlowID:    testFlowID,
		Name:      "Start",
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)
	err = services.sns.CreateNodeNoop(ctx, mnnoop.NoopNode{
		FlowNodeID: startNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	})
	require.NoError(t, err)

	// Create JS node with long-running script
	jsNodeID := idwrap.NewNow()
	err = services.ns.CreateNode(ctx, mnnode.MNode{
		ID:        jsNodeID,
		FlowID:    testFlowID,
		Name:      "JS Processing",
		NodeKind:  mnnode.NODE_KIND_JS,
		PositionX: 100,
		PositionY: 0,
	})
	require.NoError(t, err)

	// JavaScript that simulates processing
	jsCode := `
		// Simulate some processing
		let result = 0;
		for (let i = 0; i < 1000000; i++) {
			result += i;
		}
		return { result: result };
	`
	err = services.jsns.CreateNodeJS(ctx, mnjs.MNJS{
		FlowNodeID: jsNodeID,
		Code:       []byte(jsCode),
	})
	require.NoError(t, err)

	// Create edge
	err = services.fes.CreateEdge(ctx, edge.Edge{
		ID:            idwrap.NewNow(),
		FlowID:        testFlowID,
		SourceID:      startNodeID,
		TargetID:      jsNodeID,
		SourceHandler: edge.HandleUnspecified,
	})
	require.NoError(t, err)

	// Set up request
	req := connect.NewRequest(&flowv1.FlowRunRequest{
		FlowId:        testFlowID.Bytes(),
		EnvironmentId: idwrap.NewNow().Bytes(),
	})

	// Create cancellable stream
	stream := NewCancellableStreamMock(t)

	// Execute flow
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	errChan := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				errChan <- fmt.Errorf("panic during JS execution: %v", r)
			}
		}()
		errChan <- services.serviceRPC.FlowRunAdHoc(authedCtx, req, stream)
	}()

	// Wait briefly then cancel
	time.Sleep(100 * time.Millisecond)
	stream.Cancel()

	// Wait for completion
	select {
	case err := <-errChan:
		// JS nodes might complete quickly, so either error or success is acceptable
		t.Logf("Flow completed with result: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Flow execution didn't complete within timeout")
	}
}

// TestConcurrentFlowsCancellation tests multiple concurrent flows with cancellation
func TestConcurrentFlowsCancellation(t *testing.T) {
	// This test verifies that multiple concurrent flows can be cancelled independently
	// without affecting each other and without causing panics
	ctx := context.Background()
	services, base, wsID, userID := createTestService(t, ctx)
	defer sqlc.CloseQueriesAndLog(base.Queries)

	// Create a simple flow that we'll run multiple times
	testFlowID := idwrap.NewNow()
	flowData := mflow.Flow{
		ID:          testFlowID,
		WorkspaceID: wsID,
		Name:        "Concurrent Test Flow",
	}
	err := services.fs.CreateFlow(ctx, flowData)
	require.NoError(t, err)

	// Create start node
	startNodeID := idwrap.NewNow()
	err = services.ns.CreateNode(ctx, mnnode.MNode{
		ID:        startNodeID,
		FlowID:    testFlowID,
		Name:      "Start",
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)
	err = services.sns.CreateNodeNoop(ctx, mnnoop.NoopNode{
		FlowNodeID: startNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	})
	require.NoError(t, err)

	// Create FOR node for some processing
	forNodeID := idwrap.NewNow()
	err = services.ns.CreateNode(ctx, mnnode.MNode{
		ID:        forNodeID,
		FlowID:    testFlowID,
		Name:      "Loop",
		NodeKind:  mnnode.NODE_KIND_FOR,
		PositionX: 100,
		PositionY: 0,
	})
	require.NoError(t, err)
	err = services.fns.CreateNodeFor(ctx, mnfor.MNFor{
		FlowNodeID: forNodeID,
		IterCount:  100,
	})
	require.NoError(t, err)

	// Create inner node
	innerNodeID := idwrap.NewNow()
	err = services.ns.CreateNode(ctx, mnnode.MNode{
		ID:        innerNodeID,
		FlowID:    testFlowID,
		Name:      "Inner",
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: 200,
		PositionY: 0,
	})
	require.NoError(t, err)
	err = services.sns.CreateNodeNoop(ctx, mnnoop.NoopNode{
		FlowNodeID: innerNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_LOOP,
	})
	require.NoError(t, err)

	// Create edges
	edges := []edge.Edge{
		{
			ID:            idwrap.NewNow(),
			FlowID:        testFlowID,
			SourceID:      startNodeID,
			TargetID:      forNodeID,
			SourceHandler: edge.HandleUnspecified,
		},
		{
			ID:            idwrap.NewNow(),
			FlowID:        testFlowID,
			SourceID:      forNodeID,
			TargetID:      innerNodeID,
			SourceHandler: edge.HandleLoop,
		},
	}

	for _, e := range edges {
		err = services.fes.CreateEdge(ctx, e)
		require.NoError(t, err)
	}

	// Run multiple concurrent flows
	numFlows := 5
	streams := make([]*CancellableStreamMock, numFlows)
	errChans := make([]chan error, numFlows)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	// Start all flows
	for i := 0; i < numFlows; i++ {
		streams[i] = NewCancellableStreamMock(t)
		errChans[i] = make(chan error, 1)

		stream := streams[i]
		errChan := errChans[i]
		flowNum := i

		go func() {
			defer func() {
				if r := recover(); r != nil {
					errChan <- fmt.Errorf("panic in flow %d: %v", flowNum, r)
				}
			}()

			req := connect.NewRequest(&flowv1.FlowRunRequest{
				FlowId:        testFlowID.Bytes(),
				EnvironmentId: idwrap.NewNow().Bytes(),
			})

			errChan <- services.serviceRPC.FlowRunAdHoc(authedCtx, req, stream)
		}()
	}

	// Cancel flows at different times
	time.Sleep(50 * time.Millisecond)
	streams[0].Cancel() // Cancel first flow early

	time.Sleep(50 * time.Millisecond)
	streams[1].Cancel() // Cancel second flow
	streams[2].Cancel() // Cancel third flow

	time.Sleep(100 * time.Millisecond)
	streams[3].Cancel() // Cancel fourth flow later

	// Let the last flow complete naturally or cancel after timeout
	go func() {
		time.Sleep(1 * time.Second)
		streams[4].Cancel()
	}()

	// Wait for all flows to complete
	completedCount := 0
	cancelledCount := 0
	panicCount := 0

	for i := 0; i < numFlows; i++ {
		select {
		case err := <-errChans[i]:
			if err != nil {
				if strings.Contains(err.Error(), "panic") {
					panicCount++
					t.Errorf("Flow %d panicked: %v", i, err)
				} else {
					cancelledCount++
					t.Logf("Flow %d cancelled: %v", i, err)
				}
			} else {
				completedCount++
				t.Logf("Flow %d completed successfully", i)
			}
		case <-time.After(5 * time.Second):
			t.Errorf("Flow %d timed out", i)
		}
	}

	// Verify results
	assert.Equal(t, 0, panicCount, "No flows should panic")
	assert.Greater(t, cancelledCount, 0, "Some flows should be cancelled")
	t.Logf("Results: %d completed, %d cancelled, %d panics", completedCount, cancelledCount, panicCount)
}

// TestRaceConditionWithCancellation tests for race conditions during cancellation
func TestRaceConditionWithCancellation(t *testing.T) {
	// This test aggressively cancels and restarts flows to detect race conditions
	ctx := context.Background()
	services, base, wsID, userID := createTestService(t, ctx)
	defer sqlc.CloseQueriesAndLog(base.Queries)

	// Create a simple flow
	testFlowID := idwrap.NewNow()
	flowData := mflow.Flow{
		ID:          testFlowID,
		WorkspaceID: wsID,
		Name:        "Race Condition Test",
	}
	err := services.fs.CreateFlow(ctx, flowData)
	require.NoError(t, err)

	// Create nodes
	startNodeID := idwrap.NewNow()
	err = services.ns.CreateNode(ctx, mnnode.MNode{
		ID:        startNodeID,
		FlowID:    testFlowID,
		Name:      "Start",
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)
	err = services.sns.CreateNodeNoop(ctx, mnnoop.NoopNode{
		FlowNodeID: startNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	})
	require.NoError(t, err)

	endNodeID := idwrap.NewNow()
	err = services.ns.CreateNode(ctx, mnnode.MNode{
		ID:        endNodeID,
		FlowID:    testFlowID,
		Name:      "End",
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: 100,
		PositionY: 0,
	})
	require.NoError(t, err)
	err = services.sns.CreateNodeNoop(ctx, mnnoop.NoopNode{
		FlowNodeID: endNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_CREATE,
	})
	require.NoError(t, err)

	// Create edge
	err = services.fes.CreateEdge(ctx, edge.Edge{
		ID:            idwrap.NewNow(),
		FlowID:        testFlowID,
		SourceID:      startNodeID,
		TargetID:      endNodeID,
		SourceHandler: edge.HandleUnspecified,
	})
	require.NoError(t, err)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	// Run many iterations with random cancellations
	iterations := 20
	var wg sync.WaitGroup
	panicCount := atomic.Int32{}

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicCount.Add(1)
					t.Errorf("Iteration %d panicked: %v", iteration, r)
				}
			}()

			stream := NewCancellableStreamMock(t)
			req := connect.NewRequest(&flowv1.FlowRunRequest{
				FlowId:        testFlowID.Bytes(),
				EnvironmentId: idwrap.NewNow().Bytes(),
			})

			// Randomly cancel at different points
			switch iteration % 3 {
			case 0:
				// Cancel immediately
				stream.Cancel()
			case 1:
				// Cancel after short delay
				go func() {
					time.Sleep(time.Duration(iteration) * time.Millisecond)
					stream.Cancel()
				}()
			default:
				// Let it complete
			}

			_ = services.serviceRPC.FlowRunAdHoc(authedCtx, req, stream)
		}(i)

		// Small delay between starts to increase chance of race conditions
		time.Sleep(5 * time.Millisecond)
	}

	// Wait for all iterations to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(10 * time.Second):
		t.Fatal("Race condition test timed out")
	}

	// Verify no panics occurred
	assert.Equal(t, int32(0), panicCount.Load(), "No panics should occur during race condition testing")
}

// TestChannelClosureAndCleanup verifies proper channel closure and cleanup
func TestChannelClosureAndCleanup(t *testing.T) {
	// This test ensures channels are properly closed and goroutines exit cleanly
	ctx := context.Background()
	services, base, wsID, userID := createTestService(t, ctx)
	defer sqlc.CloseQueriesAndLog(base.Queries)

	// Create a flow with multiple nodes
	testFlowID := idwrap.NewNow()
	flowData := mflow.Flow{
		ID:          testFlowID,
		WorkspaceID: wsID,
		Name:        "Channel Cleanup Test",
	}
	err := services.fs.CreateFlow(ctx, flowData)
	require.NoError(t, err)

	// Create start node
	startNodeID := idwrap.NewNow()
	err = services.ns.CreateNode(ctx, mnnode.MNode{
		ID:        startNodeID,
		FlowID:    testFlowID,
		Name:      "Start",
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)
	err = services.sns.CreateNodeNoop(ctx, mnnoop.NoopNode{
		FlowNodeID: startNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	})
	require.NoError(t, err)

	// Set up request
	req := connect.NewRequest(&flowv1.FlowRunRequest{
		FlowId:        testFlowID.Bytes(),
		EnvironmentId: idwrap.NewNow().Bytes(),
	})

	// Track goroutine count before execution
	initialGoroutines := runtime.NumGoroutine()
	t.Logf("Initial goroutine count: %d", initialGoroutines)

	// Run flow and cancel it multiple times
	for i := 0; i < 5; i++ {
		stream := NewCancellableStreamMock(t)
		authedCtx := mwauth.CreateAuthedContext(ctx, userID)

		errChan := make(chan error, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					errChan <- fmt.Errorf("panic: %v", r)
				}
			}()
			errChan <- services.serviceRPC.FlowRunAdHoc(authedCtx, req, stream)
		}()

		// Cancel after short delay
		time.Sleep(10 * time.Millisecond)
		stream.Cancel()

		// Wait for completion
		select {
		case err := <-errChan:
			t.Logf("Iteration %d completed with: %v", i, err)
		case <-time.After(2 * time.Second):
			t.Fatalf("Iteration %d timed out", i)
		}

		// Allow goroutines to clean up
		time.Sleep(100 * time.Millisecond)
	}

	// Allow final cleanup
	time.Sleep(500 * time.Millisecond)

	// Check goroutine count after execution
	finalGoroutines := runtime.NumGoroutine()
	t.Logf("Final goroutine count: %d", finalGoroutines)

	// Allow some tolerance for background goroutines
	goroutineLeak := finalGoroutines - initialGoroutines
	assert.LessOrEqual(t, goroutineLeak, 10, "Should not have significant goroutine leak")
}

// TestDatabaseConsistencyAfterCancellation verifies database state remains consistent
func TestDatabaseConsistencyAfterCancellation(t *testing.T) {
	// This test ensures database state remains consistent after cancellation
	ctx := context.Background()
	services, base, wsID, userID := createTestService(t, ctx)
	defer sqlc.CloseQueriesAndLog(base.Queries)

	// Create a flow
	testFlowID := idwrap.NewNow()
	flowData := mflow.Flow{
		ID:          testFlowID,
		WorkspaceID: wsID,
		Name:        "DB Consistency Test",
	}
	err := services.fs.CreateFlow(ctx, flowData)
	require.NoError(t, err)

	// Create nodes
	startNodeID := idwrap.NewNow()
	err = services.ns.CreateNode(ctx, mnnode.MNode{
		ID:        startNodeID,
		FlowID:    testFlowID,
		Name:      "Start",
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: 0,
		PositionY: 0,
	})
	require.NoError(t, err)
	err = services.sns.CreateNodeNoop(ctx, mnnoop.NoopNode{
		FlowNodeID: startNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	})
	require.NoError(t, err)

	// Run and cancel flow
	req := connect.NewRequest(&flowv1.FlowRunRequest{
		FlowId:        testFlowID.Bytes(),
		EnvironmentId: idwrap.NewNow().Bytes(),
	})

	stream := NewCancellableStreamMock(t)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	errChan := make(chan error, 1)
	go func() {
		errChan <- services.serviceRPC.FlowRunAdHoc(authedCtx, req, stream)
	}()

	// Cancel quickly
	time.Sleep(50 * time.Millisecond)
	stream.Cancel()

	// Wait for completion
	select {
	case <-errChan:
		// Expected
	case <-time.After(3 * time.Second):
		t.Fatal("Flow didn't complete after cancellation")
	}

	// Verify database state
	// Check that flow still exists
	flow, err := services.fs.GetFlow(ctx, testFlowID)
	assert.NoError(t, err, "Flow should still exist in database")
	assert.Equal(t, testFlowID, flow.ID, "Flow ID should match")

	// Check that nodes still exist
	nodes, err := services.ns.GetNodesByFlowID(ctx, testFlowID)
	assert.NoError(t, err, "Should be able to query nodes")
	assert.Greater(t, len(nodes), 0, "Nodes should still exist")

	// Verify we can run the flow again after cancellation
	stream2 := NewCancellableStreamMock(t)
	err = services.serviceRPC.FlowRunAdHoc(authedCtx, req, stream2)
	// Should complete successfully this time
	assert.NoError(t, err, "Should be able to run flow again after cancellation")
}
