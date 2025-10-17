package rlog

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logconsole"
	logv1 "the-dev-tools/spec/dist/buf/go/log/v1"
)

func TestLogLevelToProto(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name    string
		level   logconsole.LogLevel
		want    logv1.LogLevel
		wantErr bool
	}{{
		name:  "unspecified",
		level: logconsole.LogLevelUnspecified,
		want:  logv1.LogLevel_LOG_LEVEL_UNSPECIFIED,
	}, {
		name:  "warning",
		level: logconsole.LogLevelWarning,
		want:  logv1.LogLevel_LOG_LEVEL_WARNING,
	}, {
		name:  "error",
		level: logconsole.LogLevelError,
		want:  logv1.LogLevel_LOG_LEVEL_ERROR,
	}, {
		name:    "unknown",
		level:   logconsole.LogLevel(99),
		want:    protoLogLevelFallback,
		wantErr: true,
	}}

	for _, tc := range testcases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := logLevelToProto(tc.level)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for level %d", tc.level)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestLogStreamAdHocLogLevels(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name  string
		level logconsole.LogLevel
		want  logv1.LogLevel
	}{{
		name:  "unspecified",
		level: logconsole.LogLevelUnspecified,
		want:  logv1.LogLevel_LOG_LEVEL_UNSPECIFIED,
	}, {
		name:  "warning",
		level: logconsole.LogLevelWarning,
		want:  logv1.LogLevel_LOG_LEVEL_WARNING,
	}, {
		name:  "error",
		level: logconsole.LogLevelError,
		want:  logv1.LogLevel_LOG_LEVEL_ERROR,
	}, {
		name:  "unknown",
		level: logconsole.LogLevel(99),
		want:  protoLogLevelFallback,
	}}

	for _, tc := range testcases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			baseCtx := mwauth.CreateAuthedContext(context.Background(), idwrap.NewNow())
			ctx, cancel := context.WithCancel(baseCtx)
			defer cancel()

			rpc := NewRlogRPC(logconsole.NewLogChanMap())
			stream := newTestStream()
			req := connect.NewRequest(new(emptypb.Empty))

			errCh := make(chan error, 1)
			go func() {
				errCh <- rpc.LogStreamAdHoc(ctx, req, stream)
			}()

			logID := idwrap.NewNow()
			if err := sendLogMessageWithRetry(baseCtx, &rpc.logChannels, logID, "example", tc.level, nil); err != nil {
				t.Fatalf("send log message: %v", err)
			}

			select {
			case <-stream.sent:
			case <-time.After(time.Second):
				t.Fatal("timeout waiting for log stream response")
			}

			cancel()

			if err := <-errCh; !errors.Is(err, context.Canceled) {
				t.Fatalf("expected context canceled error, got %v", err)
			}

			msgs := stream.Messages()
			if len(msgs) != 1 {
				t.Fatalf("expected 1 message, got %d", len(msgs))
			}

			if msgs[0].GetLevel() != tc.want {
				t.Fatalf("expected level %v, got %v", tc.want, msgs[0].GetLevel())
			}
		})
	}
}

func sendLogMessageWithRetry(ctx context.Context, logChannels *logconsole.LogChanMap, logID idwrap.IDWrap, name string, level logconsole.LogLevel, payload map[string]any) error {
	deadline := time.Now().Add(100 * time.Millisecond)
	var lastErr error

	for time.Now().Before(deadline) {
		if err := logChannels.SendMsgToUserWithContext(ctx, logID, name, level, payload); err != nil {
			lastErr = err
			time.Sleep(time.Millisecond * 5)
			continue
		}
		return nil
	}

	if lastErr == nil {
		lastErr = context.DeadlineExceeded
	}

	return lastErr
}

type testStream struct {
	mu   sync.Mutex
	msgs []*logv1.LogStreamResponse
	sent chan struct{}
	err  error
}

func newTestStream() *testStream {
	return &testStream{sent: make(chan struct{}, 1)}
}

func (ts *testStream) Send(res *logv1.LogStreamResponse) error {
	ts.mu.Lock()
	ts.msgs = append(ts.msgs, res)
	ts.mu.Unlock()

	select {
	case ts.sent <- struct{}{}:
	default:
	}

	return ts.err
}

func (ts *testStream) Messages() []*logv1.LogStreamResponse {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	copied := make([]*logv1.LogStreamResponse, len(ts.msgs))
	copy(copied, ts.msgs)

	return copied
}
