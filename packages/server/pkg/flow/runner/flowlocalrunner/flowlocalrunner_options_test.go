package flowlocalrunner

import (
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
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

	if fr.maxConcurrency != goroutineCount {
		t.Fatalf("default maxConcurrency = %d, want %d (CPU-derived default)", fr.maxConcurrency, goroutineCount)
	}
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
