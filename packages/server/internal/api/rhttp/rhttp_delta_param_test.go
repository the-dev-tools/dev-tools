package rhttp

import (
	"bytes"
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	httpv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
	globalv1 "the-dev-tools/spec/dist/buf/go/global/v1"
)

func TestHttpSearchParamDeltaCollection_ReturnsCorrectDeltas(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "delta-test-workspace")

	// 1. Setup Base Request & SearchParam
	baseHttpID := f.createHttp(t, ws, "Base Request")

	baseParamID := idwrap.NewNow()
	baseParam := &mhttp.HTTPSearchParam{
		ID:      baseParamID,
		HttpID:  baseHttpID,
		Key:     "query",
		Value:   "base-value",
		Enabled: true,
		IsDelta: false,
	}
	require.NoError(t, f.handler.httpSearchParamService.Create(f.ctx, baseParam), "failed to create base param")

	// 2. Create Delta Param (Override)
	deltaHttpID := idwrap.NewNow()
	deltaHttp := &mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  ws,
		Name:         "Delta Request",
		ParentHttpID: &baseHttpID,
		IsDelta:      true,
	}
	require.NoError(t, f.hs.Create(f.ctx, deltaHttp), "failed to create delta http")

	deltaParamID := idwrap.NewNow()
	deltaValue := "delta-value"
	deltaOrder := float64(2.5)
	deltaParam := &mhttp.HTTPSearchParam{
		ID:                      deltaParamID,
		HttpID:                  deltaHttpID,   // The Delta Request this override belongs to
		ParentHttpSearchParamID: &baseParamID, // The Base Param this overrides
		IsDelta:                 true,
		DeltaValue:              &deltaValue,       // Override value
		DeltaDisplayOrder:       &deltaOrder,       // Override order
	}
	// Create the delta param
	require.NoError(t, f.handler.httpSearchParamService.Create(f.ctx, deltaParam), "failed to create delta param")

	// 3. Call RPC
	resp, err := f.handler.HttpSearchParamDeltaCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err, "HttpSearchParamDeltaCollection failed")

	// 4. Verify logic
	var foundDelta *httpv1.HttpSearchParamDelta
	for _, item := range resp.Msg.Items {
		if bytes.Equal(item.DeltaHttpSearchParamId, deltaParamID.Bytes()) {
			foundDelta = item
			break
		}
	}

	require.NotNil(t, foundDelta, "Delta param not found in response")

	// CHECK 1: HttpSearchParamId should be the PARENT ID (Base Param ID)
	require.True(t, bytes.Equal(foundDelta.HttpSearchParamId, baseParamID.Bytes()), "Expected HttpSearchParamId to be %s (Base), got %x", baseParamID, foundDelta.HttpSearchParamId)

	// CHECK 2: Value should be the delta override
	require.NotNil(t, foundDelta.Value, "Expected Value to be set")
	require.Equal(t, deltaValue, *foundDelta.Value, "Expected Value to be %s", deltaValue)

	// CHECK 3: Order should be the delta override
	require.NotNil(t, foundDelta.Order, "Expected Order to be set")
	require.Equal(t, float32(deltaOrder), *foundDelta.Order, "Expected Order to be %f", deltaOrder)

	// CHECK 4: Base param should NOT be returned as a delta
	for _, item := range resp.Msg.Items {
		require.False(t, bytes.Equal(item.DeltaHttpSearchParamId, baseParamID.Bytes()), "Base param incorrectly returned in Delta Collection")
	}
}

func TestHttpSearchParamDeltaInsert_CreatesNewDelta(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "delta-insert-workspace")

	// 1. Create Base HTTP and Base Param
	baseHttpID := f.createHttp(t, ws, "Base Request")
	baseParamID := idwrap.NewNow()
	baseParam := &mhttp.HTTPSearchParam{
		ID:      baseParamID,
		HttpID:  baseHttpID,
		Key:     "filter",
		Value:   "original",
		Enabled: true,
		IsDelta: false,
	}
	require.NoError(t, f.handler.httpSearchParamService.Create(f.ctx, baseParam), "failed to create base param")

	// 2. Create Delta HTTP
	deltaHttpID := idwrap.NewNow()
	deltaHttp := &mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  ws,
		Name:         "Delta Request",
		ParentHttpID: &baseHttpID,
		IsDelta:      true,
	}
	require.NoError(t, f.hs.Create(f.ctx, deltaHttp), "failed to create delta http")

	// 3. Create Delta Param pointing to Base
	deltaParamID := idwrap.NewNow()
	deltaParam := &mhttp.HTTPSearchParam{
		ID:                      deltaParamID,
		HttpID:                  deltaHttpID,
		ParentHttpSearchParamID: &baseParamID,
		IsDelta:                 true,
	}
	require.NoError(t, f.handler.httpSearchParamService.Create(f.ctx, deltaParam), "failed to create delta param")

	// 4. Call HttpSearchParamDeltaInsert to set delta values
	newValue := "overridden"
	newEnabled := false
	newDesc := "test description"
	newOrder := float32(1.5)

	insertReq := connect.NewRequest(&httpv1.HttpSearchParamDeltaInsertRequest{
		Items: []*httpv1.HttpSearchParamDeltaInsert{
			{
				HttpSearchParamId: baseParamID.Bytes(),
				Value:             &newValue,
				Enabled:           &newEnabled,
				Description:       &newDesc,
				Order:             &newOrder,
			},
		},
	})

	_, err := f.handler.HttpSearchParamDeltaInsert(f.ctx, insertReq)
	require.NoError(t, err, "HttpSearchParamDeltaInsert failed")

	// 5. Verify the delta was created with correct values
	updatedParam, err := f.handler.httpSearchParamService.GetByID(f.ctx, baseParamID)
	require.NoError(t, err, "failed to get updated param")

	require.NotNil(t, updatedParam.DeltaValue, "DeltaValue should be set")
	require.Equal(t, newValue, *updatedParam.DeltaValue, "DeltaValue should match")

	require.NotNil(t, updatedParam.DeltaEnabled, "DeltaEnabled should be set")
	require.Equal(t, newEnabled, *updatedParam.DeltaEnabled, "DeltaEnabled should match")

	require.NotNil(t, updatedParam.DeltaDescription, "DeltaDescription should be set")
	require.Equal(t, newDesc, *updatedParam.DeltaDescription, "DeltaDescription should match")

	require.NotNil(t, updatedParam.DeltaDisplayOrder, "DeltaDisplayOrder should be set")
	require.Equal(t, float64(newOrder), *updatedParam.DeltaDisplayOrder, "DeltaDisplayOrder should match")
}

func TestHttpSearchParamDeltaUpdate_UpdatesFields(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "delta-update-workspace")

	// 1. Create Base HTTP and Base Param
	baseHttpID := f.createHttp(t, ws, "Base Request")
	baseParamID := idwrap.NewNow()
	baseParam := &mhttp.HTTPSearchParam{
		ID:      baseParamID,
		HttpID:  baseHttpID,
		Key:     "page",
		Value:   "1",
		Enabled: true,
		IsDelta: false,
	}
	require.NoError(t, f.handler.httpSearchParamService.Create(f.ctx, baseParam), "failed to create base param")

	// 2. Create Delta HTTP
	deltaHttpID := idwrap.NewNow()
	deltaHttp := &mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  ws,
		Name:         "Delta Request",
		ParentHttpID: &baseHttpID,
		IsDelta:      true,
	}
	require.NoError(t, f.hs.Create(f.ctx, deltaHttp), "failed to create delta http")

	// 3. Create Delta Param with initial override
	deltaParamID := idwrap.NewNow()
	initialValue := "2"
	initialOrder := float64(3.0)
	deltaParam := &mhttp.HTTPSearchParam{
		ID:                      deltaParamID,
		HttpID:                  deltaHttpID,
		ParentHttpSearchParamID: &baseParamID,
		IsDelta:                 true,
		DeltaValue:              &initialValue,
		DeltaDisplayOrder:       &initialOrder,
	}
	require.NoError(t, f.handler.httpSearchParamService.Create(f.ctx, deltaParam), "failed to create delta param")

	// 4. Update multiple delta fields
	updatedValue := "3"
	updatedEnabled := false
	updatedDesc := "pagination override"
	updatedOrder := float32(5.5)

	updateReq := connect.NewRequest(&httpv1.HttpSearchParamDeltaUpdateRequest{
		Items: []*httpv1.HttpSearchParamDeltaUpdate{
			{
				DeltaHttpSearchParamId: deltaParamID.Bytes(),
				Value: &httpv1.HttpSearchParamDeltaUpdate_ValueUnion{
					Kind:  httpv1.HttpSearchParamDeltaUpdate_ValueUnion_KIND_VALUE,
					Value: &updatedValue,
				},
				Enabled: &httpv1.HttpSearchParamDeltaUpdate_EnabledUnion{
					Kind:  httpv1.HttpSearchParamDeltaUpdate_EnabledUnion_KIND_VALUE,
					Value: &updatedEnabled,
				},
				Description: &httpv1.HttpSearchParamDeltaUpdate_DescriptionUnion{
					Kind:  httpv1.HttpSearchParamDeltaUpdate_DescriptionUnion_KIND_VALUE,
					Value: &updatedDesc,
				},
				Order: &httpv1.HttpSearchParamDeltaUpdate_OrderUnion{
					Kind:  httpv1.HttpSearchParamDeltaUpdate_OrderUnion_KIND_VALUE,
					Value: &updatedOrder,
				},
			},
		},
	})

	_, err := f.handler.HttpSearchParamDeltaUpdate(f.ctx, updateReq)
	require.NoError(t, err, "HttpSearchParamDeltaUpdate failed")

	// 5. Verify all fields were updated
	updatedParam, err := f.handler.httpSearchParamService.GetByID(f.ctx, deltaParamID)
	require.NoError(t, err, "failed to get updated param")

	require.NotNil(t, updatedParam.DeltaValue, "DeltaValue should be set")
	require.Equal(t, updatedValue, *updatedParam.DeltaValue, "DeltaValue should be updated")

	require.NotNil(t, updatedParam.DeltaEnabled, "DeltaEnabled should be set")
	require.Equal(t, updatedEnabled, *updatedParam.DeltaEnabled, "DeltaEnabled should be updated")

	require.NotNil(t, updatedParam.DeltaDescription, "DeltaDescription should be set")
	require.Equal(t, updatedDesc, *updatedParam.DeltaDescription, "DeltaDescription should be updated")

	require.NotNil(t, updatedParam.DeltaDisplayOrder, "DeltaDisplayOrder should be set")
	require.Equal(t, float64(updatedOrder), *updatedParam.DeltaDisplayOrder, "DeltaDisplayOrder should be updated")
}

func TestHttpSearchParamDeltaUpdate_UnsetValue(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "delta-unset-workspace")

	// 1. Create Base HTTP and Base Param
	baseHttpID := f.createHttp(t, ws, "Base Request")
	baseParamID := idwrap.NewNow()
	baseParam := &mhttp.HTTPSearchParam{
		ID:      baseParamID,
		HttpID:  baseHttpID,
		Key:     "sort",
		Value:   "asc",
		Enabled: true,
		IsDelta: false,
	}
	require.NoError(t, f.handler.httpSearchParamService.Create(f.ctx, baseParam), "failed to create base param")

	// 2. Create Delta HTTP
	deltaHttpID := idwrap.NewNow()
	deltaHttp := &mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  ws,
		Name:         "Delta Request",
		ParentHttpID: &baseHttpID,
		IsDelta:      true,
	}
	require.NoError(t, f.hs.Create(f.ctx, deltaHttp), "failed to create delta http")

	// 3. Create Delta Param with override values
	deltaParamID := idwrap.NewNow()
	overrideValue := "desc"
	overrideEnabled := false
	overrideDesc := "custom sort"
	overrideOrder := float64(7.0)
	deltaParam := &mhttp.HTTPSearchParam{
		ID:                      deltaParamID,
		HttpID:                  deltaHttpID,
		ParentHttpSearchParamID: &baseParamID,
		IsDelta:                 true,
		DeltaValue:              &overrideValue,
		DeltaEnabled:            &overrideEnabled,
		DeltaDescription:        &overrideDesc,
		DeltaDisplayOrder:       &overrideOrder,
	}
	require.NoError(t, f.handler.httpSearchParamService.Create(f.ctx, deltaParam), "failed to create delta param")

	// 4. UNSET the value and description (revert to base)
	updateReq := connect.NewRequest(&httpv1.HttpSearchParamDeltaUpdateRequest{
		Items: []*httpv1.HttpSearchParamDeltaUpdate{
			{
				DeltaHttpSearchParamId: deltaParamID.Bytes(),
				Value: &httpv1.HttpSearchParamDeltaUpdate_ValueUnion{
					Kind:  httpv1.HttpSearchParamDeltaUpdate_ValueUnion_KIND_UNSET,
					Unset: globalv1.Unset_UNSET.Enum(),
				},
				Description: &httpv1.HttpSearchParamDeltaUpdate_DescriptionUnion{
					Kind:  httpv1.HttpSearchParamDeltaUpdate_DescriptionUnion_KIND_UNSET,
					Unset: globalv1.Unset_UNSET.Enum(),
				},
			},
		},
	})

	_, err := f.handler.HttpSearchParamDeltaUpdate(f.ctx, updateReq)
	require.NoError(t, err, "HttpSearchParamDeltaUpdate failed")

	// 5. Verify UNSET fields are nil, others remain
	updatedParam, err := f.handler.httpSearchParamService.GetByID(f.ctx, deltaParamID)
	require.NoError(t, err, "failed to get updated param")

	require.Nil(t, updatedParam.DeltaValue, "DeltaValue should be unset (nil)")
	require.Nil(t, updatedParam.DeltaDescription, "DeltaDescription should be unset (nil)")

	// These should remain unchanged
	require.NotNil(t, updatedParam.DeltaEnabled, "DeltaEnabled should still be set")
	require.Equal(t, overrideEnabled, *updatedParam.DeltaEnabled, "DeltaEnabled should be unchanged")

	require.NotNil(t, updatedParam.DeltaDisplayOrder, "DeltaDisplayOrder should still be set")
	require.Equal(t, overrideOrder, *updatedParam.DeltaDisplayOrder, "DeltaDisplayOrder should be unchanged")
}

func TestHttpSearchParamDeltaUpdate_DeltaOrder(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "delta-order-workspace")

	// 1. Create Base HTTP and Base Param
	baseHttpID := f.createHttp(t, ws, "Base Request")
	baseParamID := idwrap.NewNow()
	baseOrder := float64(1.0)
	baseParam := &mhttp.HTTPSearchParam{
		ID:           baseParamID,
		HttpID:       baseHttpID,
		Key:          "limit",
		Value:        "10",
		Enabled:      true,
		DisplayOrder: baseOrder,
		IsDelta:      false,
	}
	require.NoError(t, f.handler.httpSearchParamService.Create(f.ctx, baseParam), "failed to create base param")

	// 2. Create Delta HTTP
	deltaHttpID := idwrap.NewNow()
	deltaHttp := &mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  ws,
		Name:         "Delta Request",
		ParentHttpID: &baseHttpID,
		IsDelta:      true,
	}
	require.NoError(t, f.hs.Create(f.ctx, deltaHttp), "failed to create delta http")

	// 3. Create Delta Param with NO initial order override
	deltaParamID := idwrap.NewNow()
	deltaParam := &mhttp.HTTPSearchParam{
		ID:                      deltaParamID,
		HttpID:                  deltaHttpID,
		ParentHttpSearchParamID: &baseParamID,
		IsDelta:                 true,
		// No DeltaDisplayOrder initially
	}
	require.NoError(t, f.handler.httpSearchParamService.Create(f.ctx, deltaParam), "failed to create delta param")

	// 4. Update to set a new order
	newOrder := float32(10.5)
	updateReq := connect.NewRequest(&httpv1.HttpSearchParamDeltaUpdateRequest{
		Items: []*httpv1.HttpSearchParamDeltaUpdate{
			{
				DeltaHttpSearchParamId: deltaParamID.Bytes(),
				Order: &httpv1.HttpSearchParamDeltaUpdate_OrderUnion{
					Kind:  httpv1.HttpSearchParamDeltaUpdate_OrderUnion_KIND_VALUE,
					Value: &newOrder,
				},
			},
		},
	})

	_, err := f.handler.HttpSearchParamDeltaUpdate(f.ctx, updateReq)
	require.NoError(t, err, "HttpSearchParamDeltaUpdate failed")

	// 5. Verify Order field persists correctly
	updatedParam, err := f.handler.httpSearchParamService.GetByID(f.ctx, deltaParamID)
	require.NoError(t, err, "failed to get updated param")

	require.NotNil(t, updatedParam.DeltaDisplayOrder, "DeltaDisplayOrder should be set")
	require.Equal(t, float64(newOrder), *updatedParam.DeltaDisplayOrder, "DeltaDisplayOrder should match")

	// 6. Now UNSET the order
	unsetReq := connect.NewRequest(&httpv1.HttpSearchParamDeltaUpdateRequest{
		Items: []*httpv1.HttpSearchParamDeltaUpdate{
			{
				DeltaHttpSearchParamId: deltaParamID.Bytes(),
				Order: &httpv1.HttpSearchParamDeltaUpdate_OrderUnion{
					Kind:  httpv1.HttpSearchParamDeltaUpdate_OrderUnion_KIND_UNSET,
					Unset: globalv1.Unset_UNSET.Enum(),
				},
			},
		},
	})

	_, err = f.handler.HttpSearchParamDeltaUpdate(f.ctx, unsetReq)
	require.NoError(t, err, "HttpSearchParamDeltaUpdate unset failed")

	// 7. Verify Order is now nil
	finalParam, err := f.handler.httpSearchParamService.GetByID(f.ctx, deltaParamID)
	require.NoError(t, err, "failed to get final param")

	require.Nil(t, finalParam.DeltaDisplayOrder, "DeltaDisplayOrder should be unset (nil)")
}

func TestHttpSearchParamDeltaDelete_RemovesDelta(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "delta-delete-workspace")

	// 1. Create Base HTTP and Base Param
	baseHttpID := f.createHttp(t, ws, "Base Request")
	baseParamID := idwrap.NewNow()
	baseParam := &mhttp.HTTPSearchParam{
		ID:      baseParamID,
		HttpID:  baseHttpID,
		Key:     "category",
		Value:   "all",
		Enabled: true,
		IsDelta: false,
	}
	require.NoError(t, f.handler.httpSearchParamService.Create(f.ctx, baseParam), "failed to create base param")

	// 2. Create Delta HTTP
	deltaHttpID := idwrap.NewNow()
	deltaHttp := &mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  ws,
		Name:         "Delta Request",
		ParentHttpID: &baseHttpID,
		IsDelta:      true,
	}
	require.NoError(t, f.hs.Create(f.ctx, deltaHttp), "failed to create delta http")

	// 3. Create Delta Param
	deltaParamID := idwrap.NewNow()
	deltaValue := "books"
	deltaParam := &mhttp.HTTPSearchParam{
		ID:                      deltaParamID,
		HttpID:                  deltaHttpID,
		ParentHttpSearchParamID: &baseParamID,
		IsDelta:                 true,
		DeltaValue:              &deltaValue,
	}
	require.NoError(t, f.handler.httpSearchParamService.Create(f.ctx, deltaParam), "failed to create delta param")

	// 4. Verify delta exists
	existingParam, err := f.handler.httpSearchParamService.GetByID(f.ctx, deltaParamID)
	require.NoError(t, err, "delta param should exist before deletion")
	require.True(t, existingParam.IsDelta, "param should be a delta")

	// 5. Delete the delta
	deleteReq := connect.NewRequest(&httpv1.HttpSearchParamDeltaDeleteRequest{
		Items: []*httpv1.HttpSearchParamDeltaDelete{
			{
				DeltaHttpSearchParamId: deltaParamID.Bytes(),
			},
		},
	})

	_, err = f.handler.HttpSearchParamDeltaDelete(f.ctx, deleteReq)
	require.NoError(t, err, "HttpSearchParamDeltaDelete failed")

	// 6. Verify delta is deleted
	deletedParam, err := f.handler.httpSearchParamService.GetByID(f.ctx, deltaParamID)
	require.Error(t, err, "delta param should not exist after deletion")
	require.Nil(t, deletedParam, "deleted param should be nil")

	// 7. Verify base param still exists
	baseStillExists, err := f.handler.httpSearchParamService.GetByID(f.ctx, baseParamID)
	require.NoError(t, err, "base param should still exist")
	require.False(t, baseStillExists.IsDelta, "base param should not be a delta")
}

func TestHttpSearchParamDeltaUpdate_SparsePatchInDeltaPatchMap(t *testing.T) {
	t.Parallel()

	f := newHttpFixture(t)
	ws := f.createWorkspace(t, "sparse-patch-workspace")

	// 1. Create Base HTTP and Base Param
	baseHttpID := f.createHttp(t, ws, "Base Request")
	baseParamID := idwrap.NewNow()
	baseParam := &mhttp.HTTPSearchParam{
		ID:      baseParamID,
		HttpID:  baseHttpID,
		Key:     "search",
		Value:   "test",
		Enabled: true,
		IsDelta: false,
	}
	require.NoError(t, f.handler.httpSearchParamService.Create(f.ctx, baseParam), "failed to create base param")

	// 2. Create Delta HTTP
	deltaHttpID := idwrap.NewNow()
	deltaHttp := &mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  ws,
		Name:         "Delta Request",
		ParentHttpID: &baseHttpID,
		IsDelta:      true,
	}
	require.NoError(t, f.hs.Create(f.ctx, deltaHttp), "failed to create delta http")

	// 3. Create Delta Param with some overrides
	deltaParamID := idwrap.NewNow()
	deltaValue := "override"
	deltaOrder := float64(2.0)
	deltaParam := &mhttp.HTTPSearchParam{
		ID:                      deltaParamID,
		HttpID:                  deltaHttpID,
		ParentHttpSearchParamID: &baseParamID,
		IsDelta:                 true,
		DeltaValue:              &deltaValue,
		DeltaDisplayOrder:       &deltaOrder,
	}
	require.NoError(t, f.handler.httpSearchParamService.Create(f.ctx, deltaParam), "failed to create delta param")

	// 4. Setup sync stream to capture the event and verify patch
	stream := make(chan *httpv1.HttpSearchParamDeltaSyncResponse, 10)
	streamCtx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	go func() {
		err := f.handler.streamHttpSearchParamDeltaSync(streamCtx, f.userID, func(resp *httpv1.HttpSearchParamDeltaSyncResponse) error {
			stream <- resp
			return nil
		})
		if err != nil && err != context.Canceled {
			require.FailNow(t, "stream error: %v", err)
		}
	}()

	// Wait for stream to initialize
	time.Sleep(100 * time.Millisecond)

	// 5. Update ONLY the Order field (sparse update)
	newOrder := float32(15.0)
	updateReq := connect.NewRequest(&httpv1.HttpSearchParamDeltaUpdateRequest{
		Items: []*httpv1.HttpSearchParamDeltaUpdate{
			{
				DeltaHttpSearchParamId: deltaParamID.Bytes(),
				Order: &httpv1.HttpSearchParamDeltaUpdate_OrderUnion{
					Kind:  httpv1.HttpSearchParamDeltaUpdate_OrderUnion_KIND_VALUE,
					Value: &newOrder,
				},
				// NOTE: We're NOT updating Value, Enabled, Description, or Key
			},
		},
	})

	_, err := f.handler.HttpSearchParamDeltaUpdate(f.ctx, updateReq)
	require.NoError(t, err, "HttpSearchParamDeltaUpdate failed")

	// 6. Verify the sync event contains ONLY the updated field
	select {
	case resp := <-stream:
		items := resp.GetItems()
		require.NotEmpty(t, items, "should have at least one item in sync response")

		update := items[0].GetValue().GetUpdate()
		require.NotNil(t, update, "update should not be nil")
		require.Equal(t, deltaParamID.Bytes(), update.DeltaHttpSearchParamId, "delta ID should match")

		// The Order field should be present and updated
		require.NotNil(t, update.Order, "Order should be present in sync update")
		require.Equal(t, httpv1.HttpSearchParamDeltaSyncUpdate_OrderUnion_KIND_VALUE, update.Order.Kind, "Order kind should be VALUE")
		require.Equal(t, newOrder, update.Order.GetValue(), "Order value should match")

		// Other fields should be OMITTED (nil) in a sparse patch
		require.Nil(t, update.Value, "Value should be omitted in sparse patch")
		require.Nil(t, update.Key, "Key should be omitted in sparse patch")
		require.Nil(t, update.Enabled, "Enabled should be omitted in sparse patch")
		require.Nil(t, update.Description, "Description should be omitted in sparse patch")

	case <-time.After(2 * time.Second):
		require.FailNow(t, "timeout waiting for sync update event")
	}

	// 7. Verify persistence - Order changed, Value unchanged
	updatedParam, err := f.handler.httpSearchParamService.GetByID(f.ctx, deltaParamID)
	require.NoError(t, err, "failed to get updated param")

	require.NotNil(t, updatedParam.DeltaDisplayOrder, "DeltaDisplayOrder should be set")
	require.Equal(t, float64(newOrder), *updatedParam.DeltaDisplayOrder, "DeltaDisplayOrder should be updated")

	require.NotNil(t, updatedParam.DeltaValue, "DeltaValue should still be set")
	require.Equal(t, deltaValue, *updatedParam.DeltaValue, "DeltaValue should be unchanged")
}
