//nolint:revive // exported
package rgraphql

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
	graphqlv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/graph_q_l/v1"
	globalv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/global/v1"
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

func graphqlHeaderDeltaSyncResponseFrom(event GraphQLHeaderEvent, header mgraphql.GraphQLHeader) *graphqlv1.GraphQLHeaderDeltaSyncResponse {
	var value *graphqlv1.GraphQLHeaderDeltaSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		delta := &graphqlv1.GraphQLHeaderDeltaSyncInsert{
			DeltaGraphqlHeaderId: header.ID.Bytes(),
			GraphqlId:            header.GraphQLID.Bytes(),
		}
		if header.ParentGraphQLHeaderID != nil {
			delta.GraphqlHeaderId = header.ParentGraphQLHeaderID.Bytes()
		}
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
		value = &graphqlv1.GraphQLHeaderDeltaSync_ValueUnion{
			Kind:   graphqlv1.GraphQLHeaderDeltaSync_ValueUnion_KIND_INSERT,
			Insert: delta,
		}
	case eventTypeUpdate:
		delta := &graphqlv1.GraphQLHeaderDeltaSyncUpdate{
			DeltaGraphqlHeaderId: header.ID.Bytes(),
			GraphqlId:            header.GraphQLID.Bytes(),
		}
		if header.ParentGraphQLHeaderID != nil {
			delta.GraphqlHeaderId = header.ParentGraphQLHeaderID.Bytes()
		}
		if header.DeltaKey != nil {
			keyStr := *header.DeltaKey
			delta.Key = &graphqlv1.GraphQLHeaderDeltaSyncUpdate_KeyUnion{
				Kind:  graphqlv1.GraphQLHeaderDeltaSyncUpdate_KeyUnion_KIND_VALUE,
				Value: &keyStr,
			}
		} else {
			delta.Key = &graphqlv1.GraphQLHeaderDeltaSyncUpdate_KeyUnion{
				Kind:  graphqlv1.GraphQLHeaderDeltaSyncUpdate_KeyUnion_KIND_UNSET,
				Unset: globalv1.Unset_UNSET.Enum(),
			}
		}
		if header.DeltaValue != nil {
			valueStr := *header.DeltaValue
			delta.Value = &graphqlv1.GraphQLHeaderDeltaSyncUpdate_ValueUnion{
				Kind:  graphqlv1.GraphQLHeaderDeltaSyncUpdate_ValueUnion_KIND_VALUE,
				Value: &valueStr,
			}
		} else {
			delta.Value = &graphqlv1.GraphQLHeaderDeltaSyncUpdate_ValueUnion{
				Kind:  graphqlv1.GraphQLHeaderDeltaSyncUpdate_ValueUnion_KIND_UNSET,
				Unset: globalv1.Unset_UNSET.Enum(),
			}
		}
		if header.DeltaEnabled != nil {
			enabledBool := *header.DeltaEnabled
			delta.Enabled = &graphqlv1.GraphQLHeaderDeltaSyncUpdate_EnabledUnion{
				Kind:  graphqlv1.GraphQLHeaderDeltaSyncUpdate_EnabledUnion_KIND_VALUE,
				Value: &enabledBool,
			}
		} else {
			delta.Enabled = &graphqlv1.GraphQLHeaderDeltaSyncUpdate_EnabledUnion{
				Kind:  graphqlv1.GraphQLHeaderDeltaSyncUpdate_EnabledUnion_KIND_UNSET,
				Unset: globalv1.Unset_UNSET.Enum(),
			}
		}
		if header.DeltaDescription != nil {
			descStr := *header.DeltaDescription
			delta.Description = &graphqlv1.GraphQLHeaderDeltaSyncUpdate_DescriptionUnion{
				Kind:  graphqlv1.GraphQLHeaderDeltaSyncUpdate_DescriptionUnion_KIND_VALUE,
				Value: &descStr,
			}
		} else {
			delta.Description = &graphqlv1.GraphQLHeaderDeltaSyncUpdate_DescriptionUnion{
				Kind:  graphqlv1.GraphQLHeaderDeltaSyncUpdate_DescriptionUnion_KIND_UNSET,
				Unset: globalv1.Unset_UNSET.Enum(),
			}
		}
		if header.DeltaDisplayOrder != nil {
			orderFloat := *header.DeltaDisplayOrder
			delta.Order = &graphqlv1.GraphQLHeaderDeltaSyncUpdate_OrderUnion{
				Kind:  graphqlv1.GraphQLHeaderDeltaSyncUpdate_OrderUnion_KIND_VALUE,
				Value: &orderFloat,
			}
		} else {
			delta.Order = &graphqlv1.GraphQLHeaderDeltaSyncUpdate_OrderUnion{
				Kind:  graphqlv1.GraphQLHeaderDeltaSyncUpdate_OrderUnion_KIND_UNSET,
				Unset: globalv1.Unset_UNSET.Enum(),
			}
		}
		value = &graphqlv1.GraphQLHeaderDeltaSync_ValueUnion{
			Kind:   graphqlv1.GraphQLHeaderDeltaSync_ValueUnion_KIND_UPDATE,
			Update: delta,
		}
	case eventTypeDelete:
		value = &graphqlv1.GraphQLHeaderDeltaSync_ValueUnion{
			Kind: graphqlv1.GraphQLHeaderDeltaSync_ValueUnion_KIND_DELETE,
			Delete: &graphqlv1.GraphQLHeaderDeltaSyncDelete{
				DeltaGraphqlHeaderId: header.ID.Bytes(),
			},
		}
	}

	if value == nil {
		return nil
	}

	return &graphqlv1.GraphQLHeaderDeltaSyncResponse{
		Items: []*graphqlv1.GraphQLHeaderDeltaSync{
			{
				Value: value,
			},
		},
	}
}

func graphqlAssertDeltaSyncResponseFrom(event GraphQLAssertEvent, assert mgraphql.GraphQLAssert) *graphqlv1.GraphQLAssertDeltaSyncResponse {
	var value *graphqlv1.GraphQLAssertDeltaSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		delta := &graphqlv1.GraphQLAssertDeltaSyncInsert{
			DeltaGraphqlAssertId: assert.ID.Bytes(),
			GraphqlId:            assert.GraphQLID.Bytes(),
		}
		if assert.ParentGraphQLAssertID != nil {
			delta.GraphqlAssertId = assert.ParentGraphQLAssertID.Bytes()
		}
		if assert.DeltaValue != nil {
			delta.Value = assert.DeltaValue
		}
		if assert.DeltaEnabled != nil {
			delta.Enabled = assert.DeltaEnabled
		}
		if assert.DeltaDisplayOrder != nil {
			delta.Order = assert.DeltaDisplayOrder
		}
		value = &graphqlv1.GraphQLAssertDeltaSync_ValueUnion{
			Kind:   graphqlv1.GraphQLAssertDeltaSync_ValueUnion_KIND_INSERT,
			Insert: delta,
		}
	case eventTypeUpdate:
		delta := &graphqlv1.GraphQLAssertDeltaSyncUpdate{
			DeltaGraphqlAssertId: assert.ID.Bytes(),
			GraphqlId:            assert.GraphQLID.Bytes(),
		}
		if assert.ParentGraphQLAssertID != nil {
			delta.GraphqlAssertId = assert.ParentGraphQLAssertID.Bytes()
		}
		if assert.DeltaValue != nil {
			valueStr := *assert.DeltaValue
			delta.Value = &graphqlv1.GraphQLAssertDeltaSyncUpdate_ValueUnion{
				Kind:  graphqlv1.GraphQLAssertDeltaSyncUpdate_ValueUnion_KIND_VALUE,
				Value: &valueStr,
			}
		} else {
			delta.Value = &graphqlv1.GraphQLAssertDeltaSyncUpdate_ValueUnion{
				Kind:  graphqlv1.GraphQLAssertDeltaSyncUpdate_ValueUnion_KIND_UNSET,
				Unset: globalv1.Unset_UNSET.Enum(),
			}
		}
		if assert.DeltaEnabled != nil {
			enabledBool := *assert.DeltaEnabled
			delta.Enabled = &graphqlv1.GraphQLAssertDeltaSyncUpdate_EnabledUnion{
				Kind:  graphqlv1.GraphQLAssertDeltaSyncUpdate_EnabledUnion_KIND_VALUE,
				Value: &enabledBool,
			}
		} else {
			delta.Enabled = &graphqlv1.GraphQLAssertDeltaSyncUpdate_EnabledUnion{
				Kind:  graphqlv1.GraphQLAssertDeltaSyncUpdate_EnabledUnion_KIND_UNSET,
				Unset: globalv1.Unset_UNSET.Enum(),
			}
		}
		if assert.DeltaDisplayOrder != nil {
			orderFloat := *assert.DeltaDisplayOrder
			delta.Order = &graphqlv1.GraphQLAssertDeltaSyncUpdate_OrderUnion{
				Kind:  graphqlv1.GraphQLAssertDeltaSyncUpdate_OrderUnion_KIND_VALUE,
				Value: &orderFloat,
			}
		} else {
			delta.Order = &graphqlv1.GraphQLAssertDeltaSyncUpdate_OrderUnion{
				Kind:  graphqlv1.GraphQLAssertDeltaSyncUpdate_OrderUnion_KIND_UNSET,
				Unset: globalv1.Unset_UNSET.Enum(),
			}
		}
		value = &graphqlv1.GraphQLAssertDeltaSync_ValueUnion{
			Kind:   graphqlv1.GraphQLAssertDeltaSync_ValueUnion_KIND_UPDATE,
			Update: delta,
		}
	case eventTypeDelete:
		value = &graphqlv1.GraphQLAssertDeltaSync_ValueUnion{
			Kind: graphqlv1.GraphQLAssertDeltaSync_ValueUnion_KIND_DELETE,
			Delete: &graphqlv1.GraphQLAssertDeltaSyncDelete{
				DeltaGraphqlAssertId: assert.ID.Bytes(),
			},
		}
	}

	if value == nil {
		return nil
	}

	return &graphqlv1.GraphQLAssertDeltaSyncResponse{
		Items: []*graphqlv1.GraphQLAssertDeltaSync{
			{
				Value: value,
			},
		},
	}
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
