package rflow_test

import (
	"context"
	"testing"
	"the-dev-tools/db/pkg/sqlc"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rflow"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logconsole"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowtag"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mtag"
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
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/snodeexecution"
	"the-dev-tools/server/pkg/service/stag"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/testutil"
	flowv1 "the-dev-tools/spec/dist/buf/go/flow/v1"

	"connectrpc.com/connect"
)

func TestListFlow(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	defer sqlc.CloseQueriesAndLog(queries)

	db := base.DB

	ws := sworkspace.New(queries)
	us := suser.New(queries)
	ts := stag.New(queries)

	// flow
	fs := sflow.New(queries)
	fts := sflowtag.New(queries)
	fes := sedge.New(queries)
	fvs := sflowvariable.New(queries)

	// req
	ias := sitemapi.New(queries)
	es := sitemapiexample.New(queries)
	qs := sexamplequery.New(queries)
	hs := sexampleheader.New(queries)

	// body
	brs := sbodyraw.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)

	// repsonse
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	as := sassert.New(queries)
	ars := sassertres.New(queries)

	// sub nodes
	ns := snode.New(queries)
	rns := snoderequest.New(queries)
	flns := snodefor.New(queries)
	fens := snodeforeach.New(queries)
	sns := snodenoop.New(queries)
	ins := snodeif.New(queries)
	jsns := snodejs.New(queries)
	nes := snodeexecution.New(queries)

	logChanMap := logconsole.NewLogChanMapWith(10000)

	serviceRPC := rflow.New(db, ws, us, ts,
		// flow
		fs, fts, fes, fvs,
		// req
		ias, es, qs, hs,
		// body
		brs, bfs, bues,
		// resp
		ers, erhs, as, ars,
		// sub nodes
		ns, rns, flns, fens,
		sns, *ins, jsns, nes,
		logChanMap)
	wsID := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, wsID,
		wsuserID, userID, baseCollectionID)

	testTagID := idwrap.NewNow()
	tagData := mtag.Tag{
		ID:          testTagID,
		WorkspaceID: wsID,
		Name:        "test",
		Color:       uint8(5),
	}

	err := ts.CreateTag(ctx, tagData)
	testutil.AssertFatal(t, nil, err)

	taggedTestFlowID := idwrap.NewNow()
	nonTaggedTestFlowID := idwrap.NewNow()

	taggedFlowData := mflow.Flow{
		ID:          taggedTestFlowID,
		WorkspaceID: wsID,
		Name:        tagData.Name,
	}

	nonTaggedFlowData := mflow.Flow{
		ID:          nonTaggedTestFlowID,
		WorkspaceID: wsID,
		Name:        tagData.Name,
	}

	err = fs.CreateFlow(ctx, taggedFlowData)
	testutil.AssertFatal(t, nil, err)

	err = fs.CreateFlow(ctx, nonTaggedFlowData)
	testutil.AssertFatal(t, nil, err)

	flowTagID := idwrap.NewNow()
	flowTagData := mflowtag.FlowTag{
		ID:     flowTagID,
		FlowID: taggedTestFlowID,
		TagID:  testTagID,
	}

	err = fts.CreateFlowTag(ctx, flowTagData)
	testutil.AssertFatal(t, nil, err)

	// List by tag
	req := connect.NewRequest(
		&flowv1.FlowListRequest{
			WorkspaceId: wsID.Bytes(),
			TagId:       testTagID.Bytes(),
		},
	)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	resp, err := serviceRPC.FlowList(authedCtx, req)
	testutil.AssertFatal(t, nil, err)
	testutil.AssertNotFatal(t, nil, resp.Msg)
	msg := resp.Msg
	flows := msg.Items
	testutil.Assert(t, 1, len(flows))
	flow1 := flows[0]
	flowID, err := idwrap.NewFromBytes(flow1.FlowId)
	testutil.AssertFatal(t, nil, err)
	if taggedTestFlowID.Compare(flowID) != 0 {
		t.Fatalf("failed: id mismatch")
	}
	testutil.Assert(t, "test", flow1.Name)

	// List Without Tag (all flows)
	req = connect.NewRequest(
		&flowv1.FlowListRequest{
			WorkspaceId: wsID.Bytes(),
			TagId:       nil,
		},
	)

	resp, err = serviceRPC.FlowList(authedCtx, req)
	testutil.AssertFatal(t, nil, err)
	testutil.AssertNotFatal(t, nil, resp.Msg)
	msg = resp.Msg
	flows = msg.Items
	testutil.Assert(t, 2, len(flows))

	flow1 = flows[0]
	flow2 := flows[1]

	flowID, err = idwrap.NewFromBytes(flow1.FlowId)
	testutil.AssertFatal(t, nil, err)
	if taggedTestFlowID.Compare(flowID) != 0 {
		t.Fatalf("failed: id mismatch")
	}
	testutil.Assert(t, "test", flow1.Name)
	flowID, err = idwrap.NewFromBytes(flow2.FlowId)
	testutil.AssertFatal(t, nil, err)
	if nonTaggedTestFlowID.Compare(flowID) != 0 {
		t.Fatalf("failed: id mismatch")
	}
	testutil.Assert(t, "test", flow2.Name)
}

func TestGetFlow(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	defer sqlc.CloseQueriesAndLog(queries)
	db := base.DB

	ws := sworkspace.New(queries)
	us := suser.New(queries)
	ts := stag.New(queries)

	// flow
	fs := sflow.New(queries)
	fts := sflowtag.New(queries)
	fes := sedge.New(queries)
	fvs := sflowvariable.New(queries)

	// req
	ias := sitemapi.New(queries)
	es := sitemapiexample.New(queries)
	qs := sexamplequery.New(queries)
	hs := sexampleheader.New(queries)

	// body
	brs := sbodyraw.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)

	// repsonse
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	as := sassert.New(queries)
	ars := sassertres.New(queries)

	// sub nodes
	ns := snode.New(queries)
	rns := snoderequest.New(queries)
	flns := snodefor.New(queries)
	fens := snodeforeach.New(queries)
	sns := snodenoop.New(queries)
	ins := snodeif.New(queries)
	jsns := snodejs.New(queries)
	nes := snodeexecution.New(queries)

	logChanMap := logconsole.NewLogChanMapWith(10000)

	serviceRPC := rflow.New(db, ws, us, ts,
		// flow
		fs, fts, fes, fvs,
		// req
		ias, es, qs, hs,
		// body
		brs, bfs, bues,
		// resp
		ers, erhs, as, ars,
		// sub nodes
		ns, rns, flns, fens,
		sns, *ins, jsns, nes,
		logChanMap)
	wsID := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, wsID,
		wsuserID, userID, baseCollectionID)

	testTagID := idwrap.NewNow()
	tagData := mtag.Tag{
		ID:          testTagID,
		WorkspaceID: wsID,
		Name:        "test",
		Color:       uint8(5),
	}

	err := ts.CreateTag(ctx, tagData)
	testutil.AssertFatal(t, nil, err)

	testFlowID := idwrap.NewNow()
	flowData := mflow.Flow{
		ID:          testFlowID,
		WorkspaceID: wsID,
		Name:        "test",
	}

	err = fs.CreateFlow(ctx, flowData)
	testutil.AssertFatal(t, nil, err)

	flowTagID := idwrap.NewNow()
	flowTagData := mflowtag.FlowTag{
		ID:     flowTagID,
		FlowID: testFlowID,
		TagID:  testTagID,
	}

	err = fts.CreateFlowTag(ctx, flowTagData)
	testutil.AssertFatal(t, nil, err)

	req := connect.NewRequest(
		&flowv1.FlowGetRequest{
			FlowId: testFlowID.Bytes(),
		},
	)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	resp, err := serviceRPC.FlowGet(authedCtx, req)
	testutil.AssertFatal(t, nil, err)
	testutil.AssertNotFatal(t, nil, resp.Msg)
	msg := resp.Msg
	if msg.FlowId == nil {
		t.Fatalf("failed: invalid response")
	}
	respFlowID, err := idwrap.NewFromBytes(msg.FlowId)
	if err != nil {
		t.Fatal(err)
	}

	testutil.Assert(t, flowData.Name, msg.Name)
	testutil.Assert(t, testFlowID, respFlowID)
}

func TestCreateFlow(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	defer sqlc.CloseQueriesAndLog(queries)
	db := base.DB

	ws := sworkspace.New(queries)
	us := suser.New(queries)
	ts := stag.New(queries)

	// flow
	fs := sflow.New(queries)
	fts := sflowtag.New(queries)
	fes := sedge.New(queries)
	fvs := sflowvariable.New(queries)

	// req
	ias := sitemapi.New(queries)
	es := sitemapiexample.New(queries)
	qs := sexamplequery.New(queries)
	hs := sexampleheader.New(queries)

	// body
	brs := sbodyraw.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)

	// repsonse
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	as := sassert.New(queries)
	ars := sassertres.New(queries)

	// sub nodes
	ns := snode.New(queries)
	rns := snoderequest.New(queries)
	flns := snodefor.New(queries)
	fens := snodeforeach.New(queries)
	sns := snodenoop.New(queries)
	ins := snodeif.New(queries)
	jsns := snodejs.New(queries)
	nes := snodeexecution.New(queries)

	logChanMap := logconsole.NewLogChanMapWith(10000)

	serviceRPC := rflow.New(db, ws, us, ts,
		// flow
		fs, fts, fes, fvs,
		// req
		ias, es, qs, hs,
		// body
		brs, bfs, bues,
		// resp
		ers, erhs, as, ars,
		// sub nodes
		ns, rns, flns, fens,
		sns, *ins, jsns, nes,
		logChanMap)
	wsID := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, wsID,
		wsuserID, userID, baseCollectionID)

	testTagID := idwrap.NewNow()
	tagData := mtag.Tag{
		ID:          testTagID,
		WorkspaceID: wsID,
		Name:        "test",
		Color:       uint8(5),
	}

	err := ts.CreateTag(ctx, tagData)
	testutil.AssertFatal(t, nil, err)

	const testFlowName = "test"

	req := connect.NewRequest(
		&flowv1.FlowCreateRequest{
			WorkspaceId: wsID.Bytes(),
			Name:        testFlowName,
		},
	)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	resp, err := serviceRPC.FlowCreate(authedCtx, req)
	testutil.AssertFatal(t, nil, err)
	testutil.AssertNotFatal(t, nil, resp.Msg)
	msg := resp.Msg
	if msg.FlowId == nil {
		t.Fatalf("failed: invalid response")
	}
	respFlowID, err := idwrap.NewFromBytes(msg.FlowId)
	if err != nil {
		t.Fatal(err)
	}

	flow, err := fs.GetFlow(ctx, respFlowID)
	if err != nil {
		t.Fatal(err)
	}

	testutil.Assert(t, testFlowName, flow.Name)
}

func TestUpdateFlow(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	defer sqlc.CloseQueriesAndLog(queries)
	db := base.DB

	ws := sworkspace.New(queries)
	us := suser.New(queries)
	ts := stag.New(queries)

	// flow
	fs := sflow.New(queries)
	fts := sflowtag.New(queries)
	fes := sedge.New(queries)
	fvs := sflowvariable.New(queries)

	// req
	ias := sitemapi.New(queries)
	es := sitemapiexample.New(queries)
	qs := sexamplequery.New(queries)
	hs := sexampleheader.New(queries)

	// body
	brs := sbodyraw.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)

	// repsonse
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	as := sassert.New(queries)
	ars := sassertres.New(queries)

	// sub nodes
	ns := snode.New(queries)
	rns := snoderequest.New(queries)
	flns := snodefor.New(queries)
	fens := snodeforeach.New(queries)
	sns := snodenoop.New(queries)
	ins := snodeif.New(queries)
	jsns := snodejs.New(queries)
	nes := snodeexecution.New(queries)

	logChanMap := logconsole.NewLogChanMapWith(10000)

	serviceRPC := rflow.New(db, ws, us, ts,
		// flow
		fs, fts, fes, fvs,
		// req
		ias, es, qs, hs,
		// body
		brs, bfs, bues,
		// resp
		ers, erhs, as, ars,
		// sub nodes
		ns, rns, flns, fens,
		sns, *ins, jsns, nes,
		logChanMap)
	wsID := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, wsID,
		wsuserID, userID, baseCollectionID)

	testTagID := idwrap.NewNow()
	tagData := mtag.Tag{
		ID:          testTagID,
		WorkspaceID: wsID,
		Name:        "test",
		Color:       uint8(5),
	}

	err := ts.CreateTag(ctx, tagData)
	testutil.AssertFatal(t, nil, err)

	testFlowID := idwrap.NewNow()
	flowRootData := mflow.Flow{
		ID:          testFlowID,
		WorkspaceID: wsID,
		Name:        "test",
	}

	err = fs.CreateFlow(ctx, flowRootData)
	testutil.AssertFatal(t, nil, err)

	UpdatedName := "test2"

	req := connect.NewRequest(
		&flowv1.FlowUpdateRequest{
			FlowId: testFlowID.Bytes(),
			Name:   &UpdatedName,
		},
	)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	resp, err := serviceRPC.FlowUpdate(authedCtx, req)
	testutil.AssertFatal(t, nil, err)
	testutil.AssertNotFatal(t, nil, resp.Msg)

	flow, err := fs.GetFlow(ctx, testFlowID)
	if err != nil {
		t.Fatal(err)
	}

	testutil.Assert(t, UpdatedName, flow.Name)
}

func TestDeleteFlow(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	defer sqlc.CloseQueriesAndLog(queries)
	db := base.DB

	ws := sworkspace.New(queries)
	us := suser.New(queries)
	ts := stag.New(queries)

	// flow
	fs := sflow.New(queries)
	fts := sflowtag.New(queries)
	fes := sedge.New(queries)
	fvs := sflowvariable.New(queries)

	// req
	ias := sitemapi.New(queries)
	es := sitemapiexample.New(queries)
	qs := sexamplequery.New(queries)
	hs := sexampleheader.New(queries)

	// body
	brs := sbodyraw.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)

	// repsonse
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	as := sassert.New(queries)
	ars := sassertres.New(queries)

	// sub nodes
	ns := snode.New(queries)
	rns := snoderequest.New(queries)
	flns := snodefor.New(queries)
	fens := snodeforeach.New(queries)
	sns := snodenoop.New(queries)
	ins := snodeif.New(queries)
	jsns := snodejs.New(queries)
	nes := snodeexecution.New(queries)

	logChanMap := logconsole.NewLogChanMapWith(10000)

	serviceRPC := rflow.New(db, ws, us, ts,
		// flow
		fs, fts, fes, fvs,
		// req
		ias, es, qs, hs,
		// body
		brs, bfs, bues,
		// resp
		ers, erhs, as, ars,
		// sub nodes
		ns, rns, flns, fens,
		sns, *ins, jsns, nes,
		logChanMap)

	wsID := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()
	base.GetBaseServices().CreateTempCollection(t, ctx, wsID,
		wsuserID, userID, baseCollectionID)
	testTagID := idwrap.NewNow()
	tagData := mtag.Tag{
		ID:          testTagID,
		WorkspaceID: wsID,
		Name:        "test",
		Color:       uint8(5),
	}
	err := ts.CreateTag(ctx, tagData)
	testutil.AssertFatal(t, nil, err)

	testFlowID := idwrap.NewNow()

	flowRootData := mflow.Flow{
		ID:          testFlowID,
		WorkspaceID: wsID,
		Name:        "test",
	}

	err = fs.CreateFlow(ctx, flowRootData)
	testutil.AssertFatal(t, nil, err)
	req := connect.NewRequest(
		&flowv1.FlowDeleteRequest{
			FlowId: testFlowID.Bytes(),
		},
	)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := serviceRPC.FlowDelete(authedCtx, req)
	testutil.AssertFatal(t, nil, err)
	testutil.AssertNotFatal(t, nil, resp.Msg)

	_, err = fs.GetFlow(ctx, testFlowID)
	testutil.AssertFatal(t, sflow.ErrNoFlowFound, err)
}

type ServerStreamingHandlerMock[I any] struct {
	SendStream func(*I)
}

func (s ServerStreamingHandlerMock[I]) Send(a *I) error {
	s.SendStream(a)
	return nil
}

func TestRunFlow(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	defer sqlc.CloseQueriesAndLog(queries)
	db := base.DB

	ws := sworkspace.New(queries)
	us := suser.New(queries)
	ts := stag.New(queries)

	// flow
	fs := sflow.New(queries)
	fts := sflowtag.New(queries)
	fes := sedge.New(queries)
	fvs := sflowvariable.New(queries)

	// req
	ias := sitemapi.New(queries)
	es := sitemapiexample.New(queries)
	qs := sexamplequery.New(queries)
	hs := sexampleheader.New(queries)

	// body
	brs := sbodyraw.New(queries)
	bfs := sbodyform.New(queries)
	bues := sbodyurl.New(queries)

	// repsonse
	ers := sexampleresp.New(queries)
	erhs := sexamplerespheader.New(queries)
	as := sassert.New(queries)
	ars := sassertres.New(queries)

	// sub nodes
	ns := snode.New(queries)
	rns := snoderequest.New(queries)
	flns := snodefor.New(queries)
	fens := snodeforeach.New(queries)
	sns := snodenoop.New(queries)
	ins := snodeif.New(queries)
	jsns := snodejs.New(queries)
	nes := snodeexecution.New(queries)

	logChanMap := logconsole.NewLogChanMapWith(10000)

	serviceRPC := rflow.New(db, ws, us, ts,
		// flow
		fs, fts, fes, fvs,
		// req
		ias, es, qs, hs,
		// body
		brs, bfs, bues,
		// resp
		ers, erhs, as, ars,
		// sub nodes
		ns, rns, flns, fens,
		sns, *ins, jsns, nes,
		logChanMap)

	wsID := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()
	base.GetBaseServices().CreateTempCollection(t, ctx, wsID,
		wsuserID, userID, baseCollectionID)
	testTagID := idwrap.NewNow()
	tagData := mtag.Tag{
		ID:          testTagID,
		WorkspaceID: wsID,
		Name:        "test",
		Color:       uint8(5),
	}
	err := ts.CreateTag(ctx, tagData)
	testutil.AssertFatal(t, nil, err)
	testFlowID := idwrap.NewNow()
	flowData := mflow.Flow{
		ID:          testFlowID,
		WorkspaceID: wsID,
		Name:        "test",
	}
	err = fs.CreateFlow(ctx, flowData)
	testutil.AssertFatal(t, nil, err)

	startNodeID := idwrap.NewNow()
	err = ns.CreateNode(ctx, mnnode.MNode{
		ID:        startNodeID,
		FlowID:    testFlowID,
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: 0,
		PositionY: 0,
	})
	testutil.AssertFatal(t, nil, err)

	err = sns.CreateNodeNoop(ctx, mnnoop.NoopNode{
		FlowNodeID: startNodeID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	})
	testutil.AssertFatal(t, nil, err)

	forNodeID := idwrap.NewNow()
	err = ns.CreateNode(ctx, mnnode.MNode{
		ID:        forNodeID,
		FlowID:    testFlowID,
		NodeKind:  mnnode.NODE_KIND_FOR,
		PositionX: 0,
		PositionY: 0,
	})
	testutil.AssertFatal(t, nil, err)
	err = flns.CreateNodeFor(ctx, mnfor.MNFor{
		FlowNodeID: forNodeID,
		IterCount:  0,
	})
	testutil.AssertFatal(t, nil, err)

	edge1 := edge.NewEdge(idwrap.NewNow(), startNodeID, forNodeID, edge.HandleUnspecified, edge.EdgeKindUnspecified)
	edges := []edge.Edge{edge1}

	for _, e := range edges {
		err = fes.CreateEdge(ctx, e)
		testutil.AssertFatal(t, nil, err)
	}

	req := connect.NewRequest(&flowv1.FlowRunRequest{
		FlowId:        testFlowID.Bytes(),
		EnvironmentId: idwrap.NewNow().Bytes(),
	})

	logChan := logChanMap.AddLogChannel(userID)

	stream := ServerStreamingHandlerMock[flowv1.FlowRunResponse]{
		SendStream: func(a *flowv1.FlowRunResponse) {
		},
	}

	go func() {
		for {
			_, ok := <-logChan
			if !ok {
				break
			}
		}
	}()

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	err = serviceRPC.FlowRunAdHoc(authedCtx, req, stream)

	testutil.Assert(t, nil, err)
}
