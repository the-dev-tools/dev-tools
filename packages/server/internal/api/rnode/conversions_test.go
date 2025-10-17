package rnode

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodeexecution"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	nodev1 "the-dev-tools/spec/dist/buf/go/flow/node/v1"
)

func TestNodeKindRoundTrip(t *testing.T) {
	kinds := []mnnode.NodeKind{
		mnnode.NODE_KIND_UNSPECIFIED,
		mnnode.NODE_KIND_NO_OP,
		mnnode.NODE_KIND_REQUEST,
		mnnode.NODE_KIND_CONDITION,
		mnnode.NODE_KIND_FOR,
		mnnode.NODE_KIND_FOR_EACH,
		mnnode.NODE_KIND_JS,
	}

	for _, kind := range kinds {
		protoKind, err := nodeKindModelToProto(kind)
		if err != nil {
			t.Fatalf("nodeKindModelToProto(%d) returned error: %v", kind, err)
		}

		modelKind, err := nodeKindProtoToModel(protoKind)
		if err != nil {
			t.Fatalf("nodeKindProtoToModel(%v) returned error: %v", protoKind, err)
		}

		if modelKind != kind {
			t.Fatalf("node kind round trip mismatch: got %d want %d", modelKind, kind)
		}
	}
}

func TestNodeStateModelToProtoFallback(t *testing.T) {
	value, err := nodeStateModelToProto(99)
	if err == nil {
		t.Fatal("expected error for unknown node state")
	}

	if value != nodeStateProtoFallback {
		t.Fatalf("expected fallback state %v got %v", nodeStateProtoFallback, value)
	}
}

func TestNodeNoOpRoundTrip(t *testing.T) {
	kinds := []mnnoop.NoopTypes{
		mnnoop.NODE_NO_OP_KIND_UNSPECIFIED,
		mnnoop.NODE_NO_OP_KIND_START,
		mnnoop.NODE_NO_OP_KIND_CREATE,
		mnnoop.NODE_NO_OP_KIND_THEN,
		mnnoop.NODE_NO_OP_KIND_ELSE,
		mnnoop.NODE_NO_OP_KIND_LOOP,
	}

	for _, kind := range kinds {
		protoKind, err := nodeNoOpModelToProto(kind)
		if err != nil {
			t.Fatalf("nodeNoOpModelToProto(%d) returned error: %v", kind, err)
		}

		modelKind, err := nodeNoOpProtoToModel(protoKind)
		if err != nil {
			t.Fatalf("nodeNoOpProtoToModel(%v) returned error: %v", protoKind, err)
		}

		if modelKind != kind {
			t.Fatalf("noop kind round trip mismatch: got %d want %d", modelKind, kind)
		}
	}
}

func TestLoopErrorHandlingRoundTrip(t *testing.T) {
	handlers := []mnfor.ErrorHandling{
		mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED,
		mnfor.ErrorHandling_ERROR_HANDLING_IGNORE,
		mnfor.ErrorHandling_ERROR_HANDLING_BREAK,
	}

	for _, handler := range handlers {
		protoHandler, err := loopErrorHandlingModelToProto(handler)
		if err != nil {
			t.Fatalf("loopErrorHandlingModelToProto(%d) returned error: %v", handler, err)
		}

		modelHandler, err := loopErrorHandlingProtoToModel(protoHandler)
		if err != nil {
			t.Fatalf("loopErrorHandlingProtoToModel(%v) returned error: %v", protoHandler, err)
		}

		if modelHandler != handler {
			t.Fatalf("loop error handling round trip mismatch: got %d want %d", modelHandler, handler)
		}
	}
}

func TestConvertRPCNodeToModelWithoutIDInvalidKind(t *testing.T) {
	rpc := &nodev1.Node{Kind: nodev1.NodeKind(99)}
	flowID := idwrap.NewNow()
	nodeID := idwrap.NewNow()

	_, err := ConvertRPCNodeToModelWithoutID(context.Background(), rpc, flowID, nodeID)
	if err == nil {
		t.Fatal("expected error for invalid node kind")
	}

	if !strings.Contains(err.Error(), "invalid node kind") {
		t.Fatalf("expected invalid node kind error, got %v", err)
	}
}

func TestConvertRPCNodeToModelWithoutIDMissingNoOpKind(t *testing.T) {
	rpc := &nodev1.Node{Kind: nodev1.NodeKind_NODE_KIND_NO_OP}
	flowID := idwrap.NewNow()
	nodeID := idwrap.NewNow()

	_, err := ConvertRPCNodeToModelWithoutID(context.Background(), rpc, flowID, nodeID)
	if err == nil {
		t.Fatal("expected error for missing noop kind")
	}

	if !strings.Contains(err.Error(), "no-op kind") {
		t.Fatalf("expected no-op kind error, got %v", err)
	}
}

func TestGetNodeSubUnknownKindReturnsConnectError(t *testing.T) {
	node := mnnode.MNode{
		ID:       idwrap.NewNow(),
		NodeKind: mnnode.NodeKind(99),
	}

	_, err := GetNodeSub(
		context.Background(),
		node,
		snode.NodeService{},
		snodeif.NodeIfService{},
		snoderequest.NodeRequestService{},
		snodefor.NodeForService{},
		snodeforeach.NodeForEachService{},
		snodenoop.NodeNoopService{},
		snodejs.NodeJSService{},
		snodeexecution.NodeExecutionService{},
	)
	if err == nil {
		t.Fatal("expected connect error for unknown node kind")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect error, got %T", err)
	}

	if connectErr.Code() != connect.CodeInternal {
		t.Fatalf("expected internal error code, got %v", connectErr.Code())
	}
}
