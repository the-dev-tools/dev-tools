package nwsconnection

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/coder/websocket"
)

// echoServer creates a test WS server that echoes messages back.
func echoServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Logf("accept error: %v", err)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		for {
			typ, msg, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			if err := conn.Write(r.Context(), typ, msg); err != nil {
				return
			}
		}
	}))
}

// wsURL converts an httptest server URL to a ws:// URL.
func wsURL(s *httptest.Server) string {
	return "ws" + strings.TrimPrefix(s.URL, "http")
}

func newReq(edgeMap mflow.EdgesMap, nodeMap map[idwrap.IDWrap]node.FlowNode) *node.FlowNodeRequest {
	return &node.FlowNodeRequest{
		VarMap:           make(map[string]any),
		ReadWriteLock:    &sync.RWMutex{},
		NodeMap:          nodeMap,
		EdgeSourceMap:    edgeMap,
		Timeout:          10 * time.Second,
		PendingAtmoicMap: make(map[idwrap.IDWrap]uint32),
		PendingMapMu:     &sync.Mutex{},
	}
}

func TestNodeWsConnection_Connect(t *testing.T) {
	srv := echoServer(t)
	defer srv.Close()

	nodeID := idwrap.NewNow()
	n := New(nodeID, "MyWS", wsURL(srv), nil)

	req := newReq(mflow.EdgesMap{}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := n.RunSync(ctx, req)
	if result.Err != nil {
		t.Fatalf("RunSync error: %v", result.Err)
	}

	// Verify url variable
	urlVal, err := node.ReadNodeVar(req, "MyWS", "url")
	if err != nil {
		t.Fatalf("read url var: %v", err)
	}
	if urlVal != wsURL(srv) {
		t.Errorf("url = %v, want %v", urlVal, wsURL(srv))
	}

	// Verify connected variable
	connectedVal, err := node.ReadNodeVar(req, "MyWS", "connected")
	if err != nil {
		t.Fatalf("read connected var: %v", err)
	}
	if connectedVal != true {
		t.Errorf("connected = %v, want true", connectedVal)
	}

	// Verify _conn is a *websocket.Conn
	connVal, err := node.ReadNodeVar(req, "MyWS", "_conn")
	if err != nil {
		t.Fatalf("read _conn var: %v", err)
	}
	if _, ok := connVal.(*websocket.Conn); !ok {
		t.Errorf("_conn type = %T, want *websocket.Conn", connVal)
	}

	cancel() // Clean up WS connection
}

func TestNodeWsConnection_PassiveMessageLogging(t *testing.T) {
	srv := echoServer(t)
	defer srv.Close()

	nodeID := idwrap.NewNow()
	n := New(nodeID, "MyWS", wsURL(srv), nil)

	var statuses []runner.FlowNodeStatus
	var mu sync.Mutex
	logFunc := node.LogPushFunc(func(s runner.FlowNodeStatus) {
		mu.Lock()
		defer mu.Unlock()
		statuses = append(statuses, s)
	})

	req := newReq(mflow.EdgesMap{}, nil)
	req.LogPushFunc = logFunc

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := n.RunSync(ctx, req)
	if result.Err != nil {
		t.Fatalf("RunSync error: %v", result.Err)
	}

	// Get the connection and send a message to trigger the passive listener
	connVal, _ := node.ReadNodeVar(req, "MyWS", "_conn")
	conn := connVal.(*websocket.Conn)

	if err := conn.Write(ctx, websocket.MessageText, []byte("hello")); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Wait for the echo to be read and logged
	time.Sleep(200 * time.Millisecond)

	// Verify lastMessage was set
	lastMsg, err := node.ReadNodeVar(req, "MyWS", "lastMessage")
	if err != nil {
		t.Fatalf("read lastMessage: %v", err)
	}
	if lastMsg != "hello" {
		t.Errorf("lastMessage = %v, want hello", lastMsg)
	}

	// Verify a status was emitted
	mu.Lock()
	count := len(statuses)
	mu.Unlock()
	if count == 0 {
		t.Error("expected at least one status event for the echoed message")
	}

	cancel()
}

func TestNodeWsConnection_DialError(t *testing.T) {
	nodeID := idwrap.NewNow()
	n := New(nodeID, "MyWS", "ws://127.0.0.1:1", nil) // nothing listening

	req := newReq(mflow.EdgesMap{}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result := n.RunSync(ctx, req)
	if result.Err == nil {
		t.Fatal("expected error for bad dial")
	}
}

func TestNodeWsConnection_URLInterpolation(t *testing.T) {
	srv := echoServer(t)
	defer srv.Close()

	nodeID := idwrap.NewNow()
	n := New(nodeID, "MyWS", "{{ baseUrl }}", nil)

	req := newReq(mflow.EdgesMap{}, nil)
	req.VarMap["baseUrl"] = wsURL(srv)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := n.RunSync(ctx, req)
	if result.Err != nil {
		t.Fatalf("RunSync error: %v", result.Err)
	}

	urlVal, err := node.ReadNodeVar(req, "MyWS", "url")
	if err != nil {
		t.Fatalf("read url var: %v", err)
	}
	if urlVal != wsURL(srv) {
		t.Errorf("url = %v, want %v", urlVal, wsURL(srv))
	}

	cancel()
}
