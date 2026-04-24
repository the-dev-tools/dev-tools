package rreference

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// ---------------------------------------------------------------------------
// parseReferenceContext
// ---------------------------------------------------------------------------

func TestParseReferenceContext(t *testing.T) {
	validID := idwrap.NewNow()
	validBytes := validID.Bytes()

	tests := []struct {
		name      string
		msg       referenceContextMsg
		wantErr   bool
		checkFunc func(t *testing.T, p resolveParams)
	}{
		{
			name: "all nil",
			msg:  referenceContextMsg{},
			checkFunc: func(t *testing.T, p resolveParams) {
				assert.Nil(t, p.workspaceID)
				assert.Nil(t, p.httpID)
				assert.Nil(t, p.graphqlID)
				assert.Nil(t, p.flowNodeID)
			},
		},
		{
			name: "valid workspace ID",
			msg:  referenceContextMsg{WorkspaceID: validBytes},
			checkFunc: func(t *testing.T, p resolveParams) {
				require.NotNil(t, p.workspaceID)
				assert.Equal(t, validID, *p.workspaceID)
				assert.Nil(t, p.httpID)
			},
		},
		{
			name: "mixed valid and nil",
			msg: referenceContextMsg{
				WorkspaceID: validBytes,
				FlowNodeID:  validBytes,
			},
			checkFunc: func(t *testing.T, p resolveParams) {
				require.NotNil(t, p.workspaceID)
				assert.Nil(t, p.httpID)
				assert.Nil(t, p.graphqlID)
				require.NotNil(t, p.flowNodeID)
			},
		},
		{
			name:    "invalid bytes",
			msg:     referenceContextMsg{WorkspaceID: []byte("bad")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := parseReferenceContext(tt.msg)
			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
				return
			}
			require.NoError(t, err)
			if tt.checkFunc != nil {
				tt.checkFunc(t, p)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// addExecutionDataToVarMap
// ---------------------------------------------------------------------------

func TestAddExecutionDataToVarMap(t *testing.T) {
	tests := []struct {
		name     string
		data     any
		nodeName string
		want     any // expected value at varMap[nodeName]
	}{
		{
			name:     "extracts node subtree",
			data:     map[string]any{"MyNode": map[string]any{"foo": "bar"}},
			nodeName: "MyNode",
			want:     map[string]any{"foo": "bar"},
		},
		{
			name:     "missing node key uses data directly",
			data:     map[string]any{"Other": "val"},
			nodeName: "MyNode",
			want:     map[string]any{"Other": "val"},
		},
		{
			name:     "non-map data uses as-is",
			data:     "raw-string",
			nodeName: "MyNode",
			want:     "raw-string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			varMap := make(map[string]any)
			addExecutionDataToVarMap(tt.data, tt.nodeName, varMap)
			assert.Equal(t, tt.want, varMap[tt.nodeName])
		})
	}
}

// ---------------------------------------------------------------------------
// addExecutionDataToVarMapFlat
// ---------------------------------------------------------------------------

func TestAddExecutionDataToVarMapFlat(t *testing.T) {
	tests := []struct {
		name     string
		data     any
		nodeName string
		wantKeys []string // keys expected at root level
	}{
		{
			name: "extracts sub-keys to root",
			data: map[string]any{
				"MyNode": map[string]any{"response": "ok", "status": 200},
			},
			nodeName: "MyNode",
			wantKeys: []string{"response", "status"},
		},
		{
			name:     "missing node key is no-op",
			data:     map[string]any{"Other": "val"},
			nodeName: "MyNode",
			wantKeys: nil,
		},
		{
			name: "non-map inner value is no-op",
			data: map[string]any{
				"MyNode": "not-a-map",
			},
			nodeName: "MyNode",
			wantKeys: nil,
		},
		{
			name:     "non-map data is no-op",
			data:     42,
			nodeName: "MyNode",
			wantKeys: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			varMap := make(map[string]any)
			addExecutionDataToVarMapFlat(tt.data, tt.nodeName, varMap)
			for _, k := range tt.wantKeys {
				assert.Contains(t, varMap, k)
			}
			if tt.wantKeys == nil {
				assert.Empty(t, varMap)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// nodeDefaultSchema
// ---------------------------------------------------------------------------

func TestNodeDefaultSchema(t *testing.T) {
	tests := []struct {
		name   string
		kind   mflow.NodeKind
		wantOK bool
	}{
		{"FOR", mflow.NODE_KIND_FOR, true},
		{"FOR_EACH", mflow.NODE_KIND_FOR_EACH, true},
		{"REQUEST", mflow.NODE_KIND_REQUEST, true},
		{"GRAPHQL", mflow.NODE_KIND_GRAPHQL, true},
		{"JS", mflow.NODE_KIND_JS, true},
		{"CONDITION", mflow.NODE_KIND_CONDITION, true},
		{"AI", mflow.NODE_KIND_AI, true},
		{"AI_PROVIDER", mflow.NODE_KIND_AI_PROVIDER, true},
		{"WS_CONNECTION", mflow.NODE_KIND_WS_CONNECTION, true},
		{"WS_SEND", mflow.NODE_KIND_WS_SEND, true},
		{"RUN_SUB_FLOW", mflow.NODE_KIND_RUN_SUB_FLOW, true},
		{"SUB_FLOW_RETURN has no schema", mflow.NODE_KIND_SUB_FLOW_RETURN, false},
		{"SUB_FLOW_TRIGGER handled separately", mflow.NODE_KIND_SUB_FLOW_TRIGGER, false},
		{"unknown kind", mflow.NodeKind(9999), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, ok := nodeDefaultSchema(tt.kind)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.NotNil(t, schema)
			}
		})
	}
}
