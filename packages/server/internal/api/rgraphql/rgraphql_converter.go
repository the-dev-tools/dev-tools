//nolint:revive // exported
package rgraphql

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
	graphqlv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/graph_q_l/v1"
)

// Model -> Proto

func ToAPIGraphQL(g mgraphql.GraphQL) *graphqlv1.GraphQL {
	result := &graphqlv1.GraphQL{
		GraphqlId: g.ID.Bytes(),
		Name:      g.Name,
		Url:       g.Url,
		Query:     g.Query,
		Variables: g.Variables,
	}
	if g.LastRunAt != nil {
		result.LastRunAt = timestamppb.New(time.Unix(*g.LastRunAt, 0))
	}
	return result
}

func ToAPIGraphQLHeader(h mgraphql.GraphQLHeader) *graphqlv1.GraphQLHeader {
	return &graphqlv1.GraphQLHeader{
		GraphqlHeaderId: h.ID.Bytes(),
		GraphqlId:       h.GraphQLID.Bytes(),
		Key:             h.Key,
		Value:           h.Value,
		Enabled:         h.Enabled,
		Description:     h.Description,
		Order:           h.DisplayOrder,
	}
}

func ToAPIGraphQLAssert(a mgraphql.GraphQLAssert) *graphqlv1.GraphQLAssert {
	return &graphqlv1.GraphQLAssert{
		GraphqlAssertId: a.ID.Bytes(),
		GraphqlId:       a.GraphQLID.Bytes(),
		Value:           a.Value,
		Enabled:         a.Enabled,
		Order:           a.DisplayOrder,
	}
}

func ToAPIGraphQLResponse(r mgraphql.GraphQLResponse) *graphqlv1.GraphQLResponse {
	return &graphqlv1.GraphQLResponse{
		GraphqlResponseId: r.ID.Bytes(),
		GraphqlId:         r.GraphQLID.Bytes(),
		Status:            r.Status,
		Body:              string(r.Body),
		Time:              timestamppb.New(time.Unix(r.Time, 0)),
		Duration:          r.Duration,
		Size:              r.Size,
	}
}

func ToAPIGraphQLResponseHeader(h mgraphql.GraphQLResponseHeader) *graphqlv1.GraphQLResponseHeader {
	return &graphqlv1.GraphQLResponseHeader{
		GraphqlResponseHeaderId: h.ID.Bytes(),
		GraphqlResponseId:       h.ResponseID.Bytes(),
		Key:                     h.HeaderKey,
		Value:                   h.HeaderValue,
	}
}

func ToAPIGraphQLResponseAssert(a mgraphql.GraphQLResponseAssert) *graphqlv1.GraphQLResponseAssert {
	return &graphqlv1.GraphQLResponseAssert{
		GraphqlResponseAssertId: a.ID.Bytes(),
		GraphqlResponseId:       a.ResponseID.Bytes(),
		Value:                   a.Value,
		Success:                 a.Success,
	}
}

// Sync response builders

func graphqlSyncResponseFrom(event GraphQLEvent) *graphqlv1.GraphQLSyncResponse {
	var value *graphqlv1.GraphQLSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		name := event.GraphQL.GetName()
		url := event.GraphQL.GetUrl()
		query := event.GraphQL.GetQuery()
		variables := event.GraphQL.GetVariables()
		lastRunAt := event.GraphQL.GetLastRunAt()
		value = &graphqlv1.GraphQLSync_ValueUnion{
			Kind: graphqlv1.GraphQLSync_ValueUnion_KIND_INSERT,
			Insert: &graphqlv1.GraphQLSyncInsert{
				GraphqlId: event.GraphQL.GetGraphqlId(),
				Name:      name,
				Url:       url,
				Query:     query,
				Variables: variables,
				LastRunAt: lastRunAt,
			},
		}
	case eventTypeUpdate:
		name := event.GraphQL.GetName()
		url := event.GraphQL.GetUrl()
		query := event.GraphQL.GetQuery()
		variables := event.GraphQL.GetVariables()
		lastRunAt := event.GraphQL.GetLastRunAt()

		var lastRunAtUnion *graphqlv1.GraphQLSyncUpdate_LastRunAtUnion
		if lastRunAt != nil {
			lastRunAtUnion = &graphqlv1.GraphQLSyncUpdate_LastRunAtUnion{
				Kind:  graphqlv1.GraphQLSyncUpdate_LastRunAtUnion_KIND_VALUE,
				Value: lastRunAt,
			}
		}

		value = &graphqlv1.GraphQLSync_ValueUnion{
			Kind: graphqlv1.GraphQLSync_ValueUnion_KIND_UPDATE,
			Update: &graphqlv1.GraphQLSyncUpdate{
				GraphqlId: event.GraphQL.GetGraphqlId(),
				Name:      &name,
				Url:       &url,
				Query:     &query,
				Variables: &variables,
				LastRunAt: lastRunAtUnion,
			},
		}
	case eventTypeDelete:
		value = &graphqlv1.GraphQLSync_ValueUnion{
			Kind:   graphqlv1.GraphQLSync_ValueUnion_KIND_DELETE,
			Delete: &graphqlv1.GraphQLSyncDelete{GraphqlId: event.GraphQL.GetGraphqlId()},
		}
	}

	return &graphqlv1.GraphQLSyncResponse{
		Items: []*graphqlv1.GraphQLSync{{Value: value}},
	}
}

func graphqlHeaderSyncResponseFrom(event GraphQLHeaderEvent) *graphqlv1.GraphQLHeaderSyncResponse {
	var value *graphqlv1.GraphQLHeaderSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		key := event.GraphQLHeader.GetKey()
		val := event.GraphQLHeader.GetValue()
		enabled := event.GraphQLHeader.GetEnabled()
		description := event.GraphQLHeader.GetDescription()
		order := event.GraphQLHeader.GetOrder()
		value = &graphqlv1.GraphQLHeaderSync_ValueUnion{
			Kind: graphqlv1.GraphQLHeaderSync_ValueUnion_KIND_INSERT,
			Insert: &graphqlv1.GraphQLHeaderSyncInsert{
				GraphqlHeaderId: event.GraphQLHeader.GetGraphqlHeaderId(),
				GraphqlId:       event.GraphQLHeader.GetGraphqlId(),
				Key:             key,
				Value:           val,
				Enabled:         enabled,
				Description:     description,
				Order:           order,
			},
		}
	case eventTypeUpdate:
		key := event.GraphQLHeader.GetKey()
		val := event.GraphQLHeader.GetValue()
		enabled := event.GraphQLHeader.GetEnabled()
		description := event.GraphQLHeader.GetDescription()
		order := event.GraphQLHeader.GetOrder()
		value = &graphqlv1.GraphQLHeaderSync_ValueUnion{
			Kind: graphqlv1.GraphQLHeaderSync_ValueUnion_KIND_UPDATE,
			Update: &graphqlv1.GraphQLHeaderSyncUpdate{
				GraphqlHeaderId: event.GraphQLHeader.GetGraphqlHeaderId(),
				Key:             &key,
				Value:           &val,
				Enabled:         &enabled,
				Description:     &description,
				Order:           &order,
			},
		}
	case eventTypeDelete:
		value = &graphqlv1.GraphQLHeaderSync_ValueUnion{
			Kind:   graphqlv1.GraphQLHeaderSync_ValueUnion_KIND_DELETE,
			Delete: &graphqlv1.GraphQLHeaderSyncDelete{GraphqlHeaderId: event.GraphQLHeader.GetGraphqlHeaderId()},
		}
	}

	return &graphqlv1.GraphQLHeaderSyncResponse{
		Items: []*graphqlv1.GraphQLHeaderSync{{Value: value}},
	}
}

func graphqlResponseSyncResponseFrom(event GraphQLResponseEvent) *graphqlv1.GraphQLResponseSyncResponse {
	var value *graphqlv1.GraphQLResponseSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		status := event.GraphQLResponse.GetStatus()
		body := event.GraphQLResponse.GetBody()
		t := event.GraphQLResponse.GetTime()
		duration := event.GraphQLResponse.GetDuration()
		size := event.GraphQLResponse.GetSize()
		value = &graphqlv1.GraphQLResponseSync_ValueUnion{
			Kind: graphqlv1.GraphQLResponseSync_ValueUnion_KIND_INSERT,
			Insert: &graphqlv1.GraphQLResponseSyncInsert{
				GraphqlResponseId: event.GraphQLResponse.GetGraphqlResponseId(),
				GraphqlId:         event.GraphQLResponse.GetGraphqlId(),
				Status:            status,
				Body:              body,
				Time:              t,
				Duration:          duration,
				Size:              size,
			},
		}
	case eventTypeUpdate:
		status := event.GraphQLResponse.GetStatus()
		body := event.GraphQLResponse.GetBody()
		t := event.GraphQLResponse.GetTime()
		duration := event.GraphQLResponse.GetDuration()
		size := event.GraphQLResponse.GetSize()
		value = &graphqlv1.GraphQLResponseSync_ValueUnion{
			Kind: graphqlv1.GraphQLResponseSync_ValueUnion_KIND_UPDATE,
			Update: &graphqlv1.GraphQLResponseSyncUpdate{
				GraphqlResponseId: event.GraphQLResponse.GetGraphqlResponseId(),
				Status:            &status,
				Body:              &body,
				Time:              t,
				Duration:          &duration,
				Size:              &size,
			},
		}
	case eventTypeDelete:
		value = &graphqlv1.GraphQLResponseSync_ValueUnion{
			Kind:   graphqlv1.GraphQLResponseSync_ValueUnion_KIND_DELETE,
			Delete: &graphqlv1.GraphQLResponseSyncDelete{GraphqlResponseId: event.GraphQLResponse.GetGraphqlResponseId()},
		}
	}

	return &graphqlv1.GraphQLResponseSyncResponse{
		Items: []*graphqlv1.GraphQLResponseSync{{Value: value}},
	}
}

func graphqlResponseHeaderSyncResponseFrom(event GraphQLResponseHeaderEvent) *graphqlv1.GraphQLResponseHeaderSyncResponse {
	var value *graphqlv1.GraphQLResponseHeaderSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		key := event.GraphQLResponseHeader.GetKey()
		val := event.GraphQLResponseHeader.GetValue()
		value = &graphqlv1.GraphQLResponseHeaderSync_ValueUnion{
			Kind: graphqlv1.GraphQLResponseHeaderSync_ValueUnion_KIND_INSERT,
			Insert: &graphqlv1.GraphQLResponseHeaderSyncInsert{
				GraphqlResponseHeaderId: event.GraphQLResponseHeader.GetGraphqlResponseHeaderId(),
				GraphqlResponseId:       event.GraphQLResponseHeader.GetGraphqlResponseId(),
				Key:                     key,
				Value:                   val,
			},
		}
	case eventTypeUpdate:
		key := event.GraphQLResponseHeader.GetKey()
		val := event.GraphQLResponseHeader.GetValue()
		value = &graphqlv1.GraphQLResponseHeaderSync_ValueUnion{
			Kind: graphqlv1.GraphQLResponseHeaderSync_ValueUnion_KIND_UPDATE,
			Update: &graphqlv1.GraphQLResponseHeaderSyncUpdate{
				GraphqlResponseHeaderId: event.GraphQLResponseHeader.GetGraphqlResponseHeaderId(),
				Key:                     &key,
				Value:                   &val,
			},
		}
	case eventTypeDelete:
		value = &graphqlv1.GraphQLResponseHeaderSync_ValueUnion{
			Kind:   graphqlv1.GraphQLResponseHeaderSync_ValueUnion_KIND_DELETE,
			Delete: &graphqlv1.GraphQLResponseHeaderSyncDelete{GraphqlResponseHeaderId: event.GraphQLResponseHeader.GetGraphqlResponseHeaderId()},
		}
	}

	return &graphqlv1.GraphQLResponseHeaderSyncResponse{
		Items: []*graphqlv1.GraphQLResponseHeaderSync{{Value: value}},
	}
}

// graphqlDeltaSyncResponseFrom converts GraphQLEvent to GraphQLDeltaSync response
// TODO: Implement delta sync converter once delta event publishing is implemented
func graphqlDeltaSyncResponseFrom(event GraphQLEvent) *graphqlv1.GraphQLDeltaSyncResponse {
	// For now, return nil as delta sync is not fully implemented
	// Delta CRUD operations work, but real-time sync needs separate event streams
	return nil
}

// graphqlAssertSyncResponseFrom converts GraphQLAssertEvent to GraphQLAssertSync response
func graphqlAssertSyncResponseFrom(event GraphQLAssertEvent) *graphqlv1.GraphQLAssertSyncResponse {
	var value *graphqlv1.GraphQLAssertSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		value = &graphqlv1.GraphQLAssertSync_ValueUnion{
			Kind: graphqlv1.GraphQLAssertSync_ValueUnion_KIND_INSERT,
			Insert: &graphqlv1.GraphQLAssertSyncInsert{
				GraphqlAssertId: event.GraphQLAssert.GetGraphqlAssertId(),
				GraphqlId:       event.GraphQLAssert.GetGraphqlId(),
				Value:           event.GraphQLAssert.GetValue(),
				Enabled:         event.GraphQLAssert.GetEnabled(),
				Order:           event.GraphQLAssert.GetOrder(),
			},
		}
	case eventTypeUpdate:
		value_ := event.GraphQLAssert.GetValue()
		enabled := event.GraphQLAssert.GetEnabled()
		order := event.GraphQLAssert.GetOrder()
		value = &graphqlv1.GraphQLAssertSync_ValueUnion{
			Kind: graphqlv1.GraphQLAssertSync_ValueUnion_KIND_UPDATE,
			Update: &graphqlv1.GraphQLAssertSyncUpdate{
				GraphqlAssertId: event.GraphQLAssert.GetGraphqlAssertId(),
				Value:           &value_,
				Enabled:         &enabled,
				Order:           &order,
			},
		}
	case eventTypeDelete:
		value = &graphqlv1.GraphQLAssertSync_ValueUnion{
			Kind: graphqlv1.GraphQLAssertSync_ValueUnion_KIND_DELETE,
			Delete: &graphqlv1.GraphQLAssertSyncDelete{
				GraphqlAssertId: event.GraphQLAssert.GetGraphqlAssertId(),
			},
		}
	}

	return &graphqlv1.GraphQLAssertSyncResponse{
		Items: []*graphqlv1.GraphQLAssertSync{
			{
				Value: value,
			},
		},
	}
}

// graphqlResponseAssertSyncResponseFrom converts GraphQLResponseAssertEvent to GraphQLResponseAssertSync response
func graphqlResponseAssertSyncResponseFrom(event GraphQLResponseAssertEvent) *graphqlv1.GraphQLResponseAssertSyncResponse {
	var value *graphqlv1.GraphQLResponseAssertSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		value_ := event.GraphQLResponseAssert.GetValue()
		success := event.GraphQLResponseAssert.GetSuccess()
		value = &graphqlv1.GraphQLResponseAssertSync_ValueUnion{
			Kind: graphqlv1.GraphQLResponseAssertSync_ValueUnion_KIND_INSERT,
			Insert: &graphqlv1.GraphQLResponseAssertSyncInsert{
				GraphqlResponseAssertId: event.GraphQLResponseAssert.GetGraphqlResponseAssertId(),
				GraphqlResponseId:       event.GraphQLResponseAssert.GetGraphqlResponseId(),
				Value:                   value_,
				Success:                 success,
			},
		}
	case eventTypeUpdate:
		value_ := event.GraphQLResponseAssert.GetValue()
		success := event.GraphQLResponseAssert.GetSuccess()
		value = &graphqlv1.GraphQLResponseAssertSync_ValueUnion{
			Kind: graphqlv1.GraphQLResponseAssertSync_ValueUnion_KIND_UPDATE,
			Update: &graphqlv1.GraphQLResponseAssertSyncUpdate{
				GraphqlResponseAssertId: event.GraphQLResponseAssert.GetGraphqlResponseAssertId(),
				Value:                   &value_,
				Success:                 &success,
			},
		}
	case eventTypeDelete:
		value = &graphqlv1.GraphQLResponseAssertSync_ValueUnion{
			Kind: graphqlv1.GraphQLResponseAssertSync_ValueUnion_KIND_DELETE,
			Delete: &graphqlv1.GraphQLResponseAssertSyncDelete{
				GraphqlResponseAssertId: event.GraphQLResponseAssert.GetGraphqlResponseAssertId(),
			},
		}
	}

	return &graphqlv1.GraphQLResponseAssertSyncResponse{
		Items: []*graphqlv1.GraphQLResponseAssertSync{
			{
				Value: value,
			},
		},
	}
}
