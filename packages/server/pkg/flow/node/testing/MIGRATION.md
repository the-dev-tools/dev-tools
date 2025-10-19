# Migration Guide: Converting Manual Tests to Framework

This guide helps you convert existing manual node tests to use the idiomatic Go testing framework.

## Overview

The testing framework provides:

- **Consistent patterns** for all node types
- **Reduced boilerplate** with helper functions
- **Built-in validation** for common scenarios
- **Status event handling** for nodes that emit them
- **Table-driven test structure** for maintainability

## Quick Migration Steps

### 1. Identify Your Current Test Pattern

Most manual tests follow this pattern:

```go
func TestMyNode_Something(t *testing.T) {
    // Setup: Create node, edges, variables
    id := idwrap.NewNow()
    node := mynode.New(id, "test-node", config)

    // Create edges and node map
    edge1 := edge.NewEdge(...)
    nodeMap := map[idwrap.IDWrap]node.FlowNode{...}
    edgesMap := edge.NewEdgesMap([]edge.Edge{edge1})

    // Create request
    req := &node.FlowNodeRequest{
        VarMap:        map[string]interface{}{...},
        ReadWriteLock: &sync.RWMutex{},
        NodeMap:       nodeMap,
        EdgeSourceMap: edgesMap,
    }

    // Execute
    result := node.RunSync(ctx, req)

    // Assert
    if result.Err != nil {
        t.Errorf("Expected no error, got %v", result.Err)
    }
    // More assertions...
}
```

### 2. Choose the Right Framework Pattern

#### For Basic Success Tests

```go
// Before (manual)
func TestMyNode_BasicSuccess(t *testing.T) {
    // 50+ lines of setup...
}

// After (framework)
func TestMyNode_BasicSuccess(t *testing.T) {
    creator := func() node.FlowNode {
        return mynode.New(idwrap.NewNow(), "test", config)
    }

    opts := DefaultTestNodeOptions()
    opts.EdgeMap = edge.EdgesMap{
        // Your edge configuration
    }

    TestNodeSuccess(t, creator(), opts)
}
```

#### For Error Handling Tests

```go
// Before (manual)
func TestMyNode_Error(t *testing.T) {
    // Setup...
    req.Timeout = 1 * time.Nanosecond // Force error
    result := node.RunSync(ctx, req)
    if result.Err == nil {
        t.Error("Expected error, got nil")
    }
}

// After (framework)
func TestMyNode_Error(t *testing.T) {
    creator := func() node.FlowNode {
        return mynode.New(idwrap.NewNow(), "test", config)
    }

    opts := DefaultTestNodeOptions()
    TestNodeError(t, creator(), opts, func(req *node.FlowNodeRequest) {
        req.Timeout = 1 * time.Nanosecond // Force error
    })
}
```

#### For Multiple Test Cases (Table-Driven)

```go
// Before (manual)
func TestMyNode_MultipleScenarios(t *testing.T) {
    tests := []struct {
        name     string
        setup    func() *node.FlowNodeRequest
        expected error
    }{
        {"success", func() *node.FlowNodeRequest { /* setup */ }, nil},
        {"timeout", func() *node.FlowNodeRequest { /* setup */ }, context.DeadlineExceeded},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req := tt.setup()
            result := node.RunSync(ctx, req)
            // Assertions...
        })
    }
}

// After (framework)
func TestMyNode_FrameworkTests(t *testing.T) {
    creator := func() node.FlowNode {
        return mynode.New(idwrap.NewNow(), "test", config)
    }

    testCases := []NodeTestCase{
        {
            Name: "Success",
            TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
                opts := DefaultTestNodeOptions()
                TestNodeSuccess(t, testNode, opts)
            },
        },
        {
            Name: "Timeout",
            TestFunc: func(t *testing.T, ctx *TestContext, testNode node.FlowNode) {
                opts := DefaultTestNodeOptions()
                TestNodeTimeout(t, testNode, opts)
            },
        },
    }

    RunNodeTests(t, creator, testCases)
}
```

## Specific Migration Examples

### IF Node Migration

**Before:**

```go
func TestForNode_RunSync_true(t *testing.T) {
    mockNode1ID := idwrap.NewNow()
    mockNode2ID := idwrap.NewNow()

    var runCounter int
    testFuncInc := func() { runCounter++ }

    mockNode1 := mocknode.NewMockNode(mockNode1ID, nil, testFuncInc)
    mockNode2 := mocknode.NewMockNode(mockNode2ID, nil, testFuncInc)

    nodeMap := map[idwrap.IDWrap]node.FlowNode{
        mockNode1ID: mockNode1,
        mockNode2ID: mockNode2,
    }

    id := idwrap.NewNow()
    nodeName := "test-node"

    nodeFor := nif.New(id, nodeName, mcondition.Condition{
        Comparisons: mcondition.Comparison{Expression: "1 == 1"},
    })
    ctx := context.Background()

    edge1 := edge.NewEdge(idwrap.NewNow(), id, mockNode1ID, edge.HandleThen, edge.EdgeKindUnspecified)
    edge2 := edge.NewEdge(idwrap.NewNow(), id, mockNode2ID, edge.HandleElse, edge.EdgeKindUnspecified)
    edges := []edge.Edge{edge1, edge2}
    edgesMap := edge.NewEdgesMap(edges)

    req := &node.FlowNodeRequest{
        VarMap:        map[string]interface{}{},
        ReadWriteLock: &sync.RWMutex{},
        NodeMap:       nodeMap,
        EdgeSourceMap: edgesMap,
    }

    result := nodeFor.RunSync(ctx, req)
    if result.Err != nil {
        t.Errorf("Expected err to be nil, but got %v", result.Err)
    }
    testutil.Assert(t, mockNode1ID, result.NextNodeID[0])
}
```

**After:**

```go
func TestIFNode_ConditionTrue(t *testing.T) {
    creator := func() node.FlowNode {
        return nif.New(idwrap.NewNow(), "test", mcondition.Condition{
            Comparisons: mcondition.Comparison{Expression: "1 == 1"},
        })
    }

    opts := DefaultTestNodeOptions()
    opts.EdgeMap = edge.EdgesMap{
        creator().GetID(): {
            edge.HandleThen: {idwrap.NewNow()}, // Dummy then target
            edge.HandleElse: {idwrap.NewNow()}, // Dummy else target
        },
    }

    TestNodeSuccess(t, creator(), opts)
}
```

### Node Variable Tests Migration

**Before:**

```go
func TestAddNodeVar(t *testing.T) {
    req := &node.FlowNodeRequest{
        VarMap:        make(map[string]interface{}),
        ReadWriteLock: &sync.RWMutex{},
    }

    key := "testKey"
    value := "testValue"
    nodeName := "test-node"

    err := node.WriteNodeVar(req, nodeName, key, value)
    if err != nil {
        t.Fatalf("expected no error, got %v", err)
    }

    storedValue, err := node.ReadNodeVar(req, nodeName, key)
    if err != nil {
        t.Fatalf("expected no error, got %v", err)
    }

    if storedValue != value {
        t.Fatalf("expected %v, got %v", value, storedValue)
    }
}
```

**After:**

```go
func TestNodeVariableOperations(t *testing.T) {
    ctx := NewTestContext(t)

    // Test writing and reading variables
    req := ctx.CreateNodeRequest()

    err := node.WriteNodeVar(req, "test-node", "testKey", "testValue")
    require.NoError(t, err)

    storedValue, err := node.ReadNodeVar(req, "test-node", "testKey")
    require.NoError(t, err)
    require.Equal(t, "testValue", storedValue)
}
```

## Framework Functions Reference

### Core Test Functions

| Function            | Purpose                   | When to Use                      |
| ------------------- | ------------------------- | -------------------------------- |
| `TestNodeSuccess()` | Test successful execution | Basic happy path tests           |
| `TestNodeError()`   | Test error handling       | Tests that should produce errors |
| `TestNodeTimeout()` | Test timeout behavior     | Timeout scenario tests           |
| `TestNodeAsync()`   | Test async execution      | Tests using RunAsync             |
| `RunNodeTests()`    | Run multiple test cases   | Table-driven tests               |

### Configuration Options

```go
type TestNodeOptions struct {
    VarMap             map[string]any           // Variables for the request
    EdgeMap            edge.EdgesMap           // Edge configuration
    Timeout            time.Duration           // Request timeout
    ExpectStatusEvents bool                    // Whether node emits status events
    StatusValidator    *StatusValidator        // Custom status validation
}
```

### Test Context Helpers

```go
ctx := NewTestContext(t)

// Create requests with defaults
req := ctx.CreateNodeRequest()

// Create with custom options
req := ctx.CreateNodeRequestWithOptions(TestNodeOptions{
    VarMap: map[string]any{"key": "value"},
})

// Create iteration contexts for loops
iterCtx := ctx.CreateIterationContext(1, 3)

// Create test statuses
status := ctx.CreateTestStatus(node.NodeStateRunning)

// Assertions
ctx.AssertEqual("expected", "actual", "values should match")
ctx.AssertNotNil(object, "object should not be nil")
```

## Migration Checklist

For each test file you migrate:

- [ ] **Identify the test pattern** (success, error, timeout, async)
- [ ] **Create a node creator function**
- [ ] **Choose the appropriate framework function**
- [ ] **Configure TestNodeOptions** (edges, variables, etc.)
- [ ] **Replace manual assertions** with framework helpers
- [ ] **Run the test** to ensure it still passes
- [ ] **Remove boilerplate code** (edge creation, node maps, etc.)
- [ ] **Add custom test cases** for node-specific behavior

## Common Migration Pitfalls

### 1. Edge Configuration

```go
// Wrong: Forgetting to configure edges
opts := DefaultTestNodeOptions()
TestNodeSuccess(t, node, opts) // May fail if node needs edges

// Right: Configure required edges
opts := DefaultTestNodeOptions()
opts.EdgeMap = edge.EdgesMap{
    node.GetID(): {
        edge.HandleThen: {idwrap.NewNow()},
    },
}
TestNodeSuccess(t, node, opts)
```

### 2. Status Events

```go
// Wrong: Not setting status expectation for FOR nodes
opts := DefaultTestNodeOptions()
opts.ExpectStatusEvents = false // FOR nodes emit status!

// Right: Enable status events for nodes that emit them
opts := DefaultTestNodeOptions()
opts.ExpectStatusEvents = true // FOR nodes need this
```

### 3. Variable Dependencies

```go
// Wrong: Not setting required variables
opts := DefaultTestNodeOptions()
// FOREACH node needs "items" variable

// Right: Set required variables
opts := DefaultTestNodeOptions()
opts.VarMap = map[string]any{
    "items": []string{"item1", "item2"},
}
```

## Benefits After Migration

- **Reduced code**: 50+ lines → 5-10 lines per test
- **Consistency**: All tests follow the same patterns
- **Maintainability**: Easier to add new test cases
- **Reliability**: Built-in validation and error handling
- **Readability**: Clear intent with descriptive function names

## Getting Help

- Check the `README.md` for detailed framework documentation
- Look at `example_test.go` for usage patterns
- Examine existing migrated tests in the codebase
- Run `go test ./packages/server/pkg/flow/node/testing -v` to see framework in action
