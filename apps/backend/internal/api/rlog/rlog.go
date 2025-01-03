package rlog

import (
	"context"
	"errors"
	"fmt"
	"the-dev-tools/backend/internal/api"
	"the-dev-tools/backend/pkg/idwrap"
	logv1 "the-dev-tools/spec/dist/buf/go/log/v1"
	"the-dev-tools/spec/dist/buf/go/log/v1/logv1connect"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
)

type LogMessage struct {
	LogID idwrap.IDWrap
	Value string
}

type RlogRPC struct {
	logChannels map[idwrap.IDWrap]chan LogMessage
}

func NewRlogRPC() *RlogRPC {
	return &RlogRPC{
		logChannels: make(map[idwrap.IDWrap]chan LogMessage),
	}
}

func CreateService(srv *RlogRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := logv1connect.NewLogServiceHandler(srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (r *RlogRPC) LogMessage(logID idwrap.IDWrap, value string) error {
	ch, ok := r.logChannels[logID]
	if !ok {
		return fmt.Errorf("logID not found")
	}
	ch <- LogMessage{
		LogID: logID,
		Value: value,
	}
	return nil
}

func (c *RlogRPC) LogStream(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[logv1.LogStreamResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errors.New("log.v1.LogService.LogStream is not implemented"))
}
