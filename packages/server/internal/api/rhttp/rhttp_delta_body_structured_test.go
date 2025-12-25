package rhttp

import (
	"bytes"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
	globalv1 "the-dev-tools/spec/dist/buf/go/global/v1"
)

// ============================================================================
// HttpBodyFormData Delta Tests
// ============================================================================

func TestHttpBodyFormDataDeltaCollection_ReturnsCorrectDeltas(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "delta-test-workspace")

	// 1. Setup Base Request & BodyForm
	baseHttpID := f.createHttp(t, ws, "Base Request")

	baseFormID := idwrap.NewNow()
	baseForm := &mhttp.HTTPBodyForm{
		ID:      baseFormID,
		HttpID:  baseHttpID,
		Key:     "field1",
		Value:   "value1",
		Enabled: true,
		IsDelta: false,
	}
	require.NoError(t, f.handler.httpBodyFormService.Create(f.ctx, baseForm), "failed to create base form")

	// 2. Create Delta HTTP Request with BodyForm Override
	deltaHttpID := idwrap.NewNow()
	deltaHttp := &mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  ws,
		Name:         "Delta Request",
		ParentHttpID: &baseHttpID,
		IsDelta:      true,
	}
	require.NoError(t, f.hs.Create(f.ctx, deltaHttp), "failed to create delta http")

	deltaFormID := idwrap.NewNow()
	deltaValue := "delta-value"
	deltaForm := &mhttp.HTTPBodyForm{
		ID:                   deltaFormID,
		HttpID:               deltaHttpID,   // The Delta Request this override belongs to
		ParentHttpBodyFormID: &baseFormID,   // The Base BodyForm this overrides
		IsDelta:              true,
		DeltaValue:           &deltaValue,   // Override
	}
	require.NoError(t, f.handler.httpBodyFormService.Create(f.ctx, deltaForm), "failed to create delta form")

	// 3. Call RPC
	resp, err := f.handler.HttpBodyFormDataDeltaCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err, "HttpBodyFormDataDeltaCollection failed")

	// 4. Verify logic
	var foundDelta *apiv1.HttpBodyFormDataDelta
	for _, item := range resp.Msg.Items {
		if bytes.Equal(item.DeltaHttpBodyFormDataId, deltaFormID.Bytes()) {
			foundDelta = item
			break
		}
	}

	require.NotNil(t, foundDelta, "Delta form not found in response")

	// CHECK 1: HttpBodyFormDataId should be the PARENT ID (Base Form ID)
	require.True(t, bytes.Equal(foundDelta.HttpBodyFormDataId, baseFormID.Bytes()), "Expected HttpBodyFormDataId to be %s (Base), got %x", baseFormID, foundDelta.HttpBodyFormDataId)

	// CHECK 2: Value should be the delta override
	require.NotNil(t, foundDelta.Value, "Expected Value to be set")
	require.Equal(t, deltaValue, *foundDelta.Value, "Expected Value to be %s", deltaValue)

	// CHECK 3: Base form should NOT be returned as a delta
	for _, item := range resp.Msg.Items {
		require.False(t, bytes.Equal(item.DeltaHttpBodyFormDataId, baseFormID.Bytes()), "Base form incorrectly returned in Delta Collection")
	}
}

func TestHttpBodyFormDataDeltaInsert_CreatesNewDelta(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "delta-test-workspace")

	// 1. Create Base Request with BodyForm
	baseHttpID := f.createHttp(t, ws, "Base Request")

	baseFormID := idwrap.NewNow()
	baseForm := &mhttp.HTTPBodyForm{
		ID:      baseFormID,
		HttpID:  baseHttpID,
		Key:     "field1",
		Value:   "value1",
		Enabled: true,
	}
	require.NoError(t, f.handler.httpBodyFormService.Create(f.ctx, baseForm), "create base form")

	// 2. Create Delta HTTP Request
	deltaHttpID := idwrap.NewNow()
	deltaHttp := &mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  ws,
		Name:         "Delta Request",
		ParentHttpID: &baseHttpID,
		IsDelta:      true,
	}
	require.NoError(t, f.hs.Create(f.ctx, deltaHttp), "create delta http")

	// 3. Create Delta BodyForm (empty delta, just the record)
	deltaFormID := idwrap.NewNow()
	deltaForm := &mhttp.HTTPBodyForm{
		ID:                   deltaFormID,
		HttpID:               deltaHttpID,
		ParentHttpBodyFormID: &baseFormID,
		IsDelta:              true,
	}
	require.NoError(t, f.handler.httpBodyFormService.Create(f.ctx, deltaForm), "create delta form")

	// 4. Call DeltaInsert to set delta fields
	newKey := "field1_override"
	newValue := "delta-value"
	enabled := true
	desc := "Override description"
	order := float32(1.5)

	req := &apiv1.HttpBodyFormDataDeltaInsertRequest{
		Items: []*apiv1.HttpBodyFormDataDeltaInsert{
			{
				HttpBodyFormDataId: baseFormID.Bytes(),
				Key:                &newKey,
				Value:              &newValue,
				Enabled:            &enabled,
				Description:        &desc,
				Order:              &order,
			},
		},
	}

	_, err := f.handler.HttpBodyFormDataDeltaInsert(f.ctx, connect.NewRequest(req))
	require.NoError(t, err, "DeltaInsert failed")

	// 5. Verify delta fields were set
	updatedForm, err := f.handler.httpBodyFormService.GetByID(f.ctx, baseFormID)
	require.NoError(t, err, "get updated form")

	require.NotNil(t, updatedForm.DeltaKey, "DeltaKey should be set")
	require.Equal(t, newKey, *updatedForm.DeltaKey)

	require.NotNil(t, updatedForm.DeltaValue, "DeltaValue should be set")
	require.Equal(t, newValue, *updatedForm.DeltaValue)

	require.NotNil(t, updatedForm.DeltaEnabled, "DeltaEnabled should be set")
	require.Equal(t, enabled, *updatedForm.DeltaEnabled)

	require.NotNil(t, updatedForm.DeltaDescription, "DeltaDescription should be set")
	require.Equal(t, desc, *updatedForm.DeltaDescription)

	require.NotNil(t, updatedForm.DeltaDisplayOrder, "DeltaDisplayOrder should be set")
	require.Equal(t, order, *updatedForm.DeltaDisplayOrder)
}

func TestHttpBodyFormDataDeltaUpdate_UpdatesFields(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "delta-test-workspace")

	// 1. Create Base Request with BodyForm
	baseHttpID := f.createHttp(t, ws, "Base Request")

	baseFormID := idwrap.NewNow()
	baseForm := &mhttp.HTTPBodyForm{
		ID:      baseFormID,
		HttpID:  baseHttpID,
		Key:     "field1",
		Value:   "value1",
		Enabled: true,
	}
	require.NoError(t, f.handler.httpBodyFormService.Create(f.ctx, baseForm), "create base form")

	// 2. Create Delta Request with Form Override
	deltaHttpID := idwrap.NewNow()
	deltaHttp := &mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  ws,
		Name:         "Delta Request",
		ParentHttpID: &baseHttpID,
		IsDelta:      true,
	}
	require.NoError(t, f.hs.Create(f.ctx, deltaHttp), "create delta http")

	deltaFormID := idwrap.NewNow()
	originalKey := "field1_delta"
	originalValue := "original-value"
	deltaForm := &mhttp.HTTPBodyForm{
		ID:                   deltaFormID,
		HttpID:               deltaHttpID,
		ParentHttpBodyFormID: &baseFormID,
		IsDelta:              true,
		DeltaKey:             &originalKey,
		DeltaValue:           &originalValue,
	}
	require.NoError(t, f.handler.httpBodyFormService.Create(f.ctx, deltaForm), "create delta form")

	// 3. Update delta fields
	newKey := "field1_updated"
	newValue := "updated-value"
	newEnabled := false
	newDesc := "Updated description"

	req := &apiv1.HttpBodyFormDataDeltaUpdateRequest{
		Items: []*apiv1.HttpBodyFormDataDeltaUpdate{
			{
				DeltaHttpBodyFormDataId: deltaFormID.Bytes(),
				Key: &apiv1.HttpBodyFormDataDeltaUpdate_KeyUnion{
					Kind:  apiv1.HttpBodyFormDataDeltaUpdate_KeyUnion_KIND_VALUE,
					Value: &newKey,
				},
				Value: &apiv1.HttpBodyFormDataDeltaUpdate_ValueUnion{
					Kind:  apiv1.HttpBodyFormDataDeltaUpdate_ValueUnion_KIND_VALUE,
					Value: &newValue,
				},
				Enabled: &apiv1.HttpBodyFormDataDeltaUpdate_EnabledUnion{
					Kind:  apiv1.HttpBodyFormDataDeltaUpdate_EnabledUnion_KIND_VALUE,
					Value: &newEnabled,
				},
				Description: &apiv1.HttpBodyFormDataDeltaUpdate_DescriptionUnion{
					Kind:  apiv1.HttpBodyFormDataDeltaUpdate_DescriptionUnion_KIND_VALUE,
					Value: &newDesc,
				},
			},
		},
	}

	_, err := f.handler.HttpBodyFormDataDeltaUpdate(f.ctx, connect.NewRequest(req))
	require.NoError(t, err, "DeltaUpdate failed")

	// 4. Verify updates persisted
	updated, err := f.handler.httpBodyFormService.GetByID(f.ctx, deltaFormID)
	require.NoError(t, err, "get updated form")

	require.NotNil(t, updated.DeltaKey)
	require.Equal(t, newKey, *updated.DeltaKey)

	require.NotNil(t, updated.DeltaValue)
	require.Equal(t, newValue, *updated.DeltaValue)

	require.NotNil(t, updated.DeltaEnabled)
	require.Equal(t, newEnabled, *updated.DeltaEnabled)

	require.NotNil(t, updated.DeltaDescription)
	require.Equal(t, newDesc, *updated.DeltaDescription)
}

func TestHttpBodyFormDataDeltaUpdate_UnsetValue(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "delta-test-workspace")

	// 1. Create Base Request with BodyForm
	baseHttpID := f.createHttp(t, ws, "Base Request")

	baseFormID := idwrap.NewNow()
	baseForm := &mhttp.HTTPBodyForm{
		ID:      baseFormID,
		HttpID:  baseHttpID,
		Key:     "field1",
		Value:   "value1",
		Enabled: true,
	}
	require.NoError(t, f.handler.httpBodyFormService.Create(f.ctx, baseForm), "create base form")

	// 2. Create Delta Request with Form Override
	deltaHttpID := idwrap.NewNow()
	deltaHttp := &mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  ws,
		Name:         "Delta Request",
		ParentHttpID: &baseHttpID,
		IsDelta:      true,
	}
	require.NoError(t, f.hs.Create(f.ctx, deltaHttp), "create delta http")

	deltaFormID := idwrap.NewNow()
	originalKey := "field1_delta"
	originalValue := "original-value"
	originalDesc := "original description"
	deltaForm := &mhttp.HTTPBodyForm{
		ID:                   deltaFormID,
		HttpID:               deltaHttpID,
		ParentHttpBodyFormID: &baseFormID,
		IsDelta:              true,
		DeltaKey:             &originalKey,
		DeltaValue:           &originalValue,
		DeltaDescription:     &originalDesc,
	}
	require.NoError(t, f.handler.httpBodyFormService.Create(f.ctx, deltaForm), "create delta form")

	// 3. UNSET the value field (sparse patch - only update value)
	req := &apiv1.HttpBodyFormDataDeltaUpdateRequest{
		Items: []*apiv1.HttpBodyFormDataDeltaUpdate{
			{
				DeltaHttpBodyFormDataId: deltaFormID.Bytes(),
				Value: &apiv1.HttpBodyFormDataDeltaUpdate_ValueUnion{
					Kind:  apiv1.HttpBodyFormDataDeltaUpdate_ValueUnion_KIND_UNSET,
					Unset: globalv1.Unset_UNSET.Enum(),
				},
				// Note: NOT updating Key or Description - they should persist
			},
		},
	}

	_, err := f.handler.HttpBodyFormDataDeltaUpdate(f.ctx, connect.NewRequest(req))
	require.NoError(t, err, "DeltaUpdate failed")

	// 4. Verify sparse patch worked correctly
	updated, err := f.handler.httpBodyFormService.GetByID(f.ctx, deltaFormID)
	require.NoError(t, err, "get updated form")

	// Value should be UNSET (nil)
	require.Nil(t, updated.DeltaValue, "DeltaValue should be unset")

	// Key and Description should persist (not affected by sparse patch)
	require.NotNil(t, updated.DeltaKey, "DeltaKey should persist")
	require.Equal(t, originalKey, *updated.DeltaKey)

	require.NotNil(t, updated.DeltaDescription, "DeltaDescription should persist")
	require.Equal(t, originalDesc, *updated.DeltaDescription)
}

func TestHttpBodyFormDataDeltaUpdate_DeltaOrder(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "delta-test-workspace")

	// 1. Create Base Request with BodyForm
	baseHttpID := f.createHttp(t, ws, "Base Request")

	baseFormID := idwrap.NewNow()
	baseForm := &mhttp.HTTPBodyForm{
		ID:      baseFormID,
		HttpID:  baseHttpID,
		Key:     "field1",
		Value:   "value1",
		Enabled: true,
	}
	require.NoError(t, f.handler.httpBodyFormService.Create(f.ctx, baseForm), "create base form")

	// 2. Create Delta Request with Form Override
	deltaHttpID := idwrap.NewNow()
	deltaHttp := &mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  ws,
		Name:         "Delta Request",
		ParentHttpID: &baseHttpID,
		IsDelta:      true,
	}
	require.NoError(t, f.hs.Create(f.ctx, deltaHttp), "create delta http")

	deltaFormID := idwrap.NewNow()
	deltaForm := &mhttp.HTTPBodyForm{
		ID:                   deltaFormID,
		HttpID:               deltaHttpID,
		ParentHttpBodyFormID: &baseFormID,
		IsDelta:              true,
	}
	require.NoError(t, f.handler.httpBodyFormService.Create(f.ctx, deltaForm), "create delta form")

	// 3. Set order field via DeltaUpdate
	initialOrder := float32(10.5)
	req := &apiv1.HttpBodyFormDataDeltaUpdateRequest{
		Items: []*apiv1.HttpBodyFormDataDeltaUpdate{
			{
				DeltaHttpBodyFormDataId: deltaFormID.Bytes(),
				Order: &apiv1.HttpBodyFormDataDeltaUpdate_OrderUnion{
					Kind:  apiv1.HttpBodyFormDataDeltaUpdate_OrderUnion_KIND_VALUE,
					Value: &initialOrder,
				},
			},
		},
	}

	_, err := f.handler.HttpBodyFormDataDeltaUpdate(f.ctx, connect.NewRequest(req))
	require.NoError(t, err, "DeltaUpdate failed")

	// 4. Verify order persisted
	updated, err := f.handler.httpBodyFormService.GetByID(f.ctx, deltaFormID)
	require.NoError(t, err, "get updated form")

	require.NotNil(t, updated.DeltaDisplayOrder, "DeltaDisplayOrder should be set")
	require.Equal(t, initialOrder, *updated.DeltaDisplayOrder)

	// 5. Update order to a new value
	newOrder := float32(20.75)
	req2 := &apiv1.HttpBodyFormDataDeltaUpdateRequest{
		Items: []*apiv1.HttpBodyFormDataDeltaUpdate{
			{
				DeltaHttpBodyFormDataId: deltaFormID.Bytes(),
				Order: &apiv1.HttpBodyFormDataDeltaUpdate_OrderUnion{
					Kind:  apiv1.HttpBodyFormDataDeltaUpdate_OrderUnion_KIND_VALUE,
					Value: &newOrder,
				},
			},
		},
	}

	_, err = f.handler.HttpBodyFormDataDeltaUpdate(f.ctx, connect.NewRequest(req2))
	require.NoError(t, err, "second DeltaUpdate failed")

	// 6. Verify new order persisted
	updated2, err := f.handler.httpBodyFormService.GetByID(f.ctx, deltaFormID)
	require.NoError(t, err, "get updated form second time")

	require.NotNil(t, updated2.DeltaDisplayOrder, "DeltaDisplayOrder should still be set")
	require.Equal(t, newOrder, *updated2.DeltaDisplayOrder)

	// 7. UNSET order
	req3 := &apiv1.HttpBodyFormDataDeltaUpdateRequest{
		Items: []*apiv1.HttpBodyFormDataDeltaUpdate{
			{
				DeltaHttpBodyFormDataId: deltaFormID.Bytes(),
				Order: &apiv1.HttpBodyFormDataDeltaUpdate_OrderUnion{
					Kind:  apiv1.HttpBodyFormDataDeltaUpdate_OrderUnion_KIND_UNSET,
					Unset: globalv1.Unset_UNSET.Enum(),
				},
			},
		},
	}

	_, err = f.handler.HttpBodyFormDataDeltaUpdate(f.ctx, connect.NewRequest(req3))
	require.NoError(t, err, "third DeltaUpdate (unset) failed")

	// 8. Verify order was unset
	updated3, err := f.handler.httpBodyFormService.GetByID(f.ctx, deltaFormID)
	require.NoError(t, err, "get updated form third time")

	require.Nil(t, updated3.DeltaDisplayOrder, "DeltaDisplayOrder should be unset")
}

func TestHttpBodyFormDataDeltaDelete_RemovesDelta(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "delta-test-workspace")

	// 1. Create Base Request with BodyForm
	baseHttpID := f.createHttp(t, ws, "Base Request")

	baseFormID := idwrap.NewNow()
	baseForm := &mhttp.HTTPBodyForm{
		ID:      baseFormID,
		HttpID:  baseHttpID,
		Key:     "field1",
		Value:   "value1",
		Enabled: true,
	}
	require.NoError(t, f.handler.httpBodyFormService.Create(f.ctx, baseForm), "create base form")

	// 2. Create Delta Request with Form Override
	deltaHttpID := idwrap.NewNow()
	deltaHttp := &mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  ws,
		Name:         "Delta Request",
		ParentHttpID: &baseHttpID,
		IsDelta:      true,
	}
	require.NoError(t, f.hs.Create(f.ctx, deltaHttp), "create delta http")

	deltaFormID := idwrap.NewNow()
	deltaValue := "delta-value"
	deltaForm := &mhttp.HTTPBodyForm{
		ID:                   deltaFormID,
		HttpID:               deltaHttpID,
		ParentHttpBodyFormID: &baseFormID,
		IsDelta:              true,
		DeltaValue:           &deltaValue,
	}
	require.NoError(t, f.handler.httpBodyFormService.Create(f.ctx, deltaForm), "create delta form")

	// 3. Verify delta exists
	_, err := f.handler.httpBodyFormService.GetByID(f.ctx, deltaFormID)
	require.NoError(t, err, "delta form should exist")

	// 4. Delete delta
	req := &apiv1.HttpBodyFormDataDeltaDeleteRequest{
		Items: []*apiv1.HttpBodyFormDataDeltaDelete{
			{
				DeltaHttpBodyFormDataId: deltaFormID.Bytes(),
			},
		},
	}

	_, err = f.handler.HttpBodyFormDataDeltaDelete(f.ctx, connect.NewRequest(req))
	require.NoError(t, err, "DeltaDelete failed")

	// 5. Verify delta was deleted
	_, err = f.handler.httpBodyFormService.GetByID(f.ctx, deltaFormID)
	require.Error(t, err, "delta form should not exist after deletion")

	// 6. Verify base form still exists
	baseFormAfter, err := f.handler.httpBodyFormService.GetByID(f.ctx, baseFormID)
	require.NoError(t, err, "base form should still exist")
	require.Equal(t, "field1", baseFormAfter.Key)
}

// ============================================================================
// HttpBodyUrlEncoded Delta Tests (ensuring order still works)
// ============================================================================

func TestHttpBodyUrlEncodedDeltaCollection_ReturnsCorrectDeltas(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "delta-test-workspace")

	// 1. Setup Base Request & BodyUrlEncoded
	baseHttpID := f.createHttp(t, ws, "Base Request")

	baseUrlEncodedID := idwrap.NewNow()
	baseUrlEncoded := &mhttp.HTTPBodyUrlencoded{
		ID:      baseUrlEncodedID,
		HttpID:  baseHttpID,
		Key:     "param1",
		Value:   "value1",
		Enabled: true,
		IsDelta: false,
	}
	require.NoError(t, f.handler.httpBodyUrlEncodedService.Create(f.ctx, baseUrlEncoded), "failed to create base url encoded")

	// 2. Create Delta HTTP Request with BodyUrlEncoded Override
	deltaHttpID := idwrap.NewNow()
	deltaHttp := &mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  ws,
		Name:         "Delta Request",
		ParentHttpID: &baseHttpID,
		IsDelta:      true,
	}
	require.NoError(t, f.hs.Create(f.ctx, deltaHttp), "failed to create delta http")

	deltaUrlEncodedID := idwrap.NewNow()
	deltaValue := "delta-value"
	deltaUrlEncoded := &mhttp.HTTPBodyUrlencoded{
		ID:                          deltaUrlEncodedID,
		HttpID:                      deltaHttpID,        // The Delta Request this override belongs to
		ParentHttpBodyUrlEncodedID:  &baseUrlEncodedID, // The Base BodyUrlEncoded this overrides
		IsDelta:                     true,
		DeltaValue:                  &deltaValue, // Override
	}
	require.NoError(t, f.handler.httpBodyUrlEncodedService.Create(f.ctx, deltaUrlEncoded), "failed to create delta url encoded")

	// 3. Call RPC
	resp, err := f.handler.HttpBodyUrlEncodedDeltaCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err, "HttpBodyUrlEncodedDeltaCollection failed")

	// 4. Verify logic
	var foundDelta *apiv1.HttpBodyUrlEncodedDelta
	for _, item := range resp.Msg.Items {
		if bytes.Equal(item.DeltaHttpBodyUrlEncodedId, deltaUrlEncodedID.Bytes()) {
			foundDelta = item
			break
		}
	}

	require.NotNil(t, foundDelta, "Delta url encoded not found in response")

	// CHECK 1: HttpBodyUrlEncodedId should be the PARENT ID (Base UrlEncoded ID)
	require.True(t, bytes.Equal(foundDelta.HttpBodyUrlEncodedId, baseUrlEncodedID.Bytes()), "Expected HttpBodyUrlEncodedId to be %s (Base), got %x", baseUrlEncodedID, foundDelta.HttpBodyUrlEncodedId)

	// CHECK 2: Value should be the delta override
	require.NotNil(t, foundDelta.Value, "Expected Value to be set")
	require.Equal(t, deltaValue, *foundDelta.Value, "Expected Value to be %s", deltaValue)

	// CHECK 3: Base url encoded should NOT be returned as a delta
	for _, item := range resp.Msg.Items {
		require.False(t, bytes.Equal(item.DeltaHttpBodyUrlEncodedId, baseUrlEncodedID.Bytes()), "Base url encoded incorrectly returned in Delta Collection")
	}
}

func TestHttpBodyUrlEncodedDeltaUpdate_DeltaOrder(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "delta-test-workspace")

	// 1. Create Base Request with BodyUrlEncoded
	baseHttpID := f.createHttp(t, ws, "Base Request")

	baseUrlEncodedID := idwrap.NewNow()
	baseUrlEncoded := &mhttp.HTTPBodyUrlencoded{
		ID:      baseUrlEncodedID,
		HttpID:  baseHttpID,
		Key:     "param1",
		Value:   "value1",
		Enabled: true,
	}
	require.NoError(t, f.handler.httpBodyUrlEncodedService.Create(f.ctx, baseUrlEncoded), "create base url encoded")

	// 2. Create Delta Request with BodyUrlEncoded Override
	deltaHttpID := idwrap.NewNow()
	deltaHttp := &mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  ws,
		Name:         "Delta Request",
		ParentHttpID: &baseHttpID,
		IsDelta:      true,
	}
	require.NoError(t, f.hs.Create(f.ctx, deltaHttp), "create delta http")

	deltaUrlEncodedID := idwrap.NewNow()
	deltaUrlEncoded := &mhttp.HTTPBodyUrlencoded{
		ID:                         deltaUrlEncodedID,
		HttpID:                     deltaHttpID,
		ParentHttpBodyUrlEncodedID: &baseUrlEncodedID,
		IsDelta:                    true,
	}
	require.NoError(t, f.handler.httpBodyUrlEncodedService.Create(f.ctx, deltaUrlEncoded), "create delta url encoded")

	// 3. Set order field via DeltaUpdate
	initialOrder := float32(5.25)
	req := &apiv1.HttpBodyUrlEncodedDeltaUpdateRequest{
		Items: []*apiv1.HttpBodyUrlEncodedDeltaUpdate{
			{
				DeltaHttpBodyUrlEncodedId: deltaUrlEncodedID.Bytes(),
				Order: &apiv1.HttpBodyUrlEncodedDeltaUpdate_OrderUnion{
					Kind:  apiv1.HttpBodyUrlEncodedDeltaUpdate_OrderUnion_KIND_VALUE,
					Value: &initialOrder,
				},
			},
		},
	}

	_, err := f.handler.HttpBodyUrlEncodedDeltaUpdate(f.ctx, connect.NewRequest(req))
	require.NoError(t, err, "DeltaUpdate failed")

	// 4. Verify order persisted
	updated, err := f.handler.httpBodyUrlEncodedService.GetByID(f.ctx, deltaUrlEncodedID)
	require.NoError(t, err, "get updated url encoded")

	require.NotNil(t, updated.DeltaDisplayOrder, "DeltaDisplayOrder should be set")
	require.Equal(t, initialOrder, *updated.DeltaDisplayOrder)

	// 5. UNSET order
	req2 := &apiv1.HttpBodyUrlEncodedDeltaUpdateRequest{
		Items: []*apiv1.HttpBodyUrlEncodedDeltaUpdate{
			{
				DeltaHttpBodyUrlEncodedId: deltaUrlEncodedID.Bytes(),
				Order: &apiv1.HttpBodyUrlEncodedDeltaUpdate_OrderUnion{
					Kind:  apiv1.HttpBodyUrlEncodedDeltaUpdate_OrderUnion_KIND_UNSET,
					Unset: globalv1.Unset_UNSET.Enum(),
				},
			},
		},
	}

	_, err = f.handler.HttpBodyUrlEncodedDeltaUpdate(f.ctx, connect.NewRequest(req2))
	require.NoError(t, err, "second DeltaUpdate (unset) failed")

	// 6. Verify order was unset
	updated2, err := f.handler.httpBodyUrlEncodedService.GetByID(f.ctx, deltaUrlEncodedID)
	require.NoError(t, err, "get updated url encoded second time")

	require.Nil(t, updated2.DeltaDisplayOrder, "DeltaDisplayOrder should be unset")
}

func TestHttpBodyFormDataDeltaUpdate_WithSync(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "delta-test-workspace")

	// 1. Create Base Request with BodyForm
	baseHttpID := f.createHttp(t, ws, "Base Request")

	baseFormID := idwrap.NewNow()
	baseForm := &mhttp.HTTPBodyForm{
		ID:      baseFormID,
		HttpID:  baseHttpID,
		Key:     "field1",
		Value:   "value1",
		Enabled: true,
	}
	require.NoError(t, f.handler.httpBodyFormService.Create(f.ctx, baseForm), "create base form")

	// 2. Create Delta Request with Form Override
	deltaHttpID := idwrap.NewNow()
	deltaHttp := &mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  ws,
		Name:         "Delta Request",
		ParentHttpID: &baseHttpID,
		IsDelta:      true,
	}
	require.NoError(t, f.hs.Create(f.ctx, deltaHttp), "create delta http")

	deltaFormID := idwrap.NewNow()
	originalValue := "original-value"
	deltaForm := &mhttp.HTTPBodyForm{
		ID:                   deltaFormID,
		HttpID:               deltaHttpID,
		ParentHttpBodyFormID: &baseFormID,
		IsDelta:              true,
		DeltaValue:           &originalValue,
	}
	require.NoError(t, f.handler.httpBodyFormService.Create(f.ctx, deltaForm), "create delta form")

	// 3. Start sync stream
	stream := make(chan *apiv1.HttpBodyFormDataDeltaSyncResponse, 10)
	sCtx, cancel := f.ctx, func() {}
	defer cancel()

	go func() {
		_ = f.handler.streamHttpBodyFormDeltaSync(sCtx, f.userID, func(resp *apiv1.HttpBodyFormDataDeltaSyncResponse) error {
			stream <- resp
			return nil
		})
	}()

	time.Sleep(50 * time.Millisecond)

	// 4. Update delta value
	newValue := "updated-value"
	req := &apiv1.HttpBodyFormDataDeltaUpdateRequest{
		Items: []*apiv1.HttpBodyFormDataDeltaUpdate{
			{
				DeltaHttpBodyFormDataId: deltaFormID.Bytes(),
				Value: &apiv1.HttpBodyFormDataDeltaUpdate_ValueUnion{
					Kind:  apiv1.HttpBodyFormDataDeltaUpdate_ValueUnion_KIND_VALUE,
					Value: &newValue,
				},
			},
		},
	}

	_, err := f.handler.HttpBodyFormDataDeltaUpdate(f.ctx, connect.NewRequest(req))
	require.NoError(t, err, "DeltaUpdate failed")

	// 5. Verify sync event
	select {
	case resp := <-stream:
		update := resp.Items[0].GetValue().GetUpdate()
		require.Equal(t, deltaFormID.Bytes(), update.DeltaHttpBodyFormDataId)
		require.NotNil(t, update.Value)
		require.Equal(t, newValue, update.Value.GetValue())
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for sync event")
	}
}
