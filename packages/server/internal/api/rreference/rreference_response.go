package rreference

import (
	"context"
	"encoding/json"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

func (c *ReferenceServiceRPC) getLatestResponse(ctx context.Context, httpID idwrap.IDWrap) (map[string]any, error) {
	responses, err := c.httpResponseReader.GetByHttpID(ctx, httpID)
	if err != nil {
		return nil, err
	}
	if len(responses) == 0 {
		return nil, nil
	}

	latest := responses[0]
	for _, r := range responses {
		if r.CreatedAt > latest.CreatedAt {
			latest = r
		}
	}

	var body any = string(latest.Body)
	if len(latest.Body) > 0 {
		var jsonBody any
		if err := json.Unmarshal(latest.Body, &jsonBody); err == nil {
			body = jsonBody
		}
	}

	return map[string]any{
		"status":   latest.Status,
		"body":     body,
		"headers":  map[string]string{},
		"duration": latest.Duration,
		"size":     latest.Size,
	}, nil
}

func (c *ReferenceServiceRPC) getLatestGraphQLResponse(ctx context.Context, graphqlID idwrap.IDWrap) (map[string]any, error) {
	responses, err := c.graphqlResponseReader.GetByGraphQLID(ctx, graphqlID)
	if err != nil {
		return nil, err
	}
	if len(responses) == 0 {
		return nil, nil
	}

	latest := responses[0]
	for _, r := range responses {
		if r.Time > latest.Time {
			latest = r
		}
	}

	var body any = string(latest.Body)
	var bodyMap map[string]any
	if len(latest.Body) > 0 {
		var jsonBody any
		if err := json.Unmarshal(latest.Body, &jsonBody); err == nil {
			body = jsonBody
			if m, ok := jsonBody.(map[string]any); ok {
				bodyMap = m
			}
		}
	}

	var data, errors any
	if bodyMap != nil {
		data = bodyMap["data"]
		errors = bodyMap["errors"]
	}

	return map[string]any{
		"status":   latest.Status,
		"body":     body,
		"data":     data,
		"errors":   errors,
		"headers":  map[string]string{},
		"duration": latest.Duration,
		"size":     latest.Size,
	}, nil
}

// addGraphQLConvenienceVars adds top-level convenience variables for GraphQL context.
func addGraphQLConvenienceVars(resp map[string]any, varMap map[string]any) {
	if data, ok := resp["data"]; ok && data != nil {
		varMap["data"] = data
	}
	if errs, ok := resp["errors"]; ok && errs != nil {
		varMap["errors"] = errs
	}
	status := 0
	if s, ok := resp["status"].(int32); ok {
		status = int(s)
	}
	varMap["status"] = status
	varMap["success"] = status >= 200 && status < 300
	varMap["has_data"] = resp["data"] != nil
	varMap["has_errors"] = resp["errors"] != nil
}
