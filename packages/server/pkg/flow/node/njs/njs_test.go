package njs

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	node_js_executorv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/node_js_executor/v1"
)

// mockNodeJsClient is a mock implementation of NodeJsExecutorServiceClient for testing
type mockNodeJsClient struct {
	response *connect.Response[node_js_executorv1.NodeJsExecutorRunResponse]
	err      error
}

func (m *mockNodeJsClient) NodeJsExecutorRun(
	ctx context.Context,
	req *connect.Request[node_js_executorv1.NodeJsExecutorRunRequest],
) (*connect.Response[node_js_executorv1.NodeJsExecutorRunResponse], error) {
	return m.response, m.err
}

func TestNodeJS_ConnectErrorMessageExtraction(t *testing.T) {
	tests := []struct {
		name            string
		connectErr      *connect.Error
		expectedContain string
		notContain      string
	}{
		{
			name:            "internal error extracts message without prefix",
			connectErr:      connect.NewError(connect.CodeInternal, errors.New("ReferenceError: x is not defined")),
			expectedContain: "ReferenceError: x is not defined",
			notContain:      "internal:",
		},
		{
			name:            "unknown error extracts message without prefix",
			connectErr:      connect.NewError(connect.CodeUnknown, errors.New("TypeError: cannot read property")),
			expectedContain: "TypeError: cannot read property",
			notContain:      "unknown:",
		},
		{
			name:            "invalid argument error extracts message without prefix",
			connectErr:      connect.NewError(connect.CodeInvalidArgument, errors.New("Default export must be present")),
			expectedContain: "Default export must be present",
			notContain:      "invalid_argument:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeID := idwrap.NewNow()
			mockClient := &mockNodeJsClient{
				err: tt.connectErr,
			}

			jsNode := New(nodeID, "TestNode", "const x = 1;", mockClient)

			req := &node.FlowNodeRequest{
				EdgeSourceMap: mflow.EdgesMap{},
				VarMap:        map[string]any{},
			}

			result := jsNode.RunSync(context.Background(), req)

			require.Error(t, result.Err)
			require.Contains(t, result.Err.Error(), tt.expectedContain,
				"error should contain the actual error message")
			require.NotContains(t, result.Err.Error(), tt.notContain,
				"error should not contain the error code prefix")
		})
	}
}

func TestNodeJS_NilClientReturnsError(t *testing.T) {
	nodeID := idwrap.NewNow()
	jsNode := New(nodeID, "TestNode", "const x = 1;", nil)

	req := &node.FlowNodeRequest{
		EdgeSourceMap: mflow.EdgesMap{},
		VarMap:        map[string]any{},
	}

	result := jsNode.RunSync(context.Background(), req)

	require.Error(t, result.Err)
	require.Contains(t, result.Err.Error(), "JS executor not available")
}

func TestNodeJS_SuccessfulExecution(t *testing.T) {
	nodeID := idwrap.NewNow()
	mockClient := &mockNodeJsClient{
		response: connect.NewResponse(&node_js_executorv1.NodeJsExecutorRunResponse{}),
		err:      nil,
	}

	jsNode := New(nodeID, "TestNode", "export default 42;", mockClient)

	req := &node.FlowNodeRequest{
		EdgeSourceMap: mflow.EdgesMap{},
		VarMap:        map[string]any{},
	}

	result := jsNode.RunSync(context.Background(), req)

	require.NoError(t, result.Err)
}
