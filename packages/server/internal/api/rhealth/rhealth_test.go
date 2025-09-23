package rhealth

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	healthv1 "the-dev-tools/spec/dist/buf/go/health/v1"
)

func TestHealthServiceRPC_HealthCheck(t *testing.T) {
	t.Parallel()

	svc := New()
	ctx := context.Background()
	req := connect.NewRequest(&emptypb.Empty{})

	resp, err := svc.HealthCheck(ctx, req)
	if err != nil {
		t.Fatalf("HealthCheck returned error: %v", err)
	}
	if resp == nil {
		t.Fatalf("HealthCheck returned nil response")
	}
	if resp.Msg == nil {
		t.Fatalf("HealthCheck returned nil message")
	}
	if !proto.Equal(resp.Msg, &healthv1.HealthCheckResponse{}) {
		t.Fatalf("HealthCheck returned unexpected payload: %v", resp.Msg)
	}
}
