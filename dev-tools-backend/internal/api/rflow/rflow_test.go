package rflow_test

import (
	"context"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/internal/api/rflow"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mflow"
	"dev-tools-backend/pkg/model/mflowtag"
	"dev-tools-backend/pkg/model/mtag"
	"dev-tools-backend/pkg/service/sflow"
	"dev-tools-backend/pkg/service/sflowtag"
	"dev-tools-backend/pkg/service/stag"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/service/sworkspace"
	"dev-tools-backend/pkg/testutil"
	flowv1 "dev-tools-spec/dist/buf/go/flow/v1"
	"testing"

	"connectrpc.com/connect"
)

func TestListFlow(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	defer queries.Close()
	db := base.DB

	ws := sworkspace.New(queries)
	us := suser.New(queries)
	ts := stag.New(queries)
	fs := sflow.New(queries)
	fts := sflowtag.New(queries)

	serviceRPC := rflow.New(db, ws, us, ts, fs, fts)
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
	taggedFlowData := mflow.Flow{
		ID:          taggedTestFlowID,
		WorkspaceID: wsID,
		Name:        "test",
	}
	err = fs.CreateFlow(ctx, taggedFlowData)
	testutil.AssertFatal(t, nil, err)
	nonTaggedFlowData := mflow.Flow{
		ID:          idwrap.NewNow(),
		WorkspaceID: wsID,
		Name:        "test",
	}
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
	if nonTaggedFlowData.ID.Compare(flowID) != 0 {
		t.Fatalf("failed: id mismatch")
	}
	testutil.Assert(t, "test", flow2.Name)
}

func TestGetFlow(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	defer queries.Close()
	db := base.DB

	ws := sworkspace.New(queries)
	us := suser.New(queries)
	ts := stag.New(queries)
	fs := sflow.New(queries)
	fts := sflowtag.New(queries)

	serviceRPC := rflow.New(db, ws, us, ts, fs, fts)
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

	if testFlowID.Compare(respFlowID) != 0 {
		t.Fatalf("failed: id mismatch")
	}

	testutil.Assert(t, flowData.Name, msg.Name)
	testutil.Assert(t, testFlowID, respFlowID)
}

func TestCreateFlow(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	defer queries.Close()
	db := base.DB

	ws := sworkspace.New(queries)
	us := suser.New(queries)
	ts := stag.New(queries)
	fs := sflow.New(queries)
	fts := sflowtag.New(queries)

	serviceRPC := rflow.New(db, ws, us, ts, fs, fts)
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
	defer queries.Close()
	db := base.DB

	ws := sworkspace.New(queries)
	us := suser.New(queries)
	ts := stag.New(queries)
	fs := sflow.New(queries)
	fts := sflowtag.New(queries)

	serviceRPC := rflow.New(db, ws, us, ts, fs, fts)
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

	const UpdatedName = "test2"

	req := connect.NewRequest(
		&flowv1.FlowUpdateRequest{
			FlowId: testFlowID.Bytes(),
			Name:   UpdatedName,
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
	defer queries.Close()
	db := base.DB
	ws := sworkspace.New(queries)
	us := suser.New(queries)
	ts := stag.New(queries)
	fs := sflow.New(queries)
	fts := sflowtag.New(queries)
	serviceRPC := rflow.New(db, ws, us, ts, fs, fts)
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
	req := connect.NewRequest(
		&flowv1.FlowDeleteRequest{
			FlowId: testFlowID.Bytes(),
		},
	)
	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	resp, err := serviceRPC.FlowDelete(authedCtx, req)
	testutil.AssertFatal(t, nil, err)
	testutil.AssertNotFatal(t, nil, resp.Msg)
}
