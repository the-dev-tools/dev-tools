package rhttp

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	httpv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

func TestHttpAssertInsert(t *testing.T) {
	t.Parallel()
	f := newHttpFixture(t)
	wsID := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, wsID, "test-http")

	assertID := idwrap.NewNow()
	expression := "response.status == 200"

	req := connect.NewRequest(&httpv1.HttpAssertInsertRequest{
		Items: []*httpv1.HttpAssertInsert{
			{
				HttpAssertId: assertID.Bytes(),
				HttpId:       httpID.Bytes(),
				Value:        expression,
				Enabled:      true,
				Order:        1.5,
			},
		},
	})

	_, err := f.handler.HttpAssertInsert(f.ctx, req)
	require.NoError(t, err)

	// Verify insertion via Collection
	colReq := connect.NewRequest(&emptypb.Empty{})
	resp, err := f.handler.HttpAssertCollection(f.ctx, colReq)
	require.NoError(t, err)

	var found *httpv1.HttpAssert
	for _, item := range resp.Msg.Items {
		if string(item.HttpAssertId) == string(assertID.Bytes()) {
			found = item
			break
		}
	}
	require.NotNil(t, found, "assert not found in collection")
	require.Equal(t, expression, found.Value)
	require.True(t, found.Enabled)
	require.Equal(t, float32(1.5), found.Order)
}

func TestHttpAssertInsert_EnabledFalse(t *testing.T) {
	t.Parallel()
	f := newHttpFixture(t)
	wsID := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, wsID, "test-http")

	assertID := idwrap.NewNow()
	expression := "response.status == 200"

	req := connect.NewRequest(&httpv1.HttpAssertInsertRequest{
		Items: []*httpv1.HttpAssertInsert{
			{
				HttpAssertId: assertID.Bytes(),
				HttpId:       httpID.Bytes(),
				Value:        expression,
				Enabled:      false,
				Order:        2.0,
			},
		},
	})

	_, err := f.handler.HttpAssertInsert(f.ctx, req)
	require.NoError(t, err)

	// Verify insertion via Collection
	colReq := connect.NewRequest(&emptypb.Empty{})
	resp, err := f.handler.HttpAssertCollection(f.ctx, colReq)
	require.NoError(t, err)

	var found *httpv1.HttpAssert
	for _, item := range resp.Msg.Items {
		if string(item.HttpAssertId) == string(assertID.Bytes()) {
			found = item
			break
		}
	}
	require.NotNil(t, found, "assert not found in collection")
	require.Equal(t, expression, found.Value)
	require.False(t, found.Enabled, "enabled should be false")
	require.Equal(t, float32(2.0), found.Order)
}

func TestHttpAssertInsert_Errors(t *testing.T) {
	t.Parallel()
	f := newHttpFixture(t)

	// Empty items
	reqEmpty := connect.NewRequest(&httpv1.HttpAssertInsertRequest{
		Items: []*httpv1.HttpAssertInsert{},
	})
	_, err := f.handler.HttpAssertInsert(f.ctx, reqEmpty)
	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	require.Equal(t, connect.CodeInvalidArgument, connectErr.Code())

	// Non-existent HTTP ID
	reqNotFound := connect.NewRequest(&httpv1.HttpAssertInsertRequest{
		Items: []*httpv1.HttpAssertInsert{
			{
				HttpAssertId: idwrap.NewNow().Bytes(),
				HttpId:       idwrap.NewNow().Bytes(),
				Value:        "val",
			},
		},
	})
	_, err = f.handler.HttpAssertInsert(f.ctx, reqNotFound)
	require.Error(t, err)
	connectErr, ok = err.(*connect.Error)
	require.True(t, ok)
	require.Equal(t, connect.CodeNotFound, connectErr.Code())
}

func TestHttpAssertUpdate(t *testing.T) {
	t.Parallel()
	f := newHttpFixture(t)
	wsID := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, wsID, "test-http")

	assertID := idwrap.NewNow()
	// Manually create assertion to control ID
	assertion := &mhttp.HTTPAssert{
		ID:           assertID,
		HttpID:       httpID,
		Value:        "response.status == 200",
		Description:  "desc",
		Enabled:      true,
		DisplayOrder: 1.0,
		IsDelta:      false,
		CreatedAt:    time.Now().Unix(),
		UpdatedAt:    time.Now().Unix(),
	}
	err := f.handler.httpAssertService.Create(f.ctx, assertion)
	require.NoError(t, err)

	newValue := "response.status == 201"
	req := connect.NewRequest(&httpv1.HttpAssertUpdateRequest{
		Items: []*httpv1.HttpAssertUpdate{
			{
				HttpAssertId: assertID.Bytes(),
				Value:        &newValue,
			},
		},
	})

	_, err = f.handler.HttpAssertUpdate(f.ctx, req)
	require.NoError(t, err)

	// Verify update
	colReq := connect.NewRequest(&emptypb.Empty{})
	resp, err := f.handler.HttpAssertCollection(f.ctx, colReq)
	require.NoError(t, err)

	var found *httpv1.HttpAssert
	for _, item := range resp.Msg.Items {
		if string(item.HttpAssertId) == string(assertID.Bytes()) {
			found = item
			break
		}
	}
	require.NotNil(t, found)
	require.Equal(t, newValue, found.Value)
	require.True(t, found.Enabled, "enabled should remain true")
	require.Equal(t, float32(1.0), found.Order, "order should remain unchanged")
}

func TestHttpAssertUpdate_EnabledAndOrder(t *testing.T) {
	t.Parallel()
	f := newHttpFixture(t)
	wsID := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, wsID, "test-http")

	assertID := idwrap.NewNow()
	assertion := &mhttp.HTTPAssert{
		ID:           assertID,
		HttpID:       httpID,
		Value:        "response.status == 200",
		Description:  "desc",
		Enabled:      true,
		DisplayOrder: 1.0,
		IsDelta:      false,
		CreatedAt:    time.Now().Unix(),
		UpdatedAt:    time.Now().Unix(),
	}
	err := f.handler.httpAssertService.Create(f.ctx, assertion)
	require.NoError(t, err)

	// Update enabled to false and order to 5.5
	newEnabled := false
	newOrder := float32(5.5)
	req := connect.NewRequest(&httpv1.HttpAssertUpdateRequest{
		Items: []*httpv1.HttpAssertUpdate{
			{
				HttpAssertId: assertID.Bytes(),
				Enabled:      &newEnabled,
				Order:        &newOrder,
			},
		},
	})

	_, err = f.handler.HttpAssertUpdate(f.ctx, req)
	require.NoError(t, err)

	// Verify update
	colReq := connect.NewRequest(&emptypb.Empty{})
	resp, err := f.handler.HttpAssertCollection(f.ctx, colReq)
	require.NoError(t, err)

	var found *httpv1.HttpAssert
	for _, item := range resp.Msg.Items {
		if string(item.HttpAssertId) == string(assertID.Bytes()) {
			found = item
			break
		}
	}
	require.NotNil(t, found)
	require.Equal(t, "response.status == 200", found.Value, "value should remain unchanged")
	require.False(t, found.Enabled, "enabled should be updated to false")
	require.Equal(t, float32(5.5), found.Order, "order should be updated to 5.5")
}

func TestHttpAssertUpdate_Errors(t *testing.T) {
	t.Parallel()
	f := newHttpFixture(t)

	// Empty items
	reqEmpty := connect.NewRequest(&httpv1.HttpAssertUpdateRequest{
		Items: []*httpv1.HttpAssertUpdate{},
	})
	_, err := f.handler.HttpAssertUpdate(f.ctx, reqEmpty)
	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	require.Equal(t, connect.CodeInvalidArgument, connectErr.Code())

	// Non-existent Assert ID
	val := "new"
	reqNotFound := connect.NewRequest(&httpv1.HttpAssertUpdateRequest{
		Items: []*httpv1.HttpAssertUpdate{
			{
				HttpAssertId: idwrap.NewNow().Bytes(),
				Value:        &val,
			},
		},
	})
	_, err = f.handler.HttpAssertUpdate(f.ctx, reqNotFound)
	require.Error(t, err)
	connectErr, ok = err.(*connect.Error)
	require.True(t, ok)
	require.Equal(t, connect.CodeNotFound, connectErr.Code())
}

func TestHttpAssertDelete(t *testing.T) {
	t.Parallel()
	f := newHttpFixture(t)
	wsID := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, wsID, "test-http")

	assertID := idwrap.NewNow()
	assertion := &mhttp.HTTPAssert{
		ID:          assertID,
		HttpID:      httpID,
		Value:       "response.status == 200",
		Description: "desc",
		Enabled:     true,
		IsDelta:     false,
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
	}
	err := f.handler.httpAssertService.Create(f.ctx, assertion)
	require.NoError(t, err)

	req := connect.NewRequest(&httpv1.HttpAssertDeleteRequest{
		Items: []*httpv1.HttpAssertDelete{
			{
				HttpAssertId: assertID.Bytes(),
			},
		},
	})

	_, err = f.handler.HttpAssertDelete(f.ctx, req)
	require.NoError(t, err)

	// Verify deletion
	colReq := connect.NewRequest(&emptypb.Empty{})
	resp, err := f.handler.HttpAssertCollection(f.ctx, colReq)
	require.NoError(t, err)

	found := false
	for _, item := range resp.Msg.Items {
		if string(item.HttpAssertId) == string(assertID.Bytes()) {
			found = true
			break
		}
	}
	require.False(t, found, "Assertion should have been deleted")
}

func TestHttpAssertDelete_Errors(t *testing.T) {
	t.Parallel()
	f := newHttpFixture(t)

	// Empty items
	reqEmpty := connect.NewRequest(&httpv1.HttpAssertDeleteRequest{
		Items: []*httpv1.HttpAssertDelete{},
	})
	_, err := f.handler.HttpAssertDelete(f.ctx, reqEmpty)
	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	require.Equal(t, connect.CodeInvalidArgument, connectErr.Code())

	// Non-existent Assert ID
	reqNotFound := connect.NewRequest(&httpv1.HttpAssertDeleteRequest{
		Items: []*httpv1.HttpAssertDelete{
			{
				HttpAssertId: idwrap.NewNow().Bytes(),
			},
		},
	})
	_, err = f.handler.HttpAssertDelete(f.ctx, reqNotFound)
	require.Error(t, err)
	connectErr, ok = err.(*connect.Error)
	require.True(t, ok)
	require.Equal(t, connect.CodeNotFound, connectErr.Code())
}

func TestHttpAssertCollection(t *testing.T) {
	t.Parallel()
	f := newHttpFixture(t)
	wsID := f.createWorkspace(t, "test-workspace")
	httpID1 := f.createHttp(t, wsID, "test-http-1")
	httpID2 := f.createHttp(t, wsID, "test-http-2")

	f.createHttpAssertion(t, httpID1, "val1", "desc1")
	f.createHttpAssertion(t, httpID1, "val2", "desc2")
	f.createHttpAssertion(t, httpID2, "val3", "desc3")

	colReq := connect.NewRequest(&emptypb.Empty{})
	resp, err := f.handler.HttpAssertCollection(f.ctx, colReq)
	require.NoError(t, err)
	require.Len(t, resp.Msg.Items, 3)
}

func TestHttpAssertCollection_VerifyEnabledAndOrder(t *testing.T) {
	t.Parallel()
	f := newHttpFixture(t)
	wsID := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, wsID, "test-http")

	assertID1 := idwrap.NewNow()
	assertion1 := &mhttp.HTTPAssert{
		ID:           assertID1,
		HttpID:       httpID,
		Value:        "response.status == 200",
		Description:  "enabled assertion",
		Enabled:      true,
		DisplayOrder: 1.5,
		IsDelta:      false,
		CreatedAt:    time.Now().Unix(),
		UpdatedAt:    time.Now().Unix(),
	}
	err := f.handler.httpAssertService.Create(f.ctx, assertion1)
	require.NoError(t, err)

	assertID2 := idwrap.NewNow()
	assertion2 := &mhttp.HTTPAssert{
		ID:           assertID2,
		HttpID:       httpID,
		Value:        "response.body.length > 0",
		Description:  "disabled assertion",
		Enabled:      false,
		DisplayOrder: 2.5,
		IsDelta:      false,
		CreatedAt:    time.Now().Unix(),
		UpdatedAt:    time.Now().Unix(),
	}
	err = f.handler.httpAssertService.Create(f.ctx, assertion2)
	require.NoError(t, err)

	colReq := connect.NewRequest(&emptypb.Empty{})
	resp, err := f.handler.HttpAssertCollection(f.ctx, colReq)
	require.NoError(t, err)
	require.Len(t, resp.Msg.Items, 2)

	// Find and verify first assertion
	var found1, found2 *httpv1.HttpAssert
	for _, item := range resp.Msg.Items {
		if string(item.HttpAssertId) == string(assertID1.Bytes()) {
			found1 = item
		}
		if string(item.HttpAssertId) == string(assertID2.Bytes()) {
			found2 = item
		}
	}

	require.NotNil(t, found1, "assertion 1 not found")
	require.Equal(t, "response.status == 200", found1.Value)
	require.True(t, found1.Enabled, "assertion 1 should be enabled")
	require.Equal(t, float32(1.5), found1.Order, "assertion 1 order should be 1.5")

	require.NotNil(t, found2, "assertion 2 not found")
	require.Equal(t, "response.body.length > 0", found2.Value)
	require.False(t, found2.Enabled, "assertion 2 should be disabled")
	require.Equal(t, float32(2.5), found2.Order, "assertion 2 order should be 2.5")
}
