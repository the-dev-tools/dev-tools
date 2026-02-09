package rhttp

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	apiv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/http/v1"
)

func TestHttpDelta_BodyRaw(t *testing.T) {
	t.Parallel()
	f := newHttpFixture(t)
	workspaceID := f.createWorkspace(t, "Test Workspace")
	ctx := f.ctx

	// Create base request
	httpID := f.createHttp(t, workspaceID, "Base Request")

	// Create a base body for the base request
	// This is required because a delta body raw MUST point to a base body raw in the schema
	// (constraint: CHECK (is_delta = FALSE OR parent_body_raw_id IS NOT NULL))
	baseBodyData := "base-data"
	_, err := f.handler.bodyService.Create(ctx, httpID, []byte(baseBodyData))
	require.NoError(t, err)

	// Create delta request linked to base
	deltaID := idwrap.NewNow()
	err = f.hs.Create(ctx, &mhttp.HTTP{
		ID:           deltaID,
		WorkspaceID:  workspaceID,
		Name:         "Delta Request",
		IsDelta:      true,
		ParentHttpID: &httpID,
	})
	require.NoError(t, err)

	// 1. Insert Body Raw Delta
	data := "test-data"
	_, err = f.handler.HttpBodyRawDeltaInsert(ctx, connect.NewRequest(&apiv1.HttpBodyRawDeltaInsertRequest{
		Items: []*apiv1.HttpBodyRawDeltaInsert{
			{
				HttpId: deltaID.Bytes(), // Using Delta ID as HttpId
				Data:   &data,
			},
		},
	}))
	require.NoError(t, err)

	// Verify insert
	bodyRaw, err := f.handler.bodyService.GetByHttpID(ctx, deltaID)
	require.NoError(t, err)
	require.Equal(t, []byte(data), bodyRaw.DeltaRawData)
	require.True(t, bodyRaw.IsDelta)

	// 1.1. REGRESSION TEST: Verify HttpBodyRawDeltaCollection returns DeltaRawData, not RawData
	// This was a bug where collection returned the base body content instead of delta override
	collectionResp, err := f.handler.HttpBodyRawDeltaCollection(ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)

	// Find our delta in the collection
	var foundDelta *apiv1.HttpBodyRawDelta
	for _, d := range collectionResp.Msg.Items {
		foundHttpId, _ := idwrap.NewFromBytes(d.HttpId)
		// The collection returns deltas with the delta HTTP's own ID as HttpId
		// (matches what frontend queries by: deltaHttpId)
		if foundHttpId == deltaID {
			foundDelta = d
			break
		}
	}
	require.NotNil(t, foundDelta, "Expected to find delta in collection")
	require.NotNil(t, foundDelta.Data, "Expected delta data to be set")
	require.Equal(t, data, *foundDelta.Data, "HttpBodyRawDeltaCollection should return DeltaRawData, not base RawData")

	// 2. Update Body Raw Delta
	updatedData := "updated-data"
	_, err = f.handler.HttpBodyRawDeltaUpdate(ctx, connect.NewRequest(&apiv1.HttpBodyRawDeltaUpdateRequest{
		Items: []*apiv1.HttpBodyRawDeltaUpdate{
			{
				HttpId: deltaID.Bytes(),
				Data: &apiv1.HttpBodyRawDeltaUpdate_DataUnion{
					Kind:  apiv1.HttpBodyRawDeltaUpdate_DataUnion_KIND_VALUE,
					Value: &updatedData,
				},
			},
		},
	}))
	require.NoError(t, err)

	// Verify update
	bodyRaw, err = f.handler.bodyService.GetByHttpID(ctx, deltaID)
	require.NoError(t, err)
	require.Equal(t, []byte(updatedData), bodyRaw.DeltaRawData)

	// 3. Sync (Stream)
	// Use the internal streaming method directly to avoid needing a connect.ServerStream
	// We pass a callback that just logs/validates the response
	go func() {
		_ = f.handler.streamHttpBodyRawDeltaSync(ctx, f.userID, func(resp *apiv1.HttpBodyRawDeltaSyncResponse) error {
			return nil
		})
	}()

	// 4. Delete Body Raw Delta
	_, err = f.handler.HttpBodyRawDeltaDelete(ctx, connect.NewRequest(&apiv1.HttpBodyRawDeltaDeleteRequest{
		Items: []*apiv1.HttpBodyRawDeltaDelete{
			{
				DeltaHttpId: deltaID.Bytes(),
			},
		},
	}))
	require.NoError(t, err)

	// Verify delete
	_, err = f.handler.bodyService.GetByHttpID(ctx, deltaID)
	require.Error(t, err) // Should not be found
}

func TestHttpDelta_Assert(t *testing.T) {
	t.Parallel()
	f := newHttpFixture(t)
	workspaceID := f.createWorkspace(t, "Test Workspace")
	ctx := f.ctx

	// Create base request to serve as parent
	httpID := f.createHttp(t, workspaceID, "Base Request")

	// Create delta request
	deltaID := idwrap.NewNow()
	err := f.hs.Create(ctx, &mhttp.HTTP{
		ID:           deltaID,
		WorkspaceID:  workspaceID,
		Name:         "Delta Request",
		IsDelta:      true,
		ParentHttpID: &httpID, // Required by constraint: CHECK (is_delta = FALSE OR parent_http_id IS NOT NULL)
	})
	require.NoError(t, err)

	// 1. Insert Assert Delta
	// Create a base assert first
	baseAssertID := idwrap.NewNow()
	err = f.handler.httpAssertService.Create(ctx, &mhttp.HTTPAssert{
		ID:      baseAssertID,
		HttpID:  httpID,
		Value:   "base-key == 'base-value'",
		Enabled: true,
	})
	require.NoError(t, err)

	// Call DeltaInsert — this creates a new delta child record on the delta HTTP
	newValue := "delta-value"
	deltaAssertID := idwrap.NewNow()
	_, err = f.handler.HttpAssertDeltaInsert(ctx, connect.NewRequest(&apiv1.HttpAssertDeltaInsertRequest{
		Items: []*apiv1.HttpAssertDeltaInsert{
			{
				HttpId:            deltaID.Bytes(),
				HttpAssertId:      baseAssertID.Bytes(),
				DeltaHttpAssertId: deltaAssertID.Bytes(),
				Value:             &newValue,
			},
		},
	}))
	require.NoError(t, err)

	// Verify
	assert, err := f.handler.httpAssertService.GetByID(ctx, deltaAssertID)
	require.NoError(t, err)
	require.True(t, assert.IsDelta)
	require.NotNil(t, assert.DeltaValue)
	require.Equal(t, newValue, *assert.DeltaValue)

	// Update Delta
	updatedValue := "updated-value"
	_, err = f.handler.HttpAssertDeltaUpdate(ctx, connect.NewRequest(&apiv1.HttpAssertDeltaUpdateRequest{
		Items: []*apiv1.HttpAssertDeltaUpdate{
			{
				DeltaHttpAssertId: deltaAssertID.Bytes(),
				Value: &apiv1.HttpAssertDeltaUpdate_ValueUnion{
					Kind:  apiv1.HttpAssertDeltaUpdate_ValueUnion_KIND_VALUE,
					Value: &updatedValue,
				},
			},
		},
	}))
	require.NoError(t, err)

	// Verify update
	assert, err = f.handler.httpAssertService.GetByID(ctx, deltaAssertID)
	require.NoError(t, err)
	require.NotNil(t, assert.DeltaValue)
	require.Equal(t, updatedValue, *assert.DeltaValue)

	// Delete Delta
	_, err = f.handler.HttpAssertDeltaDelete(ctx, connect.NewRequest(&apiv1.HttpAssertDeltaDeleteRequest{
		Items: []*apiv1.HttpAssertDeltaDelete{
			{
				DeltaHttpAssertId: deltaAssertID.Bytes(),
			},
		},
	}))
	require.NoError(t, err)

	// Verify delete
	_, err = f.handler.httpAssertService.GetByID(ctx, deltaAssertID)
	require.Error(t, err)
}

func TestHttpDelta_BodyFormData(t *testing.T) {
	t.Parallel()
	f := newHttpFixture(t)
	workspaceID := f.createWorkspace(t, "Test Workspace")
	ctx := f.ctx

	httpID := f.createHttp(t, workspaceID, "Base Request")

	deltaID := idwrap.NewNow()
	err := f.hs.Create(ctx, &mhttp.HTTP{
		ID:           deltaID,
		WorkspaceID:  workspaceID,
		IsDelta:      true,
		ParentHttpID: &httpID,
	})
	require.NoError(t, err)

	// Base Form
	baseFormID := idwrap.NewNow()
	err = f.handler.httpBodyFormService.Create(ctx, &mhttp.HTTPBodyForm{
		ID:      baseFormID,
		HttpID:  httpID,
		Enabled: true,
	})
	require.NoError(t, err)

	// Delta Form
	formID := idwrap.NewNow()
	err = f.handler.httpBodyFormService.Create(ctx, &mhttp.HTTPBodyForm{
		ID:                   formID,
		HttpID:               deltaID,
		IsDelta:              true,
		ParentHttpBodyFormID: &baseFormID, // Required by constraint
		Enabled:              true,
	})
	require.NoError(t, err)

	// Update
	newKey := "new-key"
	_, err = f.handler.HttpBodyFormDataDeltaUpdate(ctx, connect.NewRequest(&apiv1.HttpBodyFormDataDeltaUpdateRequest{
		Items: []*apiv1.HttpBodyFormDataDeltaUpdate{
			{
				DeltaHttpBodyFormDataId: formID.Bytes(),
				Key: &apiv1.HttpBodyFormDataDeltaUpdate_KeyUnion{
					Kind:  apiv1.HttpBodyFormDataDeltaUpdate_KeyUnion_KIND_VALUE,
					Value: &newKey,
				},
			},
		},
	}))
	require.NoError(t, err)

	form, err := f.handler.httpBodyFormService.GetByID(ctx, formID)
	require.NoError(t, err)
	require.NotNil(t, form.DeltaKey)
	require.Equal(t, newKey, *form.DeltaKey)

	// Delete
	_, err = f.handler.HttpBodyFormDataDeltaDelete(ctx, connect.NewRequest(&apiv1.HttpBodyFormDataDeltaDeleteRequest{
		Items: []*apiv1.HttpBodyFormDataDeltaDelete{
			{
				DeltaHttpBodyFormDataId: formID.Bytes(),
			},
		},
	}))
	require.NoError(t, err)

	_, err = f.handler.httpBodyFormService.GetByID(ctx, formID)
	require.Error(t, err)
}

func TestHttpDelta_BodyUrlEncoded(t *testing.T) {
	t.Parallel()
	f := newHttpFixture(t)
	workspaceID := f.createWorkspace(t, "Test Workspace")
	ctx := f.ctx

	httpID := f.createHttp(t, workspaceID, "Base Request")

	deltaID := idwrap.NewNow()
	err := f.hs.Create(ctx, &mhttp.HTTP{
		ID:           deltaID,
		WorkspaceID:  workspaceID,
		IsDelta:      true,
		ParentHttpID: &httpID,
	})
	require.NoError(t, err)

	// Base Url Encoded
	baseUrlID := idwrap.NewNow()
	err = f.handler.httpBodyUrlEncodedService.Create(ctx, &mhttp.HTTPBodyUrlencoded{
		ID:      baseUrlID,
		HttpID:  httpID,
		Enabled: true,
	})
	require.NoError(t, err)

	// Delta Url Encoded
	urlID := idwrap.NewNow()
	err = f.handler.httpBodyUrlEncodedService.Create(ctx, &mhttp.HTTPBodyUrlencoded{
		ID:                         urlID,
		HttpID:                     deltaID,
		IsDelta:                    true,
		ParentHttpBodyUrlEncodedID: &baseUrlID, // Required by constraint
		Enabled:                    true,
	})
	require.NoError(t, err)

	// Update
	newVal := "new-val"
	_, err = f.handler.HttpBodyUrlEncodedDeltaUpdate(ctx, connect.NewRequest(&apiv1.HttpBodyUrlEncodedDeltaUpdateRequest{
		Items: []*apiv1.HttpBodyUrlEncodedDeltaUpdate{
			{
				DeltaHttpBodyUrlEncodedId: urlID.Bytes(),
				Value: &apiv1.HttpBodyUrlEncodedDeltaUpdate_ValueUnion{
					Kind:  apiv1.HttpBodyUrlEncodedDeltaUpdate_ValueUnion_KIND_VALUE,
					Value: &newVal,
				},
			},
		},
	}))
	require.NoError(t, err)

	encoded, err := f.handler.httpBodyUrlEncodedService.GetByID(ctx, urlID)
	require.NoError(t, err)
	require.NotNil(t, encoded.DeltaValue)
	require.Equal(t, newVal, *encoded.DeltaValue)

	// Delete
	_, err = f.handler.HttpBodyUrlEncodedDeltaDelete(ctx, connect.NewRequest(&apiv1.HttpBodyUrlEncodedDeltaDeleteRequest{
		Items: []*apiv1.HttpBodyUrlEncodedDeltaDelete{
			{
				DeltaHttpBodyUrlEncodedId: urlID.Bytes(),
			},
		},
	}))
	require.NoError(t, err)

	_, err = f.handler.httpBodyUrlEncodedService.GetByID(ctx, urlID)
	require.Error(t, err)
}

func TestHttpDelta_SyncCoverage(t *testing.T) {
	t.Parallel()
	f := newHttpFixture(t)
	ctx := f.ctx

	go func() {
		_ = f.handler.streamHttpBodyRawDeltaSync(ctx, f.userID, func(resp *apiv1.HttpBodyRawDeltaSyncResponse) error { return nil })
	}()
	go func() {
		_ = f.handler.streamHttpAssertDeltaSync(ctx, f.userID, func(resp *apiv1.HttpAssertDeltaSyncResponse) error { return nil })
	}()
	go func() {
		_ = f.handler.streamHttpBodyFormDeltaSync(ctx, f.userID, func(resp *apiv1.HttpBodyFormDataDeltaSyncResponse) error { return nil })
	}()
	go func() {
		_ = f.handler.streamHttpBodyUrlEncodedDeltaSync(ctx, f.userID, func(resp *apiv1.HttpBodyUrlEncodedDeltaSyncResponse) error { return nil })
	}()
}
