package rflowv2

import (
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mexpect"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

// Copying a streaming request node with an expect: block and pasting it keeps both, under the
// pasted node's and request's new IDs.
func TestFlowNodesCopyPaste_KeepsStreamAndExpect(t *testing.T) {
	tc := NewRFlowTestContext(t)
	defer tc.Close()
	initCopyPasteTestContext(tc)
	tc.Svc.hsReader = shttp.NewReader(tc.DB, tc.Svc.logger, nil)
	tc.Svc.hbr = shttp.NewHttpBodyRawService(tc.Queries)

	httpID := idwrap.NewNow()
	require.NoError(t, tc.Svc.hs.Create(tc.Ctx, &mhttp.HTTP{ID: httpID, WorkspaceID: tc.WorkspaceID, Name: "Chat",
		Url: "https://app.test/api/chat", Method: "POST", BodyKind: mhttp.HttpBodyKindNone}))
	node := mflow.Node{ID: idwrap.NewNow(), FlowID: tc.FlowID, Name: "Chat", NodeKind: mflow.NODE_KIND_REQUEST}
	require.NoError(t, tc.NS.CreateNode(tc.Ctx, node))
	require.NoError(t, tc.NRS.CreateNodeRequest(tc.Ctx, mflow.NodeRequest{FlowNodeID: node.ID, HttpID: &httpID}))

	streams := shttp.NewHTTPStreamService(tc.Queries)
	checks := sflow.NewAIChecksService(tc.Queries)
	require.NoError(t, streams.Upsert(tc.Ctx, mhttp.HTTPStream{HttpID: httpID, Preset: "openai", TimeoutMs: 15000}))
	ttft := 800.0
	expect := mexpect.Expect{MaxTTFTMS: &ttft, Judge: &mexpect.JudgeSpec{Criteria: "Names where to cancel.", Output: "{{ response.text }}"}}
	require.NoError(t, checks.UpsertNodeExpect(tc.Ctx, mflow.NodeExpect{FlowNodeID: node.ID, Expect: expect}))

	copyResp, err := tc.Svc.FlowNodesCopy(tc.Ctx, connect.NewRequest(&flowv1.FlowNodesCopyRequest{
		FlowId: tc.FlowID.Bytes(), NodeIds: [][]byte{node.ID.Bytes()},
	}))
	require.NoError(t, err)
	yamlText := copyResp.Msg.GetYaml()
	require.Contains(t, yamlText, "stream: openai")
	require.Contains(t, yamlText, "max_ttft_ms: 800")

	pasteResp, err := tc.Svc.FlowNodesPaste(tc.Ctx, connect.NewRequest(&flowv1.FlowNodesPasteRequest{
		FlowId: tc.FlowID.Bytes(), Yaml: yamlText, ReferenceMode: flowv1.ReferenceMode_REFERENCE_MODE_CREATE_COPY,
	}))
	require.NoError(t, err)
	require.Len(t, pasteResp.Msg.GetNodeIds(), 1)
	pastedID, err := idwrap.NewFromBytes(pasteResp.Msg.GetNodeIds()[0])
	require.NoError(t, err)
	require.NotEqual(t, node.ID, pastedID)

	pastedExpect, err := checks.GetNodeExpect(tc.Ctx, pastedID)
	require.NoError(t, err)
	require.NotNil(t, pastedExpect)
	require.Equal(t, expect, pastedExpect.Expect)

	rn, err := tc.NRS.GetNodeRequest(tc.Ctx, pastedID)
	require.NoError(t, err)
	require.NotEqual(t, httpID, *rn.HttpID, "CREATE_COPY makes a new request")
	pastedStream, err := streams.Get(tc.Ctx, *rn.HttpID)
	require.NoError(t, err)
	require.Equal(t, &mhttp.HTTPStream{HttpID: *rn.HttpID, Preset: "openai", TimeoutMs: 15000}, pastedStream)
	require.False(t, strings.Contains(yamlText, "judge: {}"))
}
