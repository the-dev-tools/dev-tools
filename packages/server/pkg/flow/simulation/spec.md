# Mock Flow Simulation System

## Overview

The mock flow simulation system provides a simple, idiomatic Go interface for creating test flows to validate FlowLocalRunner performance and behavior under various load conditions.

## Purpose

- **Performance Testing**: Validate FlowLocalRunner executes flows efficiently in parallel mode
- **Load Testing**: Test system behavior with different flow sizes and complexities
- **Execution Mode Validation**: Verify automatic mode selection logic works correctly
- **Timeout Testing**: Ensure proper cancellation and resource cleanup
- **Integration Testing**: Validate complete FlowLocalRunner execution pipeline

## Core Components

### MockFlowParams

```go
type MockFlowParams struct {
    RequestCount int           // Number of request nodes to create
    ForLoopCount int           // Number of for loop nodes to create
    Delay        time.Duration // Execution delay per node
}
```

### CreateMockFlow

Main function that generates linear mock flows:

```go
func CreateMockFlow(params MockFlowParams) MockFlowResult
```

**Flow Pattern**: `start → request1 → request2 → ... → forLoop1 → forLoop2 → ...`

### MockFlowResult

Returns exactly what FlowLocalRunner needs:

```go
type MockFlowResult struct {
    Nodes       map[idwrap.IDWrap]node.FlowNode
    Edges       []edge.Edge
    EdgesMap    edge.EdgesMap
    StartNodeID idwrap.IDWrap
}
```

## Usage Examples

### Basic Performance Test

```go
params := MockFlowParams{
    RequestCount: 20,
    ForLoopCount: 5,
    Delay: 2 * time.Millisecond,
}

result := CreateMockFlow(params)

// Use with FlowLocalRunner
runner := flowlocalrunner.CreateFlowRunner(
    idwrap.NewNow(),
    idwrap.NewNow(),
    result.StartNodeID,
    result.Nodes,
    result.EdgesMap,
    10*time.Second,
    nil,
)

err := runner.Run(context.Background(), nodeStatusChan, flowStatusChan, nil)
```

### Load Testing Scenarios

```go
// Small flow (single mode)
smallFlow := CreateMockFlow(MockFlowParams{
    RequestCount: 3,
    ForLoopCount: 1,
    Delay: 10 * time.Millisecond,
})

// Large flow (multi mode)
largeFlow := CreateMockFlow(MockFlowParams{
    RequestCount: 50,
    ForLoopCount: 20,
    Delay: 5 * time.Millisecond,
})

// High concurrency test
concurrencyFlow := CreateMockFlow(MockFlowParams{
    RequestCount: 100,
    ForLoopCount: 0,
    Delay: 1 * time.Millisecond,
})
```

## Test Coverage

The system includes comprehensive tests covering:

### Basic Functionality Tests

- Node creation and edge connectivity
- Linear flow structure validation
- Edge case handling (zero nodes, single node types)

### Performance Tests

- Parallel execution validation
- Load testing with various flow sizes
- Performance regression detection

### Execution Mode Tests

- Automatic mode selection validation
- Single vs multi mode behavior
- Mode selection thresholds

### Timeout Tests

- Timeout handling and cancellation
- Resource cleanup verification
- Error propagation validation

### Integration Tests

- Full FlowLocalRunner execution pipeline
- Node status tracking
- Flow completion validation

## Implementation Details

### Node Creation

- Uses existing `mocknode.NewDelayedMockNode()` for all nodes
- Configurable delays for timing control
- Linear connectivity pattern (simple, predictable)

### Edge Management

- Uses existing `edge.NewEdge()` for connections
- Creates optimized `edge.EdgesMap` for FlowLocalRunner
- Proper handle types (`edge.HandleThen`)

### ID Generation

- Uses `idwrap.NewNow()` for unique IDs
- Consistent with existing codebase patterns

## Performance Characteristics

### Creation Performance

- **O(n)** node creation time
- **O(n)** edge creation time
- Minimal memory allocation

### Execution Performance

- Parallel execution for flows > 6 nodes
- Configurable delays for timing control
- Efficient goroutine pool usage

### Memory Usage

- Linear memory growth with node count
- No memory leaks (proper cleanup)
- Efficient data structures

## Integration with FlowLocalRunner

The simulation system is designed to work seamlessly with FlowLocalRunner:

1. **Node Map**: Direct compatibility with `map[idwrap.IDWrap]node.FlowNode`
2. **Edge Map**: Uses `edge.EdgesMap` for optimized lookups
3. **Start Node**: Provides clear entry point for execution
4. **Mock Interface**: Implements `node.FlowNode` interface correctly

## Best Practices

### Performance Testing

- Use realistic delays (1-10ms) for measurable timing
- Test with various flow sizes (small, medium, large)
- Verify parallel execution benefits

### Load Testing

- Start with moderate loads (20-50 nodes)
- Gradually increase to find system limits
- Monitor for goroutine leaks and memory issues

### Timeout Testing

- Use shorter timeouts than node delays to trigger cancellations
- Verify proper error propagation
- Ensure resource cleanup

## Future Extensions

The system is designed for incremental enhancement:

### Additional Node Types

- Condition nodes with configurable logic
- JavaScript nodes with custom behavior
- Custom node implementations

### Complex Flow Patterns

- Branching flows (diamond patterns)
- Parallel execution paths
- Loop structures with dependencies

### Advanced Testing

- Resource usage monitoring
- Performance benchmarking
- Stress testing utilities

## Dependencies

The simulation system leverages existing infrastructure:

- `mocknode.MockNode` - Mock node implementation
- `edge.NewEdge()` - Edge creation utilities
- `idwrap.NewNow()` - ID generation
- `flowlocalrunner.FlowLocalRunner` - Execution engine

No additional dependencies are required.

## File Structure

```
packages/server/pkg/flow/simulation/
├── mockflows.go      # Core implementation
├── mockflows_test.go # Comprehensive tests
└── spec.md          # This specification
```

## Testing

Run tests with:

```bash
go test ./packages/server/pkg/flow/simulation/... -v
```

Run specific test categories:

```bash
# Performance tests
go test ./packages/server/pkg/flow/simulation/... -run Performance -v

# Execution mode tests
go test ./packages/server/pkg/flow/simulation/... -run ExecutionMode -v

# Timeout tests
go test ./packages/server/pkg/flow/simulation/... -run Timeout -v
```

## Contributing

When extending the simulation system:

1. **Keep it Simple**: Focus on specific testing needs
2. **Use Existing Infrastructure**: Leverage mocknode and edge utilities
3. **Add Tests**: Ensure new functionality is well-tested
4. **Document Changes**: Update this spec for new features
5. **Maintain Compatibility**: Ensure FlowLocalRunner integration works

## Version History

- **v1.0.0**: Initial implementation with basic mock flows, performance testing, execution mode validation, and timeout testing
