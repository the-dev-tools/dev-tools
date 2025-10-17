package rlog

import (
	"context"
	"encoding/json"
	"fmt"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/logconsole"
	logv1 "the-dev-tools/spec/dist/buf/go/log/v1"
	"the-dev-tools/spec/dist/buf/go/log/v1/logv1connect"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
	structpb "google.golang.org/protobuf/types/known/structpb"
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

// protoLogLevelFallback ensures stream serialization always emits a valid enum.
const protoLogLevelFallback = logv1.LogLevel_LOG_LEVEL_UNSPECIFIED

func logLevelToProto(level logconsole.LogLevel) (logv1.LogLevel, error) {
	switch level {
	case logconsole.LogLevelUnspecified:
		return logv1.LogLevel_LOG_LEVEL_UNSPECIFIED, nil
	case logconsole.LogLevelWarning:
		return logv1.LogLevel_LOG_LEVEL_WARNING, nil
	case logconsole.LogLevelError:
		return logv1.LogLevel_LOG_LEVEL_ERROR, nil
	default:
		return protoLogLevelFallback, fmt.Errorf("unknown log level: %d", level)
	}
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
			var val *structpb.Value
			if logMessage.JSON != "" {
				var v any
				if err := json.Unmarshal([]byte(logMessage.JSON), &v); err == nil {
					if pv, err2 := structpb.NewValue(v); err2 == nil {
						val = pv
					}
				}
			}
			level, convErr := logLevelToProto(logMessage.Level)
			if convErr != nil {
				// debug: unknown log level from logconsole; using proto fallback.
			}
			b := &logv1.LogStreamResponse{
				LogId: logMessage.LogID.Bytes(),
				Name:  logMessage.Name,
				Level: level,
				Value: val,
			}
			if sendErr := stream.Send(b); sendErr != nil {
				return sendErr
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
