package rflow

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
)

// FlowService provides flow-related operations
// This is a stub implementation to maintain build compatibility
type FlowService struct{}

// New creates a new FlowService
func New() FlowService {
	return FlowService{}
}

// Flow represents a flow
type Flow struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GetFlow retrieves a flow by ID
func (fs FlowService) GetFlow(ctx context.Context, id string) (*connect.Response[Flow], error) {
	// Stub implementation
	return connect.NewResponse(&Flow{
		ID:   id,
		Name: "Stub Flow",
	}), nil
}

// CreateFlow creates a new flow
func (fs FlowService) CreateFlow(ctx context.Context, req *connect.Request[Flow]) (*connect.Response[Flow], error) {
	// Stub implementation
	return connect.NewResponse(req.Msg), nil
}

// UpdateFlow updates an existing flow
func (fs FlowService) UpdateFlow(ctx context.Context, req *connect.Request[Flow]) (*connect.Response[Flow], error) {
	// Stub implementation
	return connect.NewResponse(req.Msg), nil
}

// DeleteFlow deletes a flow
func (fs FlowService) DeleteFlow(ctx context.Context, req *connect.Request[Flow]) (*connect.Response[http.Empty], error) {
	// Stub implementation
	return connect.NewResponse(&http.Empty{}), nil
}

// ListFlows lists all flows
func (fs FlowService) ListFlows(ctx context.Context) (*connect.Response[[]Flow], error) {
	// Stub implementation
	flows := []Flow{
		{ID: "1", Name: "Stub Flow 1"},
		{ID: "2", Name: "Stub Flow 2"},
	}
	return connect.NewResponse(&flows), nil
}