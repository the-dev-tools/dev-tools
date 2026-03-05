//nolint:revive // exported
package rgraphql

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/mutation"

	graphqlv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/graph_q_l/v1"
)

// GraphQLHeaderDeltaCollection fetches all delta GraphQL headers for the user's workspaces
func (s *GraphQLServiceRPC) GraphQLHeaderDeltaCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[graphqlv1.GraphQLHeaderDeltaCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := s.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allDeltas []*graphqlv1.GraphQLHeaderDelta
	for _, workspace := range workspaces {
		// Get GraphQL header delta entries for this workspace
		headerList, err := s.headerService.GetDeltasByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Convert to delta format
		for _, header := range headerList {
			delta := &graphqlv1.GraphQLHeaderDelta{
				DeltaGraphqlHeaderId: header.ID.Bytes(),
				GraphqlId:            header.GraphQLID.Bytes(),
			}

			if header.ParentGraphQLHeaderID != nil {
				delta.GraphqlHeaderId = header.ParentGraphQLHeaderID.Bytes()
			}

			// Only include delta fields if they exist
			if header.DeltaKey != nil {
				delta.Key = header.DeltaKey
			}
			if header.DeltaValue != nil {
				delta.Value = header.DeltaValue
			}
			if header.DeltaEnabled != nil {
				delta.Enabled = header.DeltaEnabled
			}
			if header.DeltaDescription != nil {
				delta.Description = header.DeltaDescription
			}
			if header.DeltaDisplayOrder != nil {
				delta.Order = header.DeltaDisplayOrder
			}

			allDeltas = append(allDeltas, delta)
		}
	}

	return connect.NewResponse(&graphqlv1.GraphQLHeaderDeltaCollectionResponse{
		Items: allDeltas,
	}), nil
}

// GraphQLHeaderDeltaInsert creates new delta GraphQL header entries
func (s *GraphQLServiceRPC) GraphQLHeaderDeltaInsert(ctx context.Context, req *connect.Request[graphqlv1.GraphQLHeaderDeltaInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one delta item is required"))
	}

	// Process each delta item
	for _, item := range req.Msg.Items {
		if len(item.GraphqlHeaderId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("graphql_header_id is required for each delta item"))
		}
		if len(item.GraphqlId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("graphql_id is required for each delta item"))
		}

		headerID, err := idwrap.NewFromBytes(item.GraphqlHeaderId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		graphqlID, err := idwrap.NewFromBytes(item.GraphqlId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get parent header to copy base values
		parentHeaders, err := s.headerService.GetByIDs(ctx, []idwrap.IDWrap{headerID})
		if err != nil || len(parentHeaders) == 0 {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("parent header not found"))
		}
		parentHeader := parentHeaders[0]

		// Check workspace write access through the GraphQL entry
		workspaceID, err := s.graphqlService.Reader().GetWorkspaceID(ctx, graphqlID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.checkWorkspaceWriteAccess(ctx, workspaceID); err != nil {
			return nil, err
		}

		var deltaID idwrap.IDWrap
		if len(item.DeltaGraphqlHeaderId) > 0 {
			var err error
			deltaID, err = idwrap.NewFromBytes(item.DeltaGraphqlHeaderId)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
		} else {
			deltaID = idwrap.NewNow()
		}

		// Create delta GraphQL header entry
		deltaHeader := &mgraphql.GraphQLHeader{
			ID:                    deltaID,
			GraphQLID:             graphqlID,
			Key:                   parentHeader.Key,
			Value:                 parentHeader.Value,
			Enabled:               parentHeader.Enabled,
			Description:           parentHeader.Description,
			DisplayOrder:          parentHeader.DisplayOrder,
			ParentGraphQLHeaderID: &headerID,
			IsDelta:               true,
			DeltaKey:              item.Key,
			DeltaValue:            item.Value,
			DeltaEnabled:          item.Enabled,
			DeltaDescription:      item.Description,
			DeltaDisplayOrder:     item.Order,
			CreatedAt:             0, // Will be set by service
			UpdatedAt:             0, // Will be set by service
		}

		// Use mutation pattern for create with auto-publish
		mut := mutation.New(s.DB, mutation.WithPublisher(s.mutationPublisher()))
		if err := mut.Begin(ctx); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		err = s.headerService.TX(mut.TX()).Create(ctx, deltaHeader)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := mut.Commit(ctx); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// GraphQLHeaderDeltaUpdate updates existing delta GraphQL header entries
func (s *GraphQLServiceRPC) GraphQLHeaderDeltaUpdate(ctx context.Context, req *connect.Request[graphqlv1.GraphQLHeaderDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one GraphQL header delta must be provided"))
	}

	// Process each delta item
	for _, item := range req.Msg.Items {
		if len(item.DeltaGraphqlHeaderId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delta_graphql_header_id is required"))
		}

		deltaID, err := idwrap.NewFromBytes(item.DeltaGraphqlHeaderId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing delta GraphQL header entry
		existingDeltas, err := s.headerService.GetByIDs(ctx, []idwrap.IDWrap{deltaID})
		if err != nil || len(existingDeltas) == 0 {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("delta header not found"))
		}
		existingDelta := existingDeltas[0]

		// Verify this is actually a delta record
		if !existingDelta.IsDelta {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("specified GraphQL header entry is not a delta"))
		}

		// Check write access to the workspace
		workspaceID, err := s.graphqlService.Reader().GetWorkspaceID(ctx, existingDelta.GraphQLID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.checkWorkspaceWriteAccess(ctx, workspaceID); err != nil {
			return nil, err
		}

		// Apply updates to delta fields
		if item.Key != nil {
			switch item.Key.GetKind() {
			case graphqlv1.GraphQLHeaderDeltaUpdate_KeyUnion_KIND_UNSET:
				existingDelta.DeltaKey = nil
			case graphqlv1.GraphQLHeaderDeltaUpdate_KeyUnion_KIND_VALUE:
				keyStr := item.Key.GetValue()
				existingDelta.DeltaKey = &keyStr
			}
		}

		if item.Value != nil {
			switch item.Value.GetKind() {
			case graphqlv1.GraphQLHeaderDeltaUpdate_ValueUnion_KIND_UNSET:
				existingDelta.DeltaValue = nil
			case graphqlv1.GraphQLHeaderDeltaUpdate_ValueUnion_KIND_VALUE:
				valueStr := item.Value.GetValue()
				existingDelta.DeltaValue = &valueStr
			}
		}

		if item.Enabled != nil {
			switch item.Enabled.GetKind() {
			case graphqlv1.GraphQLHeaderDeltaUpdate_EnabledUnion_KIND_UNSET:
				existingDelta.DeltaEnabled = nil
			case graphqlv1.GraphQLHeaderDeltaUpdate_EnabledUnion_KIND_VALUE:
				enabledVal := item.Enabled.GetValue()
				existingDelta.DeltaEnabled = &enabledVal
			}
		}

		if item.Description != nil {
			switch item.Description.GetKind() {
			case graphqlv1.GraphQLHeaderDeltaUpdate_DescriptionUnion_KIND_UNSET:
				existingDelta.DeltaDescription = nil
			case graphqlv1.GraphQLHeaderDeltaUpdate_DescriptionUnion_KIND_VALUE:
				descStr := item.Description.GetValue()
				existingDelta.DeltaDescription = &descStr
			}
		}

		if item.Order != nil {
			switch item.Order.GetKind() {
			case graphqlv1.GraphQLHeaderDeltaUpdate_OrderUnion_KIND_UNSET:
				existingDelta.DeltaDisplayOrder = nil
			case graphqlv1.GraphQLHeaderDeltaUpdate_OrderUnion_KIND_VALUE:
				orderVal := item.Order.GetValue()
				existingDelta.DeltaDisplayOrder = &orderVal
			}
		}

		// Use mutation pattern for update with auto-publish
		mut := mutation.New(s.DB, mutation.WithPublisher(s.mutationPublisher()))
		if err := mut.Begin(ctx); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		err = s.headerService.TX(mut.TX()).Update(ctx, &existingDelta)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := mut.Commit(ctx); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// GraphQLHeaderDeltaDelete deletes delta GraphQL header entries
func (s *GraphQLServiceRPC) GraphQLHeaderDeltaDelete(ctx context.Context, req *connect.Request[graphqlv1.GraphQLHeaderDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one GraphQL header delta must be provided"))
	}

	// Process each delta item
	for _, item := range req.Msg.Items {
		if len(item.DeltaGraphqlHeaderId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delta_graphql_header_id is required"))
		}

		deltaID, err := idwrap.NewFromBytes(item.DeltaGraphqlHeaderId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing delta GraphQL header entry
		existingDeltas, err := s.headerService.GetByIDs(ctx, []idwrap.IDWrap{deltaID})
		if err != nil || len(existingDeltas) == 0 {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("delta header not found"))
		}
		existingDelta := existingDeltas[0]

		// Verify this is actually a delta record
		if !existingDelta.IsDelta {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("specified GraphQL header entry is not a delta"))
		}

		// Check delete access to the workspace
		workspaceID, err := s.graphqlService.Reader().GetWorkspaceID(ctx, existingDelta.GraphQLID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.checkWorkspaceDeleteAccess(ctx, workspaceID); err != nil {
			return nil, err
		}

		// Use mutation pattern for delete with auto-publish
		mut := mutation.New(s.DB, mutation.WithPublisher(s.mutationPublisher()))
		if err := mut.Begin(ctx); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		err = s.headerService.TX(mut.TX()).Delete(ctx, deltaID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := mut.Commit(ctx); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// GraphQLHeaderDeltaSync streams delta header changes to the client
func (s *GraphQLServiceRPC) GraphQLHeaderDeltaSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[graphqlv1.GraphQLHeaderDeltaSyncResponse]) error {
	// TODO: Implement streaming delta sync with proper event filtering
	// Similar to GraphQLDeltaSync, this requires a delta-specific event stream
	// that only publishes delta-related changes to prevent flooding clients
	// with non-delta header updates.
	return nil
}
