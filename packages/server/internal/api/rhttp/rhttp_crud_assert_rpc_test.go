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
