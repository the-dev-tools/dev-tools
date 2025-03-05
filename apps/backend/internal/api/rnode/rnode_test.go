package rnode_test

import (
	"bytes"
	"context"
	"testing"
	"the-dev-tools/backend/internal/api/middleware/mwauth"
	"the-dev-tools/backend/internal/api/rnode"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mflow"
	"the-dev-tools/backend/pkg/model/mnnode"
	"the-dev-tools/backend/pkg/model/mnnode/mnrequest"
	"the-dev-tools/backend/pkg/service/sbodyform"
	"the-dev-tools/backend/pkg/service/sbodyraw"
	"the-dev-tools/backend/pkg/service/sbodyurl"
	"the-dev-tools/backend/pkg/service/sexampleheader"
	"the-dev-tools/backend/pkg/service/sexamplequery"
	"the-dev-tools/backend/pkg/service/sflow"
	"the-dev-tools/backend/pkg/service/sitemapi"
	"the-dev-tools/backend/pkg/service/sitemapiexample"
	"the-dev-tools/backend/pkg/service/snode"
	"the-dev-tools/backend/pkg/service/snodefor"
	"the-dev-tools/backend/pkg/service/snodeforeach"
	"the-dev-tools/backend/pkg/service/snodeif"
	"the-dev-tools/backend/pkg/service/snodenoop"
	"the-dev-tools/backend/pkg/service/snoderequest"
	"the-dev-tools/backend/pkg/service/suser"
	"the-dev-tools/backend/pkg/testutil"
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
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	eqs := sexamplequery.New(queries)
	ehs := sexampleheader.New(queries)

	brs := sbodyraw.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)

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
	nodeService := rnode.NewNodeServiceRPC(db, us, fs, *nis, nrs, nfls, nlfes, ns, nss, ias, iaes, eqs, ehs, brs, bfs, bues)

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
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	eqs := sexamplequery.New(queries)
	ehs := sexampleheader.New(queries)

	brs := sbodyraw.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)

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
	nodeService := rnode.NewNodeServiceRPC(db, us, fs, *nis, nrs, nfls, nlfes, ns, nss, ias, iaes, eqs, ehs, brs, bfs, bues)

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
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	eqs := sexamplequery.New(queries)
	ehs := sexampleheader.New(queries)

	brs := sbodyraw.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)

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
	nodeService := rnode.NewNodeServiceRPC(db, us, fs, *nis, nrs, nfls, nlfes, ns, nss, ias, iaes, eqs, ehs, brs, bfs, bues)

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
	fs := sflow.New(queries)
	us := suser.New(queries)
	ns := snode.New(queries)
	nis := snodeif.New(queries)
	nrs := snoderequest.New(queries)
	nfls := snodefor.New(queries)
	nlfes := snodeforeach.New(queries)
	nss := snodenoop.New(queries)
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	eqs := sexamplequery.New(queries)
	ehs := sexampleheader.New(queries)

	brs := sbodyraw.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)

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
	nodeService := rnode.NewNodeServiceRPC(db, us, fs, *nis, nrs, nfls, nlfes, ns, nss, ias, iaes, eqs, ehs, brs, bfs, bues)

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
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	eqs := sexamplequery.New(queries)
	ehs := sexampleheader.New(queries)

	brs := sbodyraw.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)

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
	nodeService := rnode.NewNodeServiceRPC(db, us, fs, *nis, nrs, nfls, nlfes, ns, nss, ias, iaes, eqs, ehs, brs, bfs, bues)

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
