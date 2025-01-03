package rlog_test

import (
	"context"
	"net/http"
	"testing"
	"the-dev-tools/backend/internal/api/middleware/mwauth"
	"the-dev-tools/backend/internal/api/rlog"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/logconsole"
	logv1 "the-dev-tools/spec/dist/buf/go/log/v1"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
)

type StreamingHandlerConnMock struct{}

func (s StreamingHandlerConnMock) Spec() connect.Spec {
	return connect.Spec{}
}

func (s StreamingHandlerConnMock) Peer() connect.Peer {
	return connect.Peer{}
}

func (s StreamingHandlerConnMock) Receive(any) error {
	return nil
}

func (s StreamingHandlerConnMock) RequestHeader() http.Header {
	return http.Header{}
}

func (s StreamingHandlerConnMock) Send(any) error {
	return nil
}

func (s StreamingHandlerConnMock) ResponseHeader() http.Header {
	return http.Header{}
}

func (s StreamingHandlerConnMock) ResponseTrailer() http.Header {
	return http.Header{}
}

func TestLogStream(t *testing.T) {
	logc := logconsole.NewLogChanMap()
	userID := idwrap.NewNow()
	ctx := context.Background()

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	srv := rlog.NewRlogRPC(logc)
	req := &connect.Request[emptypb.Empty]{}
	stream := &connect.ServerStream[logv1.LogStreamResponse]{}

	deadline := time.Now().Add(2 * time.Second)
	deadlinedCtx, a := context.WithDeadline(authedCtx, deadline)
	defer a()

	srv.LogStream(deadlinedCtx, req, stream)
}
