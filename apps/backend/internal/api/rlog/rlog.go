package rlog

import (
	"context"
	"fmt"
	"the-dev-tools/backend/internal/api"
	"the-dev-tools/backend/internal/api/middleware/mwauth"
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

func (c *RlogRPC) LogStream(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[logv1.LogStreamResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return err
	}

	streamChan := make(chan LogMessage)
	c.logChannels[userID] = streamChan

	for {
		select {
		case <-ctx.Done():
			return nil
		case logMessage := <-streamChan:
			b := &logv1.LogStreamResponse{
				LogId: logMessage.LogID.Bytes(),
				Value: logMessage.Value,
			}
			stream.Send(b)
		}
	}
}
