package snoderequest

import (
	"context"
	"database/sql"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mnode/mnrequest"
	"the-dev-tools/db/pkg/sqlc/gen"
)

type NodeRequestService struct {
	queries *gen.Queries
}

func New(queries *gen.Queries) NodeRequestService {
	return NodeRequestService{queries: queries}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*NodeRequestService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &NodeRequestService{
		queries: queries,
	}, nil
}

func ConvertToDBNodeRequest(nr mnrequest.MNRequest) gen.FlowNodeRequest {
	return gen.FlowNodeRequest{
		FlowNodeID: nr.FlowNodeID,
		Name:       nr.Name,
		EndpointID: nr.EndpointID,
		ExampleID:  nr.ExampleID,
	}
}

func ConvertToModelNodeRequest(nr gen.FlowNodeRequest) *mnrequest.MNRequest {
	return &mnrequest.MNRequest{
		FlowNodeID: nr.FlowNodeID,
		Name:       nr.Name,
		EndpointID: nr.EndpointID,
		ExampleID:  nr.ExampleID,
	}
}

func (nrs NodeRequestService) GetNodeRequest(ctx context.Context, id idwrap.IDWrap) (*mnrequest.MNRequest, error) {
	nodeRequest, err := nrs.queries.GetFlowNodeRequest(ctx, id)
	if err != nil {
		return nil, err
	}
	return ConvertToModelNodeRequest(nodeRequest), nil
}

func (nrs NodeRequestService) CreateNodeRequest(ctx context.Context, nr mnrequest.MNRequest) error {
	nodeRequest := ConvertToDBNodeRequest(nr)
	return nrs.queries.CreateFlowNodeRequest(ctx, gen.CreateFlowNodeRequestParams{
		FlowNodeID: nodeRequest.FlowNodeID,
		Name:       nodeRequest.Name,
		EndpointID: nodeRequest.EndpointID,
		ExampleID:  nodeRequest.ExampleID,
	})
}

func (nrs NodeRequestService) UpdateNodeRequest(ctx context.Context, nr mnrequest.MNRequest) error {
	nodeRequest := ConvertToDBNodeRequest(nr)
	return nrs.queries.UpdateFlowNodeRequest(ctx, gen.UpdateFlowNodeRequestParams{
		FlowNodeID: nodeRequest.FlowNodeID,
		Name:       nodeRequest.Name,
		EndpointID: nodeRequest.EndpointID,
		ExampleID:  nodeRequest.ExampleID,
	})
}

func (nrs NodeRequestService) DeleteNodeRequest(ctx context.Context, id idwrap.IDWrap) error {
	err := nrs.queries.DeleteFlowNodeRequest(ctx, id)
	if err != nil {
		return err
	}
	return nil
}
