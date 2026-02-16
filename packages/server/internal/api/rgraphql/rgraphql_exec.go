//nolint:revive // exported
package rgraphql

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	devtoolsdb "github.com/the-dev-tools/dev-tools/packages/db"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/mutation"
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

	// Get user ID for version creation
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Build variable map from workspace env
	varMap, err := s.buildWorkspaceVarMap(ctx, gqlEntry.WorkspaceID)
	if err != nil {
		varMap = make(map[string]any)
	}

	// Resolve GraphQL request (handles both delta and non-delta)
	var resolvedGraphQL mgraphql.GraphQL
	var headers []mgraphql.GraphQLHeader
	var asserts []mgraphql.GraphQLAssert

	if gqlEntry.IsDelta && gqlEntry.ParentGraphQLID != nil {
		// Delta request: use resolver to merge base + delta
		resolved, err := s.resolver.Resolve(ctx, *gqlEntry.ParentGraphQLID, &gqlEntry.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to resolve delta request: %w", err))
		}
		resolvedGraphQL = resolved.Resolved
		headers = resolved.ResolvedHeaders
		asserts = resolved.ResolvedAsserts

		// Use workspace ID from original entry
		resolvedGraphQL.WorkspaceID = gqlEntry.WorkspaceID
	} else {
		// Non-delta request: load components directly
		resolvedGraphQL = *gqlEntry

		hdrs, err := s.headerService.GetByGraphQLID(ctx, gqlID)
		if err != nil {
			hdrs = []mgraphql.GraphQLHeader{}
		}
		headers = hdrs

		assrts, err := s.graphqlAssertService.GetByGraphQLID(ctx, gqlID)
		if err != nil {
			assrts = []mgraphql.GraphQLAssert{}
		}
		asserts = assrts
	}

	// Build and execute GraphQL request
	httpReq, err := prepareGraphQLRequest(&resolvedGraphQL, headers, varMap)
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

	mut := mutation.New(s.DB, mutation.WithPublisher(s.mutationPublisher()))
	if err := mut.Begin(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to begin transaction: %w", err))
	}
	defer mut.Rollback()

	tx := mut.TX()
	txResponseService := s.responseService.TX(tx)

	if err := txResponseService.Create(ctx, gqlResponse); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Store response headers
	var respHeaderEvents []GraphQLResponseHeaderEvent
	responseHeaders := make(map[string]string)
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
			// Store first value for each header key for assertion context
			if _, exists := responseHeaders[key]; !exists {
				responseHeaders[key] = val
			}
		}
	}

	// Update last_run_at
	now := time.Now().Unix()
	gqlEntry.LastRunAt = &now
	txGraphqlService := s.graphqlService.TX(tx)
	if err := txGraphqlService.Update(ctx, gqlEntry); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Create version with snapshot
	versionName := fmt.Sprintf("v%d", time.Now().UnixNano())
	versionDesc := "Auto-saved version (Run)"
	txGraphqlWriter := sgraphql.NewWriterFromQueries(gen.New(tx))

	version, err := txGraphqlWriter.CreateGraphQLVersion(ctx, gqlID, userID, versionName, versionDesc)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create version: %w", err))
	}

	// Create snapshot GraphQL entry (using version ID as GraphQL ID)
	snapshotGraphQL := &mgraphql.GraphQL{
		ID:          version.ID,
		WorkspaceID: gqlEntry.WorkspaceID,
		FolderID:    gqlEntry.FolderID,
		Name:        gqlEntry.Name,
		Url:         gqlEntry.Url,
		Query:       gqlEntry.Query,
		Variables:   gqlEntry.Variables,
		Description: gqlEntry.Description,
		IsSnapshot:  true,
		IsDelta:     false,
	}
	if err := txGraphqlWriter.Create(ctx, snapshotGraphQL); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create snapshot GraphQL: %w", err))
	}

	// Track snapshot GraphQL insertion event
	mut.Track(mutation.Event{
		Entity:      mutation.EntityGraphQL,
		Op:          mutation.OpInsert,
		ID:          version.ID,
		ParentID:    gqlEntry.WorkspaceID,
		WorkspaceID: gqlEntry.WorkspaceID,
		Payload:     *snapshotGraphQL,
	})

	// Clone headers into snapshot
	txHeaderService := s.headerService.TX(tx)
	for _, header := range headers {
		snapshotHeader := &mgraphql.GraphQLHeader{
			ID:           idwrap.NewNow(),
			GraphQLID:    version.ID,
			Key:          header.Key,
			Value:        header.Value,
			Enabled:      header.Enabled,
			Description:  header.Description,
			DisplayOrder: header.DisplayOrder,
		}
		if err := txHeaderService.Create(ctx, snapshotHeader); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to clone header: %w", err))
		}

		// Track snapshot header insertion event
		mut.Track(mutation.Event{
			Entity:      mutation.EntityGraphQLHeader,
			Op:          mutation.OpInsert,
			ID:          snapshotHeader.ID,
			ParentID:    version.ID,
			WorkspaceID: gqlEntry.WorkspaceID,
			Payload:     *snapshotHeader,
		})
	}

	// Clone request assertions into snapshot (matches HTTP pattern)
	txAssertService := s.graphqlAssertService.TX(tx)
	for _, assert := range asserts {
		snapshotAssert := &mgraphql.GraphQLAssert{
			ID:           idwrap.NewNow(),
			GraphQLID:    version.ID,
			Value:        assert.Value,
			Enabled:      assert.Enabled,
			Description:  assert.Description,
			DisplayOrder: assert.DisplayOrder,
		}
		if err := txAssertService.Create(ctx, snapshotAssert); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to clone assertion: %w", err))
		}

		// Track snapshot assertion insertion event
		mut.Track(mutation.Event{
			Entity:      mutation.EntityGraphQLAssert,
			Op:          mutation.OpInsert,
			ID:          snapshotAssert.ID,
			ParentID:    version.ID,
			WorkspaceID: gqlEntry.WorkspaceID,
			Payload:     *snapshotAssert,
		})
	}

	// Clone response into snapshot
	snapshotResponse := mgraphql.GraphQLResponse{
		ID:        idwrap.NewNow(),
		GraphQLID: version.ID,
		Status:    gqlResponse.Status,
		Body:      gqlResponse.Body,
		Time:      gqlResponse.Time,
		Duration:  gqlResponse.Duration,
		Size:      gqlResponse.Size,
		CreatedAt: gqlResponse.CreatedAt,
	}
	txResponseSvc := s.responseService.TX(tx)
	if err := txResponseSvc.Create(ctx, snapshotResponse); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create snapshot response: %w", err))
	}

	// Track snapshot response insertion event
	mut.Track(mutation.Event{
		Entity:      mutation.EntityGraphQLResponse,
		Op:          mutation.OpInsert,
		ID:          snapshotResponse.ID,
		ParentID:    version.ID,
		WorkspaceID: gqlEntry.WorkspaceID,
		Payload:     snapshotResponse,
	})

	// Clone response headers into snapshot
	for key, values := range resp.Header {
		for _, val := range values {
			snapshotRespHeader := mgraphql.GraphQLResponseHeader{
				ID:          idwrap.NewNow(),
				ResponseID:  snapshotResponse.ID,
				HeaderKey:   key,
				HeaderValue: val,
				CreatedAt:   nowUnix,
			}
			if err := txResponseSvc.CreateHeader(ctx, snapshotRespHeader); err != nil {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create snapshot response header: %w", err))
			}

			// Track snapshot response header insertion event
			mut.Track(mutation.Event{
				Entity:      mutation.EntityGraphQLResponseHeader,
				Op:          mutation.OpInsert,
				ID:          snapshotRespHeader.ID,
				ParentID:    snapshotResponse.ID,
				WorkspaceID: gqlEntry.WorkspaceID,
				Payload:     snapshotRespHeader,
			})
		}
	}

	// Evaluate assertions BEFORE commit (matches HTTP pattern)
	// This ensures response assertions exist in DB before we clone them into snapshot
	var responseAssertions []mgraphql.GraphQLResponseAssert
	if len(asserts) > 0 {
		// Prepare response data for assertion evaluation
		respData := GraphQLResponseData{
			StatusCode: int(resp.StatusCode),
			Body:       body,
			Headers:    responseHeaders,
		}

		// Evaluate and store assertions within the same transaction
		responseAssertions, err = s.evaluateAndStoreAssertions(ctx, tx, gqlID, responseID, gqlEntry.WorkspaceID, respData, asserts)
		if err != nil {
			slog.WarnContext(ctx, "Failed to evaluate assertions",
				"error", err,
				"graphql_id", gqlID.String(),
				"response_id", responseID.String())
			// Don't fail the request, assertions are supplementary
			responseAssertions = []mgraphql.GraphQLResponseAssert{}
		}
	}

	// Clone response assertions into snapshot (matches HTTP pattern)
	for _, responseAssert := range responseAssertions {
		snapshotResponseAssert := mgraphql.GraphQLResponseAssert{
			ID:         idwrap.NewNow(),
			ResponseID: snapshotResponse.ID,
			Value:      responseAssert.Value,
			Success:    responseAssert.Success,
			CreatedAt:  nowUnix,
		}
		if err := txResponseSvc.CreateAssert(ctx, snapshotResponseAssert); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to clone response assertion: %w", err))
		}

		// Track snapshot response assertion insertion event
		mut.Track(mutation.Event{
			Entity:      mutation.EntityGraphQLResponseAssert,
			Op:          mutation.OpInsert,
			ID:          snapshotResponseAssert.ID,
			ParentID:    snapshotResponse.ID,
			WorkspaceID: gqlEntry.WorkspaceID,
			Payload:     snapshotResponseAssert,
		})
	}

	// Collect events before commit for manual publishing of snapshot entities
	snapshotEvents := mut.Events()

	if err := mut.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to commit transaction: %w", err))
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

	// Publish version insert event
	if s.streamers.GraphQLVersion != nil {
		s.streamers.GraphQLVersion.Publish(GraphQLVersionTopic{WorkspaceID: gqlEntry.WorkspaceID}, GraphQLVersionEvent{
			Type:           eventTypeInsert,
			GraphQLVersion: ToAPIGraphQLVersion(*version),
		})
	}

	// Publish response assertion events (now that they're committed)
	if len(responseAssertions) > 0 && s.streamers.GraphQLResponseAssert != nil {
		topic := GraphQLResponseAssertTopic{WorkspaceID: gqlEntry.WorkspaceID}
		for _, assert := range responseAssertions {
			s.streamers.GraphQLResponseAssert.Publish(topic, GraphQLResponseAssertEvent{
				Type:                  eventTypeInsert,
				GraphQLResponseAssert: ToAPIGraphQLResponseAssert(assert),
			})
		}
	}

	// Publish snapshot sync events for snapshot response/headers/assertions
	// so the frontend receives real-time updates for the newly created snapshot data
	s.publishSnapshotSyncEvents(snapshotEvents, gqlEntry.WorkspaceID)

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

// publishSnapshotSyncEvents publishes sync events for snapshot entities
// so the frontend receives real-time updates for the newly created snapshot data.
// This function follows the same pattern as HTTP's publishSnapshotSyncEvents.
func (s *GraphQLServiceRPC) publishSnapshotSyncEvents(events []mutation.Event, workspaceID idwrap.IDWrap) {
	for _, evt := range events {
		//nolint:exhaustive
		switch evt.Entity {
		case mutation.EntityGraphQLResponse:
			if s.streamers.GraphQLResponse != nil {
				if resp, ok := evt.Payload.(mgraphql.GraphQLResponse); ok {
					s.streamers.GraphQLResponse.Publish(
						GraphQLResponseTopic{WorkspaceID: workspaceID},
						GraphQLResponseEvent{
							Type:            eventTypeInsert,
							GraphQLResponse: ToAPIGraphQLResponse(resp),
						},
					)
				}
			}
		case mutation.EntityGraphQLResponseHeader:
			if s.streamers.GraphQLResponseHeader != nil {
				if rh, ok := evt.Payload.(mgraphql.GraphQLResponseHeader); ok {
					s.streamers.GraphQLResponseHeader.Publish(
						GraphQLResponseHeaderTopic{WorkspaceID: workspaceID},
						GraphQLResponseHeaderEvent{
							Type:                  eventTypeInsert,
							GraphQLResponseHeader: ToAPIGraphQLResponseHeader(rh),
						},
					)
				}
			}
		case mutation.EntityGraphQLResponseAssert:
			if s.streamers.GraphQLResponseAssert != nil {
				if ra, ok := evt.Payload.(mgraphql.GraphQLResponseAssert); ok {
					s.streamers.GraphQLResponseAssert.Publish(
						GraphQLResponseAssertTopic{WorkspaceID: workspaceID},
						GraphQLResponseAssertEvent{
							Type:                  eventTypeInsert,
							GraphQLResponseAssert: ToAPIGraphQLResponseAssert(ra),
						},
					)
				}
			}
		}
	}
}
