package rtag_test

import (
	"context"
	"testing"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rtag"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mtag"
	"the-dev-tools/server/pkg/service/stag"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/testutil"
	tagv1 "the-dev-tools/spec/dist/buf/go/tag/v1"

	"connectrpc.com/connect"
)

func TestGetTag(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	ws := sworkspace.New(queries)
	us := suser.New(queries)
	ts := stag.New(queries)

	serviceRPC := rtag.New(db, ws, us, ts)
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
	if err != nil {
		t.Error(err)
	}

	req := connect.NewRequest(
		&tagv1.TagGetRequest{
			TagId: testTagID.Bytes(),
		},
	)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	resp, err := serviceRPC.TagGet(authedCtx, req)
	testutil.AssertFatal(t, nil, err)
	testutil.AssertNotFatal(t, nil, resp.Msg)
	msg := resp.Msg
	if msg.TagId == nil {
		t.Fatalf("CollectionGet failed: invalid response")
	}
	respTagID, err := idwrap.NewFromBytes(msg.TagId)
	if err != nil {
		t.Fatal(err)
	}

	if testTagID.Compare(respTagID) != 0 {
		t.Fatalf("CollectionGet failed: id mismatch")
	}

	testutil.Assert(t, tagData.Name, msg.Name)
	testutil.Assert(t, tagData.Color, uint8(msg.Color))
	testutil.Assert(t, testTagID, respTagID)
}

func TestCreateTag(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	ws := sworkspace.New(queries)
	us := suser.New(queries)
	ts := stag.New(queries)

	serviceRPC := rtag.New(db, ws, us, ts)
	wsID := idwrap.NewNow()
	wsuserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	baseCollectionID := idwrap.NewNow()

	base.GetBaseServices().CreateTempCollection(t, ctx, wsID,
		wsuserID, userID, baseCollectionID)

	const ExpectedName = "test"
	const ExpectedColor = uint8(5)

	req := connect.NewRequest(
		&tagv1.TagCreateRequest{
			WorkspaceId: wsID.Bytes(),
			Name:        ExpectedName,
			Color:       tagv1.TagColor(ExpectedColor),
		},
	)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	resp, err := serviceRPC.TagCreate(authedCtx, req)
	testutil.AssertFatal(t, nil, err)
	testutil.AssertNotFatal(t, nil, resp.Msg)
	msg := resp.Msg
	if msg.TagId == nil {
		t.Fatalf("CollectionGet failed: invalid response")
	}
	respTagID, err := idwrap.NewFromBytes(msg.TagId)
	testutil.Assert(t, nil, err)
	if err != nil {
		t.Fatal(err)
	}

	dbTag, err := ts.GetTag(ctx, respTagID)
	testutil.AssertFatal(t, nil, err)

	testutil.Assert(t, ExpectedName, dbTag.Name)
	testutil.Assert(t, ExpectedColor, dbTag.Color)
}

func TestUpdateTag(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	ws := sworkspace.New(queries)
	us := suser.New(queries)
	ts := stag.New(queries)

	serviceRPC := rtag.New(db, ws, us, ts)
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
	if err != nil {
		t.Error(err)
	}

	UpdatedName := "updated"
	UpdatedColor := uint8(7)
	UpdatedTagColor := tagv1.TagColor(UpdatedColor)

	req := connect.NewRequest(
		&tagv1.TagUpdateRequest{
			TagId: testTagID.Bytes(),
			Name:  &UpdatedName,
			Color: &UpdatedTagColor,
		},
	)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	resp, err := serviceRPC.TagUpdate(authedCtx, req)
	testutil.AssertFatal(t, nil, err)
	testutil.AssertNotFatal(t, nil, resp.Msg)
	dbTag, err := ts.GetTag(ctx, testTagID)
	testutil.AssertFatal(t, nil, err)

	testutil.Assert(t, UpdatedName, dbTag.Name)
	testutil.Assert(t, UpdatedColor, dbTag.Color)
}

func TestDeleteTag(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	ws := sworkspace.New(queries)
	us := suser.New(queries)
	ts := stag.New(queries)

	serviceRPC := rtag.New(db, ws, us, ts)
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
	if err != nil {
		t.Error(err)
	}

	req := connect.NewRequest(
		&tagv1.TagDeleteRequest{
			TagId: testTagID.Bytes(),
		},
	)

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	resp, err := serviceRPC.TagDelete(authedCtx, req)
	testutil.AssertFatal(t, nil, err)
	testutil.AssertNotFatal(t, nil, resp.Msg)

	_, err = ts.GetTag(ctx, testTagID)
	testutil.Assert(t, stag.ErrNoTag, err)
}
