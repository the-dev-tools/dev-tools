package rrequest_test

import (
	"context"
	"database/sql"
	"encoding/hex"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rrequest"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/testutil"
	requestv1 "the-dev-tools/spec/dist/buf/go/collection/item/request/v1"
	conditionv1 "the-dev-tools/spec/dist/buf/go/condition/v1"
)

const (
	originExampleHex  = "9CB623E0A88B0C487DBC79FAECF786FC"
	defaultExampleHex = "940F880CE8754F755CE14207EFAB3B40"
	deltaExampleHex   = "E7C36B92989B12C3F2EC43780A4949A9"
)

type deltaRPCSuite struct {
	ctx              context.Context
	rpc              rrequest.RequestRPC
	as               sassert.AssertService
	iaes             sitemapiexample.ItemApiExampleService
	originExampleID  idwrap.IDWrap
	defaultExampleID idwrap.IDWrap
	deltaExampleID   idwrap.IDWrap
	originAssertID   idwrap.IDWrap
}

func setupDeltaRPCSuite(t *testing.T) deltaRPCSuite {
	t.Helper()

	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	t.Cleanup(base.Close)

	queries := base.Queries
	db := base.DB

	baseServices := base.GetBaseServices()

	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	ehs := sexampleheader.New(queries)
	eqs := sexamplequery.New(queries)
	as := sassert.New(queries)

	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()

	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

	ctx = mwauth.CreateAuthedContext(ctx, userID)

	endpointID := idwrap.NewNow()
	endpoint := &mitemapi.ItemApi{
		ID:           endpointID,
		CollectionID: collectionID,
		Name:         "delta-endpoint",
		Method:       "GET",
		Url:          "/api/delta",
	}
	require.NoError(t, ias.CreateItemApi(ctx, endpoint))

	originExampleID := mustID(t, originExampleHex)
	originExample := &mitemapiexample.ItemApiExample{
		ID:           originExampleID,
		ItemApiID:    endpointID,
		CollectionID: collectionID,
		Name:         "Origin Example",
	}
	require.NoError(t, iaes.CreateApiExample(ctx, originExample))

	defaultExampleID := mustID(t, defaultExampleHex)
	defaultExample := &mitemapiexample.ItemApiExample{
		ID:           defaultExampleID,
		ItemApiID:    endpointID,
		CollectionID: collectionID,
		Name:         "Default Example",
		IsDefault:    true,
	}
	require.NoError(t, iaes.CreateApiExample(ctx, defaultExample))

	deltaExampleID := mustID(t, deltaExampleHex)
	deltaExample := &mitemapiexample.ItemApiExample{
		ID:              deltaExampleID,
		ItemApiID:       endpointID,
		CollectionID:    collectionID,
		Name:            "Delta Example",
		VersionParentID: &originExampleID,
	}
	require.NoError(t, iaes.CreateApiExample(ctx, deltaExample))

	condition := mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 304"}}
	originAssertID := idwrap.NewNow()
	originAssert := massert.Assert{ID: originAssertID, ExampleID: originExampleID, Condition: condition, Enable: true}
	require.NoError(t, as.CreateAssert(ctx, originAssert))

	defaultAssert := originAssert
	defaultAssert.ID = idwrap.NewNow()
	defaultAssert.ExampleID = defaultExampleID
	require.NoError(t, as.CreateAssert(ctx, defaultAssert))

	deltaAssert := originAssert
	deltaAssert.ID = idwrap.NewNow()
	deltaAssert.ExampleID = deltaExampleID
	deltaAssert.DeltaParentID = &originAssertID
	require.NoError(t, as.CreateAssert(ctx, deltaAssert))

	cs := baseServices.Cs
	us := baseServices.Us

	rpc := rrequest.New(db, cs, us, ias, iaes, ehs, eqs, as)

	return deltaRPCSuite{
		ctx:              ctx,
		rpc:              rpc,
		as:               as,
		iaes:             iaes,
		originExampleID:  originExampleID,
		defaultExampleID: defaultExampleID,
		deltaExampleID:   deltaExampleID,
		originAssertID:   originAssertID,
	}
}

func mustID(t *testing.T, hexStr string) idwrap.IDWrap {
	t.Helper()
	bytes, err := hex.DecodeString(hexStr)
	require.NoError(t, err)
	id, err := idwrap.NewFromBytes(bytes)
	require.NoError(t, err)
	return id
}

func TestAssertUpdatePropagatesThroughRPC(t *testing.T) {
	suite := setupDeltaRPCSuite(t)

	newExpr := "response.status == 200"
	condition := &conditionv1.Condition{
		Comparison: &conditionv1.Comparison{Expression: newExpr},
	}

	_, err := suite.rpc.AssertUpdate(suite.ctx, connect.NewRequest(&requestv1.AssertUpdateRequest{
		AssertId:  suite.originAssertID.Bytes(),
		Condition: condition,
	}))
	require.NoError(t, err)

	origin, err := suite.as.GetAssert(suite.ctx, suite.originAssertID)
	require.NoError(t, err)
	require.Equal(t, newExpr, origin.Condition.Comparisons.Expression)

	defaultAsserts, err := suite.as.GetAssertByExampleID(suite.ctx, suite.defaultExampleID)
	require.NoError(t, err)
	require.Len(t, defaultAsserts, 1)
	require.Equal(t, newExpr, defaultAsserts[0].Condition.Comparisons.Expression)

	deltaAsserts, err := suite.as.GetAssertByExampleID(suite.ctx, suite.deltaExampleID)
	require.NoError(t, err)
	require.Len(t, deltaAsserts, 1)
	require.Equal(t, newExpr, deltaAsserts[0].Condition.Comparisons.Expression)
	require.NotNil(t, deltaAsserts[0].DeltaParentID)
	require.Equal(t, 0, deltaAsserts[0].DeltaParentID.Compare(suite.originAssertID))
}

func TestAssertUpdateCreatesMissingDeltaThroughRPC(t *testing.T) {
	suite := setupDeltaRPCSuite(t)

	deltaAsserts, err := suite.as.GetAssertByExampleID(suite.ctx, suite.deltaExampleID)
	require.NoError(t, err)
	require.Len(t, deltaAsserts, 1)

	require.NoError(t, suite.as.DeleteAssert(suite.ctx, deltaAsserts[0].ID))

	newExpr := "response.status == 201"
	condition := &conditionv1.Condition{
		Comparison: &conditionv1.Comparison{Expression: newExpr},
	}

	_, err = suite.rpc.AssertUpdate(suite.ctx, connect.NewRequest(&requestv1.AssertUpdateRequest{
		AssertId:  suite.originAssertID.Bytes(),
		Condition: condition,
	}))
	require.NoError(t, err)

	deltaAfter, err := suite.as.GetAssertByExampleID(suite.ctx, suite.deltaExampleID)
	require.NoError(t, err)
	require.Len(t, deltaAfter, 1)
	require.Equal(t, newExpr, deltaAfter[0].Condition.Comparisons.Expression)
	require.NotNil(t, deltaAfter[0].DeltaParentID)
	require.Equal(t, 0, deltaAfter[0].DeltaParentID.Compare(suite.originAssertID))
}

func TestAssertDeleteCascadesThroughRPC(t *testing.T) {
	suite := setupDeltaRPCSuite(t)

	_, err := suite.rpc.AssertDelete(suite.ctx, connect.NewRequest(&requestv1.AssertDeleteRequest{
		AssertId: suite.originAssertID.Bytes(),
	}))
	require.NoError(t, err)

	_, err = suite.as.GetAssert(suite.ctx, suite.originAssertID)
	require.ErrorIs(t, err, sql.ErrNoRows)

	defaultAsserts, err := suite.as.GetAssertByExampleID(suite.ctx, suite.defaultExampleID)
	if err != nil {
		require.ErrorIs(t, err, sassert.ErrNoAssertFound)
	} else {
		require.Len(t, defaultAsserts, 0)
	}

	deltaAsserts, err := suite.as.GetAssertByExampleID(suite.ctx, suite.deltaExampleID)
	if err != nil {
		require.ErrorIs(t, err, sassert.ErrNoAssertFound)
	} else {
		require.Len(t, deltaAsserts, 0)
	}
}

func TestAssertUpdateNoopWhenOriginMismatch(t *testing.T) {
	suite := setupDeltaRPCSuite(t)

	ctx := suite.ctx
	_, err := suite.rpc.AssertUpdate(ctx, connect.NewRequest(&requestv1.AssertUpdateRequest{
		AssertId:  idwrap.NewNow().Bytes(),
		Condition: &conditionv1.Condition{Comparison: &conditionv1.Comparison{Expression: "response.status == 202"}},
	}))
	require.Error(t, err)
}
