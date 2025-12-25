//nolint:revive // exported
package rlog

import (
	"context"
	"reflect"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/idwrap"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/log/v1"
	"the-dev-tools/spec/dist/buf/go/api/log/v1/logv1connect"
)

const (
	EventTypeInsert = "insert"
	EventTypeUpdate = "update"
	EventTypeDelete = "delete"
)

// NewLogValue converts a Go value to a protobuf-compatible structpb.Value.
// It handles types that structpb.NewValue doesn't support natively, like []int.
func NewLogValue(v any) (*structpb.Value, error) {
	v = makeProtoCompatible(v)
	return structpb.NewValue(v)
}

// makeProtoCompatible recursively converts Go values to protobuf-compatible types.
func makeProtoCompatible(v any) any {
	if v == nil {
		return nil
	}

	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.Slice, reflect.Array:
		// Convert slices/arrays to []any
		result := make([]any, val.Len())
		for i := range val.Len() {
			result[i] = makeProtoCompatible(val.Index(i).Interface())
		}
		return result

	case reflect.Map:
		// Convert maps to map[string]any
		result := make(map[string]any)
		iter := val.MapRange()
		for iter.Next() {
			key := iter.Key().Interface()
			// Convert key to string if it isn't already
			keyStr, ok := key.(string)
			if !ok {
				keyStr = reflect.ValueOf(key).String()
			}
			result[keyStr] = makeProtoCompatible(iter.Value().Interface())
		}
		return result

	default:
		return v
	}
}

type LogTopic struct {
	UserID idwrap.IDWrap
}

type LogEvent struct {
	Type string
	Log  *apiv1.Log
}

type LogServiceRPC struct {
	stream eventstream.SyncStreamer[LogTopic, LogEvent]
}

func New(stream eventstream.SyncStreamer[LogTopic, LogEvent]) LogServiceRPC {
	return LogServiceRPC{
		stream: stream,
	}
}

func CreateService(srv LogServiceRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := logv1connect.NewLogServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func logSyncResponseFrom(evt LogEvent) *apiv1.LogSyncResponse {
	if evt.Log == nil {
		return nil
	}

	switch evt.Type {
	case EventTypeInsert:
		msg := &apiv1.LogSync{
			Value: &apiv1.LogSync_ValueUnion{
				Kind: apiv1.LogSync_ValueUnion_KIND_INSERT,
				Insert: &apiv1.LogSyncInsert{
					LogId: evt.Log.LogId,
					Name:  evt.Log.Name,
					Level: evt.Log.Level,
					Value: evt.Log.Value,
				},
			},
		}
		return &apiv1.LogSyncResponse{Items: []*apiv1.LogSync{msg}}
	case EventTypeUpdate:
		update := &apiv1.LogSyncUpdate{
			LogId: evt.Log.LogId,
		}
		if evt.Log.Name != "" {
			update.Name = &evt.Log.Name
		}
		if evt.Log.Level != apiv1.LogLevel_LOG_LEVEL_UNSPECIFIED {
			update.Level = &evt.Log.Level
		}
		if evt.Log.Value != nil {
			update.Value = &apiv1.LogSyncUpdate_ValueUnion{
				Kind:  apiv1.LogSyncUpdate_ValueUnion_KIND_VALUE,
				Value: evt.Log.Value,
			}
		}
		msg := &apiv1.LogSync{
			Value: &apiv1.LogSync_ValueUnion{
				Kind:   apiv1.LogSync_ValueUnion_KIND_UPDATE,
				Update: update,
			},
		}
		return &apiv1.LogSyncResponse{Items: []*apiv1.LogSync{msg}}
	case EventTypeDelete:
		msg := &apiv1.LogSync{
			Value: &apiv1.LogSync_ValueUnion{
				Kind: apiv1.LogSync_ValueUnion_KIND_DELETE,
				Delete: &apiv1.LogSyncDelete{
					LogId: evt.Log.LogId,
				},
			},
		}
		return &apiv1.LogSyncResponse{Items: []*apiv1.LogSync{msg}}
	default:
		return nil
	}
}

func (c *LogServiceRPC) LogCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.LogCollectionResponse], error) {
	// Authenticate the user
	_, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Since this is a read-only collection for streaming logs, we return an empty collection
	// The actual logs will be delivered through the sync stream
	return connect.NewResponse(&apiv1.LogCollectionResponse{Items: []*apiv1.Log{}}), nil
}

func (c *LogServiceRPC) LogSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.LogSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return c.streamLogSync(ctx, userID, stream.Send)
}

func (c *LogServiceRPC) streamLogSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.LogSyncResponse) error) error {
	snapshot := func(ctx context.Context) ([]eventstream.Event[LogTopic, LogEvent], error) {
		// Return empty snapshot for logs - they are streaming-only
		return []eventstream.Event[LogTopic, LogEvent]{}, nil
	}

	filter := func(topic LogTopic) bool {
		// Only deliver logs to the user who owns them
		return topic.UserID == userID
	}

	events, err := c.stream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp := logSyncResponseFrom(evt.Payload)
			if resp == nil {
				continue
			}
			if err := send(resp); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
