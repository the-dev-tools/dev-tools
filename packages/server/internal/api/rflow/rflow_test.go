package rflow

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logconsole"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/reference"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/testutil"
	flowv1 "the-dev-tools/spec/dist/buf/go/flow/v1"
)

// childByKey returns the direct map child with the given key.
func childByKey(r reference.ReferenceTreeItem, key string) (reference.ReferenceTreeItem, bool) {
	if r.Kind != reference.ReferenceKind_REFERENCE_KIND_MAP {
		return reference.ReferenceTreeItem{}, false
	}
	for _, ch := range r.Map {
		if ch.Key.Kind == reference.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY && ch.Key.Key == key {
			return ch, true
		}
	}
	return reference.ReferenceTreeItem{}, false
}

// stringValue extracts the Value from a VALUE node.
func stringValue(r reference.ReferenceTreeItem) (string, bool) {
	if r.Kind != reference.ReferenceKind_REFERENCE_KIND_VALUE {
		return "", false
	}
	return r.Value, true
}

func TestBuildLogRefs_ErrorKindClassification(t *testing.T) {
	// Non-cancellation error should be labeled as "failed"
	refs := buildLogRefs("nodeA", "id-1", "FAILURE", errors.New("boom"), nil)
	if len(refs) == 0 {
		t.Fatalf("expected reference items")
	}
	root := refs[0]
	errRef, ok := childByKey(root, "error")
	if !ok {
		t.Fatalf("missing error map")
	}
	kindRef, ok := childByKey(errRef, "kind")
	if !ok {
		t.Fatalf("missing error.kind")
	}
	if kind, ok := stringValue(kindRef); !ok || kind != "failed" {
		t.Fatalf("expected kind 'failed'")
	}

	// Cancellation by throw should be labeled as "canceled"
	refs = buildLogRefs("nodeB", "id-2", "CANCELED", runner.ErrFlowCanceledByThrow, nil)
	root = refs[0]
	errRef, ok = childByKey(root, "error")
	if !ok {
		t.Fatalf("missing error map")
	}
	kindRef, ok = childByKey(errRef, "kind")
	if !ok {
		t.Fatalf("missing error.kind")
	}
	if kind, ok := stringValue(kindRef); !ok || kind != "canceled" {
		t.Fatalf("expected kind 'canceled' for throw")
	}

	// Context cancellation should be labeled as "canceled"
	refs = buildLogRefs("nodeC", "id-3", "CANCELED", context.Canceled, nil)
	root = refs[0]
	errRef, ok = childByKey(root, "error")
	if !ok {
		t.Fatalf("missing error map")
	}
	kindRef, ok = childByKey(errRef, "kind")
	if !ok {
		t.Fatalf("missing error.kind")
	}
	if kind, ok := stringValue(kindRef); !ok || kind != "canceled" {
		t.Fatalf("expected kind 'canceled' for context cancellation")
	}
}

func TestBuildLoopNodeExecutionFromStatus(t *testing.T) {
	loopID := idwrap.NewNow()
	execID := idwrap.NewNow()
	status := runner.FlowNodeStatus{
		NodeID: loopID,
		Name:   "Loop",
		State:  mnnode.NODE_STATE_SUCCESS,
		OutputData: map[string]any{
			"completed": true,
		},
	}

	exec := buildLoopNodeExecutionFromStatus(status, execID)

	if exec.ID != execID {
		t.Fatalf("expected execution ID %s, got %s", execID, exec.ID)
	}
	if exec.NodeID != loopID {
		t.Fatalf("expected node ID %s, got %s", loopID, exec.NodeID)
	}
	if exec.Name != "Loop" {
		t.Fatalf("expected name 'Loop', got %s", exec.Name)
	}
	if exec.State != mnnode.NODE_STATE_SUCCESS {
		t.Fatalf("expected SUCCESS state, got %v", exec.State)
	}
	if exec.CompletedAt == nil {
		t.Fatalf("expected CompletedAt to be set")
	}

	if exec.Error != nil {
		t.Fatalf("expected no error, got %s", *exec.Error)
	}

	if len(exec.OutputData) == 0 {
		t.Fatalf("expected output data to be set")
	}

	statusWithError := runner.FlowNodeStatus{
		NodeID: loopID,
		Name:   "Loop",
		State:  mnnode.NODE_STATE_FAILURE,
		Error:  context.Canceled,
	}

	execErr := buildLoopNodeExecutionFromStatus(statusWithError, execID)
	if execErr.State != mnnode.NODE_STATE_FAILURE {
		t.Fatalf("expected FAILURE state, got %v", execErr.State)
	}
	if execErr.Error == nil || *execErr.Error == "" {
		t.Fatalf("expected error string to be set")
	}
}

func TestFlowDurationLifecycle(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	t.Cleanup(base.Close)

	baseServices := base.GetBaseServices()
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	userID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

	flowSvc := sflow.New(base.Queries)
	initialDuration := int32(1500)
	flowID := idwrap.NewNow()
	require.NoError(t, flowSvc.CreateFlow(ctx, mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Latency Test",
		Duration:    initialDuration,
	}))

	svc := FlowServiceRPC{
		DB:         base.DB,
		ws:         baseServices.Ws,
		us:         baseServices.Us,
		fs:         flowSvc,
		logChanMap: logconsole.NewLogChanMap(),
	}

	authedCtx := mwauth.CreateAuthedContext(ctx, userID)

	getResp, err := svc.FlowGet(authedCtx, connect.NewRequest(&flowv1.FlowGetRequest{FlowId: flowID.Bytes()}))
	require.NoError(t, err)
	require.NotNil(t, getResp.Msg.Duration)
	require.Equal(t, initialDuration, getResp.Msg.GetDuration())

	listResp, err := svc.FlowList(authedCtx, connect.NewRequest(&flowv1.FlowListRequest{WorkspaceId: workspaceID.Bytes()}))
	require.NoError(t, err)
	require.Len(t, listResp.Msg.Items, 1)
	require.NotNil(t, listResp.Msg.Items[0].Duration)
	require.Equal(t, initialDuration, listResp.Msg.Items[0].GetDuration())

	updatedDuration := int32(2750)
	_, err = svc.FlowUpdate(authedCtx, connect.NewRequest(&flowv1.FlowUpdateRequest{
		FlowId:   flowID.Bytes(),
		Duration: &updatedDuration,
	}))
	require.NoError(t, err)

	persisted, err := flowSvc.GetFlow(ctx, flowID)
	require.NoError(t, err)
	require.Equal(t, updatedDuration, persisted.Duration)
}
