package rflow

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	nodev1 "the-dev-tools/spec/dist/buf/go/flow/node/v1"
	flowv1 "the-dev-tools/spec/dist/buf/go/flow/v1"
)

type recordingStream struct {
	messages []*flowv1.FlowRunResponse
	sendErr  error
}

func (r *recordingStream) Send(resp *flowv1.FlowRunResponse) error {
	if r.sendErr != nil {
		return r.sendErr
	}
	r.messages = append(r.messages, resp)
	return nil
}

func TestNodeStateModelToProtoKnownValues(t *testing.T) {
	cases := map[mnnode.NodeState]nodev1.NodeState{
		mnnode.NODE_STATE_UNSPECIFIED: nodev1.NodeState_NODE_STATE_UNSPECIFIED,
		mnnode.NODE_STATE_RUNNING:     nodev1.NodeState_NODE_STATE_RUNNING,
		mnnode.NODE_STATE_SUCCESS:     nodev1.NodeState_NODE_STATE_SUCCESS,
		mnnode.NODE_STATE_FAILURE:     nodev1.NodeState_NODE_STATE_FAILURE,
		mnnode.NODE_STATE_CANCELED:    nodev1.NodeState_NODE_STATE_CANCELED,
	}

	for model, expected := range cases {
		t.Run(expected.String(), func(t *testing.T) {
			got, err := nodeStateModelToProto(model)
			require.NoError(t, err)
			require.Equal(t, expected, got)
		})
	}
}

func TestNodeStateModelToProtoFallback(t *testing.T) {
	got, err := nodeStateModelToProto(mnnode.NodeState(99))
	require.Error(t, err)
	require.Equal(t, nodeStateProtoFallback, got)
}

func TestNodeStateIntToProto(t *testing.T) {
	got, err := nodeStateIntToProto(int8(mnnode.NODE_STATE_FAILURE))
	require.NoError(t, err)
	require.Equal(t, nodev1.NodeState_NODE_STATE_FAILURE, got)
}

func TestNodeStateProtoToModelKnownValues(t *testing.T) {
	cases := map[nodev1.NodeState]mnnode.NodeState{
		nodev1.NodeState_NODE_STATE_UNSPECIFIED: mnnode.NODE_STATE_UNSPECIFIED,
		nodev1.NodeState_NODE_STATE_RUNNING:     mnnode.NODE_STATE_RUNNING,
		nodev1.NodeState_NODE_STATE_SUCCESS:     mnnode.NODE_STATE_SUCCESS,
		nodev1.NodeState_NODE_STATE_FAILURE:     mnnode.NODE_STATE_FAILURE,
		nodev1.NodeState_NODE_STATE_CANCELED:    mnnode.NODE_STATE_CANCELED,
	}

	for proto, expected := range cases {
		t.Run(proto.String(), func(t *testing.T) {
			got, err := nodeStateProtoToModel(proto)
			require.NoError(t, err)
			require.Equal(t, expected, got)
		})
	}
}

func TestNodeStateProtoToModelUnknown(t *testing.T) {
	got, err := nodeStateProtoToModel(nodev1.NodeState(255))
	require.Error(t, err)
	require.Equal(t, mnnode.NODE_STATE_UNSPECIFIED, got)
}

func TestSendNodeStatusForwardsValues(t *testing.T) {
	stream := &recordingStream{}
	nodeID := idwrap.NewNow()
	info := "details"
	state := nodev1.NodeState_NODE_STATE_SUCCESS

	require.NoError(t, sendNodeStatus(stream, nodeID, state, &info))
	require.Len(t, stream.messages, 1)

	resp := stream.messages[0]
	require.NotNil(t, resp.Node)
	require.Equal(t, nodeID.Bytes(), resp.Node.NodeId)
	require.Equal(t, state, resp.Node.State)
	require.Equal(t, info, resp.Node.GetInfo())
}

func TestSendNodeStatusPropagatesErrors(t *testing.T) {
	expectedErr := errors.New("send failure")
	stream := &recordingStream{sendErr: expectedErr}

	err := sendNodeStatus(stream, idwrap.NewNow(), nodev1.NodeState_NODE_STATE_FAILURE, nil)
	require.ErrorIs(t, err, expectedErr)
}
