//nolint:revive // exported
package rgraphql

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/converter"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/mutation"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/patch"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sgraphql"

	graphqlv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/graph_q_l/v1"
)

// GraphQLAssert CRUD operations

func (s *GraphQLServiceRPC) GraphQLAssertCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[graphqlv1.GraphQLAssertCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	workspaces, err := s.wsReader.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allAsserts []*graphqlv1.GraphQLAssert
	for _, workspace := range workspaces {
		allGraphQLs, err := s.getGraphQLsWithDeltasForWorkspace(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		for _, graphql := range allGraphQLs {
			asserts, err := s.graphqlAssertService.GetByGraphQLID(ctx, graphql.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			for _, assert := range asserts {
				allAsserts = append(allAsserts, converter.ToAPIGraphQLAssert(assert))
			}
		}
	}

	return connect.NewResponse(&graphqlv1.GraphQLAssertCollectionResponse{Items: allAsserts}), nil
}

func (s *GraphQLServiceRPC) GraphQLAssertInsert(ctx context.Context, req *connect.Request[graphqlv1.GraphQLAssertInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one GraphQL assert must be provided"))
	}

	// FETCH: Gather data and check permissions OUTSIDE transaction
	type insertItem struct {
		assertID    idwrap.IDWrap
		graphqlID   idwrap.IDWrap
		value       string
		enabled     bool
		order       float32
		workspaceID idwrap.IDWrap
	}
	insertData := make([]insertItem, 0, len(req.Msg.Items))

	for _, item := range req.Msg.Items {
		if len(item.GraphqlAssertId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("graphql_assert_id is required"))
		}
		if len(item.GraphqlId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("graphql_id is required"))
		}

		assertID, err := idwrap.NewFromBytes(item.GraphqlAssertId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		graphqlID, err := idwrap.NewFromBytes(item.GraphqlId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Verify the GraphQL entry exists and user has access - use pool service
		graphqlEntry, err := s.graphqlReader.Get(ctx, graphqlID)
		if err != nil {
			if errors.Is(err, sgraphql.ErrNoGraphQLFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// CHECK: Validate write access to the workspace
		if err := s.checkWorkspaceWriteAccess(ctx, graphqlEntry.WorkspaceID); err != nil {
			return nil, err
		}

		insertData = append(insertData, insertItem{
			assertID:    assertID,
			graphqlID:   graphqlID,
			value:       item.Value,
			enabled:     item.Enabled,
			order:       item.Order,
			workspaceID: graphqlEntry.WorkspaceID,
		})
	}

	// ACT: Insert asserts using mutation context with auto-publish
	mut := mutation.New(s.DB, mutation.WithPublisher(s.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	now := time.Now().UnixMilli()
	for _, data := range insertData {
		assert := mgraphql.GraphQLAssert{
			ID:           data.assertID,
			GraphQLID:    data.graphqlID,
			Value:        data.value,
			Enabled:      data.enabled,
			Description:  "",
			DisplayOrder: data.order,
		}

		if err := mut.InsertGraphQLAssert(ctx, mutation.GraphQLAssertInsertItem{
			ID:          data.assertID,
			GraphQLID:   data.graphqlID,
			WorkspaceID: data.workspaceID,
			IsDelta:     false,
			Params: gen.CreateGraphQLAssertParams{
				ID:           data.assertID.Bytes(),
				GraphqlID:    data.graphqlID.Bytes(),
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
			Entity:      mutation.EntityGraphQLAssert,
			Op:          mutation.OpInsert,
			ID:          data.assertID,
			ParentID:    data.graphqlID,
			WorkspaceID: data.workspaceID,
			Payload:     assert,
		})
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *GraphQLServiceRPC) GraphQLAssertUpdate(ctx context.Context, req *connect.Request[graphqlv1.GraphQLAssertUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one GraphQL assert must be provided"))
	}

	// FETCH: Process request data and perform all reads/checks OUTSIDE transaction
	type updateItem struct {
		existingAssert mgraphql.GraphQLAssert
		value          *string
		enabled        *bool
		order          *float32
		workspaceID    idwrap.IDWrap
	}
	updateData := make([]updateItem, 0, len(req.Msg.Items))

	for _, item := range req.Msg.Items {
		if len(item.GraphqlAssertId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("graphql_assert_id is required"))
		}

		assertID, err := idwrap.NewFromBytes(item.GraphqlAssertId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing assert - use pool service
		existingAssert, err := s.graphqlAssertService.GetByID(ctx, assertID)
		if err != nil {
			if errors.Is(err, sgraphql.ErrNoGraphQLAssertFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify the GraphQL entry exists and user has access - use pool service
		graphqlEntry, err := s.graphqlReader.Get(ctx, existingAssert.GraphQLID)
		if err != nil {
			if errors.Is(err, sgraphql.ErrNoGraphQLFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// CHECK: Validate write access to the workspace
		if err := s.checkWorkspaceWriteAccess(ctx, graphqlEntry.WorkspaceID); err != nil {
			return nil, err
		}

		updateData = append(updateData, updateItem{
			existingAssert: *existingAssert,
			value:          item.Value,
			enabled:        item.Enabled,
			order:          item.Order,
			workspaceID:    graphqlEntry.WorkspaceID,
		})
	}

	// ACT: Update asserts using mutation context with auto-publish
	mut := mutation.New(s.DB, mutation.WithPublisher(s.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	now := time.Now().UnixMilli()
	for _, data := range updateData {
		assert := data.existingAssert

		// Build patch with only changed fields
		assertPatch := patch.GraphQLAssertPatch{}

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

		if err := mut.UpdateGraphQLAssert(ctx, mutation.GraphQLAssertUpdateItem{
			ID:          assert.ID,
			GraphQLID:   assert.GraphQLID,
			WorkspaceID: data.workspaceID,
			IsDelta:     assert.IsDelta,
			Params: gen.UpdateGraphQLAssertParams{
				ID:           assert.ID.Bytes(),
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
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *GraphQLServiceRPC) GraphQLAssertDelete(ctx context.Context, req *connect.Request[graphqlv1.GraphQLAssertDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one GraphQL assert must be provided"))
	}

	// FETCH: Gather data and check permissions OUTSIDE transaction
	type deleteItem struct {
		ID          idwrap.IDWrap
		GraphQLID   idwrap.IDWrap
		WorkspaceID idwrap.IDWrap
		IsDelta     bool
	}
	deleteItems := make([]deleteItem, 0, len(req.Msg.Items))

	for _, item := range req.Msg.Items {
		if len(item.GraphqlAssertId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("graphql_assert_id is required"))
		}

		assertID, err := idwrap.NewFromBytes(item.GraphqlAssertId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing assert - use pool service
		existingAssert, err := s.graphqlAssertService.GetByID(ctx, assertID)
		if err != nil {
			if errors.Is(err, sgraphql.ErrNoGraphQLAssertFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify the GraphQL entry exists and user has access - use pool service
		graphqlEntry, err := s.graphqlReader.Get(ctx, existingAssert.GraphQLID)
		if err != nil {
			if errors.Is(err, sgraphql.ErrNoGraphQLFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// CHECK: Validate delete access to the workspace
		if err := s.checkWorkspaceDeleteAccess(ctx, graphqlEntry.WorkspaceID); err != nil {
			return nil, err
		}

		deleteItems = append(deleteItems, deleteItem{
			ID:          assertID,
			GraphQLID:   existingAssert.GraphQLID,
			WorkspaceID: graphqlEntry.WorkspaceID,
			IsDelta:     existingAssert.IsDelta,
		})
	}

	// ACT: Delete using mutation context with auto-publish
	mut := mutation.New(s.DB, mutation.WithPublisher(s.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	for _, item := range deleteItems {
		mut.Track(mutation.Event{
			Entity:      mutation.EntityGraphQLAssert,
			Op:          mutation.OpDelete,
			ID:          item.ID,
			ParentID:    item.GraphQLID,
			WorkspaceID: item.WorkspaceID,
			IsDelta:     item.IsDelta,
		})
		if err := mut.Queries().DeleteGraphQLAssert(ctx, item.ID.Bytes()); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *GraphQLServiceRPC) GraphQLAssertSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[graphqlv1.GraphQLAssertSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return s.streamGraphQLAssertSync(ctx, userID, stream.Send)
}

func (s *GraphQLServiceRPC) streamGraphQLAssertSync(ctx context.Context, userID idwrap.IDWrap, send func(*graphqlv1.GraphQLAssertSyncResponse) error) error {
	var workspaceSet sync.Map

	filter := func(topic GraphQLAssertTopic) bool {
		if _, ok := workspaceSet.Load(topic.WorkspaceID.String()); ok {
			return true
		}
		belongs, err := s.us.CheckUserBelongsToWorkspace(ctx, userID, topic.WorkspaceID)
		if err != nil || !belongs {
			return false
		}
		workspaceSet.Store(topic.WorkspaceID.String(), struct{}{})
		return true
	}

	converter := func(events []GraphQLAssertEvent) *graphqlv1.GraphQLAssertSyncResponse {
		var items []*graphqlv1.GraphQLAssertSync
		for _, event := range events {
			// Skip delta asserts (they have separate sync)
			if event.IsDelta {
				continue
			}
			if resp := graphqlAssertSyncResponseFrom(event); resp != nil && len(resp.Items) > 0 {
				items = append(items, resp.Items...)
			}
		}
		if len(items) == 0 {
			return nil
		}
		return &graphqlv1.GraphQLAssertSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(
		ctx,
		s.streamers.GraphQLAssert,
		filter,
		converter,
		send,
		nil,
	)
}

// Delta operations
func (s *GraphQLServiceRPC) GraphQLAssertDeltaCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[graphqlv1.GraphQLAssertDeltaCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := s.wsReader.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allDeltas []*graphqlv1.GraphQLAssertDelta
	for _, workspace := range workspaces {
		// Get GraphQL delta entries for this workspace
		graphqlList, err := s.graphqlReader.GetDeltasByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Get asserts for each GraphQL entry
		for _, graphql := range graphqlList {
			asserts, err := s.graphqlAssertService.GetByGraphQLID(ctx, graphql.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Convert to delta format
			for _, assert := range asserts {
				if !assert.IsDelta {
					continue
				}

				delta := &graphqlv1.GraphQLAssertDelta{
					DeltaGraphqlAssertId: assert.ID.Bytes(),
					GraphqlId:            assert.GraphQLID.Bytes(),
				}

				if assert.ParentGraphQLAssertID != nil {
					delta.GraphqlAssertId = assert.ParentGraphQLAssertID.Bytes()
				}

				// Only include delta fields if they exist
				if assert.DeltaValue != nil {
					delta.Value = assert.DeltaValue
				}
				if assert.DeltaEnabled != nil {
					delta.Enabled = assert.DeltaEnabled
				}
				if assert.DeltaDisplayOrder != nil {
					delta.Order = assert.DeltaDisplayOrder
				}

				allDeltas = append(allDeltas, delta)
			}
		}
	}

	return connect.NewResponse(&graphqlv1.GraphQLAssertDeltaCollectionResponse{
		Items: allDeltas,
	}), nil
}

func (s *GraphQLServiceRPC) GraphQLAssertDeltaInsert(ctx context.Context, req *connect.Request[graphqlv1.GraphQLAssertDeltaInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one delta item is required"))
	}

	// FETCH: Gather data and check permissions OUTSIDE transaction
	type insertItem struct {
		graphqlID   idwrap.IDWrap
		newID       idwrap.IDWrap
		parentID    idwrap.IDWrap
		workspaceID idwrap.IDWrap
		baseAssert  mgraphql.GraphQLAssert
		item        *graphqlv1.GraphQLAssertDeltaInsert
	}
	insertData := make([]insertItem, 0, len(req.Msg.Items))

	for _, item := range req.Msg.Items {
		if len(item.GraphqlId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("graphql_id is required for each delta item"))
		}

		graphqlID, err := idwrap.NewFromBytes(item.GraphqlId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		graphqlEntry, err := s.graphqlReader.Get(ctx, graphqlID)
		if err != nil {
			if errors.Is(err, sgraphql.ErrNoGraphQLFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if !graphqlEntry.IsDelta {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("specified GraphQL entry is not a delta"))
		}

		if err := s.checkWorkspaceWriteAccess(ctx, graphqlEntry.WorkspaceID); err != nil {
			return nil, err
		}

		if len(item.GraphqlAssertId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("graphql_assert_id is required"))
		}

		parentAssertID, err := idwrap.NewFromBytes(item.GraphqlAssertId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		baseAssert, err := s.graphqlAssertService.GetByID(ctx, parentAssertID)
		if err != nil {
			if errors.Is(err, sgraphql.ErrNoGraphQLAssertFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		newID := idwrap.NewNow()
		if len(item.DeltaGraphqlAssertId) > 0 {
			newID, err = idwrap.NewFromBytes(item.DeltaGraphqlAssertId)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
		}

		insertData = append(insertData, insertItem{
			graphqlID:   graphqlID,
			newID:       newID,
			parentID:    parentAssertID,
			workspaceID: graphqlEntry.WorkspaceID,
			baseAssert:  *baseAssert,
			item:        item,
		})
	}

	// ACT: Insert new delta records using mutation context
	mut := mutation.New(s.DB, mutation.WithPublisher(s.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	now := time.Now().UnixMilli()
	for _, data := range insertData {
		params := gen.CreateGraphQLAssertParams{
			ID:                     data.newID.Bytes(),
			GraphqlID:              data.graphqlID.Bytes(),
			Value:                  data.baseAssert.Value,
			Enabled:                data.baseAssert.Enabled,
			Description:            data.baseAssert.Description,
			DisplayOrder:           float64(data.baseAssert.DisplayOrder),
			ParentGraphqlAssertID:  data.parentID.Bytes(),
			IsDelta:                true,
			DeltaValue:             stringPtrToNullString(data.item.Value),
			DeltaEnabled:           boolPtrToNullBool(data.item.Enabled),
			DeltaDescription:       stringPtrToNullString(nil),
			DeltaDisplayOrder:      float32PtrToNullFloat64(data.item.Order),
			CreatedAt:              now,
			UpdatedAt:              now,
		}

		if err := mut.InsertGraphQLAssert(ctx, mutation.GraphQLAssertInsertItem{
			ID:          data.newID,
			GraphQLID:   data.graphqlID,
			WorkspaceID: data.workspaceID,
			IsDelta:     true,
			Params:      params,
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		assertService := s.graphqlAssertService.TX(mut.TX())
		updated, err := assertService.GetByID(ctx, data.newID)
		if err == nil {
			mut.UpdateLastEventPayload(*updated)
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *GraphQLServiceRPC) GraphQLAssertDeltaUpdate(ctx context.Context, req *connect.Request[graphqlv1.GraphQLAssertDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one GraphQL assert delta must be provided"))
	}

	// FETCH: Gather data and check permissions OUTSIDE transaction
	type updateItem struct {
		deltaID        idwrap.IDWrap
		existingAssert mgraphql.GraphQLAssert
		workspaceID    idwrap.IDWrap
		item           *graphqlv1.GraphQLAssertDeltaUpdate
	}
	updateData := make([]updateItem, 0, len(req.Msg.Items))

	for _, item := range req.Msg.Items {
		if len(item.DeltaGraphqlAssertId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delta_graphql_assert_id is required"))
		}

		deltaID, err := idwrap.NewFromBytes(item.DeltaGraphqlAssertId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing delta assert - use pool service
		existingAssert, err := s.graphqlAssertService.GetByID(ctx, deltaID)
		if err != nil {
			if errors.Is(err, sgraphql.ErrNoGraphQLAssertFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify this is actually a delta record
		if !existingAssert.IsDelta {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("specified GraphQL assert is not a delta"))
		}

		// Get the GraphQL entry to check workspace access - use pool service
		graphqlEntry, err := s.graphqlReader.Get(ctx, existingAssert.GraphQLID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// CHECK: Validate write access to the workspace
		if err := s.checkWorkspaceWriteAccess(ctx, graphqlEntry.WorkspaceID); err != nil {
			return nil, err
		}

		updateData = append(updateData, updateItem{
			deltaID:        deltaID,
			existingAssert: *existingAssert,
			workspaceID:    graphqlEntry.WorkspaceID,
			item:           item,
		})
	}

	// ACT: Update using mutation context
	mut := mutation.New(s.DB, mutation.WithPublisher(s.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	for _, data := range updateData {
		item := data.item
		deltaValue := data.existingAssert.DeltaValue
		deltaEnabled := data.existingAssert.DeltaEnabled
		deltaOrder := data.existingAssert.DeltaDisplayOrder
		var patchData patch.GraphQLAssertPatch

		if item.Value != nil {
			switch item.Value.GetKind() {
			case graphqlv1.GraphQLAssertDeltaUpdate_ValueUnion_KIND_UNSET:
				deltaValue = nil
				patchData.Value = patch.Unset[string]()
			case graphqlv1.GraphQLAssertDeltaUpdate_ValueUnion_KIND_VALUE:
				valueStr := item.Value.GetValue()
				deltaValue = &valueStr
				patchData.Value = patch.NewOptional(valueStr)
			}
		}
		if item.Enabled != nil {
			switch item.Enabled.GetKind() {
			case graphqlv1.GraphQLAssertDeltaUpdate_EnabledUnion_KIND_UNSET:
				deltaEnabled = nil
				patchData.Enabled = patch.Unset[bool]()
			case graphqlv1.GraphQLAssertDeltaUpdate_EnabledUnion_KIND_VALUE:
				enabledBool := item.Enabled.GetValue()
				deltaEnabled = &enabledBool
				patchData.Enabled = patch.NewOptional(enabledBool)
			}
		}
		if item.Order != nil {
			switch item.Order.GetKind() {
			case graphqlv1.GraphQLAssertDeltaUpdate_OrderUnion_KIND_UNSET:
				deltaOrder = nil
				patchData.Order = patch.Unset[float32]()
			case graphqlv1.GraphQLAssertDeltaUpdate_OrderUnion_KIND_VALUE:
				orderFloat := item.Order.GetValue()
				deltaOrder = &orderFloat
				patchData.Order = patch.NewOptional(orderFloat)
			}
		}

		assertService := s.graphqlAssertService.TX(mut.TX())
		if err := mut.UpdateGraphQLAssertDelta(ctx, mutation.GraphQLAssertDeltaUpdateItem{
			ID:          data.deltaID,
			GraphQLID:   data.existingAssert.GraphQLID,
			WorkspaceID: data.workspaceID,
			Params: gen.UpdateGraphQLAssertDeltaParams{
				ID:                data.deltaID.Bytes(),
				DeltaValue:        stringPtrToNullString(deltaValue),
				DeltaEnabled:      boolPtrToNullBool(deltaEnabled),
				DeltaDisplayOrder: float32PtrToNullFloat64(deltaOrder),
				UpdatedAt:         time.Now().UnixMilli(),
			},
			Patch: patchData,
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Update payload in tracked event
		updated, err := assertService.GetByID(ctx, data.deltaID)
		if err == nil {
			mut.UpdateLastEventPayload(*updated)
		}
	}

	if err := mut.Commit(ctx); err != nil { // Auto-publishes events!
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *GraphQLServiceRPC) GraphQLAssertDeltaDelete(ctx context.Context, req *connect.Request[graphqlv1.GraphQLAssertDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one GraphQL assert delta must be provided"))
	}

	// FETCH: Gather data and check permissions OUTSIDE transaction
	type deleteItem struct {
		deltaID     idwrap.IDWrap
		graphqlID   idwrap.IDWrap
		workspaceID idwrap.IDWrap
		assert      mgraphql.GraphQLAssert
	}
	deleteData := make([]deleteItem, 0, len(req.Msg.Items))

	for _, item := range req.Msg.Items {
		if len(item.DeltaGraphqlAssertId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delta_graphql_assert_id is required"))
		}

		deltaID, err := idwrap.NewFromBytes(item.DeltaGraphqlAssertId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing delta assert
		existingAssert, err := s.graphqlAssertService.GetByID(ctx, deltaID)
		if err != nil {
			if errors.Is(err, sgraphql.ErrNoGraphQLAssertFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify this is actually a delta record
		if !existingAssert.IsDelta {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("specified GraphQL assert is not a delta"))
		}

		// Get the GraphQL entry to check workspace access
		graphqlEntry, err := s.graphqlReader.Get(ctx, existingAssert.GraphQLID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Check delete access to the workspace
		if err := s.checkWorkspaceDeleteAccess(ctx, graphqlEntry.WorkspaceID); err != nil {
			return nil, err
		}

		deleteData = append(deleteData, deleteItem{
			deltaID:     deltaID,
			graphqlID:   existingAssert.GraphQLID,
			workspaceID: graphqlEntry.WorkspaceID,
			assert:      *existingAssert,
		})
	}

	// ACT: Execute deletes in transaction
	mut := mutation.New(s.DB, mutation.WithPublisher(s.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer mut.Rollback()

	for _, data := range deleteData {
		mut.Track(mutation.Event{
			Entity:      mutation.EntityGraphQLAssert,
			Op:          mutation.OpDelete,
			ID:          data.deltaID,
			ParentID:    data.graphqlID,
			WorkspaceID: data.workspaceID,
			IsDelta:     true,
			Payload:     data.assert,
		})
		if err := mut.Queries().DeleteGraphQLAssert(ctx, data.deltaID.Bytes()); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *GraphQLServiceRPC) GraphQLAssertDeltaSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[graphqlv1.GraphQLAssertDeltaSyncResponse]) error {
	// TODO: Implement streaming delta sync
	return nil
}

// Helper functions for null conversions
func stringPtrToNullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *s, Valid: true}
}

func boolPtrToNullBool(b *bool) sql.NullBool {
	if b == nil {
		return sql.NullBool{Valid: false}
	}
	return sql.NullBool{Bool: *b, Valid: true}
}

func float32PtrToNullFloat64(f *float32) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Float64: float64(*f), Valid: true}
}
