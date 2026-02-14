//nolint:revive // exported
package rgraphql

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	devtoolsdb "github.com/the-dev-tools/dev-tools/packages/db"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/httpclient"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/menv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sgraphql"
	graphqlv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/graph_q_l/v1"
)

const introspectionQuery = `query IntrospectionQuery {
  __schema {
    queryType { name }
    mutationType { name }
    subscriptionType { name }
    types {
      ...FullType
    }
    directives {
      name
      description
      locations
      args {
        ...InputValue
      }
    }
  }
}

fragment FullType on __Type {
  kind
  name
  description
  fields(includeDeprecated: true) {
    name
    description
    args {
      ...InputValue
    }
    type {
      ...TypeRef
    }
    isDeprecated
    deprecationReason
  }
  inputFields {
    ...InputValue
  }
  interfaces {
    ...TypeRef
  }
  enumValues(includeDeprecated: true) {
    name
    description
    isDeprecated
    deprecationReason
  }
  possibleTypes {
    ...TypeRef
  }
}

fragment InputValue on __InputValue {
  name
  description
  type { ...TypeRef }
  defaultValue
}

fragment TypeRef on __Type {
  kind
  name
  ofType {
    kind
    name
    ofType {
      kind
      name
      ofType {
        kind
        name
        ofType {
          kind
          name
          ofType {
            kind
            name
            ofType {
              kind
              name
            }
          }
        }
      }
    }
  }
}`

func (s *GraphQLServiceRPC) GraphQLRun(ctx context.Context, req *connect.Request[graphqlv1.GraphQLRunRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GraphqlId) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("graphql_id is required"))
	}

	gqlID, err := idwrap.NewFromBytes(req.Msg.GraphqlId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	gqlEntry, err := s.graphqlService.Get(ctx, gqlID)
	if err != nil {
		if errors.Is(err, sgraphql.ErrNoGraphQLFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := s.checkWorkspaceReadAccess(ctx, gqlEntry.WorkspaceID); err != nil {
		return nil, err
	}

	// Build variable map from workspace env
	varMap, err := s.buildWorkspaceVarMap(ctx, gqlEntry.WorkspaceID)
	if err != nil {
		varMap = make(map[string]any)
	}

	// Get headers
	headers, err := s.headerService.GetByGraphQLID(ctx, gqlID)
	if err != nil {
		headers = []mgraphql.GraphQLHeader{}
	}

	// Build and execute GraphQL request
	httpReq, err := prepareGraphQLRequest(gqlEntry, headers, varMap)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("failed to prepare request: %w", err))
	}

	client := httpclient.New()
	startTime := time.Now()

	resp, err := client.Do(httpReq.WithContext(ctx))
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("request failed: %w", err))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read response: %w", err))
	}

	duration := time.Since(startTime).Milliseconds()

	// Store response
	responseID := idwrap.NewNow()
	nowUnix := time.Now().Unix()

	gqlResponse := mgraphql.GraphQLResponse{
		ID:        responseID,
		GraphQLID: gqlID,
		Status:    int32(resp.StatusCode), //nolint:gosec
		Body:      body,
		Time:      startTime.Unix(),
		Duration:  int32(duration), //nolint:gosec
		Size:      int32(len(body)), //nolint:gosec
		CreatedAt: nowUnix,
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	txResponseService := s.responseService.TX(tx)

	if err := txResponseService.Create(ctx, gqlResponse); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Store response headers
	var respHeaderEvents []GraphQLResponseHeaderEvent
	for key, values := range resp.Header {
		for _, val := range values {
			headerID := idwrap.NewNow()
			respHeader := mgraphql.GraphQLResponseHeader{
				ID:          headerID,
				ResponseID:  responseID,
				HeaderKey:   key,
				HeaderValue: val,
				CreatedAt:   nowUnix,
			}
			if err := txResponseService.CreateHeader(ctx, respHeader); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			respHeaderEvents = append(respHeaderEvents, GraphQLResponseHeaderEvent{
				Type:                  eventTypeInsert,
				GraphQLResponseHeader: ToAPIGraphQLResponseHeader(respHeader),
			})
		}
	}

	// Update last_run_at
	now := time.Now().Unix()
	gqlEntry.LastRunAt = &now
	txGraphqlService := s.graphqlService.TX(tx)
	if err := txGraphqlService.Update(ctx, gqlEntry); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish events
	if s.streamers.GraphQLResponse != nil {
		s.streamers.GraphQLResponse.Publish(GraphQLResponseTopic{WorkspaceID: gqlEntry.WorkspaceID}, GraphQLResponseEvent{
			Type:            eventTypeInsert,
			GraphQLResponse: ToAPIGraphQLResponse(gqlResponse),
		})
	}
	if s.streamers.GraphQLResponseHeader != nil {
		topic := GraphQLResponseHeaderTopic{WorkspaceID: gqlEntry.WorkspaceID}
		for _, evt := range respHeaderEvents {
			s.streamers.GraphQLResponseHeader.Publish(topic, evt)
		}
	}
	if s.streamers.GraphQL != nil {
		s.streamers.GraphQL.Publish(GraphQLTopic{WorkspaceID: gqlEntry.WorkspaceID}, GraphQLEvent{
			Type:    eventTypeUpdate,
			GraphQL: ToAPIGraphQL(*gqlEntry),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *GraphQLServiceRPC) GraphQLDuplicate(ctx context.Context, req *connect.Request[graphqlv1.GraphQLDuplicateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GraphqlId) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("graphql_id is required"))
	}

	gqlID, err := idwrap.NewFromBytes(req.Msg.GraphqlId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	gqlEntry, err := s.graphqlService.Get(ctx, gqlID)
	if err != nil {
		if errors.Is(err, sgraphql.ErrNoGraphQLFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := s.checkWorkspaceWriteAccess(ctx, gqlEntry.WorkspaceID); err != nil {
		return nil, err
	}

	// Read headers outside TX
	headers, err := s.headerService.GetByGraphQLID(ctx, gqlID)
	if err != nil {
		headers = []mgraphql.GraphQLHeader{}
	}

	newGQLID := idwrap.NewNow()

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	txGraphqlService := s.graphqlService.TX(tx)
	txHeaderService := s.headerService.TX(tx)

	newEntry := &mgraphql.GraphQL{
		ID:          newGQLID,
		WorkspaceID: gqlEntry.WorkspaceID,
		FolderID:    gqlEntry.FolderID,
		Name:        fmt.Sprintf("Copy of %s", gqlEntry.Name),
		Url:         gqlEntry.Url,
		Query:       gqlEntry.Query,
		Variables:   gqlEntry.Variables,
		Description: gqlEntry.Description,
	}

	if err := txGraphqlService.Create(ctx, newEntry); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, h := range headers {
		newHeader := &mgraphql.GraphQLHeader{
			ID:           idwrap.NewNow(),
			GraphQLID:    newGQLID,
			Key:          h.Key,
			Value:        h.Value,
			Enabled:      h.Enabled,
			Description:  h.Description,
			DisplayOrder: h.DisplayOrder,
		}
		if err := txHeaderService.Create(ctx, newHeader); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Publish GraphQL insert event
	if s.streamers.GraphQL != nil {
		s.streamers.GraphQL.Publish(GraphQLTopic{WorkspaceID: gqlEntry.WorkspaceID}, GraphQLEvent{
			Type:    eventTypeInsert,
			GraphQL: ToAPIGraphQL(*newEntry),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *GraphQLServiceRPC) GraphQLIntrospect(ctx context.Context, req *connect.Request[graphqlv1.GraphQLIntrospectRequest]) (*connect.Response[graphqlv1.GraphQLIntrospectResponse], error) {
	if len(req.Msg.GraphqlId) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("graphql_id is required"))
	}

	gqlID, err := idwrap.NewFromBytes(req.Msg.GraphqlId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	gqlEntry, err := s.graphqlService.Get(ctx, gqlID)
	if err != nil {
		if errors.Is(err, sgraphql.ErrNoGraphQLFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := s.checkWorkspaceReadAccess(ctx, gqlEntry.WorkspaceID); err != nil {
		return nil, err
	}

	varMap, err := s.buildWorkspaceVarMap(ctx, gqlEntry.WorkspaceID)
	if err != nil {
		varMap = make(map[string]any)
	}

	headers, err := s.headerService.GetByGraphQLID(ctx, gqlID)
	if err != nil {
		headers = []mgraphql.GraphQLHeader{}
	}

	// Build introspection request
	body, _ := json.Marshal(map[string]any{
		"query": introspectionQuery,
	})

	url := interpolateString(gqlEntry.Url, varMap)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("failed to create request: %w", err))
	}
	httpReq.Header.Set("Content-Type", "application/json")

	for _, h := range headers {
		if h.Enabled && h.Key != "" {
			httpReq.Header.Set(interpolateString(h.Key, varMap), interpolateString(h.Value, varMap))
		}
	}

	client := httpclient.New()
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("introspection request failed: %w", err))
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read response: %w", err))
	}

	return connect.NewResponse(&graphqlv1.GraphQLIntrospectResponse{
		IntrospectionJson: string(respBody),
		Sdl:               "", // SDL conversion would need a graphql library - return empty for now
	}), nil
}

// Helper functions

func (s *GraphQLServiceRPC) buildWorkspaceVarMap(ctx context.Context, workspaceID idwrap.IDWrap) (map[string]any, error) {
	workspace, err := s.ws.Get(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace: %w", err)
	}

	var globalVars []menv.Variable
	if workspace.GlobalEnv != (idwrap.IDWrap{}) {
		globalVars, err = s.vs.GetVariableByEnvID(ctx, workspace.GlobalEnv)
		if err != nil && !errors.Is(err, senv.ErrNoVarFound) {
			return nil, fmt.Errorf("failed to get global environment variables: %w", err)
		}
	}

	varMap := make(map[string]any)
	for _, envVar := range globalVars {
		if envVar.IsEnabled() {
			varMap[envVar.VarKey] = envVar.Value
		}
	}

	return varMap, nil
}

func prepareGraphQLRequest(gql *mgraphql.GraphQL, headers []mgraphql.GraphQLHeader, varMap map[string]any) (*http.Request, error) {
	url := interpolateString(gql.Url, varMap)
	query := interpolateString(gql.Query, varMap)
	variables := interpolateString(gql.Variables, varMap)

	var varsMap map[string]any
	if variables != "" {
		if err := json.Unmarshal([]byte(variables), &varsMap); err != nil {
			varsMap = nil
		}
	}

	bodyMap := map[string]any{"query": query}
	if varsMap != nil {
		bodyMap["variables"] = varsMap
	}

	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for _, h := range headers {
		if h.Enabled && h.Key != "" {
			req.Header.Set(interpolateString(h.Key, varMap), interpolateString(h.Value, varMap))
		}
	}

	return req, nil
}

func interpolateString(s string, varMap map[string]any) string {
	result := s
	for key, val := range varMap {
		placeholder := "{{" + key + "}}"
		valStr := fmt.Sprintf("%v", val)
		result = strings.ReplaceAll(result, placeholder, valStr)
		// Also support {{ key }} (with spaces)
		placeholder = "{{ " + key + " }}"
		result = strings.ReplaceAll(result, placeholder, valStr)
	}
	return result
}
