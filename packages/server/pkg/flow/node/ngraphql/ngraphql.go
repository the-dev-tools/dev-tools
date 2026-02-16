//nolint:revive // exported
package ngraphql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/expression"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	graphqlresponse "github.com/the-dev-tools/dev-tools/packages/server/pkg/graphql/response"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/httpclient"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
)

type NodeGraphQL struct {
	FlowNodeID idwrap.IDWrap
	Name       string

	GraphQL    mgraphql.GraphQL
	Headers    []mgraphql.GraphQLHeader
	Asserts    []mgraphql.GraphQLAssert
	HttpClient httpclient.HttpClient
	SideRespChan chan NodeGraphQLSideResp
	logger       *slog.Logger
}

type NodeGraphQLSideResp struct {
	ExecutionID idwrap.IDWrap
	GraphQL     mgraphql.GraphQL
	Headers     []mgraphql.GraphQLHeader
	Response    mgraphql.GraphQLResponse
	RespHeaders []mgraphql.GraphQLResponseHeader
	RespAsserts []mgraphql.GraphQLResponseAssert
	Done        chan struct{}
}

const (
	outputResponseName = "response"
	outputRequestName  = "request"
)

type graphqlRequestBody struct {
	Query     string          `json:"query"`
	Variables json.RawMessage `json:"variables,omitempty"`
}

func New(
	id idwrap.IDWrap,
	name string,
	gql mgraphql.GraphQL,
	headers []mgraphql.GraphQLHeader,
	asserts []mgraphql.GraphQLAssert,
	httpClient httpclient.HttpClient,
	sideRespChan chan NodeGraphQLSideResp,
	logger *slog.Logger,
) *NodeGraphQL {
	return &NodeGraphQL{
		FlowNodeID:   id,
		Name:         name,
		GraphQL:      gql,
		Headers:      headers,
		Asserts:      asserts,
		HttpClient:   httpClient,
		SideRespChan: sideRespChan,
		logger:       logger,
	}
}

func (n *NodeGraphQL) GetID() idwrap.IDWrap {
	return n.FlowNodeID
}

func (n *NodeGraphQL) SetID(id idwrap.IDWrap) {
	n.FlowNodeID = id
}

func (n *NodeGraphQL) GetName() string {
	return n.Name
}

func (n *NodeGraphQL) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	nextID := mflow.GetNextNodeID(req.EdgeSourceMap, n.GetID(), mflow.HandleUnspecified)
	result := node.FlowNodeResult{
		NextNodeID: nextID,
		Err:        nil,
	}

	varMapCopy := node.DeepCopyVarMap(req)

	// Build unified environment for interpolation
	env := expression.NewUnifiedEnv(varMapCopy)

	// Track input variable reads if tracker is available
	readVars := make(map[string]any)

	// Helper to interpolate and collect reads (same pattern as HTTP REQUEST nodes)
	interpolate := func(raw string) (string, error) {
		if !expression.HasVars(raw) {
			return raw, nil
		}
		result, err := env.InterpolateWithResult(raw)
		if err != nil {
			return "", err
		}
		// Collect tracked reads
		for k, v := range result.ReadVars {
			readVars[k] = v
		}
		return result.Value, nil
	}

	// Interpolate URL, query, variables, and headers
	var err error
	url, err := interpolate(n.GraphQL.Url)
	if err != nil {
		result.Err = fmt.Errorf("failed to interpolate url: %w", err)
		return result
	}

	query, err := interpolate(n.GraphQL.Query)
	if err != nil {
		result.Err = fmt.Errorf("failed to interpolate query: %w", err)
		return result
	}

	variables, err := interpolate(n.GraphQL.Variables)
	if err != nil {
		result.Err = fmt.Errorf("failed to interpolate variables: %w", err)
		return result
	}

	// Build request body
	var varsJSON json.RawMessage
	if variables != "" {
		// Try to parse as JSON; if invalid, use as string
		if json.Valid([]byte(variables)) {
			varsJSON = json.RawMessage(variables)
		} else {
			// Wrap as JSON string
			b, _ := json.Marshal(variables)
			varsJSON = b
		}
	}

	body := graphqlRequestBody{
		Query:     query,
		Variables: varsJSON,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		result.Err = fmt.Errorf("failed to marshal graphql request body: %w", err)
		return result
	}

	// Build HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		result.Err = fmt.Errorf("failed to create graphql http request: %w", err)
		return result
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Apply headers with tracking
	for _, h := range n.Headers {
		if h.Enabled && h.Key != "" {
			key, err := interpolate(h.Key)
			if err != nil {
				result.Err = fmt.Errorf("failed to interpolate header key: %w", err)
				return result
			}
			value, err := interpolate(h.Value)
			if err != nil {
				result.Err = fmt.Errorf("failed to interpolate header value: %w", err)
				return result
			}
			httpReq.Header.Set(key, value)
		}
	}

	// Track variable reads if tracker is available (before HTTP execution)
	if req.VariableTracker != nil {
		for varKey, varValue := range readVars {
			req.VariableTracker.TrackRead(varKey, varValue)
		}
	}

	if ctx.Err() != nil {
		return result
	}

	// Execute request
	startTime := time.Now()
	httpResp, err := n.HttpClient.Do(httpReq)
	duration := time.Since(startTime)
	if err != nil {
		result.Err = fmt.Errorf("graphql request failed: %w", err)
		return result
	}
	defer httpResp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		result.Err = fmt.Errorf("failed to read graphql response body: %w", err)
		return result
	}

	if ctx.Err() != nil {
		return result
	}

	// Build response headers
	respHeaderModels := make([]mgraphql.GraphQLResponseHeader, 0)
	for key, values := range httpResp.Header {
		for _, value := range values {
			respHeaderModels = append(respHeaderModels, mgraphql.GraphQLResponseHeader{
				ID:          idwrap.NewNow(),
				HeaderKey:   key,
				HeaderValue: value,
			})
		}
	}

	// Build output map
	var respBodyParsed any
	if err := json.Unmarshal(respBody, &respBodyParsed); err != nil {
		// If not valid JSON, use as string
		respBodyParsed = string(respBody)
	}

	requestHeaders := make(map[string]any)
	for _, h := range n.Headers {
		if h.Enabled && h.Key != "" {
			requestHeaders[h.Key] = h.Value
		}
	}

	respHeaders := make(map[string]any)
	for key, values := range httpResp.Header {
		if len(values) == 1 {
			respHeaders[key] = values[0]
		} else {
			anyValues := make([]any, len(values))
			for i, v := range values {
				anyValues[i] = v
			}
			respHeaders[key] = anyValues
		}
	}

	outputMap := map[string]any{
		outputRequestName: map[string]any{
			"url":       url,
			"query":     query,
			"variables": variables,
			"headers":   requestHeaders,
		},
		outputResponseName: map[string]any{
			"status":   float64(httpResp.StatusCode),
			"body":     respBodyParsed,
			"headers":  respHeaders,
			"duration": float64(duration.Milliseconds()),
		},
	}

	// Use tracking version if tracker is available (same pattern as HTTP REQUEST nodes)
	if req.VariableTracker != nil {
		if err := node.WriteNodeVarBulkWithTracking(req, n.Name, outputMap, req.VariableTracker); err != nil {
			result.Err = err
			return result
		}
	} else {
		if err := node.WriteNodeVarBulk(req, n.Name, outputMap); err != nil {
			result.Err = err
			return result
		}
	}

	// Create response with assertions evaluated using UnifiedEnv (same pattern as HTTP)
	respCreate, err := graphqlresponse.ResponseCreateGraphQL(
		ctx,
		respBody,
		httpResp.StatusCode,
		duration,
		respHeaderModels,
		n.GraphQL.ID,
		n.Asserts,
		varMapCopy,
	)
	if err != nil {
		result.Err = err
		return result
	}

	result.AuxiliaryID = &respCreate.GraphQLResponse.ID

	// Check if any assertions failed (same pattern as HTTP)
	done := make(chan struct{})
	for _, assertRes := range respCreate.ResponseAsserts {
		if !assertRes.Success {
			result.Err = fmt.Errorf("assertion failed: %s", assertRes.Value)

			// Still send the response data even though we're failing
			n.SideRespChan <- NodeGraphQLSideResp{
				ExecutionID: req.ExecutionID,
				GraphQL:     n.GraphQL,
				Headers:     n.Headers,
				Response:    respCreate.GraphQLResponse,
				RespHeaders: respCreate.ResponseHeaders,
				RespAsserts: respCreate.ResponseAsserts,
				Done:        done,
			}
			select {
			case <-done:
			case <-ctx.Done():
			}
			return result
		}
	}

	// Send through side channel for persistence
	n.SideRespChan <- NodeGraphQLSideResp{
		ExecutionID: req.ExecutionID,
		GraphQL:     n.GraphQL,
		Headers:     n.Headers,
		Response:    respCreate.GraphQLResponse,
		RespHeaders: respCreate.ResponseHeaders,
		RespAsserts: respCreate.ResponseAsserts,
		Done:        done,
	}
	select {
	case <-done:
	case <-ctx.Done():
	}

	return result
}

func (n *NodeGraphQL) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	result := n.RunSync(ctx, req)
	if ctx.Err() != nil {
		return
	}
	resultChan <- result
}

// GetRequiredVariables implements node.VariableIntrospector.
func (n *NodeGraphQL) GetRequiredVariables() []string {
	var sources []string
	sources = append(sources, n.GraphQL.Url, n.GraphQL.Query, n.GraphQL.Variables)
	for _, h := range n.Headers {
		if h.Enabled {
			sources = append(sources, h.Key, h.Value)
		}
	}
	return expression.ExtractVarKeysFromMultiple(sources...)
}

// GetOutputVariables implements node.VariableIntrospector.
func (n *NodeGraphQL) GetOutputVariables() []string {
	return []string{
		"response.status",
		"response.body",
		"response.headers",
		"response.duration",
		"request.url",
		"request.query",
		"request.variables",
		"request.headers",
	}
}
