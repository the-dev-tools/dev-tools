package eventsync

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEventBatch_Add(t *testing.T) {
	batch := NewEventBatch()
	require.Equal(t, 0, batch.Len())

	batch.AddSimple(KindFlow, func() {})
	require.Equal(t, 1, batch.Len())

	batch.Add(KindNode, 1, func() {})
	require.Equal(t, 2, batch.Len())
}

func TestEventBatch_Clear(t *testing.T) {
	batch := NewEventBatch()
	batch.AddSimple(KindFlow, func() {})
	batch.AddSimple(KindNode, func() {})
	require.Equal(t, 2, batch.Len())

	batch.Clear()
	require.Equal(t, 0, batch.Len())
}

func TestEventBatch_Publish_EmptyBatch(t *testing.T) {
	batch := NewEventBatch()
	err := batch.Publish(context.Background()) // Should not panic
	require.NoError(t, err)
}

func TestEventBatch_Publish_Order(t *testing.T) {
	batch := NewEventBatch()

	var order []EventKind
	var mu sync.Mutex

	record := func(kind EventKind) func() {
		return func() {
			mu.Lock()
			order = append(order, kind)
			mu.Unlock()
		}
	}

	// Add events in WRONG order
	batch.AddSimple(KindHTTPHeader, record(KindHTTPHeader))
	batch.AddSimple(KindNode, record(KindNode))
	batch.AddSimple(KindFlow, record(KindFlow))
	batch.AddSimple(KindFlowFile, record(KindFlowFile))
	batch.AddSimple(KindHTTP, record(KindHTTP))
	batch.AddSimple(KindEdge, record(KindEdge))

	err := batch.Publish(context.Background())
	require.NoError(t, err)

	// Verify correct order based on dependencies
	require.Len(t, order, 6)

	// Flow must come before FlowFile and Node
	flowIdx := indexOf(order, KindFlow)
	flowFileIdx := indexOf(order, KindFlowFile)
	nodeIdx := indexOf(order, KindNode)
	edgeIdx := indexOf(order, KindEdge)
	httpIdx := indexOf(order, KindHTTP)
	httpHeaderIdx := indexOf(order, KindHTTPHeader)

	require.Less(t, flowIdx, flowFileIdx, "Flow before FlowFile")
	require.Less(t, flowIdx, nodeIdx, "Flow before Node")
	require.Less(t, nodeIdx, edgeIdx, "Node before Edge")
	require.Less(t, nodeIdx, httpIdx, "Node before HTTP")
	require.Less(t, httpIdx, httpHeaderIdx, "HTTP before HTTPHeader")
}

func TestEventBatch_Publish_SubOrder(t *testing.T) {
	batch := NewEventBatch()

	var order []int
	var mu sync.Mutex

	record := func(n int) func() {
		return func() {
			mu.Lock()
			order = append(order, n)
			mu.Unlock()
		}
	}

	// Add nodes with different subOrder (graph levels)
	batch.Add(KindNode, 2, record(2)) // Level 2
	batch.Add(KindNode, 0, record(0)) // Level 0 (start)
	batch.Add(KindNode, 1, record(1)) // Level 1

	err := batch.Publish(context.Background())
	require.NoError(t, err)

	// Should be sorted by subOrder within same kind
	require.Equal(t, []int{0, 1, 2}, order)
}

func TestEventBatch_Publish_ClearsBatch(t *testing.T) {
	batch := NewEventBatch()

	called := false
	batch.AddSimple(KindFlow, func() { called = true })

	require.Equal(t, 1, batch.Len())
	err := batch.Publish(context.Background())
	require.NoError(t, err)
	require.True(t, called)

	// Batch should be cleared after publish
	require.Equal(t, 0, batch.Len())
}

func TestEventBatch_GetOrderedKinds(t *testing.T) {
	batch := NewEventBatch()

	batch.AddSimple(KindHTTP, func() {})
	batch.AddSimple(KindFlow, func() {})
	batch.AddSimple(KindNode, func() {})
	batch.AddSimple(KindFlow, func() {}) // Duplicate
	batch.AddSimple(KindFlowFile, func() {})

	kinds := batch.GetOrderedKinds()

	// Should be deduplicated and sorted
	require.Len(t, kinds, 4) // Flow appears once due to dedup

	// Verify order
	flowIdx := indexOf(kinds, KindFlow)
	flowFileIdx := indexOf(kinds, KindFlowFile)
	nodeIdx := indexOf(kinds, KindNode)
	httpIdx := indexOf(kinds, KindHTTP)

	require.Less(t, flowIdx, flowFileIdx)
	require.Less(t, flowIdx, nodeIdx)
	require.Less(t, nodeIdx, httpIdx)
}

func TestEventBatch_Concurrent(t *testing.T) {
	batch := NewEventBatch()
	var wg sync.WaitGroup
	var mu sync.Mutex
	count := 0

	// Add events concurrently
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			batch.AddSimple(KindFlow, func() {
				mu.Lock()
				count++
				mu.Unlock()
			})
		}()
	}

	wg.Wait()
	require.Equal(t, 100, batch.Len())

	err := batch.Publish(context.Background())
	require.NoError(t, err)
	require.Equal(t, 100, count)
}

func TestEventBatch_RealWorldScenario(t *testing.T) {
	// Simulate a real import scenario
	batch := NewEventBatch()

	var order []string
	var mu sync.Mutex

	record := func(name string) func() {
		return func() {
			mu.Lock()
			order = append(order, name)
			mu.Unlock()
		}
	}

	// Add events as they would be in a real import (random order)
	batch.AddSimple(KindHTTPHeader, record("header1"))
	batch.AddSimple(KindHTTPHeader, record("header2"))
	batch.Add(KindNode, 1, record("node_level1"))
	batch.Add(KindNode, 0, record("node_level0"))
	batch.AddSimple(KindFlow, record("flow"))
	batch.AddSimple(KindFlowFile, record("flowfile"))
	batch.AddSimple(KindHTTP, record("http"))
	batch.AddSimple(KindEdge, record("edge"))

	err := batch.Publish(context.Background())
	require.NoError(t, err)

	// Verify logical order
	require.Equal(t, "flow", order[0], "Flow should be first")
	require.Equal(t, "flowfile", order[1], "FlowFile should be second")
	require.Equal(t, "node_level0", order[2], "Node level 0 should be third")
	require.Equal(t, "node_level1", order[3], "Node level 1 should be fourth")
	require.Equal(t, "edge", order[4], "Edge should be fifth")
	require.Equal(t, "http", order[5], "HTTP should be sixth")
	// Headers should be last
	require.Contains(t, order[6:], "header1")
	require.Contains(t, order[6:], "header2")
}

func TestEventBatch_Publish_Cancelled(t *testing.T) {
	batch := NewEventBatch()
	ctx, cancel := context.WithCancel(context.Background())

	count := 0
	batch.AddSimple(KindFlow, func() { count++ })
	batch.AddSimple(KindFlow, func() { count++ })

	cancel() // Cancel context before publish
	err := batch.Publish(ctx)

	require.Error(t, err)
	require.True(t, errors.Is(err, context.Canceled))
	require.Equal(t, 0, count, "No events should have been published")
}
