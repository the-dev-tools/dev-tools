package rlog

import (
	"context"
	"the-dev-tools/backend/internal/api"
	"the-dev-tools/backend/internal/api/middleware/mwauth"
	"the-dev-tools/backend/pkg/logconsole"
	logv1 "the-dev-tools/spec/dist/buf/go/log/v1"
	"the-dev-tools/spec/dist/buf/go/log/v1/logv1connect"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
)

type RlogRPC struct {
	logChannels logconsole.LogChanMap
}

func NewRlogRPC(logMap logconsole.LogChanMap) *RlogRPC {
	return &RlogRPC{
		logChannels: logMap,
	}
}

func CreateService(srv *RlogRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := logv1connect.NewLogServiceHandler(srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *RlogRPC) LogStream(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[logv1.LogStreamResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return err
	}

	streamChan := make(chan logconsole.LogMessage)
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
