//nolint:revive // exported
package rhttp

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/converter"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/mutation"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/patch"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	apiv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/http/v1"
)

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

	// FETCH: Gather data and check permissions OUTSIDE transaction
	type insertItem struct {
		assertID    idwrap.IDWrap
		httpID      idwrap.IDWrap
		value       string
		enabled     bool
		order       float32
		workspaceID idwrap.IDWrap
	}
	insertData := make([]insertItem, 0, len(req.Msg.Items))

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

		// CHECK: Validate write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		insertData = append(insertData, insertItem{
			assertID:    assertID,
			httpID:      httpID,
			value:       item.Value,
			enabled:     item.Enabled,
			order:       item.Order,
			workspaceID: httpEntry.WorkspaceID,
		})
	}

	// ACT: Insert asserts using mutation context with auto-publish
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	now := time.Now().UnixMilli()
	for _, data := range insertData {
		assert := mhttp.HTTPAssert{
			ID:           data.assertID,
			HttpID:       data.httpID,
			Value:        data.value,
			Enabled:      data.enabled,
			Description:  "",
			DisplayOrder: data.order,
		}

		if err := mut.InsertHTTPAssert(ctx, mutation.HTTPAssertInsertItem{
			ID:          data.assertID,
			HttpID:      data.httpID,
			WorkspaceID: data.workspaceID,
			IsDelta:     false,
			Params: gen.CreateHTTPAssertParams{
				ID:           data.assertID,
				HttpID:       data.httpID,
				Value:        data.value,
				Enabled:      data.enabled,
				Description:  "",
				DisplayOrder: float64(data.order),
				IsDelta:      false,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPAssert,
			Op:          mutation.OpInsert,
			ID:          data.assertID,
			ParentID:    data.httpID,
			WorkspaceID: data.workspaceID,
			Payload:     assert,
		})
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpAssertUpdate(ctx context.Context, req *connect.Request[apiv1.HttpAssertUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP assert must be provided"))
	}

	// FETCH: Process request data and perform all reads/checks OUTSIDE transaction
	type updateItem struct {
		existingAssert mhttp.HTTPAssert
		value          *string
		enabled        *bool
		order          *float32
		workspaceID    idwrap.IDWrap
	}
	updateData := make([]updateItem, 0, len(req.Msg.Items))

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

		// CHECK: Validate write access to the workspace
		if err := h.checkWorkspaceWriteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		updateData = append(updateData, updateItem{
			existingAssert: *existingAssert,
			value:          item.Value,
			enabled:        item.Enabled,
			order:          item.Order,
			workspaceID:    httpEntry.WorkspaceID,
		})
	}

	// ACT: Update asserts using mutation context with auto-publish
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	now := time.Now().UnixMilli()
	for _, data := range updateData {
		assert := data.existingAssert

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

		if err := mut.UpdateHTTPAssert(ctx, mutation.HTTPAssertUpdateItem{
			ID:          assert.ID,
			HttpID:      assert.HttpID,
			WorkspaceID: data.workspaceID,
			IsDelta:     assert.IsDelta,
			Params: gen.UpdateHTTPAssertParams{
				ID:           assert.ID,
				Value:        assert.Value,
				Enabled:      assert.Enabled,
				Description:  assert.Description,
				DisplayOrder: float64(assert.DisplayOrder),
				UpdatedAt:    now,
			},
			Patch: assertPatch,
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPAssert,
			Op:          mutation.OpUpdate,
			ID:          assert.ID,
			ParentID:    assert.HttpID,
			WorkspaceID: data.workspaceID,
			Payload:     assert,
			Patch:       assertPatch,
		})
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (h *HttpServiceRPC) HttpAssertDelete(ctx context.Context, req *connect.Request[apiv1.HttpAssertDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one HTTP assert must be provided"))
	}

	// FETCH: Gather data and check permissions OUTSIDE transaction
	type deleteItem struct {
		ID          idwrap.IDWrap
		HttpID      idwrap.IDWrap
		WorkspaceID idwrap.IDWrap
		IsDelta     bool
	}
	deleteItems := make([]deleteItem, 0, len(req.Msg.Items))

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

		// CHECK: Validate delete access to the workspace
		if err := h.checkWorkspaceDeleteAccess(ctx, httpEntry.WorkspaceID); err != nil {
			return nil, err
		}

		deleteItems = append(deleteItems, deleteItem{
			ID:          assertID,
			HttpID:      existingAssert.HttpID,
			WorkspaceID: httpEntry.WorkspaceID,
			IsDelta:     existingAssert.IsDelta,
		})
	}

	// ACT: Delete using mutation context with auto-publish
	mut := mutation.New(h.DB, mutation.WithPublisher(h.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	for _, item := range deleteItems {
		mut.Track(mutation.Event{
			Entity:      mutation.EntityHTTPAssert,
			Op:          mutation.OpDelete,
			ID:          item.ID,
			ParentID:    item.HttpID,
			WorkspaceID: item.WorkspaceID,
			IsDelta:     item.IsDelta,
		})
		if err := mut.Queries().DeleteHTTPAssert(ctx, item.ID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}
