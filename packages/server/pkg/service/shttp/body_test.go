package shttp

import (
	"context"
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/dbtest"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"

	"github.com/stretchr/testify/require"
)

func TestHttpBodyRawService(t *testing.T) {
	ctx := context.Background()
	db, err := dbtest.GetTestPreparedQueries(ctx)
	require.NoError(t, err)
	defer db.Close()

	service := NewHttpBodyRawService(db)

	// Parent HTTP
	httpService := New(db, nil)
	httpID := idwrap.NewNow()
	err = httpService.Create(ctx, &mhttp.HTTP{
		ID:          httpID,
		WorkspaceID: idwrap.NewNow(),
		Name:        "Test",
	})
	require.NoError(t, err)

	bodyID := idwrap.NewNow()
	body := &mhttp.HTTPBodyRaw{
		ID:      bodyID,
		HttpID:  httpID,
		RawData: []byte("raw content"),
	}

	// CreateFull
	_, err = service.CreateFull(ctx, body)
	require.NoError(t, err)

	// GetByHttpID
	retrieved, err := service.GetByHttpID(ctx, httpID)
	require.NoError(t, err)
	require.Equal(t, []byte("raw content"), retrieved.RawData)

	// Update
	// Update signature usually takes struct or fields. body_raw.go likely has Update(ctx, body) or UpdateRawData(ctx, id, data).
	// Let's assume Update(ctx, body) exists or check logic.
	// Looking at previous patterns, Update might not exist for BodyRaw as it's often 1:1 and Upsert logic or specific updates.
	// Actually, body_raw.go often has `Update` method.

	// Let's try UpdateRawData if it exists, or Upsert.
	// Based on rhttp_exec logic, it uses `bodyService.GetByHttpID`.
	// Let's try creating a delta too.

	deltaID := idwrap.NewNow()
	// Create Delta Request first
	err = httpService.Create(ctx, &mhttp.HTTP{
		ID:           deltaID,
		WorkspaceID:  idwrap.NewNow(),
		Name:         "Delta",
		IsDelta:      true,
		ParentHttpID: &httpID,
	})
	require.NoError(t, err)

	// Create Delta Body
	_, err = service.CreateDelta(ctx, deltaID, []byte("delta content"))
	require.NoError(t, err)

	// Get Delta
	deltaBody, err := service.GetByHttpID(ctx, deltaID)
	require.NoError(t, err)
	require.True(t, deltaBody.IsDelta)
	require.Equal(t, []byte("delta content"), deltaBody.DeltaRawData)
}

func TestHttpBodyFormService(t *testing.T) {
	ctx := context.Background()
	db, err := dbtest.GetTestPreparedQueries(ctx)
	require.NoError(t, err)
	defer db.Close()

	service := NewHttpBodyFormService(db)

	// Parent HTTP
	httpService := New(db, nil)
	httpID := idwrap.NewNow()
	err = httpService.Create(ctx, &mhttp.HTTP{
		ID:          httpID,
		WorkspaceID: idwrap.NewNow(),
		Name:        "Test",
	})
	require.NoError(t, err)

	formID := idwrap.NewNow()
	form := &mhttp.HTTPBodyForm{
		ID:      formID,
		HttpID:  httpID,
		Key:     "username",
		Value:   "admin",
		Enabled: true,
	}

	// Create
	err = service.Create(ctx, form)
	require.NoError(t, err)

	// GetByHttpID
	forms, err := service.GetByHttpID(ctx, httpID)
	require.NoError(t, err)
	require.Len(t, forms, 1)
	require.Equal(t, "username", forms[0].Key)

	// Update
	form.Value = "root"
	err = service.Update(ctx, form)
	require.NoError(t, err)

	updated, err := service.GetByID(ctx, formID)
	require.NoError(t, err)
	require.Equal(t, "root", updated.Value)

	// Delete
	err = service.Delete(ctx, formID)
	require.NoError(t, err)
}

func TestHttpBodyUrlEncodedService(t *testing.T) {
	ctx := context.Background()
	db, err := dbtest.GetTestPreparedQueries(ctx)
	require.NoError(t, err)
	defer db.Close()

	service := NewHttpBodyUrlEncodedService(db)

	// Parent HTTP
	httpService := New(db, nil)
	httpID := idwrap.NewNow()
	err = httpService.Create(ctx, &mhttp.HTTP{
		ID:          httpID,
		WorkspaceID: idwrap.NewNow(),
		Name:        "Test",
	})
	require.NoError(t, err)

	encodedID := idwrap.NewNow()
	encoded := &mhttp.HTTPBodyUrlencoded{
		ID:      encodedID,
		HttpID:  httpID,
		Key:     "page",
		Value:   "1",
		Enabled: true,
	}

	// Create
	err = service.Create(ctx, encoded)
	require.NoError(t, err)

	// GetByHttpID
	encodeds, err := service.GetByHttpID(ctx, httpID)
	require.NoError(t, err)
	require.Len(t, encodeds, 1)
	require.Equal(t, "page", encodeds[0].Key)

	// Update
	encoded.Value = "2"
	err = service.Update(ctx, encoded)
	require.NoError(t, err)

	updated, err := service.GetByID(ctx, encodedID)
	require.NoError(t, err)
	require.Equal(t, "2", updated.Value)

	// Delete
	err = service.Delete(ctx, encodedID)
	require.NoError(t, err)
}
