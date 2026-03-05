package nwssend

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"nhooyr.io/websocket"
)

// echoServer creates a test WS server that echoes messages back.
func echoServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
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

func wsURL(s *httptest.Server) string {
	return "ws" + strings.TrimPrefix(s.URL, "http")
}

func newReq(edgeMap mflow.EdgesMap) *node.FlowNodeRequest {
	return &node.FlowNodeRequest{
		VarMap:           make(map[string]any),
		ReadWriteLock:    &sync.RWMutex{},
		EdgeSourceMap:    edgeMap,
		Timeout:          10 * time.Second,
		PendingAtmoicMap: make(map[idwrap.IDWrap]uint32),
		PendingMapMu:     &sync.Mutex{},
	}
}

func TestNodeWsSend_Success(t *testing.T) {
	srv := echoServer(t)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Establish a real WebSocket connection
	conn, _, err := websocket.Dial(ctx, wsURL(srv), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Set up the WsSend node
	nodeID := idwrap.NewNow()
	n := New(nodeID, "SendMsg", "MyWS", "hello world")

	req := newReq(mflow.EdgesMap{})
	// Simulate WsConnection node having stored the connection
	_ = node.WriteNodeVar(req, "MyWS", "_conn", conn)

	result := n.RunSync(ctx, req)
	if result.Err != nil {
		t.Fatalf("RunSync error: %v", result.Err)
	}

	// Verify output variables
	sentMsg, err := node.ReadNodeVar(req, "SendMsg", "sentMessage")
	if err != nil {
		t.Fatalf("read sentMessage: %v", err)
	}
	if sentMsg != "hello world" {
		t.Errorf("sentMessage = %v, want hello world", sentMsg)
	}

	connNode, err := node.ReadNodeVar(req, "SendMsg", "connectionNode")
	if err != nil {
		t.Fatalf("read connectionNode: %v", err)
	}
	if connNode != "MyWS" {
		t.Errorf("connectionNode = %v, want MyWS", connNode)
	}

	// Read the echo back to confirm message was actually sent
	_, msg, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if string(msg) != "hello world" {
		t.Errorf("echoed = %v, want hello world", string(msg))
	}
}

func TestNodeWsSend_Interpolation(t *testing.T) {
	srv := echoServer(t)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL(srv), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	nodeID := idwrap.NewNow()
	n := New(nodeID, "SendMsg", "MyWS", "hello {{ name }}")

	req := newReq(mflow.EdgesMap{})
	_ = node.WriteNodeVar(req, "MyWS", "_conn", conn)
	req.VarMap["name"] = "world"

	result := n.RunSync(ctx, req)
	if result.Err != nil {
		t.Fatalf("RunSync error: %v", result.Err)
	}

	sentMsg, _ := node.ReadNodeVar(req, "SendMsg", "sentMessage")
	if sentMsg != "hello world" {
		t.Errorf("sentMessage = %v, want hello world", sentMsg)
	}
}

func TestNodeWsSend_MissingConnection(t *testing.T) {
	nodeID := idwrap.NewNow()
	n := New(nodeID, "SendMsg", "NonExistent", "hello")

	req := newReq(mflow.EdgesMap{})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result := n.RunSync(ctx, req)
	if result.Err == nil {
		t.Fatal("expected error for missing connection node")
	}
}
