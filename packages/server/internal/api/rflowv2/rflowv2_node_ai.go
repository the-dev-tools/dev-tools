//nolint:revive // exported
package rflowv2

import (
	"context"

	"connectrpc.com/connect"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

func (s *FlowServiceV2RPC) NodeAiCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeAiCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (s *FlowServiceV2RPC) NodeAiInsert(
	ctx context.Context,
	req *connect.Request[flowv1.NodeAiInsertRequest],
) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (s *FlowServiceV2RPC) NodeAiUpdate(
	ctx context.Context,
	req *connect.Request[flowv1.NodeAiUpdateRequest],
) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (s *FlowServiceV2RPC) NodeAiDelete(
	ctx context.Context,
	req *connect.Request[flowv1.NodeAiDeleteRequest],
) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (s *FlowServiceV2RPC) NodeAiSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeAiSyncResponse],
) error {
	return connect.NewError(connect.CodeUnimplemented, nil)
}
