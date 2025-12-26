//nolint:revive // exported
package rhttp

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/converter"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/patch"
	"the-dev-tools/server/pkg/txutil"

	"the-dev-tools/server/pkg/service/shttp"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

// assertWithWorkspace is a context carrier that pairs an assert with its workspace ID.
// This is needed because HTTPAssert doesn't store WorkspaceID directly, but we need it
// for topic extraction during bulk sync event publishing.
type assertWithWorkspace struct {
	assert      mhttp.HTTPAssert
	workspaceID idwrap.IDWrap
}

// publishBulkAssertInsert publishes multiple assert insert events in bulk.
// Items are already grouped by HttpAssertTopic by the BulkSyncTxInsert wrapper.
func (h *HttpServiceRPC) publishBulkAssertInsert(
	topic HttpAssertTopic,
	items []assertWithWorkspace,
) {
	// Convert to event slice for variadic publish
	events := make([]HttpAssertEvent, len(items))
	for i, item := range items {
		events[i] = HttpAssertEvent{
			Type:       eventTypeInsert,
			IsDelta:    item.assert.IsDelta,
			HttpAssert: converter.ToAPIHttpAssert(item.assert),
		}
	}

	// Single bulk publish for entire batch
	h.streamers.HttpAssert.Publish(topic, events...)
}

// publishBulkAssertUpdate publishes multiple assert update events in bulk.
// Items are already grouped by HttpAssertTopic by the BulkSyncTxUpdate wrapper.
func (h *HttpServiceRPC) publishBulkAssertUpdate(
	topic HttpAssertTopic,
	events []txutil.UpdateEvent[assertWithWorkspace, patch.HTTPAssertPatch],
) {
	assertEvents := make([]HttpAssertEvent, len(events))
	for i, evt := range events {
		assertEvents[i] = HttpAssertEvent{
			Type:       eventTypeUpdate,
			IsDelta:    evt.Item.assert.IsDelta,
			HttpAssert: converter.ToAPIHttpAssert(evt.Item.assert),
			Patch:      evt.Patch, // Partial updates preserved!
		}
	}
	h.streamers.HttpAssert.Publish(topic, assertEvents...)
}

// publishBulkAssertDelete publishes multiple assert delete events in bulk.
// Items are already grouped by HttpAssertTopic by the BulkSyncTxDelete wrapper.
func (h *HttpServiceRPC) publishBulkAssertDelete(
	topic HttpAssertTopic,
	events []txutil.DeleteEvent[idwrap.IDWrap],
) {
	assertEvents := make([]HttpAssertEvent, len(events))
	for i, evt := range events {
		assertEvents[i] = HttpAssertEvent{
			Type:    eventTypeDelete,
			IsDelta: evt.IsDelta,
			HttpAssert: &apiv1.HttpAssert{
				HttpAssertId: evt.ID.Bytes(),
			},
		}
	}
	h.streamers.HttpAssert.Publish(topic, assertEvents...)
}

func (h *HttpServiceRPC) HttpAssertCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpAssertCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allAsserts []*apiv1.HttpAssert
	for _, workspace := range workspaces {
		// Get HTTP entries for this workspace
		httpList, err := h.hs.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get asserts for each HTTP entry
		for _, http := range httpList {
			asserts, err := h.httpAssertService.GetByHttpID(ctx, http.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Convert to API format
			for _, assert := range asserts {
				apiAssert := converter.ToAPIHttpAssert(assert)
				allAsserts = append(allAsserts, apiAssert)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpAssertCollectionResponse{Items: allAsserts}), nil
}

func (h *HttpServiceRPC) HttpAssertInsert(ctx context.Context, req *connect.Request[apiv1.HttpAssertInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP assert must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var insertData []struct {
		assertModel *mhttp.HTTPAssert
		workspaceID idwrap.IDWrap
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpAssertId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_assert_id is required"))
		}
		if len(item.HttpId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_id is required"))
		}

		assertID, err := idwrap.NewFromBytes(item.HttpAssertId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		httpID, err := idwrap.NewFromBytes(item.HttpId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Verify the HTTP entry exists and user has access - use pool service
		httpEntry, err := h.httpReader.Get(ctx, httpID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHTTPFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		// Create the assert model
		assertModel := &mhttp.HTTPAssert{
			ID:           assertID,
			HttpID:       httpID,
			Value:        item.Value,
			Enabled:      item.Enabled,
			Description:  "",
			DisplayOrder: item.Order,
		}

		insertData = append(insertData, struct {
			assertModel *mhttp.HTTPAssert
			workspaceID idwrap.IDWrap
		}{
			assertModel: assertModel,
			workspaceID: httpEntry.WorkspaceID,
		})
	}

	// Step 2: Minimal write transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// Create bulk sync wrapper with topic extractor
	syncTx := txutil.NewBulkInsertTx[assertWithWorkspace, HttpAssertTopic](
		tx,
		func(aww assertWithWorkspace) HttpAssertTopic {
			return HttpAssertTopic{WorkspaceID: aww.workspaceID}
		},
	)

	assertWriter := shttp.NewAssertWriter(tx)

	for _, data := range insertData {
		if err := assertWriter.Create(ctx, data.assertModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Track with workspace context for bulk sync
		syncTx.Track(assertWithWorkspace{
			assert:      *data.assertModel,
			workspaceID: data.workspaceID,
		})
	}

	// Step 3: Commit and bulk publish sync events (grouped by topic)
	if err := syncTx.CommitAndPublish(ctx, h.publishBulkAssertInsert); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpAssertUpdate(ctx context.Context, req *connect.Request[apiv1.HttpAssertUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP assert must be provided"))
	}

	// Step 1: Process request data and perform all reads/checks OUTSIDE transaction
	var updateData []struct {
		existingAssert *mhttp.HTTPAssert
		value          *string
		enabled        *bool
		order          *float32
		workspaceID    idwrap.IDWrap
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpAssertId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_assert_id is required"))
		}

		assertID, err := idwrap.NewFromBytes(item.HttpAssertId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing assert - use pool service
		existingAssert, err := h.httpAssertService.GetByID(ctx, assertID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpAssertFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify the HTTP entry exists and user has access - use pool service
		httpEntry, err := h.httpReader.Get(ctx, existingAssert.HttpID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHTTPFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		updateData = append(updateData, struct {
			existingAssert *mhttp.HTTPAssert
			value          *string
			enabled        *bool
			order          *float32
			workspaceID    idwrap.IDWrap
		}{
			existingAssert: existingAssert,
			value:          item.Value,
			enabled:        item.Enabled,
			order:          item.Order,
			workspaceID:    httpEntry.WorkspaceID,
		})
	}

	// Step 2: Minimal write transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// Create bulk sync wrapper with topic extractor
	syncTx := txutil.NewBulkUpdateTx[assertWithWorkspace, patch.HTTPAssertPatch, HttpAssertTopic](
		tx,
		func(aww assertWithWorkspace) HttpAssertTopic {
			return HttpAssertTopic{WorkspaceID: aww.workspaceID}
		},
	)

	assertWriter := shttp.NewAssertWriter(tx)

	for _, data := range updateData {
		assert := *data.existingAssert

		// Build patch with only changed fields
		assertPatch := patch.HTTPAssertPatch{}

		// Update fields if provided and track in patch
		if data.value != nil {
			assert.Value = *data.value
			assertPatch.Value = patch.NewOptional(*data.value)
		}
		if data.enabled != nil {
			assert.Enabled = *data.enabled
			assertPatch.Enabled = patch.NewOptional(*data.enabled)
		}
		if data.order != nil {
			assert.DisplayOrder = *data.order
			assertPatch.Order = patch.NewOptional(*data.order)
		}

		if err := assertWriter.Update(ctx, &assert); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Track with workspace context and patch
		syncTx.Track(
			assertWithWorkspace{
				assert:      assert,
				workspaceID: data.workspaceID,
			},
			assertPatch,
		)
	}

	// Step 3: Commit and bulk publish (grouped by workspace)
	if err := syncTx.CommitAndPublish(ctx, h.publishBulkAssertUpdate); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpAssertDelete(ctx context.Context, req *connect.Request[apiv1.HttpAssertDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP assert must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var deleteData []struct {
		assertID       idwrap.IDWrap
		existingAssert *mhttp.HTTPAssert
		workspaceID    idwrap.IDWrap
	}

	for _, item := range req.Msg.Items {
		if len(item.HttpAssertId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http_assert_id is required"))
		}

		assertID, err := idwrap.NewFromBytes(item.HttpAssertId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing assert - use pool service
		existingAssert, err := h.httpAssertService.GetByID(ctx, assertID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHttpAssertFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify the HTTP entry exists and user has access - use pool service
		httpEntry, err := h.httpReader.Get(ctx, existingAssert.HttpID)
		if err != nil {
			if errors.Is(err, shttp.ErrNoHTTPFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check delete access to the workspace
		if err := h.checkWorkspaceDeleteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		deleteData = append(deleteData, struct {
			assertID       idwrap.IDWrap
			existingAssert *mhttp.HTTPAssert
			workspaceID    idwrap.IDWrap
		}{
			assertID:       assertID,
			existingAssert: existingAssert,
			workspaceID:    httpEntry.WorkspaceID,
		})
	}

	// Step 2: Minimal write transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// Create bulk sync wrapper with topic extractor
	syncTx := txutil.NewBulkDeleteTx[idwrap.IDWrap, HttpAssertTopic](
		tx,
		func(evt txutil.DeleteEvent[idwrap.IDWrap]) HttpAssertTopic {
			return HttpAssertTopic{WorkspaceID: evt.WorkspaceID}
		},
	)

	assertWriter := shttp.NewAssertWriter(tx)

	for _, data := range deleteData {
		if err := assertWriter.Delete(ctx, data.assertID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		syncTx.Track(data.assertID, data.workspaceID, data.existingAssert.IsDelta)
	}

	// Step 3: Commit and bulk publish (grouped by workspace)
	if err := syncTx.CommitAndPublish(ctx, h.publishBulkAssertDelete); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}
