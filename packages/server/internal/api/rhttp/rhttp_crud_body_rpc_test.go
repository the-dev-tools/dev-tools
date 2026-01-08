package rhttp

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	apiv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/http/v1"
)

// ========== RAW BODY TESTS ==========

func TestHttpBodyRawInsert(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, ws, "test-http")

	rawData := "test raw data"
	req := connect.NewRequest(&apiv1.HttpBodyRawInsertRequest{
		Items: []*apiv1.HttpBodyRawInsert{
			{
				HttpId: httpID.Bytes(),
				Data:   rawData,
			},
		},
	})

	_, err := f.handler.HttpBodyRawInsert(f.ctx, req)
	require.NoError(t, err, "HttpBodyRawInsert")

	// Verify
	body, err := f.handler.bodyService.GetByHttpID(f.ctx, httpID)
	require.NoError(t, err)
	require.Equal(t, rawData, string(body.RawData))
}

func TestHttpBodyRawUpdate(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, ws, "test-http")

	// Setup initial data
	initialData := "initial data"
	_, err := f.handler.bodyService.Create(f.ctx, httpID, []byte(initialData))
	require.NoError(t, err)

	// Update
	updatedData := "updated data"
	req := connect.NewRequest(&apiv1.HttpBodyRawUpdateRequest{
		Items: []*apiv1.HttpBodyRawUpdate{
			{
				HttpId: httpID.Bytes(),
				Data:   &updatedData,
			},
		},
	})

	_, err = f.handler.HttpBodyRawUpdate(f.ctx, req)
	require.NoError(t, err, "HttpBodyRawUpdate")

	// Verify
	body, err := f.handler.bodyService.GetByHttpID(f.ctx, httpID)
	require.NoError(t, err)
	require.Equal(t, updatedData, string(body.RawData))
}

func TestHttpBodyRawCollection(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID1 := f.createHttp(t, ws, "test-http-1")
	httpID2 := f.createHttp(t, ws, "test-http-2")

	// Create bodies
	_, err := f.handler.bodyService.Create(f.ctx, httpID1, []byte("data1"))
	require.NoError(t, err)
	_, err = f.handler.bodyService.Create(f.ctx, httpID2, []byte("data2"))
	require.NoError(t, err)

	req := connect.NewRequest(&emptypb.Empty{})
	resp, err := f.handler.HttpBodyRawCollection(f.ctx, req)
	require.NoError(t, err)

	require.Len(t, resp.Msg.Items, 2)
	// Simple check that we have our items
	found1 := false
	found2 := false
	for _, item := range resp.Msg.Items {
		if string(item.Data) == "data1" {
			found1 = true
		}
		if string(item.Data) == "data2" {
			found2 = true
		}
	}
	require.True(t, found1, "data1 not found")
	require.True(t, found2, "data2 not found")
}

// ========== FORM DATA TESTS ==========

func TestHttpBodyFormDataInsert(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, ws, "test-http")

	formID := idwrap.NewNow()
	key := "foo"
	value := "bar"
	req := connect.NewRequest(&apiv1.HttpBodyFormDataInsertRequest{
		Items: []*apiv1.HttpBodyFormDataInsert{
			{
				HttpBodyFormDataId: formID.Bytes(),
				HttpId:             httpID.Bytes(),
				Key:                key,
				Value:              value,
				Enabled:            true,
			},
		},
	})

	_, err := f.handler.HttpBodyFormDataInsert(f.ctx, req)
	require.NoError(t, err)

	// Verify
	items, err := f.handler.httpBodyFormService.GetByHttpID(f.ctx, httpID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, key, items[0].Key)
	require.Equal(t, value, items[0].Value)
}

func TestHttpBodyFormDataUpdate(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, ws, "test-http")

	// Insert initial
	formID := idwrap.NewNow()
	key := "foo"
	value := "bar"
	insertReq := connect.NewRequest(&apiv1.HttpBodyFormDataInsertRequest{
		Items: []*apiv1.HttpBodyFormDataInsert{
			{
				HttpBodyFormDataId: formID.Bytes(),
				HttpId:             httpID.Bytes(),
				Key:                key,
				Value:              value,
				Enabled:            true,
			},
		},
	})
	_, err := f.handler.HttpBodyFormDataInsert(f.ctx, insertReq)
	require.NoError(t, err)

	// Update
	newKey := "foo_updated"
	newValue := "bar_updated"
	updateReq := connect.NewRequest(&apiv1.HttpBodyFormDataUpdateRequest{
		Items: []*apiv1.HttpBodyFormDataUpdate{
			{
				HttpBodyFormDataId: formID.Bytes(),
				Key:                &newKey,
				Value:              &newValue,
			},
		},
	})

	_, err = f.handler.HttpBodyFormDataUpdate(f.ctx, updateReq)
	require.NoError(t, err)

	// Verify
	items, err := f.handler.httpBodyFormService.GetByHttpID(f.ctx, httpID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, newKey, items[0].Key)
	require.Equal(t, newValue, items[0].Value)
}

func TestHttpBodyFormDataDelete(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, ws, "test-http")

	// Insert initial
	formID := idwrap.NewNow()
	insertReq := connect.NewRequest(&apiv1.HttpBodyFormDataInsertRequest{
		Items: []*apiv1.HttpBodyFormDataInsert{
			{
				HttpBodyFormDataId: formID.Bytes(),
				HttpId:             httpID.Bytes(),
				Key:                "foo",
				Value:              "bar",
				Enabled:            true,
			},
		},
	})
	_, err := f.handler.HttpBodyFormDataInsert(f.ctx, insertReq)
	require.NoError(t, err)

	// Delete
	deleteReq := connect.NewRequest(&apiv1.HttpBodyFormDataDeleteRequest{
		Items: []*apiv1.HttpBodyFormDataDelete{
			{
				HttpBodyFormDataId: formID.Bytes(),
			},
		},
	})

	_, err = f.handler.HttpBodyFormDataDelete(f.ctx, deleteReq)
	require.NoError(t, err)

	// Verify
	items, err := f.handler.httpBodyFormService.GetByHttpID(f.ctx, httpID)
	require.NoError(t, err)
	require.Empty(t, items)
}

func TestHttpBodyFormDataCollection(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, ws, "test-http")

	// Insert items
	formID1 := idwrap.NewNow()
	formID2 := idwrap.NewNow()
	insertReq := connect.NewRequest(&apiv1.HttpBodyFormDataInsertRequest{
		Items: []*apiv1.HttpBodyFormDataInsert{
			{
				HttpBodyFormDataId: formID1.Bytes(),
				HttpId:             httpID.Bytes(),
				Key:                "k1",
				Value:              "v1",
				Enabled:            true,
			},
			{
				HttpBodyFormDataId: formID2.Bytes(),
				HttpId:             httpID.Bytes(),
				Key:                "k2",
				Value:              "v2",
				Enabled:            true,
			},
		},
	})
	_, err := f.handler.HttpBodyFormDataInsert(f.ctx, insertReq)
	require.NoError(t, err)

	// Collection
	req := connect.NewRequest(&emptypb.Empty{})
	resp, err := f.handler.HttpBodyFormDataCollection(f.ctx, req)
	require.NoError(t, err)

	require.Len(t, resp.Msg.Items, 2)
}

// ========== URL ENCODED TESTS ==========

func TestHttpBodyUrlEncodedInsert(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, ws, "test-http")

	encodedID := idwrap.NewNow()
	key := "foo"
	value := "bar"
	req := connect.NewRequest(&apiv1.HttpBodyUrlEncodedInsertRequest{
		Items: []*apiv1.HttpBodyUrlEncodedInsert{
			{
				HttpBodyUrlEncodedId: encodedID.Bytes(),
				HttpId:               httpID.Bytes(),
				Key:                  key,
				Value:                value,
				Enabled:              true,
			},
		},
	})

	_, err := f.handler.HttpBodyUrlEncodedInsert(f.ctx, req)
	require.NoError(t, err)

	// Verify
	items, err := f.handler.httpBodyUrlEncodedService.GetByHttpID(f.ctx, httpID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, key, items[0].Key)
	require.Equal(t, value, items[0].Value)
}

func TestHttpBodyUrlEncodedUpdate(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, ws, "test-http")

	// Insert
	encodedID := idwrap.NewNow()
	insertReq := connect.NewRequest(&apiv1.HttpBodyUrlEncodedInsertRequest{
		Items: []*apiv1.HttpBodyUrlEncodedInsert{
			{
				HttpBodyUrlEncodedId: encodedID.Bytes(),
				HttpId:               httpID.Bytes(),
				Key:                  "foo",
				Value:                "bar",
				Enabled:              true,
			},
		},
	})
	_, err := f.handler.HttpBodyUrlEncodedInsert(f.ctx, insertReq)
	require.NoError(t, err)

	// Update
	newKey := "foo_updated"
	newValue := "bar_updated"
	updateReq := connect.NewRequest(&apiv1.HttpBodyUrlEncodedUpdateRequest{
		Items: []*apiv1.HttpBodyUrlEncodedUpdate{
			{
				HttpBodyUrlEncodedId: encodedID.Bytes(),
				Key:                  &newKey,
				Value:                &newValue,
			},
		},
	})

	_, err = f.handler.HttpBodyUrlEncodedUpdate(f.ctx, updateReq)
	require.NoError(t, err)

	// Verify
	items, err := f.handler.httpBodyUrlEncodedService.GetByHttpID(f.ctx, httpID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, newKey, items[0].Key)
	require.Equal(t, newValue, items[0].Value)
}

func TestHttpBodyUrlEncodedDelete(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, ws, "test-http")

	// Insert
	encodedID := idwrap.NewNow()
	insertReq := connect.NewRequest(&apiv1.HttpBodyUrlEncodedInsertRequest{
		Items: []*apiv1.HttpBodyUrlEncodedInsert{
			{
				HttpBodyUrlEncodedId: encodedID.Bytes(),
				HttpId:               httpID.Bytes(),
				Key:                  "foo",
				Value:                "bar",
				Enabled:              true,
			},
		},
	})
	_, err := f.handler.HttpBodyUrlEncodedInsert(f.ctx, insertReq)
	require.NoError(t, err)

	// Delete
	deleteReq := connect.NewRequest(&apiv1.HttpBodyUrlEncodedDeleteRequest{
		Items: []*apiv1.HttpBodyUrlEncodedDelete{
			{
				HttpBodyUrlEncodedId: encodedID.Bytes(),
			},
		},
	})

	_, err = f.handler.HttpBodyUrlEncodedDelete(f.ctx, deleteReq)
	require.NoError(t, err)

	// Verify
	items, err := f.handler.httpBodyUrlEncodedService.GetByHttpID(f.ctx, httpID)
	require.NoError(t, err)
	require.Empty(t, items)
}

func TestHttpBodyUrlEncodedCollection(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "test-workspace")
	httpID := f.createHttp(t, ws, "test-http")

	// Insert items
	encodedID1 := idwrap.NewNow()
	encodedID2 := idwrap.NewNow()
	insertReq := connect.NewRequest(&apiv1.HttpBodyUrlEncodedInsertRequest{
		Items: []*apiv1.HttpBodyUrlEncodedInsert{
			{
				HttpBodyUrlEncodedId: encodedID1.Bytes(),
				HttpId:               httpID.Bytes(),
				Key:                  "k1",
				Value:                "v1",
				Enabled:              true,
			},
			{
				HttpBodyUrlEncodedId: encodedID2.Bytes(),
				HttpId:               httpID.Bytes(),
				Key:                  "k2",
				Value:                "v2",
				Enabled:              true,
			},
		},
	})
	_, err := f.handler.HttpBodyUrlEncodedInsert(f.ctx, insertReq)
	require.NoError(t, err)

	// Collection
	req := connect.NewRequest(&emptypb.Empty{})
	resp, err := f.handler.HttpBodyUrlEncodedCollection(f.ctx, req)
	require.NoError(t, err)

	require.Len(t, resp.Msg.Items, 2)
}
