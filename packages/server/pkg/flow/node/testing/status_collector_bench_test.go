package testing

import (
	"testing"
	"time"

	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
)

func BenchmarkStatusCollector_Capture(b *testing.B) {
	collector := NewStatusCollector()
	defer collector.Close()

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()

	status := runner.FlowNodeStatus{
		ExecutionID: executionID,
		NodeID:      nodeID,
		Name:        "test-node",
		State:       mnnode.NODE_STATE_RUNNING,
		RunDuration: 100 * time.Millisecond,
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			collector.Capture(status)
		}
	})
}

func BenchmarkStatusCollector_Filter(b *testing.B) {
	collector := NewStatusCollector()
	defer collector.Close()

	nodeID := idwrap.NewNow()
	executionID := idwrap.NewNow()

	// Pre-populate with statuses
	for i := 0; i < 1000; i++ {
		status := runner.FlowNodeStatus{
			ExecutionID: executionID,
			NodeID:      nodeID,
			Name:        "test-node",
			State:       mnnode.NodeState(i % 4), // Cycle through states
			RunDuration: time.Duration(i) * time.Millisecond,
		}
		collector.Capture(status)
	}

	filter := StatusFilter{
		NodeID: &nodeID,
		State:  &[]mnnode.NodeState{mnnode.NODE_STATE_SUCCESS}[0],
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = collector.Filter(filter)
	}
}

func BenchmarkStatusValidator_ValidateAll(b *testing.B) {
	collector := NewStatusCollector()
	defer collector.Close()

	nodeID := idwrap.NewNow()

	// Create a valid execution sequence for each benchmark iteration
	for i := 0; i < 100; i++ {
		executionID := idwrap.NewNow()

		// RUNNING -> SUCCESS sequence
		statuses := []runner.FlowNodeStatus{
			{
				ExecutionID: executionID,
				NodeID:      nodeID,
				Name:        "test-node",
				State:       mnnode.NODE_STATE_RUNNING,
				RunDuration: 100 * time.Millisecond,
			},
			{
				ExecutionID: executionID,
				NodeID:      nodeID,
				Name:        "test-node",
				State:       mnnode.NODE_STATE_SUCCESS,
				RunDuration: 200 * time.Millisecond,
			},
		}

		for _, status := range statuses {
			collector.Capture(status)
		}
	}

	validator := NewStatusValidator(collector)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.ValidateAll()
	}
}
