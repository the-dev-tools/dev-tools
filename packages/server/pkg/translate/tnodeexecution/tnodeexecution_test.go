package tnodeexecution

import (
	"testing"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnodeexecution"
	nodev1 "the-dev-tools/spec/dist/buf/go/flow/node/v1"
)

func TestModelNodeStateToProtoSuccess(t *testing.T) {
	testCases := []struct {
		name  string
		state mnnode.NodeState
		want  nodev1.NodeState
	}{
		{name: "unspecified", state: mnnode.NODE_STATE_UNSPECIFIED, want: nodev1.NodeState_NODE_STATE_UNSPECIFIED},
		{name: "running", state: mnnode.NODE_STATE_RUNNING, want: nodev1.NodeState_NODE_STATE_RUNNING},
		{name: "success", state: mnnode.NODE_STATE_SUCCESS, want: nodev1.NodeState_NODE_STATE_SUCCESS},
		{name: "failure", state: mnnode.NODE_STATE_FAILURE, want: nodev1.NodeState_NODE_STATE_FAILURE},
		{name: "canceled", state: mnnode.NODE_STATE_CANCELED, want: nodev1.NodeState_NODE_STATE_CANCELED},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := modelNodeStateToProto(tc.state)
			if err != nil {
				t.Fatalf("modelNodeStateToProto(%v) returned error: %v", tc.state, err)
			}
			if got != tc.want {
				t.Fatalf("modelNodeStateToProto(%v) = %v, want %v", tc.state, got, tc.want)
			}
		})
	}
}

func TestModelNodeExecutionStateToProtoInvalid(t *testing.T) {
	state := int8(99)
	got, err := modelNodeExecutionStateToProto(state)
	if err == nil {
		t.Fatalf("expected error for invalid state %d", state)
	}
	if got != fallbackNodeState() {
		t.Fatalf("modelNodeExecutionStateToProto(%d) = %v, want fallback %v", state, got, fallbackNodeState())
	}
}

func TestSerializeNodeExecutionStateHandling(t *testing.T) {
	type (
		serializedStateGetter func(*mnodeexecution.NodeExecution) (nodev1.NodeState, error)
	)

	serializers := []struct {
		name string
		get  serializedStateGetter
	}{
		{
			name: "NodeExecution",
			get: func(ne *mnodeexecution.NodeExecution) (nodev1.NodeState, error) {
				res, err := SerializeNodeExecutionModelToRPC(ne)
				if res == nil {
					return fallbackNodeState(), err
				}
				return res.GetState(), err
			},
		},
		{
			name: "NodeExecutionListItem",
			get: func(ne *mnodeexecution.NodeExecution) (nodev1.NodeState, error) {
				res, err := SerializeNodeExecutionModelToRPCListItem(ne)
				if res == nil {
					return fallbackNodeState(), err
				}
				return res.GetState(), err
			},
		},
		{
			name: "NodeExecutionGetResponse",
			get: func(ne *mnodeexecution.NodeExecution) (nodev1.NodeState, error) {
				res, err := SerializeNodeExecutionModelToRPCGetResponse(ne)
				if res == nil {
					return fallbackNodeState(), err
				}
				return res.GetState(), err
			},
		},
	}

	t.Run("valid", func(t *testing.T) {
		ne := newTestNodeExecution(int8(mnnode.NODE_STATE_SUCCESS))

		for _, serializer := range serializers {
			t.Run(serializer.name, func(t *testing.T) {
				got, err := serializer.get(ne)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != nodev1.NodeState_NODE_STATE_SUCCESS {
					t.Fatalf("state = %v, want %v", got, nodev1.NodeState_NODE_STATE_SUCCESS)
				}
			})
		}
	})

	t.Run("invalid", func(t *testing.T) {
		ne := newTestNodeExecution(int8(123))

		for _, serializer := range serializers {
			t.Run(serializer.name, func(t *testing.T) {
				got, err := serializer.get(ne)
				if err == nil {
					t.Fatalf("expected error for invalid state")
				}
				if got != fallbackNodeState() {
					t.Fatalf("state = %v, want fallback %v", got, fallbackNodeState())
				}
			})
		}
	})
}

func newTestNodeExecution(state int8) *mnodeexecution.NodeExecution {
	return &mnodeexecution.NodeExecution{
		ID:     idwrap.NewNow(),
		NodeID: idwrap.NewNow(),
		Name:   "example",
		State:  state,
	}
}
