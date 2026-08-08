package flowlocalrunner

import (
	"context"
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func newTestRunner(t *testing.T, opts ...Option) *FlowLocalRunner {
	t.Helper()

	startID := idwrap.NewNow()
	nodeMap := map[idwrap.IDWrap]node.FlowNode{}
	edgesMap := mflow.EdgesMap{}

	return CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), []idwrap.IDWrap{startID}, nodeMap, edgesMap, 0, nil, opts...)
}

// TestCreateFlowRunnerDefaultUnchanged locks in that constructing a runner
// without options keeps the historical CPU-derived concurrency.
func TestCreateFlowRunnerDefaultUnchanged(t *testing.T) {
	fr := newTestRunner(t)

	if goroutineCount != MaxParallelism() {
		t.Fatalf("package default = %d, want the CPU-derived %d", goroutineCount, MaxParallelism())
	}
	if fr.maxConcurrency != goroutineCount {
		t.Fatalf("default maxConcurrency = %d, want %d (CPU-derived default)", fr.maxConcurrency, goroutineCount)
	}
	if fr.leanMode {
		t.Fatal("default leanMode = true, want false")
	}
}

func TestWithLeanMode(t *testing.T) {
	if fr := newTestRunner(t, WithLeanMode(true)); !fr.leanMode {
		t.Error("WithLeanMode(true): leanMode = false, want true")
	}
	if fr := newTestRunner(t, WithLeanMode(false)); fr.leanMode {
		t.Error("WithLeanMode(false): leanMode = true, want false")
	}
}

// The runner must hand its lean setting to every node through the shared
// FlowNodeRequest, which is the only place nodes can read it from.
func TestLeanModeReachesNodeRequest(t *testing.T) {
	for _, lean := range []bool{false, true} {
		probe := &leanProbeNode{id: idwrap.NewNow()}
		fr := CreateFlowRunner(
			idwrap.NewNow(), idwrap.NewNow(), []idwrap.IDWrap{probe.id},
			map[idwrap.IDWrap]node.FlowNode{probe.id: probe},
			mflow.EdgesMap{}, 0, nil, WithLeanMode(lean),
		)

		if err := fr.RunWithEvents(t.Context(), runner.FlowEventChannels{}, nil); err != nil {
			t.Fatalf("lean=%t: RunWithEvents() error = %v", lean, err)
		}
		if probe.seen != lean {
			t.Errorf("lean=%t: node observed FlowNodeRequest.LeanMode = %t", lean, probe.seen)
		}
	}
}

// leanProbeNode records the LeanMode it was executed with.
type leanProbeNode struct {
	id   idwrap.IDWrap
	seen bool
}

func (n *leanProbeNode) GetID() idwrap.IDWrap { return n.id }
func (n *leanProbeNode) GetName() string      { return "lean-probe" }

func (n *leanProbeNode) RunSync(_ context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	n.seen = req.LeanMode
	return node.FlowNodeResult{}
}

func (n *leanProbeNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	resultChan <- n.RunSync(ctx, req)
}

func TestWithMaxConcurrency(t *testing.T) {
	fr := newTestRunner(t, WithMaxConcurrency(3))

	if fr.maxConcurrency != 3 {
		t.Fatalf("maxConcurrency = %d, want 3", fr.maxConcurrency)
	}
}

func TestWithMaxConcurrencyNonPositiveIsNoOp(t *testing.T) {
	for _, n := range []int{0, -1, -1024} {
		fr := newTestRunner(t, WithMaxConcurrency(n))

		if fr.maxConcurrency != goroutineCount {
			t.Errorf("WithMaxConcurrency(%d): maxConcurrency = %d, want default %d", n, fr.maxConcurrency, goroutineCount)
		}
	}
}

func TestOptionsAppliedInOrder(t *testing.T) {
	fr := newTestRunner(t, WithMaxConcurrency(7), WithMaxConcurrency(2))

	if fr.maxConcurrency != 2 {
		t.Fatalf("maxConcurrency = %d, want 2 (last option wins)", fr.maxConcurrency)
	}
}

func TestNilOptionIgnored(t *testing.T) {
	fr := newTestRunner(t, nil, WithMaxConcurrency(4))

	if fr.maxConcurrency != 4 {
		t.Fatalf("maxConcurrency = %d, want 4", fr.maxConcurrency)
	}
}
