package snoderequest

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
)

type NodeRequestService struct {
	queries *gen.Queries
}

func New(queries *gen.Queries) NodeRequestService {
	return NodeRequestService{queries: queries}
}

func (nrs NodeRequestService) TX(tx *sql.Tx) NodeRequestService {
	return NodeRequestService{queries: nrs.queries.WithTx(tx)}
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
		FlowNodeID:       nr.FlowNodeID,
		EndpointID:       nr.EndpointID,
		ExampleID:        nr.ExampleID,
		DeltaExampleID:   nr.DeltaExampleID,
		DeltaEndpointID:  nr.DeltaEndpointID,
		HasRequestConfig: nr.HasRequestConfig,
	}
}

func ConvertToModelNodeRequest(nr gen.FlowNodeRequest) *mnrequest.MNRequest {
	return &mnrequest.MNRequest{
		FlowNodeID:       nr.FlowNodeID,
		EndpointID:       nr.EndpointID,
		ExampleID:        nr.ExampleID,
		DeltaExampleID:   nr.DeltaExampleID,
		DeltaEndpointID:  nr.DeltaEndpointID,
		HasRequestConfig: nr.HasRequestConfig,
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
		FlowNodeID:       nodeRequest.FlowNodeID,
		EndpointID:       nodeRequest.EndpointID,
		ExampleID:        nodeRequest.ExampleID,
		DeltaExampleID:   nodeRequest.DeltaExampleID,
		DeltaEndpointID:  nodeRequest.DeltaEndpointID,
		HasRequestConfig: nodeRequest.HasRequestConfig,
	})
}

func (nrs NodeRequestService) CreateNodeRequestBulk(ctx context.Context, nr []mnrequest.MNRequest) error {
	for _, nodeRequest := range nr {
		err := nrs.queries.CreateFlowNodeRequest(ctx, gen.CreateFlowNodeRequestParams{
			FlowNodeID:       nodeRequest.FlowNodeID,
			EndpointID:       nodeRequest.EndpointID,
			ExampleID:        nodeRequest.ExampleID,
			DeltaExampleID:   nodeRequest.DeltaExampleID,
			DeltaEndpointID:  nodeRequest.DeltaEndpointID,
			HasRequestConfig: nodeRequest.HasRequestConfig,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (nrs NodeRequestService) UpdateNodeRequest(ctx context.Context, nr mnrequest.MNRequest) error {
	nodeRequest := ConvertToDBNodeRequest(nr)
	return nrs.queries.UpdateFlowNodeRequest(ctx, gen.UpdateFlowNodeRequestParams{
		FlowNodeID:       nodeRequest.FlowNodeID,
		EndpointID:       nodeRequest.EndpointID,
		ExampleID:        nodeRequest.ExampleID,
		DeltaExampleID:   nodeRequest.DeltaExampleID,
		DeltaEndpointID:  nodeRequest.DeltaEndpointID,
		HasRequestConfig: nodeRequest.HasRequestConfig,
	})
}

func (nrs NodeRequestService) DeleteNodeRequest(ctx context.Context, id idwrap.IDWrap) error {
	err := nrs.queries.DeleteFlowNodeRequest(ctx, id)
	if err != nil {
		return err
	}
	return nil
}
