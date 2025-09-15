package flowlocalrunner

import (
    "context"
    "errors"
    "testing"
    "time"
    "the-dev-tools/server/pkg/flow/edge"
    "the-dev-tools/server/pkg/flow/node"
    "the-dev-tools/server/pkg/flow/runner"
    "the-dev-tools/server/pkg/idwrap"
    "the-dev-tools/server/pkg/model/mnnode"
)

// testNode is a minimal FlowNode for testing
type testNode struct {
    id   idwrap.IDWrap
    name string
    next []idwrap.IDWrap
    err  error
}

func (n *testNode) GetID() idwrap.IDWrap { return n.id }
func (n *testNode) GetName() string      { return n.name }
func (n *testNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
    return node.FlowNodeResult{NextNodeID: n.next, Err: n.err}
}
func (n *testNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
    // Delegate to sync for simplicity
    resultChan <- n.RunSync(ctx, req)
}

// cancelAwareNode sleeps for the given duration or exits early on ctx.Done().
// If the context is canceled before sleep completes, it returns ctx.Err().
type cancelAwareNode struct {
    id    idwrap.IDWrap
    name  string
    next  []idwrap.IDWrap
    sleep time.Duration
}

func (n *cancelAwareNode) GetID() idwrap.IDWrap { return n.id }
func (n *cancelAwareNode) GetName() string      { return n.name }
func (n *cancelAwareNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
    select {
    case <-time.After(n.sleep):
        return node.FlowNodeResult{NextNodeID: n.next}
    case <-ctx.Done():
        return node.FlowNodeResult{NextNodeID: nil, Err: ctx.Err()}
    }
}
func (n *cancelAwareNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
    resultChan <- n.RunSync(ctx, req)
}

func collectFinalStates(t *testing.T, ch <-chan runner.FlowNodeStatus) map[idwrap.IDWrap]mnnode.NodeState {
    t.Helper()
    final := make(map[idwrap.IDWrap]mnnode.NodeState)
    for s := range ch {
        // Last seen state for a given ExecutionID reflects the terminal state
        final[s.ExecutionID] = s.State
    }
    return final
}

func TestFlowLocalRunner_ReportsFailureOverCancellation_Sync(t *testing.T) {
    t.Parallel()

    // Build two nodes: start -> fail
    startID := idwrap.NewNow()
    failID := idwrap.NewNow()

    start := &testNode{id: startID, name: "start", next: []idwrap.IDWrap{failID}, err: nil}
    failErr := errors.New("boom")
    fail := &testNode{id: failID, name: "fail", next: nil, err: failErr}

    nodes := map[idwrap.IDWrap]node.FlowNode{
        startID: start,
        failID:  fail,
    }
    edgesMap := edge.NewEdgesMap([]edge.Edge{
        edge.NewEdge(idwrap.NewNow(), startID, failID, edge.HandleUnspecified, int32(edge.EdgeKindNoOp)),
    })

    r := CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), startID, nodes, edgesMap, 0)

    nodeCh := make(chan runner.FlowNodeStatus, 16)
    flowCh := make(chan runner.FlowStatus, 4)

    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    done := make(chan error, 1)
    go func() { done <- r.Run(ctx, nodeCh, flowCh, nil) }()

    finalByNode := make(map[idwrap.IDWrap]mnnode.NodeState)
    for s := range nodeCh {
        finalByNode[s.NodeID] = s.State
    }

    err := <-done
    if err == nil {
        t.Fatalf("expected run error, got nil")
    }

    state, ok := finalByNode[failID]
    if !ok {
        t.Fatalf("did not receive status for failing node")
    }
    if state != mnnode.NODE_STATE_FAILURE {
        t.Fatalf("expected failing node final state FAILURE, got %v", mnnode.StringNodeState(state))
    }
}

func TestFlowLocalRunner_ReportsFailureOverCancellation_Async(t *testing.T) {
    t.Parallel()

    // Build two nodes: start -> fail
    startID := idwrap.NewNow()
    failID := idwrap.NewNow()

    start := &testNode{id: startID, name: "start", next: []idwrap.IDWrap{failID}, err: nil}
    failErr := errors.New("boom")
    fail := &testNode{id: failID, name: "fail", next: nil, err: failErr}

    nodes := map[idwrap.IDWrap]node.FlowNode{
        startID: start,
        failID:  fail,
    }
    edgesMap := edge.NewEdgesMap([]edge.Edge{
        edge.NewEdge(idwrap.NewNow(), startID, failID, edge.HandleUnspecified, int32(edge.EdgeKindNoOp)),
    })

    // Non-zero timeout triggers async runner path
    r := CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), startID, nodes, edgesMap, 100*time.Millisecond)

    nodeCh := make(chan runner.FlowNodeStatus, 16)
    flowCh := make(chan runner.FlowStatus, 4)

    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    done := make(chan error, 1)
    go func() { done <- r.Run(ctx, nodeCh, flowCh, nil) }()

    // Collect all statuses and verify the failing node ended with FAILURE, not CANCELED
    // Since Run closes the channel, iterate until closed.
    finalByNode := make(map[idwrap.IDWrap]mnnode.NodeState)
    for s := range nodeCh {
        // Keep the last state per node ID
        finalByNode[s.NodeID] = s.State
    }

    err := <-done
    if err == nil {
        t.Fatalf("expected run error, got nil")
    }

    state, ok := finalByNode[failID]
    if !ok {
        t.Fatalf("did not receive status for failing node")
    }
    if state != mnnode.NODE_STATE_FAILURE {
        t.Fatalf("expected failing node final state FAILURE, got %v", mnnode.StringNodeState(state))
    }
}

func TestFlowLocalRunner_CanceledByThrow_IsCanceledState(t *testing.T) {
    t.Parallel()

    startID := idwrap.NewNow()
    cancelID := idwrap.NewNow()

    start := &testNode{id: startID, name: "start", next: []idwrap.IDWrap{cancelID}, err: nil}
    cancelNode := &testNode{id: cancelID, name: "cancel", next: nil, err: runner.ErrFlowCanceledByThrow}

    nodes := map[idwrap.IDWrap]node.FlowNode{
        startID:  start,
        cancelID: cancelNode,
    }
    edgesMap := edge.NewEdgesMap([]edge.Edge{
        edge.NewEdge(idwrap.NewNow(), startID, cancelID, edge.HandleUnspecified, int32(edge.EdgeKindNoOp)),
    })

    r := CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), startID, nodes, edgesMap, 0)
    nodeCh := make(chan runner.FlowNodeStatus, 16)
    flowCh := make(chan runner.FlowStatus, 4)
    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    done := make(chan error, 1)
    go func() { done <- r.Run(ctx, nodeCh, flowCh, nil) }()

    finalByNode := map[idwrap.IDWrap]mnnode.NodeState{}
    for s := range nodeCh {
        finalByNode[s.NodeID] = s.State
    }
    <-done

    if got := finalByNode[cancelID]; got != mnnode.NODE_STATE_CANCELED {
        t.Fatalf("expected cancel node state CANCELED, got %v", mnnode.StringNodeState(got))
    }
}

func TestFlowLocalRunner_AsyncNodeTimeout_ClassifiedAsFailure(t *testing.T) {
    t.Parallel()

    startID := idwrap.NewNow()
    slowID := idwrap.NewNow()

    // slow node sleeps longer than runner's per-batch deadline
    start := &testNode{id: startID, name: "start", next: []idwrap.IDWrap{slowID}, err: nil}
    slow := &cancelAwareNode{id: slowID, name: "slow", next: nil, sleep: 200 * time.Millisecond}

    nodes := map[idwrap.IDWrap]node.FlowNode{
        startID: start,
        slowID:  slow,
    }
    edgesMap := edge.NewEdgesMap([]edge.Edge{
        edge.NewEdge(idwrap.NewNow(), startID, slowID, edge.HandleUnspecified, int32(edge.EdgeKindNoOp)),
    })

    // runner timeout 50ms < slow 200ms
    r := CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), startID, nodes, edgesMap, 50*time.Millisecond)
    nodeCh := make(chan runner.FlowNodeStatus, 16)
    flowCh := make(chan runner.FlowStatus, 4)
    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    done := make(chan error, 1)
    go func() { done <- r.Run(ctx, nodeCh, flowCh, nil) }()

    finalByNode := map[idwrap.IDWrap]mnnode.NodeState{}
    for s := range nodeCh {
        finalByNode[s.NodeID] = s.State
    }
    <-done

    // DeadlineExceeded is not treated as cancellation by IsCancellationError,
    // so node should end in FAILURE according to runner logic.
    if got := finalByNode[slowID]; got != mnnode.NODE_STATE_FAILURE {
        t.Fatalf("expected slow node to end FAILURE on timeout, got %v", mnnode.StringNodeState(got))
    }
}
