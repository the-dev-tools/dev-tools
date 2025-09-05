package rnode_test

import (
    "bytes"
    "context"
    "strings"
    "sync"
    "testing"
    "the-dev-tools/server/internal/api/middleware/mwauth"
    "the-dev-tools/server/internal/api/rflow"
    "the-dev-tools/server/internal/api/rnode"
    "the-dev-tools/server/pkg/flow/edge"
    "the-dev-tools/server/pkg/idwrap"
    "the-dev-tools/server/pkg/logconsole"
    "the-dev-tools/server/pkg/model/mcondition"
    "the-dev-tools/server/pkg/model/mflow"
    "the-dev-tools/server/pkg/model/mnnode"
    "the-dev-tools/server/pkg/model/mnnode/mnif"
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
    nodev1 "the-dev-tools/spec/dist/buf/go/flow/node/v1"

    "connectrpc.com/connect"
)

func TestNodeGet(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	// Create all required services
	fs := sflow.New(queries)
	us := suser.New(queries)
	ns := snode.New(queries)
	nis := snodeif.New(queries)
	nrs := snoderequest.New(queries)
	nfls := snodefor.New(queries)
	nlfes := snodeforeach.New(queries)
	nss := snodenoop.New(queries)
	jsns := snodejs.New(queries)
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	eqs := sexamplequery.New(queries)
	ehs := sexampleheader.New(queries)

	brs := sbodyraw.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)
	nes := snodeexecution.New(queries)

	// Create base test data
	wsID := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()
	base.GetBaseServices().CreateTempCollection(t, ctx, wsID,
		wsuserID, userID, baseCollectionID)

	// Create a flow
	flowID := idwrap.NewNow()
	nodeID := idwrap.NewNow()

	// Create a flow
	flow := mflow.Flow{
		ID:          flowID,
		Name:        "Test Flow",
		WorkspaceID: wsID,
	}
	err := fs.CreateFlow(ctx, flow)
	if err != nil {
		t.Fatal(err)
	}

	// Create a request node
	node := mnnode.MNode{
		ID:        nodeID,
		FlowID:    flowID,
		Name:      "Test Node",
		NodeKind:  mnnode.NODE_KIND_REQUEST,
		PositionX: 100,
		PositionY: 100,
	}
	err = ns.CreateNode(ctx, node)
	if err != nil {
		t.Fatal(err)
	}

	// Create the request sub-node
	nodeRequest := mnrequest.MNRequest{
		FlowNodeID: nodeID,
	}
	err = nrs.CreateNodeRequest(ctx, nodeRequest)
	if err != nil {
		t.Fatal(err)
	}

	// Create RPC service - use the variable directly as it implements the interface
	nodeService := rnode.NewNodeServiceRPC(db, us, fs, *nis, nrs, nfls, nlfes, ns, nss, jsns, ias, iaes, eqs, ehs, brs, bfs, bues, nes)

	// Test NodeGet
	req := connect.NewRequest(&nodev1.NodeGetRequest{
		NodeId: nodeID.Bytes(),
	})

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := nodeService.NodeGet(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}

	msg := resp.Msg
	if !bytes.Equal(msg.NodeId, nodeID.Bytes()) {
		t.Errorf("expected nodeID %v, got %v", nodeID.Bytes(), msg.NodeId)
	}

	if msg.Name != "Test Node" {
		t.Errorf("expected name 'Test Node', got %s", msg.Name)
	}

	if msg.Kind != nodev1.NodeKind_NODE_KIND_REQUEST {
		t.Errorf("expected kind REQUEST, got %v", msg.Kind)
	}
}

func TestNodeList(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	// Create all required services
	fs := sflow.New(queries)
	us := suser.New(queries)
	ns := snode.New(queries)
	nis := snodeif.New(queries)
	nrs := snoderequest.New(queries)
	nfls := snodefor.New(queries)
	nlfes := snodeforeach.New(queries)
	nss := snodenoop.New(queries)
	jsns := snodejs.New(queries)
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	eqs := sexamplequery.New(queries)
	ehs := sexampleheader.New(queries)
	brs := sbodyraw.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)
	nes := snodeexecution.New(queries)

	// Create base test data
	wsID := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()
	base.GetBaseServices().CreateTempCollection(t, ctx, wsID,
		wsuserID, userID, baseCollectionID)

	// Create a flow
	flowID := idwrap.NewNow()
	nodeID := idwrap.NewNow()

	// Create a flow
	flow := mflow.Flow{
		ID:          flowID,
		Name:        "Test Flow",
		WorkspaceID: wsID,
	}
	err := fs.CreateFlow(ctx, flow)
	if err != nil {
		t.Fatal(err)
	}

	// Create a request node
	node := mnnode.MNode{
		ID:        nodeID,
		FlowID:    flowID,
		Name:      "Test Node",
		NodeKind:  mnnode.NODE_KIND_REQUEST,
		PositionX: 100,
		PositionY: 100,
	}
	err = ns.CreateNode(ctx, node)
	if err != nil {
		t.Fatal(err)
	}

	// Create the request sub-node
	nodeRequest := mnrequest.MNRequest{
		FlowNodeID: nodeID,
	}
	err = nrs.CreateNodeRequest(ctx, nodeRequest)
	if err != nil {
		t.Fatal(err)
	}

	// Create RPC service
	nodeService := rnode.NewNodeServiceRPC(db, us, fs, *nis, nrs, nfls, nlfes, ns, nss, jsns, ias, iaes, eqs, ehs, brs, bfs, bues, nes)

	// Test NodeList
	req := connect.NewRequest(&nodev1.NodeListRequest{
		FlowId: flowID.Bytes(),
	})

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := nodeService.NodeList(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}

	msg := resp.Msg
	if len(msg.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(msg.Items))
	}

	nodeItem := msg.Items[0]
	if !bytes.Equal(nodeItem.NodeId, nodeID.Bytes()) {
		t.Errorf("expected nodeID %v, got %v", nodeID.Bytes(), nodeItem.NodeId)
	}

	if nodeItem.Kind != nodev1.NodeKind_NODE_KIND_REQUEST {
		t.Errorf("expected kind REQUEST, got %v", nodeItem.Kind)
	}
}

func TestNodeCreate(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	// Create all required services
	fs := sflow.New(queries)
	us := suser.New(queries)
	ns := snode.New(queries)
	nis := snodeif.New(queries)
	nrs := snoderequest.New(queries)
	nfls := snodefor.New(queries)
	nlfes := snodeforeach.New(queries)
	nss := snodenoop.New(queries)
	jsns := snodejs.New(queries)
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	eqs := sexamplequery.New(queries)
	ehs := sexampleheader.New(queries)

	brs := sbodyraw.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)
	nes := snodeexecution.New(queries)

	// Create base test data
	wsID := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()
	base.GetBaseServices().CreateTempCollection(t, ctx, wsID,
		wsuserID, userID, baseCollectionID)

	// Create a flow
	flowID := idwrap.NewNow()

	// Create a flow
	flow := mflow.Flow{
		ID:          flowID,
		Name:        "Test Flow",
		WorkspaceID: wsID,
	}
	err := fs.CreateFlow(ctx, flow)
	if err != nil {
		t.Fatal(err)
	}

	// Create RPC service
	nodeService := rnode.NewNodeServiceRPC(db, us, fs, *nis, nrs, nfls, nlfes, ns, nss, jsns, ias, iaes, eqs, ehs, brs, bfs, bues, nes)

	// Test NodeCreate with a REQUEST type
	position := &nodev1.Position{X: 200, Y: 300}
	nodeName := "New Request Node"

	req := connect.NewRequest(&nodev1.NodeCreateRequest{
		FlowId:   flowID.Bytes(),
		Position: position,
		Kind:     nodev1.NodeKind_NODE_KIND_REQUEST,
		Name:     nodeName,
		Request:  &nodev1.NodeRequest{},
	})

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := nodeService.NodeCreate(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	if resp.Msg == nil {
		t.Fatal("resp.Msg is nil")
	}

	// Verify node was created
	createdNodeID, err := idwrap.NewFromBytes(resp.Msg.NodeId)
	if err != nil {
		t.Fatal(err)
	}

	createdNode, err := ns.GetNode(ctx, createdNodeID)
	if err != nil {
		t.Fatal(err)
	}

	if createdNode.Name != nodeName {
		t.Errorf("expected name %s, got %s", nodeName, createdNode.Name)
	}

	if createdNode.NodeKind != mnnode.NODE_KIND_REQUEST {
		t.Errorf("expected kind REQUEST, got %d", createdNode.NodeKind)
	}

	// Verify request sub-node
	nodeRequest, err := nrs.GetNodeRequest(ctx, createdNodeID)
	if err != nil {
		t.Fatal(err)
	}

	if nodeRequest == nil {
		t.Fatal("nodeRequest is nil")
	}

	if nodeRequest.FlowNodeID != createdNodeID {
		t.Errorf("Expected FlowNodeID %v, got %v", createdNodeID, nodeRequest.FlowNodeID)
	}
}

func TestNodeUpdate(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	// Create all required services
	// Create all required services
	fs := sflow.New(queries)
	us := suser.New(queries)
	ns := snode.New(queries)
	nis := snodeif.New(queries)
	nrs := snoderequest.New(queries)
	nfls := snodefor.New(queries)
	nlfes := snodeforeach.New(queries)
	nss := snodenoop.New(queries)
	jsns := snodejs.New(queries)
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	eqs := sexamplequery.New(queries)
	ehs := sexampleheader.New(queries)

	brs := sbodyraw.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)
	nes := snodeexecution.New(queries)

	// Create base test data
	wsID := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()
	base.GetBaseServices().CreateTempCollection(t, ctx, wsID,
		wsuserID, userID, baseCollectionID)

	// Create a flow
	flowID := idwrap.NewNow()
	nodeID := idwrap.NewNow()

	// Create a flow
	flow := mflow.Flow{
		ID:          flowID,
		Name:        "Test Flow",
		WorkspaceID: wsID,
	}
	err := fs.CreateFlow(ctx, flow)
	if err != nil {
		t.Fatal(err)
	}

	// Create a request node
	initialName := "Test Node"
	initialPosX := float64(100)
	initialPosY := float64(100)
	node := mnnode.MNode{
		ID:        nodeID,
		FlowID:    flowID,
		Name:      initialName,
		NodeKind:  mnnode.NODE_KIND_REQUEST,
		PositionX: initialPosX,
		PositionY: initialPosY,
	}
	err = ns.CreateNode(ctx, node)
	if err != nil {
		t.Fatal(err)
	}

	// Create the request sub-node
	nodeRequest := mnrequest.MNRequest{
		FlowNodeID: nodeID,
	}
	err = nrs.CreateNodeRequest(ctx, nodeRequest)
	if err != nil {
		t.Fatal(err)
	}

	// Create RPC service
	nodeService := rnode.NewNodeServiceRPC(db, us, fs, *nis, nrs, nfls, nlfes, ns, nss, jsns, ias, iaes, eqs, ehs, brs, bfs, bues, nes)

	// Test NodeUpdate
	updatedName := "Updated Node Name"
	updatedPosX := float32(200)
	updatedPosY := float32(300)
	position := &nodev1.Position{X: updatedPosX, Y: updatedPosY}

	req := connect.NewRequest(&nodev1.NodeUpdateRequest{
		NodeId:   nodeID.Bytes(),
		Name:     &updatedName,
		Position: position,
		Request:  &nodev1.NodeRequest{}, // Include empty request to maintain node type
	})

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := nodeService.NodeUpdate(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	// Verify node was updated
	updatedNode, err := ns.GetNode(ctx, nodeID)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the name was updated
	if updatedNode.Name != updatedName {
		t.Errorf("expected name to be updated to %s, got %s", updatedName, updatedNode.Name)
	}

	// Check that the position was updated
	if float32(updatedNode.PositionX) != updatedPosX {
		t.Errorf("expected PositionX to be updated to %f, got %f", updatedPosX, updatedNode.PositionX)
	}

	if float32(updatedNode.PositionY) != updatedPosY {
		t.Errorf("expected PositionY to be updated to %f, got %f", updatedPosY, updatedNode.PositionY)
	}

	// Verify node type remains unchanged
	if updatedNode.NodeKind != mnnode.NODE_KIND_REQUEST {
		t.Errorf("expected NodeKind to remain %d, got %d", mnnode.NODE_KIND_REQUEST, updatedNode.NodeKind)
	}

	// Verify request sub-node still exists and is associated with the node
	nodeRequestTemp, err := nrs.GetNodeRequest(ctx, nodeID)
	if err != nil {
		t.Fatal(err)
	}
	nodeRequest = *nodeRequestTemp

	if nodeRequest.FlowNodeID != nodeID {
		t.Errorf("Expected FlowNodeID %v, got %v", nodeID, nodeRequest.FlowNodeID)
	}
}

func TestNodeDelete(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	// Create all required services
	fs := sflow.New(queries)
	us := suser.New(queries)
	ns := snode.New(queries)
	nis := snodeif.New(queries)
	nrs := snoderequest.New(queries)
	nfls := snodefor.New(queries)
	nlfes := snodeforeach.New(queries)
	nss := snodenoop.New(queries)
	jsns := snodejs.New(queries)
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	eqs := sexamplequery.New(queries)
	ehs := sexampleheader.New(queries)

	brs := sbodyraw.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)
	nes := snodeexecution.New(queries)

	// Create base test data
	wsID := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()
	base.GetBaseServices().CreateTempCollection(t, ctx, wsID,
		wsuserID, userID, baseCollectionID)

	// Create test data
	flowID := idwrap.NewNow()
	nodeID := idwrap.NewNow()

	// Create a flow
	flow := mflow.Flow{
		ID:          flowID,
		Name:        "Test Flow",
		WorkspaceID: wsID,
	}
	err := fs.CreateFlow(ctx, flow)
	if err != nil {
		t.Fatal(err)
	}

	// Create a request node
	node := mnnode.MNode{
		ID:        nodeID,
		FlowID:    flowID,
		Name:      "Test Node",
		NodeKind:  mnnode.NODE_KIND_REQUEST,
		PositionX: 100,
		PositionY: 100,
	}
	err = ns.CreateNode(ctx, node)
	if err != nil {
		t.Fatal(err)
	}

	// Create the request sub-node
	nodeRequest := mnrequest.MNRequest{
		FlowNodeID: nodeID,
	}
	err = nrs.CreateNodeRequest(ctx, nodeRequest)
	if err != nil {
		t.Fatal(err)
	}

	// Create RPC service
	nodeService := rnode.NewNodeServiceRPC(db, us, fs, *nis, nrs, nfls, nlfes, ns, nss, jsns, ias, iaes, eqs, ehs, brs, bfs, bues, nes)

	// Test NodeDelete
	req := connect.NewRequest(&nodev1.NodeDeleteRequest{
		NodeId: nodeID.Bytes(),
	})

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := nodeService.NodeDelete(authedCtx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("resp is nil")
	}

	// Verify node was deleted
	_, err = ns.GetNode(ctx, nodeID)
	if err == nil {
		t.Fatal("expected error, node should be deleted")
	}
}

// collectStreamMock captures streamed FlowRun responses for assertions
type collectStreamMock struct{
    mu    sync.Mutex
    items []*flowv1.FlowRunResponse
}

func (s *collectStreamMock) Send(resp *flowv1.FlowRunResponse) error {
    s.mu.Lock()
    s.items = append(s.items, resp)
    s.mu.Unlock()
    return nil
}

// TestFlowRun_PersistsNodeErrorAndServesToClient verifies that when a node errors during a run,
// the error is persisted to node_execution and surfaced on the flow stream.
func TestFlowRun_PersistsNodeErrorAndServesToClient(t *testing.T) {
    ctx := context.Background()
    base := testutil.CreateBaseDB(ctx, t)
    queries := base.Queries
    db := base.DB

    // Core services (mirror server wiring)
    ws := sworkspace.New(queries) // workspace service for rflow constructor
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

    logChanMap := logconsole.NewLogChanMapWith(1000)

    flowSrv := rflow.New(db, ws, us, ts,
        fs, fts, fes, fvs,
        ias, es, qs, hs,
        brs, bfs, bues,
        ers, erhs, as, ars,
        ns, rns, fns, fens,
        sns, *ins, jsns, nes,
        logChanMap,
    )

    // Minimal domain data
    wsID := idwrap.NewNow()
    wsUserID := idwrap.NewNow()
    userID := idwrap.NewNow()
    collectionID := idwrap.NewNow()
    base.GetBaseServices().CreateTempCollection(t, ctx, wsID, wsUserID, userID, collectionID)

    // Flow with: start -> if(bad expr) -> true/false
    flowID := idwrap.NewNow()
    if err := fs.CreateFlow(ctx, mflow.Flow{ID: flowID, WorkspaceID: wsID, Name: "error_e2e"}); err != nil {
        t.Fatal(err)
    }

    startID := idwrap.NewNow()
    if err := ns.CreateNode(ctx, mnnode.MNode{ID: startID, FlowID: flowID, Name: "Start", NodeKind: mnnode.NODE_KIND_NO_OP}); err != nil {
        t.Fatal(err)
    }
    if err := sns.CreateNodeNoop(ctx, mnnoop.NoopNode{FlowNodeID: startID, Type: mnnoop.NODE_NO_OP_KIND_START}); err != nil {
        t.Fatal(err)
    }

    ifID := idwrap.NewNow()
    if err := ns.CreateNode(ctx, mnnode.MNode{ID: ifID, FlowID: flowID, Name: "IfBad", NodeKind: mnnode.NODE_KIND_CONDITION}); err != nil {
        t.Fatal(err)
    }
    // Intentionally invalid expression to force evaluation error
    if err := ins.CreateNodeIf(ctx, mnif.MNIF{FlowNodeID: ifID, Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "this is not valid expr"}}}); err != nil {
        t.Fatal(err)
    }

    trueID := idwrap.NewNow()
    if err := ns.CreateNode(ctx, mnnode.MNode{ID: trueID, FlowID: flowID, Name: "True", NodeKind: mnnode.NODE_KIND_NO_OP}); err != nil {
        t.Fatal(err)
    }
    if err := sns.CreateNodeNoop(ctx, mnnoop.NoopNode{FlowNodeID: trueID, Type: mnnoop.NODE_NO_OP_KIND_THEN}); err != nil {
        t.Fatal(err)
    }

    falseID := idwrap.NewNow()
    if err := ns.CreateNode(ctx, mnnode.MNode{ID: falseID, FlowID: flowID, Name: "False", NodeKind: mnnode.NODE_KIND_NO_OP}); err != nil {
        t.Fatal(err)
    }
    if err := sns.CreateNodeNoop(ctx, mnnoop.NoopNode{FlowNodeID: falseID, Type: mnnoop.NODE_NO_OP_KIND_ELSE}); err != nil {
        t.Fatal(err)
    }

    // Edges
    if err := fes.CreateEdge(ctx, edge.Edge{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startID, TargetID: ifID, SourceHandler: edge.HandleUnspecified}); err != nil {
        t.Fatal(err)
    }
    if err := fes.CreateEdge(ctx, edge.Edge{ID: idwrap.NewNow(), FlowID: flowID, SourceID: ifID, TargetID: trueID, SourceHandler: edge.HandleThen}); err != nil {
        t.Fatal(err)
    }
    if err := fes.CreateEdge(ctx, edge.Edge{ID: idwrap.NewNow(), FlowID: flowID, SourceID: ifID, TargetID: falseID, SourceHandler: edge.HandleElse}); err != nil {
        t.Fatal(err)
    }

    // Run the flow
    req := connect.NewRequest(&flowv1.FlowRunRequest{FlowId: flowID.Bytes(), EnvironmentId: idwrap.NewNow().Bytes()})
    stream := &collectStreamMock{}
    // Register user's log channel so FlowRun can send logs without error
    _ = logChanMap.AddLogChannel(userID)
    authed := mwauth.CreateAuthedContext(ctx, userID)

    // Flow may return an error when a node fails; we still want to verify
    // that the error is persisted and surfaced via stream.
    _ = flowSrv.FlowRunAdHoc(authed, req, stream)

    // Verify node_execution contains a FAILURE with error for the IF node
    exec, err := nes.GetLatestNodeExecutionByNodeID(ctx, ifID)
    if err != nil {
        t.Fatal(err)
    }
    if exec == nil {
        t.Fatal("expected a node execution for IF node, got nil")
    }
    if exec.State != mnnode.NODE_STATE_FAILURE {
        t.Fatalf("expected FAILURE state, got %v", exec.State)
    }
    if exec.Error == nil || *exec.Error == "" {
        t.Fatalf("expected non-empty error on node execution, got %+v", exec.Error)
    }
    // Basic sanity the error came from condition evaluation
    if !strings.Contains(strings.ToLower(*exec.Error), "evaluate condition expression") &&
       !strings.Contains(strings.ToLower(*exec.Error), "normalize condition expression") {
        t.Fatalf("unexpected error message: %s", *exec.Error)
    }
    if exec.CompletedAt == nil {
        t.Fatalf("expected CompletedAt to be set")
    }

    // Best-effort: stream may finish early due to error. If a node update for the IF node
    // was delivered, ensure Info is included. Otherwise, persistence check above suffices.
    stream.mu.Lock()
    for _, it := range stream.items {
        if it != nil && it.Node != nil && bytes.Equal(it.Node.NodeId, ifID.Bytes()) {
            if it.Node.State == nodev1.NodeState_NODE_STATE_FAILURE {
                if it.Node.Info == nil || *it.Node.Info == "" {
                    t.Fatalf("expected node stream Info to include error message")
                }
            }
            break
        }
    }
    stream.mu.Unlock()

    // Now verify NodeList returns info for the failing node
    nodeRPC := rnode.NewNodeServiceRPC(db, us, fs, *ins, rns, fns, fens, ns, sns, jsns, ias, es, qs, hs, brs, bfs, bues, nes)
    listReq := connect.NewRequest(&nodev1.NodeListRequest{FlowId: flowID.Bytes()})
    listResp, err := nodeRPC.NodeList(mwauth.CreateAuthedContext(ctx, userID), listReq)
    if err != nil {
        t.Fatalf("node list failed: %v", err)
    }
    foundInfo := false
    for _, it := range listResp.Msg.Items {
        if bytes.Equal(it.NodeId, ifID.Bytes()) {
            if it.State != nodev1.NodeState_NODE_STATE_FAILURE {
                t.Fatalf("expected IF node state FAILURE, got %v", it.State)
            }
            if it.Info == nil || *it.Info == "" {
                t.Fatalf("expected NodeList info to include error message for IF node")
            }
            foundInfo = true
            break
        }
    }
    if !foundInfo {
        t.Fatalf("IF node not found in NodeList items")
    }
}
