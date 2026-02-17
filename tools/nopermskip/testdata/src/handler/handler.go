package handler

import (
	"context"

	"connectrpc.com/connect"
)

type FooRPC struct{}
type Req struct{}
type Resp struct{}

// Good: has permission check via method containing "Access"
func (s *FooRPC) GoodHandler(ctx context.Context, req *connect.Request[Req]) (*connect.Response[Resp], error) {
	s.checkWorkspaceReadAccess(ctx)
	return nil, nil
}

// Bad: no permission check
func (s *FooRPC) BadHandler(ctx context.Context, req *connect.Request[Req]) (*connect.Response[Resp], error) { // want "RPC handler BadHandler missing permission check"
	return nil, nil
}

// Good: uses GetContextUserID
func (s *FooRPC) UserHandler(ctx context.Context, req *connect.Request[Req]) (*connect.Response[Resp], error) {
	GetContextUserID(ctx)
	return nil, nil
}

// Good: uses CheckOwnerWorkspace
func (s *FooRPC) OwnerHandler(ctx context.Context, req *connect.Request[Req]) (*connect.Response[Resp], error) {
	CheckOwnerWorkspace(ctx)
	return nil, nil
}

// Good: nolint directive
//
//nolint:nopermskip
func (s *FooRPC) PublicHandler(ctx context.Context, req *connect.Request[Req]) (*connect.Response[Resp], error) {
	return nil, nil
}

// Good: streaming handler with permission check
func (s *FooRPC) StreamHandler(ctx context.Context, req *connect.Request[Req], stream *connect.ServerStream[Resp]) error {
	s.checkWorkspaceReadAccess(ctx)
	return nil
}

// Bad: streaming handler without permission check
func (s *FooRPC) BadStreamHandler(ctx context.Context, req *connect.Request[Req], stream *connect.ServerStream[Resp]) error { // want "RPC handler BadStreamHandler missing permission check"
	return nil
}

// Not an RPC handler — no connect.Request param, should be ignored
func (s *FooRPC) InternalHelper(ctx context.Context) error {
	return nil
}

// Not a method — should be ignored even with connect.Request param
func FreeFunction(ctx context.Context, req *connect.Request[Req]) (*connect.Response[Resp], error) {
	return nil, nil
}

// Unexported — should be ignored
func (s *FooRPC) unexported(ctx context.Context, req *connect.Request[Req]) (*connect.Response[Resp], error) {
	return nil, nil
}

// Good: uses listUserWorkspaces pattern
func (s *FooRPC) ListHandler(ctx context.Context, req *connect.Request[Req]) (*connect.Response[Resp], error) {
	listUserWorkspaces(ctx)
	return nil, nil
}

// Good: uses listAccessibleFlows pattern
func (s *FooRPC) ListAccessHandler(ctx context.Context, req *connect.Request[Req]) (*connect.Response[Resp], error) {
	listAccessibleFlows(ctx)
	return nil, nil
}

// Good: uses streamFlowSync pattern
func (s *FooRPC) SyncHandler(ctx context.Context, req *connect.Request[Req]) (*connect.Response[Resp], error) {
	streamFlowSync(ctx)
	return nil, nil
}

// Helper stubs
func (s *FooRPC) checkWorkspaceReadAccess(ctx context.Context) error { return nil }
func GetContextUserID(ctx context.Context)                           {}
func CheckOwnerWorkspace(ctx context.Context)                        {}
func listUserWorkspaces(ctx context.Context)                         {}
func listAccessibleFlows(ctx context.Context)                        {}
func streamFlowSync(ctx context.Context)                             {}
