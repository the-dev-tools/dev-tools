package rlog

import (
    "context"
    "the-dev-tools/server/internal/api"
    "the-dev-tools/server/internal/api/middleware/mwauth"
    "the-dev-tools/server/pkg/logconsole"
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
	return c.LogStreamAdHoc(ctx, req, stream)
}

func (c *RlogRPC) LogStreamAdHoc(ctx context.Context, req *connect.Request[emptypb.Empty], stream api.ServerStreamAdHoc[logv1.LogStreamResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return err
	}

	lmc := c.logChannels.AddLogChannel(userID)

	for {
		select {
        case logMessage := <-lmc:
            var jsonStrPtr *string
            if logMessage.JSON != "" {
                s := logMessage.JSON
                jsonStrPtr = &s
            }
            b := &logv1.LogStreamResponse{
                LogId: logMessage.LogID.Bytes(),
                Value: logMessage.Value,
                Level: logv1.LogLevel(logMessage.Level),
                Json:  jsonStrPtr,
            }
            err = stream.Send(b)
            if err != nil {
                return err
            }
            continue
		case <-ctx.Done():
			err = ctx.Err()
		}
		break
	}
	c.logChannels.DeleteLogChannel(userID)
	return err
}

// no helper needed; JSON payload comes from logconsole
