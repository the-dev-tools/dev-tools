package rflowv2

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream/memory"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcondition"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

// initCopyPasteTestContext sets up the event streams and HTTP service needed for paste tests.
func initCopyPasteTestContext(tc *RFlowTestContext) {
	tc.Svc.nodeStream = memory.NewInMemorySyncStreamer[NodeTopic, NodeEvent]()
	tc.Svc.edgeStream = memory.NewInMemorySyncStreamer[EdgeTopic, EdgeEvent]()
	tc.Svc.httpStream = memory.NewInMemorySyncStreamer[rhttp.HttpTopic, rhttp.HttpEvent]()
	hs := shttp.New(tc.Queries, tc.Svc.logger)
	tc.Svc.hs = &hs
}

func TestRemapVarRefs(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		nameMapping map[string]string
		expected    string
	}{
		{
			name:        "empty mapping",
			input:       "{{ GetUsers.response.body }}",
			nameMapping: map[string]string{},
			expected:    "{{ GetUsers.response.body }}",
		},
		{
			name:        "empty string",
			input:       "",
			nameMapping: map[string]string{"GetUsers": "GetUsers_1"},
			expected:    "",
		},
		{
			name:        "no variables",
			input:       "plain text without vars",
			nameMapping: map[string]string{"GetUsers": "GetUsers_1"},
			expected:    "plain text without vars",
		},
		{
			name:        "simple remap with path",
			input:       "{{ GetUsers.response.body }}",
			nameMapping: map[string]string{"GetUsers": "GetUsers_1"},
			expected:    "{{ GetUsers_1.response.body }}",
		},
		{
			name:        "remap with surrounding text",
			input:       "prefix {{ GetUsers.response.status }} suffix",
			nameMapping: map[string]string{"GetUsers": "GetUsers_1"},
			expected:    "prefix {{ GetUsers_1.response.status }} suffix",
		},
		{
			name:        "multiple variables same name",
			input:       "{{ GetUsers.response.body.id }} and {{ GetUsers.response.body.name }}",
			nameMapping: map[string]string{"GetUsers": "GetUsers_1"},
			expected:    "{{ GetUsers_1.response.body.id }} and {{ GetUsers_1.response.body.name }}",
		},
		{
			name:        "multiple different names",
			input:       "{{ GetUsers.response.body }} + {{ CreateUser.response.body }}",
			nameMapping: map[string]string{"GetUsers": "GetUsers_1", "CreateUser": "CreateUser_1"},
			expected:    "{{ GetUsers_1.response.body }} + {{ CreateUser_1.response.body }}",
		},
		{
			name:        "no match - different name",
			input:       "{{ OtherNode.response.body }}",
			nameMapping: map[string]string{"GetUsers": "GetUsers_1"},
			expected:    "{{ OtherNode.response.body }}",
		},
		{
			name:        "partial name match should not remap",
			input:       "{{ GetUsersAll.response.body }}",
			nameMapping: map[string]string{"GetUsers": "GetUsers_1"},
			expected:    "{{ GetUsersAll.response.body }}",
		},
		{
			name:        "variable without path",
			input:       "{{ GetUsers }}",
			nameMapping: map[string]string{"GetUsers": "GetUsers_1"},
			expected:    "{{ GetUsers_1 }}",
		},
		{
			name:        "whitespace preserved",
			input:       "{{  GetUsers.response.body  }}",
			nameMapping: map[string]string{"GetUsers": "GetUsers_1"},
			expected:    "{{  GetUsers_1.response.body  }}",
		},
		{
			name:        "nested in JSON body",
			input:       `{"id": "{{ GetUser.response.body.id }}", "name": "{{ GetUser.response.body.name }}"}`,
			nameMapping: map[string]string{"GetUser": "GetUser_2"},
			expected:    `{"id": "{{ GetUser_2.response.body.id }}", "name": "{{ GetUser_2.response.body.name }}"}`,
		},
		{
			name:        "mixed env and node references",
			input:       "{{ #env:BASE_URL }}/{{ GetUsers.response.body.path }}",
			nameMapping: map[string]string{"GetUsers": "GetUsers_1"},
			expected:    "{{ #env:BASE_URL }}/{{ GetUsers_1.response.body.path }}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := remapVarRefs(tt.input, tt.nameMapping)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestRemapVarRefsBytes(t *testing.T) {
	nameMapping := map[string]string{"request": "request_1"}

	input := []byte("return {{ request.response.body }};")
	expected := []byte("return {{ request_1.response.body }};")

	result := remapVarRefsBytes(input, nameMapping)
	require.Equal(t, expected, result)

	// Empty input
	result = remapVarRefsBytes(nil, nameMapping)
	require.Nil(t, result)

	// Empty mapping
	result = remapVarRefsBytes(input, map[string]string{})
	require.Equal(t, input, result)
}

func TestRemapJSBracketRefs(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		nameMapping map[string]string
		expected    string
	}{
		{
			name:        "double quoted ctx reference",
			input:       `const data = ctx["Get_All_Todos"].response.body;`,
			nameMapping: map[string]string{"Get_All_Todos": "Get_All_Todos_1"},
			expected:    `const data = ctx["Get_All_Todos_1"].response.body;`,
		},
		{
			name:        "single quoted ctx reference",
			input:       `const data = ctx['Get_All_Todos'].response.body;`,
			nameMapping: map[string]string{"Get_All_Todos": "Get_All_Todos_1"},
			expected:    `const data = ctx['Get_All_Todos_1'].response.body;`,
		},
		{
			name:        "custom variable name",
			input:       `const data = context["Get_All_Todos"].response.body;`,
			nameMapping: map[string]string{"Get_All_Todos": "Get_All_Todos_1"},
			expected:    `const data = context["Get_All_Todos_1"].response.body;`,
		},
		{
			name:        "any variable name works",
			input:       `const data = myVar["NodeA"].body;`,
			nameMapping: map[string]string{"NodeA": "NodeA_1"},
			expected:    `const data = myVar["NodeA_1"].body;`,
		},
		{
			name:        "multiple references",
			input:       `const a = ctx["NodeA"].body; const b = ctx["NodeB"].body;`,
			nameMapping: map[string]string{"NodeA": "NodeA_1", "NodeB": "NodeB_1"},
			expected:    `const a = ctx["NodeA_1"].body; const b = ctx["NodeB_1"].body;`,
		},
		{
			name:        "no match",
			input:       `const data = ctx["OtherNode"].body;`,
			nameMapping: map[string]string{"Get_All_Todos": "Get_All_Todos_1"},
			expected:    `const data = ctx["OtherNode"].body;`,
		},
		{
			name:        "realistic JS function",
			input:       "export default function(ctx) {\n  const todos = ctx[\"Get_All_Todos\"].response.body;\nreturn todos[0];\n}",
			nameMapping: map[string]string{"Get_All_Todos": "Get_All_Todos_1"},
			expected:    "export default function(ctx) {\n  const todos = ctx[\"Get_All_Todos_1\"].response.body;\nreturn todos[0];\n}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := remapJSBracketRefs(tt.input, tt.nameMapping)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestRemapAllRefs(t *testing.T) {
	// Both {{ }} and ctx[] should be remapped
	nameMapping := map[string]string{"GetUsers": "GetUsers_1"}
	input := `const url = "{{ GetUsers.response.body.url }}"; const data = ctx["GetUsers"].response.body;`
	expected := `const url = "{{ GetUsers_1.response.body.url }}"; const data = ctx["GetUsers_1"].response.body;`
	result := remapAllRefs(input, nameMapping)
	require.Equal(t, expected, result)
}

func TestFlowNodesCopy_JSNodes(t *testing.T) {
	tc := NewRFlowTestContext(t)
	defer tc.Close()
	initCopyPasteTestContext(tc)

	// Create two JS nodes
	node1 := mflow.Node{
		ID: idwrap.NewNow(), FlowID: tc.FlowID,
		Name: "http_request", NodeKind: mflow.NODE_KIND_JS,
		PositionX: 100, PositionY: 200,
	}
	node2 := mflow.Node{
		ID: idwrap.NewNow(), FlowID: tc.FlowID,
		Name: "process_data", NodeKind: mflow.NODE_KIND_JS,
		PositionX: 100, PositionY: 400,
	}
	require.NoError(t, tc.NS.CreateNode(tc.Ctx, node1))
	require.NoError(t, tc.NS.CreateNode(tc.Ctx, node2))
	require.NoError(t, tc.NJSS.CreateNodeJS(tc.Ctx, mflow.NodeJS{FlowNodeID: node1.ID, Code: []byte("return 1;")}))
	require.NoError(t, tc.NJSS.CreateNodeJS(tc.Ctx, mflow.NodeJS{FlowNodeID: node2.ID, Code: []byte("return {{ http_request.response.body }};")}))

	// Create edge between them
	require.NoError(t, tc.ES.CreateEdge(tc.Ctx, mflow.Edge{
		ID: idwrap.NewNow(), FlowID: tc.FlowID, SourceID: node1.ID, TargetID: node2.ID,
	}))

	// Copy both nodes
	copyResp, err := tc.Svc.FlowNodesCopy(tc.Ctx, connect.NewRequest(&flowv1.FlowNodesCopyRequest{
		FlowId:  tc.FlowID.Bytes(),
		NodeIds: [][]byte{node1.ID.Bytes(), node2.ID.Bytes()},
	}))
	require.NoError(t, err)
	require.NotEmpty(t, copyResp.Msg.GetYaml())
	require.Contains(t, copyResp.Msg.GetYaml(), "http_request")
	require.Contains(t, copyResp.Msg.GetYaml(), "process_data")
}

func TestFlowNodesPaste_SameFlow_NameDedup(t *testing.T) {
	tc := NewRFlowTestContext(t)
	defer tc.Close()
	initCopyPasteTestContext(tc)

	// Create a JS node
	node := mflow.Node{
		ID: idwrap.NewNow(), FlowID: tc.FlowID,
		Name: "my_script", NodeKind: mflow.NODE_KIND_JS,
		PositionX: 100, PositionY: 200,
	}
	require.NoError(t, tc.NS.CreateNode(tc.Ctx, node))
	require.NoError(t, tc.NJSS.CreateNodeJS(tc.Ctx, mflow.NodeJS{FlowNodeID: node.ID, Code: []byte("return 42;")}))

	// Copy
	copyResp, err := tc.Svc.FlowNodesCopy(tc.Ctx, connect.NewRequest(&flowv1.FlowNodesCopyRequest{
		FlowId:  tc.FlowID.Bytes(),
		NodeIds: [][]byte{node.ID.Bytes()},
	}))
	require.NoError(t, err)

	// Paste into same flow
	pasteResp, err := tc.Svc.FlowNodesPaste(tc.Ctx, connect.NewRequest(&flowv1.FlowNodesPasteRequest{
		FlowId:        tc.FlowID.Bytes(),
		Yaml:          copyResp.Msg.GetYaml(),
		OffsetY:        200,
		ReferenceMode: flowv1.ReferenceMode_REFERENCE_MODE_CREATE_COPY,
	}))
	require.NoError(t, err)
	require.NotEmpty(t, pasteResp.Msg.GetNodeIds())

	// Verify deduplicated name
	allNodes, err := tc.NS.GetNodesByFlowID(tc.Ctx, tc.FlowID)
	require.NoError(t, err)

	names := make(map[string]bool)
	for _, n := range allNodes {
		names[n.Name] = true
	}
	require.True(t, names["my_script"], "original should exist")
	require.True(t, names["my_script_1"], "pasted should be deduplicated")
}

func TestFlowNodesPaste_VarRemapping(t *testing.T) {
	tc := NewRFlowTestContext(t)
	defer tc.Close()
	initCopyPasteTestContext(tc)

	// Create two JS nodes where the second references the first
	node1 := mflow.Node{
		ID: idwrap.NewNow(), FlowID: tc.FlowID,
		Name: "fetch_data", NodeKind: mflow.NODE_KIND_JS,
		PositionX: 100, PositionY: 200,
	}
	node2 := mflow.Node{
		ID: idwrap.NewNow(), FlowID: tc.FlowID,
		Name: "process", NodeKind: mflow.NODE_KIND_JS,
		PositionX: 100, PositionY: 400,
	}
	require.NoError(t, tc.NS.CreateNode(tc.Ctx, node1))
	require.NoError(t, tc.NS.CreateNode(tc.Ctx, node2))
	require.NoError(t, tc.NJSS.CreateNodeJS(tc.Ctx, mflow.NodeJS{FlowNodeID: node1.ID, Code: []byte("return { items: [1,2,3] };")}))
	require.NoError(t, tc.NJSS.CreateNodeJS(tc.Ctx, mflow.NodeJS{FlowNodeID: node2.ID, Code: []byte("return {{ fetch_data.response.body.items }};")}))

	// Edge
	require.NoError(t, tc.ES.CreateEdge(tc.Ctx, mflow.Edge{
		ID: idwrap.NewNow(), FlowID: tc.FlowID, SourceID: node1.ID, TargetID: node2.ID,
	}))

	// Copy both
	copyResp, err := tc.Svc.FlowNodesCopy(tc.Ctx, connect.NewRequest(&flowv1.FlowNodesCopyRequest{
		FlowId:  tc.FlowID.Bytes(),
		NodeIds: [][]byte{node1.ID.Bytes(), node2.ID.Bytes()},
	}))
	require.NoError(t, err)

	// Paste into same flow
	pasteResp, err := tc.Svc.FlowNodesPaste(tc.Ctx, connect.NewRequest(&flowv1.FlowNodesPasteRequest{
		FlowId:        tc.FlowID.Bytes(),
		Yaml:          copyResp.Msg.GetYaml(),
		OffsetY:        200,
		ReferenceMode: flowv1.ReferenceMode_REFERENCE_MODE_CREATE_COPY,
	}))
	require.NoError(t, err)
	require.Len(t, pasteResp.Msg.GetNodeIds(), 2)

	// Find the pasted "process_1" node
	allNodes, err := tc.NS.GetNodesByFlowID(tc.Ctx, tc.FlowID)
	require.NoError(t, err)

	var pastedProcessNode *mflow.Node
	for _, n := range allNodes {
		if n.Name == "process_1" {
			pastedProcessNode = &n
			break
		}
	}
	require.NotNil(t, pastedProcessNode, "should find pasted process_1 node")

	// Check JS code was remapped
	jsData, err := tc.NJSS.GetNodeJS(tc.Ctx, pastedProcessNode.ID)
	require.NoError(t, err)
	require.Contains(t, string(jsData.Code), "fetch_data_1",
		"JS code should reference fetch_data_1 instead of fetch_data")
}

func TestFlowNodesPaste_CrossFlow(t *testing.T) {
	tc := NewRFlowTestContext(t)
	defer tc.Close()
	initCopyPasteTestContext(tc)

	// Create a node in the first flow
	node := mflow.Node{
		ID: idwrap.NewNow(), FlowID: tc.FlowID,
		Name: "my_node", NodeKind: mflow.NODE_KIND_JS,
		PositionX: 100, PositionY: 200,
	}
	require.NoError(t, tc.NS.CreateNode(tc.Ctx, node))
	require.NoError(t, tc.NJSS.CreateNodeJS(tc.Ctx, mflow.NodeJS{FlowNodeID: node.ID, Code: []byte("return 1;")}))

	// Copy
	copyResp, err := tc.Svc.FlowNodesCopy(tc.Ctx, connect.NewRequest(&flowv1.FlowNodesCopyRequest{
		FlowId:  tc.FlowID.Bytes(),
		NodeIds: [][]byte{node.ID.Bytes()},
	}))
	require.NoError(t, err)

	// Create a second flow
	flow2ID := idwrap.NewNow()
	require.NoError(t, tc.FS.CreateFlow(tc.Ctx, mflow.Flow{
		ID: flow2ID, WorkspaceID: tc.WorkspaceID, Name: "Second Flow",
	}))

	// Paste into second flow — no name collision
	pasteResp, err := tc.Svc.FlowNodesPaste(tc.Ctx, connect.NewRequest(&flowv1.FlowNodesPasteRequest{
		FlowId:        flow2ID.Bytes(),
		Yaml:          copyResp.Msg.GetYaml(),
		OffsetX:        50,
		OffsetY:        50,
		ReferenceMode: flowv1.ReferenceMode_REFERENCE_MODE_CREATE_COPY,
	}))
	require.NoError(t, err)
	require.Len(t, pasteResp.Msg.GetNodeIds(), 1)

	// Verify node in second flow with original name and offset applied
	flow2Nodes, err := tc.NS.GetNodesByFlowID(tc.Ctx, flow2ID)
	require.NoError(t, err)

	var found bool
	for _, n := range flow2Nodes {
		if n.Name == "my_node" {
			found = true
			break
		}
	}
	require.True(t, found, "pasted node should exist in second flow")
}

func TestFlowNodesPaste_ConditionVarRemap(t *testing.T) {
	tc := NewRFlowTestContext(t)
	defer tc.Close()
	initCopyPasteTestContext(tc)

	// Create a JS node and a condition node that references it
	jsNode := mflow.Node{
		ID: idwrap.NewNow(), FlowID: tc.FlowID,
		Name: "check_status", NodeKind: mflow.NODE_KIND_JS,
		PositionX: 100, PositionY: 200,
	}
	condNode := mflow.Node{
		ID: idwrap.NewNow(), FlowID: tc.FlowID,
		Name: "is_success", NodeKind: mflow.NODE_KIND_CONDITION,
		PositionX: 100, PositionY: 400,
	}
	require.NoError(t, tc.NS.CreateNode(tc.Ctx, jsNode))
	require.NoError(t, tc.NS.CreateNode(tc.Ctx, condNode))

	require.NoError(t, tc.NJSS.CreateNodeJS(tc.Ctx, mflow.NodeJS{
		FlowNodeID: jsNode.ID, Code: []byte("return { status: 200 };"),
	}))
	require.NoError(t, tc.NIFS.CreateNodeIf(tc.Ctx, mflow.NodeIf{
		FlowNodeID: condNode.ID,
		Condition: mcondition.Condition{
			Comparisons: mcondition.Comparison{
				Expression: "{{ check_status.response.body.status }} == 200",
			},
		},
	}))

	// Edge
	require.NoError(t, tc.ES.CreateEdge(tc.Ctx, mflow.Edge{
		ID: idwrap.NewNow(), FlowID: tc.FlowID, SourceID: jsNode.ID, TargetID: condNode.ID,
	}))

	// Copy both
	copyResp, err := tc.Svc.FlowNodesCopy(tc.Ctx, connect.NewRequest(&flowv1.FlowNodesCopyRequest{
		FlowId:  tc.FlowID.Bytes(),
		NodeIds: [][]byte{jsNode.ID.Bytes(), condNode.ID.Bytes()},
	}))
	require.NoError(t, err)

	// Paste
	pasteResp, err := tc.Svc.FlowNodesPaste(tc.Ctx, connect.NewRequest(&flowv1.FlowNodesPasteRequest{
		FlowId:        tc.FlowID.Bytes(),
		Yaml:          copyResp.Msg.GetYaml(),
		OffsetY:        200,
		ReferenceMode: flowv1.ReferenceMode_REFERENCE_MODE_CREATE_COPY,
	}))
	require.NoError(t, err)
	require.Len(t, pasteResp.Msg.GetNodeIds(), 2)

	// Find pasted condition node
	allNodes, err := tc.NS.GetNodesByFlowID(tc.Ctx, tc.FlowID)
	require.NoError(t, err)

	var pastedCondNode *mflow.Node
	for _, n := range allNodes {
		if n.Name == "is_success_1" {
			pastedCondNode = &n
			break
		}
	}
	require.NotNil(t, pastedCondNode)

	condData, err := tc.NIFS.GetNodeIf(tc.Ctx, pastedCondNode.ID)
	require.NoError(t, err)
	require.Contains(t, condData.Condition.Comparisons.Expression, "check_status_1",
		"condition should reference check_status_1")
}
