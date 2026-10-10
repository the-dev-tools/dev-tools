package rflowv2

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mexpect"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

// A duplicated flow keeps its AI check settings and its nodes' expect: blocks.
func TestFlowDuplicate_KeepsAIChecks(t *testing.T) {
	svc, ctx, workspaceID, _ := setupFlowDuplicateTestService(t)

	sourceFlowID := idwrap.NewNow()
	require.NoError(t, svc.fs.CreateFlow(ctx, mflow.Flow{ID: sourceFlowID, WorkspaceID: workspaceID, Name: "Source"}))
	httpID := idwrap.NewNow()
	require.NoError(t, svc.hs.Create(ctx, &mhttp.HTTP{ID: httpID, WorkspaceID: workspaceID, Name: "Chat", Url: "https://app.test"}))
	nodeID := idwrap.NewNow()
	require.NoError(t, svc.ns.CreateNode(ctx, mflow.Node{ID: nodeID, FlowID: sourceFlowID, Name: "Chat", NodeKind: mflow.NODE_KIND_REQUEST}))
	require.NoError(t, svc.nrs.CreateNodeRequest(ctx, mflow.NodeRequest{FlowNodeID: nodeID, HttpID: &httpID}))

	checks := sflow.NewAIChecksService(gen.New(svc.DB))
	iterations := 4
	settings := mexpect.FlowSettings{Iterations: &iterations, Judge: &mexpect.JudgeConfig{Provider: "openai"}, Quality: &mexpect.Quality{FailBelow: "90%"}}
	require.NoError(t, checks.UpsertFlowAIChecks(ctx, mflow.FlowAIChecks{FlowID: sourceFlowID, Settings: settings}))
	limit := 4000.0
	require.NoError(t, checks.UpsertNodeExpect(ctx, mflow.NodeExpect{FlowNodeID: nodeID, Expect: mexpect.Expect{MaxLatencyMS: &limit}}))

	_, err := svc.FlowDuplicate(ctx, connect.NewRequest(&flowv1.FlowDuplicateRequest{FlowId: sourceFlowID.Bytes()}))
	require.NoError(t, err)

	flows, err := svc.fsReader.GetFlowsByWorkspaceID(ctx, workspaceID)
	require.NoError(t, err)
	require.Len(t, flows, 2)
	var newFlowID idwrap.IDWrap
	for _, f := range flows {
		if f.ID != sourceFlowID {
			newFlowID = f.ID
		}
	}
	got, err := checks.GetFlowAIChecks(ctx, newFlowID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, settings, got.Settings)

	expects, err := checks.GetNodeExpectsByFlowID(ctx, newFlowID)
	require.NoError(t, err)
	require.Len(t, expects, 1)
	require.NotEqual(t, nodeID, expects[0].FlowNodeID)
	require.InDelta(t, 4000.0, *expects[0].Expect.MaxLatencyMS, 0)
}
