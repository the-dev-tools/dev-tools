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
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sgraphql"
	graphqlv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/graph_q_l/v1"
)

func (s *GraphQLServiceRPC) GraphQLCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[graphqlv1.GraphQLCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	workspaces, err := s.wsReader.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allItems []*graphqlv1.GraphQL
	for _, ws := range workspaces {
		items, err := s.graphqlService.GetByWorkspaceID(ctx, ws.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, item := range items {
			allItems = append(allItems, ToAPIGraphQL(item))
		}
	}

	return connect.NewResponse(&graphqlv1.GraphQLCollectionResponse{Items: allItems}), nil
}

func (s *GraphQLServiceRPC) GraphQLInsert(ctx context.Context, req *connect.Request[graphqlv1.GraphQLInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one item must be provided"))
	}

	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	workspaces, err := s.wsReader.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if len(workspaces) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("user has no workspaces"))
	}

	defaultWorkspaceID := workspaces[0].ID
	if err := s.checkWorkspaceWriteAccess(ctx, defaultWorkspaceID); err != nil {
		return nil, err
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback() //nolint:errcheck

	txGraphqlService := s.graphqlService.TX(tx)

	for _, item := range req.Msg.Items {
		if len(item.GraphqlId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("graphql_id is required"))
		}

		gqlID, err := idwrap.NewFromBytes(item.GraphqlId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		model := &mgraphql.GraphQL{
			ID:          gqlID,
			WorkspaceID: defaultWorkspaceID,
			Name:        item.Name,
			Url:         item.Url,
			Query:       item.Query,
			Variables:   item.Variables,
		}

		if err := txGraphqlService.Create(ctx, model); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if s.streamers.GraphQL != nil {
			s.streamers.GraphQL.Publish(GraphQLTopic{WorkspaceID: defaultWorkspaceID}, GraphQLEvent{
				Type:    eventTypeInsert,
				GraphQL: ToAPIGraphQL(*model),
			})
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *GraphQLServiceRPC) GraphQLUpdate(ctx context.Context, req *connect.Request[graphqlv1.GraphQLUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one item must be provided"))
	}

	for _, item := range req.Msg.Items {
		if len(item.GraphqlId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("graphql_id is required"))
		}

		gqlID, err := idwrap.NewFromBytes(item.GraphqlId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		existing, err := s.graphqlService.Get(ctx, gqlID)
		if err != nil {
			if errors.Is(err, sgraphql.ErrNoGraphQLFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.checkWorkspaceWriteAccess(ctx, existing.WorkspaceID); err != nil {
			return nil, err
		}

		if item.Name != nil {
			existing.Name = *item.Name
		}
		if item.Url != nil {
			existing.Url = *item.Url
		}
		if item.Query != nil {
			existing.Query = *item.Query
		}
		if item.Variables != nil {
			existing.Variables = *item.Variables
		}

		if err := s.graphqlService.Update(ctx, existing); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if s.streamers.GraphQL != nil {
			s.streamers.GraphQL.Publish(GraphQLTopic{WorkspaceID: existing.WorkspaceID}, GraphQLEvent{
				Type:    eventTypeUpdate,
				GraphQL: ToAPIGraphQL(*existing),
			})
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *GraphQLServiceRPC) GraphQLDelete(ctx context.Context, req *connect.Request[graphqlv1.GraphQLDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one item must be provided"))
	}

	for _, item := range req.Msg.Items {
		if len(item.GraphqlId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("graphql_id is required"))
		}

		gqlID, err := idwrap.NewFromBytes(item.GraphqlId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		existing, err := s.graphqlService.Get(ctx, gqlID)
		if err != nil {
			if errors.Is(err, sgraphql.ErrNoGraphQLFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.checkWorkspaceDeleteAccess(ctx, existing.WorkspaceID); err != nil {
			return nil, err
		}

		if err := s.graphqlService.Delete(ctx, gqlID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if s.streamers.GraphQL != nil {
			s.streamers.GraphQL.Publish(GraphQLTopic{WorkspaceID: existing.WorkspaceID}, GraphQLEvent{
				Type:    eventTypeDelete,
				GraphQL: &graphqlv1.GraphQL{GraphqlId: gqlID.Bytes()},
			})
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}
