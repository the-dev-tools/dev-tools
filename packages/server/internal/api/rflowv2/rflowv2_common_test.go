package rflowv2

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mnodeexecution"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

func TestSerializeNodeHTTP(t *testing.T) {
	nodeID := idwrap.NewNow()
	httpID := idwrap.NewNow()
	deltaHttpID := idwrap.NewNow()

	tests := []struct {
		name     string
		input    mnrequest.MNRequest
		expected *flowv1.NodeHttp
	}{
		{
			name: "With HTTP ID and Delta ID",
			input: mnrequest.MNRequest{
				FlowNodeID:  nodeID,
				HttpID:      &httpID,
				DeltaHttpID: &deltaHttpID,
			},
			expected: &flowv1.NodeHttp{
				NodeId:      nodeID.Bytes(),
				HttpId:      httpID.Bytes(),
				DeltaHttpId: deltaHttpID.Bytes(),
			},
		},
		{
			name: "Without HTTP ID",
			input: mnrequest.MNRequest{
				FlowNodeID: nodeID,
				HttpID:     nil,
			},
			expected: &flowv1.NodeHttp{
				NodeId: nodeID.Bytes(),
			},
		},
		{
			name: "With HTTP ID but no Delta ID",
			input: mnrequest.MNRequest{
				FlowNodeID:  nodeID,
				HttpID:      &httpID,
				DeltaHttpID: nil,
			},
			expected: &flowv1.NodeHttp{
				NodeId: nodeID.Bytes(),
				HttpId: httpID.Bytes(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := serializeNodeHTTP(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSerializeNode(t *testing.T) {
	nodeID := idwrap.NewNow()
	flowID := idwrap.NewNow()

	tests := []struct {
		name     string
		input    mnnode.MNode
		expected *flowv1.Node
	}{
		{
			name: "Basic Node",
			input: mnnode.MNode{
				ID:        nodeID,
				FlowID:    flowID,
				Name:      "Test Node",
				NodeKind:  mnnode.NODE_KIND_REQUEST,
				PositionX: 100.5,
				PositionY: 200.5,
			},
			expected: &flowv1.Node{
				NodeId:   nodeID.Bytes(),
				FlowId:   flowID.Bytes(),
				Kind:     flowv1.NodeKind_NODE_KIND_HTTP,
				Name:     "Test Node",
				Position: &flowv1.Position{X: 100.5, Y: 200.5},
				State:    flowv1.FlowItemState_FLOW_ITEM_STATE_UNSPECIFIED,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := serializeNode(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSerializeEdge(t *testing.T) {
	edgeID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	sourceID := idwrap.NewNow()
	targetID := idwrap.NewNow()

	tests := []struct {
		name     string
		input    edge.Edge
		expected *flowv1.Edge
	}{
		{
			name: "NoOp Edge",
			input: edge.Edge{
				ID:            edgeID,
				FlowID:        flowID,
				Kind:          int32(edge.EdgeKindNoOp),
				SourceID:      sourceID,
				TargetID:      targetID,
				SourceHandler: edge.HandleThen,
			},
			expected: &flowv1.Edge{
				EdgeId:       edgeID.Bytes(),
				FlowId:       flowID.Bytes(),
				Kind:         flowv1.EdgeKind(edge.EdgeKindNoOp),
				SourceId:     sourceID.Bytes(),
				TargetId:     targetID.Bytes(),
				SourceHandle: flowv1.HandleKind(edge.HandleThen),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := serializeEdge(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSerializeFlow(t *testing.T) {
	flowID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()

	tests := []struct {
		name     string
		input    mflow.Flow
		expected *flowv1.Flow
	}{
		{
			name: "Basic Flow",
			input: mflow.Flow{
				ID:          flowID,
				WorkspaceID: workspaceID,
				Name:        "Test Flow",
				Running:     true,
			},
			expected: &flowv1.Flow{
				FlowId:      flowID.Bytes(),
				WorkspaceId: workspaceID.Bytes(),
				Name:        "Test Flow",
				Running:     true,
			},
		},
		{
			name: "Flow with Duration",
			input: mflow.Flow{
				ID:          flowID,
				WorkspaceID: workspaceID,
				Name:        "Test Flow",
				Running:     false,
				Duration:    1234,
			},
			expected: &flowv1.Flow{
				FlowId:      flowID.Bytes(),
				WorkspaceId: workspaceID.Bytes(),
				Name:        "Test Flow",
				Running:     false,
				Duration:    ptr(int32(1234)),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := serializeFlow(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func ptr[T any](v T) *T {
	return &v
}

func TestSerializeNodeNoop(t *testing.T) {
	nodeID := idwrap.NewNow()
	tests := []struct {
		name     string
		input    mnnoop.NoopNode
		expected *flowv1.NodeNoOp
	}{
		{
			name: "NoOp Start",
			input: mnnoop.NoopNode{
				FlowNodeID: nodeID,
				Type:       mnnoop.NODE_NO_OP_KIND_START,
			},
			expected: &flowv1.NodeNoOp{
				NodeId: nodeID.Bytes(),
				Kind:   flowv1.NodeNoOpKind_NODE_NO_OP_KIND_START,
			},
		},
		{
			name: "NoOp Loop",
			input: mnnoop.NoopNode{
				FlowNodeID: nodeID,
				Type:       mnnoop.NODE_NO_OP_KIND_LOOP,
			},
			expected: &flowv1.NodeNoOp{
				NodeId: nodeID.Bytes(),
				Kind:   flowv1.NodeNoOpKind_NODE_NO_OP_KIND_LOOP,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := serializeNodeNoop(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSerializeNodeFor(t *testing.T) {
	nodeID := idwrap.NewNow()
	tests := []struct {
		name     string
		input    mnfor.MNFor
		expected *flowv1.NodeFor
	}{
		{
			name: "For Node",
			input: mnfor.MNFor{
				FlowNodeID:    nodeID,
				IterCount:     10,
				ErrorHandling: mnfor.ErrorHandling_ERROR_HANDLING_IGNORE,
				Condition: mcondition.Condition{
					Comparisons: mcondition.Comparison{
						Expression: "true",
					},
				},
			},
			expected: &flowv1.NodeFor{
				NodeId:        nodeID.Bytes(),
				Iterations:    10,
				Condition:     "true",
				ErrorHandling: flowv1.ErrorHandling_ERROR_HANDLING_IGNORE,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := serializeNodeFor(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSerializeNodeCondition(t *testing.T) {
	nodeID := idwrap.NewNow()
	tests := []struct {
		name     string
		input    mnif.MNIF
		expected *flowv1.NodeCondition
	}{
		{
			name: "Condition Node",
			input: mnif.MNIF{
				FlowNodeID: nodeID,
				Condition: mcondition.Condition{
					Comparisons: mcondition.Comparison{
						Expression: "a > b",
					},
				},
			},
			expected: &flowv1.NodeCondition{
				NodeId:    nodeID.Bytes(),
				Condition: "a > b",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := serializeNodeCondition(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSerializeNodeForEach(t *testing.T) {
	nodeID := idwrap.NewNow()
	tests := []struct {
		name     string
		input    mnforeach.MNForEach
		expected *flowv1.NodeForEach
	}{
		{
			name: "ForEach Node",
			input: mnforeach.MNForEach{
				FlowNodeID:     nodeID,
				IterExpression: "items",
				ErrorHandling:  mnfor.ErrorHandling_ERROR_HANDLING_BREAK,
				Condition: mcondition.Condition{
					Comparisons: mcondition.Comparison{
						Expression: "item.active",
					},
				},
			},
			expected: &flowv1.NodeForEach{
				NodeId:        nodeID.Bytes(),
				Path:          "items",
				Condition:     "item.active",
				ErrorHandling: flowv1.ErrorHandling_ERROR_HANDLING_BREAK,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := serializeNodeForEach(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSerializeNodeJs(t *testing.T) {
	nodeID := idwrap.NewNow()
	tests := []struct {
		name     string
		input    mnjs.MNJS
		expected *flowv1.NodeJs
	}{
		{
			name: "JS Node",
			input: mnjs.MNJS{
				FlowNodeID: nodeID,
				Code:       []byte("console.log('hello')"),
			},
			expected: &flowv1.NodeJs{
				NodeId: nodeID.Bytes(),
				Code:   "console.log('hello')",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := serializeNodeJs(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSerializeNodeExecution(t *testing.T) {
	executionID := idwrap.NewNow()
	nodeID := idwrap.NewNow()
	httpID := idwrap.NewNow()
	completedAt := time.Now().Unix()

	t.Run("Basic Execution", func(t *testing.T) {
		input := mnodeexecution.NodeExecution{
			ID:          executionID,
			NodeID:      nodeID,
			Name:        "Test Exec",
			State:       mnnode.NODE_STATE_SUCCESS,
			CompletedAt: &completedAt,
			ResponseID:  &httpID,
		}

		res := serializeNodeExecution(input)

		assert.Equal(t, executionID.Bytes(), res.NodeExecutionId)
		assert.Equal(t, nodeID.Bytes(), res.NodeId)
		assert.Equal(t, "Test Exec", res.Name)
		assert.Equal(t, flowv1.FlowItemState(mnnode.NODE_STATE_SUCCESS), res.State)
		assert.Equal(t, httpID.Bytes(), res.HttpResponseId)
		assert.Equal(t, completedAt, res.CompletedAt.Seconds)
	})

	t.Run("With Error", func(t *testing.T) {
		input := mnodeexecution.NodeExecution{
			ID:     executionID,
			NodeID: nodeID,
			Name:   "Error Exec",
			State:  mnnode.NODE_STATE_FAILURE,
			Error:  ptr("Something went wrong"),
		}

		res := serializeNodeExecution(input)

		assert.Equal(t, "Something went wrong", *res.Error)
		assert.Equal(t, flowv1.FlowItemState(mnnode.NODE_STATE_FAILURE), res.State)
	})

	t.Run("With Input/Output JSON", func(t *testing.T) {
		input := mnodeexecution.NodeExecution{
			ID:     executionID,
			NodeID: nodeID,
			Name:   "Data Exec",
			State:  mnnode.NODE_STATE_SUCCESS,
		}

		err := input.SetInputJSON(json.RawMessage(`{"foo":"bar"}`))
		require.NoError(t, err)

		err = input.SetOutputJSON(json.RawMessage(`{"baz":"qux"}`))
		require.NoError(t, err)

		res := serializeNodeExecution(input)

		// Note: structpb.NewValue(string(json)) creates a StringValue, not a StructValue.
		require.NotNil(t, res.Input)
		// Based on my analysis, the code puts the raw string into the value
		assert.Equal(t, string(`{"foo":"bar"}`), res.Input.GetStringValue())

		require.NotNil(t, res.Output)
		assert.Equal(t, string(`{"baz":"qux"}`), res.Output.GetStringValue())
	})
}

func TestSerializeFlowVariable(t *testing.T) {
	variableID := idwrap.NewNow()
	flowID := idwrap.NewNow()

	tests := []struct {
		name     string
		input    mflowvariable.FlowVariable
		expected *flowv1.FlowVariable
	}{
		{
			name: "Flow Variable",
			input: mflowvariable.FlowVariable{
				ID:          variableID,
				FlowID:      flowID,
				Name:        "var1",
				Value:       "val1",
				Enabled:     true,
				Description: "desc",
				Order:       1,
			},
			expected: &flowv1.FlowVariable{
				FlowVariableId: variableID.Bytes(),
				FlowId:         flowID.Bytes(),
				Key:            "var1",
				Value:          "val1",
				Enabled:        true,
				Description:    "desc",
				Order:          1.0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := serializeFlowVariable(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsStartNode(t *testing.T) {
	tests := []struct {
		name     string
		input    mnnode.MNode
		expected bool
	}{
		{
			name: "Start Node",
			input: mnnode.MNode{
				NodeKind: mnnode.NODE_KIND_NO_OP,
				Name:     "Start",
			},
			expected: true,
		},
		{
			name: "Start Node Lowercase",
			input: mnnode.MNode{
				NodeKind: mnnode.NODE_KIND_NO_OP,
				Name:     "start",
			},
			expected: true,
		},
		{
			name: "Not Start Node",
			input: mnnode.MNode{
				NodeKind: mnnode.NODE_KIND_NO_OP,
				Name:     "End",
			},
			expected: false,
		},
		{
			name: "Not NoOp Node",
			input: mnnode.MNode{
				NodeKind: mnnode.NODE_KIND_REQUEST,
				Name:     "Start",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isStartNode(tt.input))
		})
	}
}

func TestDeserializeNodeInsert(t *testing.T) {
	// Wrapper to call the private method
	s := &FlowServiceV2RPC{}

	t.Run("Valid Insert", func(t *testing.T) {
		flowID := idwrap.NewNow()
		nodeID := idwrap.NewNow()

		input := &flowv1.NodeInsert{
			FlowId: flowID.Bytes(),
			NodeId: nodeID.Bytes(),
			Name:   "New Node",
			Kind:   flowv1.NodeKind_NODE_KIND_HTTP,
			Position: &flowv1.Position{
				X: 10,
				Y: 20,
			},
		}

		result, err := s.deserializeNodeInsert(input)
		require.NoError(t, err)
		assert.Equal(t, flowID, result.FlowID)
		assert.Equal(t, nodeID, result.ID)
		assert.Equal(t, "New Node", result.Name)
		assert.Equal(t, mnnode.NODE_KIND_REQUEST, result.NodeKind)
		assert.Equal(t, 10.0, result.PositionX)
		assert.Equal(t, 20.0, result.PositionY)
	})

	t.Run("Valid Insert Generated ID", func(t *testing.T) {
		flowID := idwrap.NewNow()

		input := &flowv1.NodeInsert{
			FlowId: flowID.Bytes(),
			Name:   "New Node",
			Kind:   flowv1.NodeKind_NODE_KIND_HTTP,
		}

		result, err := s.deserializeNodeInsert(input)
		require.NoError(t, err)
		assert.False(t, isZeroID(result.ID))
	})

	t.Run("Nil Item", func(t *testing.T) {
		_, err := s.deserializeNodeInsert(nil)
		assert.Error(t, err)
	})

	t.Run("Missing Flow ID", func(t *testing.T) {
		input := &flowv1.NodeInsert{
			Name: "New Node",
		}
		_, err := s.deserializeNodeInsert(input)
		assert.Error(t, err)
	})

	t.Run("Invalid Flow ID", func(t *testing.T) {
		input := &flowv1.NodeInsert{
			FlowId: []byte("invalid"),
		}
		_, err := s.deserializeNodeInsert(input)
		assert.Error(t, err)
	})

	t.Run("Invalid Node ID", func(t *testing.T) {
		flowID := idwrap.NewNow()
		input := &flowv1.NodeInsert{
			FlowId: flowID.Bytes(),
			NodeId: []byte("invalid"),
		}
		_, err := s.deserializeNodeInsert(input)
		assert.Error(t, err)
	})
}
