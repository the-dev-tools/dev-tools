//nolint:revive // exported
package rgraphql

import (
	"context"
	"errors"
	"sync"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/mutation"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sgraphql"

	graphqlv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/graph_q_l/v1"
)

// GraphQLDeltaCollection fetches all delta GraphQL entries for the user's workspaces
func (s *GraphQLServiceRPC) GraphQLDeltaCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[graphqlv1.GraphQLDeltaCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := s.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allDeltas []*graphqlv1.GraphQLDelta
	for _, workspace := range workspaces {
		// Get GraphQL delta entries for this workspace
		graphqlList, err := s.graphqlService.Reader().GetDeltasByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Convert to delta format
		for _, gql := range graphqlList {
			delta := &graphqlv1.GraphQLDelta{
				DeltaGraphqlId: gql.ID.Bytes(),
			}

			if gql.ParentGraphQLID != nil {
				delta.GraphqlId = gql.ParentGraphQLID.Bytes()
			}

			// Only include delta fields if they exist
			if gql.DeltaName != nil {
				delta.Name = gql.DeltaName
			}
			if gql.DeltaUrl != nil {
				delta.Url = gql.DeltaUrl
			}
			if gql.DeltaQuery != nil {
				delta.Query = gql.DeltaQuery
			}
			if gql.DeltaVariables != nil {
				delta.Variables = gql.DeltaVariables
			}

			allDeltas = append(allDeltas, delta)
		}
	}

	return connect.NewResponse(&graphqlv1.GraphQLDeltaCollectionResponse{
		Items: allDeltas,
	}), nil
}

// GraphQLDeltaInsert creates new delta GraphQL entries
func (s *GraphQLServiceRPC) GraphQLDeltaInsert(ctx context.Context, req *connect.Request[graphqlv1.GraphQLDeltaInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one delta item is required"))
	}

	// Process each delta item
	for _, item := range req.Msg.Items {
		if len(item.GraphqlId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("graphql_id is required for each delta item"))
		}

		graphqlID, err := idwrap.NewFromBytes(item.GraphqlId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Check workspace write access
		graphqlEntry, err := s.graphqlService.Reader().Get(ctx, graphqlID)
		if err != nil {
			if errors.Is(err, sgraphql.ErrNoGraphQLFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.checkWorkspaceWriteAccess(ctx, graphqlEntry.WorkspaceID); err != nil {
			return nil, err
		}

		var deltaID idwrap.IDWrap
		if len(item.DeltaGraphqlId) > 0 {
			var err error
			deltaID, err = idwrap.NewFromBytes(item.DeltaGraphqlId)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
		} else {
			deltaID = idwrap.NewNow()
		}

		// Create delta GraphQL entry
		deltaGraphQL := &mgraphql.GraphQL{
			ID:              deltaID,
			WorkspaceID:     graphqlEntry.WorkspaceID,
			FolderID:        graphqlEntry.FolderID,
			Name:            graphqlEntry.Name,
			Url:             graphqlEntry.Url,
			Query:           graphqlEntry.Query,
			Variables:       graphqlEntry.Variables,
			Description:     graphqlEntry.Description,
			ParentGraphQLID: &graphqlID,
			IsDelta:         true,
			DeltaName:       item.Name,
			DeltaUrl:        item.Url,
			DeltaQuery:      item.Query,
			DeltaVariables:  item.Variables,
			CreatedAt:       0, // Will be set by service
			UpdatedAt:       0, // Will be set by service
		}

		// Use mutation pattern for create with auto-publish
		mut := mutation.New(s.DB, mutation.WithPublisher(s.mutationPublisher()))
		if err := mut.Begin(ctx); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		err = s.graphqlService.TX(mut.TX()).Create(ctx, deltaGraphQL)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := mut.Commit(ctx); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// GraphQLDeltaUpdate updates existing delta GraphQL entries
func (s *GraphQLServiceRPC) GraphQLDeltaUpdate(ctx context.Context, req *connect.Request[graphqlv1.GraphQLDeltaUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one GraphQL delta must be provided"))
	}

	// Process each delta item
	for _, item := range req.Msg.Items {
		if len(item.DeltaGraphqlId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delta_graphql_id is required"))
		}

		deltaID, err := idwrap.NewFromBytes(item.DeltaGraphqlId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing delta GraphQL entry
		existingDelta, err := s.graphqlService.Reader().Get(ctx, deltaID)
		if err != nil {
			if errors.Is(err, sgraphql.ErrNoGraphQLFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify this is actually a delta record
		if !existingDelta.IsDelta {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("specified GraphQL entry is not a delta"))
		}

		// Check write access to the workspace
		if err := s.checkWorkspaceWriteAccess(ctx, existingDelta.WorkspaceID); err != nil {
			return nil, err
		}

		// Apply updates
		if item.Name != nil {
			switch item.Name.GetKind() {
			case graphqlv1.GraphQLDeltaUpdate_NameUnion_KIND_UNSET:
				existingDelta.DeltaName = nil
			case graphqlv1.GraphQLDeltaUpdate_NameUnion_KIND_VALUE:
				nameStr := item.Name.GetValue()
				existingDelta.DeltaName = &nameStr
			}
		}
		if item.Url != nil {
			switch item.Url.GetKind() {
			case graphqlv1.GraphQLDeltaUpdate_UrlUnion_KIND_UNSET:
				existingDelta.DeltaUrl = nil
			case graphqlv1.GraphQLDeltaUpdate_UrlUnion_KIND_VALUE:
				urlStr := item.Url.GetValue()
				existingDelta.DeltaUrl = &urlStr
			}
		}
		if item.Query != nil {
			switch item.Query.GetKind() {
			case graphqlv1.GraphQLDeltaUpdate_QueryUnion_KIND_UNSET:
				existingDelta.DeltaQuery = nil
			case graphqlv1.GraphQLDeltaUpdate_QueryUnion_KIND_VALUE:
				queryStr := item.Query.GetValue()
				existingDelta.DeltaQuery = &queryStr
			}
		}
		if item.Variables != nil {
			switch item.Variables.GetKind() {
			case graphqlv1.GraphQLDeltaUpdate_VariablesUnion_KIND_UNSET:
				existingDelta.DeltaVariables = nil
			case graphqlv1.GraphQLDeltaUpdate_VariablesUnion_KIND_VALUE:
				variablesStr := item.Variables.GetValue()
				existingDelta.DeltaVariables = &variablesStr
			}
		}

		// Use mutation pattern for update with auto-publish
		mut := mutation.New(s.DB, mutation.WithPublisher(s.mutationPublisher()))
		if err := mut.Begin(ctx); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.graphqlService.TX(mut.TX()).Update(ctx, existingDelta); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := mut.Commit(ctx); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// GraphQLDeltaDelete deletes delta GraphQL entries
func (s *GraphQLServiceRPC) GraphQLDeltaDelete(ctx context.Context, req *connect.Request[graphqlv1.GraphQLDeltaDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one GraphQL delta must be provided"))
	}

	// Step 1: Gather data and check permissions OUTSIDE transaction
	var deleteData []struct {
		deltaID       idwrap.IDWrap
		existingDelta *mgraphql.GraphQL
	}

	for _, item := range req.Msg.Items {
		if len(item.DeltaGraphqlId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delta_graphql_id is required"))
		}

		deltaID, err := idwrap.NewFromBytes(item.DeltaGraphqlId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing delta GraphQL entry
		existingDelta, err := s.graphqlService.Reader().Get(ctx, deltaID)
		if err != nil {
			if errors.Is(err, sgraphql.ErrNoGraphQLFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Verify this is actually a delta record
		if !existingDelta.IsDelta {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("specified GraphQL entry is not a delta"))
		}

		// Check write access to the workspace
		if err := s.checkWorkspaceWriteAccess(ctx, existingDelta.WorkspaceID); err != nil {
			return nil, err
		}

		deleteData = append(deleteData, struct {
			deltaID       idwrap.IDWrap
			existingDelta *mgraphql.GraphQL
		}{
			deltaID:       deltaID,
			existingDelta: existingDelta,
		})
	}

	// Step 2: Execute deletes in transaction using mutation pattern
	mut := mutation.New(s.DB, mutation.WithPublisher(s.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, data := range deleteData {
		if err := s.graphqlService.TX(mut.TX()).Delete(ctx, data.deltaID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// GraphQLDeltaSync streams delta GraphQL changes in real-time
func (s *GraphQLServiceRPC) GraphQLDeltaSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[graphqlv1.GraphQLDeltaSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}
	return s.streamGraphQLDeltaSync(ctx, userID, stream.Send)
}

func (s *GraphQLServiceRPC) streamGraphQLDeltaSync(ctx context.Context, userID idwrap.IDWrap, send func(*graphqlv1.GraphQLDeltaSyncResponse) error) error {
	var workspaceSet sync.Map

	filter := func(topic GraphQLTopic) bool {
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

	converter := func(events []GraphQLEvent) *graphqlv1.GraphQLDeltaSyncResponse {
		var items []*graphqlv1.GraphQLDeltaSync
		for _, event := range events {
			if resp := graphqlDeltaSyncResponseFrom(event); resp != nil && len(resp.Items) > 0 {
				items = append(items, resp.Items...)
			}
		}
		if len(items) == 0 {
			return nil
		}
		return &graphqlv1.GraphQLDeltaSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(ctx, s.streamers.GraphQL, filter, converter, send, nil)
}
