package rlog_test

import (
	"context"
	"fmt"
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

type ServerStreamingHandlerMock[I any] struct {
	sendStream func(I)
}

func (s ServerStreamingHandlerMock[I]) Send(a I) error {
	s.sendStream(a)
	return nil
}

func TestLogStreamCancel(t *testing.T) {
	logc := logconsole.NewLogChanMap()
	userID := idwrap.NewNow()
	ctx := context.Background()

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	srv := rlog.NewRlogRPC(logc)
	req := &connect.Request[emptypb.Empty]{}

	var count int
	sendStream := func(a *logv1.LogStreamResponse) {
		count++
	}
	stream := ServerStreamingHandlerMock[*logv1.LogStreamResponse]{
		sendStream: sendStream,
	}

	deadline := time.Now().Add(5 * time.Millisecond)
	deadlinedCtx, a := context.WithDeadline(authedCtx, deadline)
	defer a()

	err := srv.LogStreamAdHoc(deadlinedCtx, req, stream)
	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
	if count != 0 {
		t.Errorf("expected count to be 0, got %v", count)
	}
}

func TestLogStream(t *testing.T) {
	logc := logconsole.NewLogChanMap()
	userID := idwrap.NewNow()
	ctx := context.Background()

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)
	srv := rlog.NewRlogRPC(logc)
	req := &connect.Request[emptypb.Empty]{}
	var count int
	sendStream := func(a *logv1.LogStreamResponse) {
		count++
	}
	stream := ServerStreamingHandlerMock[*logv1.LogStreamResponse]{
		sendStream: sendStream,
	}

	cancelableCtx, cancel := context.WithCancel(authedCtx)

	go func() {
		time.Sleep(50 * time.Millisecond)
		err := logc.SendMsgToUserWithContext(authedCtx, idwrap.NewNow(), "test")
		if err != nil {
			t.Error(err)
		}
		cancel()
	}()

	fmt.Println("LogStreamAdHoc")
	err := srv.LogStreamAdHoc(cancelableCtx, req, stream)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if count != 1 {
		t.Errorf("expected count to be 1, got %v", count)
	}
}
